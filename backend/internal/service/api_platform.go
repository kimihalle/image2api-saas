package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"backend/internal/model"
	"backend/internal/repo"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type APIUsageRecord struct {
	UserID, APIKeyID, Path, Model string
	Status, LatencyMS             int
	Credits                       float64
}

type APIPlatformService struct {
	ops    *repo.OperationsRepository
	usage  chan APIUsageRecord
	client *http.Client
}

func NewAPIPlatformService(ops *repo.OperationsRepository) *APIPlatformService {
	return &APIPlatformService{
		ops: ops, usage: make(chan APIUsageRecord, 4096),
		client: &http.Client{Timeout: 12 * time.Second},
	}
}

func (s *APIPlatformService) RecordUsage(record APIUsageRecord) {
	select {
	case s.usage <- record:
	default:
		// Metrics must never block customer traffic. The operations alerting layer
		// will expose sustained pressure through request/error trends.
	}
}

func (s *APIPlatformService) Run(ctx context.Context) {
	go s.runWebhookWorker(ctx)
	cleanup := time.NewTicker(time.Hour)
	defer cleanup.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case record := <-s.usage:
			s.persistUsage(ctx, record)
		case <-cleanup.C:
			_ = s.ops.DeleteExpiredIdempotency(ctx, time.Now())
		}
	}
}

func (s *APIPlatformService) persistUsage(ctx context.Context, record APIUsageRecord) {
	bucket := time.Now().UTC().Truncate(time.Hour)
	key := strings.Join([]string{bucket.Format(time.RFC3339), record.UserID, record.APIKeyID, record.Path, record.Model}, "|")
	sum := sha256.Sum256([]byte(key))
	errorsCount := int64(0)
	if record.Status >= 400 {
		errorsCount = 1
	}
	_ = s.ops.UpsertAPIUsage(ctx, &model.APIUsageBucket{
		ID: hex.EncodeToString(sum[:]), BucketStart: bucket, UserID: record.UserID,
		APIKeyID: record.APIKeyID, Path: record.Path, Model: record.Model,
		Requests: 1, Errors: errorsCount, LatencyTotalMS: int64(record.LatencyMS), Credits: record.Credits,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})
}

func (s *APIPlatformService) Usage(ctx context.Context, userID, rangeName string) (map[string]any, error) {
	duration := 7 * 24 * time.Hour
	switch rangeName {
	case "24h":
		duration = 24 * time.Hour
	case "30d":
		duration = 30 * 24 * time.Hour
	case "90d":
		duration = 90 * 24 * time.Hour
	}
	summary, paths, models, err := s.ops.APIUsageSummary(ctx, userID, time.Now().Add(-duration))
	if err != nil {
		return nil, err
	}
	return map[string]any{"summary": summary, "paths": paths, "models": models, "range": rangeName}, nil
}

type IdempotencyResult struct {
	ID         string
	Created    bool
	InProgress bool
	Conflict   bool
	HTTPStatus int
	Response   []byte
}

func (s *APIPlatformService) BeginIdempotency(ctx context.Context, userID, key string, request any) (*IdempotencyResult, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return &IdempotencyResult{Created: true}, nil
	}
	if len(key) > 255 {
		return nil, errors.New("Idempotency-Key 不能超过 255 个字符")
	}
	body, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(body)
	requestHash := hex.EncodeToString(hash[:])
	idHash := sha256.Sum256([]byte(userID + "|" + key))
	now := time.Now()
	item := &model.IdempotencyRecord{
		ID: hex.EncodeToString(idHash[:]), UserID: userID, Key: key, RequestHash: requestHash,
		Status: "processing", ExpiresAt: now.Add(24 * time.Hour), CreatedAt: now, UpdatedAt: now,
	}
	existing, created, err := s.ops.BeginIdempotency(ctx, item)
	if err != nil {
		return nil, err
	}
	result := &IdempotencyResult{ID: item.ID, Created: created}
	if created {
		return result, nil
	}
	if existing.RequestHash != requestHash {
		result.Conflict = true
		return result, nil
	}
	if existing.Status == "completed" {
		result.HTTPStatus, result.Response = existing.HTTPStatus, append([]byte(nil), existing.Response...)
		return result, nil
	}
	result.InProgress = true
	return result, nil
}

