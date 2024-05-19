package module_store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/eru-tech/eru/eru-auth/auth"
	"github.com/eru-tech/eru/eru-auth/gateway"
	"github.com/eru-tech/eru/eru-auth/module_model"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-store/store"
	"github.com/google/uuid"
	"reflect"
	"strings"
)

const (
	INSERT_PKCE_EVENT   = "insert into eruauth_pkce_events (pkce_event_id,code_verifier,code_challenge,request_id,nonce,url) values ($1,$2,$3,$4,$5,$6)"
	SELECT_PKCE_EVENT   = "select * from eruauth_pkce_events where request_id = $1"
	SELECT_IDENTITY_SUB = "select * from eruauth_identities where identity_provider_id = $1"
)

var Erufuncbaseurl = "http://localhost:8083"

type StoreHolder struct {
	Store ModuleStoreI
}
type ModuleStoreI interface {
	store.StoreI
	SaveProject(ctx context.Context, projectId string, realStore ModuleStoreI, persist bool) error
	RemoveProject(ctx context.Context, projectId string, realStore ModuleStoreI) error
	GetProjectConfig(ctx context.Context, projectId string) (*module_model.Project, error)
	GetExtendedProjectConfig(ctx context.Context, projectId string, realStore ModuleStoreI) (module_model.ExtendedProject, error)
	GetProjectList(ctx context.Context) []map[string]interface{}
	SaveMessageTemplate(ctx context.Context, projectId string, messageTemplate module_model.MessageTemplate, realStore ModuleStoreI) error
	RemoveMessageTemplate(ctx context.Context, projectId string, templateId string, realStore ModuleStoreI) error
	SaveGateway(ctx context.Context, gatewayObj gateway.GatewayI, projectId string, realStore ModuleStoreI, persist bool) error
	RemoveGateway(ctx context.Context, gatewayName string, gatewayType string, channel string, projectId string, realStore ModuleStoreI) error
	GetGatewayFromType(ctx context.Context, gatewayType string, channel string, projectId string) (gateway.GatewayI, error)
	GetMessageTemplate(ctx context.Context, gatewayName string, projectId string, templateType string) (module_model.MessageTemplate, error)
	SaveAuth(ctx context.Context, authObj auth.AuthI, projectId string, realStore ModuleStoreI, persist bool) error
	RemoveAuth(ctx context.Context, authType string, projectId string, realStore ModuleStoreI) error
	GetAuth(ctx context.Context, projectId string, authName string, s ModuleStoreI) (auth.AuthI, error)
	SavePkceEvent(ctx context.Context, msParams auth.OAuthParams, s ModuleStoreI) (err error)
	GetPkceEvent(ctx context.Context, requestId string, s ModuleStoreI) (msParams auth.OAuthParams, err error)
	SaveProjectSettings(ctx context.Context, projectId string, projectSettings module_model.ProjectSettings, realStore ModuleStoreI) error
}

type ModuleStore struct {
	Projects map[string]*module_model.Project `json:"projects"` //ProjectId is the key
}

type ModuleFileStore struct {
	store.FileStore
	ModuleStore
}
type ModuleDbStore struct {
	store.DbStore
	ModuleStore
}

func (ms *ModuleStore) SaveProject(ctx context.Context, projectId string, realStore ModuleStoreI, persist bool) error {
	//TODO to handle edit project once new project attributes are finalized
	logs.WithContext(ctx).Debug("SaveProject - Start")
	if persist {
		realStore.GetMutex().Lock()
		defer realStore.GetMutex().Unlock()
	}
	if _, ok := ms.Projects[projectId]; !ok {
		project := new(module_model.Project)
		project.ProjectId = projectId
		if ms.Projects == nil {
			ms.Projects = make(map[string]*module_model.Project)
		}
		if project.Gateways == nil {
			project.Gateways = make(map[string]gateway.GatewayI)
		}
		//if project.SmsGateways == nil {
		//	project.SmsGateways = make(map[string]module_model.SmsGateway)
		//}
		//if project.EmailGateways == nil {
		//	project.EmailGateways = make(map[string]module_model.EmailGateway)
		//}
		if project.MessageTemplates == nil {
			project.MessageTemplates = make(map[string]module_model.MessageTemplate)
		}
		if project.Auth == nil {
			project.Auth = make(map[string]auth.AuthI)
		}

		ms.Projects[projectId] = project
		if persist == true {
			logs.WithContext(ctx).Info("SaveStore called from SaveProject")
			return realStore.SaveStore(ctx, projectId, "", realStore)
		} else {
			return nil
		}
	} else {
		err := errors.New(fmt.Sprint("Project ", projectId, " already exists"))
		logs.WithContext(ctx).Info(err.Error())
		return err
	}
}

