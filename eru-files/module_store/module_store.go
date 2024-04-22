package module_store

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	eruaes "github.com/eru-tech/eru/eru-crypto/aes"
	erursa "github.com/eru-tech/eru/eru-crypto/rsa"
	"github.com/eru-tech/eru/eru-files/file_model"
	"github.com/eru-tech/eru/eru-files/storage"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eru_reads "github.com/eru-tech/eru/eru-read-write/eru-reads"
	"github.com/eru-tech/eru/eru-store/store"
	utils "github.com/eru-tech/eru/eru-utils"
	"github.com/gabriel-vasile/mimetype"
	"github.com/gobwas/glob"
	"io"
	"mime/multipart"
	"net/http"
	"reflect"
)

type StoreHolder struct {
	Store ModuleStoreI
}

type FileObj struct {
	FileType string      `json:"file_type"`
	File     interface{} `json:"file"`
}

type FileDownloadRequest struct {
	FileName        string                                       `json:"file_name" eru:"required"`
	FolderPath      string                                       `json:"folder_path" eru:"required"`
	InnerFileNames  []string                                     `json:"inner_file_names" eru:"required"`
	CsvAsJson       bool                                         `json:"csv_as_json"`
	CsvDelimited    int32                                        `json:"csv_delimited"`
	ExcelAsJson     bool                                         `json:"excel_as_json"`
	ExcelSheets     map[string]map[string]eru_reads.FileReadData `json:"excel_sheets"`
	LowerCaseHeader bool                                         `json:"lower_case_header"`
	MimeLimit       uint32                                       `json:"mime_limit"`
}

const (
	MIME_TEXT = "text/plain"
	MIME_CSV  = "text/csv"
	MIME_XLSX = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
)

type ModuleStoreI interface {
	store.StoreI
	SaveProject(ctx context.Context, projectId string, realStore ModuleStoreI, persist bool) error
	RemoveProject(ctx context.Context, projectId string, realStore ModuleStoreI) error
	GetProjectConfig(ctx context.Context, projectId string) (*file_model.Project, error)
	GetExtendedProjectConfig(ctx context.Context, projectId string, realStore ModuleStoreI) (file_model.ExtendedProject, error)
	GetProjectList(ctx context.Context) []map[string]interface{}
	SaveStorage(ctx context.Context, storageObj storage.StorageI, projectId string, realStore ModuleStoreI, persist bool) error
	RemoveStorage(ctx context.Context, storageName string, projectId string, cloudDelete bool, forceDelete bool, realStore ModuleStoreI) error
	GenerateRsaKeyPair(ctx context.Context, projectId string, keyPairName string, bits int, overwrite bool, realStore ModuleStoreI) (rsaKeyPair erursa.RsaKeyPair, err error)
	GenerateAesKey(ctx context.Context, projectId string, keyPairName string, bits int, overwrite bool, realStore ModuleStoreI) (aesKey eruaes.AesKey, err error)
	UploadFile(ctx context.Context, projectId string, storageName string, file multipart.File, header *multipart.FileHeader, docType string, fodlerPath string, s ModuleStoreI) (docId string, err error)
	UploadFileB64(ctx context.Context, projectId string, storageName string, file []byte, fileName string, docType string, fodlerPath string, s ModuleStoreI) (docId string, err error)
	UploadFileFromUrl(ctx context.Context, projectId string, storageName string, url string, fileName string, docType string, fodlerPath string, fileType string, s ModuleStoreI) (docId string, err error)
	DownloadFile(ctx context.Context, projectId string, storageName string, fileDownloadRequest FileDownloadRequest, s ModuleStoreI) (file []byte, mimeType string, err error)
	DownloadFileAsJson(ctx context.Context, projectId string, storageName string, fileDownloadRequest FileDownloadRequest, s ModuleStoreI) (jsonData []map[string]interface{}, err error)
	DownloadFileB64(ctx context.Context, projectId string, storageName string, fileDownloadRequest FileDownloadRequest, s ModuleStoreI) (fileB64 string, mimeType string, err error)
	DownloadFileUnzip(ctx context.Context, projectId string, storageName string, fileDownloadRequest FileDownloadRequest, s ModuleStoreI) (files map[string]FileObj, err error)
	SaveProjectSettings(ctx context.Context, projectId string, projectSettings file_model.ProjectSettings, realStore ModuleStoreI) error
}

