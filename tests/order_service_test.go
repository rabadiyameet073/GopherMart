package tests

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/gophermart/ecommerce-order-inventory/internal/model"
	"github.com/gophermart/ecommerce-order-inventory/internal/repository"
	"github.com/gophermart/ecommerce-order-inventory/internal/service"
)

// Mock Order Repository
type mockOrderRepo struct {
	mu     sync.Mutex
	orders map[string]*model.Order
}

func (m *mockOrderRepo) BeginTx(ctx context.Context) (pgx.Tx, error) {
	// Return a stub tx for unit test purposes
	return &stubTx{}, nil
}

func (m *mockOrderRepo) CreateTx(ctx context.Context, tx pgx.Tx, order *model.Order, items []model.OrderItem) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	order.Items = items
	m.orders[order.ID] = order
	return nil
}

func (m *mockOrderRepo) GetByID(ctx context.Context, id, userID string) (*model.Order, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.orders[id], nil
}

func (m *mockOrderRepo) ListByUser(ctx context.Context, userID string) ([]model.Order, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var list []model.Order
	for _, o := range m.orders {
		if o.UserID == userID {
			list = append(list, *o)
		}
	}
	return list, nil
}

func (m *mockOrderRepo) UpdateStatus(ctx context.Context, id string, status model.OrderStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if o, ok := m.orders[id]; ok {
		o.Status = status
	}
	return nil
}

// Stub pgx.Tx implementing transaction interface
type stubTx struct{}

