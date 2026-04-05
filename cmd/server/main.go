package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/TheTuxis/gondor-search/internal/config"
	es "github.com/TheTuxis/gondor-search/internal/elasticsearch"
	"github.com/TheTuxis/gondor-search/internal/handler"
	"github.com/TheTuxis/gondor-search/internal/middleware"
	jwtpkg "github.com/TheTuxis/gondor-search/internal/pkg/jwt"
	"github.com/TheTuxis/gondor-search/internal/service"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Init logger
	var logger *zap.Logger
	var err error
	if cfg.Environment == "production" {
		logger, err = zap.NewProduction()
	} else {
		logger, err = zap.NewDevelopment()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to init logger: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = logger.Sync() }()

	// Init Elasticsearch client
	esClient, err := es.NewClient(cfg.ElasticsearchURL, logger)
	if err != nil {
		logger.Fatal("failed to create elasticsearch client", zap.Error(err))
	}

	// Verify Elasticsearch connectivity
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := esClient.Ping(ctx); err != nil {
		logger.Warn("elasticsearch not reachable at startup, continuing anyway", zap.Error(err))
	} else {
		logger.Info("connected to Elasticsearch", zap.String("url", cfg.ElasticsearchURL))
	}

	// Init Redis client
	var redisClient *redis.Client
	if cfg.RedisURL != "" {
		opts, err := redis.ParseURL("redis://" + cfg.RedisURL)
		if err != nil {
			// Fallback: treat as host:port
			opts = &redis.Options{Addr: cfg.RedisURL}
		}
		redisClient = redis.NewClient(opts)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := redisClient.Ping(ctx).Err(); err != nil {
			logger.Warn("redis connection failed, continuing without redis", zap.Error(err))
			redisClient = nil
		} else {
			logger.Info("connected to Redis")
		}
	}

	// Init JWT manager (validate-only — tokens are issued by gondor-users-security)
	jwtManager := jwtpkg.NewManager(cfg.JWTSecret)

	// Init services
	searchService := service.NewSearchService(esClient, logger)

	// Init handlers
	healthHandler := handler.NewHealthHandler(esClient, redisClient)
	searchHandler := handler.NewSearchHandler(searchService)

	// Setup Gin
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.LoggingMiddleware(logger))
	router.Use(middleware.AuthMiddleware(jwtManager))

	// Health & metrics (no auth required — handled by skip list)
	router.GET("/health", healthHandler.Health)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Search routes
	v1 := router.Group("/v1")
	{
		v1.POST("/search", searchHandler.Search)
		v1.POST("/search/suggest", searchHandler.Suggest)
		v1.POST("/search/index", searchHandler.IndexDocument)
		v1.DELETE("/search/index/:entity_type/:id", searchHandler.DeleteDocument)
		v1.POST("/search/reindex/:entity_type", searchHandler.Reindex)
	}

	// Start server
	addr := fmt.Sprintf(":%d", cfg.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("starting server", zap.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server failed", zap.Error(err))
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")

	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("server forced to shutdown", zap.Error(err))
	}

	if redisClient != nil {
		_ = redisClient.Close()
	}

	logger.Info("server stopped")
}