func (s *APIPlatformService) CompleteIdempotency(ctx context.Context, id string, status int, response any) error {
	if id == "" {
		return nil
	}
	body, err := json.Marshal(response)
	if err != nil {
		return err
	}
	return s.ops.CompleteIdempotency(ctx, id, status, body)
}

func (s *APIPlatformService) AbortIdempotency(ctx context.Context, id string) {
	if id != "" {
		_ = s.ops.DeleteIdempotency(ctx, id)
	}
}

type WebhookEndpointInput struct {
	Name    string   `json:"name"`
	URL     string   `json:"url"`
	Events  []string `json:"events"`
	Enabled bool     `json:"enabled"`
}

var allowedWebhookEvents = map[string]bool{
	"generation.completed": true, "generation.failed": true, "generation.dead_letter": true,
	"billing.refunded": true,
}

func (s *APIPlatformService) WebhookEndpoints(ctx context.Context, userID string) ([]model.WebhookEndpoint, error) {
	return s.ops.ListWebhookEndpoints(ctx, userID)
}

func (s *APIPlatformService) CreateWebhookEndpoint(ctx context.Context, userID string, in WebhookEndpointInput) (*model.WebhookEndpoint, string, error) {
	if err := validateWebhookURL(in.URL); err != nil {
		return nil, "", err
	}
	events, err := normalizeWebhookEvents(in.Events)
	if err != nil {
		return nil, "", err
	}
	secret, err := randomWebhookSecret()
	if err != nil {
		return nil, "", err
	}
	now := time.Now()
	encoded, _ := json.Marshal(events)
	item := &model.WebhookEndpoint{
		ID: "wh-" + uuid.NewString(), UserID: userID, Name: strings.TrimSpace(in.Name),
		URL: strings.TrimSpace(in.URL), Secret: secret, SecretHint: secret[len(secret)-6:],
		Events: encoded, Enabled: in.Enabled, CreatedAt: now, UpdatedAt: now,
	}
	if item.Name == "" {
		item.Name = "默认回调"
	}
	if err := s.ops.CreateWebhookEndpoint(ctx, item); err != nil {
		return nil, "", err
	}
	return item, secret, nil
}

func (s *APIPlatformService) UpdateWebhookEndpoint(ctx context.Context, userID, id string, in WebhookEndpointInput) error {
	if err := validateWebhookURL(in.URL); err != nil {
		return err
	}
	events, err := normalizeWebhookEvents(in.Events)
	if err != nil {
		return err
	}
	encoded, _ := json.Marshal(events)
	return s.ops.UpdateWebhookEndpoint(ctx, id, userID, map[string]any{
		"name": strings.TrimSpace(in.Name), "url": strings.TrimSpace(in.URL),
		"events": datatypes.JSON(encoded), "enabled": in.Enabled, "updated_at": time.Now(),
	})
}

func (s *APIPlatformService) DeleteWebhookEndpoint(ctx context.Context, userID, id string) error {
	return s.ops.DeleteWebhookEndpoint(ctx, id, userID)
}

func (s *APIPlatformService) WebhookDeliveries(ctx context.Context, userID string) ([]model.WebhookDelivery, error) {
	return s.ops.ListWebhookDeliveries(ctx, userID, 100)
}

func (s *APIPlatformService) EmitWebhook(ctx context.Context, userID, eventType, eventID string, data any) {
	if !allowedWebhookEvents[eventType] || strings.TrimSpace(userID) == "" {
		return
	}
	endpoints, err := s.ops.EnabledWebhookEndpoints(ctx, userID, eventType)
	if err != nil {
		return
	}
	payload, _ := json.Marshal(map[string]any{
		"id": "evt_" + uuid.NewString(), "type": eventType, "created": time.Now().Unix(), "data": data,
	})
	for _, endpoint := range endpoints {
		now := time.Now()
		_ = s.ops.CreateWebhookDelivery(ctx, &model.WebhookDelivery{
			ID: "wd-" + uuid.NewString(), EndpointID: endpoint.ID, UserID: userID,
			EventType: eventType, EventID: eventID, Payload: payload, Status: "pending",
			NextAttemptAt: now, CreatedAt: now, UpdatedAt: now,
		})
	}
}

