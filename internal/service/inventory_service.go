package service

import (
	"context"

	"github.com/gophermart/ecommerce-order-inventory/internal/model"
	"github.com/gophermart/ecommerce-order-inventory/internal/repository"
	"github.com/gophermart/ecommerce-order-inventory/pkg/validator"
)

type InventoryService interface {
	GetStock(ctx context.Context, productID string) (*model.Inventory, error)
	SetStock(ctx context.Context, req model.UpdateStockRequest) error
}

type inventoryServiceImpl struct {
	invRepo repository.InventoryRepository
}

func NewInventoryService(invRepo repository.InventoryRepository) InventoryService {
	return &inventoryServiceImpl{invRepo: invRepo}
}

func (s *inventoryServiceImpl) GetStock(ctx context.Context, productID string) (*model.Inventory, error) {
	if err := validator.ValidateRequired("product_id", productID); err != nil {
		return nil, err
	}
	return s.invRepo.GetStock(ctx, productID)
}

func (s *inventoryServiceImpl) SetStock(ctx context.Context, req model.UpdateStockRequest) error {
	if err := validator.ValidateRequired("product_id", req.ProductID); err != nil {
		return err
	}
	if req.Quantity < 0 {
		return validator.ValidatePositive("quantity", req.Quantity)
	}
	return s.invRepo.SetStock(ctx, req.ProductID, req.Quantity)
}

