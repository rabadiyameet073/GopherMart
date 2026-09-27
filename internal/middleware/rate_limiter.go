package middleware

import (
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gophermart/ecommerce-order-inventory/internal/repository"
	"github.com/gophermart/ecommerce-order-inventory/pkg/logger"
	"github.com/gophermart/ecommerce-order-inventory/pkg/response"
)

func RateLimitMiddleware(redisRepo repository.RedisRepository, rate, burst int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var identifier string

			// Use user ID if authenticated, else check Authorization header, else IP address
			if userID, ok := GetUserIDFromContext(r.Context()); ok && userID != "" {
				identifier = "usr:" + userID
			} else if authHeader := r.Header.Get("Authorization"); authHeader != "" {
				parts := strings.Split(authHeader, " ")
				if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
					claims := &JWTClaims{}
					parser := jwt.NewParser()
					if _, _, err := parser.ParseUnverified(parts[1], claims); err == nil && claims.UserID != "" {
						identifier = "usr:" + claims.UserID
					}
				}
			}

			if identifier == "" {
				ip := extractIP(r)
				identifier = "ip:" + ip
			}

			allowed, remaining, err := redisRepo.AllowRateLimit(r.Context(), identifier, rate, burst)
			if err != nil {
				logger.Error("Rate limiter failure, allowing request as fail-open", "identifier", identifier, "error", err)
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", burst))
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))

			if !allowed {
				w.Header().Set("Retry-After", "1")
				response.Error(w, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", "rate limit exceeded; try again shortly")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func extractIP(r *http.Request) string {
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		parts := strings.Split(forwarded, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