type ModuleStore struct {
	Projects map[string]*file_model.Project `json:"projects"` //ProjectId is the key
}

type ModuleFileStore struct {
	store.FileStore
	ModuleStore
}
type ModuleDbStore struct {
	store.DbStore
	ModuleStore
}

func (ms *ModuleStore) GenerateRsaKeyPair(ctx context.Context, projectId string, keyPairName string, bits int, overwrite bool, realStore ModuleStoreI) (rsaKeyPair erursa.RsaKeyPair, err error) {
	logs.WithContext(ctx).Debug("GenerateRsaKeyPair - Start")
	realStore.GetMutex().Lock()
	defer realStore.GetMutex().Unlock()
	prj, err := ms.GetProjectConfig(ctx, projectId)
	if err != nil {
		return
	} else {
		if _, ok := prj.RsaKeyPairs[keyPairName]; ok && !overwrite {
			err = errors.New(fmt.Sprint("keyPairName ", keyPairName, " already exists"))
			logs.WithContext(ctx).Info(err.Error())
			return
		} else {
			rsaKeyPair, err = prj.GenerateRsaKeyPair(ctx, bits, keyPairName)
			err = realStore.SaveStore(ctx, projectId, "", realStore)
		}
	}
	return
}

func (ms *ModuleStore) GenerateAesKey(ctx context.Context, projectId string, keyName string, bits int, overwrite bool, realStore ModuleStoreI) (aesKey eruaes.AesKey, err error) {
	logs.WithContext(ctx).Debug("GenerateAesKey - Start")
	realStore.GetMutex().Lock()
	defer realStore.GetMutex().Unlock()
	prj, err := ms.GetProjectConfig(ctx, projectId)
	if err != nil {
		return
	} else {
		if _, ok := prj.AesKeys[keyName]; ok && !overwrite {
			err = errors.New(fmt.Sprint("keyname ", keyName, " already exists"))
			logs.WithContext(ctx).Info(err.Error())
			return
		} else {
			aesKey, err = prj.GenerateAesKey(ctx, bits, keyName)
			err = realStore.SaveStore(ctx, projectId, "", realStore)
		}
	}
	return
}

func (ms *ModuleStore) SaveStorage(ctx context.Context, storageObj storage.StorageI, projectId string, realStore ModuleStoreI, persist bool) error {
	logs.WithContext(ctx).Debug("SaveStorage - Start")
	if persist {
		realStore.GetMutex().Lock()
		defer realStore.GetMutex().Unlock()
	}
	prj, err := ms.GetProjectConfig(ctx, projectId)
	if err != nil {
		return err
	}

	if persist == true {
		err = storageObj.CreateStorage(ctx)
		if err != nil {
			return err
		}
	}

	err = prj.AddStorage(ctx, storageObj)

	if persist == true {
		return realStore.SaveStore(ctx, projectId, "", realStore)
	}
	return nil
}

func (ms *ModuleStore) GetStorageClone(ctx context.Context, projectId string, storageName string, s ModuleStoreI) (storageObjClone storage.StorageI, prj *file_model.Project, err error) {
	prj, err = ms.GetProjectConfig(ctx, projectId)
	if err != nil {
		return
	}

	if storageObj, ok := prj.Storages[storageName]; !ok {
		err = errors.New(fmt.Sprint("storage ", storageName, " not found"))
		logs.WithContext(ctx).Error(err.Error())
		return
	} else {
		storageObjJson, storageObjJsonErr := json.Marshal(storageObj)
		if storageObjJsonErr != nil {
			err = errors.New(fmt.Sprint("error while cloning storageObj (marshal)"))
			logs.WithContext(ctx).Error(err.Error())
			logs.WithContext(ctx).Error(storageObjJsonErr.Error())
			return
		}
		storageObjJson = s.ReplaceVariables(ctx, projectId, storageObjJson)

		iCloneI := reflect.New(reflect.TypeOf(storageObj))
		storageObjCloneErr := json.Unmarshal(storageObjJson, iCloneI.Interface())
		if storageObjCloneErr != nil {
			err = errors.New(fmt.Sprint("error while cloning storageObj(unmarshal)"))
			logs.WithContext(ctx).Error(err.Error())
			logs.WithContext(ctx).Error(storageObjCloneErr.Error())
			return
		}
		return iCloneI.Elem().Interface().(storage.StorageI), prj, nil
	}
}

