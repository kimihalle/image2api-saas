package handler

import (
	"net/http"
	"strings"
	"unicode/utf8"

	"backend/internal/service"
	"github.com/gin-gonic/gin"
)

type AnnouncementHandler struct {
	svc *service.AnnouncementService
}

func NewAnnouncementHandler(svc *service.AnnouncementService) *AnnouncementHandler {
	return &AnnouncementHandler{svc: svc}
}

// Get — logged-in user: the current announcement + whether THIS user has seen it.
func (h *AnnouncementHandler) Get(c *gin.Context) {
	user := currentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "未登录或会话已过期"})
		return
	}
	c.JSON(http.StatusOK, h.svc.ForUser(c.Request.Context(), user))
}

// MarkSeen — user dismissed the announcement of the given version.
func (h *AnnouncementHandler) MarkSeen(c *gin.Context) {
	user := currentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "未登录或会话已过期"})
		return
	}
	var body struct {
		Version string `json:"version"`
	}
	_ = c.ShouldBindJSON(&body)
	if err := h.svc.MarkSeen(c.Request.Context(), user.ID, body.Version); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to save"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// AdminGet — admin editor: the raw markdown.
func (h *AnnouncementHandler) AdminGet(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"content": h.svc.Content(c.Request.Context())})
}

// AdminPut — admin saves new markdown (re-pops for everyone who hasn't seen it).
func (h *AnnouncementHandler) AdminPut(c *gin.Context) {
	var body struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid request body"})
		return
	}
	if err := h.svc.Save(c.Request.Context(), body.Content); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to save"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *AnnouncementHandler) Ticker(c *gin.Context) {
	c.JSON(http.StatusOK, h.svc.PublicTicker(c.Request.Context()))
}

func (h *AnnouncementHandler) AdminTickerGet(c *gin.Context) {
	c.JSON(http.StatusOK, h.svc.Ticker(c.Request.Context()))
}

func (h *AnnouncementHandler) AdminTickerPut(c *gin.Context) {
	var body service.TickerConfig
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid request body"})
		return
	}
	body.Content = strings.TrimSpace(body.Content)
	if utf8.RuneCountInString(body.Content) > 500 {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "滚动公告最多 500 个字符"})
		return
	}
	if err := h.svc.SaveTicker(c.Request.Context(), body); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to save"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": h.svc.Ticker(c.Request.Context())})
}
