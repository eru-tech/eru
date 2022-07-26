package gotemplate

import (
	"bytes"
	"crypto/md5"
	b64 "encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	eruaes "github.com/eru-tech/eru/eru-crypto/aes"
	erursa "github.com/eru-tech/eru/eru-crypto/rsa"
	erusha "github.com/eru-tech/eru/eru-crypto/sha"
	"log"
	"strings"

	//"strconv"
	"github.com/google/uuid"
	"text/template"
)

type GoTemplate struct {
	Name     string
	Template string
}

func (goTmpl *GoTemplate) Execute(obj interface{}, outputFormat string) (output interface{}, err error) {
	log.Println("inside Execute of GoTemplate")
	var funcs = template.FuncMap{
		"repeat": func(n int) []int {
			var res []int
			for i := 0; i < n; i++ {
				res = append(res, i+1)
			}
			return res
		},
		"inc": func(n int) int {
			return n + 1
		},
		"marshalJSON": func(j interface{}) ([]byte, error) {
			d, err := json.Marshal(j)
			return d, err
		},
		"unmarshalJSON": func(b []byte) (d interface{}, err error) {
			err = json.Unmarshal(b, &d)
			return
		},
		"b64Encode": func(str []byte) (string, error) {
			log.Print("printing str from b64Encode")
			log.Print(string(str))
			return b64.StdEncoding.EncodeToString(str), nil
		},
		"b64Decode": func(str string) (string, error) {
			decodeBytes, err := b64.StdEncoding.DecodeString(str)
			if err != nil {
				log.Println(err)
				return "", err
			}
			return string(decodeBytes), nil
		},
		"aesEncryptECB": func(pb []byte, k []byte) ([]byte, error) {
			dst, err := eruaes.EncryptECB(pb, k)
			return dst, err
		},
		"len": func(j interface{}) (int, error) {
			strJ, err := json.Marshal(j)
			return len(strJ), err

		},
		"aesDecryptECB": func(eb []byte, k []byte) ([]byte, error) {
			return eruaes.DecryptECB(eb, k)
		},
		"encryptRSACert": func(j []byte, pubK string) ([]byte, error) {
			return erursa.EncryptWithCert(j, pubK)
		},
		"bytesToString": func(b []byte) string {
			return string(b)
		},
		"stringToByte": func(s string) []byte {
			return []byte(s)
		},
		"generateAesKey": func(bits int) ([]byte, error) {
			aesObj, err := eruaes.GenerateKey(bits)
			if err != nil {
				return nil, err
			}
			return aesObj.Key, nil
		},
		"shaHash": func(b string, bits int) (string, error) {
			log.Print("string for shaHash = ", b)
			switch bits {
			case 256:
				return hex.EncodeToString(erusha.NewSHA256([]byte(b))), nil
			case 512:
				return hex.EncodeToString(erusha.NewSHA512([]byte(b))), nil
			default:
				return "", errors.New(fmt.Sprint("SHA function not defined for ", bits, "bits"))
			}
		},
		"md5": func(b string) string {
			hash := md5.Sum([]byte(b))
			log.Print(b)
			log.Print(hash)
			log.Print(hex.EncodeToString(hash[:]))
			return hex.EncodeToString(hash[:])
		},
		"saveVar": func(vars map[string]interface{}, ketToSave string, valueToSave interface{}) error {
			vars[ketToSave] = valueToSave
			return nil
		},
		"concatMapKeyVal": func(vars map[string]interface{}, keys []string, seprator string) string {
			str := ""
			for _, k := range keys {
				str = fmt.Sprint(str, k, "=", vars[k], "|")
			}
			log.Print("string from concatMapKeyVal = ", str)
			return str
		},
		"concatMapKeyValUnordered": func(vars map[string]interface{}, seprator string) string {
			str := ""
			log.Print(vars)
			for k, _ := range vars {
				log.Print("inside concatMapKeyValUnordered loop")
				str = fmt.Sprint(str, k, "=", vars[k], seprator)
			}
			return str
		},
		"overwriteMap": func(orgMap map[string]interface{}, b []byte) (d interface{}, err error) {
			log.Print("overwriteMap")
			log.Print(orgMap)
			newMap := make(map[string]interface{})
			err = json.Unmarshal(b, &newMap)
			for k, v := range newMap {
				orgMap[k] = v
			}
			log.Print(orgMap)
			d, err = json.Marshal(orgMap)
			return
		},
		"logobject": func(v interface{}) (err error) {
			vobj, err := json.Marshal(v)
			if err != nil {
				log.Println(err)
				return
			}
			log.Println(string(vobj))
			return
		},
		"logstring": func(str interface{}) (err error) {
			log.Println(str)
			return
		},
		"uuid": func() (uuidStr string, err error) {
			uuidStr = uuid.New().String()
			return
		},
		"concat": func(sep string, inStr ...string) (str string, err error) {
			log.Print("inside concat")
			log.Print(inStr)
			str = strings.Join(inStr, sep)
			log.Print(str)
			return
		},
	}

	buf := &bytes.Buffer{}
	//log.Println("goTmpl.Name = ", goTmpl.Name)
	//log.Println("goTmpl.Template = ", goTmpl.Template)

	t := template.Must(template.New(goTmpl.Name).Funcs(funcs).Parse(goTmpl.Template))
	if err := t.Execute(buf, obj); err != nil {
		return "", err
	}
	switch outputFormat {
	case "string":
		if buf.String() == "<no value>" {
			return nil, errors.New("Template returned <no value>")
		}
		return buf.String(), nil
	case "json":
		log.Println("buf.String()")
		log.Println("-------------------")
		log.Println(buf.String())
		if err = json.Unmarshal([]byte(buf.String()), &output); err != nil {
			return nil, errors.New(fmt.Sprintf("Unable to marhsal templated output to JSON : ", buf.String(), " ", err))
		} else {
			return
		}
	}
	return nil, errors.New(fmt.Sprint("Unknown output format : ", outputFormat))
}
