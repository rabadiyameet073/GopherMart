package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/gophermart/ecommerce-order-inventory/internal/model"
)

var (
	ErrDuplicateInFlight = errors.New("a concurrent request with this Idempotency-Key is currently in-flight")
	ErrIdempotencyConflict = errors.New("idempotency key was used previously with a different request payload")
)

type IdempotencyRepository interface {
	Get(ctx context.Context, key, userID string) (*model.IdempotencyRecord, error)
	CreateProcessing(ctx context.Context, record *model.IdempotencyRecord) error
	Complete(ctx context.Context, key string, statusCode int, responseBody string) error
	Fail(ctx context.Context, key string, statusCode int, responseBody string) error
}

type pgIdempotencyRepository struct {
	pool *pgxpool.Pool
}

func NewIdempotencyRepository(pool *pgxpool.Pool) IdempotencyRepository {
	return &pgIdempotencyRepository{pool: pool}
}

func (r *pgIdempotencyRepository) Get(ctx context.Context, key, userID string) (*model.IdempotencyRecord, error) {
	query := `
		SELECT key, user_id, request_hash, status, response_status_code, response_body, created_at, expires_at
		FROM idempotency_keys
		WHERE key = $1 AND user_id = $2
	`
	var rec model.IdempotencyRecord
	var resBody *string
	var resStatus *int

	err := r.pool.QueryRow(ctx, query, key, userID).Scan(
		&rec.Key, &rec.UserID, &rec.RequestHash, &rec.Status, &resStatus, &resBody, &rec.CreatedAt, &rec.ExpiresAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to check idempotency key: %w", err)
	}

	if resBody != nil {
		rec.ResponseBody = *resBody
	}
	if resStatus != nil {
		rec.ResponseStatusCode = *resStatus
	}

	// Check expiration
	if time.Now().After(rec.ExpiresAt) {
		return nil, nil
	}

	return &rec, nil
}

func (r *pgIdempotencyRepository) CreateProcessing(ctx context.Context, rec *model.IdempotencyRecord) error {
	query := `
		INSERT INTO idempotency_keys (key, user_id, request_hash, status, created_at, expires_at)
		VALUES ($1, $2, $3, 'PROCESSING', NOW(), $4)
	`
	_, err := r.pool.Exec(ctx, query, rec.Key, rec.UserID, rec.RequestHash, rec.ExpiresAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique violation
			return ErrDuplicateInFlight
		}
		return fmt.Errorf("failed to register idempotency record: %w", err)
	}
	return nil
}

func (r *pgIdempotencyRepository) Complete(ctx context.Context, key string, statusCode int, responseBody string) error {
	query := `
		UPDATE idempotency_keys
		SET status = 'COMPLETED', response_status_code = $1, response_body = $2
		WHERE key = $3
	`
	_, err := r.pool.Exec(ctx, query, statusCode, responseBody, key)
	if err != nil {
		return fmt.Errorf("failed to complete idempotency record: %w", err)
	}
	return nil
}

func (r *pgIdempotencyRepository) Fail(ctx context.Context, key string, statusCode int, responseBody string) error {
	query := `
		UPDATE idempotency_keys
		SET status = 'FAILED', response_status_code = $1, response_body = $2
		WHERE key = $3
	`
	_, err := r.pool.Exec(ctx, query, statusCode, responseBody, key)
	if err != nil {
		return fmt.Errorf("failed to mark idempotency record failed: %w", err)
	}
	return nil
}
