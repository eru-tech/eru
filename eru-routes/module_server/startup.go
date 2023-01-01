package module_server

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/eru-tech/eru/eru-routes/module_store"
	"log"
	"os"
	"strings"
)

const StoreTableName = "eruroutes_config"

func StartUp() (module_store.ModuleStoreI, error) {
	eruqlbaseurl := os.Getenv("ERUQL_BASEURL")
	if eruqlbaseurl == "" {
		eruqlbaseurl = "http://localhost:8087"
		log.Print("'eruqlbaseurl' environment variable not found - setting default value as http://localhost:8087")
	}
	//s.Eruqlbaseurl=eruqlbaseurl
	module_store.Eruqlbaseurl = eruqlbaseurl
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
		myStore.SetStoreTableName(StoreTableName)
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
	if err == nil {
		err = json.Unmarshal(storeBytes, myStore)
		//module_store.UnMarshalStore(storeBytes, myStore)
	}
	//s.Store = myStore
	return myStore, err
}
