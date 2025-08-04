package module_server

import (
	"context"

	"github.com/eru-tech/eru/eru-gateway/module_server/handlers"
	"github.com/eru-tech/eru/eru-gateway/module_store"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

func StartUp() (module_store.ModuleStoreI, error) {
	logs.WithContext(context.Background()).Debug("StartUp - Start")

	return module_store.LoadStore(handlers.StoreTableName, handlers.StoreTenantTableName)
}
