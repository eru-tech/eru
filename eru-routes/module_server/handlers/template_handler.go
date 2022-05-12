package handlers

import (
	"encoding/json"
	"fmt"
	erujwt "github.com/eru-tech/eru/eru-crypto/jwt"
	"github.com/eru-tech/eru/eru-routes/module_store"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	"github.com/eru-tech/eru/eru-templates/gotemplate"
	utils "github.com/eru-tech/eru/eru-utils"
	"log"
	"net/http"
)

type TemplateBody struct {
	Name     string
	Template string
	Object   interface{}
}

type TemplateVars struct {
	Header interface{}
	Params interface{}
	Token  interface{}
	Body   interface{}
}

func ExecuteTemplateHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Print("inside ExecuteTemplateHandler")

		tmplBodyFromReq := json.NewDecoder(r.Body)
		tmplBodyFromReq.DisallowUnknownFields()

		var tmplBody TemplateBody
		jwkurl := "https://cognito-idp.ap-south-1.amazonaws.com/ap-south-1_44nu2KbZ0/.well-known/jwks.json"
		erujwt.DecryptTokenJWK(r.Header.Get("Authorization"), jwkurl)

		if err := tmplBodyFromReq.Decode(&tmplBody); err != nil {
			log.Println(err)
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			err := utils.ValidateStruct(tmplBody, "")
			if err != nil {
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": fmt.Sprint("missing field in object : ", err.Error())})
				return
			}
		}

		goTmpl := gotemplate.GoTemplate{tmplBody.Name, tmplBody.Template}
		str, err := goTmpl.Execute(tmplBody.Object, "json")
		if err != nil {
			log.Println(err)
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(str)
		}
		return
	}
}