func (ms *ModuleStore) RemoveProject(ctx context.Context, projectId string, realStore ModuleStoreI) error {
	logs.WithContext(ctx).Debug("RemoveProject - Start")
	realStore.GetMutex().Lock()
	defer realStore.GetMutex().Unlock()
	if _, ok := ms.Projects[projectId]; ok {
		delete(ms.Projects, projectId)
		logs.WithContext(ctx).Info("SaveStore called from RemoveProject")
		return realStore.SaveStore(ctx, projectId, "", realStore)
	} else {
		err := errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
		logs.WithContext(ctx).Info(err.Error())
		return err
	}
}

func (ms *ModuleStore) GetExtendedProjectConfig(ctx context.Context, projectId string, realStore ModuleStoreI) (ePrj module_model.ExtendedProject, err error) {
	logs.WithContext(ctx).Debug("GetExtendedProjectConfig - Start")
	ePrj = module_model.ExtendedProject{}
	if prj, ok := ms.Projects[projectId]; ok {
		ePrj.Variables, err = realStore.FetchVars(ctx, projectId)
		ePrj.SecretManager, err = realStore.FetchSm(ctx, projectId)
		ePrj.ProjectId = prj.ProjectId
		ePrj.ProjectSettings = prj.ProjectSettings
		ePrj.MessageTemplates = prj.MessageTemplates
		ePrj.Gateways = prj.Gateways
		ePrj.Auth = prj.Auth
		return ePrj, nil
	} else {
		err = errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
		}
		return module_model.ExtendedProject{}, err
	}
}

func (ms *ModuleStore) GetProjectConfig(ctx context.Context, projectId string) (*module_model.Project, error) {
	logs.WithContext(ctx).Debug("GetProjectConfig - Start")
	if _, ok := ms.Projects[projectId]; ok {
		return ms.Projects[projectId], nil
	} else {
		err := errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
		logs.WithContext(ctx).Info(err.Error())
		return nil, err
	}
}

func (ms *ModuleStore) GetProjectList(ctx context.Context) []map[string]interface{} {
	logs.WithContext(ctx).Debug("GetProjectList - Start")
	projects := make([]map[string]interface{}, len(ms.Projects))
	i := 0
	for k := range ms.Projects {
		project := make(map[string]interface{})
		project["project_name"] = k
		projects[i] = project
		i++
	}
	return projects
}

func (ms *ModuleStore) SaveMessageTemplate(ctx context.Context, projectId string, messageTemplate module_model.MessageTemplate, realStore ModuleStoreI) error {
	logs.WithContext(ctx).Debug("SaveMessageTemplate - Start")
	realStore.GetMutex().Lock()
	defer realStore.GetMutex().Unlock()
	err := ms.checkProjectExists(ctx, projectId)
	if err != nil {
		return err
	}
	if ms.Projects[projectId].MessageTemplates == nil {
		ms.Projects[projectId].MessageTemplates = make(map[string]module_model.MessageTemplate)
	}
	templateName := fmt.Sprint(messageTemplate.GatewayName, "_", messageTemplate.TemplateType)
	ms.Projects[projectId].MessageTemplates[templateName] = messageTemplate
	return realStore.SaveStore(ctx, projectId, "", realStore)
}

