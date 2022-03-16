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

func (store *FileStore) LoadStore(fp string) (err error) {
	log.Print("loading filestore")
	if fp == "" {
		fp = getStoreSaveFilePath()
	}
	storeData, err := ioutil.ReadFile(fp)
	if err != nil {
		log.Print(err)
		log.Print("creating new blank config file at ", fp)
		store.SaveStore(fp)
		return err
	}

	// Marshalling the store
	//store = new(FileStore)
	err = json.Unmarshal(storeData, store)
	if err != nil {
		log.Print(err)
		return err
	}
	return nil
}

func (store *FileStore) SaveStore(fp string) error {
	log.Print("saving filestore")
	if fp == "" {
		fp = getStoreSaveFilePath()
	}
	log.Print(fp)
	var err error
	var storeData []byte
	storeData, err = json.Marshal(store)

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
