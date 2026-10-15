package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gophermart/ecommerce-order-inventory/internal/config"
	"github.com/gophermart/ecommerce-order-inventory/internal/handler"
	"github.com/gophermart/ecommerce-order-inventory/internal/repository"
	"github.com/gophermart/ecommerce-order-inventory/internal/service"
	"github.com/gophermart/ecommerce-order-inventory/internal/worker"
	"github.com/gophermart/ecommerce-order-inventory/pkg/database"
	"github.com/gophermart/ecommerce-order-inventory/pkg/logger"
)

func main() {
	logger.Init()
	logger.Info("Starting GopherMart Order & Inventory Service")

	cfg := config.Load()

	// 1. Initialize Postgres
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pgDB, err := database.NewPostgresDB(ctx, cfg.PostgresURL)
	if err != nil {
		logger.Error("Failed to initialize PostgreSQL pool", "error", err)
		os.Exit(1)
	}
	defer pgDB.Close()
	logger.Info("Connected to PostgreSQL successfully")

	// 2. Initialize Redis
	redisClient, err := database.NewRedisClient(ctx, cfg.RedisAddr, cfg.RedisPass, cfg.RedisDB)
	if err != nil {
		logger.Error("Failed to initialize Redis client", "error", err)
		os.Exit(1)
	}
	defer redisClient.Close()
	logger.Info("Connected to Redis successfully")

	// 3. Initialize Repositories
	userRepo := repository.NewUserRepository(pgDB.Pool)
	productRepo := repository.NewProductRepository(pgDB.Pool)
	invRepo := repository.NewInventoryRepository(pgDB.Pool)
	orderRepo := repository.NewOrderRepository(pgDB.Pool)
	idempotencyRepo := repository.NewIdempotencyRepository(pgDB.Pool)
	redisRepo := repository.NewRedisRepository(redisClient.Client)
	cartRepo := repository.NewCartRepository(redisClient.Client)

	// 4. Initialize Bounded Worker Pool
	workerPool := worker.NewPool(cfg.WorkerCount, cfg.WorkerQueue)
	workerPool.Start()

	// 5. Initialize Services
	authService := service.NewAuthService(userRepo, cfg.JWTSecret, cfg.JWTExpiry)
	invService := service.NewInventoryService(invRepo)
	cartService := service.NewCartService(cartRepo)
	orderService := service.NewOrderService(orderRepo, productRepo, invRepo, idempotencyRepo, cartRepo, workerPool)

	// 6. Initialize Handlers
	authHandler := handler.NewAuthHandler(authService)
	productHandler := handler.NewProductHandler(productRepo)
	invHandler := handler.NewInventoryHandler(invService)
	cartHandler := handler.NewCartHandler(cartService)
	orderHandler := handler.NewOrderHandler(orderService)
	healthHandler := handler.NewHealthHandler(pgDB.Pool, redisClient.Client)

	// 7. Setup Router & Server
	router := handler.SetupRouter(handler.RouterParams{
		AuthHandler:      authHandler,
		ProductHandler:   productHandler,
		InventoryHandler: invHandler,
		OrderHandler:     orderHandler,
		CartHandler:      cartHandler,
		HealthHandler:    healthHandler,
		RedisRepo:        redisRepo,
		JWTSecret:        cfg.JWTSecret,
		RateLimit:        cfg.RateLimit,
		RateBurst:        cfg.RateBurst,
		MaxBodyBytes:     cfg.MaxBodyBytes,
	})

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 8. Start Server Asynchronously
	go func() {
		logger.Info(fmt.Sprintf("HTTP Server listening on port %s", cfg.Port))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("HTTP Server fatal error", "error", err)
			os.Exit(1)
		}
	}()

	// 9. Graceful Shutdown on Interrupt or SIGTERM
	shutdownSig := make(chan os.Signal, 1)
	signal.Notify(shutdownSig, os.Interrupt, syscall.SIGTERM)
	<-shutdownSig

	logger.Info("Shutdown signal received, initiating graceful teardown...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("Server forced to shutdown", "error", err)
	} else {
		logger.Info("HTTP server gracefully stopped")
	}

	// Drain and stop worker pool
	if err := workerPool.Stop(5 * time.Second); err != nil {
		logger.Error("Worker pool shutdown error", "error", err)
	}

	logger.Info("GopherMart service stopped cleanly")
}