func (ms *ModuleStore) UploadFile(ctx context.Context, projectId string, storageName string, file multipart.File, header *multipart.FileHeader, docType string, folderPath string, s ModuleStoreI) (docId string, err error) {
	logs.WithContext(ctx).Info("UploadFile - Start")
	storageObjClone, prj, sErr := ms.GetStorageClone(ctx, projectId, storageName, s)
	if sErr != nil {
		return
	}
	keyName, kpErr := storageObjClone.GetAttribute("key_pair")
	if kpErr != nil {
		err = kpErr
		return
	}
	kmsName, kpErr := storageObjClone.GetAttribute("key_id")
	if kpErr != nil {
		err = kpErr
		return
	}
	kmsMap, kmsErr := s.FetchKms(ctx, projectId)
	if kmsErr != nil {
		err = kmsErr
		return
	}
	storageObjClone.SetKms(ctx, kmsMap[kmsName.(string)])
	docId, err = storageObjClone.UploadFile(ctx, file, header, docType, folderPath, prj.AesKeys[keyName.(string)])
	return
}

func (ms *ModuleStore) UploadFileB64(ctx context.Context, projectId string, storageName string, file []byte, fileName string, docType string, folderPath string, s ModuleStoreI) (docId string, err error) {
	logs.WithContext(ctx).Debug("UploadFileB64 - Start")
	storageObjClone, prj, sErr := ms.GetStorageClone(ctx, projectId, storageName, s)
	if sErr != nil {
		return
	}
	keyName, kpErr := storageObjClone.GetAttribute("key_pair")
	if err != nil {
		err = kpErr
		return
	}
	docId, err = storageObjClone.UploadFileB64(ctx, file, fileName, docType, folderPath, prj.AesKeys[keyName.(string)])
	return
}

func (ms *ModuleStore) UploadFileFromUrl(ctx context.Context, projectId string, storageName string, url string, fileName string, docType string, folderPath string, fileType string, s ModuleStoreI) (docId string, err error) {
	logs.WithContext(ctx).Debug("UploadFileFromUrl - Start")
	reqHeaders := http.Header{}
	res, respHeaders, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, reqHeaders, nil, nil, nil, nil)
	_ = res
	if err != nil {
		return "", err
	}
	if respHeaders.Get("Content-Type") != fileType {
		logs.WithContext(ctx).Warn("mismatch file type")
	}
	respBody := ""
	if respMap, ok := res.(map[string]interface{}); ok {
		if respBodyI, okb := respMap["body"]; okb {
			respBody = respBodyI.(string)
			return ms.UploadFileB64(ctx, projectId, storageName, []byte(respBody), fileName, docType, folderPath, s)
		} else {
			err = errors.New("response body or file attribute not found")
			logs.WithContext(ctx).Error(err.Error())
			return "", err
		}
	} else {
		err = errors.New("response is not a map")
		logs.WithContext(ctx).Error(err.Error())
		return "", err
	}
}

