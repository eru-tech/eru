package file_model

import (
	"context"
	eruaes "github.com/eru-tech/eru/eru-crypto/aes"
	erursa "github.com/eru-tech/eru/eru-crypto/rsa"
	"github.com/eru-tech/eru/eru-files/storage"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	utils "github.com/eru-tech/eru/eru-utils"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

type StoreCompare struct {
	DeleteStorages   []string               `json:"delete_storages"`
	NewStorages      []string               `json:"new_storages"`
	MismatchStorages map[string]interface{} `json:"mismatch_storages"`
	DeleteKeys       []string               `json:"delete_keys"`
	NewKeys          []string               `json:"new_keys"`
	MismatchKeys     map[string]interface{} `json:"mismatch_keys"`
}

type FileProjectI interface {
	AddStorage(ctx context.Context, storageObj storage.StorageI)
	GenerateRsaKeyPair(ctx context.Context, bits int)
	CompareProject(ctx context.Context, compareProject Project) (StoreCompare, error)
}

type Project struct {
	ProjectId       string                       `json:"project_id" eru:"required"`
	Storages        map[string]storage.StorageI  `json:"storages"`
	RsaKeyPairs     map[string]erursa.RsaKeyPair `json:"rsa_keypairs"`
	AesKeys         map[string]eruaes.AesKey     `json:"aes_keys"`
	ProjectSettings ProjectSettings              `json:"project_settings"`
}
type ProjectSettings struct {
	ClaimsKey string `json:"claims_key" eru:"required"`
}

func (prj *Project) AddStorage(ctx context.Context, storageObjI storage.StorageI) error {
	logs.WithContext(ctx).Debug("AddStorage - Start")
	storageName, err := storageObjI.GetAttribute("storage_name")
	if err == nil {
		prj.Storages[storageName.(string)] = storageObjI
		return nil
	}
	return err
}

func (prj *Project) GenerateRsaKeyPair(ctx context.Context, bits int, keyPairName string) (erursa.RsaKeyPair, error) {
	logs.WithContext(ctx).Debug("GenerateRsaKeyPair - Start")
	if prj.RsaKeyPairs == nil {
		prj.RsaKeyPairs = make(map[string]erursa.RsaKeyPair)
	}
	var err error
	prj.RsaKeyPairs[keyPairName], err = erursa.GenerateKeyPair(ctx, bits)
	return prj.RsaKeyPairs[keyPairName], err
}

func (prj *Project) GenerateAesKey(ctx context.Context, bits int, keyName string) (eruaes.AesKey, error) {
	logs.WithContext(ctx).Debug("GenerateAesKey - Start")
	if prj.AesKeys == nil {
		prj.AesKeys = make(map[string]eruaes.AesKey)
	}
	var err error
	prj.AesKeys[keyName], err = eruaes.GenerateKey(ctx, bits)
	return prj.AesKeys[keyName], err
}

func (prj *Project) CompareProject(ctx context.Context, compareProject Project) (StoreCompare, error) {
	logs.WithContext(ctx).Debug("CompareProject - Start")
	storeCompare := StoreCompare{}
	for _, ms := range prj.Storages {
		msNameI, _ := ms.GetAttribute("storage_name")
		msName := msNameI.(string)
		var diffR utils.DiffReporter
		sFound := false
		for _, cs := range compareProject.Storages {
			csNameI, _ := cs.GetAttribute("storage_name")
			csName := csNameI.(string)
			if msName == csName {
				sFound = true
				if !cmp.Equal(ms, cs, cmpopts.IgnoreUnexported(storage.AwsStorage{}), cmp.Reporter(&diffR)) {
					if storeCompare.MismatchStorages == nil {
						storeCompare.MismatchStorages = make(map[string]interface{})
					}
					storeCompare.MismatchStorages[msName] = diffR.Output()
				}
				break
			}
		}
		if !sFound {
			storeCompare.DeleteStorages = append(storeCompare.DeleteStorages, msName)
		}
	}

	for _, cs := range compareProject.Storages {
		csNameI, _ := cs.GetAttribute("storage_name")
		csName := csNameI.(string)
		sFound := false
		for _, ms := range prj.Storages {
			msNameI, _ := ms.GetAttribute("storage_name")
			msName := msNameI.(string)
			if msName == csName {
				sFound = true
				break
			}
		}
		if !sFound {
			storeCompare.NewStorages = append(storeCompare.NewStorages, csName)
		}
	}

	//compare Keys
	for mKey, mk := range prj.AesKeys {
		var diffR utils.DiffReporter
		kFound := false
		for cKey, ck := range compareProject.AesKeys {
			if mKey == cKey {
				kFound = true
				if !cmp.Equal(mk, ck, cmp.Reporter(&diffR)) {
					if storeCompare.MismatchKeys == nil {
						storeCompare.MismatchKeys = make(map[string]interface{})
					}
					storeCompare.MismatchKeys[mKey] = diffR.Output()
				}
				break
			}
		}
		if !kFound {
			storeCompare.DeleteKeys = append(storeCompare.DeleteKeys, mKey)
		}
	}

	for cKey, _ := range compareProject.AesKeys {
		kFound := false
		for mKey, _ := range prj.AesKeys {
			if mKey == cKey {
				kFound = true
				break
			}
		}
		if !kFound {
			storeCompare.NewKeys = append(storeCompare.NewKeys, cKey)
		}
	}

	return storeCompare, nil
}
