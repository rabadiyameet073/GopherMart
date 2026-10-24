# GopherMart — Quick-Commerce Backend in Go
## System Architecture & Technical Implementation Specification

> **Purpose:** Comprehensive architectural specification and engineering design document for the GopherMart quick-commerce dark-store backend.

---

## 1. Project Overview

**Project name:** GopherMart  
**Project type:** Quick-commerce REST API  
**Primary language:** Go  
**Core technologies:** Go, PostgreSQL, Redis, Docker Compose, JWT, `log/slog`

GopherMart is a backend service for a quick-commerce platform where customers can register, sign in, browse products, maintain a temporary shopping cart, and place orders. The system must handle concurrent purchases safely, prevent duplicate orders when clients retry requests, and process order-related events asynchronously.

The project should demonstrate more than basic CRUD. Its main engineering goals are:

- Clear separation of HTTP handlers, business services, and data repositories.
- Secure authentication and authorization.
- Reliable inventory management under concurrent order requests.
- Redis-backed carts with expiration.
- Idempotent order creation.
- Bounded asynchronous processing using goroutines and channels.
- Consistent use of `context.Context`, structured logging, tests, and graceful shutdown.
- Reproducible local development using Docker Compose.
- Clear documentation and a credible, chronological Git history.

### One-line description

**GopherMart is a Go-based quick-commerce backend featuring JWT authentication, Redis carts, transactional inventory management, idempotent orders, and asynchronous event processing.**

---

## 2. Important Git History Requirement

The requested résumé timeline lists the project as **August 2026 – Present**. Keep the repository history honest and consistent with the actual development timeline.

**Do not backdate commits or manipulate GitHub commit timestamps to make work appear to have happened three months earlier.** Commit author/committer dates can be altered locally, but that does not prove the work was performed then and can misrepresent the project's history. Instead:

1. Make real commits as features are completed.
2. Use small, meaningful commit messages.
3. Keep a dated development log in `docs/DEVELOPMENT_LOG.md`.
4. If the project was genuinely developed earlier, preserve the original repository/history or document the actual earlier work and evidence.
5. Do not claim an earlier start date on the résumé unless it is accurate.

---

## 3. Scope and Feature Requirements

### 3.1 Authentication and authorization

Implement:

- `POST /api/v1/auth/register`
- `POST /api/v1/auth/login`
- `GET /api/v1/users/me`

Requirements:

- Store passwords as secure hashes using a well-maintained password-hashing library such as `golang.org/x/crypto/bcrypt`.
- Never store or log plaintext passwords.
- Validate email and password inputs and enforce reasonable length limits.
- Enforce unique email addresses in PostgreSQL.
- Issue signed JWT access tokens with subject/user ID, issued-at, expiry, and issuer claims.
- Read JWT signing secrets from environment variables; never hard-code secrets.
- Validate token signature, expiry, issuer, and expected signing algorithm.
- Protect private routes with authentication middleware.
- Return consistent JSON errors without leaking stack traces or sensitive implementation details.
- Use a configurable, short-lived access token. Add refresh tokens only if they can be implemented securely and documented; they are not required for the MVP.

### 3.2 Products and inventory

Implement:

- `GET /api/v1/products`
- `GET /api/v1/products/{id}`
- `POST /api/v1/products` (admin-only if roles are implemented)
- `PATCH /api/v1/products/{id}` (admin-only if roles are implemented)

A product should include:

- `id`
- `name`
- `description`
- `sku`
- `price` represented safely as integer minor units (for example, paise/cents) rather than floating-point arithmetic
- `stock_quantity`
- `created_at`
- `updated_at`

Requirements:

- PostgreSQL is the source of truth for product and stock data.
- Enforce unique SKU values and non-negative stock constraints at the database level.
- Use pagination for product listing.
- Validate request bodies and return clear status codes.
- Do not allow ordinary customers to change inventory.
- Define the currency and units in documentation. Avoid silently mixing rupees/paise or dollars/cents.

### 3.3 Cart in Redis

Implement:

