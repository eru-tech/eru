package module_server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/eru-tech/eru/eru-gateway/module_store"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"os"
	"strings"
)

const StoreTableName = "erugateway_config"

func StartUp() (module_store.ModuleStoreI, error) {
	logs.WithContext(context.Background()).Debug("StartUp - Start")
	storeType := strings.ToUpper(os.Getenv("STORE_TYPE"))
	if storeType == "" {
		storeType = "STANDALONE"
		logs.WithContext(context.Background()).Info("STORE_TYPE environment variable not found - loading default standlone store")
	}
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
			logs.WithContext(context.Background()).Error(err.Error())
			return nil, err
		}
	default:
		err = errors.New(fmt.Sprint("Invalid STORE_TYPE ", storeType))
		logs.WithContext(context.Background()).Error(err.Error())
		return nil, err
	}
	storeBytes, err := myStore.GetStoreByteArray("")
	if err == nil {
		err = json.Unmarshal(storeBytes, myStore)
		if err != nil {
			logs.WithContext(context.Background()).Error(err.Error())
		}
		err = myStore.SetStoreFromBytes(context.Background(), storeBytes, myStore)
		if err != nil {
			logs.WithContext(context.Background()).Error(err.Error())
			return nil, err
		}
		//module_store.UnMarshalStore(storeBytes, myStore)
	} else {
		logs.WithContext(context.Background()).Error(err.Error())
	}
	return myStore, err
}
