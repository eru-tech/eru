package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"log"
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
	log.Print("dbString = ", dbString)
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
	log.Print("GetStoreByteArray from dbstore")
	if dbString == "" {
		dbString = getStoreDbPath()
		if dbString == "" {
			return nil, errors.New("No value found for environment variable STORE_DB_PATH")
		}
	}
	log.Print("Creating DB connection for GetStoreByteArray")
	log.Println(store.DbType)
	db, err := sqlx.Open(store.DbType, dbString)
	if err != nil {
		log.Print(err)
		log.Print("creating blank db store")
		store.SaveStore("", nil)
		return nil, err
	}
	defer db.Close()
	rows, err := db.Queryx(fmt.Sprint("select * from ", store.StoreTableName, " limit 1"))
	if err != nil {
		log.Print(err)
		log.Print("creating blank db store")
		store.SaveStore("", nil)
		return nil, err
	}
	mapping := make(map[string]interface{})
	var storeData interface{}
	for rows.Next() {
		err = rows.MapScan(mapping)
		if err != nil {
			log.Print(err)
			log.Print("creating blank db store")
			store.SaveStore("", nil)
			return nil, err
		}
		storeData = mapping["config"]
		storeUpdateTime := mapping["create_date"]
		// Marshalling the store
		//store = new(FileStore)
		store.UpdateTime = storeUpdateTime.(time.Time)

	}
	if storeData == nil {
		log.Print("no config data retrived from db")
		log.Print("creating blank db store")
		store.SaveStore("", nil)
		return nil, err
	}
	return storeData.([]byte), err
}

func (store *DbStore) LoadStore(dbString string, ms StoreI) (err error) {
	log.Print("loading dbstore")
	if dbString == "" {
		dbString = getStoreDbPath()
	}
	log.Print("Creating DB connection for Load DB store")
	db, err := sqlx.Open(store.DbType, dbString)
	defer db.Close()
	if err != nil {
		log.Print(err)
		log.Print("creating blank db store")
		store.SaveStore("", nil)
		return err
	}
	rows, err := db.Queryx(fmt.Sprint("select * from ", store.StoreTableName, " limit 1"))
	if err != nil {
		log.Print(err)
		log.Print("creating blank db store")
		store.SaveStore("", nil)
		return err
	}
	mapping := make(map[string]interface{})
	for rows.Next() {
		err = rows.MapScan(mapping)
		if err != nil {
			log.Print(err)
			log.Print("creating blank db store")
			store.SaveStore("", nil)
			return err
		}
		storeData := mapping["config"]
		storeUpdateTime := mapping["create_date"]
		// Marshalling the store
		//store = new(FileStore)
		err = json.Unmarshal(storeData.([]byte), ms)
		if err != nil {
			log.Print(err)
			log.Print("creating blank db store")
			store.SaveStore("", nil)
			return err
		}
		store.UpdateTime = storeUpdateTime.(time.Time)
		//log.Print("storeData == ",storeData)
		log.Print("storeUpdateTime == ", storeUpdateTime)
	}
	if err != nil {
		log.Print(err)
		log.Print("creating blank db store")
		store.SaveStore("", nil)
		return err
	}
	//loadEnvironmentVariable(store)
	return nil
}

func (store *DbStore) SaveStore(dbString string, ms StoreI) (err error) {
	log.Print("saving dbstore")
	if dbString == "" {
		dbString = getStoreDbPath()
	}
	log.Print("Creating DB connection for Save DB store")
	db, err := sqlx.Open(store.DbType, dbString)
	defer db.Close()
	if err != nil {
		log.Print(err)
		log.Print("creating blank db store")
		store.SaveStore("", nil) //TODO - seems recursive call in case of error
		return err
	}
	tx := db.MustBegin()
	ctx, cancel := context.WithTimeout(context.Background(), 100000*time.Millisecond) //TODO: to get context as argument
	defer cancel()
	storeData, err := json.Marshal(ms)
	if err != nil {
		log.Print("Error in json.Marshal")
		log.Print(err)
		tx.Rollback()
		return err
	}
	strStoreData := strings.Replace(string(storeData), "'", "''", -1)
	//log.Println(strStoreData)
	query := fmt.Sprint("update ", store.StoreTableName, " set create_date=current_timestamp , config = '", strStoreData, "' returning create_date")
	//log.Print(query)
	stmt, err := tx.PreparexContext(ctx, query)
	if err != nil {
		log.Print("Error in tx.PreparexContext")
		log.Print(err)
		tx.Rollback()
		return err
	}
	rw, err := stmt.QueryxContext(ctx)
	if err != nil {
		log.Print("Error in stmt.QueryxContext")
		log.Print(err)
		tx.Rollback()
		return err
	}
	for rw.Rows.Next() {
		resDoc := make(map[string]interface{})
		err = rw.MapScan(resDoc)
		if err != nil {
			log.Print("Error in rw.MapScan")
			log.Print(err)
			tx.Rollback()
			return err
		}
		log.Print("Old store.UpdateTime = ", store.UpdateTime)
		store.UpdateTime = resDoc["create_date"].(time.Time)
		log.Print("New store.UpdateTime = ", store.UpdateTime)
	}
	err = tx.Commit()
	if err != nil {
		log.Print("Error in tx.Commit")
		log.Print(err)
		tx.Rollback()
	}
	return nil
}

func (store *DbStore) getStoreDBConnStr() (string, error) {
	dbConStr := os.Getenv("storedb")
	if dbConStr == "" {
		log.Print("storedb environment variable not found")
		return "", errors.New(fmt.Sprint("storedb environment variable not found"))
	}
	return dbConStr, nil
}
