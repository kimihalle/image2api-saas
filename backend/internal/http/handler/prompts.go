package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"backend/internal/repo"
	"backend/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type PromptHandler struct {
	service *service.PromptService
}

func NewPromptHandler(promptService *service.PromptService) *PromptHandler {
	return &PromptHandler{service: promptService}
}

func (h *PromptHandler) PublicCategories(c *gin.Context) { h.categories(c, false) }
func (h *PromptHandler) AdminCategories(c *gin.Context)  { h.categories(c, true) }

func (h *PromptHandler) categories(c *gin.Context, admin bool) {
	rows, err := h.service.Categories(c.Request.Context(), admin)
	if err != nil {
		writePromptError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows})
}

func (h *PromptHandler) PublicList(c *gin.Context) { h.list(c, false) }
func (h *PromptHandler) AdminList(c *gin.Context)  { h.list(c, true) }

func (h *PromptHandler) list(c *gin.Context, admin bool) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "24"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	filter := repo.PromptListFilter{
		MediaType: c.Query("media_type"), Category: c.Query("category"), Query: c.Query("q"),
		Status: c.Query("status"), Sort: c.Query("sort"), Limit: limit, Offset: offset, Admin: admin,
	}
	if raw := c.Query("featured"); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err == nil {
			filter.Featured = &value
		}
	}
	userID := ""
	if user := currentUser(c); user != nil {
		userID = user.ID
	}
	rows, total, err := h.service.List(c.Request.Context(), filter, userID)
	if err != nil {
		writePromptError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows, "total": total, "limit": limit, "offset": offset})
}

func (h *PromptHandler) PublicGet(c *gin.Context) { h.get(c, false) }
func (h *PromptHandler) AdminGet(c *gin.Context)  { h.get(c, true) }

func (h *PromptHandler) get(c *gin.Context, admin bool) {
	userID := ""
	if user := currentUser(c); user != nil {
		userID = user.ID
	}
	item, err := h.service.Get(c.Request.Context(), c.Param("id"), userID, admin)
	if err != nil {
		writePromptError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *PromptHandler) FavoriteIDs(c *gin.Context) {
	user := currentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "请先登录"})
		return
	}
	ids, err := h.service.FavoriteIDs(c.Request.Context(), user.ID)
	if err != nil {
		writePromptError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": ids})
}

func (h *PromptHandler) Favorite(c *gin.Context) {
	user := currentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "请先登录"})
		return
	}
	var body struct {
		Favorite bool `json:"favorite"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "请求参数无效"})
		return
	}
	favorite, count, err := h.service.SetFavorite(c.Request.Context(), user.ID, c.Param("id"), body.Favorite)
	if err != nil {
		writePromptError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"favorited": favorite, "favorite_count": count})
}

func (h *PromptHandler) Use(c *gin.Context) {
	user := currentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "请先登录"})
		return
	}
	var body struct {
		Values map[string]string `json:"values"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "请求参数无效"})
		return
	}
	result, err := h.service.RenderAndRecord(c.Request.Context(), user.ID, c.Param("id"), body.Values)
	if err != nil {
		writePromptError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *PromptHandler) CreateCategory(c *gin.Context) {
	var body service.PromptCategoryInput
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "请求参数无效"})
		return
	}
	item, err := h.service.CreateCategory(c.Request.Context(), body)
	if err != nil {
		writePromptError(c, err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h *PromptHandler) UpdateCategory(c *gin.Context) {
	var body service.PromptCategoryInput
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "请求参数无效"})
		return
	}
	if err := h.service.UpdateCategory(c.Request.Context(), c.Param("id"), body); err != nil {
		writePromptError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *PromptHandler) DeleteCategory(c *gin.Context) {
	if err := h.service.DeleteCategory(c.Request.Context(), c.Param("id")); err != nil {
		writePromptError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *PromptHandler) CreateTemplate(c *gin.Context) {
	var body service.PromptTemplateInput
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "请求参数无效"})
		return
	}
	adminID := ""
	if user := currentUser(c); user != nil {
		adminID = user.ID
	}
	item, err := h.service.CreateTemplate(c.Request.Context(), body, adminID)
	if err != nil {
		writePromptError(c, err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h *PromptHandler) UpdateTemplate(c *gin.Context) {
	var body service.PromptTemplateInput
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "请求参数无效"})
		return
	}
	if err := h.service.UpdateTemplate(c.Request.Context(), c.Param("id"), body); err != nil {
		writePromptError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *PromptHandler) DeleteTemplate(c *gin.Context) {
	if err := h.service.DeleteTemplate(c.Request.Context(), c.Param("id")); err != nil {
		writePromptError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *PromptHandler) Sources(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": h.service.Sources()})
}

func (h *PromptHandler) Sync(c *gin.Context) {
	var body struct {
		Source string `json:"source"`
		Limit  int    `json:"limit"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "请求参数无效"})
		return
	}
	item, err := h.service.Sync(c.Request.Context(), strings.TrimSpace(body.Source), body.Limit)
	if err != nil {
		writePromptError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *PromptHandler) ImportBatches(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	rows, err := h.service.ImportBatches(c.Request.Context(), limit)
	if err != nil {
		writePromptError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows})
}

func writePromptError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	detail := "灵感库操作失败"
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		status, detail = http.StatusNotFound, "模板或分类不存在"
	case errors.Is(err, service.ErrPromptInvalid):
		status, detail = http.StatusBadRequest, err.Error()
	case strings.Contains(err.Error(), "不能删除"):
		status, detail = http.StatusConflict, err.Error()
	}
	c.JSON(status, gin.H{"detail": detail})
}
