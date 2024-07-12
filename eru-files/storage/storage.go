package storage

import (
	"context"
	"encoding/json"
	"errors"
	eruaes "github.com/eru-tech/eru/eru-crypto/aes"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-secret-manager/kms"
	"mime/multipart"
)

type StorageI interface {
	UploadFile(ctx context.Context, file multipart.File, header *multipart.FileHeader, docType string, folderPath string, keyName eruaes.AesKey) (docId string, err error)
	UploadFileB64(ctx context.Context, file []byte, fileName string, docType string, folderPath string, keyName eruaes.AesKey) (docId string, err error)
	DownloadFile(ctx context.Context, folderPath string, fileName string, keyName eruaes.AesKey) (file []byte, err error)
	GetAttribute(attributeName string) (attributeValue interface{}, err error)
	MakeFromJson(ctx context.Context, rj *json.RawMessage) error
	CreateStorage(ctx context.Context, cloneStorage StorageI, persist bool) (err error)
	DeleteStorage(ctx context.Context, forceDelete bool, cloneStorage StorageI) (err error)
	Init(ctx context.Context) error
	SetKms(ctx context.Context, kmsObj kms.KmsStoreI) (err error)
	BucketExists(ctx context.Context) (exists bool, err error)
	EmptyBucket() error
}

type Storage struct {
	StorageType  string        `json:"storage_type" eru:"required"`
	StorageName  string        `json:"storage_name" eru:"required"`
	EncryptFiles bool          `json:"encrypt_files" eru:"required"`
	KeyPair      string        `json:"key_pair"`
	KmsId        string        `json:"key_id"`
	KmsKey       kms.KmsStoreI `json:"-"`
}

func (storage *Storage) GetAttribute(attributeName string) (attributeValue interface{}, err error) {
	switch attributeName {
	case "storage_name":
		return storage.StorageName, nil
	case "storage_type":
		return storage.StorageType, nil
	case "key_pair":
		return storage.KeyPair, nil
	case "key_id":
		return storage.KmsId, nil
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

func (storage *Storage) CreateStorage(ctx context.Context, cloneStorage StorageI, persist bool) (err error) {
	err = errors.New("method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return
}

func (storage *Storage) BucketExists(ctx context.Context) (exists bool, err error) {
	err = errors.New("method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return
}

func (storage *Storage) EmptyBucket() (err error) {
	err = errors.New("method not implemented")
	logs.WithContext(context.Background()).Error(err.Error())
	return
}

func (storage *Storage) DeleteStorage(ctx context.Context, forceDelete bool, cloneStorage StorageI) (err error) {
	err = errors.New("method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return
}

func (storage *Storage) SetKms(ctx context.Context, kmsObj kms.KmsStoreI) (err error) {
	storage.KmsKey = kmsObj
	return
}
