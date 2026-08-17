package repo

import (
	"context"
	"errors"
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

func (r *OperationsRepository) UpsertAPIUsage(ctx context.Context, item *model.APIUsageBucket) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"requests":         gorm.Expr("api_usage_buckets.requests + EXCLUDED.requests"),
			"errors":           gorm.Expr("api_usage_buckets.errors + EXCLUDED.errors"),
			"latency_total_ms": gorm.Expr("api_usage_buckets.latency_total_ms + EXCLUDED.latency_total_ms"),
			"credits":          gorm.Expr("api_usage_buckets.credits + EXCLUDED.credits"),
			"updated_at":       item.UpdatedAt,
		}),
	}).Create(item).Error
}

type APIUsageSummary struct {
	Requests     int64   `json:"requests"`
	Errors       int64   `json:"errors"`
	AvgLatencyMS float64 `json:"avg_latency_ms"`
	Credits      float64 `json:"credits"`
}

type APIUsageDimension struct {
	Name         string  `json:"name"`
	Requests     int64   `json:"requests"`
	Errors       int64   `json:"errors"`
	AvgLatencyMS float64 `json:"avg_latency_ms"`
	Credits      float64 `json:"credits"`
}

func (r *OperationsRepository) APIUsageSummary(ctx context.Context, userID string, since time.Time) (*APIUsageSummary, []APIUsageDimension, []APIUsageDimension, error) {
	q := r.db.WithContext(ctx).Model(&model.APIUsageBucket{}).Where("bucket_start >= ?", since)
	if userID != "" {
		q = q.Where("user_id = ?", userID)
	}
	var summary APIUsageSummary
	if err := q.Select(`COALESCE(SUM(requests),0) requests, COALESCE(SUM(errors),0) errors,
		CASE WHEN SUM(requests)>0 THEN SUM(latency_total_ms)::float / SUM(requests) ELSE 0 END avg_latency_ms,
		COALESCE(SUM(credits),0) credits`).Scan(&summary).Error; err != nil {
		return nil, nil, nil, err
	}
	dimensions := func(column string) ([]APIUsageDimension, error) {
		var out []APIUsageDimension
		query := r.db.WithContext(ctx).Model(&model.APIUsageBucket{}).Where("bucket_start >= ?", since)
		if userID != "" {
			query = query.Where("user_id = ?", userID)
		}
		err := query.Select(column + ` AS name, SUM(requests) requests, SUM(errors) errors,
			CASE WHEN SUM(requests)>0 THEN SUM(latency_total_ms)::float / SUM(requests) ELSE 0 END avg_latency_ms,
			COALESCE(SUM(credits),0) credits`).Group(column).Order("requests DESC").Limit(20).Scan(&out).Error
		return out, err
	}
	byPath, err := dimensions("path")
	if err != nil {
		return nil, nil, nil, err
	}
	byModel, err := dimensions("COALESCE(NULLIF(model,''),'未指定')")
	return &summary, byPath, byModel, err
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

func (r *OperationsRepository) ListWebhookEndpoints(ctx context.Context, userID string) ([]model.WebhookEndpoint, error) {
	var out []model.WebhookEndpoint
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at DESC").Find(&out).Error
	return out, err
}

func (r *OperationsRepository) GetWebhookEndpoint(ctx context.Context, id, userID string) (*model.WebhookEndpoint, error) {
	var out model.WebhookEndpoint
	err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).First(&out).Error
	return &out, err
}

func (r *OperationsRepository) CreateWebhookEndpoint(ctx context.Context, item *model.WebhookEndpoint) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *OperationsRepository) UpdateWebhookEndpoint(ctx context.Context, id, userID string, patch map[string]any) error {
	return r.db.WithContext(ctx).Model(&model.WebhookEndpoint{}).Where("id = ? AND user_id = ?", id, userID).Updates(patch).Error
}

func (r *OperationsRepository) DeleteWebhookEndpoint(ctx context.Context, id, userID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("endpoint_id = ? AND user_id = ?", id, userID).Delete(&model.WebhookDelivery{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ? AND user_id = ?", id, userID).Delete(&model.WebhookEndpoint{}).Error
	})
}

func (r *OperationsRepository) EnabledWebhookEndpoints(ctx context.Context, userID, eventType string) ([]model.WebhookEndpoint, error) {
	var out []model.WebhookEndpoint
	err := r.db.WithContext(ctx).Where("user_id = ? AND enabled = ? AND events @> ?::jsonb", userID, true, `[`+`"`+eventType+`"`+`]`).Find(&out).Error
	return out, err
}

