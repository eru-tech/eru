package module_server

import (
	module_handlers "github.com/eru-tech/eru/eru-ql/module_server/handlers"
	"github.com/eru-tech/eru/eru-ql/module_store"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	"github.com/gorilla/mux"
	"net/http"
)

func SetServiceName() {
	server_handlers.ServerName = "eru-ql"
	server_handlers.RepoName = "eruql.json"
}

func AddModuleRoutes(serverRouter *mux.Router, sh *module_store.StoreHolder) {

	//store functions specific to files
	serverRouter.Methods(http.MethodPost).Path("/graphql/{project}/execute").HandlerFunc(module_handlers.GraphqlExecuteHandler(sh.Store))
	serverRouter.Path("/graphql/{project}/ws/execute").HandlerFunc(module_handlers.GraphqlWsExecuteHandler(sh.Store))
	serverRouter.Methods(http.MethodPost).Path("/sql/{project}/execute").HandlerFunc(module_handlers.SqlExecuteHandler(sh.Store))

	storeRouter := serverRouter.PathPrefix("/store").Subrouter()
	storeRouter.Methods(http.MethodPost).Path("/{project}/compare").HandlerFunc(module_handlers.StoreCompareHandler(sh.Store))
	storeRouter.Methods(http.MethodPost).Path("/{project}/save").HandlerFunc(module_handlers.ProjectSaveHandler(sh.Store))
	storeRouter.Methods(http.MethodDelete).Path("/{project}/remove").HandlerFunc(module_handlers.ProjectRemoveHandler(sh.Store))
	storeRouter.Methods(http.MethodGet).Path("/project/list").HandlerFunc(module_handlers.ProjectListHandler(sh.Store))
	storeRouter.Methods(http.MethodGet).Path("/{project}/config").HandlerFunc(module_handlers.ProjectConfigHandler(sh.Store))

	storeRouter.Methods(http.MethodPost).Path("/{project}/settings/save").HandlerFunc(module_handlers.ProjectSetingsSaveHandler(sh.Store))
	//storeRouter.Methods(http.MethodGet).Path("/{project}/generateaeskey").HandlerFunc(module_handlers.ProjectGenerateAesKeyHandler(sh.Store))

	storeRouter.Methods(http.MethodPost).Path("/{project}/myquery/save/{queryname}/{querytype}").HandlerFunc(module_handlers.ProjectMyQuerySaveHandler(sh.Store))
	storeRouter.Methods(http.MethodDelete).Path("/{project}/myquery/remove/{queryname}").HandlerFunc(module_handlers.ProjectMyQueryRemoveHandler(sh.Store))
	storeRouter.Methods(http.MethodGet).Path("/{project}/myquery/list/{querytype}").HandlerFunc(module_handlers.ProjectMyQueryListHandler(sh.Store))
	storeRouter.Methods(http.MethodGet).Path("/{project}/myquery/list").HandlerFunc(module_handlers.ProjectMyQueryListNamesHandler(sh.Store))
	storeRouter.Methods(http.MethodGet).Path("/{project}/myquery/config/{queryname}").HandlerFunc(module_handlers.ProjectMyQueryConfigHandler(sh.Store))
	storeRouter.Methods(http.MethodPost).Path("/{project}/myquery/execute/{queryname}").HandlerFunc(module_handlers.ProjectMyQueryExecuteHandler(sh.Store))
	storeRouter.Methods(http.MethodPost).Path("/{project}/myquery/execute/{queryname}/{outputtype}").HandlerFunc(module_handlers.ProjectMyQueryExecuteHandler(sh.Store))
	storeRouter.Methods(http.MethodPost).Path("/{project}/myquery/execute/{queryname}/{outputtype}/{encode}").HandlerFunc(module_handlers.ProjectMyQueryExecuteHandler(sh.Store))
	storeRouter.Methods(http.MethodGet).Path("/{project}/datasource/defaultdriverconfig/{dbtype}").HandlerFunc(module_handlers.DefaultDriverConfigHandler())
	storeRouter.Methods(http.MethodGet).Path("/{project}/datasource/defaultotherdbconfig/{dbtype}").HandlerFunc(module_handlers.DefaultOtherDBConfigHandler())
	storeRouter.Methods(http.MethodGet).Path("/{project}/datasource/defaultdbsecurityrules/{dbtype}").HandlerFunc(module_handlers.DefaultDBSecurityRulesHandler())

	storeRouter.Methods(http.MethodPost).Path("/{project}/datasource/save/{dbalias}").HandlerFunc(module_handlers.ProjectDataSourceSaveHandler(sh.Store))
	storeRouter.Methods(http.MethodDelete).Path("/{project}/datasource/remove/{dbalias}").HandlerFunc(module_handlers.ProjectDataSourceRemoveHandler(sh.Store))
	storeRouter.Methods(http.MethodGet).Path("/{project}/datasource/list").HandlerFunc(module_handlers.ProjectDataSourceListHandler(sh.Store))
	storeRouter.Methods(http.MethodGet).Path("/{project}/datasource/config/{dbalias}").HandlerFunc(module_handlers.ProjectDataSourceConfigHandler(sh.Store))
	storeRouter.Methods(http.MethodGet).Path("/{project}/datasource/schema/{dbalias}").HandlerFunc(module_handlers.ProjectDataSourceSchemaHandler(sh.Store))
	storeRouter.Methods(http.MethodPost).Path("/{project}/datasource/schema/{dbalias}/addtable/{tablename}").HandlerFunc(module_handlers.ProjectDataSourceSchemaAddTableHandler(sh.Store))
	storeRouter.Methods(http.MethodDelete).Path("/{project}/datasource/schema/{dbalias}/removetable/{tablename}").HandlerFunc(module_handlers.ProjectDataSourceSchemaRemoveTableHandler(sh.Store))
	storeRouter.Methods(http.MethodPost).Path("/{project}/datasource/schema/{dbalias}/addjoin").HandlerFunc(module_handlers.ProjectDataSourceSchemaAddJoinHandler(sh.Store))
	storeRouter.Methods(http.MethodDelete).Path("/{project}/datasource/schema/{dbalias}/removejoin").HandlerFunc(module_handlers.ProjectDataSourceSchemaRemoveJoinHandler(sh.Store))
	storeRouter.Methods(http.MethodPost).Path("/{project}/datasource/schema/{dbalias}/savetable/{tablename}").HandlerFunc(module_handlers.ProjectDataSourceSchemaSaveTableHandler(sh.Store))
	storeRouter.Methods(http.MethodDelete).Path("/{project}/datasource/schema/{dbalias}/droptable/{tablename}").HandlerFunc(module_handlers.ProjectDataSourceSchemaDropTableHandler(sh.Store))
	storeRouter.Methods(http.MethodPost).Path("/{project}/datasource/schema/{dbalias}/securetable/{tablename}").HandlerFunc(module_handlers.ProjectDataSourceSchemaSecureTableHandler(sh.Store))
	storeRouter.Methods(http.MethodPost).Path("/{project}/datasource/schema/{dbalias}/transformtable/{tablename}").HandlerFunc(module_handlers.ProjectDataSourceSchemaTransformTableHandler(sh.Store))
	storeRouter.Methods(http.MethodPost).Path("/{project}/datasource/schema/{dbalias}/maskcolumn/{tablename}/{colname}").HandlerFunc(module_handlers.ProjectDataSourceSchemaMasColumnHandler(sh.Store))
}