func (ms *ModuleStore) DownloadFileB64(ctx context.Context, projectId string, storageName string, fileDownloadRequest FileDownloadRequest, s ModuleStoreI) (fileB64 string, mimeType string, err error) {
	logs.WithContext(ctx).Debug("DownloadFileB64 - Start")
	f, mt, e := ms.DownloadFile(ctx, projectId, storageName, fileDownloadRequest, s)
	return base64.StdEncoding.EncodeToString(f), mt, e
}
func (ms *ModuleStore) DownloadFileUnzip(ctx context.Context, projectId string, storageName string, fileDownloadRequest FileDownloadRequest, s ModuleStoreI) (files map[string]FileObj, err error) {
	logs.WithContext(ctx).Debug("DownloadFileUnzip - Start")
	f, _, e := ms.DownloadFile(ctx, projectId, storageName, fileDownloadRequest, s)
	zipReader, err := zip.NewReader(bytes.NewReader(f), int64(len(f)))
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
	}
	// Read all the files from zip archive
	fo := FileObj{}
	files = make(map[string]FileObj)
	for _, zipFile := range zipReader.File {
		fileToUnzip := false
		for _, ifn := range fileDownloadRequest.InnerFileNames {
			g := glob.MustCompile(ifn)
			if g.Match(zipFile.Name) {
				fileToUnzip = true
			}
		}
		if len(fileDownloadRequest.InnerFileNames) == 0 {
			fileToUnzip = true
		}
		if fileToUnzip {
			logs.WithContext(ctx).Info(fmt.Sprint("Reading file:", zipFile.Name))
			unzippedFileBytes, ziperr := readZipFile(ctx, zipFile)
			if ziperr != nil {
				err = ziperr
			}
			mimetype.SetLimit(2000)
			if fileDownloadRequest.MimeLimit > 0 {
				mimetype.SetLimit(fileDownloadRequest.MimeLimit)
			}
			fMime := mimetype.Detect(unzippedFileBytes)
			logs.WithContext(ctx).Info(fmt.Sprint("fileDownloadRequest.CsvAsJson = ", fileDownloadRequest.CsvAsJson))
			logs.WithContext(ctx).Info(fmt.Sprint("fMime = ", fMime))
			logs.WithContext(ctx).Info(fmt.Sprint("fileDownloadRequest.Mime_Limit = ", fileDownloadRequest.MimeLimit))

			if fileDownloadRequest.CsvAsJson && fMime.Is(MIME_CSV) {
				jsonData, jsonErr := csvToJson(ctx, unzippedFileBytes, fileDownloadRequest)
				if jsonErr != nil {
					err = jsonErr
					return
				}
				fo.File = jsonData
			} else if fileDownloadRequest.ExcelAsJson && fMime.Is(MIME_XLSX) {
				logs.WithContext(ctx).Info("inside MIME_XLSX")
				logs.WithContext(ctx).Info(fmt.Sprint("fileDownloadRequest.ExcelAsJson = ", fileDownloadRequest.ExcelAsJson))
				if !fileDownloadRequest.ExcelAsJson {
					fo.File = base64.StdEncoding.EncodeToString(unzippedFileBytes)
				} else {
					var sheets map[string]eru_reads.FileReadData
					if fileDownloadRequest.ExcelSheets != nil {
						for fn, v := range fileDownloadRequest.ExcelSheets {
							if fn == zipFile.Name || fn == "*" {
								sheets = v
								break
							}
						}
					}
					erd := eru_reads.ExcelReadData{Sheets: sheets}
					jsonData, jsonErr := erd.ReadAsJson(ctx, unzippedFileBytes)
					if jsonErr != nil {
						err = jsonErr
						return
					}
					fo.File = jsonData
				}
			} else {
				fo.File = base64.StdEncoding.EncodeToString(unzippedFileBytes)
			}
			fo.FileType = fMime.String()
			files[zipFile.Name] = fo
		}
	}
	return files, e
}
func (ms *ModuleStore) DownloadFile(ctx context.Context, projectId string, storageName string, fileDownloadRequest FileDownloadRequest, s ModuleStoreI) (file []byte, mimeType string, err error) {
	logs.WithContext(ctx).Debug("DownloadFile - Start")
	storageObjClone, prj, sErr := ms.GetStorageClone(ctx, projectId, storageName, s)
	if sErr != nil {
		return
	}
	keyName, kpErr := storageObjClone.GetAttribute("key_pair")
	if err != nil {
		err = kpErr
		return
	}
	kmsName, kpErr := storageObjClone.GetAttribute("key_id")
	if kpErr != nil {
		err = kpErr
		return
	}
	kmsMap, kmsErr := s.FetchKms(ctx, projectId)
	if kmsErr != nil {
		err = kmsErr
		return
	}
	storageObjClone.SetKms(ctx, kmsMap[kmsName.(string)])

	file, err = storageObjClone.DownloadFile(ctx, fileDownloadRequest.FolderPath, fileDownloadRequest.FileName, prj.AesKeys[keyName.(string)])
	return file, mimetype.Detect(file).String(), err
}

