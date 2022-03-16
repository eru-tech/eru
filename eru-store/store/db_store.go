package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jmoiron/sqlx"
	"log"
	"os"
	"strings"
	"time"
)

type DbStore struct {
	Store
	DbType     string
	UpdateTime time.Time
	storeType  string
	conStr     string
}

func getStoreDbPath() string {
	dbString := os.Getenv("STORE_DB_PATH")
	log.Print("dbString = ", dbString)
	return dbString
}
func (store *DbStore) SetDbType(dbtype string) {
	store.DbType = strings.ToLower(dbtype)
}
func (store *DbStore) LoadStore(dbString string) (err error) {
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
		store.SaveStore("")
		return err
	}
	rows, err := db.Queryx("select * from eruconfig limit 1")
	if err != nil {
		log.Print(err)
		log.Print("creating blank db store")
		store.SaveStore("")
		return err
	}
	mapping := make(map[string]interface{})
	for rows.Next() {
		err = rows.MapScan(mapping)
		if err != nil {
			log.Print(err)
			log.Print("creating blank db store")
			store.SaveStore("")
			return err
		}
		storeData := mapping["config"]
		storeUpdateTime := mapping["create_date"]
		// Marshalling the store
		//store = new(FileStore)
		err = json.Unmarshal(storeData.([]byte), store)
		if err != nil {
			log.Print(err)
			log.Print("creating blank db store")
			store.SaveStore("")
			return err
		}
		store.UpdateTime = storeUpdateTime.(time.Time)
		//log.Print("storeData == ",storeData)
		log.Print("storeUpdateTime == ", storeUpdateTime)
	}
	if err != nil {
		log.Print(err)
		log.Print("creating blank db store")
		store.SaveStore("")
		return err
	}
	//loadEnvironmentVariable(store)
	return nil
}

func (store *DbStore) SaveStore(dbString string) (err error) {
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
		store.SaveStore("")
		return err
	}
	tx := db.MustBegin()
	ctx, cancel := context.WithTimeout(context.Background(), 100000*time.Millisecond) //TODO: to get context as argument
	defer cancel()
	storeData, err := json.Marshal(store)
	if err != nil {
		log.Print("Error in json.Marshal")
		log.Print(err)
		tx.Rollback()
		return err
	}
	strStoreData := strings.Replace(string(storeData), "'", "''", -1)
	query := fmt.Sprint("update eruconfig set create_date=current_timestamp , config = '", strStoreData, "' returning create_date")
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