func (s *APIPlatformService) TestWebhook(ctx context.Context, userID, id string) error {
	endpoint, err := s.ops.GetWebhookEndpoint(ctx, id, userID)
	if err != nil {
		return err
	}
	payload, _ := json.Marshal(map[string]any{
		"id": "evt_" + uuid.NewString(), "type": "webhook.test", "created": time.Now().Unix(),
		"data": map[string]any{"message": "Webhook 配置验证成功"},
	})
	now := time.Now()
	return s.ops.CreateWebhookDelivery(ctx, &model.WebhookDelivery{
		ID: "wd-" + uuid.NewString(), EndpointID: endpoint.ID, UserID: userID,
		EventType: "webhook.test", EventID: "test", Payload: payload, Status: "pending",
		NextAttemptAt: now, CreatedAt: now, UpdatedAt: now,
	})
}

func (s *APIPlatformService) runWebhookWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
		item, err := s.ops.ClaimWebhookDelivery(ctx)
		if err != nil || item == nil {
			continue
		}
		s.deliverWebhook(ctx, item)
	}
}

func (s *APIPlatformService) deliverWebhook(ctx context.Context, item *model.WebhookDelivery) {
	endpoint, err := s.ops.GetWebhookEndpoint(ctx, item.EndpointID, item.UserID)
	if err != nil || !endpoint.Enabled {
		item.Status, item.LastError = "failed", "Webhook 地址已删除或停用"
		_ = s.ops.FinishWebhookDelivery(ctx, item)
		return
	}
	timestamp := fmt.Sprint(time.Now().Unix())
	signature := webhookSignature(endpoint.Secret, timestamp, item.Payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.URL, bytes.NewReader(item.Payload))
	if err == nil {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Webhook-Id", item.ID)
		req.Header.Set("X-Webhook-Timestamp", timestamp)
		req.Header.Set("X-Webhook-Signature", signature)
	}
	var responseBody []byte
	if err == nil {
		var response *http.Response
		response, err = s.client.Do(req)
		if response != nil {
			item.HTTPStatus = response.StatusCode
			responseBody, _ = io.ReadAll(io.LimitReader(response.Body, 2048))
			response.Body.Close()
			if response.StatusCode < 200 || response.StatusCode >= 300 {
				err = fmt.Errorf("HTTP %d", response.StatusCode)
			}
		}
	}
	now := time.Now()
	item.ResponseBody = strings.TrimSpace(string(responseBody))
	endpoint.LastCalledAt, endpoint.LastStatus = &now, item.HTTPStatus
	if err == nil {
		item.Status, item.LastError, item.DeliveredAt = "delivered", "", &now
		endpoint.LastError = ""
	} else {
		item.LastError, endpoint.LastError = err.Error(), err.Error()
		if item.Attempts >= 8 {
			item.Status = "failed"
		} else {
			item.Status = "retry"
			delay := time.Duration(1<<min(item.Attempts, 8)) * time.Minute
			item.NextAttemptAt = now.Add(delay)
		}
	}
	_ = s.ops.FinishWebhookDelivery(ctx, item)
	_ = s.ops.TouchWebhookEndpoint(ctx, endpoint)
}

func webhookSignature(secret, timestamp string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "."))
	mac.Write(payload)
	return "v1=" + hex.EncodeToString(mac.Sum(nil))
}

func validateWebhookURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Hostname() == "" {
		return errors.New("请输入有效的 HTTP 或 HTTPS 回调地址")
	}
	addresses, err := net.LookupIP(parsed.Hostname())
	if err != nil {
		return errors.New("Webhook 域名无法解析")
	}
	for _, address := range addresses {
		if address.IsLoopback() || address.IsPrivate() || address.IsLinkLocalUnicast() || address.IsUnspecified() {
			return errors.New("Webhook 不允许访问内网或本机地址")
		}
	}
	return nil
}

func normalizeWebhookEvents(values []string) ([]string, error) {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if !allowedWebhookEvents[value] {
			return nil, fmt.Errorf("不支持的 Webhook 事件: %s", value)
		}
		if !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	if len(out) == 0 {
		return nil, errors.New("请至少选择一个 Webhook 事件")
	}
	return out, nil
}

