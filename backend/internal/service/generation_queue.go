package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"backend/internal/model"
	"backend/internal/repo"
	"gorm.io/gorm"
)

const maxQueuedJobsPerUser = 20

type GenerationQueueService struct {
	jobs     *repo.GenerationJobRepository
	users    *repo.UserRepository
	models   *repo.ModelRepository
	gen      *UserGenerationService
	events   *GenerationEventService
	platform *APIPlatformService
	wake     chan struct{}
}

func NewGenerationQueueService(jobs *repo.GenerationJobRepository, users *repo.UserRepository, models *repo.ModelRepository, gen *UserGenerationService, events *GenerationEventService, platform *APIPlatformService) *GenerationQueueService {
	return &GenerationQueueService{jobs: jobs, users: users, models: models, gen: gen, events: events, platform: platform, wake: make(chan struct{}, 1)}
}

func (s *GenerationQueueService) SubmitImages(ctx context.Context, user *model.User, in UserGenerateRequest, count int) ([]map[string]any, error) {
	if user == nil || strings.TrimSpace(user.ID) == "" {
		return nil, errors.New("未登录或会话已过期")
	}
	if count < 1 || count > 4 {
		return nil, errors.New("单次生成数量必须为 1 到 4")
	}
	in.Model = strings.TrimSpace(in.Model)
	in.Prompt = strings.TrimSpace(in.Prompt)
	in.Ratio = strings.TrimSpace(in.Ratio)
	in.Resolution = strings.TrimSpace(in.Resolution)
	if in.Model == "" || in.Prompt == "" {
		return nil, errors.New("请选择模型并填写生成描述")
	}
	if len([]rune(in.Prompt)) > 2000 {
		return nil, errors.New("生成描述不能超过 2000 个字符")
	}

	item, err := s.models.Get(ctx, in.Model)
	if err != nil || !item.Enabled || item.Type != "image" {
		return nil, ErrUnknownModel
	}
	if err := s.gen.v1.checkBannedPrompt(ctx, &APIPrincipal{User: user, TokenType: "session"}, in.Prompt); err != nil {
		return nil, err
	}
	if supported := repo.JSONStrings(item.Ratios); len(supported) > 0 && !containsString(supported, in.Ratio) {
		return nil, fmt.Errorf("%w: aspect ratio %s", ErrUnsupportedParams, in.Ratio)
	}
	if _, ok := modelPrice(item, "image", in.Resolution, "", user.Role == "agent"); !ok {
		return nil, fmt.Errorf("%w: resolution %s", ErrUnsupportedParams, in.Resolution)
	}
	refLimit := 0
	if item.ImageToImage {
		refLimit = item.MaxReferenceImages
		if refLimit < 1 {
			refLimit = 1
		}
	}
	if len(in.ReferenceImages) > refLimit {
		return nil, errors.New("参考图数量超过当前模型限制")
	}
	for i, value := range in.ReferenceImages {
		in.ReferenceImages[i] = rawReferenceBase64(value)
	}
	if err := ensureReferenceSizes(in.ReferenceImages); err != nil {
		return nil, err
	}

	if in.DeAI && !s.gen.v1.deaiEnabled(ctx) {
		in.DeAI = false
	}
	price, _ := modelPrice(item, "image", in.Resolution, "", user.Role == "agent")
	if in.DeAI {
		price += s.gen.v1.deaiSurcharge(ctx, in.Resolution)
	}
	if user.Credits < price*float64(count) {
		return nil, ErrInsufficientFunds
	}
	payload, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	rows := make([]*model.GenerationJob, 0, count)
	for i := 0; i < count; i++ {
		rows = append(rows, &model.GenerationJob{
			ID:            "job-" + randomUpper(16),
			UserID:        user.ID,
			Kind:          "image",
			Status:        "queued",
			Stage:         "queued",
			Progress:      0,
			MaxAttempts:   3,
			Payload:       append([]byte(nil), payload...),
			EstimatedCost: price,
			AvailableAt:   now,
			CreatedAt:     now.Add(time.Duration(i) * time.Microsecond),
			UpdatedAt:     now,
		})
	}
	if err := s.jobs.CreateBatchWithinLimit(ctx, user.ID, rows, maxQueuedJobsPerUser); err != nil {
		if errors.Is(err, repo.ErrGenerationQueueCapacity) {
			return nil, errors.New("等待中的任务过多，请稍后再提交")
		}
		return nil, err
	}
	s.signal()
	out := make([]map[string]any, 0, len(rows))
	for i, row := range rows {
		shaped := shapeGenerationJob(row, int64(i+1))
		out = append(out, shaped)
		s.events.Publish(ctx, user.ID, map[string]any{"event": "job.created", "job": shaped})
	}
	return out, nil
}

