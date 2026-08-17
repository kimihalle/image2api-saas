package handler

import (
	"net/http"

	"backend/internal/service"
	"github.com/gin-gonic/gin"
)

type OperationsHandler struct {
	ops    *service.OperationsService
	backup *service.BackupService
}

func NewOperationsHandler(ops *service.OperationsService, backup *service.BackupService) *OperationsHandler {
	return &OperationsHandler{ops: ops, backup: backup}
}

func (h *OperationsHandler) Snapshot(c *gin.Context) {
	data, err := h.ops.Snapshot(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "读取运营状态失败"})
		return
	}
	c.JSON(http.StatusOK, data)
}

func (h *OperationsHandler) ReliabilitySettings(c *gin.Context) {
	data, err := h.ops.ReliabilitySettings(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "读取保障配置失败"})
		return
	}
	c.JSON(http.StatusOK, data)
}

func (h *OperationsHandler) SaveReliabilitySettings(c *gin.Context) {
	var body service.ReliabilitySettings
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "请求参数格式错误"})
		return
	}
	if err := h.ops.SaveReliabilitySettings(c.Request.Context(), body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *OperationsHandler) ResolveAlert(c *gin.Context) {
	if err := h.ops.ResolveAlert(c.Request.Context(), c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *OperationsHandler) RunReconciliation(c *gin.Context) {
	item, err := h.ops.RunReconciliation(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error(), "data": item})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item})
}

func (h *OperationsHandler) ReconciliationIssues(c *gin.Context) {
	items, err := h.ops.ReconciliationIssues(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "读取对账差异失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *OperationsHandler) RunBackup(c *gin.Context) {
	item, err := h.backup.RunNow(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error(), "data": item})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item})
}
