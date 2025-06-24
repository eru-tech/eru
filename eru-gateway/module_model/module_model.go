package module_model

import (
	"context"
	"fmt"
	"net/http"

	"github.com/eru-tech/eru/eru-crypto/jwt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-store/store"
	utils "github.com/eru-tech/eru/eru-utils"
)

type StoreCompare struct {
	store.StoreCompare
	DeleteListenerRules   []string               `json:"delete_listener_rules"`
	NewListenerRules      []string               `json:"new_listener_rules"`
	MismatchListenerRules map[string]interface{} `json:"mismatch_listener_rules"`
	DeleteAuthorizer      []string               `json:"delete_authorizer"`
	NewAuthorizer         []string               `json:"new_authorizer"`
	MismatchAuthorizer    map[string]interface{} `json:"mismatch_authorizer"`
	MismatchSettings      map[string]interface{} `json:"mismatch_settings"`
}

type ModuleProjectI interface {
}

type Authorizer struct {
	AuthorizerName string   `json:"authorizer_name"`
	TokenHeaderKey string   `json:"token_header_key"`
	KidHeaderKey   string   `json:"kid_header_key"`
	SecretAlgo     string   `json:"secret_algo"`
	JwkUrl         string   `json:"jwk_url"`
	TokenUrl       string   `json:"token_url"`
	TokenUrlKey    string   `json:"token_url_key"`
	TokenKey       string   `json:"token_key"`
	TokenJwkUrl    string   `json:"token_jwk_url"`
	Audience       []string `json:"audience"`
	Issuer         []string `json:"issuer"`
	AccessTokenUrl string   `json:"access_token_url"`
	IdTokenKey     string   `json:"id_token_key"`
}

type ListenerRule struct {
	RuleRank              int64             `json:"rule_rank" eru:"required"`
	RuleName              string            `json:"rule_name" eru:"required"`
	Hosts                 []string          `json:"hosts"`
	Paths                 []PathStruct      `json:"paths"`
	Headers               []MapStruct       `json:"headers"`
	AddHeaders            []MapStructCustom `json:"add_headers"`
	Params                []MapStruct       `json:"params"`
	Methods               []string          `json:"methods"`
	SourceIP              []string          `json:"source_ip"`
	TargetHosts           []TargetHost      `json:"target_hosts" eru:"required"`
	AuthorizerName        string            `json:"authorizer_name"`
	AuthorizerException   []PathStruct      `json:"authorizer_exception"`
	AuthorizerExceptionIP []string          `json:"authorizer_exception_ip"`
}

type MapStruct struct {
	Key   string `json:"key" eru:"required"`
	Value string `json:"value" eru:"required"`
}

type MapStructCustom struct {
	MapStruct
	IsTemplate bool `json:"is_template" eru:"required"`
}

type PathStruct struct {
	MatchType string `json:"match_type" eru:"required"`
	Path      string `json:"path" eru:"required"`
}

type TargetHost struct {
	//Name       string `json:"name"`
	Host       string `json:"host" eru:"required"`
	Port       string `json:"port"`
	Method     string `json:"method"`
	Scheme     string `json:"scheme" eru:"required"`
	Allocation int64  `json:"allocation"`
}
type ProjectSettings struct {
	ClaimsKey string `json:"claims_key" eru:"required"`
}

func (authorizer Authorizer) VerifyAccessToken(ctx context.Context, accessToken string) (valid bool) {
	valid = false
	headers := http.Header{}
	headers.Set("Content-Type", "application/x-www-form-urlencoded")
	postBody := make(map[string]string)
	postBody["token"] = accessToken
	introspectRes, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, authorizer.AccessTokenUrl, headers, postBody, nil, nil, nil)
	if err != nil {
		_ = logs.Err(ctx, err, "")
		return
	}
	logs.WithContext(ctx).Info(fmt.Sprint(introspectRes))
	if resMap, resMapOk := introspectRes.(map[string]interface{}); resMapOk {
		if validI, validIOk := resMap["active"]; validIOk {
			if validBool, validBoolOk := validI.(bool); validBoolOk {
				valid = validBool
			}
		}
		return valid
	}
	return
}

func (authorizer Authorizer) VerifyToken(ctx context.Context, token string, kid string) (claims interface{}, err error) {
	jwkUrl := authorizer.JwkUrl
	if kid != "" {
		jwkUrl = fmt.Sprint(jwkUrl, "/", kid)
	}
	logs.WithContext(ctx).Info(jwkUrl)
	claims, err = jwt.DecryptTokenJWK(ctx, token, jwkUrl)
	if err != nil {
		return
	}
	logs.WithContext(ctx).Info(authorizer.TokenUrl)
	if authorizer.TokenUrl != "" {
		headers := http.Header{}
		headers.Set("Content-Type", "application/json")
		postBody := make(map[string]string)
		postBody[authorizer.TokenUrlKey] = token
		hookRes, _, _, _, hookErr := utils.CallHttp(ctx, http.MethodPost, authorizer.TokenUrl, headers, nil, nil, nil, postBody)
		if hookErr != nil {
			err = logs.Err(ctx, hookErr, "")
			return
		}
		if hookResMap, hookResMapOk := hookRes.(map[string]interface{}); hookResMapOk {
			tokenJwkUrl := authorizer.TokenJwkUrl
			if claimsToken, claimsTokenOk := hookResMap[authorizer.TokenKey].(string); claimsTokenOk {
				claims, err = jwt.DecryptTokenJWK(ctx, claimsToken, tokenJwkUrl)
				if err != nil {
					return
				}
			} else {
				logs.WithContext(ctx).Warn("claimsToken is not a string")
			}
		} else {
			logs.WithContext(ctx).Warn("hookRes is not a map")
		}
	}
	return
}
