package store

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
)

var filePath = "/config/config.json"

type FileStore struct {
	Store
}

func getStoreSaveFilePath() string {
	wd, err := os.Getwd()
	if err != nil {
		log.Print(err)
	}
	log.Print("wd = ", wd)
	log.Print("filePath = ", filePath)
	return fmt.Sprint(wd, filePath)
}
func (store *FileStore) GetStoreByteArray(fp string) (b []byte, err error) {
	log.Print("GetStoreByteArray from filestore")
	if fp == "" {
		fp = getStoreSaveFilePath()
	}
	storeData, err := ioutil.ReadFile(fp)
	if err != nil {
		log.Println(err)
		err = store.SaveStore(fp, store)
		if err == nil {
			storeData, err = json.Marshal(store)
			if err != nil {
				log.Print("error json.Marshal(store)")
				log.Print(err)
				return nil, err
			}
		}
	}
	//log.Println(string(storeData))
	return storeData, err
}
func (store *FileStore) LoadStore(fp string, ms StoreI) (err error) {
	log.Print("loading filestore")
	if fp == "" {
		fp = getStoreSaveFilePath()
	}
	storeData, err := ioutil.ReadFile(fp)
	log.Println(string(storeData))
	if err != nil {
		log.Print(err)
		log.Print("creating new blank config file at ", fp)
		store.SaveStore(fp, nil)
		return err
	}
	err = json.Unmarshal(storeData, ms)
	if err != nil {
		log.Print("error json.Unmarshal(storeData, ms)")
		log.Print(err)
		return err
	}
	return nil
}

func (store *FileStore) SaveStore(fp string, ms StoreI) error {
	log.Println("saving filestore")
	if fp == "" {
		fp = getStoreSaveFilePath()
	}
	var err error
	var storeData []byte
	log.Println("before json.Marshal")
	storeData, err = json.Marshal(ms)
	log.Println("after json.Marshal")
	// Check for error during marshaling
	if err != nil {
		log.Print("marshaling error = ", err)
		return err
	}
	err = ioutil.WriteFile(fp, storeData, 0644)
	if err != nil {
		log.Print("WriteFile error = ", err)
	}
	return err
}
