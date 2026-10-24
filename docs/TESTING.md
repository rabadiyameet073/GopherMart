# GopherMart — Testing & Empirical Validation Guide

## 1. Test Strategy Overview
GopherMart follows a multi-tier testing strategy combining pure business logic unit tests, table-driven suites, Go race detector verification, and k6 load tests.

```text
tests/
├── concurrent_order_test.go    # Race detector, overselling, idempotency, context cancellation tests
├── auth_test.go                # Registration, login, password hashing, and token validation tests
├── cart_test.go                # Redis shopping cart CRUD and sliding TTL tests
└── k6/
    ├── load_test.js            # General endpoint throughput and rate-limit verification
    └── overselling_stress_test.js # High-concurrency flash sale overselling test
```

## 2. Running Test Suites

### Standard Unit & Table-Driven Tests
```bash
go test -v ./...
```

### Race Condition & Concurrency Audit
The Go race detector analyzes memory accesses across goroutines to catch data races:
```bash
go test -race -v ./tests/... ./internal/worker/...
```

### Static Analysis & Vet
```bash
go vet ./...
```

### k6 High-Concurrency Stress Tests
Simulate 50 virtual users competing for limited items:
```bash
docker-compose run --rm k6 run /scripts/overselling_stress_test.js
```

## 3. Empirical Verification Standard
- Tests must be deterministic (no flaky time sleeps).
- Concurrency tests must assert that remaining stock equals exactly `initial - successes`.
