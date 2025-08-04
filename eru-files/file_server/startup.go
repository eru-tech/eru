package file_server

import (
	"context"

	handlers "github.com/eru-tech/eru/eru-files/file_server/handlers"
	"github.com/eru-tech/eru/eru-files/module_store"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

func StartUp() (module_store.ModuleStoreI, error) {
	logs.WithContext(context.Background()).Debug("StartUp - Start")
	return module_store.LoadStore(handlers.StoreTableName, handlers.StoreTenantTableName)
}
