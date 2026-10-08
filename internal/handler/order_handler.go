package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gophermart/ecommerce-order-inventory/internal/middleware"
	"github.com/gophermart/ecommerce-order-inventory/internal/model"
	"github.com/gophermart/ecommerce-order-inventory/internal/repository"
	"github.com/gophermart/ecommerce-order-inventory/internal/service"
	"github.com/gophermart/ecommerce-order-inventory/pkg/response"
)

type OrderHandler struct {
	orderService service.OrderService
}

func NewOrderHandler(orderService service.OrderService) *OrderHandler {
	return &OrderHandler{orderService: orderService}
}

func (h *OrderHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok || userID == "" {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized user")
		return
	}

	idempotencyKey := r.Header.Get("Idempotency-Key")

	var req model.CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_BODY", "failed to parse request json")
		return
	}

	order, isReplay, err := h.orderService.CreateOrder(r.Context(), userID, idempotencyKey, req)
	if err != nil {
		if errors.Is(err, repository.ErrInsufficientStock) {
			response.Error(w, http.StatusConflict, "INSUFFICIENT_STOCK", err.Error())
			return
		}
		if errors.Is(err, repository.ErrDuplicateInFlight) {
			response.Error(w, http.StatusConflict, "IN_FLIGHT_REQUEST", err.Error())
			return
		}
		if errors.Is(err, repository.ErrIdempotencyConflict) {
			response.Error(w, http.StatusUnprocessableEntity, "IDEMPOTENCY_CONFLICT", err.Error())
			return
		}
		response.Error(w, http.StatusBadRequest, "ORDER_CREATION_FAILED", err.Error())
		return
	}

	if isReplay {
		w.Header().Set("X-Cache-Lookup", "HIT-IDEMPOTENT")
	}

	response.JSON(w, http.StatusCreated, order)
}

func (h *OrderHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserIDFromContext(r.Context())
	vars := mux.Vars(r)
	orderID := vars["id"]

	order, err := h.orderService.GetOrderByID(r.Context(), orderID, userID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if order == nil {
		response.Error(w, http.StatusNotFound, "NOT_FOUND", "order not found")
		return
	}

	response.JSON(w, http.StatusOK, order)
}

func (h *OrderHandler) ListOrders(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserIDFromContext(r.Context())

	orders, err := h.orderService.ListUserOrders(r.Context(), userID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	response.JSON(w, http.StatusOK, orders)
}