func (ms *ModuleStore) RemoveMessageTemplate(ctx context.Context, projectId string, templateName string, realStore ModuleStoreI) error {
	logs.WithContext(ctx).Debug("RemoveMessageTemplate - Start")
	realStore.GetMutex().Lock()
	defer realStore.GetMutex().Unlock()
	err := ms.checkProjectExists(ctx, projectId)
	if err != nil {
		return err
	}
	if _, ok := ms.Projects[projectId].MessageTemplates[templateName]; ok {
		delete(ms.Projects[projectId].MessageTemplates, templateName)
		return realStore.SaveStore(ctx, projectId, "", realStore)
	} else {
		err = errors.New(fmt.Sprint("MessageTemplates ", templateName, " does not exists"))
		logs.WithContext(ctx).Info(err.Error())
		return err
	}
}

func (ms *ModuleStore) SaveGateway(ctx context.Context, gatewayObj gateway.GatewayI, projectId string, realStore ModuleStoreI, persist bool) error {
	logs.WithContext(ctx).Debug("SaveGateway - Start")
	if persist {
		realStore.GetMutex().Lock()
		defer realStore.GetMutex().Unlock()
	}
	prj, err := ms.GetProjectConfig(ctx, projectId)
	if err != nil {
		return err
	}
	err = prj.AddGateway(ctx, gatewayObj)
	if persist == true {
		return realStore.SaveStore(ctx, projectId, "", realStore)
	}
	return nil
}

func (ms *ModuleStore) RemoveGateway(ctx context.Context, gatewayName string, gatewayType string, channel string, projectId string, realStore ModuleStoreI) error {
	logs.WithContext(ctx).Debug("RemoveGateway - Start")
	realStore.GetMutex().Lock()
	defer realStore.GetMutex().Unlock()
	if prg, ok := ms.Projects[projectId]; ok {
		gKey := fmt.Sprint(gatewayName, "_", gatewayType, "_", channel)
		if _, ok := prg.Gateways[gKey]; ok {
			delete(prg.Gateways, gKey)
			logs.WithContext(ctx).Info("SaveStore called from RemoveGateway")
			return realStore.SaveStore(ctx, projectId, "", realStore)
		} else {
			err := errors.New(fmt.Sprint("Gateway ", gKey, " does not exists"))
			logs.WithContext(ctx).Info(err.Error())
			return err
		}
	} else {
		err := errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
		logs.WithContext(ctx).Info(err.Error())
		return err
	}
}

func (ms *ModuleStore) GetGatewayFromType(ctx context.Context, gatewayType string, channel string, projectId string) (gateway.GatewayI, error) {
	logs.WithContext(ctx).Debug("GetGatewayFromType - Start")
	if prg, ok := ms.Projects[projectId]; ok {
		//todo to random selection of gateway based on allocation in case multiple gateway are defined
		if prg.Gateways != nil {
			for _, v := range prg.Gateways {
				gt, err := v.GetAttribute("gateway_type")
				ch, err := v.GetAttribute("channel")
				if err != nil {
					return nil, err
				}
				if gt.(string) == gatewayType && ch.(string) == channel {
					return v, nil
				}
			}
		} else {
			err := errors.New(fmt.Sprint("No Gateways Defined"))
			logs.WithContext(ctx).Info(err.Error())
			return nil, err
		}
	} else {
		err := errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
		logs.WithContext(ctx).Info(err.Error())
		return nil, err
	}
	err := errors.New(fmt.Sprint("Gateway ", gatewayType, " not found"))
	logs.WithContext(ctx).Info(err.Error())
	return nil, err
}

