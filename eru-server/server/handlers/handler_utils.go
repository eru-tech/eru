package handlers

import (
	"github.com/rs/cors"
	"net/http"
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
			return true
		},
		//AllowedOrigins: []string{"127.0.0.1"},
		AllowedMethods: []string{"GET", "PUT", "POST", "DELETE"},
		AllowedHeaders: []string{"Authorization", "Content-Type"},
		//ExposedHeaders: []string{"Authorization", "Content-Type"},
	})
}
