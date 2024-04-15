package kms

import (
	"context"
	"encoding/json"
	"errors"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

const (
	AuthTypeSecret = "SECRET"
	AuthTypeIAM    = "IAM"
)

type KmsStore struct {
	KmsStoreType string `json:"kms_store_type" eru:"required"`
}

type KmsStoreI interface {
	Init(ctx context.Context) (err error)
	MakeFromJson(ctx context.Context, rj *json.RawMessage) error
	CreateKey(ctx context.Context, keyName string, keyDesc string) (err error)
	ListKeys(ctx context.Context) (keyList []string, err error)
	Encrypt(ctx context.Context, keyId string, plainText []byte) (encryptedText []byte, err error)
	Decrypt(ctx context.Context, keyId string, encryptedText []byte) (plainText []byte, err error)
}

func GetKms(storageType string) KmsStoreI {
	switch storageType {
	case "AWS":
		return new(AwsKmsStore)
	case "GCP":
		return new(GcpKmsStore)

	default:
		return nil
	}
	return nil
}

func (kmsStore *KmsStore) Init(ctx context.Context) (err error) {
	err = errors.New("method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return
}

func (kmsStore *KmsStore) CreateKey(ctx context.Context, keyName string, keyDesc string) (err error) {
	err = errors.New("method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return
}

func (kmsStore *KmsStore) Decrypt(ctx context.Context, keyId string, encryptedText []byte) (plainText []byte, err error) {
	err = errors.New("method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return
}

func (kmsStore *KmsStore) ListKeys(ctx context.Context) (keyList []string, err error) {
	err = errors.New("method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return
}

func (kmsStore *KmsStore) Encrypt(ctx context.Context, keyId string, plainText []byte) (encryptedText []byte, err error) {
	err = errors.New("method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return
}

func (kmsStore *KmsStore) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &kmsStore)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}
