package tests

import (
	"testing"

	"github.com/gophermart/ecommerce-order-inventory/internal/model"
	"github.com/gophermart/ecommerce-order-inventory/pkg/validator"
)

func TestProductValidation(t *testing.T) {
	tests := []struct {
		name        string
		req         model.CreateProductRequest
		expectError bool
	}{
		{
			name: "Valid Product",
			req: model.CreateProductRequest{
				SKU:          "SKU-TEST-001",
				Name:         "Fresh Strawberries",
				Description:  "Organic sweet strawberries (250g)",
				PriceCents:   399,
				Category:     "Produce",
				InitialStock: 50,
			},
			expectError: false,
		},
		{
			name: "Missing SKU",
			req: model.CreateProductRequest{
				SKU:          "",
				Name:         "Fresh Strawberries",
				PriceCents:   399,
				InitialStock: 50,
			},
			expectError: true,
		},
		{
			name: "Missing Name",
			req: model.CreateProductRequest{
				SKU:          "SKU-TEST-002",
				Name:         "",
				PriceCents:   399,
				InitialStock: 50,
			},
			expectError: true,
		},
		{
			name: "Negative or Zero Price",
			req: model.CreateProductRequest{
				SKU:          "SKU-TEST-003",
				Name:         "Free Product",
				PriceCents:   0,
				InitialStock: 50,
			},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validator.ValidateRequired("name", tc.req.Name)
			if err == nil {
				err = validator.ValidateRequired("sku", tc.req.SKU)
			}
			if err == nil {
				err = validator.ValidatePositive("price_cents", tc.req.PriceCents)
			}

			if tc.expectError && err == nil {
				t.Errorf("Expected validation error for %s, got nil", tc.name)
			}
			if !tc.expectError && err != nil {
				t.Errorf("Expected valid product for %s, got error: %v", tc.name, err)
			}
		})
	}
}
