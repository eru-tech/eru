package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCache is a Redis-backed cache implementation.
type RedisCache struct {
	CacheStore
	Client *redis.Client
}

// NewRedisCache creates and configures a new Redis cache client.
func NewRedisCache() (*RedisCache, error) {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		return nil, errors.New("REDIS_ADDR environment variable not set")
	}
	redisPassword := os.Getenv("REDIS_PASSWORD") // Can be empty
	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       0, // use default DB
	})
	// Check connection
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}
	return &RedisCache{
		CacheStore: CacheStore{CacheStoreType: "REDIS"},
		Client:     rdb,
	}, nil
}

func (rc *RedisCache) Get(ctx context.Context, key string) (string, error) {
	return rc.Client.Get(ctx, key).Result()
}

func (rc *RedisCache) Set(ctx context.Context, key string, value interface{}) error {
	return rc.SetWithTTL(ctx, key, value, 0) // 0 TTL means persist forever
}

func (rc *RedisCache) SetWithTTL(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	// Marshal the value to JSON if it's not a simple string/byte array
	p, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return rc.Client.Set(ctx, key, p, ttl).Err()
}

func (rc *RedisCache) GetKeys(ctx context.Context, pattern string) ([]string, error) {
	return rc.Client.Keys(ctx, pattern).Result()
}

func (rc *RedisCache) Delete(ctx context.Context, key string) error {
	return rc.Client.Del(ctx, key).Err()
}