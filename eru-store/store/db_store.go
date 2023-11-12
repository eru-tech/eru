package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

type DbStore struct {
	Store
	DbType         string
	UpdateTime     time.Time
	StoreTableName string
	storeType      string
	conStr         string
	Con            *sqlx.DB `json:"-"`
	ConStatus      bool     `json:"-"`
}

type Queries struct {
	Query string
	Vals  []interface{}
}

func getStoreDbPath() string {
	dbString := os.Getenv("STORE_DB_PATH")
	logs.Logger.Debug(fmt.Sprint("dbString = ", dbString))
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

func (store *DbStore) GetStoreByteArray(dbString string) (b []byte, err error) {
	//TODO to implement this function
	logs.Logger.Debug("GetStoreByteArray - Start")
	if dbString == "" {
		dbString = getStoreDbPath()
		if dbString == "" {
			err := errors.New("No value found for environment variable STORE_DB_PATH")
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
	rows, err := db.Queryx(fmt.Sprint("select * from ", store.StoreTableName, " limit 1"))
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
	defer db.Close()
	if err != nil {
		logs.Logger.Error(err.Error())
		return err
	}
	rows, err := db.Queryx(fmt.Sprint("select * from ", store.StoreTableName, " limit 1"))
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

func (store *DbStore) SaveStore(ctx context.Context, dbString string, ms StoreI) (err error) {
	logs.WithContext(ctx).Debug("SaveStore - Start")
	if dbString == "" {
		dbString = getStoreDbPath()
	}
	logs.WithContext(ctx).Info("Creating DB connection for Save DB store")
	db, err := sqlx.Open(store.DbType, dbString)
	defer db.Close()
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	tx := db.MustBegin()
	storeData, err := json.Marshal(ms)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		tx.Rollback()
		return err
	}
	strStoreData := strings.Replace(string(storeData), "'", "''", -1)

	if strings.Contains(strStoreData, "IntrospectionQuery") {
		logs.WithContext(ctx).Info(fmt.Sprint("IntrospectionQuery found in store config"))
	}

	query := fmt.Sprint("update ", store.StoreTableName, " set create_date=current_timestamp , config = '", strStoreData, "' returning create_date")
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
		logs.WithContext(ctx).Info(fmt.Sprint("Old store.UpdateTime = ", store.UpdateTime))
		store.UpdateTime = resDoc["create_date"].(time.Time)
		logs.WithContext(ctx).Info(fmt.Sprint("New store.UpdateTime = ", store.UpdateTime))
	}
	err = tx.Commit()
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprint("Error in tx.Commit : ", err.Error()))
		tx.Rollback()
	}
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
