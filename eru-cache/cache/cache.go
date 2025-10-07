package cache

import (
	"context"
	"encoding/json"
	"errors"
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
	"cache_sk":      {TblName: "eru_cache", ColName: "cache_sk", OwnDataType: "integer", IsNullable: false, PrimaryKey: true, AutoIncrement: true},
	"project_id":    {TblName: "eru_cache", ColName: "project_id", OwnDataType: "varchar", IsNullable: false, PrimaryKey: false, CharMaxLength: 100},
	"tenant_id":     {TblName: "eru_cache", ColName: "tenant_id", OwnDataType: "varchar", IsNullable: false, PrimaryKey: false, CharMaxLength: 100},
	"cache_key":     {TblName: "eru_cache", ColName: "cache_key", OwnDataType: "varchar", IsNullable: false, PrimaryKey: false, CharMaxLength: 500},
	"cache_value":   {TblName: "eru_cache", ColName: "cache_value", OwnDataType: "string", IsNullable: false, PrimaryKey: false},
	"created_at":    {TblName: "eru_cache", ColName: "created_at", OwnDataType: "datetime", IsNullable: false, PrimaryKey: false, DefaultValue: "CURRENT_TIMESTAMP"},
	"updated_at":    {TblName: "eru_cache", ColName: "updated_at", OwnDataType: "datetime", IsNullable: false, PrimaryKey: false, DefaultValue: "CURRENT_TIMESTAMP"},
	"expires_at":    {TblName: "eru_cache", ColName: "expires_at", OwnDataType: "datetime", IsNullable: true, PrimaryKey: false},
	"access_count":  {TblName: "eru_cache", ColName: "access_count", OwnDataType: "biginteger", IsNullable: false, PrimaryKey: false, DefaultValue: "0"},
	"last_accessed": {TblName: "eru_cache", ColName: "last_accessed", OwnDataType: "datetime", IsNullable: false, PrimaryKey: false, DefaultValue: "CURRENT_TIMESTAMP"},
}

