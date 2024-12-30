package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type DbStore struct {
	Store
	DbType               string    `json:"db_type"`
	UpdateTime           time.Time `json:"update_time"`
	StoreTableName       string    `json:"store_table_name"`
	StoreTenantTableName string    `json:"store_tenant_table_name"`
	storeType            string
	conStr               string
	Con                  *sqlx.DB `json:"-"`
	ConStatus            bool     `json:"-"`
}

type Queries struct {
	Query string
	Vals  []interface{}
}

func getStoreDbPath() string {
	dbString := os.Getenv("STORE_DB_PATH")
	dbUser := os.Getenv("STORE_DB_USER")
	dbPassword := os.Getenv("STORE_DB_PASSWORD")
	dbString = strings.Replace(strings.Replace(dbString, "ENV_STORE_DB_USER", dbUser, -1), "ENV_STORE_DB_PASSWORD", dbPassword, -1)
	return dbString
}
func (store *DbStore) SetDbType(dbtype string) {
	store.DbType = strings.ToLower(dbtype)
}

func (store *DbStore) GetDbType() string {
	return store.DbType
}

func (store *DbStore) SetStoreTableName(tablename string) {
	store.StoreTableName = strings.ToLower(tablename)
}
func (store *DbStore) SetStoreTenantTableName(tablename string) {
	store.StoreTenantTableName = strings.ToLower(tablename)
}

func (store *DbStore) GetStoreTableName() (tablename string) {
	return store.StoreTableName
}
func (store *DbStore) GetStoreTenantTableName() (tablename string) {
	return store.StoreTenantTableName
}

func (store *DbStore) GetStoreByteArray(dbString string) (b []byte, err error) {
	//TODO to implement this function
	logs.Logger.Debug("GetStoreByteArray - Start")
	if dbString == "" {
		dbString = getStoreDbPath()
		if dbString == "" {
			err := errors.New("no value found for environment variable STORE_DB_PATH")
			logs.Logger.Error(err.Error())
			return nil, err
		}
	}
	logs.Logger.Info("Creating DB connection for GetStoreByteArray")
	db, err := sqlx.Open(store.DbType, dbString)
	if err != nil {
		logs.Logger.Error(err.Error())
		return nil, err
	}
	defer db.Close()
	logs.Logger.Info(fmt.Sprint("db connection succesfull - fetch config from ", store.StoreTableName))
	loadQuery := fmt.Sprint("select config, create_date from ", store.StoreTableName, " limit 1")
	if store.StoreTenantTableName != "" {
		loadQuery = fmt.Sprint("with prj as (select b.*  from ", store.StoreTableName, " a, jsonb_each(config->'projects') b), pt as (select project_id, max(update_date) create_date, jsonb_object_agg(tenant_id,config) tenant_config from ", store.StoreTenantTableName, " group by project_id), fpt as (select max(create_date) create_date, jsonb_object_agg(a.key , a.value||jsonb_build_object('tenants',coalesce(b.tenant_config,'{}'::jsonb))) project_config from prj a left join pt b on a.key=b.project_id) select a.config||jsonb_build_object('projects',coalesce(b.project_config,'{}'::jsonb)) config, greatest(a.create_date,b.create_date) create_date from eruai_config a left join fpt b on 1=1")
	}
	logs.Logger.Info(loadQuery)
	rows, err := db.Queryx(loadQuery)

	if err != nil {
		logs.Logger.Error(err.Error())
		return nil, err
	}
	logs.Logger.Info("config fetched succesfully")
	mapping := make(map[string]interface{})
	var storeData interface{}
	for rows.Next() {
		err = rows.MapScan(mapping)
		if err != nil {
			logs.Logger.Error(err.Error())
			return nil, err
		}
		storeData = mapping["config"]
		storeUpdateTime := mapping["create_date"]
		store.UpdateTime = storeUpdateTime.(time.Time)
	}
	if storeData == nil {
		err = errors.New("no config data retrived from db")
		logs.Logger.Error(err.Error())
		return nil, err
	}
	logs.Logger.Info("config loaded successfully")
	return storeData.([]byte), err
}

