package storage

import (
	"context"
	"encoding/json"
	eruaes "github.com/eru-tech/eru/eru-crypto/aes"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"mime/multipart"
)

type GcpStorage struct {
	Storage
}

func (gcpStorage *GcpStorage) UploadFile(ctx context.Context, file multipart.File, header *multipart.FileHeader, docType string, folderPath string, keyName eruaes.AesKey) (docId string, err error) {
	return docId, err
}

func (gcpStorage *GcpStorage) UploadFileB64(ctx context.Context, file []byte, fileName string, docType string, folderPath string, keyName eruaes.AesKey) (docId string, err error) {
	return docId, err
}

func (gcpStorage *GcpStorage) DownloadFile(ctx context.Context, folderPath string, fileName string, keyName eruaes.AesKey) (file []byte, err error) {
	return
}

func (gcpStorage *GcpStorage) Init(ctx context.Context) error {
	return nil
}

func (gcpStorage *GcpStorage) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	err := json.Unmarshal(*rj, &gcpStorage)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}
