package file_server

import (
	file_handlers "github.com/eru-tech/eru/eru-files/file_server/handlers"
	"github.com/eru-tech/eru/eru-files/module_store"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	"github.com/gorilla/mux"
	"net/http"
)

func SetServiceName() {
	server_handlers.ServerName = "eru-files"
}
func AddFileRoutes(serverRouter *mux.Router, sh *module_store.StoreHolder) {

	//store routes specific to files
	storeRouter := serverRouter.PathPrefix("/store").Subrouter()

	storeRouter.Methods(http.MethodPost).Path("/{project}/storage/save/{storagename}/{storagetype}").HandlerFunc(file_handlers.StorageSaveHandler(sh.Store))
	storeRouter.Methods(http.MethodDelete).Path("/{project}/storage/remove/{storagename}").HandlerFunc(file_handlers.StorageRemoveHandler(sh.Store))

	storeRouter.Methods(http.MethodPost).Path("/{project}/save").HandlerFunc(file_handlers.ProjectSaveHandler(sh.Store))
	storeRouter.Methods(http.MethodDelete).Path("/{project}/remove").HandlerFunc(file_handlers.ProjectRemoveHandler(sh.Store))
	storeRouter.Methods(http.MethodGet).Path("/project/list").HandlerFunc(file_handlers.ProjectListHandler(sh.Store))
	storeRouter.Methods(http.MethodGet).Path("/{project}/config").HandlerFunc(file_handlers.ProjectConfigHandler(sh.Store))

	storeRouter.Methods(http.MethodPost).Path("/{project}/rsakeypair/generate/{keypairname}").HandlerFunc(file_handlers.RsaKeyPairGenerateHandler(sh.Store))
	storeRouter.Methods(http.MethodPost).Path("/{project}/aeskey/generate/{keyname}").HandlerFunc(file_handlers.AesKeyGenerateHandler(sh.Store))

	// routes for file events
	fileRouter := serverRouter.PathPrefix("/files/{project}").Subrouter()
	fileRouter.Methods(http.MethodPost).Path("/{storagename}/upload").HandlerFunc(file_handlers.FileUploadHandler(sh.Store))
	fileRouter.Methods(http.MethodPost).Path("/{storagename}/uploadb64").HandlerFunc(file_handlers.FileUploadHandlerB64(sh.Store))
	fileRouter.Methods(http.MethodPost).Path("/{storagename}/uploadfromurl").HandlerFunc(file_handlers.FileUploadHandlerFromUrl(sh.Store))
	fileRouter.Methods(http.MethodPost, http.MethodGet).Path("/{storagename}/download").HandlerFunc(file_handlers.FileDownloadHandler(sh.Store))
	fileRouter.Methods(http.MethodPost, http.MethodGet).Path("/{storagename}/downloadb64").HandlerFunc(file_handlers.FileDownloadHandlerB64(sh.Store))
	fileRouter.Methods(http.MethodPost, http.MethodGet).Path("/{storagename}/downloadunzip").HandlerFunc(file_handlers.FileDownloadHandlerUnzip(sh.Store))
	//fileRouter.Methods(http.MethodPost).Path("/testEncrypt/{text}").HandlerFunc(file_handlers.TestEncrypt(sh.Store))
	//fileRouter.Methods(http.MethodPost).Path("/testAesEncrypt/{text}/{keyname}").HandlerFunc(file_handlers.TestAesEncrypt(sh.Store))
}
