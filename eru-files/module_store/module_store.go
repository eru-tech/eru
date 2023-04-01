package module_store

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/csv"
	"errors"
	"fmt"
	eruaes "github.com/eru-tech/eru/eru-crypto/aes"
	erursa "github.com/eru-tech/eru/eru-crypto/rsa"
	"github.com/eru-tech/eru/eru-files/file_model"
	"github.com/eru-tech/eru/eru-files/storage"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-store/store"
	utils "github.com/eru-tech/eru/eru-utils"
	"github.com/gabriel-vasile/mimetype"
	"github.com/gobwas/glob"
	"io"
	"mime/multipart"
	"net/http"
)

type StoreHolder struct {
	Store ModuleStoreI
}

type FileObj struct {
	FileType string      `json:"file_type"`
	File     interface{} `json:"file"`
}

type FileDownloadRequest struct {
	FileName        string   `json:"file_name" eru:"required"`
	FolderPath      string   `json:"folder_path" eru:"required"`
	InnerFileNames  []string `json:"inner_file_names" eru:"required"`
	CsvAsJson       bool     `json:"csv_as_json"`
	LowerCaseHeader bool     `json:"lower_case_header"`
	Mime_Limit      uint32   `json:"mime_limit"`
}

const (
	MIME_CSV = "text/csv"
)

type ModuleStoreI interface {
	store.StoreI
	SaveProject(ctx context.Context, projectId string, realStore ModuleStoreI, persist bool) error
	RemoveProject(ctx context.Context, projectId string, realStore ModuleStoreI) error
	GetProjectConfig(ctx context.Context, projectId string) (*file_model.Project, error)
	GetProjectList(ctx context.Context) []map[string]interface{}
	SaveStorage(ctx context.Context, storageObj storage.StorageI, projectId string, realStore ModuleStoreI, persist bool) error
	RemoveStorage(ctx context.Context, storageName string, projectId string, realStore ModuleStoreI) error
	GenerateRsaKeyPair(ctx context.Context, projectId string, keyPairName string, bits int, overwrite bool, realStore ModuleStoreI) (rsaKeyPair erursa.RsaKeyPair, err error)
	GenerateAesKey(ctx context.Context, projectId string, keyPairName string, bits int, overwrite bool, realStore ModuleStoreI) (aesKey eruaes.AesKey, err error)
	UploadFile(ctx context.Context, projectId string, storageName string, file multipart.File, header *multipart.FileHeader, docType string, fodlerPath string) (docId string, err error)
	UploadFileB64(ctx context.Context, projectId string, storageName string, file []byte, fileName string, docType string, fodlerPath string) (docId string, err error)
	UploadFileFromUrl(ctx context.Context, projectId string, storageName string, url string, fileName string, docType string, fodlerPath string, fileType string) (docId string, err error)
	DownloadFile(ctx context.Context, projectId string, storageName string, folderPath string, fileName string) (file []byte, mimeType string, err error)
	DownloadFileB64(ctx context.Context, projectId string, storageName string, folderPath string, fileName string) (fileB64 string, mimeType string, err error)
	DownloadFileUnzip(ctx context.Context, projectId string, storageName string, fileDownloadRequest FileDownloadRequest) (files map[string]FileObj, err error)
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
			err = realStore.SaveStore(ctx, "", realStore)
		}
	}
	return
}

func (ms *ModuleStore) GenerateAesKey(ctx context.Context, projectId string, keyName string, bits int, overwrite bool, realStore ModuleStoreI) (aesKey eruaes.AesKey, err error) {
	logs.WithContext(ctx).Debug("GenerateAesKey - Start")
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
			err = realStore.SaveStore(ctx, "", realStore)
		}
	}
	return
}

func (ms *ModuleStore) SaveStorage(ctx context.Context, storageObj storage.StorageI, projectId string, realStore ModuleStoreI, persist bool) error {
	logs.WithContext(ctx).Debug("SaveStorage - Start")
	prj, err := ms.GetProjectConfig(ctx, projectId)
	if err != nil {
		return err
	}
	err = prj.AddStorage(ctx, storageObj)
	if persist == true {
		return realStore.SaveStore(ctx, "", realStore)
	}
	return nil
}

func (ms *ModuleStore) UploadFile(ctx context.Context, projectId string, storageName string, file multipart.File, header *multipart.FileHeader, docType string, folderPath string) (docId string, err error) {
	logs.WithContext(ctx).Debug("UploadFile - Start")
	prj, err := ms.GetProjectConfig(ctx, projectId)
	if err != nil {
		return
	}

	if storageObj, ok := prj.Storages[storageName]; !ok {
		err = errors.New(fmt.Sprint("storage ", storageName, " not found"))
		logs.WithContext(ctx).Info(err.Error())
		return
	} else {
		keyName, kpErr := storageObj.GetAttribute("KeyPair")
		if err != nil {
			err = kpErr
			return
		}
		docId, err = storageObj.UploadFile(ctx, file, header, docType, folderPath, prj.AesKeys[keyName.(string)])
		return
	}
}

func (ms *ModuleStore) UploadFileB64(ctx context.Context, projectId string, storageName string, file []byte, fileName string, docType string, folderPath string) (docId string, err error) {
	logs.WithContext(ctx).Debug("UploadFileB64 - Start")
	prj, err := ms.GetProjectConfig(ctx, projectId)
	if err != nil {
		return
	}

	if storageObj, ok := prj.Storages[storageName]; !ok {
		err = errors.New(fmt.Sprint("storage ", storageName, " not found"))
		logs.WithContext(ctx).Info(err.Error())
		return
	} else {
		keyName, kpErr := storageObj.GetAttribute("KeyPair")
		if err != nil {
			err = kpErr
			return
		}
		docId, err = storageObj.UploadFileB64(ctx, file, fileName, docType, folderPath, prj.AesKeys[keyName.(string)])
		return
	}
}

