package module_server

import (
	"context"
	"os"

	"github.com/eru-tech/eru/eru-auth/module_server/handlers"
	"github.com/eru-tech/eru/eru-auth/module_store"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

func StartUp() (module_store.ModuleStoreI, error) {
	erufuncbaseurl := os.Getenv("ERUFUNCTIONS_BASEURL")
	if erufuncbaseurl == "" {
		erufuncbaseurl = "http://localhost:8083"
		logs.WithContext(context.Background()).Info("'ERUFUNCTIONS_BASEURL' environment variable not found - setting default value as http://localhost:8083")
	}
	module_store.Erufuncbaseurl = erufuncbaseurl

	return module_store.LoadStore(handlers.StoreTableName, handlers.StoreTenantTableName)
}