func (store *DbStore) LoadStore(dbString string, ms StoreI) (err error) {
	logs.Logger.Debug("LoadStore - Start")
	if dbString == "" {
		dbString = getStoreDbPath()
	}
	logs.Logger.Info("Creating DB connection for Load DB store")
	db, err := sqlx.Open(store.DbType, dbString)
	if err != nil {
		logs.Logger.Error(err.Error())
		return err
	}
	defer db.Close()
	loadQuery := fmt.Sprint("select * from ", store.StoreTableName, " limit 1")
	if store.StoreTenantTableName != "" {
		loadQuery = fmt.Sprint("with prj as (select b.*  from ", store.StoreTableName, " a, jsonb_each(config->'projects') b), pt as (select project_id, max(update_date) create_date, jsonb_object_agg(tenant_id,config) tenant_config from ", store.StoreTenantTableName, " group by project_id), fpt as (select max(create_date) create_date, jsonb_object_agg(a.key , a.value||jsonb_build_object('tenants',coalesce(b.tenant_config,'{}'::jsonb))) project_config from prj a left join pt b on a.key=b.project_id) select a.config||jsonb_build_object('projects',coalesce(b.project_config,'{}'::jsonb)) config, greatest(a.create_date,b.create_date) create_date from eruai_config a left join fpt b on 1=1")
	}

	logs.Logger.Info(loadQuery)
	rows, err := db.Queryx(loadQuery)
	if err != nil {
		logs.Logger.Error(err.Error())
		return err
	}
	mapping := make(map[string]interface{})
	for rows.Next() {
		err = rows.MapScan(mapping)
		if err != nil {
			logs.Logger.Error(err.Error())
			return err
		}
		storeData := mapping["config"]
		storeUpdateTime := mapping["create_date"]
		// Marshalling the store
		//store = new(FileStore)
		err = json.Unmarshal(storeData.([]byte), ms)
		if err != nil {
			logs.Logger.Error(err.Error())
			return err
		}
		store.UpdateTime = storeUpdateTime.(time.Time)
		logs.Logger.Info(fmt.Sprint("storeUpdateTime == ", storeUpdateTime))
	}
	if err != nil {
		logs.Logger.Error(err.Error())
		return err
	}
	return nil
}

