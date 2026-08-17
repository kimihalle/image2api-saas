package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"backend/internal/model"
	"backend/internal/provider/sanbao"
)

func capabilityInt(m map[string]any, key string) int {
	if m == nil {
		return 0
	}
	switch v := m[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		n, _ := v.Int64()
		return int(n)
	}
	return 0
}

func (s *V1Service) generateSanbaoVideo(ctx context.Context, principal *APIPrincipal, eventID string, modelItem *model.ModelConfig, in V1VideoRequest, ratio, resolution string, duration int) ([]byte, string, error) {
	if s.sanbao == nil {
		return nil, "", errors.New("sanbao client not configured")
	}
	items, err := s.tokens.ListByPool(ctx, "sanbao")
	if err != nil {
		return nil, "", err
	}
	active := make([]model.TokenAccount, 0, len(items))
	for _, item := range items {
		if item.Status == "active" && !item.Dead && strings.TrimSpace(item.Value) != "" {
			active = append(active, item)
		}
	}
	s.rotateRoundRobin("sanbao", active)
	if len(active) == 0 {
		return nil, "", ErrNoProviderAccount
	}

	upstreamModel := strings.TrimSpace(modelItem.UpstreamModel)
	if upstreamModel == "" {
		upstreamModel = strings.TrimPrefix(modelItem.ID, "sanbao:")
	}
	request := sanbao.CreateRequest{Model: upstreamModel, Prompt: in.Prompt, Ratio: ratio, Resolution: resolution, Duration: duration, Concurrency: 1, Reference: strings.TrimSpace(in.ReferenceMode)}
	if request.Reference == "" {
		request.Reference = "all"
	}
	var upstreamCost float64
	data, err := s.runPoolWithFailover(ctx, eventID, "sanbao", active, "video", func(token model.TokenAccount) ([]byte, error) {
		images, err := s.sanbaoMedia(ctx, token.Value, "image", in.ReferenceImages)
		if err != nil {
			return nil, err
		}
		videos, err := s.sanbaoMedia(ctx, token.Value, "video", in.ReferenceVideos)
		if err != nil {
			return nil, err
		}
		audios, err := s.sanbaoMedia(ctx, token.Value, "audio", in.ReferenceAudios)
		if err != nil {
			return nil, err
		}
		request.Images, request.Videos, request.Audios = images, videos, audios
		task, err := s.sanbao.CreateVideo(ctx, token.Value, request)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(task.ID) == "" {
			return nil, errors.New("sanbao returned no task id")
		}
		meta := map[string]any{"model": upstreamModel, "ratio": ratio, "resolution": resolution, "duration": duration, "reference": request.Reference}
		next := time.Now().Add(4 * time.Second)
		_ = s.events.UpdateAsyncVideo(context.WithoutCancel(ctx), eventID, task.ID, max(0, task.Progress), task.Cost, meta, &next)
		for {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(4 * time.Second):
			}
			task, err = s.sanbao.Video(ctx, token.Value, task.ID)
			if err != nil {
				return nil, err
			}
			upstreamCost = task.Cost
			next = time.Now().Add(4 * time.Second)
			_ = s.events.UpdateAsyncVideo(context.WithoutCancel(ctx), eventID, task.ID, max(0, task.Progress), task.Cost, nil, &next)
			switch strings.ToLower(task.Status) {
			case "succeeded", "success", "completed":
				u := strings.TrimSpace(task.DownloadURL)
				if u == "" {
					u = strings.TrimSpace(task.VideoURL)
				}
				if u == "" {
					return nil, errors.New("sanbao task succeeded without video url")
				}
				b, _, err := s.sanbao.Download(ctx, u)
				return b, err
			case "failed", "error", "cancelled":
				return nil, fmt.Errorf("sanbao generation failed: %v", task.Error)
			}
		}
	}, func(err error) (bool, bool, bool, bool) {
		return errors.Is(err, sanbao.ErrAuth), errors.Is(err, sanbao.ErrQuotaExhausted), errors.Is(err, sanbao.ErrTemporaryUpstream), false
	}, nil, true)
	if err != nil {
		return nil, "", err
	}
	_, key := s.allocateOutput(principal, "mp4", in.BaseURL)
	if err := s.store.Put(ctx, key, data, "video/mp4"); err != nil {
		return nil, "", fmt.Errorf("store sanbao video: %w", err)
	}
	_ = s.events.UpdateAsyncVideo(context.WithoutCancel(ctx), eventID, "", 100, upstreamCost, nil, nil)
	return data, key, nil
}

