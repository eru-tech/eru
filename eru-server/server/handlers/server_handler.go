package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

var ServerName = "unkown"

func HelloHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/hello" {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method is not supported.", http.StatusNotFound)
		return
	}
	fmt.Fprintf(w, fmt.Sprint("Hello ", ServerName))
}

func EchoHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm((1 << 20) * 10)
	log.Println(err)
	formData := r.MultipartForm
	/*
		for k, v := range r.Header {
			log.Println(k, " = ", v)
			//w.Header()[k] = v
		}*/
	//w.WriteHeader(200)
	res := make(map[string]interface{})
	res["FormData"] = formData
	res["Host"] = r.Host
	res["Header"] = r.Header
	res["URL"] = r.URL
	tmplBodyFromReq := json.NewDecoder(r.Body)
	tmplBodyFromReq.DisallowUnknownFields()
	var tmplBody interface{}
	if err := tmplBodyFromReq.Decode(&tmplBody); err != nil {
		log.Println(err)
	}
	res["Body"] = tmplBody
	res["Method"] = r.Method
	res["MultipartForm"] = r.MultipartForm
	res["RequestURI"] = r.RequestURI
	//log.Println(res)
	FormatResponse(w, 200)
	_ = json.NewEncoder(w).Encode(res)

	/*t, err := io.Copy(w, r.Body)
	if err != nil {
		log.Println("================")
		log.Println(err)
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	log.Println(t)

	*/

}
