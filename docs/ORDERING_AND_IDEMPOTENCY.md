# GopherMart — Ordering & Idempotency Design

## 1. The Need for Idempotency in Mobile Quick Commerce
Mobile clients often experience network drops immediately after submitting an order. When an HTTP connection times out or breaks, the client cannot know whether the server completed the transaction. Clients automatically retry the request. Without idempotency protection, a retry would double-charge the customer and allocate a second order.

## 2. Idempotency Header & Verification Workflow
Every `POST /api/v1/orders` request requires an `Idempotency-Key: <UUID>` HTTP header.

```mermaid
sequenceDiagram
    participant Client
    participant API as GopherMart API
    participant DB as PostgreSQL

    Client->>API: POST /api/v1/orders (Idempotency-Key: K1, Payload: P1)
    API->>API: Compute SHA-256 Hash(P1)
    API->>DB: SELECT * FROM idempotency_keys WHERE key = K1 AND user_id = U1
    alt Key Not Found
        API->>DB: INSERT INTO idempotency_keys (key, user_id, hash, status='PROCESSING')
        API->>DB: Execute Order Transaction (FOR UPDATE Stock Deduction)
        API->>DB: UPDATE idempotency_keys SET status='COMPLETED', response_body=OrderJSON
        API-->>Client: 201 Created (New Order)
    else Key Found with Status 'COMPLETED'
        alt Hashes Match
            API-->>Client: 201 Created (Cached Order, Header: X-Cache-Lookup: HIT-IDEMPOTENT)
        else Hash Mismatch
            API-->>Client: 422 Unprocessable Entity (Payload Conflict)
        end
    else Key Found with Status 'PROCESSING'
        API-->>Client: 409 Conflict (Concurrent request in-flight)
    end
```

## 3. Failure & Crash Resilience
- If the checkout transaction fails (e.g. insufficient stock), the idempotency record is marked `FAILED` or rolled back, allowing the customer to rectify cart quantities without permanently locking the key.
- Records are stamped with a 24-hour expiration (`expires_at`), matching the lifetime of dark-store picking cycles.
