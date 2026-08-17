package handler

import (
	"net/http"
	"strings"

	"backend/internal/service"
	"github.com/gin-gonic/gin"
)

type OperationsHandler struct {
	ops      *service.OperationsService
	backup   *service.BackupService
	platform *service.APIPlatformService
}

func NewOperationsHandler(ops *service.OperationsService, backup *service.BackupService, platform *service.APIPlatformService) *OperationsHandler {
	return &OperationsHandler{ops: ops, backup: backup, platform: platform}
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

func (h *OperationsHandler) AdminAPIUsage(c *gin.Context) {
	data, err := h.platform.Usage(c.Request.Context(), "", c.DefaultQuery("range", "7d"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "读取 API 数据失败"})
		return
	}
	c.JSON(http.StatusOK, data)
}

func (h *OperationsHandler) MyAPIUsage(c *gin.Context) {
	user := currentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "未登录或会话已过期"})
		return
	}
	data, err := h.platform.Usage(c.Request.Context(), user.ID, c.DefaultQuery("range", "7d"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "读取 API 数据失败"})
		return
	}
	c.JSON(http.StatusOK, data)
}

func (h *OperationsHandler) WebhookEndpoints(c *gin.Context) {
	user := currentUser(c)
	items, err := h.platform.WebhookEndpoints(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "读取 Webhook 失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *OperationsHandler) CreateWebhook(c *gin.Context) {
	user := currentUser(c)
	var body service.WebhookEndpointInput
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "请求参数格式错误"})
		return
	}
	item, secret, err := h.platform.CreateWebhookEndpoint(c.Request.Context(), user.ID, body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": item, "secret": secret})
}

func (h *OperationsHandler) UpdateWebhook(c *gin.Context) {
	user := currentUser(c)
	var body service.WebhookEndpointInput
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "请求参数格式错误"})
		return
	}
	if err := h.platform.UpdateWebhookEndpoint(c.Request.Context(), user.ID, c.Param("id"), body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *OperationsHandler) DeleteWebhook(c *gin.Context) {
	user := currentUser(c)
	if err := h.platform.DeleteWebhookEndpoint(c.Request.Context(), user.ID, c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *OperationsHandler) TestWebhook(c *gin.Context) {
	user := currentUser(c)
	if err := h.platform.TestWebhook(c.Request.Context(), user.ID, c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"ok": true})
}

func (h *OperationsHandler) WebhookDeliveries(c *gin.Context) {
	user := currentUser(c)
	items, err := h.platform.WebhookDeliveries(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "读取投递记录失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *OperationsHandler) Workflows(c *gin.Context) {
	user := currentUser(c)
	items, err := h.platform.WorkflowPresets(c.Request.Context(), user.ID, strings.TrimSpace(c.Query("kind")))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "读取工作流失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *OperationsHandler) CreateWorkflow(c *gin.Context) {
	user := currentUser(c)
	var body service.WorkflowPresetInput
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "请求参数格式错误"})
		return
	}
	item, err := h.platform.CreateWorkflowPreset(c.Request.Context(), user.ID, body, false)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": item})
}

func (h *OperationsHandler) UpdateWorkflow(c *gin.Context) {
	user := currentUser(c)
	var body service.WorkflowPresetInput
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "请求参数格式错误"})
		return
	}
	if err := h.platform.UpdateWorkflowPreset(c.Request.Context(), user.ID, c.Param("id"), body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *OperationsHandler) DeleteWorkflow(c *gin.Context) {
	user := currentUser(c)
	if err := h.platform.DeleteWorkflowPreset(c.Request.Context(), user.ID, c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *OperationsHandler) RenderWorkflow(c *gin.Context) {
	user := currentUser(c)
	var body struct {
		Values map[string]string `json:"values"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "请求参数格式错误"})
		return
	}
	data, err := h.platform.RenderWorkflow(c.Request.Context(), user.ID, c.Param("id"), body.Values)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}
