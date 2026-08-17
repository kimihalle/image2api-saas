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
			Where("user_id = ? AND status IN ?", userID, []string{"queued", "retry_wait", "processing"}).
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
		Where("user_id = ? AND status IN ?", userID, []string{"queued", "retry_wait", "processing"}).
		Order("created_at ASC").
		Limit(100).
		Find(&items).Error
	return items, err
}

func (r *GenerationJobRepository) CountActiveOwned(ctx context.Context, userID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.GenerationJob{}).
		Where("user_id = ? AND status IN ?", userID, []string{"queued", "retry_wait", "processing"}).
		Count(&count).Error
	return count, err
}

func (r *GenerationJobRepository) QueuePosition(ctx context.Context, item *model.GenerationJob) int64 {
	if item == nil || (item.Status != "queued" && item.Status != "retry_wait") {
		return 0
	}
	var count int64
	err := r.db.WithContext(ctx).Model(&model.GenerationJob{}).
		Where("status IN ? AND available_at <= ? AND (created_at < ? OR (created_at = ? AND id <= ?))", []string{"queued", "retry_wait"}, time.Now(), item.CreatedAt, item.CreatedAt, item.ID).
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
			Where("status IN ? AND available_at <= ?", []string{"queued", "retry_wait"}, time.Now()).
			Order("available_at ASC, created_at ASC").
			Limit(1).
			Find(&item)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		now := time.Now()
		if err := tx.Model(&model.GenerationJob{}).Where("id = ? AND status IN ?", item.ID, []string{"queued", "retry_wait"}).Updates(map[string]any{
			"status":     "processing",
			"stage":      "allocating",
			"progress":   10,
			"worker_id":  workerID,
			"locked_at":  &now,
			"started_at": gorm.Expr("COALESCE(started_at, ?)", now),
			"attempts":   gorm.Expr("attempts + 1"),
			"updated_at": now,
		}).Error; err != nil {
			return err
		}
		item.Status = "processing"
		item.Stage = "allocating"
		item.Progress = 10
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
		"stage":       "completed",
		"progress":    100,
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
		"stage":       "failed",
		"progress":    100,
		"payload":     nil,
		"error":       message,
		"finished_at": &now,
		"locked_at":   nil,
		"worker_id":   "",
		"updated_at":  now,
	}).Error
}

func (r *GenerationJobRepository) SetProgress(ctx context.Context, id, stage string, progress int) error {
	if progress < 0 {
		progress = 0
	}
	if progress > 99 {
		progress = 99
	}
	return r.db.WithContext(ctx).Model(&model.GenerationJob{}).Where("id = ? AND status = ?", id, "processing").Updates(map[string]any{
		"stage": stage, "progress": progress, "updated_at": time.Now(),
	}).Error
}

func (r *GenerationJobRepository) Retry(ctx context.Context, id, code, message string, availableAt time.Time) error {
	return r.db.WithContext(ctx).Model(&model.GenerationJob{}).Where("id = ? AND status = ?", id, "processing").Updates(map[string]any{
		"status": "retry_wait", "stage": "retry_wait", "progress": 0,
		"error_code": code, "last_error": message, "error": message, "retryable": true,
		"available_at": availableAt, "locked_at": nil, "worker_id": "", "updated_at": time.Now(),
	}).Error
}

func (r *GenerationJobRepository) DeadLetter(ctx context.Context, id, code, message string) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&model.GenerationJob{}).Where("id = ? AND status = ?", id, "processing").Updates(map[string]any{
		"status": "dead_letter", "stage": "dead_letter", "progress": 100,
		"error_code": code, "last_error": message, "error": message, "retryable": false,
		"dead_at": &now, "finished_at": &now, "locked_at": nil, "worker_id": "", "updated_at": now,
	}).Error
}

func (r *GenerationJobRepository) RetryDead(ctx context.Context, id string) (bool, error) {
	now := time.Now()
	result := r.db.WithContext(ctx).Model(&model.GenerationJob{}).Where("id = ? AND status = ?", id, "dead_letter").Updates(map[string]any{
		"status": "queued", "stage": "queued", "progress": 0, "attempts": 0,
		"error": "", "error_code": "", "last_error": "", "retryable": false,
		"dead_at": nil, "finished_at": nil, "available_at": now, "updated_at": now,
	})
	return result.RowsAffected == 1, result.Error
}

func (r *GenerationJobRepository) ListDead(ctx context.Context, limit, offset int) ([]model.GenerationJob, int64, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := r.db.WithContext(ctx).Model(&model.GenerationJob{}).Where("status = ?", "dead_letter")
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var out []model.GenerationJob
	err := q.Order("dead_at DESC").Limit(limit).Offset(offset).Find(&out).Error
	return out, total, err
}

func (r *GenerationJobRepository) CancelOwned(ctx context.Context, id, userID string) (bool, error) {
	now := time.Now()
	result := r.db.WithContext(ctx).Model(&model.GenerationJob{}).
		Where("id = ? AND user_id = ? AND status IN ?", id, userID, []string{"queued", "retry_wait"}).
		Updates(map[string]any{
			"status":      "cancelled",
			"stage":       "cancelled",
			"progress":    100,
			"payload":     nil,
			"finished_at": &now,
			"updated_at":  now,
		})
	return result.RowsAffected > 0, result.Error
}

func (r *GenerationJobRepository) RequeueStale(ctx context.Context, before time.Time) (int64, error) {
	result := r.db.WithContext(ctx).Model(&model.GenerationJob{}).
		Where("status = ? AND locked_at < ? AND attempts < max_attempts", "processing", before).
		Updates(map[string]any{
			"status":       "retry_wait",
			"stage":        "retry_wait",
			"progress":     0,
			"error_code":   "worker_timeout",
			"last_error":   "Worker 租约超时，任务已重新排队",
			"retryable":    true,
			"worker_id":    "",
			"locked_at":    nil,
			"available_at": time.Now(),
			"updated_at":   time.Now(),
		})
	if result.Error != nil {
		return result.RowsAffected, result.Error
	}
	now := time.Now()
	dead := r.db.WithContext(ctx).Model(&model.GenerationJob{}).
		Where("status = ? AND locked_at < ? AND attempts >= max_attempts", "processing", before).
		Updates(map[string]any{
			"status": "dead_letter", "stage": "dead_letter", "progress": 100,
			"error_code": "worker_timeout", "last_error": "Worker 多次超时，任务已进入死信队列",
			"error": "Worker 多次超时，任务已进入死信队列", "retryable": false,
			"dead_at": &now, "finished_at": &now, "locked_at": nil, "worker_id": "", "updated_at": now,
		})
	return result.RowsAffected + dead.RowsAffected, dead.Error
}

func (r *GenerationJobRepository) DeleteFinishedBefore(ctx context.Context, before time.Time) error {
	return r.db.WithContext(ctx).
		Where("status IN ? AND finished_at < ?", []string{"completed", "failed", "cancelled"}, before).
		Delete(&model.GenerationJob{}).Error
}
