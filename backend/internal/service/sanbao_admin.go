package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"backend/internal/model"
	"backend/internal/provider/sanbao"
	"backend/internal/repo"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type SanbaoAdminService struct {
	client *sanbao.Client
	tokens *repo.TokenRepository
	models *repo.ModelRepository
	events *repo.EventRepository
}

func NewSanbaoAdminService(client *sanbao.Client, tokens *repo.TokenRepository, models *repo.ModelRepository, events *repo.EventRepository) *SanbaoAdminService {
	return &SanbaoAdminService{client: client, tokens: tokens, models: models, events: events}
}

func (s *SanbaoAdminService) Accounts(ctx context.Context) ([]model.TokenAccount, map[string]int64, error) {
	items, err := s.tokens.ListByPool(ctx, "sanbao")
	if err != nil {
		return nil, nil, err
	}
	inflight, _ := s.events.InFlightByAccount(ctx)
	return items, inflight, nil
}

func (s *SanbaoAdminService) ImportKeys(ctx context.Context, raw string, weight, concurrency int) (int, []string, error) {
	seen := map[string]bool{}
	existing, _ := s.tokens.ListByPool(ctx, "sanbao")
	for _, item := range existing {
		seen[strings.TrimSpace(item.Value)] = true
	}
	imported := 0
	var failures []string
	for _, line := range strings.FieldsFunc(raw, func(r rune) bool { return r == '\n' || r == '\r' || r == ',' }) {
		key := strings.TrimSpace(line)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		if !strings.HasPrefix(key, "sk_sanbao_") {
			failures = append(failures, maskKey(key)+": 格式不正确")
			continue
		}
		me, err := s.client.Me(ctx, key)
		if err != nil {
			failures = append(failures, maskKey(key)+": "+err.Error())
			continue
		}
		id := "sanbao-" + randomUpper(14)
		credits := me.Credits
		if credits == 0 {
			credits = me.Balance
		}
		meta := datatypes.JSONMap{"credits": credits, "upstream_id": fmt.Sprint(me.ID), "checked_at": time.Now().Unix()}
		item := &model.TokenAccount{ID: id, Pool: "sanbao", Value: key, Status: "active", Meta: meta, AccountEmail: me.Email, AccountDisplayName: me.Name, Weight: weight, Concurrency: max(1, concurrency), CreatedAt: time.Now(), UpdatedAt: time.Now()}
		if item.AccountDisplayName == "" {
			item.AccountDisplayName = "三宝账号"
		}
		if err := s.tokens.Create(ctx, item); err != nil {
			failures = append(failures, maskKey(key)+": "+err.Error())
			continue
		}
		imported++
	}
	if imported == 0 && len(failures) == 0 {
		return 0, nil, errors.New("请粘贴至少一个 API Key")
	}
	return imported, failures, nil
}

func (s *SanbaoAdminService) TestAccount(ctx context.Context, id string) (*model.TokenAccount, error) {
	item, err := s.tokens.Get(ctx, "sanbao", id)
	if err != nil {
		return nil, err
	}
	me, err := s.client.Me(ctx, item.Value)
	if err != nil {
		patch := map[string]any{"last_used_at": time.Now(), "fails": gorm.Expr("fails + 1"), "fail_total": gorm.Expr("fail_total + 1")}
		if errors.Is(err, sanbao.ErrAuth) {
			patch["status"] = "disabled"
			patch["dead"] = true
		}
		_, _ = s.tokens.Update(ctx, "sanbao", id, patch)
		return nil, err
	}
	credits := me.Credits
	if credits == 0 {
		credits = me.Balance
	}
	meta := item.Meta
	if meta == nil {
		meta = datatypes.JSONMap{}
	}
	meta["credits"] = credits
	meta["checked_at"] = time.Now().Unix()
	meta["upstream_id"] = fmt.Sprint(me.ID)
	return s.tokens.Update(ctx, "sanbao", id, map[string]any{"status": "active", "dead": false, "fails": 0, "last_used_at": time.Now(), "account_email": me.Email, "account_display_name": me.Name, "meta": meta})
}

