package module_store

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	eruaes "github.com/eru-tech/eru/eru-crypto/aes"
	erursa "github.com/eru-tech/eru/eru-crypto/rsa"
	"github.com/eru-tech/eru/eru-files/storage"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-repos/repos"
	"github.com/eru-tech/eru/eru-secret-manager/sm"
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

	var prjSm map[string]*json.RawMessage
	if _, ok := storeMap["secret_manager"]; ok {
		if storeMap["secret_manager"] != nil {
			err = json.Unmarshal(*storeMap["secret_manager"], &prjSm)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return err
			}
			for prj, smJson := range prjSm {
				var smObj map[string]*json.RawMessage
				err = json.Unmarshal(*smJson, &smObj)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					return err
				}
				var smType string
				if _, stOk := smObj["sm_store_type"]; stOk {
					err = json.Unmarshal(*smObj["sm_store_type"], &smType)
					if err != nil {
						logs.WithContext(ctx).Error(err.Error())
						return err
					}
					smI := sm.GetSm(smType)
					err = smI.MakeFromJson(ctx, smJson)
					if err == nil {
						err = msi.SaveSm(ctx, prj, smI, msi, false)
						if err != nil {
							return err
						}
					} else {
						return err
					}
				} else {
					logs.WithContext(ctx).Info("ignoring secret manager as sm_store_type attribute not found")
				}
			}
		} else {
			logs.WithContext(ctx).Info("secret manager attribute is nil")
		}
	} else {
		logs.WithContext(ctx).Info("secret manager attribute not found in store")
	}

	var prjRepo map[string]*json.RawMessage
	if _, ok := storeMap["repos"]; ok {
		if storeMap["repos"] != nil {
			err = json.Unmarshal(*storeMap["repos"], &prjRepo)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return err
			}
			for prj, repoJson := range prjRepo {
				var repoObj map[string]*json.RawMessage
				err = json.Unmarshal(*repoJson, &repoObj)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					return err
				}
				var repoType string
				if _, rtOk := repoObj["repo_type"]; rtOk {
					err = json.Unmarshal(*repoObj["repo_type"], &repoType)
					if err != nil {
						logs.WithContext(ctx).Error(err.Error())
						return err
					}
					repoI := repos.GetRepo(repoType)
					err = repoI.MakeFromJson(ctx, repoJson)
					if err == nil {
						err = msi.SaveRepo(ctx, prj, repoI, msi, false)
						if err != nil {
							return err
						}
					} else {
						return err
					}
				} else {
					logs.WithContext(ctx).Info("ignoring repo as repo type not found")
				}
			}
		} else {
			logs.WithContext(ctx).Info("repos attribute is nil")
		}
	} else {
		logs.WithContext(ctx).Info("repos attribute not found in store")
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
func (ms *ModuleStore) checkProjectExists(ctx context.Context, projectId string) error {
	logs.WithContext(ctx).Debug("checkProjectExists - Start")
	_, ok := ms.Projects[projectId]
	if !ok {
		err := errors.New(fmt.Sprint("project ", projectId, " not found"))
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}
