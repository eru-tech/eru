package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

func HelloHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/hello" {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method is not supported.", http.StatusNotFound)
		return
	}

	//log.Print("log testing")
	fmt.Fprintf(w, "Hello!")
}

func EchoHandler(w http.ResponseWriter, r *http.Request) {

	for k, v := range r.Header {
		log.Println(k, " = ", v)
		//w.Header()[k] = v
	}
	//w.WriteHeader(200)
	res := make(map[string]interface{})
	res["Host"] = r.Host
	res["Header"] = r.Header
	res["URL"] = r.URL
	res["Body"] = r.Body
	res["Method"] = r.Method
	res["MultipartForm"] = r.MultipartForm
	res["RequestURI"] = r.RequestURI
	log.Println(res)
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
