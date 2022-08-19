package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/eru-tech/eru/eru-files/module_store"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	"github.com/gorilla/mux"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
	//"github.com/aws/aws-sdk-go/aws/session"
	//"github.com/aws/aws-sdk-go/service/s3"
)

const (
	encodedForm   = "application/x-www-form-urlencoded"
	multiPartForm = "multipart/form-data"
)

/*
func TestEncrypt(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		projectId := vars["project"]
		text := vars["text"]
		s.TestEncrypt(projectId, text)
	}
}
func TestAesEncrypt(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		projectId := vars["project"]
		text := vars["text"]
		keyName := vars["keyname"]
		s.TestAesEncrypt(projectId, text, keyName)
	}
}

*/
func FileDownloadHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		log.Println("FileUploadHandler called")
		vars := mux.Vars(r)
		projectId := vars["project"]
		storageName := vars["storagename"]

		var err error
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Minute)
		defer cancel()
		_ = ctx

		dfFromReq := json.NewDecoder(r.Body)
		dfFromReq.DisallowUnknownFields()
		dfFromObj := make(map[string]string)

		if err := dfFromReq.Decode(&dfFromObj); err != nil {
			log.Println(err)
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}

		file, err := s.DownloadFile(projectId, storageName, dfFromObj["file_name"])
		if err != nil {
			log.Println(err)
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		//server_handlers.FormatResponse(w,http.StatusOK)
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", "attachment; filename=test.pdf")
		_, _ = io.Copy(w, bytes.NewReader(file))
	}
}
func FileUploadHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		log.Println("FileUploadHandler called")
		vars := mux.Vars(r)
		projectId := vars["project"]
		storageName := vars["storagename"]

		var err error
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Minute)
		defer cancel()
		_ = ctx

		reqContentType := strings.Split(r.Header.Get("Content-type"), ";")[0]
		log.Print("reqContentType = ", reqContentType)
		if reqContentType == encodedForm || reqContentType == multiPartForm {
			log.Println("inside encodedForm || multiPartForm")
			err = r.ParseMultipartForm((1 << 20) * 10)

			/*
				f, _, _ := r.FormFile("metadata")
				//metadata, _ := ioutil.ReadAll(f)
				if f != nil {
					buf2 := bytes.NewBuffer(nil)
					if _, err := io.Copy(buf2, f); err != nil {
						log.Println(err)
					}
					log.Println(buf2.String())
				}
				for _, h := range r.MultipartForm.File["media"] {
					file1, _ := h.Open()
					buf1 := bytes.NewBuffer(nil)
					if _, err := io.Copy(buf1, file1); err != nil {
						log.Println(err)
					}
					log.Println(buf1.String())
					//tmpfile, _ := os.Create("./" + h.Filename)
					//io.Copy(tmpfile, file)
					//tmpfile.Close()
					file1.Close()
				}*/

			formData := r.MultipartForm
			folderPath := formData.Value["folderpath"][0]
			docTypes := formData.Value["doctype"]
			//keyPairName := formData.Value["keyPairName"][0]
			log.Println(folderPath)
			log.Println(docTypes)
			fileNames := make(map[string]string)
			files := formData.File["files"]
			for _, f := range files {
				docType := ""
				for _, dt := range docTypes {
					tmpDt := strings.Split(dt, ":")
					log.Println(tmpDt)
					if tmpDt[1] == f.Filename {
						docType = tmpDt[0]
						break
					}
				}
				log.Println(docType)
				file, err := f.Open()
				defer file.Close()
				if err != nil {
					fmt.Fprintln(w, err)
					return
				}
				log.Println(f.Filename)
				log.Println(f.Size)
				log.Println(f.Header)
				//TODO - check for file size and check for file meme
				docId, err := s.UploadFile(projectId, storageName, file, f, docType, folderPath)
				if err != nil {
					log.Println(err)
					server_handlers.FormatResponse(w, 400)
					_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
					return
				}
				fileNames[f.Filename] = docId
			}
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"files": fileNames})

			//file,header,err := r.FormFile("files")
			//defer file.Close()
			//if err != nil {
			//	log.Println(err)
			//}

			//buf := bytes.NewBuffer(nil)
			//if _, err := io.Copy(buf, file); err != nil {
			//	log.Println(err)
			//}
			//log.Println(buf.String())

			//mimeType := http.DetectContentType(buf.Bytes())

			//log.Println(mimeType)

			return
		} else {
			err = r.ParseForm()
		}
		if err != nil {
			log.Println(err.Error())
			fmt.Errorf("Could not parse form: %s", err)
			return
		}
	}
}