func (store *DbStore) SaveStore(ctx context.Context, projectId string, dbString string, ms StoreI) (err error) {
	logs.WithContext(ctx).Debug("SaveStore - Start")

	if dbString == "" {
		dbString = getStoreDbPath()
	}
	logs.WithContext(ctx).Info("Creating DB connection for Save DB store")
	db, err := sqlx.Open(store.DbType, dbString)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	defer db.Close()

	tx := db.MustBegin()
	//storeData, err := json.Marshal(ms)
	storeData, err := ms.GetStoreWithoutTenants(context.Background(), ms)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		tx.Rollback()
		return err
	}
	strStoreData := strings.Replace(string(storeData), "'", "''", -1)
	query := fmt.Sprint("update ", store.StoreTableName, " set create_date=current_timestamp , config = '", strStoreData, "' returning create_date")
	/* if store.StoreTenantTableName != "" {
		query = fmt.Sprint("update ", store.StoreTableName, " x set create_date=current_timestamp , config = y.config from (with json_data as (select '", strStoreData, "'::jsonb as config) SELECT jsonb_set(config,'{projects}',(SELECT jsonb_object_agg(key,value - 'tenants') FROM jsonb_each(config->'projects') ) ) AS config FROM json_data) y where 1=1 returning x.create_date")
	} */
	stmt, err := tx.PreparexContext(ctx, query)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprint("Error in tx.PreparexContext : ", err.Error()))
		tx.Rollback()
		return err
	}
	rw, err := stmt.QueryxContext(ctx)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprint("Error in stmt.QueryxContext : ", err.Error()))
		tx.Rollback()
		return err
	}
	for rw.Rows.Next() {
		resDoc := make(map[string]interface{})
		err = rw.MapScan(resDoc)
		if err != nil {
			logs.WithContext(ctx).Error(fmt.Sprint("Error in rw.MapScan : ", err.Error()))
			tx.Rollback()
			return err
		}
		store.UpdateTime = resDoc["create_date"].(time.Time)
	}
	err = tx.Commit()
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprint("Error in tx.Commit : ", err.Error()))
		tx.Rollback()
	}

	if store.ProjectRepos != nil {
		if repo, repoOk := store.ProjectRepos[projectId]; repoOk {
			if repo.GetAttribute("auto_commit").(bool) {
				err = ms.CommitRepo(ctx, projectId, ms)
				if err != nil {
					logs.WithContext(ctx).Warn(fmt.Sprint("auto commit failed : ", err.Error()))
				} else {
					logs.WithContext(ctx).Info(fmt.Sprint("store changes successfully committed to repo for project : ", projectId))
				}
			}
		}
	}
	return nil
}
func (store *DbStore) SaveTenantStore(ctx context.Context, projectId string, tenantId string, dbString string, tenantConfig interface{}) (err error) {
	logs.WithContext(ctx).Debug("SaveTenantStore - Start")
	if dbString == "" {
		dbString = getStoreDbPath()
	}
	logs.WithContext(ctx).Info("Creating DB connection for Save DB store")
	db, err := sqlx.Open(store.DbType, dbString)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	defer db.Close()

	tx := db.MustBegin()
	storeData, err := json.Marshal(tenantConfig)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		tx.Rollback()
		return err
	}
	strStoreData := strings.Replace(string(storeData), "'", "''", -1)

	query := fmt.Sprint("INSERT INTO ", store.StoreTenantTableName, " (tenant_id,project_id,config) VALUES ('", tenantId, "','", projectId, "','", strStoreData, "') ON CONFLICT (tenant_id) DO UPDATE SET config = EXCLUDED.config, update_date=CURRENT_TIMESTAMP;")
	stmt, err := tx.PreparexContext(ctx, query)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprint("Error in tx.PreparexContext : ", err.Error()))
		tx.Rollback()
		return err
	}
	rw, err := stmt.QueryxContext(ctx)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprint("Error in stmt.QueryxContext : ", err.Error()))
		tx.Rollback()
		return err
	}
	for rw.Rows.Next() {
		resDoc := make(map[string]interface{})
		err = rw.MapScan(resDoc)
		if err != nil {
			logs.WithContext(ctx).Error(fmt.Sprint("Error in rw.MapScan : ", err.Error()))
			tx.Rollback()
			return err
		}
		store.UpdateTime = resDoc["create_date"].(time.Time)
	}
	err = tx.Commit()
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprint("Error in tx.Commit : ", err.Error()))
		tx.Rollback()
	}

	//TODO: implement repo commit for tennat config changes

	/* if store.ProjectRepos != nil {
		if repo, repoOk := store.ProjectRepos[projectId]; repoOk {
			if repo.GetAttribute("auto_commit").(bool) {
				err = ms.CommitRepo(ctx, projectId, ms)
				if err != nil {
					logs.WithContext(ctx).Warn(fmt.Sprint("auto commit failed : ", err.Error()))
				} else {
					logs.WithContext(ctx).Info(fmt.Sprint("store changes successfully committed to repo for project : ", projectId))
				}
			}
		}
	} */
	return nil
}

func (store *DbStore) getStoreDBConnStr() (string, error) {
	logs.Logger.Debug("getStoreDBConnStr - Start")
	dbConStr := os.Getenv("storedb")
	if dbConStr == "" {
		err := errors.New(fmt.Sprint("storedb environment variable not found"))
		logs.Logger.Error(err.Error())
		return "", err
	}
	return dbConStr, nil
}

