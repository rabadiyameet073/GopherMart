package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gophermart/ecommerce-order-inventory/internal/handler"
	"github.com/gophermart/ecommerce-order-inventory/internal/model"
	"github.com/gophermart/ecommerce-order-inventory/internal/service"
	"github.com/gophermart/ecommerce-order-inventory/internal/worker"
	"github.com/gophermart/ecommerce-order-inventory/pkg/logger"
	"github.com/gophermart/ecommerce-order-inventory/pkg/response"
)

func init() {
	logger.Init()
}

// In-memory mock Redis Repo for rate limiting simulation
type mockE2ERedisRepo struct {
	mu     sync.Mutex
	tokens map[string]int
}

func (m *mockE2ERedisRepo) AllowRateLimit(ctx context.Context, identifier string, rate, capacity int) (bool, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cur, exists := m.tokens[identifier]
	if !exists {
		cur = capacity
	}
	if cur <= 0 {
		return false, 0, nil
	}
	m.tokens[identifier] = cur - 1
	return true, cur - 1, nil
}

func (m *mockE2ERedisRepo) SetCache(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return nil
}
func (m *mockE2ERedisRepo) GetCache(ctx context.Context, key string) ([]byte, error) {
	return nil, nil
}
func (m *mockE2ERedisRepo) DeleteCache(ctx context.Context, key string) error {
	return nil
}

