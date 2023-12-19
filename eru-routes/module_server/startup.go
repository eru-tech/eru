package module_server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-routes/module_store"
	"os"
	"strconv"
	"strings"
)

const StoreTableName = "eruroutes_config"

func StartUp() (module_store.ModuleStoreI, error) {
	var err error

	eruqlbaseurl := os.Getenv("ERUQL_BASEURL")
	if eruqlbaseurl == "" {
		eruqlbaseurl = "http://localhost:8087"
		logs.WithContext(context.Background()).Info("'eruqlbaseurl' environment variable not found - setting default value as http://localhost:8087")
	}
	module_store.Eruqlbaseurl = eruqlbaseurl

	funcThreads := os.Getenv("FUNC_THREADS")
	if funcThreads == "" {
		funcThreads = "3"
		logs.WithContext(context.Background()).Info("'FUNC_THREADS' environment variable not found - setting default value as 3")
	}
	module_store.FuncThreads, err = strconv.Atoi(funcThreads)
	if err != nil {
		err = nil
		logs.WithContext(context.Background()).Info("'FUNC_THREADS' environment variable is non numeric - setting default value as 3")
		module_store.FuncThreads = 3
	}

	loopThreads := os.Getenv("LOOP_THREADS")
	if loopThreads == "" {
		loopThreads = "3"
		logs.WithContext(context.Background()).Info("'LOOP_THREADS' environment variable not found - setting default value as 3")
	}
	module_store.LoopThreads, err = strconv.Atoi(loopThreads)
	if err != nil {
		err = nil
		logs.WithContext(context.Background()).Info("'LOOP_THREADS' environment variable is non numeric - setting default value as 3")
		module_store.LoopThreads = 3
	}

	storeType := strings.ToUpper(os.Getenv("STORE_TYPE"))
	if storeType == "" {
		storeType = "STANDALONE"
		logs.WithContext(context.Background()).Info("STORE_TYPE environment variable not found - loading default standlone store")
	}
	var myStore module_store.ModuleStoreI
	switch storeType {
	case "POSTGRES":
		myStore = new(module_store.ModuleDbStore)
		myStore.SetDbType(storeType)
		myStore.SetStoreTableName(StoreTableName)
	case "STANDALONE":
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
	//s.Store = myStore
	return myStore, err
}
