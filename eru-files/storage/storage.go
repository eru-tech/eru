package storage

import (
	"context"
	"encoding/json"
	"errors"
	eruaes "github.com/eru-tech/eru/eru-crypto/aes"
	"mime/multipart"
)

type StorageI interface {
	UploadFile(ctx context.Context, file multipart.File, header *multipart.FileHeader, docType string, folderPath string, keyName eruaes.AesKey) (docId string, err error)
	UploadFileB64(ctx context.Context, file []byte, fileName string, docType string, folderPath string, keyName eruaes.AesKey) (docId string, err error)
	DownloadFile(ctx context.Context, folderPath string, fileName string, keyName eruaes.AesKey) (file []byte, err error)
	GetAttribute(attributeName string) (attributeValue interface{}, err error)
	MakeFromJson(ctx context.Context, rj *json.RawMessage) error
	Init(ctx context.Context) error
}

type Storage struct {
	StorageType  string `json:"storage_type" eru:"required"`
	StorageName  string `json:"storage_name" eru:"required"`
	EncryptFiles bool   `json:"encrypt_files" eru:"required"`
	KeyPair      string `json:"key_pair" eru:"required"`
}

func (storage *Storage) GetAttribute(attributeName string) (attributeValue interface{}, err error) {
	switch attributeName {
	case "StorageName":
		return storage.StorageName, nil
	case "StorageType":
		return storage.StorageType, nil
	case "KeyPair":
		return storage.KeyPair, nil
	default:
		return nil, errors.New("Attribute not found")
	}
}

func GetStorage(storageType string) StorageI {
	switch storageType {
	case "AWS":
		return new(AwsStorage)
	case "GCP":
		return new(GcpStorage)
	default:
		return nil
	}
	return nil
}