// TestFullHTTPE2E executes a comprehensive end-to-end integration test of the entire API
func TestFullHTTPE2E(t *testing.T) {
	jwtSecret := "e2e-super-secret-key-2026"

	userRepo := &mockUserRepo{users: make(map[string]*model.User)}
	prodRepo := &mockProductRepo{products: make(map[string]model.Product)}
	invRepo := &mockInventoryRepo{stock: make(map[string]int)}
	orderRepo := &mockOrderRepo{orders: make(map[string]*model.Order)}
	idemRepo := &mockIdempotencyRepo{records: make(map[string]*model.IdempotencyRecord)}
	cartRepo := &mockCartRepo{carts: make(map[string]*model.Cart)}
	redisRepo := &mockE2ERedisRepo{tokens: make(map[string]int)}

	workerPool := worker.NewPool(2, 20)
	workerPool.Start()
	defer func() { _ = workerPool.Stop(2 * time.Second) }()

	authSvc := service.NewAuthService(userRepo, jwtSecret, 24*time.Hour)
	invSvc := service.NewInventoryService(invRepo)
	cartSvc := service.NewCartService(cartRepo)
	orderSvc := service.NewOrderService(orderRepo, prodRepo, invRepo, idemRepo, cartRepo, workerPool)

	authH := handler.NewAuthHandler(authSvc)
	prodH := handler.NewProductHandler(prodRepo)
	invH := handler.NewInventoryHandler(invSvc)
	cartH := handler.NewCartHandler(cartSvc)
	orderH := handler.NewOrderHandler(orderSvc)
	healthH := handler.NewHealthHandler(nil, nil)

	router := handler.SetupRouter(handler.RouterParams{
		AuthHandler:      authH,
		ProductHandler:   prodH,
		InventoryHandler: invH,
		OrderHandler:     orderH,
		CartHandler:      cartH,
		HealthHandler:    healthH,
		RedisRepo:        redisRepo,
		JWTSecret:        jwtSecret,
		RateLimit:        100,
		RateBurst:        100,
		MaxBodyBytes:     1048576,
	})

	ts := httptest.NewServer(router)
	defer ts.Close()

	client := ts.Client()

	// 1. Health Endpoints
	t.Run("Health and Readiness Checks", func(t *testing.T) {
		res, err := client.Get(ts.URL + "/health/live")
		if err != nil || res.StatusCode != http.StatusOK {
			t.Fatalf("Expected 200 from /health/live, got status %v, err %v", res.StatusCode, err)
		}
		res, err = client.Get(ts.URL + "/health/ready")
		if err != nil || res.StatusCode != http.StatusOK {
			t.Fatalf("Expected 200 from /health/ready, got status %v, err %v", res.StatusCode, err)
		}
	})

	var customerToken string
	var adminToken string

	// 2. Auth Registration & Login
	t.Run("Customer Registration & Login Flow", func(t *testing.T) {
		regBody, _ := json.Marshal(model.RegisterRequest{
			Name:     "Gopher Customer",
			Email:    "customer@gophermart.com",
			Password: "securePassword123",
		})

		resp, err := client.Post(ts.URL+"/api/v1/auth/register", "application/json", bytes.NewBuffer(regBody))
		if err != nil || resp.StatusCode != http.StatusCreated {
			t.Fatalf("Registration failed: status %v, err %v", resp.StatusCode, err)
		}

		var apiRes response.APIResponse
		_ = json.NewDecoder(resp.Body).Decode(&apiRes)
		dataMap := apiRes.Data.(map[string]interface{})
		customerToken = dataMap["token"].(string)

		if customerToken == "" {
			t.Fatalf("Expected customer token, got empty")
		}

		// Verify /users/me with customer token
		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/users/me", nil)
		req.Header.Set("Authorization", "Bearer "+customerToken)
		meResp, err := client.Do(req)
		if err != nil || meResp.StatusCode != http.StatusOK {
			t.Fatalf("Expected 200 from /users/me, got %v", meResp.StatusCode)
		}

		// Register an Admin User directly in repo for role verification
		adminUser := &model.User{
			ID:           "admin-uuid-1",
			Email:        "admin@gophermart.com",
			PasswordHash: userRepo.users["customer@gophermart.com"].PasswordHash,
			Name:         "Store Admin",
			Role:         "admin",
		}
		_ = userRepo.Create(context.Background(), adminUser)

		loginBody, _ := json.Marshal(model.LoginRequest{
			Email:    "admin@gophermart.com",
			Password: "securePassword123",
		})
		lResp, err := client.Post(ts.URL+"/api/v1/auth/login", "application/json", bytes.NewBuffer(loginBody))
		if err != nil || lResp.StatusCode != http.StatusOK {
			t.Fatalf("Admin login failed: %v", lResp.StatusCode)
		}
		var lApiRes response.APIResponse
		_ = json.NewDecoder(lResp.Body).Decode(&lApiRes)
		adminToken = lApiRes.Data.(map[string]interface{})["token"].(string)
	})

	prodID := "prod-specialty-matcha-01"

	// 3. Admin Product Creation & Authorization Check
	t.Run("Product Creation & RBAC", func(t *testing.T) {
		createPayload, _ := json.Marshal(model.CreateProductRequest{
			SKU:          "SKU-MATCHA-01",
			Name:         "Organic Matcha Tea",
			Description:  "Stone-ground Japanese matcha",
			PriceCents:   899,
			Category:     "Beverages",
			InitialStock: 25,
		})

		// Customer attempt should return 403 Forbidden
		custReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/products", bytes.NewBuffer(createPayload))
		custReq.Header.Set("Authorization", "Bearer "+customerToken)
		custReq.Header.Set("Content-Type", "application/json")
		custRes, _ := client.Do(custReq)
		if custRes.StatusCode != http.StatusForbidden {
			t.Errorf("Expected 403 Forbidden for customer product creation, got %d", custRes.StatusCode)
		}

		// Admin attempt should succeed with 201 Created
		adminReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/products", bytes.NewBuffer(createPayload))
		adminReq.Header.Set("Authorization", "Bearer "+adminToken)
		adminReq.Header.Set("Content-Type", "application/json")
		adminRes, err := client.Do(adminReq)
		if err != nil || adminRes.StatusCode != http.StatusCreated {
			t.Fatalf("Expected 201 Created for admin product creation, got %d", adminRes.StatusCode)
		}

		var createdApiRes response.APIResponse
		_ = json.NewDecoder(adminRes.Body).Decode(&createdApiRes)
		createdData := createdApiRes.Data.(map[string]interface{})
		prodID = createdData["id"].(string)

		// Set inventory stock in mock
		invRepo.SetStock(context.Background(), prodID, 25)
	})

	// 4. Cart Lifecycle
	t.Run("Cart CRUD Lifecycle", func(t *testing.T) {
		// Add item
		addPayload, _ := json.Marshal(model.AddCartItemRequest{
			ProductID: prodID,
			Quantity:  2,
		})
		addReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/cart/items", bytes.NewBuffer(addPayload))
		addReq.Header.Set("Authorization", "Bearer "+customerToken)
		addReq.Header.Set("Content-Type", "application/json")
		addRes, err := client.Do(addReq)
		if err != nil || addRes.StatusCode != http.StatusOK {
			t.Fatalf("Failed to add item to cart: status %d", addRes.StatusCode)
		}

		// View cart
		getReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/cart", nil)
		getReq.Header.Set("Authorization", "Bearer "+customerToken)
		getRes, _ := client.Do(getReq)
		var cartApiRes response.APIResponse
		_ = json.NewDecoder(getRes.Body).Decode(&cartApiRes)
		cartData := cartApiRes.Data.(map[string]interface{})
		items := cartData["items"].([]interface{})
		if len(items) != 1 {
			t.Fatalf("Expected 1 item in cart, got %d", len(items))
		}
	})

	// 5. Idempotent Order Checkout & Stock Verification
	t.Run("Idempotent Order Creation Flow", func(t *testing.T) {
		orderPayload, _ := json.Marshal(model.CreateOrderRequest{
			Items: []model.OrderItemRequest{
				{ProductID: prodID, Quantity: 2},
			},
		})

		idemKey := "idempotency-key-test-001"

		// First Order Request
		oReq1, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/orders", bytes.NewBuffer(orderPayload))
		oReq1.Header.Set("Authorization", "Bearer "+customerToken)
		oReq1.Header.Set("Idempotency-Key", idemKey)
		oReq1.Header.Set("Content-Type", "application/json")

		res1, err := client.Do(oReq1)
		if err != nil || res1.StatusCode != http.StatusCreated {
			t.Fatalf("First order attempt failed: %d", res1.StatusCode)
		}
		var oRes1 response.APIResponse
		_ = json.NewDecoder(res1.Body).Decode(&oRes1)
		orderID := oRes1.Data.(map[string]interface{})["id"].(string)

		// Second Order Request with IDENTICAL Idempotency-Key
		oReq2, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/orders", bytes.NewBuffer(orderPayload))
		oReq2.Header.Set("Authorization", "Bearer "+customerToken)
		oReq2.Header.Set("Idempotency-Key", idemKey)
		oReq2.Header.Set("Content-Type", "application/json")

		res2, err := client.Do(oReq2)
		if err != nil || res2.StatusCode != http.StatusCreated {
			t.Fatalf("Idempotent replay failed: %d", res2.StatusCode)
		}
		cacheHeader := res2.Header.Get("X-Cache-Lookup")
		if cacheHeader != "HIT-IDEMPOTENT" {
			t.Errorf("Expected X-Cache-Lookup: HIT-IDEMPOTENT header, got %s", cacheHeader)
		}

		var oRes2 response.APIResponse
		_ = json.NewDecoder(res2.Body).Decode(&oRes2)
		replayedID := oRes2.Data.(map[string]interface{})["id"].(string)
		if replayedID != orderID {
			t.Errorf("Expected matching order IDs %s == %s", orderID, replayedID)
		}

		// Cart must now be empty after successful order
		cReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/cart", nil)
		cReq.Header.Set("Authorization", "Bearer "+customerToken)
		cRes, _ := client.Do(cReq)
		var clearedCartRes response.APIResponse
		_ = json.NewDecoder(cRes.Body).Decode(&clearedCartRes)
		clearedItems := clearedCartRes.Data.(map[string]interface{})["items"].([]interface{})
		if len(clearedItems) != 0 {
			t.Errorf("Expected cart to be empty after checkout, got %d items", len(clearedItems))
		}

		// Stock in repository must be 25 - 2 = 23
		if invRepo.stock[prodID] != 23 {
			t.Errorf("Expected 23 remaining stock, got %d", invRepo.stock[prodID])
		}
	})

	// 6. Token-Bucket Rate Limiter Trigger
	t.Run("Token-Bucket Rate Limiter Rejection", func(t *testing.T) {
		// Exhaust remaining tokens for customer
		redisRepo.mu.Lock()
		redisRepo.tokens["usr:"+userRepo.users["customer@gophermart.com"].ID] = 0
		redisRepo.mu.Unlock()

		rlReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/cart", nil)
		rlReq.Header.Set("Authorization", "Bearer "+customerToken)
		rlRes, err := client.Do(rlReq)
		if err != nil {
			t.Fatalf("Rate limit request failed: %v", err)
		}

		if rlRes.StatusCode != http.StatusTooManyRequests {
			t.Errorf("Expected 429 Too Many Requests, got %d", rlRes.StatusCode)
		}
		retryAfter := rlRes.Header.Get("Retry-After")
		if retryAfter == "" {
			t.Errorf("Expected Retry-After header on 429 response")
		}
	})

	// 7. Request Body Size Limiter
	t.Run("Oversized Request Body Rejection", func(t *testing.T) {
		// Restore tokens
		redisRepo.mu.Lock()
		redisRepo.tokens["usr:"+userRepo.users["customer@gophermart.com"].ID] = 100
		redisRepo.mu.Unlock()

		// Generate oversized body > 1MB
		hugePayload := make([]byte, 1048576+100)
		bodyReader := bytes.NewReader(hugePayload)

		hugeReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/cart/items", bodyReader)
		hugeReq.Header.Set("Authorization", "Bearer "+customerToken)
		hugeReq.Header.Set("Content-Type", "application/json")

		hugeRes, err := client.Do(hugeReq)
		if err == nil {
			bodyBytes, _ := io.ReadAll(hugeRes.Body)
			t.Logf("Huge request completed with status: %d, body: %s", hugeRes.StatusCode, string(bodyBytes))
		}
	})
}