func (ms *ModuleStore) DownloadFileAsJson(ctx context.Context, projectId string, storageName string, fileDownloadRequest FileDownloadRequest, s ModuleStoreI) (jsonData []map[string]interface{}, err error) {
	logs.WithContext(ctx).Debug("DownloadFileAsJson - Start")
	f, m, e := ms.DownloadFile(ctx, projectId, storageName, fileDownloadRequest, s)
	if e != nil {
		logs.WithContext(ctx).Error(e.Error())
		return
	}
	logs.WithContext(ctx).Info(fmt.Sprint(m))

	fMime := mimetype.Detect(f)
	logs.WithContext(ctx).Info(fmt.Sprint("fileDownloadRequest.ExcelAsJson = ", fileDownloadRequest.ExcelAsJson))
	logs.WithContext(ctx).Info(fmt.Sprint("fMime ", fMime))

	if fileDownloadRequest.CsvAsJson && (fMime.Is(MIME_TEXT) || fMime.Is(MIME_CSV)) {
		jsonData, err = csvToJson(ctx, f, fileDownloadRequest)
		if err != nil {
			return
		}
	} else if fileDownloadRequest.ExcelAsJson && fMime.Is(MIME_XLSX) {
		var sheets map[string]eru_reads.FileReadData
		if fileDownloadRequest.ExcelSheets != nil {
			for fn, v := range fileDownloadRequest.ExcelSheets {
				if fn == fileDownloadRequest.FileName || fn == "*" {
					sheets = v
					break
				}
			}
		}
		erd := eru_reads.ExcelReadData{Sheets: sheets}
		jsonDataObj, jsonErr := erd.ReadAsJson(ctx, f)
		if jsonErr != nil {
			err = jsonErr
			return
		}
		jsonData = append(jsonData, jsonDataObj)
	}
	return
}

