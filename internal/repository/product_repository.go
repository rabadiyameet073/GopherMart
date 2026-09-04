package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/gophermart/ecommerce-order-inventory/internal/model"
)

type ProductRepository interface {
	List(ctx context.Context, category string, limit, offset int) (*model.PaginatedProductsResponse, error)
	GetByID(ctx context.Context, id string) (*model.ProductWithStock, error)
	GetByIDs(ctx context.Context, ids []string) (map[string]model.Product, error)
	Create(ctx context.Context, p *model.Product, initialStock int) error
	Update(ctx context.Context, id string, req model.UpdateProductRequest) (*model.ProductWithStock, error)
}

type pgProductRepository struct {
	pool *pgxpool.Pool
}

func NewProductRepository(pool *pgxpool.Pool) ProductRepository {
	return &pgProductRepository{pool: pool}
}

func (r *pgProductRepository) List(ctx context.Context, category string, limit, offset int) (*model.PaginatedProductsResponse, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	var countQuery string
	var query string
	var total int

	if category != "" {
		countQuery = `SELECT COUNT(*) FROM products WHERE category = $1`
		if err := r.pool.QueryRow(ctx, countQuery, category).Scan(&total); err != nil {
			return nil, fmt.Errorf("failed to count products: %w", err)
		}

		query = `
			SELECT p.id, p.sku, p.name, p.description, p.price_cents, p.category, p.created_at, p.updated_at,
			       COALESCE(i.quantity, 0) as stock_quantity
			FROM products p
			LEFT JOIN inventory i ON p.id = i.product_id
			WHERE p.category = $1
			ORDER BY p.name ASC
			LIMIT $2 OFFSET $3
		`
		rows, err := r.pool.Query(ctx, query, category, limit, offset)
		if err != nil {
			return nil, fmt.Errorf("failed to query products: %w", err)
		}
		defer rows.Close()

		items, err := scanProductsWithStock(rows)
		if err != nil {
			return nil, err
		}
		return &model.PaginatedProductsResponse{
			Items:  items,
			Total:  total,
			Limit:  limit,
			Offset: offset,
		}, nil
	}

	countQuery = `SELECT COUNT(*) FROM products`
	if err := r.pool.QueryRow(ctx, countQuery).Scan(&total); err != nil {
		return nil, fmt.Errorf("failed to count products: %w", err)
	}

	query = `
		SELECT p.id, p.sku, p.name, p.description, p.price_cents, p.category, p.created_at, p.updated_at,
		       COALESCE(i.quantity, 0) as stock_quantity
		FROM products p
		LEFT JOIN inventory i ON p.id = i.product_id
		ORDER BY p.name ASC
		LIMIT $1 OFFSET $2
	`
	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query products: %w", err)
	}
	defer rows.Close()

	items, err := scanProductsWithStock(rows)
	if err != nil {
		return nil, err
	}

	return &model.PaginatedProductsResponse{
		Items:  items,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}, nil
}

func scanProductsWithStock(rows pgx.Rows) ([]model.ProductWithStock, error) {
	var products []model.ProductWithStock
	for rows.Next() {
		var p model.ProductWithStock
		err := rows.Scan(
			&p.ID, &p.SKU, &p.Name, &p.Description, &p.PriceCents, &p.Category, &p.CreatedAt, &p.UpdatedAt,
			&p.StockQuantity,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan product: %w", err)
		}
		products = append(products, p)
	}
	return products, nil
}

func (r *pgProductRepository) GetByID(ctx context.Context, id string) (*model.ProductWithStock, error) {
	query := `
		SELECT p.id, p.sku, p.name, p.description, p.price_cents, p.category, p.created_at, p.updated_at,
		       COALESCE(i.quantity, 0) as stock_quantity
		FROM products p
		LEFT JOIN inventory i ON p.id = i.product_id
		WHERE p.id = $1
	`
	var p model.ProductWithStock
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&p.ID, &p.SKU, &p.Name, &p.Description, &p.PriceCents, &p.Category, &p.CreatedAt, &p.UpdatedAt,
		&p.StockQuantity,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get product by id: %w", err)
	}
	return &p, nil
}

func (r *pgProductRepository) GetByIDs(ctx context.Context, ids []string) (map[string]model.Product, error) {
	query := `
		SELECT id, sku, name, description, price_cents, category, created_at, updated_at
		FROM products
		WHERE id = ANY($1)
	`
	rows, err := r.pool.Query(ctx, query, ids)
	if err != nil {
		return nil, fmt.Errorf("failed to get products by ids: %w", err)
	}
	defer rows.Close()

	products := make(map[string]model.Product)
	for rows.Next() {
		var p model.Product
		if err := rows.Scan(&p.ID, &p.SKU, &p.Name, &p.Description, &p.PriceCents, &p.Category, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan product: %w", err)
		}
		products[p.ID] = p
	}

	return products, nil
}

func (r *pgProductRepository) Create(ctx context.Context, p *model.Product, initialStock int) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	insertProduct := `
		INSERT INTO products (id, sku, name, description, price_cents, category, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		RETURNING created_at, updated_at
	`
	err = tx.QueryRow(ctx, insertProduct, p.ID, p.SKU, p.Name, p.Description, p.PriceCents, p.Category).
		Scan(&p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to insert product: %w", err)
	}

	insertInventory := `
		INSERT INTO inventory (product_id, quantity, reserved_quantity, version, updated_at)
		VALUES ($1, $2, 0, 1, NOW())
	`
	_, err = tx.Exec(ctx, insertInventory, p.ID, initialStock)
	if err != nil {
		return fmt.Errorf("failed to insert inventory: %w", err)
	}

	return tx.Commit(ctx)
}

func (r *pgProductRepository) Update(ctx context.Context, id string, req model.UpdateProductRequest) (*model.ProductWithStock, error) {
	current, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, errors.New("product not found")
	}

	name := current.Name
	if req.Name != nil {
		name = *req.Name
	}
	desc := current.Description
	if req.Description != nil {
		desc = *req.Description
	}
	price := current.PriceCents
	if req.PriceCents != nil {
		price = *req.PriceCents
	}
	category := current.Category
	if req.Category != nil {
		category = *req.Category
	}

	updateQuery := `
		UPDATE products
		SET name = $1, description = $2, price_cents = $3, category = $4, updated_at = NOW()
		WHERE id = $5
		RETURNING updated_at
	`
	var updatedAt pgx.Rows
	_ = updatedAt
	err = r.pool.QueryRow(ctx, updateQuery, name, desc, price, category, id).Scan(&current.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to update product: %w", err)
	}

	current.Name = name
	current.Description = desc
	current.PriceCents = price
	current.Category = category
	return current, nil
}

