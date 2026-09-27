package middleware

import (
	"fmt"
	"net/http"

	"github.com/gophermart/ecommerce-order-inventory/pkg/logger"
	"github.com/gophermart/ecommerce-order-inventory/pkg/response"
)

func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				logger.Error("Server Panic Recovered", "panic", fmt.Sprintf("%v", rec))
				response.Error(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "an unexpected error occurred")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

