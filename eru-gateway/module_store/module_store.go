package module_store

import (
	"errors"
	"fmt"
	"github.com/eru-tech/eru/eru-gateway/module_model"
	"github.com/eru-tech/eru/eru-store/store"
	"log"
	"net/http"
	"strings"
)

type StoreHolder struct {
	Store ModuleStoreI
}
type ModuleStoreI interface {
	store.StoreI
	SaveListenerRule(listenerRule *module_model.ListenerRule, realStore ModuleStoreI, persist bool) error
	ReplaceListenerRule(listenerRule *module_model.ListenerRule) error
	RemoveListenerRule(listenerRuleName string, realStore ModuleStoreI) error
	GetListenerRules() []*module_model.ListenerRule
	GetListenerRule(listenerRuleName string) (*module_model.ListenerRule, error)
	GetTargetGroupAuthorizer(r *http.Request) (module_model.TargetHost, module_model.Authorizer, []module_model.MapStructCustom, error)
	SaveAuthorizer(authorizer module_model.Authorizer, realStore ModuleStoreI, persist bool) error
	RemoveAuthorizer(authorizerName string, realStore ModuleStoreI) error
	GetAuthorizer(authorizerName string) (module_model.Authorizer, error)
	GetAuthorizers() map[string]module_model.Authorizer
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

func (ms *ModuleStore) GetTargetGroupAuthorizer(r *http.Request) (module_model.TargetHost, module_model.Authorizer, []module_model.MapStructCustom, error) {
	listenerRuleFound := false
	if ms.ListenerRules != nil {
		for _, v := range ms.ListenerRules {
			//TODO to sort the array on RuleRank before looping

			//check for hosts
			for _, host := range v.Hosts {
				log.Println(r.Host)
				if strings.Split(r.Host, ":")[0] == host {
					log.Println(fmt.Sprint("host match = ", host))
					listenerRuleFound = true
					break
				}
			}

			//check for Methods
			for _, method := range v.Methods {
				//resetting listenerRuleFound to false as Method array length > 1 - so it has to pass this match too
				listenerRuleFound = false
				if r.Method == method {
					log.Println(fmt.Sprint("method match = ", method))
					listenerRuleFound = true
					break
				}
			}

			//check for Paths
			for _, path := range v.Paths {
				//resetting listenerRuleFound to false as Path array length > 1 - so it has to pass this match too
				listenerRuleFound = false
				log.Println(path.Path)
				log.Println(r.URL.Path)
				switch path.MatchType {
				case MatchTypePrefix:
					if strings.HasPrefix(r.URL.Path, path.Path) {
						log.Println(fmt.Sprint("path match = ", path.Path))
						listenerRuleFound = true
						break
					}
				case MatchTypeExact:
					if r.URL.Path == path.Path {
						log.Println(fmt.Sprint("path match = ", path.Path))
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
					log.Println(fmt.Sprint("header match = ", header.Key, " = ", header.Value))
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
					log.Println(fmt.Sprint("param match = ", param.Key, " = ", param.Value))
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
					log.Println(fmt.Sprint("sourceIP match = ", sourceIP))
					listenerRuleFound = true
					break
				}
			}
			log.Println(listenerRuleFound)
			if listenerRuleFound {
				pathExceptionFound := false
				for _, pathException := range v.AuthorizerException {
					switch pathException.MatchType {
					case MatchTypePrefix:
						if strings.HasPrefix(r.URL.Path, pathException.Path) {
							log.Println(fmt.Sprint("pathException match = ", pathException.Path))
							pathExceptionFound = true
							break
						}
					case MatchTypeExact:
						if r.URL.Path == pathException.Path {
							log.Println(fmt.Sprint("pathException match = ", pathException.Path))
							pathExceptionFound = true
							break
						}
					default:
						//do nothing
					}
				}
				if pathExceptionFound || v.AuthorizerName == "" {
					return v.TargetHosts[0], module_model.Authorizer{}, v.AddHeaders, nil
				} else {
					authorizer, err := ms.GetAuthorizer(v.AuthorizerName)
					if err != nil {
						return module_model.TargetHost{}, module_model.Authorizer{}, nil, err
					}
					return v.TargetHosts[0], authorizer, v.AddHeaders, nil
				}
			}
		}
	}
	return module_model.TargetHost{}, module_model.Authorizer{}, nil, errors.New(fmt.Sprint("Listener Rule not found for this request"))
}

func (ms *ModuleStore) GetListenerRule(listenerRuleName string) (*module_model.ListenerRule, error) {
	if ms.ListenerRules != nil {
		for _, v := range ms.ListenerRules {
			if v.RuleName == listenerRuleName {
				return v, nil
			}
		}
	}
	return nil, errors.New(fmt.Sprint("Listener Rule ", listenerRuleName, " not found"))
}

func (ms *ModuleStore) ReplaceListenerRule(listenerRule *module_model.ListenerRule) error {
	if ms.ListenerRules != nil {
		for i, v := range ms.ListenerRules {
			if v.RuleName == listenerRule.RuleName {
				ms.ListenerRules[i] = listenerRule
				return nil
			}
		}
	}
	return errors.New(fmt.Sprint("Listener Rule ", listenerRule.RuleName, " not found"))
}

func (ms *ModuleStore) SaveListenerRule(listenerRule *module_model.ListenerRule, realStore ModuleStoreI, persist bool) error {
	//TODO to check for duplicate rank
	err := ms.ReplaceListenerRule(listenerRule)
	if err != nil {
		ms.ListenerRules = append(ms.ListenerRules, listenerRule)
	}
	if persist == true {
		log.Print("SaveStore called from SaveListenerRule")
		return realStore.SaveStore("", realStore)
	} else {
		return nil
	}
}
func (ms *ModuleStore) RemoveListenerRule(listenerRuleName string, realStore ModuleStoreI) error {
	if ms.ListenerRules != nil {
		for i, v := range ms.ListenerRules {
			if v.RuleName == listenerRuleName {
				ms.ListenerRules = append(ms.ListenerRules[:i], ms.ListenerRules[i+1:]...)
				return realStore.SaveStore("", realStore)
			}
		}
	}
	return errors.New(fmt.Sprint("Listener Rule ", listenerRuleName, " not found"))
}

func (ms *ModuleStore) GetListenerRules() []*module_model.ListenerRule {
	return ms.ListenerRules
}

func (ms *ModuleStore) SaveAuthorizer(authorizer module_model.Authorizer, realStore ModuleStoreI, persist bool) error {
	if ms.Authorizers == nil {
		ms.Authorizers = make(map[string]module_model.Authorizer)
	}
	ms.Authorizers[authorizer.AuthorizerName] = authorizer
	if persist == true {
		log.Print("SaveStore called from SaveAuthorizer")
		return realStore.SaveStore("", realStore)
	} else {
		return nil
	}

}

func (ms *ModuleStore) RemoveAuthorizer(authorizerName string, realStore ModuleStoreI) error {
	if _, authOk := ms.Authorizers[authorizerName]; authOk {
		delete(ms.Authorizers, authorizerName)
		return realStore.SaveStore("", realStore)
	} else {
		return errors.New(fmt.Sprint("Authorizer ", authorizerName, " not found"))
	}

}
func (ms *ModuleStore) GetAuthorizer(authorizerName string) (module_model.Authorizer, error) {
	if _, authOk := ms.Authorizers[authorizerName]; authOk {
		return ms.Authorizers[authorizerName], nil
	} else {
		return module_model.Authorizer{}, errors.New(fmt.Sprint("Authorizer ", authorizerName, " not found"))
	}

}

func (ms *ModuleStore) GetAuthorizers() map[string]module_model.Authorizer {
	return ms.Authorizers
}
