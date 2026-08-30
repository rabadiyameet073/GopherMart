package model

import "time"

type Product struct {
	ID          string    `json:"id"`
	SKU         string    `json:"sku"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	PriceCents  int       `json:"price_cents"`
	Category    string    `json:"category"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ProductWithStock struct {
	Product
	StockQuantity int `json:"stock_quantity"`
}

type CreateProductRequest struct {
	SKU           string `json:"sku"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	PriceCents    int    `json:"price_cents"`
	Category      string `json:"category"`
	InitialStock  int    `json:"initial_stock"`
}

type UpdateProductRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	PriceCents  *int    `json:"price_cents,omitempty"`
	Category    *string `json:"category,omitempty"`
}

type PaginatedProductsResponse struct {
	Items  []ProductWithStock `json:"items"`
	Total  int                `json:"total"`
	Limit  int                `json:"limit"`
	Offset int                `json:"offset"`
}
