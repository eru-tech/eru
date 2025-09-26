package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

// InMemoryCache is an in-memory cache implementation.
type InMemoryCache struct {
	CacheStore
	CacheValues map[string]CacheValue `json:"cache_values"`
}

// CacheValue holds the data for an in-memory cache item.
type CacheValue struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
}

// NewInMemoryCache creates a new in-memory cache.
func NewInMemoryCache() *InMemoryCache {
	return &InMemoryCache{
		CacheStore:  CacheStore{CacheStoreType: "INMEMORY"},
		CacheValues: make(map[string]CacheValue),
	}
}

func (imc *InMemoryCache) Get(ctx context.Context, key string) (string, error) {
	if imc.CacheValues != nil {
		if cv, cvOk := imc.CacheValues[key]; cvOk {
			logs.WithContext(ctx).Info(fmt.Sprintf("cache key %s found", key))
			// If value is already a string, return it directly
			if str, ok := cv.Value.(string); ok {
				return str, nil
			}
			// Otherwise marshal to string to match the interface
			res, err := json.Marshal(cv.Value)
			if err != nil {
				return "", err
			}
			return string(res), nil
		}
	}
	return "", fmt.Errorf("cache key %s not found", key)
}

func (imc *InMemoryCache) Set(ctx context.Context, key string, value interface{}) error {
	if imc.CacheValues == nil {
		imc.CacheValues = make(map[string]CacheValue)
	}
	imc.CacheValues[key] = CacheValue{Key: key, Value: value}
	return nil
}

func (imc *InMemoryCache) SetWithTTL(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	// In-memory TTL is not implemented, so we just call Set
	if ttl > 0 {
		logs.WithContext(ctx).Warn("InMemoryCache does not support TTL. Setting value without expiration.")
	}
	return imc.Set(ctx, key, value)
}

func (imc *InMemoryCache) GetKeys(ctx context.Context, pattern string) ([]string, error) {
	// This is inefficient for in-memory but fine for this example.
	// A real implementation might use a more complex pattern matcher.
	logs.WithContext(ctx).Warn("InMemoryCache GetKeys pattern matching is not fully implemented and returns all keys.")
	var keys []string
	for k := range imc.CacheValues {
		keys = append(keys, k)
	}
	return keys, nil
}

func (imc *InMemoryCache) Delete(ctx context.Context, key string) error {
	if imc.CacheValues != nil {
		delete(imc.CacheValues, key)
	}
	return nil
}
func (imc *InMemoryCache) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &imc)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}
