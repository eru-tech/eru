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
	MatchType         string `eru:"required"`
	RewriteUrl        string
	TargetHosts       []TargetHost `eru:"required"`
	AllowedHosts      []string
	AllowedMethods    []string
	EnableCache       bool
	RequestHeaders    []Headers
	QueryParams       []Headers
	ResponseHeaders   []Headers
	TransformRequest  string
	TransformResponse string
	IsPublic          bool
}

type TargetHost struct {
	Host       string `eru:"required"`
	Port       string
	Method     string `eru:"required"`
	Scheme     string `eru:"required"`
	Allocation int64
}

type Headers struct {
	Key   string `eru:"required"`
	Value string `eru:"required"`
}

func (route *Route) GetTargetSchemeHostPortPath(url string) (scheme string, host string, port string, path string, err error) {
	targetHost, err := route.getTargetHost()
	if err != nil {
		log.Println(err)
		return
	}
	scheme = targetHost.Scheme
	port = targetHost.Port
	host = targetHost.Host
	switch route.MatchType {
	case MatchTypePrefix:
		path = fmt.Sprint(route.RewriteUrl, strings.TrimPrefix(strings.Split(url, route.RouteName)[1], route.Url))
	case MatchTypeExact:
		path = route.RewriteUrl
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
	log.Println(route.AllowedMethods)
	log.Println(method)
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

	if strings.HasPrefix(strings.ToUpper(url), "/PUBLIC") && !route.IsPublic {
		err = errors.New("route is not public")
		return
	}

	if route.MatchType == MatchTypePrefix && !strings.HasPrefix(strings.ToUpper(strings.Split(url, route.RouteName)[1]), strings.ToUpper(route.Url)) {
		err = errors.New("URL Prefix mismatch")
		return
	}

	if route.MatchType == MatchTypeExact && !strings.EqualFold(strings.Split(url, route.RouteName)[1], route.Url) {
		log.Println(url, " - ", route.RouteName, " - ", route.Url)
		err = errors.New("URL mismatch")
		return
	}
	return
}

func (route *Route) getTargetHost() (targetHost TargetHost, err error) {
	//TODO Random selection of target based on allocation
	if len(route.TargetHosts) > 0 {
		return route.TargetHosts[0], err
	}
	err = errors.New(fmt.Sprint("No Target Host defined for this route :", route.RouteName))
	return
}
