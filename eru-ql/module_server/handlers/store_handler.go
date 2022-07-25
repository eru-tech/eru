package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/eru-tech/eru/eru-ql/module_model"
	"github.com/eru-tech/eru/eru-ql/module_store"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	utils "github.com/eru-tech/eru/eru-utils"
	"github.com/golang-jwt/jwt/v4"
	"github.com/gorilla/mux"
	"github.com/lestrrat-go/jwx/jwk"
	"log"
	"net/http"
	"strings"
)

func ProjectSaveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		projectID := vars["project"]
		err := s.SaveProject(projectID, s, true)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("project ", projectID, " created successfully")})
		}
	}
}

func ProjectRemoveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		projectID := vars["project"]
		err := s.RemoveProject(projectID, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("project ", projectID, " removed successfully")})
		}
	}
}

func ProjectListHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		//token, err := VerifyToken(r.Header.Values("Authorization")[0])
		//log.Print(token.Method)
		//log.Print(err)
		projectIds := s.GetProjectList()
		server_handlers.FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"projects": projectIds})
	}
}

func ProjectConfigHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		projectID := vars["project"]
		log.Print(projectID)
		project, err := s.GetProjectConfig(projectID)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"project": project})
		}
	}
}

func ProjectConfigSaveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		projectId := vars["project"]
		log.Print(projectId)

		prjConfigFromReq := json.NewDecoder(r.Body)
		prjConfigFromReq.DisallowUnknownFields()

		var projectCOnfig module_model.ProjectConfig

		if err := prjConfigFromReq.Decode(&projectCOnfig); err != nil {
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			err := utils.ValidateStruct(projectCOnfig, "")
			if err != nil {
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": fmt.Sprint("missing field in object : ", err.Error())})
				return
			}
		}

		err := s.SaveProjectConfig(projectId, projectCOnfig, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("project config config for ", projectId, " saved successfully")})
		}
	}
}

func ProjectGenerateAesKeyHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bytes := make([]byte, 32) //generate a random 32 byte key for AES-256
		_, err := rand.Read(bytes)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"key": hex.EncodeToString(bytes)})
		}
		return
	}
}

//TODO - to remove below func
func base64DecodeStripped(s string) ([]byte, error) {
	if i := len(s) % 4; i != 0 {
		s += strings.Repeat("=", 4-i)
	}
	decoded, err := base64.StdEncoding.DecodeString(s)
	return (decoded), err
}

//TODO - to remove below func
func VerifyToken(tokenString string) (*jwt.Token, error) {
	//publicKeyBinary, err := base64DecodeStripped("vBiu6A1bDW42cb3bMXqAKUTxS27XGkL42GD1pPltE4VbmEV7Egwr-8aUSSeqfenZ7W89rj80o9ar1te4cSxiO34luzVEEBw0dLbfqDV43Wl1dfilmKkggGLrQBW2SiQc3jxxcOrSqGnmAcd0AVqp_t-XZyLOyj_gw925pxdE2zzLZsUiwSywjn2s7yfhrgA6EsWiQsBBYcWNHd_7-C4QHBYSHbJIB1DNee3Lb2b5YH9JkQ_OTMYRE7XRYH4w0rPNVBzTyf_F0MZYvJfjgZOZJCzPjViGsR0NFTV9lV12v0QcZ8utAAE2m10Ix3d4FW3xfHtjzFgcYVjiyadKUyIkYQ")
	//log.Print(string(publicKeyBinary))
	//log.Print(err)
	keySet, err := jwk.Fetch(context.Background(), "https://cognito-idp.ap-south-1.amazonaws.com/ap-south-1_44nu2KbZ0/.well-known/jwks.json")
	//log.Print(keySet)
	//pubBytes := pem.EncodeToMemory(&pem.Block{
	//	Type:  "RSA PUBLIC KEY",
	//Bytes: publicKeyBinary,
	//	Bytes: []byte("vBiu6A1bDW42cb3bMXqAKUTxS27XGkL42GD1pPltE4VbmEV7Egwr-8aUSSeqfenZ7W89rj80o9ar1te4cSxiO34luzVEEBw0dLbfqDV43Wl1dfilmKkggGLrQBW2SiQc3jxxcOrSqGnmAcd0AVqp_t-XZyLOyj_gw925pxdE2zzLZsUiwSywjn2s7yfhrgA6EsWiQsBBYcWNHd_7-C4QHBYSHbJIB1DNee3Lb2b5YH9JkQ_OTMYRE7XRYH4w0rPNVBzTyf_F0MZYvJfjgZOZJCzPjViGsR0NFTV9lV12v0QcZ8utAAE2m10Ix3d4FW3xfHtjzFgcYVjiyadKUyIkYQ"),
	//})

	//log.Print(string(pubBytes))

	//key, err := jwt.ParseRSAPublicKeyFromPEM(pubBytes)
	//log.Print(key)
	if err != nil {
		log.Print("error in ParseRSAPublicKeyFromPEM")
		return nil, fmt.Errorf("validate: parse key: %w", err)
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		//Make sure that the token method conform to "SigningMethodHMAC"
		//if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
		//	return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		//}
		//return []byte(os.Getenv("ACCESS_SECRET")), nil
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		kid, ok := token.Header["kid"].(string)
		keys, ok := keySet.LookupKeyID(kid)
		if !ok {
			return nil, fmt.Errorf("key with specified kid is not present in jwks")
		}
		var publickey interface{}
		err = keys.Raw(&publickey)
		if err != nil {
			return nil, fmt.Errorf("could not parse pubkey")
		}

		return publickey, nil
	})
	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		for k, v := range claims {
			log.Print(k, " ", v)
		}
	} else {
		log.Printf("Invalid JWT Token")
		//return nil, false
	}

	return token, nil
}
