package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	AppEnv          string
	Port            string
	LogLevel        string
	PostgresURL     string
	RedisAddr       string
	RedisPass       string
	RedisDB         int
	JWTSecret       string
	JWTIssuer       string
	JWTExpiry       time.Duration
	CartTTL         time.Duration
	RateLimit       int // requests per second
	RateBurst       int // maximum bucket capacity
	WorkerCount     int
	WorkerQueue     int
	WorkerRetry     int
	ShutdownTimeout time.Duration
	MaxBodyBytes    int64
}

func Load() *Config {
	return &Config{
		AppEnv:          getEnv("APP_ENV", "development"),
		Port:            getEnv("PORT", getEnv("HTTP_ADDR", "8080")),
		LogLevel:        getEnv("LOG_LEVEL", "info"),
		PostgresURL:     getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/gophermart?sslmode=disable"),
		RedisAddr:       getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPass:       getEnv("REDIS_PASSWORD", ""),
		RedisDB:         getEnvAsInt("REDIS_DB", 0),
		JWTSecret:       getEnv("JWT_SECRET", "super-secret-gophermart-jwt-key-2026"),
		JWTIssuer:       getEnv("JWT_ISSUER", "gophermart"),
		JWTExpiry:       time.Duration(getEnvAsInt("JWT_EXPIRY_HOURS", 24)) * time.Hour,
		CartTTL:         time.Duration(getEnvAsInt("CART_TTL_HOURS", 24)) * time.Hour,
		RateLimit:       getEnvAsInt("RATE_LIMIT_RPS", 20),
		RateBurst:       getEnvAsInt("RATE_LIMIT_BURST", 40),
		WorkerCount:     getEnvAsInt("WORKER_POOL_SIZE", getEnvAsInt("WORKER_COUNT", 5)),
		WorkerQueue:     getEnvAsInt("WORKER_QUEUE_SIZE", 100),
		WorkerRetry:     getEnvAsInt("WORKER_MAX_RETRIES", 3),
		ShutdownTimeout: time.Duration(getEnvAsInt("SHUTDOWN_TIMEOUT_SECONDS", 15)) * time.Second,
		MaxBodyBytes:    int64(getEnvAsInt("MAX_BODY_BYTES", 1048576)), // 1 MB default
	}
}

func getEnv(key, defaultVal string) string {
	if val, exists := os.LookupEnv(key); exists && val != "" {
		return val
	}
	return defaultVal
}

func getEnvAsInt(key string, defaultVal int) int {
	valStr := getEnv(key, "")
	if val, err := strconv.Atoi(valStr); err == nil {
		return val
	}
	return defaultVal
}
