package cache

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	utils "github.com/eru-tech/eru/eru-utils"
)

const (
	DEFAULT_CACHE_TABLE_NAME = "eru_cache"
)

type CacheTableColumn struct {
	TblSchema         string `json:"tbl_schema" eru:"required"`
	TblName           string `json:"tbl_name" eru:"required"`
	ColName           string `json:"col_name" eru:"required"`
	OwnDataType       string `json:"own_data_type" eru:"required"`
	PrimaryKey        bool   `json:"primary_key" eru:"required"`
	IsNullable        bool   `json:"is_nullable" eru:"required"`
	DefaultValue      string `json:"default_value"`
	AutoIncrement     bool   `json:"auto_increment"`
	CharMaxLength     int    `json:"char_max_length"`
	NumericPrecision  string `json:"numeric_precision"`
	NumericScale      int    `json:"numeric_scale"`
	DatetimePrecision int    `json:"datetime_precision"`
}

var ExpectedCacheTableSchema = map[string]CacheTableColumn{
	"project_id":    {ColName: "project_id", OwnDataType: "varchar", IsNullable: false, PrimaryKey: true},
	"cache_key":     {ColName: "cache_key", OwnDataType: "varchar", IsNullable: false, PrimaryKey: true},
	"cache_value":   {ColName: "cache_value", OwnDataType: "text", IsNullable: false, PrimaryKey: false},
	"created_at":    {ColName: "created_at", OwnDataType: "timestamp", IsNullable: false, PrimaryKey: false, DefaultValue: "CURRENT_TIMESTAMP"},
	"updated_at":    {ColName: "updated_at", OwnDataType: "timestamp", IsNullable: false, PrimaryKey: false, DefaultValue: "CURRENT_TIMESTAMP"},
	"expires_at":    {ColName: "expires_at", OwnDataType: "timestamp", IsNullable: true, PrimaryKey: false},
	"access_count":  {ColName: "access_count", OwnDataType: "bigint", IsNullable: false, PrimaryKey: false, DefaultValue: "0"},
	"last_accessed": {ColName: "last_accessed", OwnDataType: "timestamp", IsNullable: false, PrimaryKey: false, DefaultValue: "CURRENT_TIMESTAMP"},
}

// CacheStoreI defines the interface for a generic cache.
type CacheStoreI interface {
	Get(ctx context.Context, key string) (value string, err error)
	Set(ctx context.Context, key string, value interface{}) (err error)
	SetWithTTL(ctx context.Context, key string, value interface{}, ttl time.Duration) (err error)
	GetKeys(ctx context.Context, pattern string) ([]string, error)
	Delete(ctx context.Context, key string) error
	ValidatePersistence(ctx context.Context, projectId string) error
}

