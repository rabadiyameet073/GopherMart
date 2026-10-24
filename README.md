# GopherMart – ECommerce Order & Inventory Service

> **Dark-Store Quick Commerce Backend** built with **Go**, **PostgreSQL**, **Redis**, and **Docker**.

---

## 🚀 Key Highlights & Architectural Achievements

- **Layered Clean Architecture**: Strict separation of concerns across handlers, services, and repositories, secured with JWT authentication, Redis token-bucket rate limiting, input validation, and structured JSON logging (`log/slog`).
- **Zero Overselling with PostgreSQL Row-Level Locking**: Concurrent stock deductions lock inventory records within transactions using `SELECT quantity FROM inventory WHERE product_id = $1 FOR UPDATE`. Verified with concurrent tests under the Go race detector (`go test -race`).
- **Idempotent Order Creation**: Enforces `Idempotency-Key` HTTP header with canonical SHA-256 request payload hashing to safely handle network timeouts, mobile retries, and duplicate submissions without double-charging stock.
- **Bounded Goroutine Worker Pool**: Asynchronous order events (dark-store picking dispatch, customer notifications) are queued through a fixed-size worker pool with per-job context timeouts, exponential backoff retries, and graceful termination.
- **Verification & Stress Testing**: Comprehensive test suite including table-driven tests, Go race detector verification, and k6 load test scripts simulating high-concurrency order placement.

---

## 📁 Repository Structure

```text
gophermart/
├── cmd/
│   └── server/
│       └── main.go                 # Application entrypoint & graceful shutdown
├── internal/
│   ├── config/                     # Environment configuration loader
│   ├── handler/                    # REST HTTP handlers & Gorilla Mux router
│   ├── middleware/                 # JWT Auth, Token-Bucket Rate Limiter, Logger, Recovery
│   ├── model/                      # Domain entities & DTO contracts
│   ├── repository/                 # PostgreSQL & Redis data access layer
│   ├── service/                    # Business logic & transactional orchestration
│   └── worker/                     # Bounded goroutine worker pool with exponential backoff
├── migrations/
│   ├── 000001_init_schema.up.sql   # PostgreSQL schema with constraints & indexes
│   ├── 000001_init_schema.down.sql # Reversible teardown migration
│   └── seeds/
│       └── 001_initial_dark_store_data.sql # Seed products and stock
├── pkg/
│   ├── database/                   # pgxpool & redis client factories
│   ├── logger/                     # Structured JSON logging with slog
│   ├── response/                   # Standardized JSON response envelopes
│   └── validator/                  # Request payload validation rules
├── tests/
│   ├── concurrent_order_test.go    # Go race detector & concurrency tests
│   └── k6/
│       ├── load_test.js            # General k6 load test
│       └── overselling_stress_test.js # High-concurrency flash sale test
├── Dockerfile                      # Production multi-stage Alpine build
├── docker-compose.yml              # PostgreSQL 16, Redis 7, API, and k6 stack
├── Makefile                        # Automation shortcuts
└── go.mod                          # Go module dependencies
```

---

## 🛠️ Getting Started

### 1. Prerequisites
- **Go**: 1.23+ (`go version`)
- **Docker & Docker Compose** (or local PostgreSQL & Redis)

### 2. Configuration
Copy the sample environment file:
```bash
cp .env.example .env
```

### 3. Run with Docker Compose
To launch PostgreSQL, Redis, and the GopherMart API service together:
```bash
docker-compose up -d --build
```
The database will automatically initialize schemas and seed sample dark-store items.

### 4. Run Locally
```bash
# Download dependencies
go mod tidy

# Run server
go run cmd/server/main.go
```

The API will be live on `http://localhost:8080`.

---

## 🧪 Testing & Verification

### Run Table-Driven & Race Detector Tests
Verify zero data races and concurrency safety:
```bash
go test -race -v ./tests/... ./internal/worker/...
```

### Run k6 High-Concurrency Stress Test
Simulates 50 concurrent virtual users competing for 10 units of inventory:
```bash
docker-compose run --rm k6 run /scripts/overselling_stress_test.js
```

---

## 📡 API Endpoints

### Public Endpoints
- `GET /health` - Liveness health check
- `GET /ready` - Readiness check (validates PostgreSQL & Redis connectivity)
- `POST /api/v1/auth/register` - Register a customer account
- `POST /api/v1/auth/login` - Authenticate and retrieve JWT token
- `GET /api/v1/products` - Browse catalog (with stock status)
- `GET /api/v1/products/{id}` - Get product details

### Protected Endpoints (`Authorization: Bearer <token>`)
- `POST /api/v1/orders` - Place order (**requires `Idempotency-Key` header**)
- `GET /api/v1/orders` - List user orders
- `GET /api/v1/orders/{id}` - Fetch order details by ID
- `GET /api/v1/inventory/{productId}` - View warehouse inventory
- `PUT /api/v1/inventory/{productId}` - Adjust stock quantity

---

## 👤 Author & Architecture

- **Author**: Meet Rabadiya ([@rabadiyameet073](https://github.com/rabadiyameet073))
- **Email**: `rabadiyameet09@gmail.com`
- **License**: MIT License

---

## 🧪 Verification & Engineering Standards

- **Zero Race Conditions**: All code is rigorously tested with the Go race detector (`go test -race ./...`).
- **Idempotency Guarantee**: Order submission endpoints enforce idempotency key tracking to guarantee at-most-once processing.
- **ACID Transactions**: Inventory deductions strictly utilize PostgreSQL row-level locks (`SELECT ... FOR UPDATE`) within transactions to mathematically prevent negative stock.
