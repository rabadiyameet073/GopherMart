package model

import "time"

type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "PENDING"
	OrderStatusConfirmed OrderStatus = "CONFIRMED"
	OrderStatusPicked    OrderStatus = "PICKED"
	OrderStatusDispatched OrderStatus = "DISPATCHED"
	OrderStatusFailed    OrderStatus = "FAILED"
)

type Order struct {
	ID               string      `json:"id"`
	UserID           string      `json:"user_id"`
	Status           OrderStatus `json:"status"`
	TotalAmountCents int         `json:"total_amount_cents"`
	Items            []OrderItem `json:"items,omitempty"`
	CreatedAt        time.Time   `json:"created_at"`
	UpdatedAt        time.Time   `json:"updated_at"`
}

type OrderItem struct {
	ID         string `json:"id"`
	OrderID    string `json:"order_id"`
	ProductID  string `json:"product_id"`
	Quantity   int    `json:"quantity"`
	PriceCents int    `json:"price_cents"`
}

type OrderItemRequest struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
}

type CreateOrderRequest struct {
	Items []OrderItemRequest `json:"items"`
}

type IdempotencyRecord struct {
	Key                string    `json:"key"`
	UserID             string    `json:"user_id"`
	RequestHash        string    `json:"request_hash"`
	Status             string    `json:"status"` // PROCESSING, COMPLETED, FAILED
	ResponseStatusCode int       `json:"response_status_code"`
	ResponseBody       string    `json:"response_body"`
	CreatedAt          time.Time `json:"created_at"`
	ExpiresAt          time.Time `json:"expires_at"`
}

type OrderEvent struct {
	OrderID    string      `json:"order_id"`
	UserID     string      `json:"user_id"`
	Status     OrderStatus `json:"status"`
	Timestamp  time.Time   `json:"timestamp"`
	RetryCount int         `json:"retry_count"`
}
