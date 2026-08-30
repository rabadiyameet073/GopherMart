package model

import "time"

type Inventory struct {
	ID               string    `json:"id"`
	ProductID        string    `json:"product_id"`
	Quantity         int       `json:"quantity"`
	ReservedQuantity int       `json:"reserved_quantity"`
	Version          int       `json:"version"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type UpdateStockRequest struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
}