func (s *GenerationQueueService) GetOwned(ctx context.Context, userID, id string) (map[string]any, error) {
	item, err := s.jobs.GetOwned(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	return shapeGenerationJob(item, s.jobs.QueuePosition(ctx, item)), nil
}

func (s *GenerationQueueService) ActiveOwned(ctx context.Context, userID string) ([]map[string]any, error) {
	items, err := s.jobs.ListActiveOwned(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(items))
	for i := range items {
		position := int64(0)
		if items[i].Status == "queued" || items[i].Status == "retry_wait" {
			position = s.jobs.QueuePosition(ctx, &items[i])
		}
		out = append(out, shapeGenerationJob(&items[i], position))
	}
	return out, nil
}

func (s *GenerationQueueService) CancelOwned(ctx context.Context, userID, id string) error {
	ok, err := s.jobs.CancelOwned(ctx, id, userID)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("任务已开始执行，当前不能取消")
	}
	if item, getErr := s.jobs.GetOwned(ctx, id, userID); getErr == nil {
		s.events.Publish(ctx, userID, map[string]any{"event": "job.updated", "job": shapeGenerationJob(item, 0)})
	}
	return nil
}

func (s *GenerationQueueService) Run(ctx context.Context, workers int) {
	if workers < 1 {
		workers = 1
	}
	if n, err := s.jobs.RequeueStale(ctx, time.Now().Add(-15*time.Minute)); err != nil {
		log.Printf("generation queue: requeue stale: %v", err)
	} else if n > 0 {
		log.Printf("generation queue: recovered %d stale job(s)", n)
	}

	work := make(chan *model.GenerationJob)
	slots := make(chan struct{}, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			for item := range work {
				s.process(context.Background(), item)
				<-slots
			}
		}(i)
	}

	workerID := "gen-" + randomUpper(12)
	cleanup := time.NewTicker(time.Hour)
	recovery := time.NewTicker(time.Minute)
	defer cleanup.Stop()
	defer recovery.Stop()
	defer func() {
		close(work)
		wg.Wait()
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case <-recovery.C:
			if n, err := s.jobs.RequeueStale(ctx, time.Now().Add(-15*time.Minute)); err != nil {
				log.Printf("generation queue: recover stale: %v", err)
			} else if n > 0 {
				log.Printf("generation queue: recovered %d stale job(s)", n)
			}
			continue
		case slots <- struct{}{}:
		}
		item, err := s.jobs.ClaimNext(ctx, workerID)
		if err != nil {
			<-slots
			log.Printf("generation queue: claim: %v", err)
			if !waitQueue(ctx, s.wake, time.Second) {
				return
			}
			continue
		}
		if item == nil {
			<-slots
			select {
			case <-ctx.Done():
				return
			case <-s.wake:
			case <-time.After(time.Second):
			case <-cleanup.C:
				if err := s.jobs.DeleteFinishedBefore(ctx, time.Now().Add(-7*24*time.Hour)); err != nil {
					log.Printf("generation queue: cleanup: %v", err)
				}
			}
			continue
		}
		select {
		case <-ctx.Done():
			<-slots
			return
		case work <- item:
			s.events.Publish(ctx, item.UserID, map[string]any{"event": "job.updated", "job": shapeGenerationJob(item, 0)})
		}
	}
}

