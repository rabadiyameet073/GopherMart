package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/gophermart/ecommerce-order-inventory/internal/model"
)

type CartRepository interface {
	GetCart(ctx context.Context, userID string) (*model.Cart, error)
	SaveCart(ctx context.Context, cart *model.Cart, ttl time.Duration) error
	ClearCart(ctx context.Context, userID string) error
}

type redisCartRepository struct {
	client *redis.Client
}

func NewCartRepository(client *redis.Client) CartRepository {
	return &redisCartRepository{client: client}
}

func (r *redisCartRepository) GetCart(ctx context.Context, userID string) (*model.Cart, error) {
	key := fmt.Sprintf("cart:%s", userID)
	data, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return &model.Cart{UserID: userID, Items: []model.CartItem{}}, nil
		}
		return nil, fmt.Errorf("failed to retrieve cart from redis: %w", err)
	}

	var cart model.Cart
	if err := json.Unmarshal(data, &cart); err != nil {
		return nil, fmt.Errorf("failed to deserialize cart: %w", err)
	}
	return &cart, nil
}

func (r *redisCartRepository) SaveCart(ctx context.Context, cart *model.Cart, ttl time.Duration) error {
	key := fmt.Sprintf("cart:%s", cart.UserID)
	data, err := json.Marshal(cart)
	if err != nil {
		return fmt.Errorf("failed to serialize cart: %w", err)
	}

	if err := r.client.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("failed to persist cart in redis: %w", err)
	}
	return nil
}

func (r *redisCartRepository) ClearCart(ctx context.Context, userID string) error {
	key := fmt.Sprintf("cart:%s", userID)
	return r.client.Del(ctx, key).Err()
}
