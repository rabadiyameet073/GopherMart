# GopherMart — API Specification

## 1. General Principles
- **Base URL**: `http://localhost:8080`
- **Content-Type**: `application/json`
- **Authentication**: Bearer Token in `Authorization: Bearer <token>`
- **Response Format**: Uniform JSON envelopes:
  ```json
  {
    "success": true,
    "data": { ... }
  }
  ```
  ```json
  {
    "success": false,
    "error": {
      "code": "ERROR_CODE",
      "message": "Human-readable explanation"
    }
  }
  ```

---

## 2. Public Endpoints

### `GET /health` & `GET /health/live`
- **Purpose**: Liveness probe.
- **Response**: `200 OK`

### `GET /ready` & `GET /health/ready`
- **Purpose**: Readiness probe (validates PostgreSQL and Redis ping).
- **Response**: `200 OK` or `503 Service Unavailable`

### `POST /api/v1/auth/register`
- **Request Body**:
  ```json
  {
    "name": "Jane Doe",
    "email": "jane@example.com",
    "password": "securePassword123"
  }
  ```
- **Response**: `201 Created`

### `POST /api/v1/auth/login`
- **Request Body**:
  ```json
  {
    "email": "jane@example.com",
    "password": "securePassword123"
  }
  ```
- **Response**: `200 OK` with JWT token and user info.

### `GET /api/v1/products`
- **Query Params**:
  - `category` (optional)
  - `limit` (default: 20, max: 100)
  - `offset` (default: 0)
- **Response**: `200 OK` with paginated list and metadata.

### `GET /api/v1/products/{id}`
- **Response**: `200 OK` or `404 Not Found`

---

## 3. Protected Endpoints (Requires `Authorization: Bearer <token>`)

### `GET /api/v1/users/me`
- **Response**: `200 OK` with authenticated user profile.

### `GET /api/v1/cart`
- **Response**: `200 OK` with current user cart.

### `POST /api/v1/cart/items`
- **Request Body**:
  ```json
  {
    "product_id": "b0000000-0000-0000-0000-000000000001",
    "quantity": 2
  }
  ```
- **Response**: `200 OK`

### `PATCH /api/v1/cart/items/{productId}`
- **Request Body**: `{"quantity": 3}`
- **Response**: `200 OK`

### `DELETE /api/v1/cart/items/{productId}`
- **Response**: `200 OK`

### `DELETE /api/v1/cart`
- **Response**: `200 OK` (clears cart)

### `POST /api/v1/orders`
- **Headers**: `Idempotency-Key: <unique-uuid>`
- **Request Body**:
  ```json
  {
    "items": [
      {
        "product_id": "b0000000-0000-0000-0000-000000000001",
        "quantity": 1
      }
    ]
  }
  ```
- **Responses**:
  - `201 Created`: Order confirmed. Header `X-Cache-Lookup: HIT-IDEMPOTENT` on idempotent replay.
  - `409 Conflict`: Insufficient stock or in-flight request.
  - `422 Unprocessable Entity`: Idempotency key reused with mismatched request payload.

### `GET /api/v1/orders`
- **Response**: `200 OK` with list of user's historical orders.

### `GET /api/v1/orders/{id}`
- **Response**: `200 OK` with order details and line items.

---

## 4. Admin-Only Endpoints (Requires `role: admin` in JWT)

### `POST /api/v1/products`
- **Request Body**:
  ```json
  {
    "sku": "SKU-TEA-006",
    "name": "Matcha Green Tea (250g)",
    "description": "Ceremonial grade organic Japanese matcha",
    "price_cents": 1299,
    "category": "Beverages",
    "initial_stock": 40
  }
  ```
- **Response**: `201 Created`

### `PATCH /api/v1/products/{id}`
- **Request Body**: Partial update `{ "price_cents": 1199 }`
- **Response**: `200 OK`