func (s *V1Service) sanbaoMedia(ctx context.Context, key, kind string, inputs []string) ([]sanbao.Media, error) {
	out := make([]sanbao.Media, 0, len(inputs))
	for i, raw := range inputs {
		v := strings.TrimSpace(raw)
		if v == "" {
			continue
		}
		if strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://") {
			out = append(out, sanbao.Media{Tag: fmt.Sprintf("%s%d", kind, i+1), URL: v})
			continue
		}
		ct := "application/octet-stream"
		if p := strings.Index(v, ","); strings.HasPrefix(v, "data:") && p > 0 {
			header := v[5:p]
			v = v[p+1:]
			if semi := strings.Index(header, ";"); semi >= 0 {
				ct = header[:semi]
			} else {
				ct = header
			}
		}
		b, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			b, err = base64.RawStdEncoding.DecodeString(v)
		}
		if err != nil {
			return nil, fmt.Errorf("invalid %s reference %d", kind, i+1)
		}
		ext := ".bin"
		if kind == "image" {
			ext = ".png"
		} else if kind == "video" {
			ext = ".mp4"
		} else if kind == "audio" {
			ext = ".mp3"
		}
		u, err := s.sanbao.Upload(ctx, key, kind, fmt.Sprintf("reference-%d%s", i+1, ext), ct, b)
		if err != nil {
			return nil, err
		}
		out = append(out, sanbao.Media{Tag: fmt.Sprintf("%s%d", kind, i+1), URL: u})
	}
	return out, nil
}

// ResumeSanbaoJobs reattaches polling to jobs that already have an upstream id.
// It is called during bootstrap before the stale-job maintenance sweep starts.
func (s *V1Service) ResumeSanbaoJobs(ctx context.Context) {
	items, err := s.events.PendingProviderVideos(ctx, "sanbao")
	if err != nil {
		return
	}
	for i := range items {
		ev := items[i]
		next := time.Now().Add(4 * time.Second)
		_ = s.events.UpdateAsyncVideo(ctx, ev.ID, "", max(0, ev.Progress), 0, nil, &next)
		go s.resumeSanbaoJob(context.Background(), ev)
	}
}

func (s *V1Service) resumeSanbaoJob(parent context.Context, ev model.EventLog) {
	ctx, cancel := context.WithTimeout(parent, videoGenBudget)
	defer cancel()
	acct, err := s.tokens.Get(ctx, "sanbao", ev.AccountID)
	user, uerr := s.users.GetByID(ctx, ev.UserID)
	principal := &APIPrincipal{User: user, TokenType: "resume", CreditTransactionID: ev.CreditTransactionID}
	if err != nil || acct == nil || strings.TrimSpace(acct.Value) == "" || uerr != nil {
		_ = s.refundIfNeeded(context.Background(), principal, ev.ID, ev.Cost)
		_ = s.events.UpdateStatus(context.Background(), ev.ID, "failed", "任务恢复失败：原三宝账号或用户不存在", 0)
		return
	}
	started := time.Now()
	for {
		select {
		case <-ctx.Done():
			_ = s.refundIfNeeded(context.Background(), principal, ev.ID, ev.Cost)
			_ = s.events.UpdateStatus(context.Background(), ev.ID, "failed", "任务恢复轮询超时", 0)
			return
		case <-time.After(4 * time.Second):
		}
		task, pollErr := s.sanbao.Video(ctx, acct.Value, ev.UpstreamTaskID)
		if pollErr != nil {
			if errors.Is(pollErr, sanbao.ErrTemporaryUpstream) {
				continue
			}
			_ = s.refundIfNeeded(context.Background(), principal, ev.ID, ev.Cost)
			_ = s.events.UpdateStatus(context.Background(), ev.ID, "failed", pollErr.Error(), 0)
			return
		}
		next := time.Now().Add(4 * time.Second)
		_ = s.events.UpdateAsyncVideo(context.Background(), ev.ID, task.ID, max(0, task.Progress), task.Cost, nil, &next)
		switch strings.ToLower(task.Status) {
		case "failed", "error", "cancelled":
			_ = s.refundIfNeeded(context.Background(), principal, ev.ID, ev.Cost)
			_ = s.events.UpdateStatus(context.Background(), ev.ID, "failed", fmt.Sprintf("sanbao generation failed: %v", task.Error), 0)
			return
		case "succeeded", "success", "completed":
			u := strings.TrimSpace(task.DownloadURL)
			if u == "" {
				u = strings.TrimSpace(task.VideoURL)
			}
			b, _, downloadErr := s.sanbao.Download(ctx, u)
			if downloadErr != nil {
				_ = s.refundIfNeeded(context.Background(), principal, ev.ID, ev.Cost)
				_ = s.events.UpdateStatus(context.Background(), ev.ID, "failed", "视频转存失败: "+downloadErr.Error(), 0)
				return
			}
			_, key := s.allocateOutput(principal, "mp4", "")
			if putErr := s.store.Put(ctx, key, b, "video/mp4"); putErr != nil {
				_ = s.refundIfNeeded(context.Background(), principal, ev.ID, ev.Cost)
				_ = s.events.UpdateStatus(context.Background(), ev.ID, "failed", "视频转存失败: "+putErr.Error(), 0)
				return
			}
			_ = s.events.MarkVideoReady(context.Background(), ev.ID, key, int(time.Since(started).Milliseconds()))
			if s.credits != nil {
				_, _ = s.credits.Capture(context.Background(), ev.CreditTransactionID)
			}
			_ = s.models.IncrementGenerationCount(context.Background(), ev.Model)
			_ = s.users.IncrementGenerationCount(context.Background(), ev.UserID)
			return
		}
	}
}