// CacheStore is a base struct to be embedded by specific implementations.
type CacheStore struct {
	CacheStoreType string `json:"cache_store_type"`
	CacheDbAlias   string `json:"cache_db_alias"`
	PersistEnabled bool   `json:"-"`
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

func (cs *CacheStore) ValidatePersistence(ctx context.Context, projectId string) error {
	logs.WithContext(ctx).Info("ValidatePersistence - Start")

	if cs.CacheDbAlias == "" {
		logs.WithContext(ctx).Info("Cache DB alias not set, persistence disabled")
		cs.PersistEnabled = false
		return nil
	}

	eruqlURL := os.Getenv("ERUQL_BASEURL")
	if eruqlURL == "" {
		logs.WithContext(ctx).Warn("ERUQL_BASEURL environment variable not set, persistence could not be enabled")
		cs.PersistEnabled = false
		return nil
	}

	tableExists, columns, err := cs.checkCacheTableExists(ctx, eruqlURL, projectId)
	if err != nil {
		cs.PersistEnabled = false
		return err
	}

	if !tableExists {
		logs.WithContext(ctx).Info("Cache table does not exist, creating it")
		err = cs.createCacheTable(ctx, eruqlURL, projectId)
		if err != nil {
			cs.PersistEnabled = false
			return err
		}
		return nil
	}

	isValidSchema := cs.validateTableColumns(ctx, columns)

	if !isValidSchema {
		err = logs.Err(ctx, fmt.Errorf("cache table schema is invalid for alias: %s", cs.CacheDbAlias), "Cache table schema is invalid")
		cs.PersistEnabled = false
		return err
	}

	cs.PersistEnabled = true
	return nil
}

func (cs *CacheStore) checkCacheTableExists(ctx context.Context, eruqlURL string, projectId string) (bool, map[string]CacheTableColumn, error) {
	logs.WithContext(ctx).Debug("checkCacheTableExists - Start")

	url := fmt.Sprintf("%s/%s/datasource/schema/%s/%s",
		strings.TrimSuffix(eruqlURL, "/"),
		projectId,
		cs.CacheDbAlias,
		DEFAULT_CACHE_TABLE_NAME)

	logs.WithContext(ctx).Info(fmt.Sprintf("Checking cache table existence at: %s", url))

	res, _, _, statusCode, err := utils.CallHttp(ctx, "GET", url, nil, nil, nil, nil, nil)
	if err != nil {
		err = logs.Err(ctx, err, "error calling eru-ql API")
		return false, nil, err
	}

	columns, ok := res.(map[string]CacheTableColumn)
	if !ok {
		err = logs.Err(ctx, fmt.Errorf("unexpected response format from eru-ql API"), "Cache table schema not found")
		return false, nil, err
	}

	if statusCode == http.StatusOK {
		return true, columns, nil
	} else {
		err = logs.Err(ctx, fmt.Errorf("eru-ql API returned status: %d", statusCode), "Cache table not found")
		return false, nil, err
	}
}

func (cs *CacheStore) validateTableColumns(ctx context.Context, actualColumns map[string]CacheTableColumn) bool {
	logs.WithContext(ctx).Debug("validateTableColumns - Start")

	var expectedCols []string
	var actualCols []string
	for colName := range ExpectedCacheTableSchema {
		expectedCols = append(expectedCols, colName)
	}

	for actualColName, actualCol := range actualColumns {
		expectedCol, exists := ExpectedCacheTableSchema[strings.ToLower(actualColName)]
		if !exists {
			_ = logs.Err(ctx, fmt.Errorf("unexpected column found: %s", actualColName), "Unexpected column found")
			return false
		}
		actualCols = append(actualCols, actualColName)

		if expectedCol.OwnDataType != actualCol.OwnDataType {
			_ = logs.Err(ctx, fmt.Errorf("column %s has incompatible data type. Expected: %s, Found: %s",
				actualColName, expectedCol.OwnDataType, actualCol.OwnDataType), "Column has incompatible data type")
			return false
		}

		if expectedCol.IsNullable != actualCol.IsNullable {
			_ = logs.Err(ctx, fmt.Errorf("column %s has incompatible nullable setting. Expected: %v, Found: %v",
				actualColName, expectedCol.IsNullable, actualCol.IsNullable), "Column has incompatible nullable setting")
			return false
		}
	}

	if len(expectedCols) != len(actualCols) {
		var missingCols []string
		for _, colName := range expectedCols {
			if !slices.Contains(actualCols, colName) {
				missingCols = append(missingCols, colName)
			}
		}
		_ = logs.Err(ctx, fmt.Errorf("missing required columns: %v", missingCols), "Missing required columns")
		return false
	}
	return true
}

func (cs *CacheStore) createCacheTable(ctx context.Context, eruqlURL string, projectId string) error {
	logs.WithContext(ctx).Debug("createCacheTable - Start")

	url := fmt.Sprintf("%s/%s/datasource/schema/%s/savetable/%s",
		strings.TrimSuffix(eruqlURL, "/"),
		projectId,
		cs.CacheDbAlias,
		DEFAULT_CACHE_TABLE_NAME)

	headers := http.Header{}
	headers.Set("Content-Type", "application/json")

	_, _, _, statusCode, err := utils.CallHttp(ctx, "POST", url, headers, nil, nil, nil, ExpectedCacheTableSchema)
	if err != nil {
		err = logs.Err(ctx, err, "error calling eru-ql create table API")
		return err
	}

	if statusCode == http.StatusOK {
		logs.WithContext(ctx).Info("Cache table created successfully")
		return nil
	} else {
		err = logs.Err(ctx, fmt.Errorf("eru-ql create table API returned status: %d", statusCode), "Cache table creation failed")
		return err
	}
}

// GetCacheStore is a factory function that returns a cache implementation.
func GetCacheStore(cacheStoreType string, projectId string) (cs CacheStoreI) {
	ctx := context.Background()
	logs.WithContext(ctx).Info(fmt.Sprintf("GetCacheStore called for type: %s", cacheStoreType))
	switch strings.ToUpper(cacheStoreType) {
	case "REDIS":
		redisCache, err := NewRedisCache()
		if err != nil {
			_ = logs.Err(ctx, err, "failed to create redis cache")
			return nil
		}
		cs = redisCache
	case "ETCD":
		etcdCache, err := NewEtcdCache()
		if err != nil {
			_ = logs.Err(ctx, err, "failed to create etcd cache")
			return nil
		}
		cs = etcdCache
	case "INMEMORY":
		cs = new(InMemoryCache)
	default:
		_ = logs.Err(ctx, fmt.Errorf("unsupported cache type: %s", cacheStoreType), "unsupported cache type")
		return nil
	}
	cs.ValidatePersistence(ctx, projectId)
	return cs
}
