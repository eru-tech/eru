package module_store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/eru-tech/eru/eru-gateway/module_model"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-secret-manager/sm"
	"github.com/eru-tech/eru/eru-store/store"
	utils "github.com/eru-tech/eru/eru-utils"
	"github.com/google/go-cmp/cmp"
	"net/http"
	"strings"
)

type StoreHolder struct {
	Store ModuleStoreI
}
type ModuleStoreI interface {
	store.StoreI
	SaveListenerRule(ctx context.Context, istenerRule *module_model.ListenerRule, realStore ModuleStoreI, persist bool) error
	ReplaceListenerRule(ctx context.Context, listenerRule *module_model.ListenerRule) error
	RemoveListenerRule(ctx context.Context, listenerRuleName string, realStore ModuleStoreI) error
	GetListenerRules(ctx context.Context) []*module_model.ListenerRule
	GetListenerRule(ctx context.Context, listenerRuleName string) (*module_model.ListenerRule, error)
	GetTargetGroupAuthorizer(ctx context.Context, r *http.Request) (module_model.TargetHost, module_model.Authorizer, []module_model.MapStructCustom, error)
	SaveAuthorizer(ctx context.Context, authorizer module_model.Authorizer, realStore ModuleStoreI, persist bool) error
	RemoveAuthorizer(ctx context.Context, authorizerName string, realStore ModuleStoreI) error
	GetAuthorizer(ctx context.Context, authorizerName string) (module_model.Authorizer, error)
	GetAuthorizers(ctx context.Context) map[string]module_model.Authorizer
	CompareModuleStore(ctx context.Context, ms ExendedModuleStore, realStore ModuleStoreI) (module_model.StoreCompare, error)
	SaveProjectSettings(ctx context.Context, projectSettings module_model.ProjectSettings, realStore ModuleStoreI, persist bool) error
	GetProjectSettings(ctx context.Context) module_model.ProjectSettings
	GetGatewayConfig(ctx context.Context) ModuleStore
	GetExtendedGatewayConfig(ctx context.Context, realStore ModuleStoreI) ExendedModuleStore
}

const MatchTypePrefix = "PREFIX"
const MatchTypeExact = "EXACT"

type ExendedModuleStore struct {
	ModuleStore
	Variables     store.Variables `json:"variables"`
	SecretManager sm.SmStoreI     `json:"secret_manager"`
}

type ModuleStore struct {
	ListenerRules   []*module_model.ListenerRule       `json:"listener_rules" eru:"required"`
	Authorizers     map[string]module_model.Authorizer `json:"authorizers"`
	ProjectSettings module_model.ProjectSettings       `json:"project_settings"`
}

type ModuleFileStore struct {
	store.FileStore
	ModuleStore
}
type ModuleDbStore struct {
	store.DbStore
	ModuleStore
}

