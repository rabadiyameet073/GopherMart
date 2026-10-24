# GopherMart — Project Overview

## 1. Executive Summary
GopherMart is a portfolio-grade, production-minded quick-commerce REST API backend written in Go (1.23+), backed by PostgreSQL, Redis, and Docker Compose. It serves as a dark-store fulfillment engine where customers register, authenticate, browse catalogs, maintain real-time carts, and execute concurrent checkouts with zero risk of overselling or duplicate charging.

## 2. Core Engineering Goals
- **Strict Layered Clean Architecture**: Complete separation of HTTP delivery, application domain logic, and data infrastructure.
- **Transactional Row-Level Locking**: Atomic stock deductions preventing race conditions across concurrent checkouts (`SELECT ... FOR UPDATE`).
- **Deterministic Deadlock Prevention**: Consistent alphabetical sorting of product IDs prior to row lock acquisition.
- **Idempotency Guarantees**: Header-driven (`Idempotency-Key`) order deduplication with SHA-256 payload verification.
- **High-Throughput Redis Integration**: Sliding TTL shopping carts and atomic Lua token-bucket rate limiting.
- **Bounded Asynchronous Processing**: Fixed-worker pool with context deadlines and exponential backoff retries.
- **Operational Hygiene**: Structured JSON logging via `log/slog`, graceful signal termination (`SIGINT`/`SIGTERM`), and comprehensive empirical test suites.

## 3. Technology Stack Justification
| Component | Technology | Rationale |
| :--- | :--- | :--- |
| **Language** | Go 1.23+ | Native concurrency primitives (goroutines, channels), fast startup, low memory footprint. |
| **Database** | PostgreSQL 16 | ACID transactions, strict row-level locking, non-negative table constraints. |
| **Connection Pool** | `jackc/pgx/v5` | High-performance native Go driver with tuned connection pooling. |
| **In-Memory Cache** | Redis 7 (`go-redis/v9`) | Sub-millisecond latency for temporary cart state and atomic Lua rate limiting. |
| **HTTP Routing** | `gorilla/mux` | Standard library-compatible, idiomatic pattern matching for REST resources. |
| **Authentication** | `golang-jwt/jwt/v5` | Cryptographically signed HMAC-SHA256 stateless access tokens. |
| **Password Hashing** | `golang.org/x/crypto/bcrypt` | Adaptive work factor (cost 10), resistant to GPU brute-force attacks. |
| **Containerization** | Docker & Compose | Multi-stage Alpine builds with deterministic reproducible local testing. |