- `GET /api/v1/cart`
- `POST /api/v1/cart/items`
- `PATCH /api/v1/cart/items/{productID}`
- `DELETE /api/v1/cart/items/{productID}`
- `DELETE /api/v1/cart`

Requirements:

- Store carts in Redis, keyed by authenticated user ID.
- Set a configurable TTL, such as 24 hours, and refresh the TTL when a cart is changed.
- Store product IDs and quantities, not authoritative product prices or stock.
- Validate positive quantities and sensible cart-size limits.
- Check product availability when adding items and revalidate prices and stock during checkout.
- Treat the cart as temporary intent, not a reservation of inventory.
- Handle missing or expired carts gracefully.
- Do not trust client-submitted prices.

### 3.4 Order placement and overselling prevention

Implement:

- `POST /api/v1/orders`
- `GET /api/v1/orders`
- `GET /api/v1/orders/{id}`

Order creation should use an `Idempotency-Key` header.

Requirements:

- Authenticate the customer before order creation.
- Read cart contents and validate all requested products, quantities, and current prices.
- Use a PostgreSQL transaction for order creation and inventory deduction.
- Prevent overselling when multiple customers buy the last units concurrently.
- Use safe locking or conditional stock updates. A robust approach is to lock product rows in a deterministic order (`SELECT ... FOR UPDATE`) and then validate and decrement stock within one transaction. Alternatively, use conditional updates such as `UPDATE products SET stock_quantity = stock_quantity - $1 WHERE id = $2 AND stock_quantity >= $1`, checking affected rows.
- Lock products in a consistent order to reduce deadlock risk.
- Insert the order and order items in the same transaction as stock changes.
- Roll back all changes if any item is unavailable or any operation fails.
- Store money as integer minor units.
- Calculate totals on the server.
- Clear the cart only after the order transaction commits successfully.
- Publish an order-created event only after a successful commit. Use an after-commit mechanism or a transactional outbox if reliable event delivery is included.
- Ensure retries with the same idempotency key do not create duplicate orders.

#### Idempotency design

Use a table such as `idempotency_records` with:

- `user_id`
- `idempotency_key`
- request hash or equivalent request identity
- response status and stored response/order ID
- creation and expiry timestamps

Enforce a unique constraint on `(user_id, idempotency_key)`. Define behavior for:

- Same key and same request: return the original result.
- Same key but different request: return `409 Conflict`.
- Concurrent requests using the same key: only one request may create the order.
- Failed transaction: do not leave a misleading successful idempotency record.

Document retention/cleanup policy. Avoid implementing idempotency as only an in-memory map, because it would not survive restarts or multiple app instances.

### 3.5 Asynchronous order-event worker pool

Use goroutines and channels to process events after order creation. Initial event types can include:

- `order.created`
- `order.confirmation_requested`
- `order.notification_requested`

For the MVP, confirmation and notification can be clearly labelled stubs that log a simulated action. Do not claim that real email or SMS was sent.

Requirements:

- Use a bounded channel or bounded queue to avoid unbounded memory growth.
- Use a configurable number of workers.
- Pass `context.Context` through operations.
- Add retry behavior with capped exponential backoff for retryable failures.
- Distinguish retryable from permanent failures.
- Make handlers safe to retry; use event/order IDs to avoid duplicate side effects where applicable.
- Handle queue saturation explicitly (for example, return a controlled error, persist to an outbox, or record a measurable drop policy). Never silently lose important events.
- Shut workers down cleanly: stop accepting new work, drain within a deadline, then cancel remaining work.
- Add tests for successful processing, retry behavior, queue saturation, and shutdown.

**Reliability note:** A plain in-memory channel does not survive a process crash. For the initial MVP, document this limitation. For a stronger version, add a PostgreSQL transactional outbox and a dispatcher that publishes pending events to workers. This ensures the order transaction and event record commit atomically.

### 3.6 HTTP server and middleware

Use the standard library where practical, or select a small, well-maintained router and explain the choice.

Implement:

- Request ID middleware.
- Structured request logging using `log/slog`.
- Panic recovery.
- Authentication middleware for private endpoints.
- Request body size limits.
- JSON content-type checks and strict decoding where appropriate.
- Read-header, read, write, and idle server timeouts.
- Consistent JSON response/error shapes.
- Health endpoints:
  - `GET /health/live`
  - `GET /health/ready`