func (ms *ModuleStore) GetTargetGroupAuthorizer(ctx context.Context, r *http.Request) (module_model.TargetHost, module_model.Authorizer, []module_model.MapStructCustom, error) {
	logs.WithContext(ctx).Debug("GetTargetGroupAuthorizer - Start")
	listenerRuleFound := false
	if ms.ListenerRules != nil {
		for _, v := range ms.ListenerRules {
			//TODO to sort the array on RuleRank before looping
			//check for hosts
			for _, host := range v.Hosts {
				if strings.Split(r.Host, ":")[0] == host {
					logs.WithContext(ctx).Info(fmt.Sprint("host match = ", host))
					listenerRuleFound = true
					break
				}
			}

			//check for Methods
			for _, method := range v.Methods {
				//resetting listenerRuleFound to false as Method array length > 1 - so it has to pass this match too
				listenerRuleFound = false
				if r.Method == method {
					logs.WithContext(ctx).Info(fmt.Sprint("method match = ", method))
					listenerRuleFound = true
					break
				}
			}

			//check for Paths
			for _, path := range v.Paths {
				//resetting listenerRuleFound to false as Path array length > 1 - so it has to pass this match too
				listenerRuleFound = false
				switch path.MatchType {
				case MatchTypePrefix:
					if strings.HasPrefix(r.URL.Path, path.Path) {
						logs.WithContext(ctx).Info(fmt.Sprint("path match = ", path.Path))
						listenerRuleFound = true
						break
					}
				case MatchTypeExact:
					if r.URL.Path == path.Path {
						logs.WithContext(ctx).Info(fmt.Sprint("path match = ", path.Path))
						listenerRuleFound = true
						break
					}
				default:
					//do nothing
				}
			}
			//check for Headers
			for _, header := range v.Headers {
				//resetting listenerRuleFound to false as Headers array length > 1 - so it has to pass this match too
				listenerRuleFound = false
				if r.Header.Get(header.Key) == header.Value {
					logs.WithContext(ctx).Info(fmt.Sprint("header match = ", header.Key, " = ", header.Value))
					listenerRuleFound = true
					break
				}
			}
			//check for Params
			for _, param := range v.Params {
				//resetting listenerRuleFound to false as Headers array length > 1 - so it has to pass this match too
				reqParams := r.URL.Query()
				listenerRuleFound = false
				if reqParams.Get(param.Key) == param.Value {
					listenerRuleFound = true
					logs.WithContext(ctx).Info(fmt.Sprint("param match = ", param.Key, " = ", param.Value))
					r.URL.RawQuery = reqParams.Encode()
					break
				}
				r.URL.RawQuery = reqParams.Encode()
			}

			//check for SourceIP
			for _, sourceIP := range v.SourceIP {
				//resetting listenerRuleFound to false as SourceIP array length > 1 - so it has to pass this match too
				listenerRuleFound = false
				if strings.Split(r.RemoteAddr, ":")[0] == sourceIP {
					logs.WithContext(ctx).Info(fmt.Sprint("sourceIP match = ", sourceIP))
					listenerRuleFound = true
					break
				}
			}
			logs.WithContext(ctx).Info(fmt.Sprint("listenerRuleFound = ", listenerRuleFound))
			if listenerRuleFound {
				pathExceptionFound := false
				for _, pathException := range v.AuthorizerException {
					switch pathException.MatchType {
					case MatchTypePrefix:
						if strings.HasPrefix(r.URL.Path, pathException.Path) {
							logs.WithContext(ctx).Info(fmt.Sprint("pathException MatchTypePrefix = ", pathException.Path))
							pathExceptionFound = true
							r.Header.Set("is_public", "true")
							break
						}
					case MatchTypeExact:
						if r.URL.Path == pathException.Path {
							logs.WithContext(ctx).Info(fmt.Sprint("pathException MatchTypeExact = ", pathException.Path))
							pathExceptionFound = true
							r.Header.Set("is_public", "true")
							break
						}
					default:
						//do nothing
					}
				}
				if pathExceptionFound || v.AuthorizerName == "" {
					return v.TargetHosts[0], module_model.Authorizer{}, v.AddHeaders, nil
				} else {
					authorizer, err := ms.GetAuthorizer(ctx, v.AuthorizerName)
					if err != nil {
						return module_model.TargetHost{}, module_model.Authorizer{}, nil, err
					}
					return v.TargetHosts[0], authorizer, v.AddHeaders, nil
				}
			}
		}
	}
	err := errors.New(fmt.Sprint("Listener Rule not found for request host = ", r.Host, " and path = ", r.URL))
	logs.WithContext(ctx).Error(err.Error())
	return module_model.TargetHost{}, module_model.Authorizer{}, nil, err
}

func (ms *ModuleStore) GetListenerRule(ctx context.Context, listenerRuleName string) (*module_model.ListenerRule, error) {
	logs.WithContext(ctx).Debug("GetListenerRule - Start")
	if ms.ListenerRules != nil {
		for _, v := range ms.ListenerRules {
			if v.RuleName == listenerRuleName {
				return v, nil
			}
		}
	}
	err := errors.New(fmt.Sprint("Listener Rule ", listenerRuleName, " not found"))
	logs.WithContext(ctx).Info(err.Error())
	return nil, err
}

func (ms *ModuleStore) ReplaceListenerRule(ctx context.Context, listenerRule *module_model.ListenerRule) error {
	logs.WithContext(ctx).Debug("ReplaceListenerRule - Start")
	if ms.ListenerRules != nil {
		for i, v := range ms.ListenerRules {
			if v.RuleName == listenerRule.RuleName {
				ms.ListenerRules[i] = listenerRule
				return nil
			}
		}
	}
	err := errors.New(fmt.Sprint("Listener Rule ", listenerRule.RuleName, " not found"))
	logs.WithContext(ctx).Info(err.Error())
	return err
}

func (ms *ModuleStore) SaveListenerRule(ctx context.Context, listenerRule *module_model.ListenerRule, realStore ModuleStoreI, persist bool) error {
	logs.WithContext(ctx).Debug("SaveListenerRule - Start")
	if persist {
		realStore.GetMutex().Lock()
		defer realStore.GetMutex().Unlock()
	}
	//TODO to check for duplicate rank
	err := ms.ReplaceListenerRule(ctx, listenerRule)
	if err != nil {
		ms.ListenerRules = append(ms.ListenerRules, listenerRule)
	}
	if persist == true {
		logs.WithContext(ctx).Info("SaveStore called from SaveListenerRule")
		return realStore.SaveStore(ctx, "gateway", "", realStore)
	} else {
		return nil
	}
}
func (ms *ModuleStore) RemoveListenerRule(ctx context.Context, listenerRuleName string, realStore ModuleStoreI) error {
	logs.WithContext(ctx).Debug("RemoveListenerRule - Start")
	realStore.GetMutex().Lock()
	defer realStore.GetMutex().Unlock()
	if ms.ListenerRules != nil {
		for i, v := range ms.ListenerRules {
			if v.RuleName == listenerRuleName {
				ms.ListenerRules = append(ms.ListenerRules[:i], ms.ListenerRules[i+1:]...)
				return realStore.SaveStore(ctx, "gateway", "", realStore)
			}
		}
	}
	err := errors.New(fmt.Sprint("Listener Rule ", listenerRuleName, " not found"))
	logs.WithContext(ctx).Info(err.Error())
	return err
}

