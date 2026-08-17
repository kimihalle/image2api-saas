package repo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"backend/internal/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func openGenerationQueueIntegrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("GENERATION_QUEUE_INTEGRATION_DSN")
	if dsn == "" {
		t.Skip("GENERATION_QUEUE_INTEGRATION_DSN is not configured")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	if err := db.AutoMigrate(&model.GenerationJob{}); err != nil {
		t.Fatalf("migrate generation jobs: %v", err)
	}
	return db
}

func TestGenerationQueueConcurrentClaimCancelAndRecovery(t *testing.T) {
	db := openGenerationQueueIntegrationDB(t)
	ctx := context.Background()
	repository := NewGenerationJobRepository(db)
	prefix := fmt.Sprintf("qtest-%d", time.Now().UnixNano())
	now := time.Now()
	rows := make([]*model.GenerationJob, 8)
	for i := range rows {
		rows[i] = &model.GenerationJob{
			ID: prefix + fmt.Sprintf("-%02d", i), UserID: prefix,
			Kind: "image", Status: "queued", Payload: []byte(`{}`),
			AvailableAt: now, CreatedAt: now.Add(time.Duration(i) * time.Microsecond), UpdatedAt: now,
		}
	}
	if err := repository.CreateBatch(ctx, rows); err != nil {
		t.Fatalf("create jobs: %v", err)
	}
	t.Cleanup(func() { db.Where("id LIKE ?", prefix+"%").Delete(&model.GenerationJob{}) })

	claimed := make(chan string, len(rows))
	errs := make(chan error, len(rows))
	var wg sync.WaitGroup
	for i := range rows {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			item, err := repository.ClaimNext(ctx, fmt.Sprintf("worker-%d", worker))
			if err != nil {
				errs <- err
				return
			}
			if item == nil {
				errs <- fmt.Errorf("worker %d did not claim a job", worker)
				return
			}
			claimed <- item.ID
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	close(claimed)
	seen := map[string]bool{}
	for id := range claimed {
		if seen[id] {
			t.Fatalf("job %s was claimed more than once", id)
		}
		seen[id] = true
	}
	if len(seen) != len(rows) {
		t.Fatalf("claimed %d jobs, want %d", len(seen), len(rows))
	}

	if ok, err := repository.CancelOwned(ctx, rows[0].ID, prefix); err != nil || ok {
		t.Fatalf("cancel processing job = (%v, %v), want false", ok, err)
	}
	stale := now.Add(-20 * time.Minute)
	if err := db.Model(&model.GenerationJob{}).Where("id = ?", rows[0].ID).Update("locked_at", stale).Error; err != nil {
		t.Fatalf("make job stale: %v", err)
	}
	if count, err := repository.RequeueStale(ctx, now.Add(-15*time.Minute)); err != nil || count != 1 {
		t.Fatalf("requeue stale = (%d, %v), want 1", count, err)
	}
	if ok, err := repository.CancelOwned(ctx, rows[0].ID, prefix); err != nil || !ok {
		t.Fatalf("cancel queued job = (%v, %v), want true", ok, err)
	}
}

func TestGenerationQueueConcurrentSubmissionsRespectUserLimit(t *testing.T) {
	db := openGenerationQueueIntegrationDB(t)
	ctx := context.Background()
	repository := NewGenerationJobRepository(db)
	prefix := fmt.Sprintf("qlimit-%d", time.Now().UnixNano())
	now := time.Now()
	const batches = 10
	const perBatch = 4
	const limit = 20
	var wg sync.WaitGroup
	errs := make(chan error, batches)
	for batch := 0; batch < batches; batch++ {
		wg.Add(1)
		go func(batch int) {
			defer wg.Done()
			rows := make([]*model.GenerationJob, perBatch)
			for index := range rows {
				rows[index] = &model.GenerationJob{
					ID: fmt.Sprintf("lim-%d-%02d-%02d", now.UnixNano(), batch, index), UserID: prefix,
					Kind: "image", Status: "queued", Payload: []byte(`{}`),
					AvailableAt: now.Add(time.Hour), CreatedAt: now, UpdatedAt: now,
				}
			}
			errs <- repository.CreateBatchWithinLimit(ctx, prefix, rows, limit)
		}(batch)
	}
	wg.Wait()
	close(errs)
	accepted := 0
	for err := range errs {
		switch {
		case err == nil:
			accepted += perBatch
		case errors.Is(err, ErrGenerationQueueCapacity):
		default:
			t.Fatalf("unexpected submission error: %v", err)
		}
	}
	t.Cleanup(func() { db.Where("user_id = ?", prefix).Delete(&model.GenerationJob{}) })
	if accepted != limit {
		t.Fatalf("accepted %d jobs, want exactly %d", accepted, limit)
	}
	active, err := repository.CountActiveOwned(ctx, prefix)
	if err != nil || active != limit {
		t.Fatalf("active jobs = (%d, %v), want %d", active, err, limit)
	}
}
