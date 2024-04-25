package cache

import (
	"context"
	"errors"
	"fmt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

type CacheStore struct {
	CacheStoreType string                `json:"cache_store_type" eru:"required"`
	CacheValues    map[string]CacheValue `json:"cache_values" eru:"required"`
}

type CacheValue struct {
	Key   string      `json:"key" eru:"required"`
	Value interface{} `json:"key" eru:"required"`
}

type CacheStoreI interface {
	Get(ctx context.Context, key string) (value interface{}, err error)
	Set(ctx context.Context, key string, value interface{}) (err error)
}

func GetCacheStore(cacheStoreType string) CacheStoreI {
	logs.WithContext(context.TODO()).Info("GetCacheStore called")
	switch cacheStoreType {
	case "ERU":
		cs := CacheStore{CacheStoreType: "ERU", CacheValues: make(map[string]CacheValue)}
		return &cs
	default:
		return nil
	}
	return nil
}

func (cacheStore *CacheStore) Get(ctx context.Context, key string) (value interface{}, err error) {
	if cacheStore.CacheValues != nil {
		if cv, cvOk := cacheStore.CacheValues[key]; cvOk {
			logs.WithContext(ctx).Info(fmt.Sprint("cache key ", key, " found"))
			return cv.Value, nil
		} else {
			err = errors.New(fmt.Sprint("cache key ", key, " not found"))
			return
		}
	} else {
		err = errors.New("cache values map is not defined")
		return
	}
}

func (cacheStore *CacheStore) Set(ctx context.Context, key string, value interface{}) (err error) {
	if cacheStore.CacheValues == nil {
		cacheStore.CacheValues = make(map[string]CacheValue)
	}
	cacheStore.CacheValues[key] = CacheValue{Key: key, Value: value}
	return
}