func (r *OperationsRepository) CreateWebhookDelivery(ctx context.Context, item *model.WebhookDelivery) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *OperationsRepository) ClaimWebhookDelivery(ctx context.Context) (*model.WebhookDelivery, error) {
	var claimed *model.WebhookDelivery
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item model.WebhookDelivery
		now := time.Now()
		result := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("(status IN ? AND next_attempt_at <= ?) OR (status = ? AND updated_at < ?)",
				[]string{"pending", "retry"}, now, "delivering", now.Add(-5*time.Minute)).
			Order("next_attempt_at ASC").Limit(1).Find(&item)
		if result.Error != nil || result.RowsAffected == 0 {
			return result.Error
		}
		if err := tx.Model(&model.WebhookDelivery{}).Where("id = ?", item.ID).Updates(map[string]any{
			"status": "delivering", "attempts": gorm.Expr("attempts + 1"), "updated_at": now,
		}).Error; err != nil {
			return err
		}
		item.Status = "delivering"
		item.Attempts++
		claimed = &item
		return nil
	})
	return claimed, err
}

func (r *OperationsRepository) FinishWebhookDelivery(ctx context.Context, item *model.WebhookDelivery) error {
	return r.db.WithContext(ctx).Model(&model.WebhookDelivery{}).Where("id = ?", item.ID).Updates(map[string]any{
		"status": item.Status, "http_status": item.HTTPStatus, "response_body": item.ResponseBody,
		"last_error": item.LastError, "next_attempt_at": item.NextAttemptAt,
		"delivered_at": item.DeliveredAt, "updated_at": time.Now(),
	}).Error
}

func (r *OperationsRepository) TouchWebhookEndpoint(ctx context.Context, item *model.WebhookEndpoint) error {
	return r.db.WithContext(ctx).Model(&model.WebhookEndpoint{}).Where("id = ?", item.ID).Updates(map[string]any{
		"last_status": item.LastStatus, "last_error": item.LastError, "last_called_at": item.LastCalledAt, "updated_at": time.Now(),
	}).Error
}

func (r *OperationsRepository) ListWebhookDeliveries(ctx context.Context, userID string, limit int) ([]model.WebhookDelivery, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var out []model.WebhookDelivery
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at DESC").Limit(limit).Find(&out).Error
	return out, err
}

func (r *OperationsRepository) ListWorkflowPresets(ctx context.Context, ownerID, kind string) ([]model.WorkflowPreset, error) {
	q := r.db.WithContext(ctx).Where("enabled = ? AND (owner_id = ? OR visibility = ?)", true, ownerID, "public")
	if kind != "" {
		q = q.Where("kind = ?", kind)
	}
	var out []model.WorkflowPreset
	err := q.Order("visibility DESC, use_count DESC, created_at DESC").Find(&out).Error
	return out, err
}

func (r *OperationsRepository) GetWorkflowPreset(ctx context.Context, id, ownerID string) (*model.WorkflowPreset, error) {
	var out model.WorkflowPreset
	err := r.db.WithContext(ctx).Where("id = ? AND enabled = ? AND (owner_id = ? OR visibility = ?)", id, true, ownerID, "public").First(&out).Error
	return &out, err
}

func (r *OperationsRepository) CreateWorkflowPreset(ctx context.Context, item *model.WorkflowPreset) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *OperationsRepository) UpdateWorkflowPreset(ctx context.Context, id, ownerID string, patch map[string]any) error {
	result := r.db.WithContext(ctx).Model(&model.WorkflowPreset{}).Where("id = ? AND owner_id = ?", id, ownerID).Updates(patch)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *OperationsRepository) DeleteWorkflowPreset(ctx context.Context, id, ownerID string) error {
	result := r.db.WithContext(ctx).Where("id = ? AND owner_id = ?", id, ownerID).Delete(&model.WorkflowPreset{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *OperationsRepository) IncrementWorkflowUse(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Model(&model.WorkflowPreset{}).Where("id = ? AND enabled = ?", id, true).
		UpdateColumn("use_count", gorm.Expr("use_count + 1"))
	if result.RowsAffected == 0 && result.Error == nil {
		return errors.New("workflow preset not found")
	}
	return result.Error
}