Readiness should verify dependencies appropriately, while liveness should indicate that the process is running. Do not expose secrets or internal diagnostics publicly.

### 3.7 Context, cancellation, and graceful shutdown

Use `context.Context` for request-scoped database, Redis, and worker operations.

Requirements:

- Do not use `context.Background()` inside ordinary request/business/repository code when a caller context is available.
- Use a root context for application lifecycle management.
- Listen for OS termination signals.
- Stop accepting new HTTP requests during shutdown.
- Allow in-flight requests and workers a bounded grace period to finish.
- Close database and Redis clients after dependent work has stopped.
- Avoid goroutine leaks and ensure channels have clear ownership/closure rules.
- Do not close a channel from multiple owners or send to a closed channel.

### 3.8 Database and migrations

Use PostgreSQL with version-controlled migrations.

Suggested tables:

- `users`
- `products`
- `orders`
- `order_items`
- `idempotency_records`
- Optional: `outbox_events`

Requirements:

- Use constraints, indexes, and foreign keys where appropriate.
- Store timestamps consistently, preferably in UTC.
- Use a migration tool or a small documented migration approach.
- Never rely solely on startup-time ad hoc table creation for a portfolio project.
- Add indexes for common lookups, such as user email, SKU, order user/date, and idempotency key.
- Include a documented migration and rollback workflow.

### 3.9 Configuration and secrets

Use environment variables and validate configuration at startup.

Example variables:

```env
APP_ENV=development
HTTP_ADDR=:8080
LOG_LEVEL=debug
DATABASE_URL=postgres://gophermart:gophermart_dev@postgres:5432/gophermart?sslmode=disable
REDIS_ADDR=redis:6379
REDIS_PASSWORD=
JWT_SECRET=replace-with-a-long-random-local-secret
JWT_ISSUER=gophermart
JWT_TTL=15m
CART_TTL=24h
WORKER_COUNT=4
WORKER_QUEUE_SIZE=100
SHUTDOWN_TIMEOUT=15s
```

These values are examples for local development only. Use a `.env.example` file with placeholders. Do not commit real secrets, production credentials, or personal tokens. Ensure `.env` is ignored by Git.

### 3.10 Docker Compose

Provide a Docker Compose environment containing:

- `app`
- `postgres`
- `redis`

Requirements:

- Add health checks and dependency readiness handling.
- Persist database data using a named volume.
- Use a non-root application user where practical.
- Keep development defaults separate from production recommendations.
- Support a one-command local startup and clear teardown instructions.
- Avoid exposing database/Redis ports unnecessarily outside local development.
- Document that Compose health checks do not replace application-level retry and timeout handling.

### 3.11 Tests and quality checks

Include tests that prove the core engineering claims.

Minimum test plan:

1. Registration and login success/failure.
2. Passwords are hashed, never stored in plaintext.
3. JWT middleware accepts valid tokens and rejects invalid/expired tokens.
4. Product validation and pagination.
5. Redis cart create/update/delete and TTL behavior.
6. Order success updates stock and persists order/items.
7. Insufficient stock rolls back the entire order.
8. Two concurrent orders competing for the final unit result in at most one success.
9. Repeating an idempotency key returns the same order rather than creating another.
10. Reusing a key with a different request returns a conflict.
11. Worker retry and graceful shutdown behavior.
12. Health/readiness endpoints.

Use table-driven tests where useful. Use unit tests for pure business logic and integration tests with real PostgreSQL/Redis containers or a documented Compose test environment for concurrency and persistence guarantees. Do not claim tests passed unless they were actually run.

Recommended checks:

```bash
gofmt -w .
go vet ./...
go test ./...
go test -race ./...
```

Add linting only if configured and documented. Keep the test suite deterministic and avoid relying on arbitrary sleeps.

### 3.12 Observability

Use `log/slog` structured JSON logs outside local development and readable logs locally.

Log useful fields such as:

