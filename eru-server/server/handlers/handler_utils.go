package handlers

import (
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
		AllowedMethods:   []string{"GET", "PUT", "POST", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowOriginRequestFunc: func(r *http.Request, s string) bool {
			if AllowedOrigins == "" {
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
		//Debug: true,
		//ExposedHeaders: []string{"Authorization", "Content-Type"},
	})
}
