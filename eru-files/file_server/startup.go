package file_server

import (
	"errors"
	"fmt"
	"github.com/eru-tech/eru/eru-files/module_store"
	"log"
	"os"
	"strings"
)

func StartUp() (module_store.ModuleStoreI, error) {
	storeType := strings.ToUpper(os.Getenv("STORE_TYPE"))
	if storeType == "" {
		storeType = "STANDALONE"
		log.Print("STORE_TYPE environment variable not found - loading default standlone store")
	}
	log.Print(storeType)
	var myStore module_store.ModuleStoreI
	var err error
	switch storeType {
	case "POSTGRES":
		myStore = new(module_store.ModuleDbStore)
		myStore.SetDbType(storeType)
	case "STANDALONE":
		// myStore, err = store.LoadStoreFromFile()
		myStore = new(module_store.ModuleFileStore)
		if err != nil {
			return nil, err
		}
	default:
		return nil, errors.New(fmt.Sprint("Invalid STORE_TYPE ", storeType))
	}
	storeBytes, err := myStore.GetStoreByteArray("")
	module_store.UnMarshalStore(storeBytes, myStore)
	//s.Store = myStore
	return myStore, nil
}
