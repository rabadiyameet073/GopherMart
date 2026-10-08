package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/gophermart/ecommerce-order-inventory/internal/model"
	"github.com/gophermart/ecommerce-order-inventory/internal/repository"
	"github.com/gophermart/ecommerce-order-inventory/internal/worker"
	"github.com/gophermart/ecommerce-order-inventory/pkg/logger"
	"github.com/gophermart/ecommerce-order-inventory/pkg/validator"
)

var (
	ErrEmptyOrder = errors.New("order must contain at least one item")
)

type OrderService interface {
	CreateOrder(ctx context.Context, userID, idempotencyKey string, req model.CreateOrderRequest) (*model.Order, bool, error)
	GetOrderByID(ctx context.Context, orderID, userID string) (*model.Order, error)
	ListUserOrders(ctx context.Context, userID string) ([]model.Order, error)
}

type orderServiceImpl struct {
	orderRepo       repository.OrderRepository
	productRepo     repository.ProductRepository
	inventoryRepo   repository.InventoryRepository
	idempotencyRepo repository.IdempotencyRepository
	cartRepo        repository.CartRepository
	workerPool      *worker.Pool
}

func NewOrderService(
	orderRepo repository.OrderRepository,
	productRepo repository.ProductRepository,
	inventoryRepo repository.InventoryRepository,
	idempotencyRepo repository.IdempotencyRepository,
	cartRepo repository.CartRepository,
	workerPool *worker.Pool,
) OrderService {
	return &orderServiceImpl{
		orderRepo:       orderRepo,
		productRepo:     productRepo,
		inventoryRepo:   inventoryRepo,
		idempotencyRepo: idempotencyRepo,
		cartRepo:        cartRepo,
		workerPool:      workerPool,
	}
}

