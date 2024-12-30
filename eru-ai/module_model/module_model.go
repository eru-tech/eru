package module_model

import (
	"context"
	"encoding/json"
	"errors"

	models "github.com/eru-tech/eru/eru-ai/models"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-secret-manager/sm"
	"github.com/eru-tech/eru/eru-store/store"
	utils "github.com/eru-tech/eru/eru-utils"
	"github.com/google/go-cmp/cmp"
)

type StoreCompare struct {
	store.StoreCompare
	DeleteModel   []string               `json:"delete_model"`
	NewModel      []string               `json:"new_model"`
	MismatchModel map[string]interface{} `json:"mismatch_model"`
}

type ModuleProjectI interface {
	AddModel(ctx context.Context, tenantId string, modelObj models.ModelI) error
	RemoveModel(ctx context.Context, tenantId string, modelId string) error
	CompareProject(ctx context.Context, compareProject Project) (StoreCompare, error)
}
type ExtendedProject struct {
	Project
	Variables           store.Variables            `json:"variables"`
	SecretManager       sm.SmStoreI                `json:"secret_manager"`
	TenantSecretManager sm.SmStoreI                `json:"tenant_secret_manager"`
	TenantVariables     map[string]store.Variables `json:"tenant_variables"`
}

type Project struct {
	ProjectId       string                  `json:"project_id" eru:"required"`
	Tenants         map[string]TenantConfig `json:"tenants"`
	ProjectSettings ProjectSettings         `json:"project_settings"`
}

type TenantConfig struct {
	TenantId string                   `json:"tenant_id" eru:"required"`
	Model    map[string]models.ModelI `json:"model"`
}

type ProjectSettings struct {
	ClaimsKey string `json:"claims_key" eru:"required"`
}

func (prj *Project) AddModel(ctx context.Context, tenantId string, modelObj models.ModelI) error {
	logs.WithContext(ctx).Debug("AddModel - Start")
	if prj.Tenants == nil {
		prj.Tenants = make(map[string]TenantConfig)
	}

	if _, ok := prj.Tenants[tenantId]; !ok {
		prj.Tenants[tenantId] = TenantConfig{
			TenantId: tenantId,
			Model:    make(map[string]models.ModelI),
		}
	}
	modelIdI, _ := modelObj.GetAttribute(ctx, "model_id")
	modelId := modelIdI.(string)
	if modelId == "" {
		return errors.New("model_id cannot be blank")
	}
	prj.Tenants[tenantId].Model[modelId] = modelObj
	return nil
}

func (prj *Project) RemoveModel(ctx context.Context, tenantId string, modelId string) error {
	logs.WithContext(ctx).Debug("RemoveModel - Start")
	if _, ok := prj.Tenants[tenantId]; !ok {
		return errors.New("tenant not found")
	}
	if _, ok := prj.Tenants[tenantId].Model[modelId]; !ok {
		return errors.New("model not found")
	}
	delete(prj.Tenants[tenantId].Model, modelId)
	return nil
}

func (ePrj *ExtendedProject) CompareProject(ctx context.Context, compareProject ExtendedProject) (StoreCompare, error) {
	logs.WithContext(ctx).Debug("CompareProject - Start")
	storeCompare := StoreCompare{}
	storeCompare.CompareVariables(ctx, ePrj.Variables, compareProject.Variables)
	storeCompare.CompareSecretManager(ctx, ePrj.SecretManager, compareProject.SecretManager)

	var diffR utils.DiffReporter
	if !cmp.Equal(ePrj.ProjectSettings, compareProject.ProjectSettings, cmp.Reporter(&diffR)) {
		if storeCompare.MismatchSettings == nil {
			storeCompare.MismatchSettings = make(map[string]interface{})
		}
		storeCompare.MismatchSettings["settings"] = diffR.Output()
	}
	//TODO: Add tenant comparison
	for _, tenantConfig := range ePrj.Tenants {
		for _, mm := range tenantConfig.Model {
			mllmNameI, _ := mm.GetAttribute(ctx, "llm_name")
			mllmName := mllmNameI.(string)
			var diffR utils.DiffReporter
			aFound := false
			for _, cm := range compareProject.Tenants[tenantConfig.TenantId].Model {
				cllmNameI, _ := cm.GetAttribute(ctx, "llm_name")
				cllmName := cllmNameI.(string)
				if mllmName == cllmName {
					aFound = true
					if !cmp.Equal(mm, cm, cmp.Reporter(&diffR)) {
						if storeCompare.MismatchModel == nil {
							storeCompare.MismatchModel = make(map[string]interface{})
						}
						storeCompare.MismatchModel[mllmName] = diffR.Output()
					}
					break
				}
			}
			if !aFound {
				storeCompare.DeleteModel = append(storeCompare.DeleteModel, mllmName)
			}
			for _, cm := range compareProject.Tenants[tenantConfig.TenantId].Model {
				cllmNameI, _ := cm.GetAttribute(ctx, "llm_name")
				cllmName := cllmNameI.(string)
				rFound := false
				for _, mm := range tenantConfig.Model {
					mllmNameI, _ := mm.GetAttribute(ctx, "llm_name")
					mllmName := mllmNameI.(string)
					if mllmName == cllmName {
						rFound = true
						break
					}
				}
				if !rFound {
					storeCompare.NewModel = append(storeCompare.NewModel, cllmName)
				}
			}
		}
	}
	return storeCompare, nil
}

