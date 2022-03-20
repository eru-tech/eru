package file_model

import (
	eruaes "github.com/eru-tech/eru/eru-crypto/aes"
	erursa "github.com/eru-tech/eru/eru-crypto/rsa"
	"github.com/eru-tech/eru/eru-files/storage"
	"log"
)

type FileProjectI interface {
	AddStorage(storageObj storage.StorageI)
	GenerateRsaKeyPair(bits int)
}

type Project struct {
	ProjectId   string                       `json:"project_id" eru:"required"`
	Storages    map[string]storage.StorageI  `json:"storages"`
	RsaKeyPairs map[string]erursa.RsaKeyPair `json:"rsa_keypairs"`
	AesKeys     map[string]eruaes.AesKey     `json:"aes_keys"`
}

func (prg *Project) AddStorage(storageObjI storage.StorageI) error {
	storageName, err := storageObjI.GetAttribute("StorageName")
	log.Print(storageName.(string))
	if err == nil {
		prg.Storages[storageName.(string)] = storageObjI
		return nil
	}
	log.Println(prg)
	return err
}

func (prg *Project) GenerateRsaKeyPair(bits int, keyPairName string) (erursa.RsaKeyPair, error) {
	if prg.RsaKeyPairs == nil {
		prg.RsaKeyPairs = make(map[string]erursa.RsaKeyPair)
	}
	var err error
	prg.RsaKeyPairs[keyPairName], err = erursa.GenerateKeyPair(bits)
	return prg.RsaKeyPairs[keyPairName], err
}

func (prg *Project) GenerateAesKey(bits int, keyName string) (eruaes.AesKey, error) {
	if prg.AesKeys == nil {
		prg.AesKeys = make(map[string]eruaes.AesKey)
	}
	var err error
	prg.AesKeys[keyName], err = eruaes.GenerateKey(bits)
	log.Println(prg.AesKeys[keyName])
	return prg.AesKeys[keyName], err
}