- request ID
- route/method/status
- request duration
- order ID
- event type
- worker attempt
- error category

Never log passwords, JWTs, authorization headers, full payment details, or secret environment values. Avoid logging entire cart contents or personal information unnecessarily.

Metrics/tracing can be an optional extension, not a blocker for the MVP.

---

## 4. Suggested Architecture

Use a modular monolith. Do not split into microservices merely for appearance.

```text
cmd/
  api/
    main.go
internal/
  config/
  http/
    handlers/
    middleware/
    response/
    router/
  auth/
  users/
  products/
  cart/
  orders/
  events/
  workers/
  database/
  redis/
  observability/
migrations/
docs/
tests/
```

Suggested dependency direction:

```text
HTTP handlers
     |
     v
Application/services
     |
     +----> Repositories / infrastructure interfaces
                    |
                    +----> PostgreSQL
                    +----> Redis
                    +----> Event queue / outbox
```

Guidelines:

- Keep HTTP concerns out of business logic.
- Keep SQL and Redis commands out of handlers.
- Define interfaces at useful boundaries; avoid creating an interface for every trivial type.
- Inject dependencies rather than relying on global clients.
- Keep transaction boundaries explicit.
- Prefer simple code that can be explained in an interview over unnecessary abstraction.

---

## 5. API Contract Draft

All JSON responses should use a consistent structure. Example:

```json
{
  "data": {
    "id": "example-id"
  },
  "error": null
}
```

Error example:

```json
{
  "data": null,
  "error": {
    "code": "INSUFFICIENT_STOCK",
    "message": "One or more items are unavailable"
  }
}
```

Suggested endpoints:

| Method | Endpoint | Access | Purpose |
|---|---|---|---|
| POST | `/api/v1/auth/register` | Public | Register a customer |
| POST | `/api/v1/auth/login` | Public | Authenticate and issue access token |
| GET | `/api/v1/users/me` | Authenticated | Current user profile |
| GET | `/api/v1/products` | Public | Paginated product list |
| GET | `/api/v1/products/{id}` | Public | Product details |
| POST | `/api/v1/products` | Admin | Create product |
| PATCH | `/api/v1/products/{id}` | Admin | Update product |
| GET | `/api/v1/cart` | Authenticated | View current cart |
| POST | `/api/v1/cart/items` | Authenticated | Add an item |
| PATCH | `/api/v1/cart/items/{productID}` | Authenticated | Change quantity |
| DELETE | `/api/v1/cart/items/{productID}` | Authenticated | Remove an item |
| DELETE | `/api/v1/cart` | Authenticated | Clear cart |
| POST | `/api/v1/orders` | Authenticated | Place order; requires `Idempotency-Key` |
| GET | `/api/v1/orders` | Authenticated | List the current user's orders |
| GET | `/api/v1/orders/{id}` | Owner/admin | Get an order |
| GET | `/health/live` | Public/internal | Liveness check |
| GET | `/health/ready` | Internal | Dependency readiness |

Use UUIDs or another consistent ID strategy and document it.

---

## 6. Implementation Roadmap

Build in small, working increments. Each phase should leave the repository in a buildable state.

### Phase 0 — Repository foundation

- Initialize Go module.
- Choose and document the router, PostgreSQL driver, Redis client, JWT library, and migration tool.
- Add `.gitignore`, `.env.example`, `README.md`, and initial docs.
- Add Docker Compose for PostgreSQL and Redis.
- Add config parsing/validation and a minimal health endpoint.
- Verify formatting, build, and startup.

**Acceptance:** The app starts locally, configuration errors are clear, and dependencies become healthy.

### Phase 1 — PostgreSQL and migrations

- Add database connection pooling.
- Add migrations for users and products.
- Add repository patterns for database access.
- Add product listing and product detail endpoints.
- Add pagination and database constraints.

**Acceptance:** A clean database can be migrated and products can be queried through the API.

### Phase 2 — Authentication

- Add registration and login.
- Hash passwords.
- Implement JWT generation/validation and auth middleware.
- Add current-user endpoint.
- Add auth unit/integration tests.

