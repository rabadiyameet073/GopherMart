package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gophermart/ecommerce-order-inventory/internal/model"
	"github.com/gophermart/ecommerce-order-inventory/internal/repository"
	"github.com/gophermart/ecommerce-order-inventory/pkg/response"
	"github.com/gophermart/ecommerce-order-inventory/pkg/validator"
)

type ProductHandler struct {
	productRepo repository.ProductRepository
}

func NewProductHandler(productRepo repository.ProductRepository) *ProductHandler {
	return &ProductHandler{productRepo: productRepo}
}

func (h *ProductHandler) List(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 20
	offset := 0

	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		limit = l
	}
	if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
		offset = o
	}

	res, err := h.productRepo.List(r.Context(), category, limit, offset)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	response.JSON(w, http.StatusOK, res)
}

func (h *ProductHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	product, err := h.productRepo.GetByID(r.Context(), id)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if product == nil {
		response.Error(w, http.StatusNotFound, "NOT_FOUND", "product not found")
		return
	}

	response.JSON(w, http.StatusOK, product)
}

func (h *ProductHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.CreateProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_BODY", "failed to parse request json")
		return
	}

	if err := validator.ValidateRequired("name", req.Name); err != nil {
		response.Error(w, http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
		return
	}
	if err := validator.ValidateRequired("sku", req.SKU); err != nil {
		response.Error(w, http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
		return
	}
	if err := validator.ValidatePositive("price_cents", req.PriceCents); err != nil {
		response.Error(w, http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
		return
	}

	product := &model.Product{
		ID:          uuid.New().String(),
		SKU:         req.SKU,
		Name:        req.Name,
		Description: req.Description,
		PriceCents:  req.PriceCents,
		Category:    req.Category,
	}

	if err := h.productRepo.Create(r.Context(), product, req.InitialStock); err != nil {
		response.Error(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	response.JSON(w, http.StatusCreated, product)
}

func (h *ProductHandler) Update(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var req model.UpdateProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_BODY", "failed to parse request json")
		return
	}

	product, err := h.productRepo.Update(r.Context(), id, req)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "UPDATE_FAILED", err.Error())
		return
	}

	response.JSON(w, http.StatusOK, product)
}
