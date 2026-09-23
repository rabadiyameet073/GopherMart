package tests

import (
	"context"
	"testing"
	"time"

	"github.com/gophermart/ecommerce-order-inventory/internal/model"
	"github.com/gophermart/ecommerce-order-inventory/internal/service"
)

type mockCartRepo struct {
	carts map[string]*model.Cart
}

func (m *mockCartRepo) GetCart(ctx context.Context, userID string) (*model.Cart, error) {
	if c, ok := m.carts[userID]; ok {
		return c, nil
	}
	return &model.Cart{UserID: userID, Items: []model.CartItem{}}, nil
}

func (m *mockCartRepo) SaveCart(ctx context.Context, cart *model.Cart, ttl time.Duration) error {
	m.carts[cart.UserID] = cart
	return nil
}

func (m *mockCartRepo) ClearCart(ctx context.Context, userID string) error {
	delete(m.carts, userID)
	return nil
}

func TestCartService_Operations(t *testing.T) {
	repo := &mockCartRepo{carts: make(map[string]*model.Cart)}
	cartSvc := service.NewCartService(repo)
	userID := "user-cart-test-123"

	// 1. Initial Cart Empty
	cart, err := cartSvc.GetCart(context.Background(), userID)
	if err != nil {
		t.Fatalf("GetCart failed: %v", err)
	}
	if len(cart.Items) != 0 {
		t.Errorf("Expected 0 items, got %d", len(cart.Items))
	}

	// 2. Add Item
	addReq := model.AddCartItemRequest{
		ProductID: "prod-milk-001",
		Quantity:  2,
	}
	cart, err = cartSvc.AddItem(context.Background(), userID, addReq)
	if err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}
	if len(cart.Items) != 1 || cart.Items[0].Quantity != 2 {
		t.Errorf("Expected 1 item with quantity 2, got %+v", cart.Items)
	}

	// 3. Increment Quantity on Add Item
	cart, err = cartSvc.AddItem(context.Background(), userID, addReq)
	if err != nil {
		t.Fatalf("AddItem 2 failed: %v", err)
	}
	if len(cart.Items) != 1 || cart.Items[0].Quantity != 4 {
		t.Errorf("Expected quantity 4 after adding existing item, got %d", cart.Items[0].Quantity)
	}

	// 4. Update Item Quantity
	updateReq := model.UpdateCartItemRequest{
		Quantity: 5,
	}
	cart, err = cartSvc.UpdateItem(context.Background(), userID, "prod-milk-001", updateReq)
	if err != nil {
		t.Fatalf("UpdateItem failed: %v", err)
	}
	if cart.Items[0].Quantity != 5 {
		t.Errorf("Expected quantity 5, got %d", cart.Items[0].Quantity)
	}

	// 5. Remove Item
	cart, err = cartSvc.RemoveItem(context.Background(), userID, "prod-milk-001")
	if err != nil {
		t.Fatalf("RemoveItem failed: %v", err)
	}
	if len(cart.Items) != 0 {
		t.Errorf("Expected empty cart after item removal, got %d items", len(cart.Items))
	}

	// 6. Clear Cart
	_, _ = cartSvc.AddItem(context.Background(), userID, addReq)
	if err := cartSvc.ClearCart(context.Background(), userID); err != nil {
		t.Fatalf("ClearCart failed: %v", err)
	}
	cart, _ = cartSvc.GetCart(context.Background(), userID)
	if len(cart.Items) != 0 {
		t.Errorf("Expected 0 items after ClearCart, got %d", len(cart.Items))
	}
}
