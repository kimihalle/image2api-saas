package handler

import (
	"errors"
	"net/http"

	"backend/internal/service"
	"github.com/gin-gonic/gin"
)

type ConcurrencyGroupHandler struct {
	svc *service.ConcurrencyGroupService
}

func NewConcurrencyGroupHandler(svc *service.ConcurrencyGroupService) *ConcurrencyGroupHandler {
	return &ConcurrencyGroupHandler{svc: svc}
}

func (h *ConcurrencyGroupHandler) List(c *gin.Context) {
	items, err := h.svc.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to load concurrency groups"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *ConcurrencyGroupHandler) Create(c *gin.Context) {
	var body struct {
		Name           string `json:"name"`
		MaxConcurrency int    `json:"max_concurrency"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid request body"})
		return
	}
	g, err := h.svc.Create(c.Request.Context(), body.Name, body.MaxConcurrency)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "id": g.ID})
}

func (h *ConcurrencyGroupHandler) Update(c *gin.Context) {
	var body struct {
		Name           *string `json:"name"`
		MaxConcurrency *int    `json:"max_concurrency"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid request body"})
		return
	}
	g, err := h.svc.Update(c.Request.Context(), c.Param("id"), body.Name, body.MaxConcurrency)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "id": g.ID})
}

func (h *ConcurrencyGroupHandler) SetDefault(c *gin.Context) {
	if err := h.svc.SetDefault(c.Request.Context(), c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *ConcurrencyGroupHandler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), c.Param("id")); err != nil {
		if errors.Is(err, service.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"detail": "分组不存在"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
