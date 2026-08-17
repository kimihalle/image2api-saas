package repo

import (
	"context"
	"time"

	"backend/internal/model"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type OperationsRepository struct{ db *gorm.DB }

func NewOperationsRepository(db *gorm.DB) *OperationsRepository { return &OperationsRepository{db: db} }

type OperationalMetrics struct {
	QueueQueued     int64   `json:"queue_queued"`
	QueueProcessing int64   `json:"queue_processing"`
	QueueRetrying   int64   `json:"queue_retrying"`
	QueueDead       int64   `json:"queue_dead"`
	OldestQueueSecs int64   `json:"oldest_queue_seconds"`
	AccountsTotal   int64   `json:"accounts_total"`
	AccountsActive  int64   `json:"accounts_active"`
	AccountsCooling int64   `json:"accounts_cooling"`
	AccountsDead    int64   `json:"accounts_dead"`
	Success15m      int64   `json:"success_15m"`
	Failed15m       int64   `json:"failed_15m"`
	PendingEvents   int64   `json:"pending_events"`
	ReservedCredits int64   `json:"reserved_credits"`
	PaidOrdersToday int64   `json:"paid_orders_today"`
	RevenueToday    float64 `json:"revenue_today"`
	OpenAlerts      int64   `json:"open_alerts"`
}

func (r *OperationsRepository) OperationalMetrics(ctx context.Context) (*OperationalMetrics, error) {
	var out OperationalMetrics
	var queue struct {
		QueueQueued, QueueProcessing, QueueRetrying, QueueDead, OldestQueueSecs int64
	}
	if err := r.db.WithContext(ctx).Raw(`SELECT
			COUNT(*) FILTER (WHERE status='queued') queue_queued,
			COUNT(*) FILTER (WHERE status='processing') queue_processing,
			COUNT(*) FILTER (WHERE status='retry_wait') queue_retrying,
			COUNT(*) FILTER (WHERE status='dead_letter') queue_dead,
			COALESCE(EXTRACT(EPOCH FROM (NOW()-MIN(created_at) FILTER (WHERE status='queued')))::bigint,0) oldest_queue_secs
			FROM generation_jobs`).Scan(&queue).Error; err != nil {
		return nil, err
	}
	out.QueueQueued, out.QueueProcessing, out.QueueRetrying = queue.QueueQueued, queue.QueueProcessing, queue.QueueRetrying
	out.QueueDead, out.OldestQueueSecs = queue.QueueDead, queue.OldestQueueSecs

	var accounts struct{ AccountsTotal, AccountsActive, AccountsCooling, AccountsDead int64 }
	if err := r.db.WithContext(ctx).Raw(`SELECT
		COUNT(*) accounts_total,
		COUNT(*) FILTER (WHERE status='active' AND dead=false AND (cooldown_until IS NULL OR cooldown_until <= NOW())) accounts_active,
		COUNT(*) FILTER (WHERE cooldown_until > NOW()) accounts_cooling,
		COUNT(*) FILTER (WHERE dead=true OR status='disabled') accounts_dead
		FROM token_accounts`).Scan(&accounts).Error; err != nil {
		return nil, err
	}
	out.AccountsTotal, out.AccountsActive = accounts.AccountsTotal, accounts.AccountsActive
	out.AccountsCooling, out.AccountsDead = accounts.AccountsCooling, accounts.AccountsDead

	var events struct{ Success15m, Failed15m, PendingEvents int64 }
	if err := r.db.WithContext(ctx).Raw(`SELECT
			COUNT(*) FILTER (WHERE status='success' AND ts >= NOW()-INTERVAL '15 minutes') success_15m,
			COUNT(*) FILTER (WHERE status='failed' AND ts >= NOW()-INTERVAL '15 minutes') failed_15m,
		COUNT(*) FILTER (WHERE status='pending') pending_events
		FROM event_logs`).Scan(&events).Error; err != nil {
		return nil, err
	}
	out.Success15m, out.Failed15m, out.PendingEvents = events.Success15m, events.Failed15m, events.PendingEvents
	if err := r.db.WithContext(ctx).Model(&model.CreditTransaction{}).Where("status = ?", "reserved").Count(&out.ReservedCredits).Error; err != nil {
		return nil, err
	}
	var orders struct {
		PaidOrdersToday int64
		RevenueToday    float64
	}
	if err := r.db.WithContext(ctx).Raw(`SELECT COUNT(*) paid_orders_today, COALESCE(SUM(amount),0) revenue_today
		FROM orders WHERE status='paid' AND source='epay' AND paid_at >= date_trunc('day', NOW())`).Scan(&orders).Error; err != nil {
		return nil, err
	}
	out.PaidOrdersToday, out.RevenueToday = orders.PaidOrdersToday, orders.RevenueToday
	if err := r.db.WithContext(ctx).Model(&model.SystemAlert{}).Where("status = ?", "open").Count(&out.OpenAlerts).Error; err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *OperationsRepository) UpsertAlert(ctx context.Context, item *model.SystemAlert) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "fingerprint"}},
		DoUpdates: clause.Assignments(map[string]any{
			"category":     item.Category,
			"severity":     item.Severity,
			"title":        item.Title,
			"message":      item.Message,
			"status":       "open",
			"count":        gorm.Expr("system_alerts.count + 1"),
			"last_seen_at": item.LastSeenAt,
			"resolved_at":  nil,
			"updated_at":   item.UpdatedAt,
		}),
	}).Create(item).Error
}

