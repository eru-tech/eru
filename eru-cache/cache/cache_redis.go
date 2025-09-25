package cache

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"time"

	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/redis/go-redis/v9"
)

// RedisCache is a Redis-backed cache implementation.
type RedisCache struct {
	CacheStore
	RedisAddr     string `json:"redis_addr" eru:"required"`
	RedisPassword string `json:"redis_password" eru:"required"`
	RedisDB       int    `json:"redis_db" eru:"required"`
	Client        *redis.Client
}

// NewRedisCache creates and configures a new Redis cache client.
func NewRedisCache() (rc *RedisCache, err error) {
	rc = &RedisCache{
		CacheStore: CacheStore{CacheStoreType: "REDIS"},
	}
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		err = errors.New("REDIS_ADDR environment variable not set")
		return
	}
	redisPassword := os.Getenv("REDIS_PASSWORD") // Can be empty
	rc.RedisAddr = redisAddr
	rc.RedisPassword = redisPassword
	rc.RedisDB = 0

	err = rc.Init(context.Background())
	if err != nil {
		return
	}
	return
}

func (rc *RedisCache) Init(ctx context.Context) error {
	rdb := redis.NewClient(&redis.Options{
		Addr:     rc.RedisAddr,
		Password: rc.RedisPassword,
		DB:       rc.RedisDB,
	})
	// Check connection
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		err = logs.Err(ctx, err, "failed to connect to redis")
		return err
	}
	rc.Client = rdb
	return nil
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
func (rc *RedisCache) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &rc)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}