func (s *orderServiceImpl) CreateOrder(ctx context.Context, userID, idempotencyKey string, req model.CreateOrderRequest) (*model.Order, bool, error) {
	if len(req.Items) == 0 {
		return nil, false, ErrEmptyOrder
	}

	for _, it := range req.Items {
		if err := validator.ValidateRequired("product_id", it.ProductID); err != nil {
			return nil, false, err
		}
		if err := validator.ValidatePositive("quantity", it.Quantity); err != nil {
			return nil, false, err
		}
	}

	// 1. Idempotency Check
	var reqHash string
	if idempotencyKey != "" {
		payloadBytes, err := json.Marshal(req)
		if err != nil {
			return nil, false, fmt.Errorf("failed to serialize order payload: %w", err)
		}
		h := sha256.Sum256(payloadBytes)
		reqHash = hex.EncodeToString(h[:])

		existingRecord, err := s.idempotencyRepo.Get(ctx, idempotencyKey, userID)
		if err != nil {
			return nil, false, err
		}

		if existingRecord != nil {
			if existingRecord.RequestHash != reqHash {
				return nil, false, repository.ErrIdempotencyConflict
			}
			if existingRecord.Status == "PROCESSING" {
				return nil, false, repository.ErrDuplicateInFlight
			}
			if existingRecord.Status == "COMPLETED" {
				var cachedOrder model.Order
				if err := json.Unmarshal([]byte(existingRecord.ResponseBody), &cachedOrder); err == nil {
					logger.Info("Idempotent order request replayed from cache", "idempotency_key", idempotencyKey, "order_id", cachedOrder.ID)
					return &cachedOrder, true, nil
				}
			}
		} else {
			// Register processing status
			record := &model.IdempotencyRecord{
				Key:         idempotencyKey,
				UserID:      userID,
				RequestHash: reqHash,
				Status:      "PROCESSING",
				ExpiresAt:   time.Now().Add(24 * time.Hour),
			}
			if err := s.idempotencyRepo.CreateProcessing(ctx, record); err != nil {
				return nil, false, err
			}
		}
	}

	// 2. Fetch Products
	productIDs := make([]string, len(req.Items))
	for i, it := range req.Items {
		productIDs[i] = it.ProductID
	}

	productsMap, err := s.productRepo.GetByIDs(ctx, productIDs)
	if err != nil {
		s.failIdempotency(ctx, idempotencyKey, err)
		return nil, false, err
	}

	for _, it := range req.Items {
		if _, exists := productsMap[it.ProductID]; !exists {
			errNotFound := fmt.Errorf("product %s not found", it.ProductID)
			s.failIdempotency(ctx, idempotencyKey, errNotFound)
			return nil, false, errNotFound
		}
	}

	// 3. Sort items deterministically by ProductID to prevent database deadlocks
	sort.Slice(req.Items, func(i, j int) bool {
		return req.Items[i].ProductID < req.Items[j].ProductID
	})

	// Begin Transaction with Row-Level Locking to Prevent Overselling
	tx, err := s.orderRepo.BeginTx(ctx)
	if err != nil {
		s.failIdempotency(ctx, idempotencyKey, err)
		return nil, false, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	orderID := uuid.New().String()
	var totalCents int
	orderItems := make([]model.OrderItem, len(req.Items))

	for i, it := range req.Items {
		prod := productsMap[it.ProductID]
		linePrice := prod.PriceCents * it.Quantity
		totalCents += linePrice

		// Deduct stock under row lock: SELECT ... FOR UPDATE
		if err := s.inventoryRepo.DeductStockTx(ctx, tx, it.ProductID, it.Quantity); err != nil {
			s.failIdempotency(ctx, idempotencyKey, err)
			return nil, false, err
		}

		orderItems[i] = model.OrderItem{
			ID:         uuid.New().String(),
			OrderID:    orderID,
			ProductID:  it.ProductID,
			Quantity:   it.Quantity,
			PriceCents: prod.PriceCents,
		}
	}

	newOrder := &model.Order{
		ID:               orderID,
		UserID:           userID,
		Status:           model.OrderStatusConfirmed,
		TotalAmountCents: totalCents,
		Items:            orderItems,
	}

	if err := s.orderRepo.CreateTx(ctx, tx, newOrder, orderItems); err != nil {
		s.failIdempotency(ctx, idempotencyKey, err)
		return nil, false, fmt.Errorf("failed to persist order: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		s.failIdempotency(ctx, idempotencyKey, err)
		return nil, false, fmt.Errorf("failed to commit order transaction: %w", err)
	}

	// Clear user's Redis shopping cart after successful commit
	if s.cartRepo != nil {
		_ = s.cartRepo.ClearCart(ctx, userID)
	}

	// 4. Complete Idempotency record
	if idempotencyKey != "" {
		resBytes, _ := json.Marshal(newOrder)
		_ = s.idempotencyRepo.Complete(ctx, idempotencyKey, http.StatusCreated, string(resBytes))
	}

	// 5. Emit Asynchronous Order Event to Bounded Worker Pool
	if s.workerPool != nil {
		event := model.OrderEvent{
			OrderID:   newOrder.ID,
			UserID:    newOrder.UserID,
			Status:    newOrder.Status,
			Timestamp: time.Now(),
		}
		job := worker.Job{
			Event:      event,
			MaxRetries: 3,
			Timeout:    5 * time.Second,
			Handler:    s.handleAsyncOrderEvent,
		}
		if err := s.workerPool.Submit(job); err != nil {
			logger.Warn("Failed to submit async order event to pool", "order_id", newOrder.ID, "error", err)
		}
	}

	return newOrder, false, nil
}

func (s *orderServiceImpl) handleAsyncOrderEvent(ctx context.Context, evt model.OrderEvent) error {
	logger.Info("Asynchronously dispatching dark-store fulfillment and notification",
		"order_id", evt.OrderID,
		"user_id", evt.UserID,
		"status", evt.Status,
	)
	// Simulate order routing to dark-store picker
	return nil
}

func (s *orderServiceImpl) failIdempotency(ctx context.Context, key string, err error) {
	if key != "" {
		_ = s.idempotencyRepo.Fail(ctx, key, http.StatusBadRequest, err.Error())
	}
}

func (s *orderServiceImpl) GetOrderByID(ctx context.Context, orderID, userID string) (*model.Order, error) {
	return s.orderRepo.GetByID(ctx, orderID, userID)
}

func (s *orderServiceImpl) ListUserOrders(ctx context.Context, userID string) ([]model.Order, error) {
	return s.orderRepo.ListByUser(ctx, userID)
}
