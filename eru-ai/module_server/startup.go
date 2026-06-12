package module_server

import (
	"context"
	"fmt"
	"os"

	"github.com/eru-tech/eru/eru-ai/module_server/handlers"
	"github.com/eru-tech/eru/eru-ai/module_store"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
)

func StartUp(ctx context.Context) (module_store.ModuleStoreI, error) {
	erufuncbaseurl := os.Getenv("ERUFUNCTIONS_BASEURL")
	if erufuncbaseurl == "" {
		erufuncbaseurl = "http://localhost:8083"
		logs.WithContext(ctx).Info("'ERUFUNCTIONS_BASEURL' environment variable not found - setting default value as http://localhost:8083")
	}
	module_store.Erufuncbaseurl = erufuncbaseurl

	eruauthbaseurl := os.Getenv("ERUAUTH_BASEURL")
	if eruauthbaseurl == "" {
		eruauthbaseurl = "http://localhost:8085"
		logs.WithContext(ctx).Info("'ERUAUTH_BASEURL' environment variable not found - setting default value as http://localhost:8085")
	}
	module_store.Eruauthbaseurl = eruauthbaseurl

	eruqlbaseurl := os.Getenv("ERUQL_BASEURL")
	if eruqlbaseurl == "" {
		eruqlbaseurl = "http://localhost:8087"
		logs.WithContext(ctx).Info("'ERUQL_BASEURL' environment variable not found - setting default value as http://localhost:8087")
	}
	server_handlers.EruqlBaseUrl = eruqlbaseurl
	module_store.Eruqlbaseurl = eruqlbaseurl

	eruaibaseurl := os.Getenv("ERUAI_BASEURL")
	if eruaibaseurl == "" {
		eruaibaseurl = "http://localhost:8088"
		logs.WithContext(ctx).Info("'ERUAI_BASEURL' environment variable not found - setting default value as http://localhost:8088")
	}
	module_store.Eruaibaseurl = eruaibaseurl
	logs.WithContext(ctx).Info(fmt.Sprintf("ERUAI_BASEURL: %s", module_store.Eruaibaseurl))

	erufilesbaseurl := os.Getenv("ERUFILES_BASEURL")
	if erufilesbaseurl == "" {
		erufilesbaseurl = "http://localhost:8082"
		logs.WithContext(ctx).Info("'ERUFILES_BASEURL' environment variable not found - setting default value as http://localhost:8082")
	}
	module_store.Erufilesbaseurl = erufilesbaseurl

	return module_store.LoadStore(ctx, handlers.StoreTableName, handlers.StoreTenantTableName)
}