func (ePrj *ExtendedProject) UnmarshalJSON(b []byte) error {
	logs.Logger.Info("UnMarshal ExtendedProject - Start")
	ctx := context.Background()
	var ePrjMap map[string]*json.RawMessage
	err := json.Unmarshal(b, &ePrjMap)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}

	projectId := ""
	if _, ok := ePrjMap["project_id"]; ok {
		if ePrjMap["project_id"] != nil {
			err = json.Unmarshal(*ePrjMap["project_id"], &projectId)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return err
			}
			ePrj.ProjectId = projectId
		}
	}

	var ps ProjectSettings
	if _, ok := ePrjMap["project_settings"]; ok {
		if ePrjMap["project_settings"] != nil {
			err = json.Unmarshal(*ePrjMap["project_settings"], &ps)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return err
			}
			ePrj.ProjectSettings = ps
		}
	}

	var vars store.Variables
	if _, ok := ePrjMap["variables"]; ok {
		if ePrjMap["variables"] != nil {
			err = json.Unmarshal(*ePrjMap["variables"], &vars)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return err
			}
			ePrj.Variables = vars
		}
	}

	var smObj map[string]*json.RawMessage
	var smJson *json.RawMessage
	if _, ok := ePrjMap["secret_manager"]; ok {
		if ePrjMap["secret_manager"] != nil {
			err = json.Unmarshal(*ePrjMap["secret_manager"], &smObj)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return err
			}
			err = json.Unmarshal(*ePrjMap["secret_manager"], &smJson)
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
					ePrj.SecretManager = smI
				} else {
					return err
				}
			} else {
				logs.WithContext(ctx).Info("ignoring secret manager as sm_store_type attribute not found")
			}
		} else {
			logs.WithContext(ctx).Info("secret manager attribute is nil")
		}
	} else {
		logs.WithContext(ctx).Info("secret manager attribute not found in store")
	}

	var tenants map[string]*json.RawMessage
	if _, ok := ePrjMap["tenants"]; ok {
		if ePrjMap["tenants"] != nil {
			err = json.Unmarshal(*ePrjMap["tenants"], &tenants)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return err
			}
			for tenantId, tenantConfigJson := range tenants {
				var tenantConfigObj map[string]*json.RawMessage
				err = json.Unmarshal(*tenantConfigJson, &tenantConfigObj)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					return err
				}
				var model map[string]*json.RawMessage
				if _, ok := tenantConfigObj["models"]; ok {
					if tenantConfigObj["models"] != nil {
						err = json.Unmarshal(*tenantConfigObj["models"], &model)
						if err != nil {
							logs.WithContext(ctx).Error(err.Error())
							return err
						}
						for _, modelJson := range model {
							var modelObj map[string]*json.RawMessage
							err = json.Unmarshal(*modelJson, &modelObj)
							if err != nil {
								logs.WithContext(ctx).Error(err.Error())
								return err
							}
							var llmName string
							err = json.Unmarshal(*modelObj["llm_name"], &llmName)
							if err != nil {
								logs.WithContext(ctx).Error(err.Error())
								return err
							}
							var modelId string
							err = json.Unmarshal(*modelObj["model_id"], &modelId)
							if err != nil {
								logs.WithContext(ctx).Error(err.Error())
								return err
							}
							modelI := models.GetModel(llmName)
							err = modelI.MakeFromJson(ctx, modelJson)
							if err == nil {
								if ePrj.Tenants == nil {
									ePrj.Tenants = make(map[string]TenantConfig)
								}
								ePrj.Tenants[tenantId].Model[modelId] = modelI
							} else {
								return err
							}
						}
					}
				} else {
					logs.WithContext(ctx).Info("model attribute is nil")
				}
			}
		} else {
			logs.WithContext(ctx).Info("tenant attribute is nil")
		}
	} else {
		logs.WithContext(ctx).Info("model attribute not found in store")
	}

	return nil
}