func (ms *ModuleStore) GetListenerRules(ctx context.Context) []*module_model.ListenerRule {
	logs.WithContext(ctx).Debug("GetListenerRules - Start")
	return ms.ListenerRules
}

func (ms *ModuleStore) GetGatewayConfig(ctx context.Context) ModuleStore {
	logs.WithContext(ctx).Debug("GetGatewayConfig - Start")
	return *ms
}

func (ms *ModuleStore) GetExtendedGatewayConfig(ctx context.Context, realStore ModuleStoreI) (ems ExendedModuleStore) {
	logs.WithContext(ctx).Debug("GetExtendedGatewayConfig - Start")
	var err error
	ems.Variables, err = realStore.FetchVars(ctx, "gateway")
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
	}
	ems.SecretManager, err = realStore.FetchSm(ctx, "gateway")
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
	}
	ems.Authorizers = ms.Authorizers
	ems.ListenerRules = ms.ListenerRules
	ems.ProjectSettings = ms.ProjectSettings
	return ems
}

func (ms *ModuleStore) SaveAuthorizer(ctx context.Context, authorizer module_model.Authorizer, realStore ModuleStoreI, persist bool) error {
	logs.WithContext(ctx).Debug("SaveAuthorizer - Start")
	if persist {
		realStore.GetMutex().Lock()
		defer realStore.GetMutex().Unlock()
	}
	if ms.Authorizers == nil {
		ms.Authorizers = make(map[string]module_model.Authorizer)
	}
	ms.Authorizers[authorizer.AuthorizerName] = authorizer
	if persist == true {
		logs.WithContext(ctx).Info("SaveStore called from SaveAuthorizer")
		return realStore.SaveStore(ctx, "gateway", "", realStore)
	} else {
		return nil
	}

}

func (ms *ModuleStore) RemoveAuthorizer(ctx context.Context, authorizerName string, realStore ModuleStoreI) error {
	logs.WithContext(ctx).Debug("RemoveAuthorizer - Start")
	realStore.GetMutex().Lock()
	defer realStore.GetMutex().Unlock()
	if _, authOk := ms.Authorizers[authorizerName]; authOk {
		delete(ms.Authorizers, authorizerName)
		return realStore.SaveStore(ctx, "gateway", "", realStore)
	} else {
		return errors.New(fmt.Sprint("Authorizer ", authorizerName, " not found"))
	}

}
func (ms *ModuleStore) GetAuthorizer(ctx context.Context, authorizerName string) (module_model.Authorizer, error) {
	logs.WithContext(ctx).Debug("GetAuthorizer - Start")
	if _, authOk := ms.Authorizers[authorizerName]; authOk {
		return ms.Authorizers[authorizerName], nil
	} else {
		return module_model.Authorizer{}, errors.New(fmt.Sprint("Authorizer ", authorizerName, " not found"))
	}

}

func (ms *ModuleStore) GetAuthorizers(ctx context.Context) map[string]module_model.Authorizer {
	logs.WithContext(ctx).Debug("GetAuthorizers - Start")
	return ms.Authorizers
}

func (ms *ModuleStore) GetProjectSettings(ctx context.Context) module_model.ProjectSettings {
	logs.WithContext(ctx).Debug("GetProjectSettings - Start")
	return ms.ProjectSettings
}

