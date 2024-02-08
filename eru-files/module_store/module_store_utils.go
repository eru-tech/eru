package module_store

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	eruaes "github.com/eru-tech/eru/eru-crypto/aes"
	erursa "github.com/eru-tech/eru/eru-crypto/rsa"
	"github.com/eru-tech/eru/eru-files/storage"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-store/store"
	utils "github.com/eru-tech/eru/eru-utils"
)

func UnMarshalStore(ctx context.Context, b []byte, msi ModuleStoreI) error {
	logs.WithContext(ctx).Debug("UnMarshalStore - Start")
	var storeMap map[string]*json.RawMessage
	err := json.Unmarshal(b, &storeMap)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}

	var vars map[string]*store.Variables
	if _, ok := storeMap["Variables"]; ok {
		err = json.Unmarshal(*storeMap["Variables"], &vars)
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
			return err
		}
		msi.SetVars(ctx, vars)
	}

	var prjs map[string]*json.RawMessage
	if _, ok := storeMap["projects"]; ok {

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

func csvToJson(ctx context.Context, fileBytes []byte, fileDownloadRequest FileDownloadRequest) (jsonData []map[string]interface{}, err error) {
	logs.WithContext(ctx).Info(fmt.Sprint("inside csvToJson"))
	csvReader := csv.NewReader(bytes.NewReader(fileBytes))
	logs.WithContext(ctx).Info(fmt.Sprint("fileDownloadRequest.CsvDelimited = ", fileDownloadRequest.CsvDelimited))
	if fileDownloadRequest.CsvDelimited != 0 {
		csvReader.Comma = rune(fileDownloadRequest.CsvDelimited)
	}
	logs.WithContext(ctx).Info(fmt.Sprint(csvReader.Comma))
	csvData, csvErr := csvReader.ReadAll()
	if csvErr != nil {
		err = csvErr
		logs.WithContext(ctx).Error(err.Error())
		return
	}
	jsonData, err = utils.CsvToMap(ctx, csvData, fileDownloadRequest.LowerCaseHeader)
	if err != nil {
		return
	}
	return jsonData, err
}
