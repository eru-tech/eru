package cache

import (
	"context"
	"fmt"
	"strings"
	"time"

	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

// CacheStoreI defines the interface for a generic cache.
type CacheStoreI interface {
	Get(ctx context.Context, key string) (value string, err error)
	Set(ctx context.Context, key string, value interface{}) (err error)
	SetWithTTL(ctx context.Context, key string, value interface{}, ttl time.Duration) (err error)
	GetKeys(ctx context.Context, pattern string) ([]string, error)
	Delete(ctx context.Context, key string) error
}

// CacheStore is a base struct to be embedded by specific implementations.
type CacheStore struct {
	CacheStoreType string `json:"cache_store_type"`
}

func (cs *CacheStore) Delete(ctx context.Context, key string) error {
	return nil
}
func (cs *CacheStore) Get(ctx context.Context, key string) (value string, err error) {
	return "", nil
}
func (cs *CacheStore) Set(ctx context.Context, key string, value interface{}) (err error) {
	return nil
}
func (cs *CacheStore) SetWithTTL(ctx context.Context, key string, value interface{}, ttl time.Duration) (err error) {
	return nil
}
func (cs *CacheStore) GetKeys(ctx context.Context, pattern string) ([]string, error) {
	return nil, nil
}

// GetCacheStore is a factory function that returns a cache implementation.
func GetCacheStore(cacheStoreType string) CacheStoreI {
	logs.WithContext(context.TODO()).Info(fmt.Sprintf("GetCacheStore called for type: %s", cacheStoreType))
	switch strings.ToUpper(cacheStoreType) {
	case "REDIS":
		redisCache, err := NewRedisCache()
		if err != nil {
			logs.WithContext(context.TODO()).Error(fmt.Sprintf("failed to create redis cache: %v", err))
			return nil
		}
		return redisCache
	case "ETCD":
		etcdCache, err := NewEtcdCache()
		if err != nil {
			logs.WithContext(context.TODO()).Error(fmt.Sprintf("failed to create etcd cache: %v", err))
			return nil
		}
		return etcdCache
	case "INMEMORY":
		return new(InMemoryCache)
	default:
		logs.WithContext(context.TODO()).Error(fmt.Sprintf("unsupported cache type: %s", cacheStoreType))
		return new(CacheStore)
	}
}
