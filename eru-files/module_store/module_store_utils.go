package module_store

import (
	"context"
	"encoding/json"
	eruaes "github.com/eru-tech/eru/eru-crypto/aes"
	erursa "github.com/eru-tech/eru/eru-crypto/rsa"
	"github.com/eru-tech/eru/eru-files/storage"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-store/store"
)

func UnMarshalStore(ctx context.Context, b []byte, msi ModuleStoreI) error {
	logs.WithContext(ctx).Debug("UnMarshalStore - Start")
	var storeMap map[string]*json.RawMessage
	err := json.Unmarshal(b, &storeMap)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}

	var prjs map[string]*json.RawMessage
	var vars map[string]*store.Variables
	if _, ok := storeMap["projects"]; ok {

		err = json.Unmarshal(*storeMap["Variables"], &vars)
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
			return err
		}
		msi.SetVars(ctx, vars)

		err = json.Unmarshal(*storeMap["projects"], &prjs)
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
			return err
		}

		for prj, prjJson := range prjs {
			msi.SaveProject(ctx, prj, nil, false)

			var prjObjs map[string]*json.RawMessage
			err = json.Unmarshal(*prjJson, &prjObjs)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return err
			}
			p, e := msi.GetProjectConfig(ctx, prj)
			if e != nil {
				return err
			}
			var aeskeys map[string]eruaes.AesKey
			err = json.Unmarshal(*prjObjs["aes_keys"], &aeskeys)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return err
			}
			p.AesKeys = aeskeys
			var keypairs map[string]*json.RawMessage
			err = json.Unmarshal(*prjObjs["rsa_keypairs"], &keypairs)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return err
			}
			for keypairKey, keypairJson := range keypairs {

				var keypairObj erursa.RsaKeyPair
				err = json.Unmarshal(*keypairJson, &keypairObj)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					return err
				}

				p.RsaKeyPairs[keypairKey] = keypairObj
			}
			var storages map[string]*json.RawMessage
			err = json.Unmarshal(*prjObjs["storages"], &storages)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return err
			}
			for _, storageJson := range storages {
				var storageObj map[string]*json.RawMessage
				err = json.Unmarshal(*storageJson, &storageObj)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					return err
				}
				var storageType string
				err = json.Unmarshal(*storageObj["storage_type"], &storageType)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					return err
				}
				storageI := storage.GetStorage(storageType)
				err = storageI.MakeFromJson(ctx, storageJson)
				if err == nil {
					err = msi.SaveStorage(ctx, storageI, prj, nil, false)
					if err != nil {
						return err
					}
				} else {
					return err
				}
			}
		}
	}
	return nil
}
