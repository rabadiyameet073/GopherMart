package handler

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gophermart/ecommerce-order-inventory/internal/middleware"
	"github.com/gophermart/ecommerce-order-inventory/internal/repository"
)

type RouterParams struct {
	AuthHandler      *AuthHandler
	ProductHandler   *ProductHandler
	InventoryHandler *InventoryHandler
	OrderHandler     *OrderHandler
	CartHandler      *CartHandler
	HealthHandler    *HealthHandler
	RedisRepo        repository.RedisRepository
	JWTSecret        string
	RateLimit        int
	RateBurst        int
	MaxBodyBytes     int64
}

func SetupRouter(params RouterParams) http.Handler {
	r := mux.NewRouter()

	// Global Middlewares
	r.Use(middleware.RecoveryMiddleware)
	r.Use(middleware.LoggingMiddleware)
	if params.MaxBodyBytes > 0 {
		r.Use(middleware.BodyLimitMiddleware(params.MaxBodyBytes))
	}
	if params.RedisRepo != nil {
		r.Use(middleware.RateLimitMiddleware(params.RedisRepo, params.RateLimit, params.RateBurst))
	}

	// Health Endpoints
	r.HandleFunc("/health", params.HealthHandler.Health).Methods(http.MethodGet)
	r.HandleFunc("/health/live", params.HealthHandler.Health).Methods(http.MethodGet)
	r.HandleFunc("/ready", params.HealthHandler.Ready).Methods(http.MethodGet)
	r.HandleFunc("/health/ready", params.HealthHandler.Ready).Methods(http.MethodGet)

	api := r.PathPrefix("/api/v1").Subrouter()

	// Public Auth Endpoints
	api.HandleFunc("/auth/register", params.AuthHandler.Register).Methods(http.MethodPost)
	api.HandleFunc("/auth/login", params.AuthHandler.Login).Methods(http.MethodPost)

	// Public Product Catalog Endpoints (Paginated)
	api.HandleFunc("/products", params.ProductHandler.List).Methods(http.MethodGet)
	api.HandleFunc("/products/{id}", params.ProductHandler.GetByID).Methods(http.MethodGet)

	// Protected Endpoints (Requires valid JWT)
	authSubrouter := api.PathPrefix("").Subrouter()
	authSubrouter.Use(middleware.AuthMiddleware(params.JWTSecret))

	// User Profile
	authSubrouter.HandleFunc("/users/me", params.AuthHandler.GetMe).Methods(http.MethodGet)

	// Redis Shopping Cart
	if params.CartHandler != nil {
		authSubrouter.HandleFunc("/cart", params.CartHandler.GetCart).Methods(http.MethodGet)
		authSubrouter.HandleFunc("/cart/items", params.CartHandler.AddItem).Methods(http.MethodPost)
		authSubrouter.HandleFunc("/cart/items/{productId}", params.CartHandler.UpdateItem).Methods(http.MethodPatch)
		authSubrouter.HandleFunc("/cart/items/{productId}", params.CartHandler.RemoveItem).Methods(http.MethodDelete)
		authSubrouter.HandleFunc("/cart", params.CartHandler.ClearCart).Methods(http.MethodDelete)
	}

	// Inventory Management
	authSubrouter.HandleFunc("/inventory/{productId}", params.InventoryHandler.GetStock).Methods(http.MethodGet)
	authSubrouter.HandleFunc("/inventory/{productId}", params.InventoryHandler.SetStock).Methods(http.MethodPut)

	// Order Operations with Idempotency & Concurrency protection
	authSubrouter.HandleFunc("/orders", params.OrderHandler.CreateOrder).Methods(http.MethodPost)
	authSubrouter.HandleFunc("/orders", params.OrderHandler.ListOrders).Methods(http.MethodGet)
	authSubrouter.HandleFunc("/orders/{id}", params.OrderHandler.GetOrder).Methods(http.MethodGet)

	// Admin-Only Product Management
	adminSubrouter := authSubrouter.PathPrefix("").Subrouter()
	adminSubrouter.Use(middleware.RequireRole("admin"))
	adminSubrouter.HandleFunc("/products", params.ProductHandler.Create).Methods(http.MethodPost)
	adminSubrouter.HandleFunc("/products/{id}", params.ProductHandler.Update).Methods(http.MethodPatch)

	return r
}
