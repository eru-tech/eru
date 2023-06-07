package handlers

import (
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
	return cors.New(cors.Options{
		AllowCredentials: true,
		AllowOriginFunc: func(s string) bool {
			logs.Logger.Info(fmt.Sprint("Origin Asked = ", s))
			logs.Logger.Info(fmt.Sprint("Allowed Asked = ", AllowedOrigins))
			if AllowedOrigins != "" {
				return true
			}
			envOrigin := strings.Split(AllowedOrigins, ",")
			for _, o := range envOrigin {
				if o == s {
					return true
				}
			}
			return false
		},
		//AllowedOrigins: []string{"127.0.0.1"},
		AllowedMethods: []string{"GET", "PUT", "POST", "DELETE"},
		AllowedHeaders: []string{"Authorization", "Content-Type"},
		//ExposedHeaders: []string{"Authorization", "Content-Type"},
	})
}
