package module_model

import (
	"context"
	"github.com/eru-tech/eru/eru-crypto/jwt"
)

type StoreCompare struct {
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
	SecretAlgo     string   `json:"secret_algo"`
	JwkUrl         string   `json:"jwk_url"`
	Audience       []string `json:"audience"`
	Issuer         []string `json:"issuer"`
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

func (authorizer Authorizer) VerifyToken(ctx context.Context, token string) (claims interface{}, err error) {
	claims, err = jwt.DecryptTokenJWK(ctx, token, authorizer.JwkUrl)
	if err != nil {
		return
	}
	return
}