func (s *GenerationQueueService) process(ctx context.Context, item *model.GenerationJob) {
	defer func() {
		if value := recover(); value != nil {
			s.handleFailure(context.Background(), item, "worker_panic", true, "任务执行异常，请稍后重试")
			log.Printf("generation queue: panic job=%s value=%v", item.ID, value)
		}
	}()
	user, err := s.users.GetByID(ctx, item.UserID)
	if err != nil || user.Status != "active" {
		s.handleFailure(ctx, item, "user_unavailable", false, "账号不可用，任务未扣费")
		return
	}
	var request UserGenerateRequest
	if err := json.Unmarshal(item.Payload, &request); err != nil {
		s.handleFailure(ctx, item, "invalid_payload", false, "任务参数损坏，任务未扣费")
		return
	}
	_ = s.jobs.SetProgress(ctx, item.ID, "submitting", 25)
	item.Stage, item.Progress = "submitting", 25
	s.events.Publish(ctx, item.UserID, map[string]any{"event": "job.updated", "job": shapeGenerationJob(item, 0)})
	result, err := s.gen.Generate(ctx, user, request)
	if err != nil {
		code, retryable, message := classifyQueueError(err)
		s.handleFailure(ctx, item, code, retryable, message)
		return
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		s.handleFailure(ctx, item, "result_encoding", false, "生成完成但结果解析失败")
		return
	}
	if err := s.jobs.Complete(ctx, item.ID, encoded); err != nil {
		log.Printf("generation queue: complete job=%s: %v", item.ID, err)
	}
	item.Status, item.Stage, item.Progress = "completed", "completed", 100
	item.Result = encoded
	now := time.Now()
	item.FinishedAt = &now
	s.events.Publish(ctx, item.UserID, map[string]any{"event": "job.completed", "job": shapeGenerationJob(item, 0)})
	if s.platform != nil {
		s.platform.EmitWebhook(ctx, item.UserID, "generation.completed", item.ID, webhookJobPayload(item))
	}
}

func (s *GenerationQueueService) handleFailure(ctx context.Context, item *model.GenerationJob, code string, retryable bool, message string) {
	if retryable && item.Attempts < item.MaxAttempts {
		delay := time.Duration(1<<max(0, item.Attempts-1)) * 5 * time.Second
		available := time.Now().Add(delay)
		_ = s.jobs.Retry(ctx, item.ID, code, message, available)
		item.Status, item.Stage, item.Progress = "retry_wait", "retry_wait", 0
		item.Error, item.LastError, item.ErrorCode, item.Retryable = message, message, code, true
		item.AvailableAt = available
		s.events.Publish(ctx, item.UserID, map[string]any{"event": "job.retrying", "job": shapeGenerationJob(item, s.jobs.QueuePosition(ctx, item))})
		s.signal()
		return
	}
	if retryable {
		_ = s.jobs.DeadLetter(ctx, item.ID, code, message)
		item.Status, item.Stage, item.Progress = "dead_letter", "dead_letter", 100
		item.Error, item.LastError, item.ErrorCode, item.Retryable = message, message, code, false
		s.events.Publish(ctx, item.UserID, map[string]any{"event": "job.dead_letter", "job": shapeGenerationJob(item, 0)})
		if s.platform != nil {
			s.platform.EmitWebhook(ctx, item.UserID, "generation.dead_letter", item.ID, webhookJobPayload(item))
			s.platform.EmitWebhook(ctx, item.UserID, "billing.refunded", item.ID, webhookRefundPayload(item))
		}
		return
	}
	_ = s.jobs.Fail(ctx, item.ID, message)
	item.Status, item.Stage, item.Progress = "failed", "failed", 100
	item.Error, item.ErrorCode = message, code
	s.events.Publish(ctx, item.UserID, map[string]any{"event": "job.failed", "job": shapeGenerationJob(item, 0)})
	if s.platform != nil {
		s.platform.EmitWebhook(ctx, item.UserID, "generation.failed", item.ID, webhookJobPayload(item))
		s.platform.EmitWebhook(ctx, item.UserID, "billing.refunded", item.ID, webhookRefundPayload(item))
	}
}

func webhookJobPayload(item *model.GenerationJob) map[string]any {
	return map[string]any{
		"id": item.ID, "kind": item.Kind, "status": item.Status, "stage": item.Stage,
		"error": emptyOrNil(item.Error), "error_code": emptyOrNil(item.ErrorCode),
		"attempts": item.Attempts, "estimated_cost": item.EstimatedCost,
		"created_at": item.CreatedAt, "finished_at": item.FinishedAt,
	}
}