**Acceptance:** Valid users can log in; invalid credentials and invalid tokens are rejected.

### Phase 3 — Redis cart

- Connect Redis with timeouts.
- Implement cart CRUD.
- Add TTL and expiry tests.
- Add validation for product existence and quantity.

**Acceptance:** A user's cart persists in Redis until its TTL expires and cannot be accessed by another user.

### Phase 4 — Transactional orders

- Add order/order-item schema.
- Implement stock validation and atomic inventory decrement.
- Implement order transaction and rollback behavior.
- Add idempotency table and semantics.
- Add concurrent order integration tests.

**Acceptance:** The last unit of stock cannot be sold twice; retries with the same key do not create duplicates.

### Phase 5 — Worker pool and events

- Define event payloads and worker interfaces.
- Add bounded queue and worker lifecycle.
- Add retry/backoff and cancellation.
- Add confirmation/notification stubs.
- Add queue saturation and shutdown tests.
- Decide whether to add a transactional outbox.

**Acceptance:** Events are processed by a bounded worker pool; failures are handled according to documented policy.

### Phase 6 — Production-minded HTTP and shutdown

- Add request IDs, structured logs, recovery, body limits, and timeouts.
- Add readiness/liveness endpoints.
- Implement signal handling and graceful shutdown.
- Verify no goroutine leaks or race detector failures.

**Acceptance:** The server shuts down predictably and logs requests without exposing secrets.

### Phase 7 — Integration, documentation, and polish

- Complete Docker Compose with application container and health checks.
- Add integration-test instructions.
- Add API examples and an API collection or OpenAPI spec if feasible.
- Write architecture and design decision docs.
- Run all checks.
- Review README, screenshots/diagrams if useful, and project limitations.

**Acceptance:** A new developer can clone the repository, configure it, run it, test it, and understand the architecture using the docs.

---

## 7. Repository Documentation Plan

Create these documents over the project lifecycle:

```text
README.md
docs/
  PROJECT_OVERVIEW.md
  ARCHITECTURE.md
  API_SPECIFICATION.md
  DATABASE_SCHEMA.md
  ORDERING_AND_IDEMPOTENCY.md
  CONCURRENCY_AND_INVENTORY.md
  WORKERS_AND_EVENTS.md
  SECURITY.md
  TESTING.md
  DEVELOPMENT_LOG.md
  DECISIONS.md
```

### Document purposes

- `README.md`: overview, stack, quick start, configuration, test commands, API highlights, and limitations.
- `PROJECT_OVERVIEW.md`: goals, scope, non-goals, users, features, and acceptance criteria.
- `ARCHITECTURE.md`: modules, dependency flow, transaction boundaries, and infrastructure.
- `API_SPECIFICATION.md`: endpoints, request/response examples, status codes, auth, and idempotency header.
- `DATABASE_SCHEMA.md`: tables, relationships, constraints, indexes, and migrations.
- `ORDERING_AND_IDEMPOTENCY.md`: order lifecycle, transaction steps, retry behavior, and failure scenarios.
- `CONCURRENCY_AND_INVENTORY.md`: row locks/conditional updates, lock ordering, race scenarios, and concurrency test design.
- `WORKERS_AND_EVENTS.md`: event schema, queue capacity, retry strategy, shutdown, and delivery guarantees.
- `SECURITY.md`: password hashing, JWT handling, secrets, authorization, input validation, and logging rules.
- `TESTING.md`: unit/integration tests, race detector, setup, fixtures, and expected checks.
- `DEVELOPMENT_LOG.md`: genuine progress notes and dates.
- `DECISIONS.md`: short architecture decision records, alternatives considered, and rationale.

Keep documentation synchronized with actual implementation. Mark planned features as **Planned**, partially implemented features as **In progress**, and only verified features as **Implemented**.

---

## 8. Git Commit Strategy

Use conventional, meaningful commits that represent actual work. Suggested sequence (make each commit only after the corresponding changes are implemented and checked):