func (r *OperationsRepository) ResolveAlert(ctx context.Context, id string) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&model.SystemAlert{}).Where("id = ?", id).
		Updates(map[string]any{"status": "resolved", "resolved_at": &now, "updated_at": now}).Error
}

func (r *OperationsRepository) ResolveByFingerprint(ctx context.Context, fingerprint string) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&model.SystemAlert{}).
		Where("fingerprint = ? AND status = ?", fingerprint, "open").
		Updates(map[string]any{"status": "resolved", "resolved_at": &now, "updated_at": now}).Error
}

func (r *OperationsRepository) ListAlerts(ctx context.Context, status string, limit int) ([]model.SystemAlert, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	q := r.db.WithContext(ctx).Model(&model.SystemAlert{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var out []model.SystemAlert
	err := q.Order("CASE severity WHEN 'critical' THEN 0 WHEN 'warning' THEN 1 ELSE 2 END, last_seen_at DESC").Limit(limit).Find(&out).Error
	return out, err
}

func (r *OperationsRepository) CreateReconciliationRun(ctx context.Context, item *model.ReconciliationRun) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *OperationsRepository) FinishReconciliationRun(ctx context.Context, item *model.ReconciliationRun) error {
	return r.db.WithContext(ctx).Model(&model.ReconciliationRun{}).Where("id = ?", item.ID).Updates(map[string]any{
		"status": item.Status, "checked": item.Checked, "issues": item.Issues,
		"auto_repaired": item.AutoRepaired, "unresolved": item.Unresolved,
		"error": item.Error, "finished_at": item.FinishedAt, "updated_at": item.UpdatedAt,
	}).Error
}

func (r *OperationsRepository) CreateReconciliationIssue(ctx context.Context, item *model.ReconciliationIssue) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *OperationsRepository) ListReconciliationRuns(ctx context.Context, limit int) ([]model.ReconciliationRun, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	var out []model.ReconciliationRun
	err := r.db.WithContext(ctx).Order("started_at DESC").Limit(limit).Find(&out).Error
	return out, err
}

func (r *OperationsRepository) ListReconciliationIssues(ctx context.Context, runID string) ([]model.ReconciliationIssue, error) {
	var out []model.ReconciliationIssue
	err := r.db.WithContext(ctx).Where("run_id = ?", runID).Order("created_at ASC").Find(&out).Error
	return out, err
}

