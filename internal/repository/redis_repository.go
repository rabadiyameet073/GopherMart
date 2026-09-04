package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// tokenBucketLuaScript performs atomic token bucket rate limiting in Redis
const tokenBucketLuaScript = `
local key = KEYS[1]
local rate = tonumber(ARGV[1])        -- tokens added per second
local capacity = tonumber(ARGV[2])    -- maximum bucket burst capacity
local now = tonumber(ARGV[3])         -- current timestamp in seconds
local requested = tonumber(ARGV[4])   -- tokens requested

-- Fetch bucket state
local data = redis.call("HMGET", key, "tokens", "last_updated")
local tokens = tonumber(data[1])
local last_updated = tonumber(data[2])

if tokens == nil or last_updated == nil then
    tokens = capacity
    last_updated = now
else
    local delta = math.max(0, now - last_updated)
    tokens = math.min(capacity, tokens + delta * rate)
    last_updated = now
end

if tokens >= requested then
    tokens = tokens - requested
    redis.call("HMSET", key, "tokens", tokens, "last_updated", last_updated)
    redis.call("EXPIRE", key, math.ceil(capacity / rate) * 2)
    return {1, math.floor(tokens)} -- Allowed
else
    redis.call("HMSET", key, "tokens", tokens, "last_updated", last_updated)
    redis.call("EXPIRE", key, math.ceil(capacity / rate) * 2)
    return {0, math.floor(tokens)} -- Rate limited
end
`

type RedisRepository interface {
	AllowRateLimit(ctx context.Context, identifier string, rate, capacity int) (bool, int, error)
	SetCache(ctx context.Context, key string, value []byte, ttl time.Duration) error
	GetCache(ctx context.Context, key string) ([]byte, error)
	DeleteCache(ctx context.Context, key string) error
}

type redisRepositoryImpl struct {
	client *redis.Client
	script *redis.Script
}

func NewRedisRepository(client *redis.Client) RedisRepository {
	return &redisRepositoryImpl{
		client: client,
		script: redis.NewScript(tokenBucketLuaScript),
	}
}

func (r *redisRepositoryImpl) AllowRateLimit(ctx context.Context, identifier string, rate, capacity int) (bool, int, error) {
	key := fmt.Sprintf("rate_limit:%s", identifier)
	now := float64(time.Now().UnixNano()) / 1e9

	res, err := r.script.Run(ctx, r.client, []string{key}, rate, capacity, now, 1).Result()
	if err != nil {
		return false, 0, fmt.Errorf("failed to execute token bucket script: %w", err)
	}

	results, ok := res.([]interface{})
	if !ok || len(results) < 2 {
		return false, 0, fmt.Errorf("unexpected script output: %v", res)
	}

	allowed := results[0].(int64) == 1
	remaining := int(results[1].(int64))
	return allowed, remaining, nil
}

func (r *redisRepositoryImpl) SetCache(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl).Err()
}

func (r *redisRepositoryImpl) GetCache(ctx context.Context, key string) ([]byte, error) {
	return r.client.Get(ctx, key).Bytes()
}

func (r *redisRepositoryImpl) DeleteCache(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}
