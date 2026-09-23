package service

import (
	"context"
	"errors"
	"time"

	"github.com/gophermart/ecommerce-order-inventory/internal/model"
	"github.com/gophermart/ecommerce-order-inventory/internal/repository"
	"github.com/gophermart/ecommerce-order-inventory/pkg/validator"
)

var (
	ErrCartItemNotFound = errors.New("product not found in cart")
)

type CartService interface {
	GetCart(ctx context.Context, userID string) (*model.Cart, error)
	AddItem(ctx context.Context, userID string, req model.AddCartItemRequest) (*model.Cart, error)
	UpdateItem(ctx context.Context, userID, productID string, req model.UpdateCartItemRequest) (*model.Cart, error)
	RemoveItem(ctx context.Context, userID, productID string) (*model.Cart, error)
	ClearCart(ctx context.Context, userID string) error
}

type cartServiceImpl struct {
	cartRepo repository.CartRepository
	ttl      time.Duration
}

func NewCartService(cartRepo repository.CartRepository) CartService {
	return &cartServiceImpl{
		cartRepo: cartRepo,
		ttl:      24 * time.Hour,
	}
}

func (s *cartServiceImpl) GetCart(ctx context.Context, userID string) (*model.Cart, error) {
	return s.cartRepo.GetCart(ctx, userID)
}

func (s *cartServiceImpl) AddItem(ctx context.Context, userID string, req model.AddCartItemRequest) (*model.Cart, error) {
	if err := validator.ValidateRequired("product_id", req.ProductID); err != nil {
		return nil, err
	}
	if err := validator.ValidatePositive("quantity", req.Quantity); err != nil {
		return nil, err
	}

	cart, err := s.cartRepo.GetCart(ctx, userID)
	if err != nil {
		return nil, err
	}

	found := false
	for i := range cart.Items {
		if cart.Items[i].ProductID == req.ProductID {
			cart.Items[i].Quantity += req.Quantity
			found = true
			break
		}
	}

	if !found {
		cart.Items = append(cart.Items, model.CartItem{
			ProductID: req.ProductID,
			Quantity:  req.Quantity,
		})
	}

	if err := s.cartRepo.SaveCart(ctx, cart, s.ttl); err != nil {
		return nil, err
	}

	return cart, nil
}

func (s *cartServiceImpl) UpdateItem(ctx context.Context, userID, productID string, req model.UpdateCartItemRequest) (*model.Cart, error) {
	if err := validator.ValidatePositive("quantity", req.Quantity); err != nil {
		return nil, err
	}

	cart, err := s.cartRepo.GetCart(ctx, userID)
	if err != nil {
		return nil, err
	}

	found := false
	for i := range cart.Items {
		if cart.Items[i].ProductID == productID {
			cart.Items[i].Quantity = req.Quantity
			found = true
			break
		}
	}

	if !found {
		return nil, ErrCartItemNotFound
	}

	if err := s.cartRepo.SaveCart(ctx, cart, s.ttl); err != nil {
		return nil, err
	}

	return cart, nil
}

func (s *cartServiceImpl) RemoveItem(ctx context.Context, userID, productID string) (*model.Cart, error) {
	cart, err := s.cartRepo.GetCart(ctx, userID)
	if err != nil {
		return nil, err
	}

	var newItems []model.CartItem
	found := false
	for _, it := range cart.Items {
		if it.ProductID == productID {
			found = true
			continue
		}
		newItems = append(newItems, it)
	}

	if !found {
		return nil, ErrCartItemNotFound
	}

	cart.Items = newItems
	if err := s.cartRepo.SaveCart(ctx, cart, s.ttl); err != nil {
		return nil, err
	}

	return cart, nil
}

func (s *cartServiceImpl) ClearCart(ctx context.Context, userID string) error {
	return s.cartRepo.ClearCart(ctx, userID)
}
