package storage

import (
	"encoding/json"
	eruaes "github.com/eru-tech/eru/eru-crypto/aes"
	"log"
	"mime/multipart"
)

type GcpStorage struct {
	Storage
}

func (gcpStorage *GcpStorage) UploadFile(file multipart.File, header *multipart.FileHeader, docType string, folderPath string, keyName eruaes.AesKey) (docId string, err error) {
	return docId, err
}

func (gcpStorage *GcpStorage) DownloadFile(fileName string, keyName eruaes.AesKey) (file []byte, err error) {
	return
}

func (gcpStorage *GcpStorage) Init() error {
	return nil
}

func (gcpStorage *GcpStorage) MakeFromJson(rj *json.RawMessage) error {
	err := json.Unmarshal(*rj, &gcpStorage)
	if err != nil {
		log.Print("error json.Unmarshal(*rj, &awsStorage)")
		log.Print(err)
		return err
	}
	return nil
}