type CacheData struct {
	CacheKey     string    `json:"cache_key"`
	CacheValue   string    `json:"cache_value"`
	ProjectId    string    `json:"project_id"`
	TenantId     string    `json:"tenant_id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	ExpiresAt    time.Time `json:"expires_at"`
	AccessCount  int       `json:"access_count"`
	LastAccessed time.Time `json:"last_accessed"`
}

// CacheStoreI defines the interface for a generic cache.
type CacheStoreI interface {
	Init(ctx context.Context) error
	Get(ctx context.Context, key string) (value string, err error)
	Set(ctx context.Context, key string, value interface{}) (err error)
	SetWithTTL(ctx context.Context, key string, value interface{}, ttl time.Duration) (err error)
	GetKeys(ctx context.Context, pattern string) ([]string, error)
	Delete(ctx context.Context, key string) error
	ValidatePersistence(ctx context.Context, projectId string) error
	MakeFromJson(ctx context.Context, rj *json.RawMessage) error
	GetAttribute(ctx context.Context, attributeName string) (attributeValue interface{}, err error)
	SyncPersistence(ctx context.Context, cacheStoreI CacheStoreI) error
	SyncToDatabase(ctx context.Context, projectId string, cacheData []CacheData) error
	LoadFromDatabase(ctx context.Context, projectId, tenantId, cacheKey string) ([]CacheData, error)
}

// CacheStore is a base struct to be embedded by specific implementations.
type CacheStore struct {
	CacheStoreType string `json:"cache_store_type"`
	CacheDbAlias   string `json:"cache_db_alias"`
	PersistEnabled bool   `json:"persist_enabled"`
	PersistError   bool   `json:"persist_error"`
}

func (cs *CacheStore) Init(ctx context.Context) error {
	return nil
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

func (cs *CacheStore) GetAttribute(ctx context.Context, attributeName string) (attributeValue interface{}, err error) {
	switch attributeName {
	case "cache_store_type":
		return cs.CacheStoreType, nil
	case "cache_db_alias":
		return cs.CacheDbAlias, nil
	case "persist_enabled":
		return cs.PersistEnabled, nil
	case "persist_error":
		return cs.PersistError, nil
	default:
		return nil, errors.New("attribute not found")
	}
}

func (cs *CacheStore) ValidatePersistence(ctx context.Context, projectId string) error {
	logs.WithContext(ctx).Info("ValidatePersistence - Start")

	if cs.CacheDbAlias == "" {
		logs.WithContext(ctx).Info("Cache DB alias not set, persistence disabled")
		cs.PersistEnabled = false
		cs.PersistError = true
		return nil
	}
	if cs.PersistEnabled && !cs.PersistError {
		return nil
	}

	eruqlURL := os.Getenv("ERUQL_BASEURL")
	if eruqlURL == "" {
		logs.WithContext(ctx).Warn("ERUQL_BASEURL environment variable not set, persistence could not be enabled")
		cs.PersistEnabled = false
		cs.PersistError = true
		return nil
	}

	tableExists, columns, schema, err := cs.checkCacheTableExists(ctx, eruqlURL, projectId)
	if err != nil {
		cs.PersistEnabled = false
		cs.PersistError = true
		return nil
	}

	if !tableExists {
		logs.WithContext(ctx).Info("Cache table does not exist, creating it")
		err = cs.createCacheTable(ctx, eruqlURL, projectId, schema)
		if err != nil {
			cs.PersistEnabled = false
			cs.PersistError = true
			return nil
		}
		return nil
	}

	isValidSchema := cs.validateTableColumns(ctx, columns)

	if !isValidSchema {
		_ = logs.Err(ctx, fmt.Errorf("cache table schema is invalid for alias: %s", cs.CacheDbAlias), "Cache table schema is invalid")
		cs.PersistEnabled = false
		cs.PersistError = true
		return nil
	}

	cs.PersistEnabled = true
	cs.PersistError = false
	return nil
}

func (cs *CacheStore) checkCacheTableExists(ctx context.Context, eruqlURL string, projectId string) (bool, map[string]CacheTableColumn, string, error) {
	logs.WithContext(ctx).Debug("checkCacheTableExists - Start")

	url := fmt.Sprintf("%s/store/%s/datasource/tablecheck/%s/%s",
		strings.TrimSuffix(eruqlURL, "/"),
		projectId,
		cs.CacheDbAlias,
		DEFAULT_CACHE_TABLE_NAME)

	logs.WithContext(ctx).Info(fmt.Sprintf("Checking cache table existence at: %s", url))
	tableExists := false
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, nil, nil, nil, nil)
	if err != nil {
		if strings.Contains(err.Error(), "Table not found") {
			return false, nil, "", nil
		}
		err = logs.Err(ctx, err, "error calling eru-ql API")
		return false, nil, "", err
	}
	if resMap, resMapOk := res.(map[string]interface{}); !resMapOk {
		err = logs.Err(ctx, fmt.Errorf("respons is not a map"), "fail to check cache table existence")
		return false, nil, "", err
	} else {
		columns := make(map[string]CacheTableColumn)
		schema := ""
		ok := false
		if columnsI, columnsIOk := resMap["columns"]; !columnsIOk {
			_ = logs.Err(ctx, fmt.Errorf("columns not found in response"), "fail to check cache table existence")
		} else {
			if columnsI != nil {
				columnsIBytes, err := json.Marshal(columnsI)
				if err != nil {
					_ = logs.Err(ctx, err, "fail to marshal columns")
				} else {
					err = json.Unmarshal(columnsIBytes, &columns)
					if err != nil {
						_ = logs.Err(ctx, err, "fail to unmarshal columns")
					} else {
						tableExists = true
					}
				}
			}
		}
		if schI, schIOk := resMap["schema"]; !schIOk {
			_ = logs.Err(ctx, fmt.Errorf("schema not found in response"), "fail to check cache table existence")
		} else {
			if schema, ok = schI.(string); !ok {
				_ = logs.Err(ctx, fmt.Errorf("schema is not a string"), "fail to check cache table existence")
			}
		}
		return tableExists, columns, schema, nil
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

		if !strings.EqualFold(strings.ToLower(expectedCol.OwnDataType), strings.ToLower(actualCol.OwnDataType)) {
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

func (cs *CacheStore) createCacheTable(ctx context.Context, eruqlURL string, projectId string, schema string) error {
	logs.WithContext(ctx).Debug("createCacheTable - Start")

	url := fmt.Sprintf("%s/store/%s/datasource/schema/%s/savetable/%s/true",
		strings.TrimSuffix(eruqlURL, "/"),
		projectId,
		cs.CacheDbAlias,
		DEFAULT_CACHE_TABLE_NAME)

	headers := http.Header{}
	headers.Set("Content-Type", "application/json")

	tableObj := make(map[string]CacheTableColumn)
	for colName, col := range ExpectedCacheTableSchema {
		col.TblSchema = schema
		tableObj[colName] = col
	}
	tableObjBytes, _ := json.Marshal(tableObj)
	logs.WithContext(ctx).Info(fmt.Sprintf("tableObjBytes: %s", string(tableObjBytes)))
	logs.WithContext(ctx).Info(fmt.Sprintf("ExpectedCacheTableSchema: %v", ExpectedCacheTableSchema))
	res, _, _, statusCode, err := utils.CallHttp(ctx, "POST", url, headers, nil, nil, nil, tableObj)
	if err != nil {
		err = logs.Err(ctx, err, "error calling eru-ql create table API")
		return err
	}
	logs.WithContext(ctx).Info(fmt.Sprintf("res: %v", res))
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
		cs, _ = NewRedisCache()
		//ignore error as return blank cache store as returned by NewRedisCache
	case "ETCD":
		cs, _ = NewEtcdCache()
		//ignore error as return blank cache store as returned by NewEtcdCache
	case "INMEMORY":
		cs = new(InMemoryCache)
	default:
		_ = logs.Err(ctx, fmt.Errorf("unsupported cache type: %s", cacheStoreType), "unsupported cache type")
		return nil
	}
	cs.ValidatePersistence(ctx, projectId)
	return cs
}
func (cs *CacheStore) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	err := logs.Err(ctx, errors.New("not implemented"), "not implemented")
	return err
}
func (cs *CacheStore) SyncPersistence(ctx context.Context, cacheStoreI CacheStoreI) error {
	logs.WithContext(ctx).Debug("SyncPersistence - Start")

	cacheDbAliasI, cacheDbAliasErr := cacheStoreI.GetAttribute(ctx, "cache_db_alias")
	if cacheDbAliasErr != nil {
		return cacheDbAliasErr
	}
	cacheDbAlias, ok := cacheDbAliasI.(string)
	if !ok {
		return fmt.Errorf("cache_db_alias is not a string")
	}
	cacheStoreTypeI, cacheStoreTypeErr := cacheStoreI.GetAttribute(ctx, "cache_store_type")
	if cacheStoreTypeErr != nil {
		return cacheStoreTypeErr
	}
	cacheStoreType, ok := cacheStoreTypeI.(string)
	if !ok {
		return fmt.Errorf("cache_store_type is not a string")
	}
	if !(strings.EqualFold(cs.CacheDbAlias, cacheDbAlias) && strings.EqualFold(cs.CacheStoreType, cacheStoreType)) {
		//return with no error without syncing so validate will be called again with new dbalias and storetype
		return nil
	}

	peI, peErr := cacheStoreI.GetAttribute(ctx, "persist_enabled")
	if peErr != nil {
		return peErr
	}
	pe, ok := peI.(bool)
	if !ok {
		err := logs.Err(ctx, fmt.Errorf("persist_enabled is not a boolean"), "persist_enabled is not a boolean")
		return err
	}
	cs.PersistEnabled = pe

	peI, peErr = cacheStoreI.GetAttribute(ctx, "persist_error")
	if peErr != nil {
		return peErr
	}
	pe, ok = peI.(bool)
	if !ok {
		err := logs.Err(ctx, fmt.Errorf("persist_error is not a boolean"), "persist_error is not a boolean")
		return err
	}
	cs.PersistError = pe
	return nil
}

func (cs *CacheStore) SyncToDatabase(ctx context.Context, projectId string, cacheData []CacheData) error {
	logs.WithContext(ctx).Debug("SyncToDatabase - Start")

	if !cs.PersistEnabled {
		logs.WithContext(ctx).Info("Persistence not enabled, skipping cache data sync")
		return nil
	}

	if cs.CacheDbAlias == "" {
		return fmt.Errorf("cache database alias not configured")
	}

	eruqlURL := os.Getenv("ERUQL_BASEURL")
	if eruqlURL == "" {
		return fmt.Errorf("ERUQL_BASEURL environment variable not set")
	}

	mutation := `
	mutation {
  results : insert_public___eru_cache(docs: $docs) $$dbalias$$  {
   err : error
  }
}`
	mutation = strings.Replace(mutation, "$$dbalias$$", fmt.Sprintf("@%s", cs.CacheDbAlias), -1)

	variables := map[string]interface{}{
		"docs": cacheData,
	}

	requestBody := map[string]interface{}{
		"query":     mutation,
		"variables": variables,
	}

	headers := http.Header{}
	headers.Set("Content-Type", "application/json")

	url := fmt.Sprintf("%s/graphql/%s/execute", strings.TrimSuffix(eruqlURL, "/"), projectId)
	b, _ := json.Marshal(requestBody)
	logs.WithContext(ctx).Info(fmt.Sprintf("requestBody: %s", string(b)))
	_, _, _, statusCode, err := utils.CallHttp(ctx, "POST", url, headers, nil, nil, nil, requestBody)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("Failed to sync cache data to database: %v", err))
		return err
	}

	if statusCode != http.StatusOK {
		logs.WithContext(ctx).Error(fmt.Sprintf("Cache data sync returned status: %d", statusCode))
		return fmt.Errorf("cache data sync failed with status: %d", statusCode)
	}

	logs.WithContext(ctx).Info(fmt.Sprintf("Successfully synced cache data to database: %d", len(cacheData)))
	return nil
}

func (cs *CacheStore) LoadFromDatabase(ctx context.Context, projectId, tenantId, cacheKey string) (cacheData []CacheData, err error) {
	logs.WithContext(ctx).Debug("LoadFromDatabase - Start")

	if !cs.PersistEnabled {
		logs.WithContext(ctx).Info("Persistence not enabled, skipping database load")
		return nil, nil
	}

	if cs.CacheDbAlias == "" {
		return nil, fmt.Errorf("cache database alias not configured")
	}

	eruqlURL := os.Getenv("ERUQL_BASEURL")
	if eruqlURL == "" {
		return nil, fmt.Errorf("ERUQL_BASEURL environment variable not set")
	}

	query := `
	query {
  cache: public___eru_cache (where : $where) $$dbalias$$  {
    project_id
    tenant_id
    cache_key
    cache_value
    created_at
    updated_at
    expires_at
    access_count
    last_accessed
  }
}`
	query = strings.Replace(query, "$$dbalias$$", fmt.Sprintf("@%s", cs.CacheDbAlias), -1)

	variables := map[string]interface{}{
		"where": map[string]interface{}{
			"cache_key":  cacheKey,
			"project_id": projectId,
			"tenant_id":  tenantId,
		},
	}

	requestBody := map[string]interface{}{
		"query":     query,
		"variables": variables,
	}

	headers := http.Header{}
	headers.Set("Content-Type", "application/json")

	url := fmt.Sprintf("%s/graphql/%s/execute", strings.TrimSuffix(eruqlURL, "/"), projectId)

	res, _, _, statusCode, err := utils.CallHttp(ctx, "POST", url, headers, nil, nil, nil, requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to query cache data from database: %v", err)
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("database query returned status: %d", statusCode)
	}

	if res == nil {
		logs.WithContext(ctx).Info("No cache data found in database")
		return []CacheData{}, nil
	}
	if resArray, ok := res.([]interface{}); !ok {
		return nil, fmt.Errorf("unexpected response format from database")
	} else if len(resArray) > 0 {
		responseData, ok := resArray[0].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("unexpected response format from database")
		}
		cacheData, ok := responseData["cache"].([]CacheData)
		if !ok {
			// Try to convert each entry individually
			if cacheEntries, ok := responseData["cache"].([]interface{}); ok {
				cacheData = make([]CacheData, len(cacheEntries))
				for i, entry := range cacheEntries {
					if entryMap, ok := entry.(map[string]interface{}); ok {
						entryBytes, err := json.Marshal(entryMap)
						if err != nil {
							return nil, fmt.Errorf("failed to marshal cache entry: %v", err)
						}
						if err := json.Unmarshal(entryBytes, &cacheData[i]); err != nil {
							return nil, fmt.Errorf("failed to unmarshal cache entry: %v", err)
						}
					}
				}
				return cacheData, nil
			}
			return []CacheData{}, nil
		}
		return cacheData, nil

	} else {
		return []CacheData{}, nil
	}
}