func (store *DbStore) CreateConn() error {
	logs.Logger.Debug("CreateConn - Start")
	connString := getStoreDbPath()
	db, err := sqlx.Open(store.DbType, connString)
	if err != nil {
		logs.Logger.Error(err.Error())
		store.ConStatus = false
		return err
	}
	logs.Logger.Info("db connection was successfully done for fetch dummy query")
	_, err = db.Queryx("select 1")
	if err != nil {
		store.ConStatus = false
		logs.Logger.Error(err.Error())
		return err
	}
	logs.Logger.Info("dummy query success - setting con as true")
	store.Con = db
	store.ConStatus = true
	return nil
}

func (store *DbStore) GetConn() *sqlx.DB {
	logs.Logger.Debug("CreateConn - Start")
	return store.Con
}

func (store *DbStore) ExecuteDbFetch(ctx context.Context, query Queries) (output []map[string]interface{}, err error) {
	logs.WithContext(ctx).Debug("ExecuteDbFetch - Start")

	db := store.GetConn()
	if db == nil {
		err = store.CreateConn()
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
			return nil, err
		}
		db = store.GetConn()
	}

	rows, err := db.Queryx(query.Query, query.Vals...)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
	mapping := make(map[string]interface{})
	colsType, ee := rows.ColumnTypes()
	if ee != nil {
		return nil, ee
	}
	for rows.Next() {
		innerResultRow := make(map[string]interface{})
		ee = rows.MapScan(mapping)
		if ee != nil {
			return nil, ee
		}
		for _, colType := range colsType {
			if colType.DatabaseTypeName() == "NUMERIC" && mapping[colType.Name()] != nil {
				f := 0.0
				if reflect.TypeOf(mapping[colType.Name()]).String() == "[]uint8" {
					f, err = strconv.ParseFloat(string(mapping[colType.Name()].([]byte)), 64)
					mapping[colType.Name()] = f
				} else if reflect.TypeOf(mapping[colType.Name()]).String() == "float64" {
					f = mapping[colType.Name()].(float64)
					mapping[colType.Name()] = f
				}
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					return nil, err
				}
			} else if (colType.DatabaseTypeName() == "JSONB" || colType.DatabaseTypeName() == "JSON") && mapping[colType.Name()] != nil {
				bytesToUnmarshal := mapping[colType.Name()].([]byte)
				var v interface{}
				err = json.Unmarshal(bytesToUnmarshal, &v)
				if err != nil {
					return nil, err
				}
				mapping[colType.Name()] = &v
			}
			innerResultRow[colType.Name()] = mapping[colType.Name()]
		}
		output = append(output, innerResultRow)
	}
	return
}

func (store *DbStore) ExecuteDbSave(ctx context.Context, queries []Queries) (output [][]map[string]interface{}, err error) {
	logs.WithContext(ctx).Debug("ExecuteDbSave - Start")
	db := store.GetConn()
	if db == nil {
		err = store.CreateConn()
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
			return nil, err
		}
		db = store.GetConn()
	}
	tx := db.MustBegin()
	for _, q := range queries {
		//logs.WithContext(ctx).Info(q.Query)
		//logs.WithContext(ctx).Info(fmt.Sprint(q.Vals))
		stmt, err := tx.PreparexContext(ctx, q.Query)
		if err != nil {
			logs.WithContext(ctx).Error(fmt.Sprint("Error in tx.PreparexContext : ", err.Error()))
			tx.Rollback()
			return nil, err
		}
		rw, err := stmt.QueryxContext(ctx, q.Vals...)
		if err != nil {
			logs.WithContext(ctx).Error(fmt.Sprint("Error in stmt.QueryxContext : ", err.Error()))
			tx.Rollback()
			return nil, err
		}
		var innerOutput []map[string]interface{}
		for rw.Rows.Next() {
			resDoc := make(map[string]interface{})
			err = rw.MapScan(resDoc)
			if err != nil {
				logs.WithContext(ctx).Error(fmt.Sprint("Error in rw.MapScan : ", err.Error()))
				tx.Rollback()
				return nil, err
			}
			innerOutput = append(innerOutput, resDoc)
		}
		output = append(output, innerOutput)
	}
	err = tx.Commit()
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprint("Error in tx.Commit : ", err.Error()))
		tx.Rollback()
	}
	return
}