func (ms *ModuleStore) GetMessageTemplate(ctx context.Context, gatewayName string, projectId string, templateType string) (mt module_model.MessageTemplate, err error) {
	logs.WithContext(ctx).Debug("GetMessageTemplate - Start")
	if prg, ok := ms.Projects[projectId]; ok {
		if prg.MessageTemplates != nil {
			for k, v := range prg.MessageTemplates {
				if k == fmt.Sprint(gatewayName, "_", templateType) {
					return v, nil
				}
			}
		} else {
			err = errors.New(fmt.Sprint("No Message Templates Defined"))
			logs.WithContext(ctx).Info(err.Error())
			return mt, err

		}
	} else {
		err = errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
		logs.WithContext(ctx).Info(err.Error())
		return mt, err
	}
	err = errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
	logs.WithContext(ctx).Info(err.Error())
	return mt, err
}
func (ms *ModuleStore) SaveAuth(ctx context.Context, authObj auth.AuthI, projectId string, realStore ModuleStoreI, persist bool) error {
	logs.WithContext(ctx).Debug("SaveAuth - Start")
	if persist {
		realStore.GetMutex().Lock()
		defer realStore.GetMutex().Unlock()
	}

	//cloning authObj to replace variables and execute PerformPreSaveTask with actual values
	authObjClone, err := ms.GetAuthCloneObject(ctx, projectId, authObj, realStore)
	prj, err := ms.GetProjectConfig(ctx, projectId)
	if err != nil {
		return err
	}
	authName, err := authObjClone.GetAttribute(ctx, "auth_name")
	if err != nil {
		return err
	}
	if persist == true {
		err = authObjClone.PerformPreSaveTask(ctx)
		if err != nil {
			return err
		}
	}
	//save original authObj with variables
	err = prj.AddAuth(ctx, authName.(string), authObj)
	if err != nil {
		return err
	}

	if persist == true {
		return realStore.SaveStore(ctx, projectId, "", realStore)
	}
	return nil
}
func (ms *ModuleStore) RemoveAuth(ctx context.Context, authName string, projectId string, realStore ModuleStoreI) (err error) {
	logs.WithContext(ctx).Debug("RemoveAuth - Start")
	realStore.GetMutex().Lock()
	defer realStore.GetMutex().Unlock()
	if prg, ok := ms.Projects[projectId]; ok {
		if authObj, ok := prg.Auth[authName]; ok {
			err = authObj.PerformPreDeleteTask(ctx)
			if err != nil {
				return
			}
		} else {
			err = errors.New(fmt.Sprint("Auth ", authName, " does not exists"))
			logs.WithContext(ctx).Info(err.Error())
			return err
		}
		err = prg.RemoveAuth(ctx, authName)
		if err != nil {
			return err
		}
	} else {
		err = errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
		logs.WithContext(ctx).Info(err.Error())
		return err
	}
	return realStore.SaveStore(ctx, projectId, "", realStore)
}

func (ms *ModuleStore) GetAuthClone(ctx context.Context, projectId string, authName string, s ModuleStoreI) (authObjClone auth.AuthI, err error) {
	logs.WithContext(ctx).Debug("GetAuthClone - Start")
	prj, err := ms.GetProjectConfig(ctx, projectId)
	if err != nil {
		return
	}

	if authObj, ok := prj.Auth[authName]; !ok {
		err = errors.New(fmt.Sprint("auth ", authName, " not found"))
		logs.WithContext(ctx).Error(err.Error())
		return
	} else {
		authObjClone, err = ms.GetAuthCloneObject(ctx, projectId, authObj, s)
		authObjClone.SetAuthDb(GetAuthDb(s.GetDbType()))
		var kmsIdI interface{}
		kmsIdI, err = authObjClone.GetAttribute(ctx, "kms_id")
		if err == nil && kmsIdI != nil {
			kmsMap, kmsErr := s.FetchKms(ctx, projectId)
			if kmsErr != nil {
				err = kmsErr
				//return
			} else {
				authObjClone.SetKms(ctx, kmsMap[kmsIdI.(string)])
			}
		}

		return
	}
}

func (ms *ModuleStore) GetAuthCloneObject(ctx context.Context, projectId string, authObj auth.AuthI, s ModuleStoreI) (authObjClone auth.AuthI, err error) {
	logs.WithContext(ctx).Debug("GetAuGetAuthCloneObjectth - Start")

	authObjJson, authObjJsonErr := json.Marshal(authObj)
	if authObjJsonErr != nil {
		err = errors.New(fmt.Sprint("error while cloning authObj (marshal)"))
		logs.WithContext(ctx).Error(err.Error())
		logs.WithContext(ctx).Error(authObjJsonErr.Error())
		return
	}
	authObjJson = s.ReplaceVariables(ctx, projectId, authObjJson, nil)

	iCloneI := reflect.New(reflect.TypeOf(authObj))
	authObjCloneErr := json.Unmarshal(authObjJson, iCloneI.Interface())
	if authObjCloneErr != nil {
		err = errors.New(fmt.Sprint("error while cloning authObj(unmarshal)"))
		logs.WithContext(ctx).Error(err.Error())
		logs.WithContext(ctx).Error(authObjCloneErr.Error())
		return
	}
	return iCloneI.Elem().Interface().(auth.AuthI), nil
}

