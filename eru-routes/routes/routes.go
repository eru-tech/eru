package routes

import (
	"errors"
	"fmt"
	"log"
	"strings"
)

const MatchTypePrefix = "PREFIX"
const MatchTypeExact = "EXACT"

type Route struct {
	RouteName         string `eru:"required"`
	Url               string `eru:"required"`
	finalUrl          string
	MatchType         string `eru:"required"`
	RewriteUrl        string
	TargetHosts       []TargetHost `eru:"required"`
	AllowedHosts      []string
	AllowedMethods    []string
	EnableCache       bool
	RequestHeaders    []Headers
	ResponseHeaders   []Headers
	TransformRequest  string
	TransformResponse string
}

type TargetHost struct {
	Host       string `eru:"required"`
	Port       string
	Method     string `eru:"required"`
	Allocation int64
}

type Headers struct {
	Key   string `eru:"required"`
	Value string `eru:"required"`
}

func (route *Route) MakeFinalUrl(url string) (err error) {
	switch route.MatchType {
	case MatchTypePrefix:
		route.finalUrl = route.RewriteUrl + strings.TrimPrefix(strings.Split(url, route.RouteName)[1], route.Url)
		log.Println(route.finalUrl)
	case MatchTypeExact:
		route.finalUrl = route.RewriteUrl
		log.Println(route.finalUrl)
	default:
		//do nothing
	}
	return
}

func (route *Route) Validate(host string, url string, method string) (err error) {
	safeHost := true
	safeMethod := true
	if len(route.AllowedHosts) > 0 {
		safeHost = false
		for i := 0; i < len(route.AllowedHosts); i++ {
			if route.AllowedHosts[i] == host {
				safeHost = true
				break
			}
		}
	}
	if !safeHost {
		err = errors.New("Host not allowed")
		return
	}

	if len(route.AllowedMethods) > 0 {
		safeMethod = false
		for i := 0; i < len(route.AllowedMethods); i++ {
			if route.AllowedMethods[i] == method {
				safeMethod = true
				break
			}
		}
	}
	if !safeMethod {
		err = errors.New("Method not allowed")
		return
	}

	if route.MatchType != MatchTypePrefix && route.MatchType != MatchTypeExact {
		err = errors.New(fmt.Sprint("Incorrect MatchType - needed ", MatchTypePrefix, " or ", MatchTypeExact, "."))
		return
	}

	if route.MatchType == MatchTypePrefix && !strings.HasPrefix(strings.ToUpper(strings.Split(url, route.RouteName)[1]), strings.ToUpper(route.Url)) {
		err = errors.New("URL Prefix mismatch")
		return
	}

	if route.MatchType == MatchTypeExact && !strings.EqualFold(strings.Split(url, route.RouteName)[1], route.Url) {
		err = errors.New("URL mismatch")
		return
	}
	return
}
