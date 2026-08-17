package repo

import (
	"context"
	"errors"
	"time"

	"backend/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GenerationJobRepository struct {
	db *gorm.DB
}

var ErrGenerationQueueCapacity = errors.New("generation queue capacity exceeded")

func NewGenerationJobRepository(db *gorm.DB) *GenerationJobRepository {
	return &GenerationJobRepository{db: db}
}

func (r *GenerationJobRepository) CreateBatch(ctx context.Context, jobs []*model.GenerationJob) error {
	if len(jobs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&jobs).Error
}

// CreateBatchWithinLimit serializes submissions per user with a transaction
// advisory lock. Without it, two simultaneous requests could both observe the
// same active count and exceed the per-user queue limit.
func (r *GenerationJobRepository) CreateBatchWithinLimit(ctx context.Context, userID string, jobs []*model.GenerationJob, limit int64) error {
	if len(jobs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", "generation-queue:"+userID).Error; err != nil {
			return err
		}
		var active int64
		if err := tx.Model(&model.GenerationJob{}).
			Where("user_id = ? AND status IN ?", userID, []string{"queued", "processing"}).
			Count(&active).Error; err != nil {
			return err
		}
		if active+int64(len(jobs)) > limit {
			return ErrGenerationQueueCapacity
		}
		return tx.Create(&jobs).Error
	})
}

func (r *GenerationJobRepository) GetOwned(ctx context.Context, id, userID string) (*model.GenerationJob, error) {
	var item model.GenerationJob
	err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).First(&item).Error
	return &item, err
}

func (r *GenerationJobRepository) ListActiveOwned(ctx context.Context, userID string) ([]model.GenerationJob, error) {
	var items []model.GenerationJob
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND status IN ?", userID, []string{"queued", "processing"}).
		Order("created_at ASC").
		Limit(100).
		Find(&items).Error
	return items, err
}

func (r *GenerationJobRepository) CountActiveOwned(ctx context.Context, userID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.GenerationJob{}).
		Where("user_id = ? AND status IN ?", userID, []string{"queued", "processing"}).
		Count(&count).Error
	return count, err
}

func (r *GenerationJobRepository) QueuePosition(ctx context.Context, item *model.GenerationJob) int64 {
	if item == nil || item.Status != "queued" {
		return 0
	}
	var count int64
	err := r.db.WithContext(ctx).Model(&model.GenerationJob{}).
		Where("status = ? AND (created_at < ? OR (created_at = ? AND id <= ?))", "queued", item.CreatedAt, item.CreatedAt, item.ID).
		Count(&count).Error
	if err != nil {
		return 0
	}
	return count
}

// ClaimNext atomically leases one queued row. SKIP LOCKED lets multiple worker
// processes consume the same PostgreSQL queue without duplicate execution.
func (r *GenerationJobRepository) ClaimNext(ctx context.Context, workerID string) (*model.GenerationJob, error) {
	var claimed *model.GenerationJob
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item model.GenerationJob
		result := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = ? AND available_at <= ?", "queued", time.Now()).
			Order("created_at ASC").
			Limit(1).
			Find(&item)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		now := time.Now()
		if err := tx.Model(&model.GenerationJob{}).Where("id = ? AND status = ?", item.ID, "queued").Updates(map[string]any{
			"status":     "processing",
			"worker_id":  workerID,
			"locked_at":  &now,
			"started_at": gorm.Expr("COALESCE(started_at, ?)", now),
			"attempts":   gorm.Expr("attempts + 1"),
			"updated_at": now,
		}).Error; err != nil {
			return err
		}
		item.Status = "processing"
		item.WorkerID = workerID
		item.LockedAt = &now
		item.StartedAt = &now
		item.Attempts++
		claimed = &item
		return nil
	})
	return claimed, err
}

func (r *GenerationJobRepository) Complete(ctx context.Context, id string, result []byte) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&model.GenerationJob{}).Where("id = ? AND status = ?", id, "processing").Updates(map[string]any{
		"status":      "completed",
		"result":      result,
		"payload":     nil,
		"error":       "",
		"finished_at": &now,
		"locked_at":   nil,
		"worker_id":   "",
		"updated_at":  now,
	}).Error
}

func (r *GenerationJobRepository) Fail(ctx context.Context, id, message string) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&model.GenerationJob{}).Where("id = ? AND status = ?", id, "processing").Updates(map[string]any{
		"status":      "failed",
		"payload":     nil,
		"error":       message,
		"finished_at": &now,
		"locked_at":   nil,
		"worker_id":   "",
		"updated_at":  now,
	}).Error
}

func (r *GenerationJobRepository) CancelOwned(ctx context.Context, id, userID string) (bool, error) {
	now := time.Now()
	result := r.db.WithContext(ctx).Model(&model.GenerationJob{}).
		Where("id = ? AND user_id = ? AND status = ?", id, userID, "queued").
		Updates(map[string]any{
			"status":      "cancelled",
			"payload":     nil,
			"finished_at": &now,
			"updated_at":  now,
		})
	return result.RowsAffected > 0, result.Error
}

func (r *GenerationJobRepository) RequeueStale(ctx context.Context, before time.Time) (int64, error) {
	result := r.db.WithContext(ctx).Model(&model.GenerationJob{}).
		Where("status = ? AND locked_at < ?", "processing", before).
		Updates(map[string]any{
			"status":       "queued",
			"worker_id":    "",
			"locked_at":    nil,
			"available_at": time.Now(),
			"updated_at":   time.Now(),
		})
	return result.RowsAffected, result.Error
}

func (r *GenerationJobRepository) DeleteFinishedBefore(ctx context.Context, before time.Time) error {
	return r.db.WithContext(ctx).
		Where("status IN ? AND finished_at < ?", []string{"completed", "failed", "cancelled"}, before).
		Delete(&model.GenerationJob{}).Error
}