func (ms *ModuleStore) CompareModuleStore(ctx context.Context, cms ExendedModuleStore, realStore ModuleStoreI) (module_model.StoreCompare, error) {
	storeCompare := module_model.StoreCompare{}
	vars, err := realStore.FetchVars(ctx, "gateway")
	if err != nil {
		logs.WithContext(ctx).Warn(fmt.Sprint("ignoring variables to compare : ", err.Error()))
	}
	storeCompare.CompareVariables(ctx, vars, cms.Variables)

	sm, smerr := realStore.FetchSm(ctx, "gateway")
	if smerr != nil {
		logs.WithContext(ctx).Warn(fmt.Sprint("ignoring secret manager to compare : ", smerr.Error()))
	}
	storeCompare.CompareSecretManager(ctx, sm, cms.SecretManager)

	var oDiffR utils.DiffReporter
	if !cmp.Equal(ms.ProjectSettings, cms.ProjectSettings, cmp.Reporter(&oDiffR)) {
		if storeCompare.MismatchSettings == nil {
			storeCompare.MismatchSettings = make(map[string]interface{})
		}
		storeCompare.MismatchSettings["settings"] = oDiffR.Output()
	}

	for _, mlr := range ms.ListenerRules {
		var diffR utils.DiffReporter
		lrFound := false
		for _, clr := range cms.ListenerRules {
			if mlr.RuleName == clr.RuleName {
				lrFound = true
				logs.Logger.Info(fmt.Sprint(mlr))
				logs.Logger.Info(fmt.Sprint(clr))
				if !cmp.Equal(*mlr, *clr, cmp.Reporter(&diffR)) {
					if storeCompare.MismatchListenerRules == nil {
						storeCompare.MismatchListenerRules = make(map[string]interface{})
					}
					storeCompare.MismatchListenerRules[mlr.RuleName] = diffR.Output()
				}
				break
			}
		}
		if !lrFound {
			storeCompare.DeleteListenerRules = append(storeCompare.DeleteListenerRules, mlr.RuleName)
		}
	}

	for _, clr := range cms.ListenerRules {
		lrFound := false
		for _, mlr := range ms.ListenerRules {
			if mlr.RuleName == clr.RuleName {
				lrFound = true
				break
			}
		}
		if !lrFound {
			storeCompare.NewListenerRules = append(storeCompare.NewListenerRules, clr.RuleName)
		}
	}

	//compare authorizer
	for _, mlr := range ms.Authorizers {
		var diffR utils.DiffReporter
		authFound := false
		for _, auth := range cms.Authorizers {
			if mlr.AuthorizerName == auth.AuthorizerName {
				authFound = true
				if !cmp.Equal(mlr, auth, cmp.Reporter(&diffR)) {
					if storeCompare.MismatchAuthorizer == nil {
						storeCompare.MismatchAuthorizer = make(map[string]interface{})
					}
					storeCompare.MismatchAuthorizer[mlr.AuthorizerName] = diffR.Output()
				}
				break
			}
		}
		if !authFound {
			storeCompare.DeleteAuthorizer = append(storeCompare.DeleteAuthorizer, mlr.AuthorizerName)
		}
	}

	for _, auth := range cms.Authorizers {
		authFound := false
		for _, mlr := range ms.Authorizers {
			if mlr.AuthorizerName == auth.AuthorizerName {
				authFound = true
				break
			}
		}
		if !authFound {
			storeCompare.NewAuthorizer = append(storeCompare.NewAuthorizer, auth.AuthorizerName)
		}
	}
	return storeCompare, nil
}

func (ms *ModuleStore) SaveProjectSettings(ctx context.Context, projectSettings module_model.ProjectSettings, realStore ModuleStoreI, persist bool) error {
	logs.WithContext(ctx).Debug("SaveProjectConfig - Start")
	if persist {
		realStore.GetMutex().Lock()
		defer realStore.GetMutex().Unlock()
	}

	ms.ProjectSettings = projectSettings
	if persist == true {
		logs.WithContext(ctx).Info("SaveStore called from SaveAuthorizer")
		return realStore.SaveStore(ctx, "gateway", "", realStore)
	} else {
		return nil
	}
}

func (eMs *ExendedModuleStore) UnmarshalJSON(b []byte) error {
	logs.Logger.Info("UnMarshal ExendedModuleStore - Start")
	ctx := context.Background()
	var ePrjMap map[string]*json.RawMessage
	err := json.Unmarshal(b, &ePrjMap)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}

	var ps module_model.ProjectSettings
	if _, ok := ePrjMap["project_settings"]; ok {
		if ePrjMap["project_settings"] != nil {
			err = json.Unmarshal(*ePrjMap["project_settings"], &ps)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return err
			}
			eMs.ProjectSettings = ps
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
			eMs.Variables = vars
		}
	}

	//ListenerRules   []*module_model.ListenerRule       `json:"listener_rules" eru:"required"`
	//Authorizers     map[string]module_model.Authorizer `json:"authorizers"`

	var ak map[string]module_model.Authorizer
	if _, ok := ePrjMap["authorizers"]; ok {
		if ePrjMap["authorizers"] != nil {
			err = json.Unmarshal(*ePrjMap["authorizers"], &ak)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return err
			}
			eMs.Authorizers = ak
		}
	}

	var lr []*module_model.ListenerRule
	if _, ok := ePrjMap["listener_rules"]; ok {
		if ePrjMap["listener_rules"] != nil {
			err = json.Unmarshal(*ePrjMap["listener_rules"], &lr)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return err
			}
			eMs.ListenerRules = lr
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
					eMs.SecretManager = smI
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

	return nil
}
