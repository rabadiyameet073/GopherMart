# Multi-stage Dockerfile for GopherMart
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install git and ca-certificates
RUN apk add --no-cache git ca-certificates

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build statically linked binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o gophermart ./cmd/server

# Final production stage
FROM alpine:3.20

WORKDIR /app

RUN apk --no-cache add ca-certificates tzdata && \
    addgroup -S gopher && adduser -S gopher -G gopher

COPY --from=builder /app/gophermart /app/gophermart
COPY --from=builder /app/migrations /app/migrations

USER gopher

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

ENTRYPOINT ["/app/gophermart"]