func (ms *ModuleStore) GetAuth(ctx context.Context, projectId string, authName string, s ModuleStoreI) (auth.AuthI, error) {
	logs.WithContext(ctx).Debug("GetAuth - Start")
	return ms.GetAuthClone(ctx, projectId, authName, s)

	/*
		if prg, ok := ms.Projects[projectId]; ok {
			if prg.Auth != nil {
				for k, v := range prg.Auth {
					if k == authName {

						return v, nil
					}
				}
			} else {
				err := errors.New(fmt.Sprint("No Auth Defined for the project : ", projectId))
				logs.WithContext(ctx).Info(err.Error())
				return nil, err
			}
		} else {
			err := errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
			logs.WithContext(ctx).Info(err.Error())
			return nil, err
		}
		err := errors.New(fmt.Sprint("Auth ", authName, " not found"))
		logs.WithContext(ctx).Info(err.Error())
		return nil, err
	*/
}

func (ms *ModuleStore) SavePkceEvent(ctx context.Context, msParams auth.OAuthParams, s ModuleStoreI) (err error) {
	logs.WithContext(ctx).Debug("SavePkceEvent - Start")
	s.GetMutex().Lock()
	defer s.GetMutex().Unlock()
	var queries []store.Queries
	query := store.Queries{}
	query.Query = INSERT_PKCE_EVENT
	var vals []interface{}
	vals = append(vals, uuid.New().String(), msParams.CodeVerifier, msParams.CodeChallenge, msParams.ClientRequestId, msParams.Nonce, msParams.Url)
	query.Vals = vals
	queries = append(queries, query)

	logs.WithContext(ctx).Info(fmt.Sprint(vals))

	output, err := s.ExecuteDbSave(ctx, queries)
	logs.WithContext(ctx).Info(fmt.Sprint(output))
	if err != nil {
		logs.WithContext(ctx).Info(err.Error())
	}
	return
}

func (ms *ModuleStore) GetPkceEvent(ctx context.Context, requestId string, s ModuleStoreI) (msParams auth.OAuthParams, err error) {
	logs.WithContext(ctx).Debug("GetPkceEvent - Start")
	query := store.Queries{}
	query.Query = SELECT_PKCE_EVENT
	var vals []interface{}
	vals = append(vals, requestId)
	query.Vals = vals
	output, err := s.ExecuteDbFetch(ctx, query)
	if len(output) > 0 {
		msParams.CodeVerifier = output[0]["code_verifier"].(string)
		msParams.CodeChallenge = output[0]["code_challenge"].(string)
		msParams.ClientRequestId = output[0]["request_id"].(string)
		msParams.Nonce = output[0]["nonce"].(string)
		msParams.Url = output[0]["url"].(string)
	}
	if err != nil {
		logs.WithContext(ctx).Info(err.Error())
	}
	return
}

func GetAuthDb(dbType string) auth.AuthDbI {
	switch strings.ToUpper(dbType) {
	case "POSTGRES":
		return new(auth.AuthDbPostgres)
	case "MYSQL":
		return new(auth.AuthDbMysql)
	default:
		return new(auth.AuthDb)
	}
}

func (ms *ModuleStore) SaveProjectSettings(ctx context.Context, projectId string, projectSettings module_model.ProjectSettings, realStore ModuleStoreI) error {
	logs.WithContext(ctx).Debug("SaveProjectConfig - Start")
	realStore.GetMutex().Lock()
	defer realStore.GetMutex().Unlock()
	err := ms.checkProjectExists(ctx, projectId)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	ms.Projects[projectId].ProjectSettings = projectSettings
	logs.WithContext(ctx).Info("SaveStore called from SaveProjectSettings")
	return realStore.SaveStore(ctx, projectId, "", realStore)
}