func (ms *ModuleStore) SaveProject(ctx context.Context, projectId string, realStore ModuleStoreI, persist bool) error {
	logs.WithContext(ctx).Debug("SaveProject - Start")
	if persist {
		realStore.GetMutex().Lock()
		defer realStore.GetMutex().Unlock()
	}
	//TODO to handle edit project once new project attributes are finalized
	if _, ok := ms.Projects[projectId]; !ok {
		project := new(file_model.Project)
		project.ProjectId = projectId
		if ms.Projects == nil {
			ms.Projects = make(map[string]*file_model.Project)
		}
		if project.Storages == nil {
			project.Storages = make(map[string]storage.StorageI)
		}
		if project.RsaKeyPairs == nil {
			project.RsaKeyPairs = make(map[string]erursa.RsaKeyPair)
		}
		if project.AesKeys == nil {
			project.AesKeys = make(map[string]eruaes.AesKey)
		}
		ms.Projects[projectId] = project
		if persist == true {
			logs.WithContext(ctx).Info("SaveStore called from SaveProject")
			return realStore.SaveStore(ctx, projectId, "", realStore)
		} else {
			return nil
		}
	} else {
		err := errors.New(fmt.Sprint("Project ", projectId, " already exists"))
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
}

func (ms *ModuleStore) RemoveStorage(ctx context.Context, storageName string, projectId string, cloudDelete bool, forceDelete bool, realStore ModuleStoreI) (err error) {
	logs.WithContext(ctx).Debug("RemoveStorage - Start")
	realStore.GetMutex().Lock()
	defer realStore.GetMutex().Unlock()
	if prg, ok := ms.Projects[projectId]; ok {
		if _, ok := prg.Storages[storageName]; ok {
			if cloudDelete {
				err = prg.Storages[storageName].DeleteStorage(ctx, forceDelete)
				if err != nil {
					return
				}
			}

			delete(prg.Storages, storageName)
			logs.WithContext(ctx).Info("SaveStore called from RemoveStorage")
			return realStore.SaveStore(ctx, projectId, "", realStore)
		} else {
			err := errors.New(fmt.Sprint("Storage ", storageName, " does not exists"))
			logs.WithContext(ctx).Error(err.Error())
			return err
		}
	} else {
		err := errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
}

func (ms *ModuleStore) RemoveProject(ctx context.Context, projectId string, realStore ModuleStoreI) error {
	logs.WithContext(ctx).Debug("RemoveProject - Start")
	realStore.GetMutex().Lock()
	defer realStore.GetMutex().Unlock()
	if _, ok := ms.Projects[projectId]; ok {
		delete(ms.Projects, projectId)
		logs.WithContext(ctx).Info("SaveStore called from RemoveProject")
		return realStore.SaveStore(ctx, projectId, "", realStore)
	} else {
		err := errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
}
func (ms *ModuleStore) GetExtendedProjectConfig(ctx context.Context, projectId string, realStore ModuleStoreI) (ePrj file_model.ExtendedProject, err error) {
	logs.WithContext(ctx).Debug("GetExtendedProjectConfig - Start")
	ePrj = file_model.ExtendedProject{}
	if prj, ok := ms.Projects[projectId]; ok {
		ePrj.Variables, err = realStore.FetchVars(ctx, projectId)
		ePrj.SecretManager, err = realStore.FetchSm(ctx, projectId)
		ePrj.ProjectId = prj.ProjectId
		ePrj.Storages = prj.Storages
		ePrj.ProjectSettings = prj.ProjectSettings
		ePrj.AesKeys = prj.AesKeys
		ePrj.RsaKeyPairs = prj.RsaKeyPairs
		return ePrj, nil
	} else {
		err := errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
		}
		return file_model.ExtendedProject{}, err
	}
}
func (ms *ModuleStore) GetProjectConfig(ctx context.Context, projectId string) (*file_model.Project, error) {
	logs.WithContext(ctx).Debug("GetProjectConfig - Start")
	if _, ok := ms.Projects[projectId]; ok {
		return ms.Projects[projectId], nil
	} else {
		err := errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
}

func (ms *ModuleStore) GetProjectList(ctx context.Context) []map[string]interface{} {
	logs.WithContext(ctx).Debug("GetProjectList - Start")
	projects := make([]map[string]interface{}, len(ms.Projects))
	i := 0
	for k := range ms.Projects {
		project := make(map[string]interface{})
		project["project_name"] = k
		//project["lastUpdateDate"] = time.Now()
		projects[i] = project
		i++
	}
	return projects
}

func (ms *ModuleStore) SaveProjectSettings(ctx context.Context, projectId string, projectSettings file_model.ProjectSettings, realStore ModuleStoreI) error {
	logs.WithContext(ctx).Debug("SaveProjectConfig - Start")
	realStore.GetMutex().Lock()
	defer realStore.GetMutex().Unlock()
	err := ms.checkProjectExists(ctx, projectId)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	ms.Projects[projectId].ProjectSettings = projectSettings
	logs.WithContext(ctx).Info("SaveStore called from SaveProjectSettings")
	return realStore.SaveStore(ctx, projectId, "", realStore)
}

func readZipFile(ctx context.Context, zipFile *zip.File) ([]byte, error) {
	logs.WithContext(ctx).Debug("readZipFile - Start")
	zf, err := zipFile.Open()
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
	defer zf.Close()
	return io.ReadAll(zf)
}

func GetStore(storeType string) ModuleStoreI {
	switch storeType {
	case "POSTGRES":
		return new(ModuleDbStore)
	case "STANDALONE":
		return new(ModuleFileStore)
	default:
		return nil
	}
}