func webhookRefundPayload(item *model.GenerationJob) map[string]any {
	return map[string]any{
		"generation_id": item.ID, "amount": item.EstimatedCost,
		"reason": emptyOrNil(item.Error), "created_at": time.Now(),
	}
}

func (s *GenerationQueueService) DeadJobs(ctx context.Context, limit, offset int) ([]map[string]any, int64, error) {
	items, total, err := s.jobs.ListDead(ctx, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	out := make([]map[string]any, 0, len(items))
	for i := range items {
		out = append(out, shapeGenerationJob(&items[i], 0))
	}
	return out, total, nil
}

func (s *GenerationQueueService) RetryDead(ctx context.Context, id string) error {
	ok, err := s.jobs.RetryDead(ctx, id)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("死信任务不存在或已被处理")
	}
	s.signal()
	return nil
}

func (s *GenerationQueueService) signal() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func waitQueue(ctx context.Context, wake <-chan struct{}, delay time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-wake:
		return true
	case <-time.After(delay):
		return true
	}
}

func shapeGenerationJob(item *model.GenerationJob, position int64) map[string]any {
	var result any
	if len(item.Result) > 0 {
		_ = json.Unmarshal(item.Result, &result)
	}
	progress := item.Progress
	if item.Status == "completed" && progress < 100 {
		progress = 100
	}
	return map[string]any{
		"id":             item.ID,
		"kind":           item.Kind,
		"status":         item.Status,
		"stage":          item.Stage,
		"progress":       progress,
		"position":       position,
		"estimated_cost": item.EstimatedCost,
		"result":         result,
		"error":          emptyOrNil(item.Error),
		"error_code":     emptyOrNil(item.ErrorCode),
		"last_error":     emptyOrNil(item.LastError),
		"retryable":      item.Retryable,
		"attempts":       item.Attempts,
		"max_attempts":   item.MaxAttempts,
		"next_retry_at":  item.AvailableAt,
		"created_at":     item.CreatedAt,
		"started_at":     item.StartedAt,
		"finished_at":    item.FinishedAt,
	}
}

func rawReferenceBase64(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "data:") {
		if comma := strings.IndexByte(value, ','); comma >= 0 {
			return value[comma+1:]
		}
	}
	return value
}

func friendlyQueueError(err error) string {
	switch {
	case errors.Is(err, ErrInsufficientFunds):
		return "可用额度不足，任务未扣费"
	case errors.Is(err, ErrConcurrencyFull), errors.Is(err, ErrUserConcurrencyFull):
		return "当前并发已满，请稍后重新提交"
	case errors.Is(err, ErrNoProviderAccount):
		return "当前没有可用生成账号，任务未扣费"
	case errors.Is(err, ErrUnknownModel), errors.Is(err, gorm.ErrRecordNotFound):
		return "模型已停用或不存在，任务未扣费"
	default:
		message := strings.TrimSpace(err.Error())
		if message == "" {
			return "生成失败，已自动处理退款"
		}
		return message
	}
}

func classifyQueueError(err error) (string, bool, string) {
	message := friendlyQueueError(err)
	switch {
	case errors.Is(err, ErrConcurrencyFull), errors.Is(err, ErrProviderExecution):
		return "provider_busy", true, message
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return "upstream_timeout", true, "上游响应超时，系统将自动重试"
	case errors.Is(err, ErrNoProviderAccount):
		return "no_provider_account", true, message
	case errors.Is(err, ErrInsufficientFunds):
		return "insufficient_funds", false, message
	case errors.Is(err, ErrUnknownModel), errors.Is(err, gorm.ErrRecordNotFound):
		return "model_unavailable", false, message
	case errors.Is(err, ErrBannedPrompt), errors.Is(err, ErrUnsupportedParams):
		return "invalid_request", false, message
	default:
		lower := strings.ToLower(err.Error())
		if strings.Contains(lower, "timeout") || strings.Contains(lower, "temporar") || strings.Contains(lower, "overload") || strings.Contains(lower, "rate limit") || strings.Contains(lower, "429") || strings.Contains(lower, "502") || strings.Contains(lower, "503") {
			return "temporary_upstream", true, message
		}
		return "generation_failed", false, message
	}
}
