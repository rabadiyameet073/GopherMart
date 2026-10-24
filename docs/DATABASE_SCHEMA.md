# GopherMart — Database Schema Documentation

## 1. Schema Diagram & Relationships

```mermaid
erDiagram
    users ||--o{ orders : "places"
    users ||--o{ idempotency_keys : "owns"
    orders ||--|{ order_items : "contains"
    products ||--o{ order_items : "referenced in"
    products ||--|| inventory : "tracks"

    users {
        uuid id PK
        varchar email UK
        varchar password_hash
        varchar name
        varchar role
        timestamptz created_at
        timestamptz updated_at
    }

    products {
        uuid id PK
        varchar sku UK
        varchar name
        text description
        integer price_cents
        varchar category
        timestamptz created_at
        timestamptz updated_at
    }

    inventory {
        uuid id PK
        uuid product_id FK,UK
        integer quantity "CHECK >= 0"
        integer reserved_quantity "CHECK >= 0"
        integer version
        timestamptz updated_at
    }

    orders {
        uuid id PK
        uuid user_id FK
        varchar status
        integer total_amount_cents "CHECK >= 0"
        timestamptz created_at
        timestamptz updated_at
    }

    order_items {
        uuid id PK
        uuid order_id FK
        uuid product_id FK
        integer quantity "CHECK > 0"
        integer price_cents "CHECK >= 0"
        timestamptz created_at
    }

    idempotency_keys {
        varchar key PK
        uuid user_id FK
        varchar request_hash
        varchar status
        integer response_status_code
        text response_body
        timestamptz created_at
        timestamptz expires_at
    }
```

## 2. Integrity & Invariant Guarantees
- **Non-Negative Stock**: `inventory.quantity` is guarded by an explicit SQL constraint `CHECK (quantity >= 0)`. Any attempt to decrement past zero will immediately trigger a database constraint violation.
- **Price Representation**: Money is strictly stored as integer minor units (`price_cents`, `total_amount_cents`) to completely eliminate IEEE 754 floating-point rounding errors.
- **Cascade Rules**: Deleting a product automatically cascades to its inventory row; deleting an order cascades to its items.

## 3. Indexing Strategy
- `idx_products_category`: B-Tree index for catalog category filtering.
- `idx_inventory_product_id`: Fast lookup for stock availability checks.
- `idx_orders_user_id`: Efficient pagination of historical user orders.
- `idx_orders_status`: Fast state query filtering for fulfillment dispatch.
- `idx_order_items_order_id`: Fast join retrieval for invoice line items.
- `idx_idempotency_expires`: Enables periodic background cleanup of expired idempotency keys.