func (ms *ModuleStore) UploadFileFromUrl(ctx context.Context, projectId string, storageName string, url string, fileName string, docType string, folderPath string, fileType string) (docId string, err error) {
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
			return ms.UploadFileB64(ctx, projectId, storageName, []byte(respBody), fileName, docType, folderPath)
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

func (ms *ModuleStore) DownloadFileB64(ctx context.Context, projectId string, storageName string, folderPath string, fileName string) (fileB64 string, mimeType string, err error) {
	logs.WithContext(ctx).Debug("DownloadFileB64 - Start")
	f, mt, e := ms.DownloadFile(ctx, projectId, storageName, folderPath, fileName)
	return base64.StdEncoding.EncodeToString(f), mt, e
}
func (ms *ModuleStore) DownloadFileUnzip(ctx context.Context, projectId string, storageName string, fileDownloadRequest FileDownloadRequest) (files map[string]FileObj, err error) {
	logs.WithContext(ctx).Debug("DownloadFileUnzip - Start")
	f, _, e := ms.DownloadFile(ctx, projectId, storageName, fileDownloadRequest.FolderPath, fileDownloadRequest.FileName)
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
			logs.WithContext(ctx).Debug(fmt.Sprint("Reading file:", zipFile.Name))
			unzippedFileBytes, ziperr := readZipFile(ctx, zipFile)
			if ziperr != nil {
				err = ziperr
			}
			mimetype.SetLimit(2000)
			if fileDownloadRequest.Mime_Limit > 0 {
				mimetype.SetLimit(fileDownloadRequest.Mime_Limit)
			}
			fMime := mimetype.Detect(unzippedFileBytes)
			logs.WithContext(ctx).Debug(fmt.Sprint("fileDownloadRequest.CsvAsJson = ", fileDownloadRequest.CsvAsJson))
			logs.WithContext(ctx).Debug(fmt.Sprint("fMime = ", fMime))
			logs.WithContext(ctx).Debug(fmt.Sprint("fileDownloadRequest.Mime_Limit = ", fileDownloadRequest.Mime_Limit))

			if fileDownloadRequest.CsvAsJson && fMime.Is(MIME_CSV) {
				csvReader := csv.NewReader(bytes.NewReader(unzippedFileBytes))
				csvData, csvErr := csvReader.ReadAll()
				if csvErr != nil {
					err = csvErr
					logs.WithContext(ctx).Error(err.Error())
					return
				}
				jsonData, jsonErr := utils.CsvToMap(ctx, csvData, fileDownloadRequest.LowerCaseHeader)
				if jsonErr != nil {
					err = jsonErr
					return
				}
				fo.File = jsonData
			} else {
				fo.File = base64.StdEncoding.EncodeToString(unzippedFileBytes)
			}
			fo.FileType = fMime.String()
			files[zipFile.Name] = fo
		}
	}
	return files, e
}
func (ms *ModuleStore) DownloadFile(ctx context.Context, projectId string, storageName string, folderPath string, fileName string) (file []byte, mimeType string, err error) {
	logs.WithContext(ctx).Debug("DownloadFile - Start")
	prj, err := ms.GetProjectConfig(ctx, projectId)
	if err != nil {
		return
	}

	if storageObj, ok := prj.Storages[storageName]; !ok {
		err = errors.New(fmt.Sprint("storage ", storageName, " not found"))
		logs.WithContext(ctx).Error(err.Error())
		return
	} else {
		keyName, kpErr := storageObj.GetAttribute("KeyPair")
		if err != nil {
			err = kpErr
			return
		}
		file, err = storageObj.DownloadFile(ctx, folderPath, fileName, prj.AesKeys[keyName.(string)])
		return file, mimetype.Detect(file).String(), err
	}
}

func (ms *ModuleStore) SaveProject(ctx context.Context, projectId string, realStore ModuleStoreI, persist bool) error {
	logs.WithContext(ctx).Debug("SaveProject - Start")
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
			return realStore.SaveStore(ctx, "", realStore)
		} else {
			return nil
		}
	} else {
		err := errors.New(fmt.Sprint("Project ", projectId, " already exists"))
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
}

func (ms *ModuleStore) RemoveStorage(ctx context.Context, storageName string, projectId string, realStore ModuleStoreI) error {
	logs.WithContext(ctx).Debug("RemoveStorage - Start")
	if prg, ok := ms.Projects[projectId]; ok {
		if _, ok := prg.Storages[storageName]; ok {
			delete(prg.Storages, storageName)
			logs.WithContext(ctx).Info("SaveStore called from RemoveStorage")
			return realStore.SaveStore(ctx, "", realStore)
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
	if _, ok := ms.Projects[projectId]; ok {
		delete(ms.Projects, projectId)
		logs.WithContext(ctx).Info("SaveStore called from RemoveProject")
		return realStore.SaveStore(ctx, "", realStore)
	} else {
		err := errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
		logs.WithContext(ctx).Error(err.Error())
		return err
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
		project["projectName"] = k
		//project["lastUpdateDate"] = time.Now()
		projects[i] = project
		i++
	}
	return projects
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
