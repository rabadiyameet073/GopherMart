# GopherMart — Security & Hardening Guide

## 1. Authentication & Token Architecture
- **Stateless JWTs**: Access tokens are signed using HMAC-SHA256 (`HS256`) with a cryptographically random secret string.
- **Claims**: Tokens encode `sub` (User UUID), `email`, `role` (`customer` or `admin`), `iat` (issued at), and `exp` (expires at).
- **Validation**:
  - Rejects tokens with `none` algorithm or mismatched signing methods.
  - Automatically verifies expiration timestamps.
  - Injects User ID and Role into `r.Context()` for downstream handlers.

## 2. Password Storage
- Passwords are encrypted using `golang.org/x/crypto/bcrypt` with `DefaultCost` (work factor 10).
- Passwords are never stored, transmitted in logs, or exposed in JSON responses (`PasswordHash` tag is `json:"-"`).
- Password comparison uses constant-time bcrypt verification to prevent timing attacks.

## 3. Rate Limiting Protection (Token Bucket)
- Prevents brute-force credential stuffing and flash-sale DDoS attacks.
- Implemented as an atomic Redis Lua script.
- Scoped to User ID when authenticated, and remote client IP when unauthenticated.
- Rejection returns `429 Too Many Requests` with a `Retry-After: 1` header.

## 4. Input Sanitization & Payload Protection
- **Request Body Limit**: Guarded by `http.MaxBytesReader` (default 1MB) to protect against memory exhaustion attacks.
- **Strict Parameterization**: All SQL queries use PostgreSQL placeholders (`$1, $2, ...`) via `pgxpool`, completely preventing SQL injection.

