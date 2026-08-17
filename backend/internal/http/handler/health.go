package handler

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"backend/internal/storage"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

type HealthHandler struct {
	db    *sql.DB
	redis *redis.Client
	store *storage.Client
}

func NewHealthHandler(db *sql.DB, redisClient *redis.Client, store *storage.Client) *HealthHandler {
	return &HealthHandler{db: db, redis: redisClient, store: store}
}

// Live proves the HTTP process is running. Load balancers should use Handle for
// readiness and Live only as a restart probe.
func (h *HealthHandler) Live(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *HealthHandler) Handle(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	components := gin.H{"postgres": "ok", "redis": "ok", "storage": "ok"}
	ready := true
	if h.db == nil || h.db.PingContext(ctx) != nil {
		components["postgres"] = "unavailable"
		ready = false
	}
	if h.redis == nil || h.redis.Ping(ctx).Err() != nil {
		components["redis"] = "unavailable"
		ready = false
	}
	if h.store == nil || h.store.Check(ctx) != nil {
		components["storage"] = "unavailable"
		ready = false
	}
	status := http.StatusOK
	if !ready {
		status = http.StatusServiceUnavailable
	}
	c.JSON(status, gin.H{"ok": ready, "components": components})
}
