package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gophermart/ecommerce-order-inventory/internal/model"
	"github.com/gophermart/ecommerce-order-inventory/internal/repository"
	"github.com/gophermart/ecommerce-order-inventory/internal/service"
	"github.com/gophermart/ecommerce-order-inventory/pkg/response"
)

type InventoryHandler struct {
	invService service.InventoryService
}

func NewInventoryHandler(invService service.InventoryService) *InventoryHandler {
	return &InventoryHandler{invService: invService}
}

func (h *InventoryHandler) GetStock(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	productID := vars["productId"]

	inv, err := h.invService.GetStock(r.Context(), productID)
	if err != nil {
		if errors.Is(err, repository.ErrProductNotFound) {
			response.Error(w, http.StatusNotFound, "NOT_FOUND", "inventory item not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, "INVENTORY_ERROR", err.Error())
		return
	}

	response.JSON(w, http.StatusOK, inv)
}

func (h *InventoryHandler) SetStock(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	productID := vars["productId"]

	var body struct {
		Quantity int `json:"quantity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_BODY", "failed to decode json")
		return
	}

	req := model.UpdateStockRequest{
		ProductID: productID,
		Quantity:  body.Quantity,
	}

	if err := h.invService.SetStock(r.Context(), req); err != nil {
		response.Error(w, http.StatusBadRequest, "UPDATE_FAILED", err.Error())
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{
		"message": "stock updated successfully",
	})
}
