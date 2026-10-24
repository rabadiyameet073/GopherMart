# GopherMart — Chronological Development Log

This document records the engineering progress, architectural decisions, and testing milestones for **GopherMart** (Quick-Commerce Order & Inventory Backend in Go).

---

## Milestone 1: Environment Setup & Project Scaffolding
- Initialized Go module `github.com/gophermart/ecommerce-order-inventory` (Go 1.23+).
- Formulated layered clean architecture with clear separation of concerns:
  - `cmd/server/`: Application entrypoint, graceful shutdown.
  - `internal/handler/`: HTTP routing and standard JSON responses.
  - `internal/service/`: Business rules, transaction boundaries, idempotency, event dispatch.
  - `internal/repository/`: Direct PostgreSQL `pgxpool` queries and Redis commands.
  - `internal/model/`: Domain models and DTO contracts.
  - `internal/worker/`: Bounded goroutine worker pool with exponential backoff.
  - `internal/middleware/`: JWT authentication, Redis token-bucket rate limiter, structured logging, panic recovery.
  - `pkg/`: Reusable database, logger, response, and validator packages.
- Configured development tooling and build automation:
  - Docker Compose setup with PostgreSQL 16 and Redis 7 containers.
  - Project Makefile with targets for migration, build, seeding, and race-detector testing.
  - Standardized `.env.example` configuration template.

---

## Milestone 2: Relational Schema & Migration Architecture
- Created reversible PostgreSQL migrations:
  - `migrations/000001_init_schema.up.sql`: Tables for `users`, `products`, `inventory`, `orders`, `order_items`, `idempotency_keys` with foreign keys, check constraints (`quantity >= 0`), and composite indexes.
  - `migrations/000001_init_schema.down.sql`: Reversible teardown.
  - `migrations/seeds/001_initial_dark_store_data.sql`: Seed data with organic milk, sourdough bread, cold brew nitro coffee, avocados, and crisps.

---

## Milestone 3: Authentication & Security
- Implemented bcrypt password hashing and constant-time comparison.
- Added JWT token issuance with HMAC-SHA256 signing, claims extraction, expiration, and authentication middleware.
- Enforced input validation for email formats, password complexity, and required fields.

---

## Milestone 4: Concurrency & Zero Overselling Protection
- Formulated transactional stock updates using PostgreSQL row-level locks:
  ```sql
  SELECT quantity FROM inventory WHERE product_id = $1 FOR UPDATE
  ```
- Guaranteed that product IDs are sorted deterministically before row acquisition to prevent database deadlocks.
- Implemented database-level `CHECK (quantity >= 0)` constraint as a secondary invariant against negative stock.
- Created `tests/concurrent_order_test.go` verifying 100 concurrent checkout attempts under the Go race detector (`go test -race`).

---

## Milestone 5: Idempotency & Rate Limiting
- Enforced `Idempotency-Key` HTTP header on order submissions.
- Computed SHA-256 hash of incoming request payloads to detect duplicate submissions and prevent payload tampering (returning `409 Conflict` or `422 Unprocessable Entity` on mismatch).
- Implemented Redis Token-Bucket rate limiting using an atomic Lua script (`HMGET` / `HMSET` with bucket replenishment calculation and burst capacity protection).

---

## Milestone 6: Redis Shopping Cart
- Built Redis cart repository with automatic 24-hour TTL refreshed on every cart mutation.
- Provided endpoints:
  - `GET /api/v1/cart`
  - `POST /api/v1/cart/items`
  - `PATCH /api/v1/cart/items/{productId}`
  - `DELETE /api/v1/cart/items/{productId}`
  - `DELETE /api/v1/cart`
- Integrated automatic cart clearance upon successful order placement commit.

---

## Milestone 7: Bounded Goroutine Worker Pool & Graceful Shutdown
- Built bounded worker pool (`internal/worker/pool.go`):
  - Fixed number of worker goroutines consuming from a buffered job channel.
  - Exponential backoff retry mechanism for transient failures (`50ms * 2^(retry-1)`).
  - Context timeouts per job.
  - Table-driven unit tests verifying retry progression and completion (`internal/worker/pool_test.go`).
- Wired graceful shutdown in `cmd/server/main.go` listening for `SIGINT` and `SIGTERM`, draining in-flight HTTP requests and flushing the worker pool within bounded deadlines.

---

## Milestone 8: Load Testing & DevOps Containerization
- Authored production multi-stage `Dockerfile` and `docker-compose.yml` for PostgreSQL 16, Redis 7, API, and k6.
- Created k6 load testing scripts:
  - `tests/k6/load_test.js`: General endpoint load and rate-limit verification.
  - `tests/k6/overselling_stress_test.js`: Flash-sale spike simulation asserting zero overselling.
- Provided `Makefile` with targets: `build`, `run`, `test`, `test-race`, `docker-up`, `docker-down`, `k6-load`, `k6-stress`.

