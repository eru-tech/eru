package module_store

import (
	"context"
	"errors"
	"fmt"
	"github.com/eru-tech/eru/eru-gateway/module_model"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
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
	CompareListenerRules(ctx context.Context, lrs []module_model.ListenerRule) (module_model.StoreCompare, error)
}

const MatchTypePrefix = "PREFIX"
const MatchTypeExact = "EXACT"

type ModuleStore struct {
	ListenerRules []*module_model.ListenerRule `eru:"required"`
	Authorizers   map[string]module_model.Authorizer
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
	err := errors.New(fmt.Sprint("Listener Rule not found for this request"))
	logs.WithContext(ctx).Info(err.Error())
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
	//TODO to check for duplicate rank
	err := ms.ReplaceListenerRule(ctx, listenerRule)
	if err != nil {
		ms.ListenerRules = append(ms.ListenerRules, listenerRule)
	}
	if persist == true {
		logs.WithContext(ctx).Info("SaveStore called from SaveListenerRule")
		return realStore.SaveStore(ctx, "", realStore)
	} else {
		return nil
	}
}
func (ms *ModuleStore) RemoveListenerRule(ctx context.Context, listenerRuleName string, realStore ModuleStoreI) error {
	logs.WithContext(ctx).Debug("RemoveListenerRule - Start")
	if ms.ListenerRules != nil {
		for i, v := range ms.ListenerRules {
			if v.RuleName == listenerRuleName {
				ms.ListenerRules = append(ms.ListenerRules[:i], ms.ListenerRules[i+1:]...)
				return realStore.SaveStore(ctx, "", realStore)
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

func (ms *ModuleStore) SaveAuthorizer(ctx context.Context, authorizer module_model.Authorizer, realStore ModuleStoreI, persist bool) error {
	logs.WithContext(ctx).Debug("SaveAuthorizer - Start")
	if ms.Authorizers == nil {
		ms.Authorizers = make(map[string]module_model.Authorizer)
	}
	ms.Authorizers[authorizer.AuthorizerName] = authorizer
	if persist == true {
		logs.WithContext(ctx).Info("SaveStore called from SaveAuthorizer")
		return realStore.SaveStore(ctx, "", realStore)
	} else {
		return nil
	}

}

func (ms *ModuleStore) RemoveAuthorizer(ctx context.Context, authorizerName string, realStore ModuleStoreI) error {
	logs.WithContext(ctx).Debug("RemoveAuthorizer - Start")
	if _, authOk := ms.Authorizers[authorizerName]; authOk {
		delete(ms.Authorizers, authorizerName)
		return realStore.SaveStore(ctx, "", realStore)
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

func (ms *ModuleStore) CompareListenerRules(ctx context.Context, lrs []module_model.ListenerRule) (module_model.StoreCompare, error) {
	storeCompare := module_model.StoreCompare{}
	for _, mlr := range ms.ListenerRules {
		var diffR utils.DiffReporter
		lrFound := false
		for _, clr := range lrs {
			if mlr.RuleName == clr.RuleName {
				lrFound = true
				logs.Logger.Info(fmt.Sprint(mlr))
				logs.Logger.Info(fmt.Sprint(clr))
				if !cmp.Equal(*mlr, clr, cmp.Reporter(&diffR)) {
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

	for _, clr := range lrs {
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
	return storeCompare, nil
}
