package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"backend/internal/model"
	"backend/internal/repo"
	"github.com/google/uuid"
)

type OperationsService struct {
	ops      *repo.OperationsRepository
	credits  *repo.CreditRepository
	settings *repo.SiteSettingRepository
	client   *http.Client
	mu       sync.Mutex
	notifyMu sync.Mutex
	notified map[string]time.Time
}

type ReliabilitySettings struct {
	AlertWebhookURL    string `json:"alert_webhook_url"`
	BackupEnabled      bool   `json:"backup_enabled"`
	BackupRetentionDay int    `json:"backup_retention_days"`
}

func (s *OperationsService) ReliabilitySettings(ctx context.Context) (*ReliabilitySettings, error) {
	webhook, err := s.settings.GetValue(ctx, "alerts.webhook_url")
	if err != nil {
		return nil, err
	}
	enabledRaw, err := s.settings.GetValue(ctx, "backup.enabled")
	if err != nil {
		return nil, err
	}
	retentionRaw, err := s.settings.GetValue(ctx, "backup.retention_days")
	if err != nil {
		return nil, err
	}
	retention := 14
	if parsed, parseErr := strconv.Atoi(strings.TrimSpace(retentionRaw)); parseErr == nil && parsed >= 1 && parsed <= 365 {
		retention = parsed
	}
	return &ReliabilitySettings{
		AlertWebhookURL:    strings.TrimSpace(webhook),
		BackupEnabled:      enabledRaw == "" || strings.EqualFold(enabledRaw, "true"),
		BackupRetentionDay: retention,
	}, nil
}

func (s *OperationsService) SaveReliabilitySettings(ctx context.Context, in ReliabilitySettings) error {
	if in.BackupRetentionDay < 1 || in.BackupRetentionDay > 365 {
		return errors.New("备份保留天数必须为 1 到 365")
	}
	webhook := strings.TrimSpace(in.AlertWebhookURL)
	if webhook != "" && !strings.HasPrefix(webhook, "https://") && !strings.HasPrefix(webhook, "http://") {
		return errors.New("告警 Webhook 必须使用 http 或 https 地址")
	}
	return s.settings.UpsertValues(ctx, map[string]string{
		"alerts.webhook_url":    webhook,
		"backup.enabled":        strconv.FormatBool(in.BackupEnabled),
		"backup.retention_days": strconv.Itoa(in.BackupRetentionDay),
	})
}

func NewOperationsService(ops *repo.OperationsRepository, credits *repo.CreditRepository, settings *repo.SiteSettingRepository) *OperationsService {
	return &OperationsService{
		ops: ops, credits: credits, settings: settings,
		client: &http.Client{Timeout: 8 * time.Second}, notified: map[string]time.Time{},
	}
}

func (s *OperationsService) Snapshot(ctx context.Context) (map[string]any, error) {
	metrics, err := s.ops.OperationalMetrics(ctx)
	if err != nil {
		return nil, err
	}
	alerts, err := s.ops.ListAlerts(ctx, "", 50)
	if err != nil {
		return nil, err
	}
	reconciliations, err := s.ops.ListReconciliationRuns(ctx, 12)
	if err != nil {
		return nil, err
	}
	backups, err := s.ops.ListBackupRuns(ctx, 12)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"metrics": metrics, "alerts": alerts,
		"reconciliations": reconciliations, "backups": backups,
	}, nil
}

func (s *OperationsService) Alerts(ctx context.Context, status string) ([]model.SystemAlert, error) {
	return s.ops.ListAlerts(ctx, status, 100)
}

func (s *OperationsService) ResolveAlert(ctx context.Context, id string) error {
	return s.ops.ResolveAlert(ctx, strings.TrimSpace(id))
}

func (s *OperationsService) ReconciliationRuns(ctx context.Context) ([]model.ReconciliationRun, error) {
	return s.ops.ListReconciliationRuns(ctx, 50)
}

func (s *OperationsService) ReconciliationIssues(ctx context.Context, runID string) ([]model.ReconciliationIssue, error) {
	return s.ops.ListReconciliationIssues(ctx, strings.TrimSpace(runID))
}