func randomWebhookSecret() (string, error) {
	data := make([]byte, 32)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return "whsec_" + hex.EncodeToString(data), nil
}

type WorkflowPresetInput struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Kind        string           `json:"kind"`
	ModelID     string           `json:"model_id"`
	Prompt      string           `json:"prompt"`
	Visibility  string           `json:"visibility"`
	Variables   []PromptVariable `json:"variables"`
	Defaults    map[string]any   `json:"defaults"`
	Enabled     bool             `json:"enabled"`
}

func (s *APIPlatformService) WorkflowPresets(ctx context.Context, ownerID, kind string) ([]model.WorkflowPreset, error) {
	return s.ops.ListWorkflowPresets(ctx, ownerID, kind)
}

func (s *APIPlatformService) CreateWorkflowPreset(ctx context.Context, ownerID string, in WorkflowPresetInput, admin bool) (*model.WorkflowPreset, error) {
	item, err := normalizeWorkflowInput(in)
	if err != nil {
		return nil, err
	}
	if !admin || item.Visibility != "public" {
		item.Visibility = "private"
	}
	now := time.Now()
	item.ID, item.OwnerID, item.CreatedAt, item.UpdatedAt = "wf-"+uuid.NewString(), ownerID, now, now
	if item.Visibility == "public" {
		item.OwnerID = ""
	}
	if err := s.ops.CreateWorkflowPreset(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *APIPlatformService) UpdateWorkflowPreset(ctx context.Context, ownerID, id string, in WorkflowPresetInput) error {
	item, err := normalizeWorkflowInput(in)
	if err != nil {
		return err
	}
	return s.ops.UpdateWorkflowPreset(ctx, id, ownerID, map[string]any{
		"name": item.Name, "description": item.Description, "kind": item.Kind,
		"model_id": item.ModelID, "prompt": item.Prompt, "variables": item.Variables,
		"defaults": item.Defaults, "enabled": item.Enabled, "updated_at": time.Now(),
	})
}

func (s *APIPlatformService) DeleteWorkflowPreset(ctx context.Context, ownerID, id string) error {
	return s.ops.DeleteWorkflowPreset(ctx, id, ownerID)
}

func (s *APIPlatformService) RenderWorkflow(ctx context.Context, ownerID, id string, values map[string]string) (map[string]any, error) {
	item, err := s.ops.GetWorkflowPreset(ctx, id, ownerID)
	if err != nil {
		return nil, err
	}
	var variables []PromptVariable
	if len(item.Variables) > 0 {
		if err := json.Unmarshal(item.Variables, &variables); err != nil {
			return nil, errors.New("工作流变量配置损坏")
		}
	}
	prompt := item.Prompt
	for _, variable := range variables {
		value := strings.TrimSpace(values[variable.Name])
		if value == "" {
			value = variable.Default
		}
		if variable.Required && value == "" {
			return nil, fmt.Errorf("请填写%s", variable.Label)
		}
		prompt = strings.ReplaceAll(prompt, "{{"+variable.Name+"}}", value)
	}
	_ = s.ops.IncrementWorkflowUse(ctx, item.ID)
	return map[string]any{
		"id": item.ID, "kind": item.Kind, "model": item.ModelID, "prompt": prompt, "defaults": item.Defaults,
	}, nil
}

func normalizeWorkflowInput(in WorkflowPresetInput) (*model.WorkflowPreset, error) {
	in.Name, in.Kind, in.Prompt = strings.TrimSpace(in.Name), strings.TrimSpace(in.Kind), strings.TrimSpace(in.Prompt)
	if in.Name == "" || in.Prompt == "" || (in.Kind != "image" && in.Kind != "video") {
		return nil, errors.New("工作流名称、类型和提示词不能为空")
	}
	variables, _ := json.Marshal(in.Variables)
	if in.Defaults == nil {
		in.Defaults = map[string]any{}
	}
	return &model.WorkflowPreset{
		Name: in.Name, Description: strings.TrimSpace(in.Description), Kind: in.Kind,
		ModelID: strings.TrimSpace(in.ModelID), Prompt: in.Prompt, Variables: variables,
		Defaults: in.Defaults, Visibility: strings.TrimSpace(in.Visibility), Enabled: in.Enabled,
	}, nil
}
