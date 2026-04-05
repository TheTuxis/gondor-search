package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	es "github.com/TheTuxis/gondor-search/internal/elasticsearch"
)

type HealthHandler struct {
	esClient    *es.Client
	redisClient *redis.Client
}

func NewHealthHandler(esClient *es.Client, redisClient *redis.Client) *HealthHandler {
	return &HealthHandler{esClient: esClient, redisClient: redisClient}
}

func (h *HealthHandler) Health(c *gin.Context) {
	status := "healthy"
	checks := make(map[string]string)

	// Check Elasticsearch
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	if err := h.esClient.Ping(ctx); err != nil {
		status = "unhealthy"
		checks["elasticsearch"] = "error: " + err.Error()
	} else {
		checks["elasticsearch"] = "ok"
	}

	// Check Redis
	if h.redisClient != nil {
		if err := h.redisClient.Ping(c.Request.Context()).Err(); err != nil {
			checks["redis"] = "error: " + err.Error()
		} else {
			checks["redis"] = "ok"
		}
	} else {
		checks["redis"] = "not configured"
	}

	code := http.StatusOK
	if status == "unhealthy" {
		code = http.StatusServiceUnavailable
	}

	c.JSON(code, gin.H{
		"status":  status,
		"service": "gondor-search",
		"checks":  checks,
	})
}
