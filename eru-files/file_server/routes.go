package file_server

import (
	"net/http"

	file_handlers "github.com/eru-tech/eru/eru-files/file_server/handlers"
	"github.com/eru-tech/eru/eru-files/module_store"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	"github.com/gorilla/mux"
)

func SetServiceName() {
	server_handlers.ServerName = "eru-files"
}
func AddFileRoutes(serverRouter *mux.Router, sh *module_store.StoreHolder) {

	//store functions specific to files
	serverRouter.Methods(http.MethodGet).Path("/{event_name}").HandlerFunc(file_handlers.ConfigSyncHandler(sh))
	storeRouter := serverRouter.PathPrefix("/store").Subrouter()
	storeRouter.Methods(http.MethodGet).Path("/load").HandlerFunc(file_handlers.StoreLoadHandler(sh))
	storeRouter.Methods(http.MethodPost).Path("/{project}/compare").HandlerFunc(file_handlers.StoreCompareHandler(sh))

	storeRouter.Methods(http.MethodPost).Path("/{project}/storage/save/{storagename}/{storagetype}").HandlerFunc(file_handlers.StorageSaveHandler(sh))
	storeRouter.Methods(http.MethodDelete).Path("/{project}/storage/remove/{storagename}").HandlerFunc(file_handlers.StorageRemoveHandler(sh))
	storeRouter.Methods(http.MethodDelete).Path("/{project}/storage/remove/{storagename}/{clouddelete}").HandlerFunc(file_handlers.StorageRemoveHandler(sh))
	storeRouter.Methods(http.MethodDelete).Path("/{project}/storage/remove/{storagename}/{clouddelete}/{forcedelete}").HandlerFunc(file_handlers.StorageRemoveHandler(sh))

	storeRouter.Methods(http.MethodPost).Path("/{project}/save").HandlerFunc(file_handlers.ProjectSaveHandler(sh))
	storeRouter.Methods(http.MethodDelete).Path("/{project}/remove").HandlerFunc(file_handlers.ProjectRemoveHandler(sh))
	storeRouter.Methods(http.MethodGet).Path("/project/list").HandlerFunc(file_handlers.ProjectListHandler(sh))
	storeRouter.Methods(http.MethodGet).Path("/{project}/config").HandlerFunc(file_handlers.ProjectConfigHandler(sh))

	storeRouter.Methods(http.MethodPost).Path("/{project}/rsakeypair/generate/{keypairname}").HandlerFunc(file_handlers.RsaKeyPairGenerateHandler(sh))
	storeRouter.Methods(http.MethodPost).Path("/{project}/aeskey/generate/{keyname}").HandlerFunc(file_handlers.AesKeyGenerateHandler(sh))
	storeRouter.Methods(http.MethodPost).Path("/{project}/settings/save").HandlerFunc(file_handlers.ProjectSetingsSaveHandler(sh))

	// functions for file events
	fileRouter := serverRouter.PathPrefix("/files/{project}").Subrouter()
	fileRouter.Methods(http.MethodPost).Path("/{storagename}/upload").HandlerFunc(file_handlers.FileUploadHandler(sh))
	fileRouter.Methods(http.MethodPost).Path("/{storagename}/uploadb64").HandlerFunc(file_handlers.FileUploadHandlerB64(sh))
	fileRouter.Methods(http.MethodPost).Path("/{storagename}/uploadfromurl").HandlerFunc(file_handlers.FileUploadHandlerFromUrl(sh))
	fileRouter.Methods(http.MethodPost, http.MethodGet).Path("/{storagename}/download").HandlerFunc(file_handlers.FileDownloadHandler(sh))
	fileRouter.Methods(http.MethodPost, http.MethodGet).Path("/{storagename}/downloadb64").HandlerFunc(file_handlers.FileDownloadHandlerB64(sh))
	fileRouter.Methods(http.MethodPost, http.MethodGet).Path("/{storagename}/downloadunzip").HandlerFunc(file_handlers.FileDownloadHandlerUnzip(sh))
	fileRouter.Methods(http.MethodPost, http.MethodGet).Path("/stringtofile").HandlerFunc(file_handlers.StringToFileHandler(sh))
	fileRouter.Methods(http.MethodPost).Path("/exceltojson").HandlerFunc(file_handlers.ExcelToJsonHandler(sh))
	fileRouter.Methods(http.MethodPost).Path("/exceltojsonb64").HandlerFunc(file_handlers.ExcelToJsonB64Handler(sh))
	fileRouter.Methods(http.MethodPost).Path("/csvdatatojson").HandlerFunc(file_handlers.CsvDataToJsonHandler(sh))
	//fileRouter.Methods(http.MethodPost).Path("/exceltojsonurl").HandlerFunc(file_handlers.ExcelToJsonUrlHandler(sh))
	fileRouter.Methods(http.MethodPost).Path("/jsonvalidate").HandlerFunc(file_handlers.JsonValidatorHandler(sh))
}
