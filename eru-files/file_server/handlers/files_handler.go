package server

import (
	"bytes"
	b64 "encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/eru-tech/eru/eru-files/module_store"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	utils "github.com/eru-tech/eru/eru-utils"
	"github.com/gorilla/mux"
	"io"
	"net/http"
	"strings"
	//"github.com/aws/aws-sdk-go/aws/session"
	//"github.com/aws/aws-sdk-go/service/s3"
)

const (
	encodedForm   = "application/x-www-form-urlencoded"
	multiPartForm = "multipart/form-data"
)

func FileDownloadHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		logs.WithContext(r.Context()).Debug("FileDownloadHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		storageName := vars["storagename"]

		var err error

		//ctx, cancel := context.WithTimeout(r.Context(), 30*time.Minute)
		//defer cancel()
		//_ = ctx

		dfFromReq := json.NewDecoder(r.Body)
		dfFromReq.DisallowUnknownFields()
		dfFromObj := make(map[string]string)

		if err := dfFromReq.Decode(&dfFromObj); err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}

		file, mimeType, err := s.DownloadFile(r.Context(), projectId, storageName, dfFromObj["folder_path"], dfFromObj["file_name"])
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		//server_handlers.FormatResponse(w,http.StatusOK)
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", mimeType)
		w.Header().Set("Content-Disposition", fmt.Sprint("attachment; filename=", strings.Replace(dfFromObj["file_name"], ".enc", "", -1)))
		_, _ = io.Copy(w, bytes.NewReader(file))
	}
}

func FileDownloadHandlerB64(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		logs.WithContext(r.Context()).Debug("FileDownloadHandlerB64 - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		storageName := vars["storagename"]

		var err error

		//ctx, cancel := context.WithTimeout(r.Context(), 30*time.Minute)
		//defer cancel()
		//_ = ctx

		dfFromReq := json.NewDecoder(r.Body)
		dfFromReq.DisallowUnknownFields()
		dfFromObj := make(map[string]string)

		if err := dfFromReq.Decode(&dfFromObj); err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}

		fileB64, mimeType, err := s.DownloadFileB64(r.Context(), projectId, storageName, dfFromObj["folder_path"], dfFromObj["file_name"])
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		server_handlers.FormatResponse(w, http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"file": fileB64, "file_type": mimeType})
	}
}
func FileDownloadHandlerUnzip(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		logs.WithContext(r.Context()).Debug("FileDownloadHandlerUnzip - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		storageName := vars["storagename"]

		var err error
		//ctx, cancel := context.WithTimeout(r.Context(), 30*time.Minute)
		//defer cancel()
		//_ = ctx

		dfFromReq := json.NewDecoder(r.Body)
		dfFromReq.DisallowUnknownFields()
		dfFromObj := module_store.FileDownloadRequest{}
		if err := dfFromReq.Decode(&dfFromObj); err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		vErr := utils.ValidateStruct(r.Context(), dfFromObj, "")
		if vErr != nil {
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": vErr.Error()})
			return
		}

		files, err := s.DownloadFileUnzip(r.Context(), projectId, storageName, dfFromObj)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		server_handlers.FormatResponse(w, http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"files": files})
	}
}

func FileUploadHandlerB64(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		logs.WithContext(r.Context()).Debug("FileUploadHandlerB64 - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		storageName := vars["storagename"]

		var err error
		//ctx, cancel := context.WithTimeout(r.Context(), 30*time.Minute)
		//defer cancel()
		//_ = ctx

		ufFromReq := json.NewDecoder(r.Body)
		ufFromReq.DisallowUnknownFields()
		ufFromObj := make(map[string]string)

		if err := ufFromReq.Decode(&ufFromObj); err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		file, ok := ufFromObj["file"]
		if !ok {
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "file attribute missing"})
			return
		}
		docType, ok := ufFromObj["doc_type"]
		if !ok {
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "doc_type attribute missing"})
			return
		}
		fileName, ok := ufFromObj["file_name"]
		if !ok {
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "file_name attribute missing"})
			return
		}
		folderPath, ok := ufFromObj["folder_path"]
		if !ok {
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "folder_path attribute missing"})
			return
		}
		fileBytes, err := b64.StdEncoding.DecodeString(file)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "base64 decode failed"})
			return
		}
		docId, err := s.UploadFileB64(r.Context(), projectId, storageName, fileBytes, fileName, docType, folderPath)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		fileNames := make(map[string]string)
		fileNames[fileName] = docId
		server_handlers.FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"files": fileNames})
	}
}

func FileUploadHandlerFromUrl(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		logs.WithContext(r.Context()).Debug("FileUploadHandlerFromUrl - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		storageName := vars["storagename"]

		var err error
		//ctx, cancel := context.WithTimeout(r.Context(), 30*time.Minute)
		//defer cancel()
		//_ = ctx

		ufFromReq := json.NewDecoder(r.Body)
		ufFromReq.DisallowUnknownFields()
		ufFromObj := make(map[string]string)

		if err := ufFromReq.Decode(&ufFromObj); err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		url, ok := ufFromObj["url"]
		if !ok {
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "file attribute missing"})
			return
		}
		docType, ok := ufFromObj["doc_type"]
		if !ok {
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "doc_type attribute missing"})
			return
		}

		fileName, ok := ufFromObj["file_name"]
		if !ok {
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "file_name attribute missing"})
			return
		}

		fileType, ok := ufFromObj["file_type"]
		if !ok {
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "file_type attribute missing"})
			return
		}

		folderPath, ok := ufFromObj["folder_path"]
		if !ok {
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "folder_path attribute missing"})
			return
		}
		docId, err := s.UploadFileFromUrl(r.Context(), projectId, storageName, url, fileName, docType, folderPath, fileType)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		fileNames := make(map[string]string)
		fileNames[fileName] = docId
		server_handlers.FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"files": fileNames})
	}
}

func FileUploadHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		logs.WithContext(r.Context()).Debug("FileUploadHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		storageName := vars["storagename"]

		var err error
		//ctx, cancel := context.WithTimeout(r.Context(), 30*time.Minute)
		//defer cancel()
		//_ = ctx

		reqContentType := strings.Split(r.Header.Get("Content-type"), ";")[0]
		if reqContentType == encodedForm || reqContentType == multiPartForm {
			err = r.ParseMultipartForm((1 << 20) * 10)

			formData := r.MultipartForm
			folderPath := formData.Value["folderpath"][0]
			docTypes := formData.Value["doctype"]
			//keyPairName := formData.Value["keyPairName"][0]
			fileNames := make(map[string]string)
			files := formData.File["files"]
			for _, f := range files {
				docType := ""
				for _, dt := range docTypes {
					tmpDt := strings.Split(dt, ":")
					if tmpDt[1] == f.Filename {
						docType = tmpDt[0]
						break
					}
				}
				file, err := f.Open()
				defer file.Close()
				if err != nil {
					fmt.Fprintln(w, err)
					return
				}
				//TODO - check for file size and check for file meme
				docId, err := s.UploadFile(r.Context(), projectId, storageName, file, f, docType, folderPath)
				if err != nil {
					server_handlers.FormatResponse(w, 400)
					_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
					return
				}
				fileNames[f.Filename] = docId
			}
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"files": fileNames})

			return
		} else {
			err = r.ParseForm()
		}
		if err != nil {
			logs.WithContext(r.Context()).Error(fmt.Sprint("Could not parse form: %s", err))
			return
		}
	}
}
