package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/gophermart/ecommerce-order-inventory/internal/model"
)

var (
	ErrInsufficientStock = errors.New("insufficient inventory stock for requested product")
	ErrProductNotFound   = errors.New("product inventory record not found")
)

type InventoryRepository interface {
	GetStock(ctx context.Context, productID string) (*model.Inventory, error)
	SetStock(ctx context.Context, productID string, quantity int) error
	DeductStockTx(ctx context.Context, tx pgx.Tx, productID string, quantityToDeduct int) error
}

type pgInventoryRepository struct {
	pool *pgxpool.Pool
}

func NewInventoryRepository(pool *pgxpool.Pool) InventoryRepository {
	return &pgInventoryRepository{pool: pool}
}

func (r *pgInventoryRepository) GetStock(ctx context.Context, productID string) (*model.Inventory, error) {
	query := `SELECT id, product_id, quantity, reserved_quantity, version, updated_at FROM inventory WHERE product_id = $1`
	var inv model.Inventory
	err := r.pool.QueryRow(ctx, query, productID).
		Scan(&inv.ID, &inv.ProductID, &inv.Quantity, &inv.ReservedQuantity, &inv.Version, &inv.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrProductNotFound
		}
		return nil, fmt.Errorf("failed to fetch stock: %w", err)
	}
	return &inv, nil
}

func (r *pgInventoryRepository) SetStock(ctx context.Context, productID string, quantity int) error {
	query := `
		INSERT INTO inventory (product_id, quantity, reserved_quantity, version, updated_at)
		VALUES ($1, $2, 0, 1, NOW())
		ON CONFLICT (product_id) DO UPDATE
		SET quantity = EXCLUDED.quantity, updated_at = NOW()
	`
	_, err := r.pool.Exec(ctx, query, productID, quantity)
	if err != nil {
		return fmt.Errorf("failed to update stock: %w", err)
	}
	return nil
}

// DeductStockTx locks the inventory row with SELECT ... FOR UPDATE, verifies available stock,
// and deducts quantity within the caller's active database transaction.
func (r *pgInventoryRepository) DeductStockTx(ctx context.Context, tx pgx.Tx, productID string, quantityToDeduct int) error {
	selectQuery := `
		SELECT quantity
		FROM inventory
		WHERE product_id = $1
		FOR UPDATE
	`
	var currentQuantity int
	err := tx.QueryRow(ctx, selectQuery, productID).Scan(&currentQuantity)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrProductNotFound
		}
		return fmt.Errorf("failed to acquire row lock for product %s: %w", productID, err)
	}

	if currentQuantity < quantityToDeduct {
		return fmt.Errorf("%w: requested %d, available %d", ErrInsufficientStock, quantityToDeduct, currentQuantity)
	}

	updateQuery := `
		UPDATE inventory
		SET quantity = quantity - $1,
		    updated_at = NOW()
		WHERE product_id = $2
	`
	cmdTag, err := tx.Exec(ctx, updateQuery, quantityToDeduct, productID)
	if err != nil {
		return fmt.Errorf("failed to deduct inventory: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		return errors.New("zero rows updated during inventory deduction")
	}

	return nil
}
