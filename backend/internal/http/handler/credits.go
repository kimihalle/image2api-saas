package handler

import (
	"net/http"
	"strings"

	"backend/internal/repo"
	"github.com/gin-gonic/gin"
)

type CreditHandler struct{ credits *repo.CreditRepository }

func NewCreditHandler(credits *repo.CreditRepository) *CreditHandler {
	return &CreditHandler{credits: credits}
}

func (h *CreditHandler) Mine(c *gin.Context) {
	user := currentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "not authenticated"})
		return
	}
	limit, offset := parseInt(c.Query("limit"), 20), parseInt(c.Query("offset"), 0)
	items, total, err := h.credits.ListByUser(c.Request.Context(), user.ID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to load credit ledger"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items, "total": total})
}

func (h *CreditHandler) Admin(c *gin.Context) {
	limit, offset := parseInt(c.Query("limit"), 50), parseInt(c.Query("offset"), 0)
	items, total, err := h.credits.List(c.Request.Context(), strings.TrimSpace(c.Query("status")), strings.TrimSpace(c.Query("user_id")), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to load credit ledger"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items, "total": total})
}
