package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"backend/internal/model"
	"backend/internal/service"
	"github.com/gin-gonic/gin"
)

type SanbaoHandler struct {
	svc *service.SanbaoAdminService
	v1  *service.V1Service
}

func NewSanbaoHandler(svc *service.SanbaoAdminService, v1 *service.V1Service) *SanbaoHandler {
	return &SanbaoHandler{svc: svc, v1: v1}
}

func (h *SanbaoHandler) Accounts(c *gin.Context) {
	items, inflight, err := h.svc.Accounts(c.Request.Context())
	if err != nil {
		c.JSON(500, gin.H{"detail": err.Error()})
		return
	}
	out := make([]gin.H, 0, len(items))
	for _, x := range items {
		out = append(out, gin.H{"id": x.ID, "name": x.AccountDisplayName, "email": x.AccountEmail, "key_preview": maskSecret(x.Value), "status": x.Status, "dead": x.Dead, "weight": x.Weight, "concurrency": x.Concurrency, "credits": x.Meta["credits"], "success_total": x.SuccessTotal, "fail_total": x.FailTotal, "inflight": inflight[x.ID], "last_used_at": x.LastUsedAt, "updated_at": x.UpdatedAt})
	}
	c.JSON(200, gin.H{"data": out})
}
func (h *SanbaoHandler) Import(c *gin.Context) {
	var b struct {
		Keys        string `json:"keys"`
		Weight      int    `json:"weight"`
		Concurrency int    `json:"concurrency"`
	}
	if c.ShouldBindJSON(&b) != nil {
		c.JSON(400, gin.H{"detail": "invalid request body"})
		return
	}
	n, fail, err := h.svc.ImportKeys(c.Request.Context(), b.Keys, b.Weight, b.Concurrency)
	if err != nil {
		c.JSON(400, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true, "imported": n, "failures": fail})
}
func (h *SanbaoHandler) Test(c *gin.Context) {
	x, err := h.svc.TestAccount(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true, "credits": x.Meta["credits"]})
}
func (h *SanbaoHandler) UpdateAccount(c *gin.Context) {
	var b map[string]any
	if c.ShouldBindJSON(&b) != nil {
		c.JSON(400, gin.H{"detail": "invalid request body"})
		return
	}
	x, err := h.svc.UpdateAccount(c.Request.Context(), c.Param("id"), b)
	if err != nil {
		c.JSON(400, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true, "id": x.ID})
}
func (h *SanbaoHandler) Delete(c *gin.Context) {
	if err := h.svc.DeleteAccount(c.Request.Context(), c.Param("id")); err != nil {
		c.JSON(400, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
func (h *SanbaoHandler) Sync(c *gin.Context) {
	n, err := h.svc.SyncModels(c.Request.Context())
	if err != nil {
		c.JSON(400, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true, "synced": n})
}
func (h *SanbaoHandler) Models(c *gin.Context) {
	items, err := h.svc.Models(c.Request.Context())
	if err != nil {
		c.JSON(500, gin.H{"detail": err.Error()})
		return
	}
	out := make([]gin.H, 0, len(items))
	for _, m := range items {
		out = append(out, videoModelJSON(m))
	}
	c.JSON(200, gin.H{"data": out})
}
func (h *SanbaoHandler) UpdateModel(c *gin.Context) {
	var b map[string]any
	if c.ShouldBindJSON(&b) != nil {
		c.JSON(400, gin.H{"detail": "invalid request body"})
		return
	}
	m, err := h.svc.UpdateModel(c.Request.Context(), c.Param("id"), b)
	if err != nil {
		c.JSON(400, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(200, videoModelJSON(*m))
}

func (h *SanbaoHandler) CreateJob(c *gin.Context) {
	u := currentUser(c)
	if u == nil {
		c.JSON(401, gin.H{"detail": "未登录或会话已过期"})
		return
	}
	var b struct {
		Model           string   `json:"model"`
		Prompt          string   `json:"prompt"`
		Ratio           string   `json:"ratio"`
		Resolution      string   `json:"resolution"`
		Duration        string   `json:"duration"`
		ReferenceMode   string   `json:"reference_mode"`
		ReferenceImages []string `json:"reference_images"`
		ReferenceVideos []string `json:"reference_videos"`
		ReferenceAudios []string `json:"reference_audios"`
		Concurrency     int      `json:"concurrency"`
	}
	if c.ShouldBindJSON(&b) != nil {
		c.JSON(400, gin.H{"detail": "invalid request body"})
		return
	}
	resp, err := h.v1.StartSessionVideoJob(c.Request.Context(), u, service.V1VideoRequest{Model: b.Model, Prompt: b.Prompt, AspectRatio: b.Ratio, Resolution: b.Resolution, Duration: b.Duration, ReferenceMode: b.ReferenceMode, ReferenceImages: b.ReferenceImages, ReferenceVideos: b.ReferenceVideos, ReferenceAudios: b.ReferenceAudios, Concurrency: b.Concurrency, BaseURL: requestBaseURL(c)})
	if err != nil {
		c.JSON(videoHTTPStatus(err), gin.H{"detail": err.Error()})
		return
	}
	c.JSON(200, resp)
}
func (h *SanbaoHandler) Job(c *gin.Context) {
	u := currentUser(c)
	resp, err := h.v1.VideoJob(c.Request.Context(), &service.APIPrincipal{User: u, TokenType: "session"}, c.Param("id"))
	if err != nil {
		c.JSON(404, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(200, resp)
}
func (h *SanbaoHandler) Content(c *gin.Context) {
	u := currentUser(c)
	body, ct, err := h.v1.OpenVideoContent(c.Request.Context(), &service.APIPrincipal{User: u, TokenType: "session"}, c.Param("id"))
	if err != nil {
		c.JSON(404, gin.H{"detail": err.Error()})
		return
	}
	defer body.Close()
	c.Header("Content-Type", ct)
	c.Status(200)
	_, _ = io.Copy(c.Writer, body)
}
func (h *SanbaoHandler) DeleteJob(c *gin.Context) {
	u := currentUser(c)
	if err := h.v1.DeleteVideoJob(c.Request.Context(), &service.APIPrincipal{User: u, TokenType: "session"}, c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func videoModelJSON(m model.ModelConfig) gin.H {
	return gin.H{"id": m.ID, "name": m.Name, "alias": m.Alias, "provider": m.Provider, "enabled": m.Enabled, "weight": m.Weight, "ratios": decodeStringArray(m.Ratios), "resolutions": decodeStringArray(m.Resolutions), "durations": decodeStringArray(m.Durations), "prices": m.Prices, "duration_prices": m.DurationPrices, "prices_agent": m.PricesAgent, "duration_prices_agent": m.DurationPricesAgent, "max_reference_images": m.MaxReferenceImages, "capabilities": m.Capabilities, "upstream_model": m.UpstreamModel, "generation_count": m.GenerationCount}
}
func maskSecret(v string) string {
	v = strings.TrimSpace(v)
	if len(v) < 12 {
		return "******"
	}
	return v[:6] + "..." + v[len(v)-4:]
}
func videoHTTPStatus(err error) int {
	switch {
	case errors.Is(err, service.ErrInsufficientFunds):
		return http.StatusPaymentRequired
	case errors.Is(err, service.ErrConcurrencyFull), errors.Is(err, service.ErrUserConcurrencyFull):
		return http.StatusTooManyRequests
	case errors.Is(err, service.ErrNoProviderAccount), errors.Is(err, service.ErrProviderTemporary):
		return http.StatusServiceUnavailable
	default:
		return http.StatusBadRequest
	}
}
func decodeStringArray(v []byte) []string { var out []string; _ = json.Unmarshal(v, &out); return out }