1. `chore: initialize Go module and project structure`
2. `docs: add project overview and development roadmap`
3. `chore: add Docker Compose for PostgreSQL and Redis`
4. `feat(config): add validated environment configuration`
5. `feat(db): add PostgreSQL connection and migrations`
6. `feat(products): add product listing and inventory schema`
7. `feat(auth): implement registration and JWT login`
8. `feat(middleware): protect routes with JWT authentication`
9. `feat(cart): implement Redis cart with TTL`
10. `feat(orders): add transactional order placement`
11. `feat(orders): prevent overselling under concurrency`
12. `feat(orders): add persistent idempotency keys`
13. `feat(workers): add bounded order event worker pool`
14. `feat(server): add graceful shutdown and request timeouts`
15. `test: cover auth cart order and worker flows`
16. `docs: complete API architecture and testing guides`

These are planning examples, not a claim that commits already exist. Do not create commits for work that has not been done. Add a commit when a coherent, tested change is ready.

---

## 9. Definition of Done

The MVP is complete only when:

- [ ] The project builds and starts with documented commands.
- [ ] PostgreSQL migrations work from a clean database.
- [ ] Register/login and protected routes work.
- [ ] Passwords are hashed and secrets are not committed.
- [ ] Product APIs validate input and paginate results.
- [ ] Redis carts have a TTL and are scoped to users.
- [ ] Orders are created transactionally.
- [ ] Concurrent checkout cannot oversell inventory.
- [ ] Idempotency prevents duplicate orders across retries/restarts.
- [ ] Workers are bounded, cancellable, and shut down gracefully.
- [ ] Errors and logs are structured and do not leak secrets.
- [ ] Unit and integration tests cover the important failure cases.
- [ ] `go test ./...`, `go vet ./...`, and `go test -race ./...` have been run, with outcomes recorded accurately.
- [ ] Docker Compose, `.env.example`, README, and docs match the actual implementation.
- [ ] The repository's history and résumé timeline are truthful.

---

## 10. Instructions for the AI Coding Assistant

You are acting as a senior Go backend engineer, software architect, code reviewer, and mentor.

Build **GopherMart** in the current repository. Do not generate a huge, unreviewable code dump. Work incrementally and explain important decisions.

### Before coding

1. Inspect the existing repository and report its current structure and state.
2. Do not overwrite existing user work without explaining the impact.
3. Identify missing requirements and propose the first small milestone.
4. Confirm the chosen libraries and explain why each is appropriate.
5. Create or update `docs/PROJECT_OVERVIEW.md` and the initial roadmap before implementing the whole system.
6. If the environment has file-editing or terminal tools, use them to make real changes and run the relevant commands. Do not merely claim that files or tests were created.

### For every milestone

1. State the goal and acceptance criteria.
2. Implement only the scope of that milestone.
3. Keep the project compiling and runnable.
4. Add tests for the new behavior.
5. Run formatting, tests, and static checks that are available.
6. Report the exact commands run and whether they passed.
7. Update documentation to match the implementation.
8. Summarize files changed, decisions made, limitations, and the next step.
9. Suggest a conventional commit message; create a commit only when explicitly authorized and when the changes are ready.
10. Never fabricate test results, GitHub activity, benchmark results, or implementation status.

### Engineering standards

- Use idiomatic Go and clear names.
- Keep functions focused and errors wrapped with useful context.
- Prefer explicit dependencies and small, meaningful interfaces.
- Use parameterized SQL; never concatenate untrusted values into queries.
- Validate all external input.
- Use contexts and deadlines correctly.
- Protect against duplicate orders and concurrent inventory races.
- Make money calculations exact using integer minor units.
- Avoid global mutable state and goroutine leaks.
- Handle partial failures deliberately.
- Document trade-offs and MVP limitations.
- Do not add Kubernetes, microservices, payment processing, a frontend, or unnecessary abstractions unless requested. Keep the first release focused on the backend.

### First task

Start by inspecting the repository. Then provide:

1. A concise summary of the existing project state.
2. The proposed folder structure.
3. The recommended dependency choices.
4. The first milestone's exact tasks.
5. The initial documentation files to create.
6. The commands to verify the initial setup.

**Do not implement every phase at once. Begin with Phase 0 and wait for review before proceeding to the next phase.**
