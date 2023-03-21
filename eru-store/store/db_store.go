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
}

func getStoreDbPath() string {
	dbString := os.Getenv("STORE_DB_PATH")
	logs.Logger.Debug(fmt.Sprint("dbString = ", dbString))
	return dbString
}
func (store *DbStore) SetDbType(dbtype string) {
	store.DbType = strings.ToLower(dbtype)
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
			return nil, errors.New("No value found for environment variable STORE_DB_PATH")
		}
	}
	logs.Logger.Info("Creating DB connection for GetStoreByteArray")
	db, err := sqlx.Open(store.DbType, dbString)
	if err != nil {
		logs.Logger.Error(err.Error())
		//log.Print("creating blank db store")
		//store.SaveStore("", nil)
		return nil, err
	}
	defer db.Close()
	logs.Logger.Info(fmt.Sprint("db connection succesfull - fetch config from ", store.StoreTableName))
	rows, err := db.Queryx(fmt.Sprint("select * from ", store.StoreTableName, " limit 1"))
	if err != nil {
		logs.Logger.Error(err.Error())
		//log.Print("creating blank db store")
		//store.SaveStore("", nil)
		return nil, err
	}
	logs.Logger.Info("config fetched succesfully")
	mapping := make(map[string]interface{})
	var storeData interface{}
	for rows.Next() {
		err = rows.MapScan(mapping)
		if err != nil {
			logs.Logger.Error(err.Error())
			//log.Print("creating blank db store")
			//store.SaveStore("", nil)
			return nil, err
		}
		storeData = mapping["config"]
		storeUpdateTime := mapping["create_date"]
		// Marshalling the store
		//store = new(FileStore)
		store.UpdateTime = storeUpdateTime.(time.Time)

	}
	if storeData == nil {
		err = errors.New("no config data retrived from db")
		logs.Logger.Error(err.Error())
		//log.Print("creating blank db store")
		//store.SaveStore("", nil)
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
		//log.Print("creating blank db store")
		//store.SaveStore("", nil)
		return err
	}
	rows, err := db.Queryx(fmt.Sprint("select * from ", store.StoreTableName, " limit 1"))
	if err != nil {
		logs.Logger.Error(err.Error())
		//log.Print("creating blank db store")
		//store.SaveStore("", nil)
		return err
	}
	mapping := make(map[string]interface{})
	for rows.Next() {
		err = rows.MapScan(mapping)
		if err != nil {
			logs.Logger.Error(err.Error())
			//log.Print("creating blank db store")
			//store.SaveStore("", nil)
			return err
		}
		storeData := mapping["config"]
		storeUpdateTime := mapping["create_date"]
		// Marshalling the store
		//store = new(FileStore)
		err = json.Unmarshal(storeData.([]byte), ms)
		if err != nil {
			logs.Logger.Error(err.Error())
			//log.Print("creating blank db store")
			//store.SaveStore("", nil)
			return err
		}
		store.UpdateTime = storeUpdateTime.(time.Time)
		//log.Print("storeData == ",storeData)
		logs.Logger.Info(fmt.Sprint("storeUpdateTime == ", storeUpdateTime))
	}
	if err != nil {
		logs.Logger.Error(err.Error())
		//log.Print("creating blank db store")
		//store.SaveStore("", nil)
		return err
	}
	//loadEnvironmentVariable(store)
	return nil
}

func (store *DbStore) SaveStore(dbString string, ms StoreI) (err error) {
	logs.Logger.Debug("SaveStore - Start")
	if dbString == "" {
		dbString = getStoreDbPath()
	}
	logs.Logger.Info("Creating DB connection for Save DB store")
	db, err := sqlx.Open(store.DbType, dbString)
	defer db.Close()
	if err != nil {
		logs.Logger.Error(err.Error())
		//log.Print("creating blank db store")
		//store.SaveStore("", nil) //TODO - seems recursive call in case of error
		return err
	}
	tx := db.MustBegin()
	ctx, cancel := context.WithTimeout(context.Background(), 100000*time.Millisecond) //TODO: to get context as argument
	defer cancel()
	storeData, err := json.Marshal(ms)
	if err != nil {
		logs.Logger.Error(err.Error())
		tx.Rollback()
		return err
	}
	strStoreData := strings.Replace(string(storeData), "'", "''", -1)
	//log.Println(strStoreData)
	query := fmt.Sprint("update ", store.StoreTableName, " set create_date=current_timestamp , config = '", strStoreData, "' returning create_date")
	//log.Print(query)
	stmt, err := tx.PreparexContext(ctx, query)
	if err != nil {
		logs.Logger.Error(fmt.Sprint("Error in tx.PreparexContext : ", err.Error()))
		tx.Rollback()
		return err
	}
	rw, err := stmt.QueryxContext(ctx)
	if err != nil {
		logs.Logger.Error(fmt.Sprint("Error in stmt.QueryxContext : ", err.Error()))
		tx.Rollback()
		return err
	}
	for rw.Rows.Next() {
		resDoc := make(map[string]interface{})
		err = rw.MapScan(resDoc)
		if err != nil {
			logs.Logger.Error(fmt.Sprint("Error in rw.MapScan : ", err.Error()))
			tx.Rollback()
			return err
		}
		logs.Logger.Info(fmt.Sprint("Old store.UpdateTime = ", store.UpdateTime))
		store.UpdateTime = resDoc["create_date"].(time.Time)
		logs.Logger.Info(fmt.Sprint("New store.UpdateTime = ", store.UpdateTime))
	}
	err = tx.Commit()
	if err != nil {
		logs.Logger.Error(fmt.Sprint("Error in tx.Commit : ", err.Error()))
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
