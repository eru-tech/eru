package handlers

import (
	"context"
	"fmt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/rs/cors"
	"net/http"
	"strings"
)

func FormatResponse(w http.ResponseWriter, status int) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
}

// makeCorsObject takes required config and make a new cors object
func MakeCorsObject() *cors.Cors {
	logs.WithContext(context.Background()).Info("Inside MakeCorsObject")
	logs.WithContext(context.Background()).Info(fmt.Sprint("Allowed Origins = ", AllowedOrigins))
	return cors.New(cors.Options{
		AllowCredentials: true,
		AllowedMethods:   []string{"GET", "PUT", "POST", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowOriginRequestFunc: func(r *http.Request, s string) bool {
			logs.WithContext(context.Background()).Info(fmt.Sprint("Origin Asked = ", s))
			logs.WithContext(context.Background()).Info(fmt.Sprint("Allowed Origins = ", AllowedOrigins))
			if AllowedOrigins == "" {
				logs.WithContext(context.Background()).Info(fmt.Sprint("inside AllowedOrigins == \"\""))
				return true
			}
			envOrigin := strings.Split(AllowedOrigins, ",")
			logs.WithContext(context.Background()).Info(fmt.Sprint(envOrigin))
			for _, o := range envOrigin {
				logs.WithContext(context.Background()).Info(fmt.Sprint("checking for ", o))
				logs.WithContext(context.Background()).Info(fmt.Sprint(o == s))
				if o == s {
					return true
				}
			}
			return false
		},
		//AllowedOrigins: []string{"127.0.0.1"},
		Debug: true,
		//ExposedHeaders: []string{"Authorization", "Content-Type"},
	})
}