func (s *stubTx) Begin(ctx context.Context) (pgx.Tx, error) { return s, nil }
func (s *stubTx) Commit(ctx context.Context) error         { return nil }
func (s *stubTx) Rollback(ctx context.Context) error       { return nil }
func (s *stubTx) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (s *stubTx) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults { return nil }
func (s *stubTx) LargeObjects() pgx.LargeObjects                              { return pgx.LargeObjects{} }
func (s *stubTx) Prepare(ctx context.Context, name, sql string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (s *stubTx) Exec(ctx context.Context, sql string, arguments ...any) (commandTag pgconn.CommandTag, err error) {
	return pgconn.NewCommandTag("UPDATE 1"), nil
}
func (s *stubTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return nil, nil
}
func (s *stubTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return nil
}
func (s *stubTx) Conn() *pgx.Conn { return nil }

// Mock Product Repository
type mockProductRepo struct {
	products map[string]model.Product
}

func (m *mockProductRepo) List(ctx context.Context, category string, limit, offset int) (*model.PaginatedProductsResponse, error) {
	return nil, nil
}

func (m *mockProductRepo) GetByID(ctx context.Context, id string) (*model.ProductWithStock, error) {
	if p, ok := m.products[id]; ok {
		return &model.ProductWithStock{Product: p, StockQuantity: 10}, nil
	}
	return nil, nil
}

func (m *mockProductRepo) GetByIDs(ctx context.Context, ids []string) (map[string]model.Product, error) {
	res := make(map[string]model.Product)
	for _, id := range ids {
		if p, ok := m.products[id]; ok {
			res[id] = p
		}
	}
	return res, nil
}

func (m *mockProductRepo) Create(ctx context.Context, p *model.Product, initialStock int) error {
	m.products[p.ID] = *p
	return nil
}

func (m *mockProductRepo) Update(ctx context.Context, id string, req model.UpdateProductRequest) (*model.ProductWithStock, error) {
	return nil, nil
}

// Mock Inventory Repository
type mockInventoryRepo struct {
	mu    sync.Mutex
	stock map[string]int
}

func (m *mockInventoryRepo) GetStock(ctx context.Context, productID string) (*model.Inventory, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return &model.Inventory{ProductID: productID, Quantity: m.stock[productID]}, nil
}

func (m *mockInventoryRepo) SetStock(ctx context.Context, productID string, quantity int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stock[productID] = quantity
	return nil
}

func (m *mockInventoryRepo) DeductStockTx(ctx context.Context, tx pgx.Tx, productID string, quantityToDeduct int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	current, ok := m.stock[productID]
	if !ok {
		return repository.ErrProductNotFound
	}
	if current < quantityToDeduct {
		return repository.ErrInsufficientStock
	}
	m.stock[productID] = current - quantityToDeduct
	return nil
}

// Mock Idempotency Repository
type mockIdempotencyRepo struct {
	mu      sync.Mutex
	records map[string]*model.IdempotencyRecord
}

func (m *mockIdempotencyRepo) Get(ctx context.Context, key, userID string) (*model.IdempotencyRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.records[key], nil
}

func (m *mockIdempotencyRepo) CreateProcessing(ctx context.Context, record *model.IdempotencyRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.records[record.Key]; ok {
		return repository.ErrDuplicateInFlight
	}
	m.records[record.Key] = record
	return nil
}

func (m *mockIdempotencyRepo) Complete(ctx context.Context, key string, statusCode int, responseBody string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if rec, ok := m.records[key]; ok {
		rec.Status = "COMPLETED"
		rec.ResponseStatusCode = statusCode
		rec.ResponseBody = responseBody
	}
	return nil
}

func (m *mockIdempotencyRepo) Fail(ctx context.Context, key string, statusCode int, responseBody string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if rec, ok := m.records[key]; ok {
		rec.Status = "FAILED"
		rec.ResponseStatusCode = statusCode
		rec.ResponseBody = responseBody
	}
	return nil
}

func TestOrderService_CreateOrder(t *testing.T) {
	prodID := "prod-artisan-loaf-002"
	orderRepo := &mockOrderRepo{orders: make(map[string]*model.Order)}
	prodRepo := &mockProductRepo{
		products: map[string]model.Product{
			prodID: {
				ID:         prodID,
				Name:       "Artisan Sourdough",
				PriceCents: 499,
				SKU:        "SKU-BREAD-002",
			},
		},
	}
	invRepo := &mockInventoryRepo{
		stock: map[string]int{
			prodID: 5, // initial stock 5
		},
	}
	idemRepo := &mockIdempotencyRepo{records: make(map[string]*model.IdempotencyRecord)}
	cartRepo := &mockCartRepo{carts: make(map[string]*model.Cart)}

	// Pre-populate user's cart
	userID := "usr-checkout-001"
	_ = cartRepo.SaveCart(context.Background(), &model.Cart{
		UserID: userID,
		Items:  []model.CartItem{{ProductID: prodID, Quantity: 2}},
	}, 24*time.Hour)

	orderSvc := service.NewOrderService(orderRepo, prodRepo, invRepo, idemRepo, cartRepo, nil)

	// 1. Order with Empty Items should fail
	_, _, err := orderSvc.CreateOrder(context.Background(), userID, "idem-1", model.CreateOrderRequest{Items: []model.OrderItemRequest{}})
	if !errors.Is(err, service.ErrEmptyOrder) {
		t.Errorf("Expected ErrEmptyOrder, got %v", err)
	}

	// 2. Successful Order with stock deduction
	orderReq := model.CreateOrderRequest{
		Items: []model.OrderItemRequest{
			{ProductID: prodID, Quantity: 2},
		},
	}
	order, isReplay, err := orderSvc.CreateOrder(context.Background(), userID, "idem-1", orderReq)
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}
	if isReplay {
		t.Errorf("Expected fresh order, got replay")
	}
	if order.TotalAmountCents != 499*2 {
		t.Errorf("Expected total 998 cents, got %d", order.TotalAmountCents)
	}

	// Verify inventory reduced from 5 to 3
	if invRepo.stock[prodID] != 3 {
		t.Errorf("Expected remaining stock 3, got %d", invRepo.stock[prodID])
	}

	// Verify cart cleared on commit
	userCart, _ := cartRepo.GetCart(context.Background(), userID)
	if len(userCart.Items) != 0 {
		t.Errorf("Expected cart to be cleared on commit, got %d items", len(userCart.Items))
	}

	// 3. Repeated request with same idempotency key returns cached replay
	replayOrder, isReplay, err := orderSvc.CreateOrder(context.Background(), userID, "idem-1", orderReq)
	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}
	if !isReplay {
		t.Errorf("Expected isReplay to be true")
	}
	if replayOrder.ID != order.ID {
		t.Errorf("Expected replay order ID %s, got %s", order.ID, replayOrder.ID)
	}
	// Stock must NOT have been deducted a second time
	if invRepo.stock[prodID] != 3 {
		t.Errorf("Stock was incorrectly deducted during replay: %d", invRepo.stock[prodID])
	}

	// 4. Mismatched payload with same key returns conflict
	mismatchedReq := model.CreateOrderRequest{
		Items: []model.OrderItemRequest{
			{ProductID: prodID, Quantity: 1}, // different quantity
		},
	}
	_, _, err = orderSvc.CreateOrder(context.Background(), userID, "idem-1", mismatchedReq)
	if !errors.Is(err, repository.ErrIdempotencyConflict) {
		t.Errorf("Expected ErrIdempotencyConflict on payload mismatch, got: %v", err)
	}

	// 5. Insufficient stock rejects order and rolls back
	excessiveReq := model.CreateOrderRequest{
		Items: []model.OrderItemRequest{
			{ProductID: prodID, Quantity: 100}, // only 3 available
		},
	}
	_, _, err = orderSvc.CreateOrder(context.Background(), userID, "idem-2", excessiveReq)
	if !errors.Is(err, repository.ErrInsufficientStock) {
		t.Errorf("Expected ErrInsufficientStock, got: %v", err)
	}
	// Stock remains 3
	if invRepo.stock[prodID] != 3 {
		t.Errorf("Stock changed unexpectedly after rejection: %d", invRepo.stock[prodID])
	}
}