func (s *OperationsService) RunReconciliation(ctx context.Context) (*model.ReconciliationRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	run := &model.ReconciliationRun{
		ID: "rec-" + uuid.NewString(), Status: "running", StartedAt: now,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := s.ops.CreateReconciliationRun(ctx, run); err != nil {
		return nil, err
	}
	finish := func(status string, runErr error) {
		done := time.Now()
		run.Status, run.FinishedAt, run.UpdatedAt = status, &done, done
		if runErr != nil {
			run.Error = runErr.Error()
		}
		if err := s.ops.FinishReconciliationRun(context.Background(), run); err != nil {
			log.Printf("reconciliation: finish run=%s: %v", run.ID, err)
		}
	}

	candidates, err := s.ops.ReconciliationCandidates(ctx, now.Add(-10*time.Minute))
	if err != nil {
		finish("failed", err)
		return run, err
	}
	run.Checked = int64(len(candidates))
	for _, candidate := range candidates {
		issue := &model.ReconciliationIssue{
			ID: "ri-" + uuid.NewString(), RunID: run.ID, Reference: candidate.TransactionID,
			UserID: candidate.UserID, Actual: candidate.TxStatus, Status: "unresolved",
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
		eventStatus := "missing"
		if candidate.EventStatus != nil {
			eventStatus = *candidate.EventStatus
		}
		switch {
		case candidate.TxStatus == "reserved" && eventStatus == "success":
			issue.Kind, issue.Expected = "success_not_captured", "captured"
			issue.Detail = "生成已成功，但额度仍处于预扣状态"
			if changed, e := s.credits.Capture(ctx, candidate.TransactionID); e == nil && changed {
				issue.Status = "repaired"
				run.AutoRepaired++
			} else if e != nil {
				issue.Detail += ": " + e.Error()
			}
		case candidate.TxStatus == "reserved" && eventStatus == "failed":
			issue.Kind, issue.Expected = "failed_not_refunded", "refunded"
			issue.Detail = "生成已失败，但额度仍处于预扣状态"
			if _, changed, e := s.credits.Refund(ctx, candidate.TransactionID); e == nil && changed {
				issue.Status = "repaired"
				run.AutoRepaired++
			} else if e != nil {
				issue.Detail += ": " + e.Error()
			}
		case candidate.TxStatus == "reserved" && eventStatus == "missing":
			issue.Kind, issue.Expected = "orphan_reservation", "refunded"
			issue.Detail = "预扣记录未绑定生成事件且已超过安全窗口"
			if _, changed, e := s.credits.Refund(ctx, candidate.TransactionID); e == nil && changed {
				issue.Status = "repaired"
				run.AutoRepaired++
			} else if e != nil {
				issue.Detail += ": " + e.Error()
			}
		case candidate.TxStatus == "reserved" && eventStatus == "pending":
			issue.Kind, issue.Expected = "stale_pending_reservation", "manual_review"
			issue.Detail = "生成事件长期处于处理中，系统为避免错误退款已保留现场等待人工核对"
		case candidate.TxStatus == "captured" && eventStatus == "failed":
			issue.Kind, issue.Expected = "failed_but_captured", "refunded"
			issue.Detail = "生成失败但额度已结算，自动执行差错退款"
			if _, changed, e := s.credits.RefundCapturedMismatch(ctx, candidate.TransactionID); e == nil && changed {
				issue.Status = "repaired"
				run.AutoRepaired++
			} else if e != nil {
				issue.Detail += ": " + e.Error()
			}
		default:
			issue.Kind, issue.Expected = "refund_success_conflict", "manual_review"
			issue.Detail = "成功生成对应的账本已退款，需要人工核对上游结果"
		}
		run.Issues++
		if issue.Status != "repaired" {
			run.Unresolved++
		}
		_ = s.ops.CreateReconciliationIssue(ctx, issue)
	}

	negative, err := s.ops.NegativeBalanceUsers(ctx)
	if err != nil {
		finish("failed", err)
		return run, err
	}
	for _, user := range negative {
		run.Checked++
		run.Issues++
		run.Unresolved++
		_ = s.ops.CreateReconciliationIssue(ctx, &model.ReconciliationIssue{
			ID: "ri-" + uuid.NewString(), RunID: run.ID, Kind: "negative_balance", Reference: user.ID,
			UserID: user.ID, Expected: ">= 0", Actual: fmt.Sprintf("%.4f", user.Credits),
			Detail: "用户余额为负数，已保留现场等待人工核对", Status: "unresolved",
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		})
	}
	finish("completed", nil)
	if run.Unresolved > 0 {
		s.raise(ctx, "finance:reconciliation", "finance", "critical", "自动对账发现未解决差异", fmt.Sprintf("批次 %s 仍有 %d 条差异需要人工处理", run.ID, run.Unresolved))
	} else {
		_ = s.ops.ResolveByFingerprint(ctx, "finance:reconciliation")
	}
	return run, nil
}

func (s *OperationsService) Evaluate(ctx context.Context) error {
	metrics, err := s.ops.OperationalMetrics(ctx)
	if err != nil {
		return err
	}
	s.condition(ctx, metrics.QueueQueued >= 100 || metrics.OldestQueueSecs >= 300,
		"queue:backlog", "queue", "warning", "生成队列出现积压",
		fmt.Sprintf("排队 %d，最早任务已等待 %d 秒", metrics.QueueQueued, metrics.OldestQueueSecs))
	s.condition(ctx, metrics.QueueDead > 0, "queue:dead-letter", "queue", "critical", "存在死信任务",
		fmt.Sprintf("当前有 %d 个任务重试耗尽，需要运营处理", metrics.QueueDead))
	s.condition(ctx, metrics.AccountsTotal > 0 && metrics.AccountsActive == 0,
		"provider:no-active-account", "provider", "critical", "没有可调度的上游账号", "全部账号均不可用或正在冷却")
	total15m := metrics.Success15m + metrics.Failed15m
	failureHigh := total15m >= 10 && float64(metrics.Failed15m)/float64(total15m) >= 0.30
	s.condition(ctx, failureHigh, "generation:failure-rate", "generation", "critical", "近期生成失败率过高",
		fmt.Sprintf("最近 15 分钟成功 %d、失败 %d", metrics.Success15m, metrics.Failed15m))
	s.condition(ctx, metrics.ReservedCredits >= 50, "finance:reserved-backlog", "finance", "warning", "预扣账本数量异常",
		fmt.Sprintf("当前有 %d 条额度仍处于预扣状态", metrics.ReservedCredits))
	return nil
}

func (s *OperationsService) condition(ctx context.Context, active bool, fingerprint, category, severity, title, message string) {
	if active {
		s.raise(ctx, fingerprint, category, severity, title, message)
		return
	}
	_ = s.ops.ResolveByFingerprint(ctx, fingerprint)
}

func (s *OperationsService) raise(ctx context.Context, fingerprint, category, severity, title, message string) {
	now := time.Now()
	item := &model.SystemAlert{
		ID: "alt-" + uuid.NewString(), Fingerprint: fingerprint, Category: category,
		Severity: severity, Title: title, Message: message, Status: "open", Count: 1,
		FirstSeenAt: now, LastSeenAt: now, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.ops.UpsertAlert(ctx, item); err != nil {
		log.Printf("operations: alert %s: %v", fingerprint, err)
		return
	}
	s.notifyAlert(item)
}

func (s *OperationsService) notifyAlert(item *model.SystemAlert) {
	s.notifyMu.Lock()
	if last := s.notified[item.Fingerprint]; time.Since(last) < 30*time.Minute {
		s.notifyMu.Unlock()
		return
	}
	s.notified[item.Fingerprint] = time.Now()
	s.notifyMu.Unlock()
	url, _ := s.settings.GetValue(context.Background(), "alerts.webhook_url")
	url = strings.TrimSpace(url)
	if url == "" {
		return
	}
	body, _ := json.Marshal(map[string]any{
		"event": "system.alert", "severity": item.Severity, "title": item.Title,
		"message": item.Message, "fingerprint": item.Fingerprint, "timestamp": item.LastSeenAt,
	})
	go func() {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := s.client.Do(req)
		if err != nil {
			log.Printf("operations: alert webhook: %v", err)
			return
		}
		resp.Body.Close()
	}()
}

func (s *OperationsService) Run(ctx context.Context) {
	evaluateTicker := time.NewTicker(time.Minute)
	reconcileTicker := time.NewTicker(24 * time.Hour)
	defer evaluateTicker.Stop()
	defer reconcileTicker.Stop()
	_ = s.Evaluate(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-evaluateTicker.C:
			if err := s.Evaluate(ctx); err != nil {
				log.Printf("operations: evaluate: %v", err)
			}
		case <-reconcileTicker.C:
			if _, err := s.RunReconciliation(ctx); err != nil {
				log.Printf("operations: reconcile: %v", err)
			}
		}
	}
}
