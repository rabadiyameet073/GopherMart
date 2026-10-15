package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/gophermart/ecommerce-order-inventory/pkg/response"
)

type HealthHandler struct {
	db  *pgxpool.Pool
	rdb *redis.Client
}

func NewHealthHandler(db *pgxpool.Pool, rdb *redis.Client) *HealthHandler {
	return &HealthHandler{db: db, rdb: rdb}
}

func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, http.StatusOK, map[string]string{
		"status": "UP",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}

func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if h.db != nil {
		if err := h.db.Ping(ctx); err != nil {
			response.Error(w, http.StatusServiceUnavailable, "POSTGRES_DOWN", err.Error())
			return
		}
	}

	if h.rdb != nil {
		if err := h.rdb.Ping(ctx).Err(); err != nil {
			response.Error(w, http.StatusServiceUnavailable, "REDIS_DOWN", err.Error())
			return
		}
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"status":   "READY",
		"postgres": "connected",
		"redis":    "connected",
	})
}
