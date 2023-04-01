package file_model

import (
	"context"
	eruaes "github.com/eru-tech/eru/eru-crypto/aes"
	erursa "github.com/eru-tech/eru/eru-crypto/rsa"
	"github.com/eru-tech/eru/eru-files/storage"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

type FileProjectI interface {
	AddStorage(ctx context.Context, storageObj storage.StorageI)
	GenerateRsaKeyPair(ctx context.Context, bits int)
}

type Project struct {
	ProjectId   string                       `json:"project_id" eru:"required"`
	Storages    map[string]storage.StorageI  `json:"storages"`
	RsaKeyPairs map[string]erursa.RsaKeyPair `json:"rsa_keypairs"`
	AesKeys     map[string]eruaes.AesKey     `json:"aes_keys"`
}

func (prg *Project) AddStorage(ctx context.Context, storageObjI storage.StorageI) error {
	logs.WithContext(ctx).Debug("AddStorage - Start")
	storageName, err := storageObjI.GetAttribute("StorageName")
	if err == nil {
		prg.Storages[storageName.(string)] = storageObjI
		return nil
	}
	return err
}

func (prg *Project) GenerateRsaKeyPair(ctx context.Context, bits int, keyPairName string) (erursa.RsaKeyPair, error) {
	logs.WithContext(ctx).Debug("GenerateRsaKeyPair - Start")
	if prg.RsaKeyPairs == nil {
		prg.RsaKeyPairs = make(map[string]erursa.RsaKeyPair)
	}
	var err error
	prg.RsaKeyPairs[keyPairName], err = erursa.GenerateKeyPair(ctx, bits)
	return prg.RsaKeyPairs[keyPairName], err
}

func (prg *Project) GenerateAesKey(ctx context.Context, bits int, keyName string) (eruaes.AesKey, error) {
	logs.WithContext(ctx).Debug("GenerateAesKey - Start")
	if prg.AesKeys == nil {
		prg.AesKeys = make(map[string]eruaes.AesKey)
	}
	var err error
	prg.AesKeys[keyName], err = eruaes.GenerateKey(ctx, bits)
	return prg.AesKeys[keyName], err
}
