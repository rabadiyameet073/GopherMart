package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gophermart/ecommerce-order-inventory/internal/middleware"
	"github.com/gophermart/ecommerce-order-inventory/internal/model"
	"github.com/gophermart/ecommerce-order-inventory/internal/service"
	"github.com/gophermart/ecommerce-order-inventory/pkg/response"
)

type CartHandler struct {
	cartService service.CartService
}

func NewCartHandler(cartService service.CartService) *CartHandler {
	return &CartHandler{cartService: cartService}
}

func (h *CartHandler) GetCart(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok || userID == "" {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized user")
		return
	}

	cart, err := h.cartService.GetCart(r.Context(), userID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "CART_ERROR", err.Error())
		return
	}

	response.JSON(w, http.StatusOK, cart)
}

func (h *CartHandler) AddItem(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok || userID == "" {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized user")
		return
	}

	var req model.AddCartItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_BODY", "failed to parse request json")
		return
	}

	cart, err := h.cartService.AddItem(r.Context(), userID, req)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "CART_ERROR", err.Error())
		return
	}

	response.JSON(w, http.StatusOK, cart)
}

func (h *CartHandler) UpdateItem(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok || userID == "" {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized user")
		return
	}

	vars := mux.Vars(r)
	productID := vars["productId"]

	var req model.UpdateCartItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_BODY", "failed to parse request json")
		return
	}

	cart, err := h.cartService.UpdateItem(r.Context(), userID, productID, req)
	if err != nil {
		if errors.Is(err, service.ErrCartItemNotFound) {
			response.Error(w, http.StatusNotFound, "NOT_FOUND", "item not in cart")
			return
		}
		response.Error(w, http.StatusBadRequest, "CART_ERROR", err.Error())
		return
	}

	response.JSON(w, http.StatusOK, cart)
}

func (h *CartHandler) RemoveItem(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok || userID == "" {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized user")
		return
	}

	vars := mux.Vars(r)
	productID := vars["productId"]

	cart, err := h.cartService.RemoveItem(r.Context(), userID, productID)
	if err != nil {
		if errors.Is(err, service.ErrCartItemNotFound) {
			response.Error(w, http.StatusNotFound, "NOT_FOUND", "item not in cart")
			return
		}
		response.Error(w, http.StatusBadRequest, "CART_ERROR", err.Error())
		return
	}

	response.JSON(w, http.StatusOK, cart)
}

func (h *CartHandler) ClearCart(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok || userID == "" {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized user")
		return
	}

	if err := h.cartService.ClearCart(r.Context(), userID); err != nil {
		response.Error(w, http.StatusInternalServerError, "CART_ERROR", err.Error())
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{
		"message": "cart cleared successfully",
	})
}