func (s *SanbaoAdminService) UpdateAccount(ctx context.Context, id string, patch map[string]any) (*model.TokenAccount, error) {
	allowed := map[string]any{}
	for _, key := range []string{"status", "weight", "concurrency"} {
		if v, ok := patch[key]; ok {
			allowed[key] = v
		}
	}
	if status, ok := allowed["status"].(string); ok && status == "active" {
		allowed["dead"] = false
	}
	return s.tokens.Update(ctx, "sanbao", id, allowed)
}

func (s *SanbaoAdminService) DeleteAccount(ctx context.Context, id string) error {
	_, err := s.tokens.Delete(ctx, "sanbao", id)
	return err
}

func (s *SanbaoAdminService) SyncModels(ctx context.Context) (int, error) {
	accounts, err := s.tokens.ListByPool(ctx, "sanbao")
	if err != nil {
		return 0, err
	}
	var key string
	for _, a := range accounts {
		if a.Status == "active" && !a.Dead && strings.TrimSpace(a.Value) != "" {
			key = a.Value
			break
		}
	}
	if key == "" {
		return 0, errors.New("没有可用的三宝 API Key")
	}
	upstream, err := s.client.Models(ctx, key)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, m := range upstream {
		if strings.TrimSpace(m.ID) == "" {
			continue
		}
		id := "sanbao:" + m.ID
		caps := datatypes.JSONMap{}
		b, _ := json.Marshal(m)
		_ = json.Unmarshal(b, &caps)
		ratios := m.Ratios
		if len(ratios) == 0 {
			ratios = []string{"16:9", "9:16", "1:1"}
		}
		resolutions := m.Resolutions
		if len(resolutions) == 0 {
			resolutions = []string{"720p"}
		}
		durations := make([]string, 0, len(m.Durations))
		for _, d := range m.Durations {
			durations = append(durations, fmt.Sprintf("%ds", d))
		}
		if len(durations) == 0 && m.DefaultDurationSeconds > 0 {
			durations = []string{fmt.Sprintf("%ds", m.DefaultDurationSeconds)}
		}
		if len(durations) == 0 {
			durations = []string{"5s"}
		}
		prices := datatypes.JSONMap{}
		for _, r := range resolutions {
			prices[r] = float64(0)
		}
		durationPrices := datatypes.JSONMap{"per_second": float64(0)}
		existing, getErr := s.models.Get(ctx, id)
		if getErr == nil && existing != nil {
			_, err = s.models.Update(ctx, id, map[string]any{"name": defaultString(m.Name, m.ID), "provider": "sanbao", "upstream_model": m.ID, "ratios": jsonValue(ratios), "resolutions": jsonValue(resolutions), "durations": jsonValue(durations), "max_reference_images": m.MaxImages, "reference_mode": "all", "capabilities": caps})
		} else {
			item := &model.ModelConfig{ID: id, Type: "video", Name: defaultString(m.Name, m.ID), Provider: "sanbao", Enabled: false, Ratios: jsonValue(ratios), Prices: prices, Resolutions: jsonValue(resolutions), DurationPrices: durationPrices, Durations: jsonValue(durations), MaxReferenceImages: m.MaxImages, ReferenceMode: "all", UpstreamModel: m.ID, Capabilities: caps, CreatedAt: time.Now(), UpdatedAt: time.Now()}
			err = s.models.Create(ctx, item)
		}
		if err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func (s *SanbaoAdminService) Models(ctx context.Context) ([]model.ModelConfig, error) {
	items, err := s.models.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]model.ModelConfig, 0)
	for _, m := range items {
		if m.Provider == "sanbao" && m.Type == "video" {
			out = append(out, m)
		}
	}
	return out, nil
}

func (s *SanbaoAdminService) UpdateModel(ctx context.Context, id string, body map[string]any) (*model.ModelConfig, error) {
	item, err := s.models.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if item.Provider != "sanbao" {
		return nil, errors.New("不是三宝视频模型")
	}
	patch := map[string]any{}
	for _, k := range []string{"name", "alias", "enabled", "weight", "prices", "duration_prices", "prices_agent", "duration_prices_agent"} {
		if v, ok := body[k]; ok {
			patch[k] = v
		}
	}
	return s.models.Update(ctx, item.ID, patch)
}

func jsonValue(v any) datatypes.JSON { b, _ := json.Marshal(v); return datatypes.JSON(b) }
func maskKey(v string) string {
	v = strings.TrimSpace(v)
	if len(v) <= 10 {
		return "***"
	}
	return v[:6] + "..." + v[len(v)-4:]
}
