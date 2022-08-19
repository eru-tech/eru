package module_store

import (
	"errors"
	"fmt"
	eruaes "github.com/eru-tech/eru/eru-crypto/aes"
	erursa "github.com/eru-tech/eru/eru-crypto/rsa"
	"github.com/eru-tech/eru/eru-files/file_model"
	"github.com/eru-tech/eru/eru-files/storage"
	"github.com/eru-tech/eru/eru-store/store"
	"log"
	"mime/multipart"
)

type StoreHolder struct {
	Store ModuleStoreI
}
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
	DownloadFile(projectId string, storageName string, fileName string) (file []byte, err error)
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

func (ms *ModuleStore) DownloadFile(projectId string, storageName string, fileName string) (file []byte, err error) {
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
		file, err = storageObj.DownloadFile(fileName, prj.AesKeys[keyName.(string)])
		return file, err
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
