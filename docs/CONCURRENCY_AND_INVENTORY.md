# GopherMart — Concurrency & Inventory Control

## 1. The Overselling Challenge in Quick Commerce
In a high-throughput dark-store service, sudden demand spikes (e.g., flash sales, limited items like cold brew or fresh avocados) cause dozens of concurrent checkout requests to target the final remaining units of stock. 

A naive `SELECT quantity ...` followed by an `UPDATE inventory SET quantity = quantity - 1` suffers from classical Read-Modify-Write race conditions, resulting in negative inventory and broken fulfillment promises.

## 2. PostgreSQL Row-Level Locking (`FOR UPDATE`)
GopherMart enforces strict serialized stock deduction using PostgreSQL explicit row-level locking inside a database transaction:

```sql
-- Step 1: Lock the inventory row exclusively
SELECT quantity
FROM inventory
WHERE product_id = $1
FOR UPDATE;

-- Step 2: Validate available quantity against requested quantity in Go
-- If insufficient, abort transaction and return ErrInsufficientStock (HTTP 409 Conflict)

-- Step 3: Decrement stock
UPDATE inventory
SET quantity = quantity - $1,
    updated_at = NOW()
WHERE product_id = $2;
```

While Transaction A holds the `FOR UPDATE` lock, Transaction B attempting to read the same inventory row will block at the database level until Transaction A commits or rolls back.

## 3. Deadlock Prevention via Deterministic Lock Ordering
When an order contains multiple items (e.g., Item X and Item Y), concurrent checkouts could cause a database deadlock if Transaction 1 locks X then Y, while Transaction 2 locks Y then X.

To prevent deadlocks mathematically, GopherMart sorts all item product IDs in ascending alphabetical order before beginning row lock acquisitions:

```go
sort.Slice(req.Items, func(i, j int) bool {
    return req.Items[i].ProductID < req.Items[j].ProductID
})
```

Because all transactions acquire locks in the exact same deterministic sequence, circular wait conditions are completely impossible.

## 4. Empirical Verification Under the Go Race Detector
In `tests/concurrent_order_test.go`, we verify concurrency safety by firing 100 concurrent goroutines against a product with initial stock = 10.
- Exactly 10 requests succeed (HTTP 201).
- Exactly 90 requests are rejected (HTTP 409 Conflict).
- Remaining stock is exactly 0.
- Executed and validated cleanly under `go test -race`.

