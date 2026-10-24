# GopherMart — Architecture Decision Records (ADR)

## ADR-001: Integer Minor Units for Monetary Values
- **Status**: Accepted
- **Context**: Quick commerce calculations require adding item unit prices, quantities, and taxes. Floating-point numbers (`float64`) introduce binary representation rounding errors (e.g., `0.1 + 0.2 != 0.3`).
- **Decision**: Represent all monetary amounts strictly as integer minor units (e.g. cents / paise) as `price_cents` and `total_amount_cents`.
- **Consequences**: Zero floating point drift, exact integer arithmetic across invoices and orders.

## ADR-002: PostgreSQL Row-Level Locking (`FOR UPDATE`) for Inventory
- **Status**: Accepted
- **Context**: Flash sales create massive concurrent checkout demand. If two transactions read stock simultaneously and update, overselling occurs.
- **Decision**: Use `SELECT quantity FROM inventory WHERE product_id = $1 FOR UPDATE` inside an ACID transaction with deterministic alphabetical ordering of product IDs.
- **Consequences**: Guaranteed serialized stock allocation, zero overselling, zero deadlock risk.

## ADR-003: Header-Based Idempotency with SHA-256 Payload Hash
- **Status**: Accepted
- **Context**: Unstable mobile networks cause dropped connections and automatic retries.
- **Decision**: Require `Idempotency-Key` header on order creation. Verify canonical SHA-256 hash of the request payload against persisted database records.
- **Consequences**: Safe transparent replays of confirmed orders; prevents accidental duplicate charges.

## ADR-004: In-Memory Bounded Goroutine Worker Pool for Events
- **Status**: Accepted
- **Context**: Order confirmation notifications and dark-store picking tickets should not delay HTTP response latencies.
- **Decision**: Dispatch order events into a bounded buffered channel consumed by a fixed pool of 5 worker goroutines with exponential backoff retries.
- **Consequences**: Low latency HTTP responses, memory bound protection, clean graceful shutdown via `sync.WaitGroup`.
