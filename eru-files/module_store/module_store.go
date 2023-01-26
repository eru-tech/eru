package module_store

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/csv"
	"errors"
	"fmt"
	eruaes "github.com/eru-tech/eru/eru-crypto/aes"
	erursa "github.com/eru-tech/eru/eru-crypto/rsa"
	"github.com/eru-tech/eru/eru-files/file_model"
	"github.com/eru-tech/eru/eru-files/storage"
	"github.com/eru-tech/eru/eru-store/store"
	utils "github.com/eru-tech/eru/eru-utils"
	"github.com/gabriel-vasile/mimetype"
	"github.com/gobwas/glob"
	"io/ioutil"
	"log"
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
}

const (
	MIME_CSV = "text/csv"
)

type ModuleStoreI interface {
	store.StoreI
	SaveProject(projectId string, realStore ModuleStoreI, persist bool) error
	RemoveProject(projectId string, realStore ModuleStoreI) error
	GetProjectConfig(projectId string) (*file_model.Project, error)
	GetProjectList() []map[string]interface{}
	SaveStorage(storageObj storage.StorageI, projectId string, realStore ModuleStoreI, persist bool) error
	RemoveStorage(storageName string, projectId string, realStore ModuleStoreI) error
	GenerateRsaKeyPair(projectId string, keyPairName string, bits int, overwrite bool, realStore ModuleStoreI) (rsaKeyPair erursa.RsaKeyPair, err error)
	GenerateAesKey(projectId string, keyPairName string, bits int, overwrite bool, realStore ModuleStoreI) (aesKey eruaes.AesKey, err error)
	UploadFile(projectId string, storageName string, file multipart.File, header *multipart.FileHeader, docType string, fodlerPath string) (docId string, err error)
	UploadFileB64(projectId string, storageName string, file []byte, fileName string, docType string, fodlerPath string) (docId string, err error)
	UploadFileFromUrl(projectId string, storageName string, url string, fileName string, docType string, fodlerPath string, fileType string) (docId string, err error)
	DownloadFile(projectId string, storageName string, folderPath string, fileName string) (file []byte, mimeType string, err error)
	DownloadFileB64(projectId string, storageName string, folderPath string, fileName string) (fileB64 string, mimeType string, err error)
	DownloadFileUnzip(projectId string, storageName string, fileDownloadRequest FileDownloadRequest) (files map[string]FileObj, err error)
	//TestEncrypt(projectId string, text string)
	//TestAesEncrypt(projectId string, text string, keyName string)
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

/*func (ms *ModuleStore) TestEncrypt(projectId string, text string) {
	prj, err := ms.GetProjectConfig(projectId)
	if err != nil {
		log.Print(err)
		return
	} else {
		etext, err := erursa.Encrypt([]byte(text), prj.RsaKeyPairs["testkeypair1"].PublicKey)
		log.Println(err)
		log.Println(text)
		log.Println(string(etext))
		dtext, err := erursa.Decrypt(etext, prj.RsaKeyPairs["testkeypair1"].PrivateKey)
		log.Println(string(dtext))
	}
}

func (ms *ModuleStore) TestAesEncrypt(projectId string, text string, keyName string) {
	prj, err := ms.GetProjectConfig(projectId)
	if err != nil {
		log.Print(err)
		return
	} else {
		log.Println("keyName = ", keyName)
		etext, err := eruaes.Encrypt([]byte(text), prj.AesKeys[keyName].Key)
		log.Println(err)
		log.Println(text)
		log.Println(string(etext))
		dtext, err := eruaes.Decrypt(etext, prj.AesKeys[keyName].Key)
		log.Println(string(dtext))
	}
}
*/
func (ms *ModuleStore) GenerateRsaKeyPair(projectId string, keyPairName string, bits int, overwrite bool, realStore ModuleStoreI) (rsaKeyPair erursa.RsaKeyPair, err error) {
	log.Println("inside GenerateRsaKeyPair")
	prj, err := ms.GetProjectConfig(projectId)
	if err != nil {
		log.Print(err)
		return
	} else {
		if _, ok := prj.RsaKeyPairs[keyPairName]; ok && !overwrite {
			err = errors.New(fmt.Sprint("keyPairName ", keyPairName, " already exists"))
			return
		} else {
			rsaKeyPair, err = prj.GenerateRsaKeyPair(bits, keyPairName)
			err = realStore.SaveStore("", realStore)
		}
	}
	return
}

func (ms *ModuleStore) GenerateAesKey(projectId string, keyName string, bits int, overwrite bool, realStore ModuleStoreI) (aesKey eruaes.AesKey, err error) {
	log.Println("inside GenerateAesKey")
	log.Println(bits)
	prj, err := ms.GetProjectConfig(projectId)
	if err != nil {
		log.Print(err)
		return
	} else {
		if _, ok := prj.AesKeys[keyName]; ok && !overwrite {
			err = errors.New(fmt.Sprint("keyname ", keyName, " already exists"))
			return
		} else {
			aesKey, err = prj.GenerateAesKey(bits, keyName)
			err = realStore.SaveStore("", realStore)
		}
	}
	return
}

func (ms *ModuleStore) SaveStorage(storageObj storage.StorageI, projectId string, realStore ModuleStoreI, persist bool) error {
	log.Println("inside SaveStorage")
	prj, err := ms.GetProjectConfig(projectId)
	if err != nil {
		log.Print(err)
		return err
	}
	err = prj.AddStorage(storageObj)
	if persist == true {
		return realStore.SaveStore("", realStore)
	}
	return nil
}

func (ms *ModuleStore) UploadFile(projectId string, storageName string, file multipart.File, header *multipart.FileHeader, docType string, folderPath string) (docId string, err error) {
	log.Println("inside UploadFile")
	log.Println(docType)
	prj, err := ms.GetProjectConfig(projectId)
	if err != nil {
		log.Print("error in GetProjectConfig ", err)
		return
	}

	if storageObj, ok := prj.Storages[storageName]; !ok {
		err = errors.New(fmt.Sprint("storage ", storageName, " not found"))
		return
	} else {
		keyName, kpErr := storageObj.GetAttribute("KeyPair")
		if err != nil {
			err = kpErr
			log.Print(err)
			return
		}
		log.Print(keyName.(string))
		docId, err = storageObj.UploadFile(file, header, docType, folderPath, prj.AesKeys[keyName.(string)])
		return
	}
}

func (ms *ModuleStore) UploadFileB64(projectId string, storageName string, file []byte, fileName string, docType string, folderPath string) (docId string, err error) {
	log.Println("inside UploadFile")
	log.Println(docType)
	prj, err := ms.GetProjectConfig(projectId)
	if err != nil {
		log.Print("error in GetProjectConfig ", err)
		return
	}

	if storageObj, ok := prj.Storages[storageName]; !ok {
		err = errors.New(fmt.Sprint("storage ", storageName, " not found"))
		return
	} else {
		keyName, kpErr := storageObj.GetAttribute("KeyPair")
		if err != nil {
			err = kpErr
			log.Print(err)
			return
		}
		log.Print(keyName.(string))
		docId, err = storageObj.UploadFileB64(file, fileName, docType, folderPath, prj.AesKeys[keyName.(string)])
		return
	}
}

func (ms *ModuleStore) UploadFileFromUrl(projectId string, storageName string, url string, fileName string, docType string, folderPath string, fileType string) (docId string, err error) {
	log.Print(url)
	reqHeaders := http.Header{}
	res, respHeaders, _, _, err := utils.CallHttp(http.MethodGet, url, reqHeaders, nil, nil, nil, nil)
	_ = res
	if err != nil {
		log.Print(err)
		return "", err
	}
	log.Print(respHeaders.Get("Content-Type"))
	log.Print(fileType)
	if respHeaders.Get("Content-Type") != fileType {
		log.Print("mismatch file type")

	}
	respBody := ""
	if respMap, ok := res.(map[string]interface{}); ok {
		if respBodyI, okb := respMap["body"]; okb {
			respBody = respBodyI.(string)
			return ms.UploadFileB64(projectId, storageName, []byte(respBody), fileName, docType, folderPath)
		} else {
			err = errors.New("response body or file attribute not found")
			log.Print(err)
			return "", err
		}
	} else {
		err = errors.New("response is not a map")
		log.Print(err)
		return "", err
	}
}

func (ms *ModuleStore) DownloadFileB64(projectId string, storageName string, folderPath string, fileName string) (fileB64 string, mimeType string, err error) {
	f, mt, e := ms.DownloadFile(projectId, storageName, folderPath, fileName)
	return base64.StdEncoding.EncodeToString(f), mt, e
}
func (ms *ModuleStore) DownloadFileUnzip(projectId string, storageName string, fileDownloadRequest FileDownloadRequest) (files map[string]FileObj, err error) {
	f, _, e := ms.DownloadFile(projectId, storageName, fileDownloadRequest.FolderPath, fileDownloadRequest.FileName)
	zipReader, err := zip.NewReader(bytes.NewReader(f), int64(len(f)))
	if err != nil {
		log.Print(err)
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
			log.Print("Reading file:", zipFile.Name)
			unzippedFileBytes, ziperr := readZipFile(zipFile)
			if ziperr != nil {
				err = ziperr
				log.Println(err)
			}
			mimetype.SetLimit(1000)
			fMime := mimetype.Detect(unzippedFileBytes)
			log.Print("fileDownloadRequest.CsvAsJson = ", fileDownloadRequest.CsvAsJson)
			log.Print("fMime = ", fMime)
			if fileDownloadRequest.CsvAsJson && fMime.Is(MIME_CSV) {
				log.Print("csv to json to be converted")
				csvReader := csv.NewReader(bytes.NewReader(unzippedFileBytes))
				csvData, csvErr := csvReader.ReadAll()
				if csvErr != nil {
					err = csvErr
					log.Print(err)
					return
				}
				jsonData, jsonErr := utils.CsvToMap(csvData, fileDownloadRequest.LowerCaseHeader)
				if jsonErr != nil {
					err = jsonErr
					log.Print(err)
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
func (ms *ModuleStore) DownloadFile(projectId string, storageName string, folderPath string, fileName string) (file []byte, mimeType string, err error) {
	log.Println("inside DownloadFile")
	prj, err := ms.GetProjectConfig(projectId)
	if err != nil {
		log.Print("error in GetProjectConfig ", err)
		return
	}

	if storageObj, ok := prj.Storages[storageName]; !ok {
		err = errors.New(fmt.Sprint("storage ", storageName, " not found"))
		return
	} else {
		keyName, kpErr := storageObj.GetAttribute("KeyPair")
		if err != nil {
			err = kpErr
			log.Print(err)
			return
		}
		log.Print(keyName.(string))
		file, err = storageObj.DownloadFile(folderPath, fileName, prj.AesKeys[keyName.(string)])
		return file, mimetype.Detect(file).String(), err
	}
}

func (ms *ModuleStore) SaveProject(projectId string, realStore ModuleStoreI, persist bool) error {
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
			log.Print("SaveStore called from SaveProject")
			return realStore.SaveStore("", realStore)
		} else {
			return nil
		}
	} else {
		return errors.New(fmt.Sprint("Project ", projectId, " already exists"))
	}
}

func (ms *ModuleStore) RemoveStorage(storageName string, projectId string, realStore ModuleStoreI) error {
	if prg, ok := ms.Projects[projectId]; ok {
		if _, ok := prg.Storages[storageName]; ok {
			delete(prg.Storages, storageName)
			log.Print("SaveStore called from RemoveStorage")
			return realStore.SaveStore("", realStore)
		} else {
			return errors.New(fmt.Sprint("Storage ", storageName, " does not exists"))
		}
	} else {
		return errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
	}
}

func (ms *ModuleStore) RemoveProject(projectId string, realStore ModuleStoreI) error {
	if _, ok := ms.Projects[projectId]; ok {
		delete(ms.Projects, projectId)
		log.Print("SaveStore called from RemoveProject")
		return realStore.SaveStore("", realStore)
	} else {
		return errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
	}
}

func (ms *ModuleStore) GetProjectConfig(projectId string) (*file_model.Project, error) {
	if _, ok := ms.Projects[projectId]; ok {
		//log.Println(store.Projects[projectId])
		return ms.Projects[projectId], nil
	} else {
		return nil, errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
	}
}

func (ms *ModuleStore) GetProjectList() []map[string]interface{} {
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

func readZipFile(zipFile *zip.File) ([]byte, error) {
	zf, err := zipFile.Open()
	if err != nil {
		log.Print(err)
		return nil, err
	}
	defer zf.Close()
	return ioutil.ReadAll(zf)
}
