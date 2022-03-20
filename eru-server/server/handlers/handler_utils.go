package server

import (
	"bytes"
	"github.com/rs/cors"
	"github.com/segmentio/ksuid"
	"io/ioutil"
	"log"
	"net/http"
)

func FormatResponse(w http.ResponseWriter, status int) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
}

func LogRequestResponse(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		//requestID := r.Header.Get(helpers.HeaderRequestID)
		requestID := ""
		if requestID == "" {
			// set a new request id header of request
			requestID = ksuid.New().String()
			//r.Header.Set(helpers.HeaderRequestID, requestID)
			r.Header.Set("requestID", requestID)
		}

		var reqBody []byte
		if r.Header.Get("Content-Type") == "application/json" {
			reqBody, _ = ioutil.ReadAll(r.Body)
			r.Body = ioutil.NopCloser(bytes.NewBuffer(reqBody))
		}

		//helpers.Logger.LogInfo(requestID, "Request", map[string]interface{}{"method": r.Method, "url": r.URL.Path, "queryVars": r.URL.Query(), "body": string(reqBody)})
		log.Print(requestID, "Request", map[string]interface{}{"method": r.Method, "url": r.URL.Path, "queryVars": r.URL.Query(), "body": string(reqBody)})
		next.ServeHTTP(w, r.WithContext(nil))

	})
}

// makeCorsObject takes required config and make a new cors object
func MakeCorsObject() *cors.Cors {
	return cors.New(cors.Options{
		AllowCredentials: true,
		AllowOriginFunc: func(s string) bool {
			return true
		},
		//AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "PUT", "POST", "DELETE"},
		AllowedHeaders: []string{"Authorization", "Content-Type"},
		//ExposedHeaders: []string{"Authorization", "Content-Type"},
	})
}
