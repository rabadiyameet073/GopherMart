package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/gophermart/ecommerce-order-inventory/internal/model"
)

type OrderRepository interface {
	BeginTx(ctx context.Context) (pgx.Tx, error)
	CreateTx(ctx context.Context, tx pgx.Tx, order *model.Order, items []model.OrderItem) error
	GetByID(ctx context.Context, id, userID string) (*model.Order, error)
	ListByUser(ctx context.Context, userID string) ([]model.Order, error)
	UpdateStatus(ctx context.Context, id string, status model.OrderStatus) error
}

type pgOrderRepository struct {
	pool *pgxpool.Pool
}

func NewOrderRepository(pool *pgxpool.Pool) OrderRepository {
	return &pgOrderRepository{pool: pool}
}

func (r *pgOrderRepository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.pool.Begin(ctx)
}

func (r *pgOrderRepository) CreateTx(ctx context.Context, tx pgx.Tx, order *model.Order, items []model.OrderItem) error {
	orderQuery := `
		INSERT INTO orders (id, user_id, status, total_amount_cents, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
		RETURNING created_at, updated_at
	`
	err := tx.QueryRow(ctx, orderQuery, order.ID, order.UserID, order.Status, order.TotalAmountCents).
		Scan(&order.CreatedAt, &order.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to insert order: %w", err)
	}

	itemQuery := `
		INSERT INTO order_items (id, order_id, product_id, quantity, price_cents, created_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
	`
	for _, it := range items {
		_, err := tx.Exec(ctx, itemQuery, it.ID, order.ID, it.ProductID, it.Quantity, it.PriceCents)
		if err != nil {
			return fmt.Errorf("failed to insert order item: %w", err)
		}
	}

	return nil
}

func (r *pgOrderRepository) GetByID(ctx context.Context, id, userID string) (*model.Order, error) {
	orderQuery := `
		SELECT id, user_id, status, total_amount_cents, created_at, updated_at
		FROM orders
		WHERE id = $1 AND user_id = $2
	`
	var o model.Order
	err := r.pool.QueryRow(ctx, orderQuery, id, userID).Scan(
		&o.ID, &o.UserID, &o.Status, &o.TotalAmountCents, &o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get order: %w", err)
	}

	itemsQuery := `
		SELECT id, order_id, product_id, quantity, price_cents
		FROM order_items
		WHERE order_id = $1
	`
	rows, err := r.pool.Query(ctx, itemsQuery, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get order items: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var it model.OrderItem
		if err := rows.Scan(&it.ID, &it.OrderID, &it.ProductID, &it.Quantity, &it.PriceCents); err != nil {
			return nil, fmt.Errorf("failed to scan order item: %w", err)
		}
		o.Items = append(o.Items, it)
	}

	return &o, nil
}

func (r *pgOrderRepository) ListByUser(ctx context.Context, userID string) ([]model.Order, error) {
	query := `
		SELECT id, user_id, status, total_amount_cents, created_at, updated_at
		FROM orders
		WHERE user_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list orders: %w", err)
	}
	defer rows.Close()

	var orders []model.Order
	for rows.Next() {
		var o model.Order
		if err := rows.Scan(&o.ID, &o.UserID, &o.Status, &o.TotalAmountCents, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan order: %w", err)
		}
		orders = append(orders, o)
	}

	return orders, nil
}

func (r *pgOrderRepository) UpdateStatus(ctx context.Context, id string, status model.OrderStatus) error {
	query := `
		UPDATE orders
		SET status = $1, updated_at = NOW()
		WHERE id = $2
	`
	_, err := r.pool.Exec(ctx, query, status, id)
	if err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}
	return nil
}