type ReconciliationCandidate struct {
	TransactionID string    `gorm:"column:transaction_id"`
	UserID        string    `gorm:"column:user_id"`
	TxStatus      string    `gorm:"column:tx_status"`
	Amount        float64   `gorm:"column:amount"`
	CreatedAt     time.Time `gorm:"column:created_at"`
	EventID       *string   `gorm:"column:event_id"`
	EventStatus   *string   `gorm:"column:event_status"`
	EventRefunded *bool     `gorm:"column:event_refunded"`
}

func (r *OperationsRepository) ReconciliationCandidates(ctx context.Context, olderThan time.Time) ([]ReconciliationCandidate, error) {
	var out []ReconciliationCandidate
	err := r.db.WithContext(ctx).Raw(`
		SELECT ct.id AS transaction_id, ct.user_id, ct.status AS tx_status,
		       ct.amount, ct.created_at, ct.event_id,
		       e.status AS event_status, e.refunded AS event_refunded
		FROM credit_transactions ct
		LEFT JOIN event_logs e ON e.id = ct.event_id
		WHERE ct.created_at < ? AND (
			(ct.status = 'reserved' AND (e.status IN ('pending','success','failed') OR e.id IS NULL)) OR
			(ct.status = 'captured' AND e.status = 'failed') OR
			(ct.status = 'refunded' AND e.status = 'success')
		)
		ORDER BY ct.created_at ASC
		LIMIT 2000`, olderThan).Scan(&out).Error
	return out, err
}

func (r *OperationsRepository) NegativeBalanceUsers(ctx context.Context) ([]model.User, error) {
	var out []model.User
	err := r.db.WithContext(ctx).Where("credits < 0").Limit(200).Find(&out).Error
	return out, err
}

func (r *OperationsRepository) CreateBackupRun(ctx context.Context, item *model.BackupRun) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *OperationsRepository) FinishBackupRun(ctx context.Context, item *model.BackupRun) error {
	return r.db.WithContext(ctx).Model(&model.BackupRun{}).Where("id = ?", item.ID).Updates(map[string]any{
		"status": item.Status, "storage_key": item.StorageKey, "size_bytes": item.SizeBytes,
		"checksum": item.Checksum, "error": item.Error, "finished_at": item.FinishedAt,
		"updated_at": item.UpdatedAt,
	}).Error
}

func (r *OperationsRepository) ListBackupRuns(ctx context.Context, limit int) ([]model.BackupRun, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	var out []model.BackupRun
	err := r.db.WithContext(ctx).Order("started_at DESC").Limit(limit).Find(&out).Error
	return out, err
}

func (r *OperationsRepository) BeginIdempotency(ctx context.Context, item *model.IdempotencyRecord) (*model.IdempotencyRecord, bool, error) {
	created := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// A process can stop after reserving the key but before recording the
		// response. Reclaim expired records and abandoned processing leases so a
		// client is not told "still processing" for the full retention window.
		now := time.Now()
		if err := tx.Where("id = ? AND (expires_at < ? OR (status = ? AND updated_at < ?))",
			item.ID, now, "processing", now.Add(-30*time.Minute)).
			Delete(&model.IdempotencyRecord{}).Error; err != nil {
			return err
		}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(item)
		if result.Error != nil {
			return result.Error
		}
		created = result.RowsAffected == 1
		if created {
			return nil
		}
		return tx.First(item, "id = ?", item.ID).Error
	})
	return item, created, err
}

func (r *OperationsRepository) CompleteIdempotency(ctx context.Context, id string, httpStatus int, response []byte) error {
	return r.db.WithContext(ctx).Model(&model.IdempotencyRecord{}).Where("id = ?", id).Updates(map[string]any{
		"status": "completed", "http_status": httpStatus, "response": datatypes.JSON(response), "updated_at": time.Now(),
	}).Error
}

func (r *OperationsRepository) DeleteIdempotency(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&model.IdempotencyRecord{}, "id = ?", id).Error
}

func (r *OperationsRepository) DeleteExpiredIdempotency(ctx context.Context, now time.Time) error {
	return r.db.WithContext(ctx).Where("expires_at < ?", now).Delete(&model.IdempotencyRecord{}).Error
}
