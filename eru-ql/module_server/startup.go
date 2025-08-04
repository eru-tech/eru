package module_server

import (
	"context"

	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-ql/module_server/handlers"
	"github.com/eru-tech/eru/eru-ql/module_store"
)

func StartUp() (module_store.ModuleStoreI, error) {
	logs.WithContext(context.Background()).Debug("StartUp - Start")
	return module_store.LoadStore(handlers.StoreTableName, handlers.StoreTenantTableName)
}
