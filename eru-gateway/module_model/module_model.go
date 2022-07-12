package module_model

import "github.com/eru-tech/eru/eru-crypto/jwt"

type ModuleProjectI interface {
}

type Authorizer struct {
	AuthorizerName string
	TokenHeaderKey string
	SecretAlgo     string
	JwkUrl         string
	Audience       []string
	Issuer         []string
}

type ListenerRule struct {
	RuleRank            int64  `eru:"required"`
	RuleName            string `eru:"required"`
	Hosts               []string
	Paths               []PathStruct
	Headers             []MapStruct
	AddHeaders          []MapStructCustom
	Params              []MapStruct
	Methods             []string
	SourceIP            []string
	TargetHosts         []TargetHost `eru:"required"`
	AuthorizerName      string
	AuthorizerException []PathStruct
}

type MapStruct struct {
	Key   string `eru:"required"`
	Value string `eru:"required"`
}

type MapStructCustom struct {
	MapStruct
	IsTemplate bool
}

type PathStruct struct {
	MatchType string `eru:"required"`
	Path      string `eru:"required"`
}

type TargetHost struct {
	Host       string `eru:"required"`
	Port       string
	Method     string
	Scheme     string `eru:"required"`
	Allocation int64
}

func (authorizer Authorizer) VerifyToken(token string) (claims interface{}, err error) {
	claims, err = jwt.DecryptTokenJWK(token, authorizer.JwkUrl)
	if err != nil {
		return
	}
	return
}
