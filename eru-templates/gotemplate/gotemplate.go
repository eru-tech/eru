package gotemplate

import (
	"bytes"
	b64 "encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	eruaes "github.com/eru-tech/eru/eru-crypto/aes"
	erumd5 "github.com/eru-tech/eru/eru-crypto/md5"
	erursa "github.com/eru-tech/eru/eru-crypto/rsa"
	erusha "github.com/eru-tech/eru/eru-crypto/sha"
	"log"
	"strconv"
	"strings"
	"time"

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
			if err != nil {
				log.Print(err)
			}
			return d, err
		},
		"unmarshalJSON": func(b []byte) (d interface{}, err error) {
			err = json.Unmarshal(b, &d)
			return
		},
		"b64Encode": func(str []byte) (string, error) {
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
		"hexEncode": func(str []byte) string {
			return hex.EncodeToString(str)
		},
		"hexDecode": func(str string) (string, error) {
			decodeBytes, err := hex.DecodeString(str)
			return string(decodeBytes), err
		},
		"len": func(j interface{}) (int, error) {
			strJ, err := json.Marshal(j)
			return len(strJ), err
		},
		"aesEncryptECB": func(pb []byte, k []byte) ([]byte, error) {
			dst, err := eruaes.EncryptECB(pb, k)
			return dst, err
		},
		"aesDecryptECB": func(eb []byte, k []byte) ([]byte, error) {
			return eruaes.DecryptECB(eb, k)
		},
		"aesEncryptCBC": func(pb []byte, k []byte, iv []byte) ([]byte, error) {
			dst, err := eruaes.EncryptCBC(pb, k, iv)
			return []byte(dst), err
		},
		"aesDecryptCBC": func(eb []byte, k []byte, iv []byte) ([]byte, error) {
			return eruaes.DecryptCBC(eb, k, iv)
		},
		"encryptRSACert": func(j []byte, pubK string) ([]byte, error) {
			return erursa.EncryptWithCert(j, pubK)
		},
		"bytesToString": func(b []byte) string {
			//str , uerr := strconv.Unquote(string(b))
			//log.Print(uerr)
			return string(b)
		},
		"stringToByte": func(s string) []byte {
			return []byte(s)
		},
		"unquote": func(s string) string {
			str, uerr := strconv.Unquote(string(s))
			log.Print(uerr)
			return str
		},
		"generateAesKey": func(bits int) ([]byte, error) {
			aesObj, err := eruaes.GenerateKey(bits)
			if err != nil {
				return nil, err
			}
			return aesObj.Key, nil
		},
		"shaHash": func(b string, bits int) (string, error) {
			switch bits {
			case 256:
				return hex.EncodeToString(erusha.NewSHA256([]byte(b))), nil
			case 512:
				return hex.EncodeToString(erusha.NewSHA512([]byte(b))), nil
			default:
				return "", errors.New(fmt.Sprint("SHA function not defined for ", bits, "bits"))
			}
		},
		"md5": func(str string, output string) (string, error) {
			return erumd5.Md5(str, output)
		},
		"PKCS7Pad": func(buf []byte, size int) []byte {
			return eruaes.Pad(buf, size)
		},
		"PKCS7Unpad": func(buf []byte) ([]byte, error) {
			return eruaes.Unpad(buf)
		},
		"saveVar": func(vars map[string]interface{}, ketToSave string, valueToSave interface{}) error {
			vars[ketToSave] = valueToSave
			log.Print("saveVar printed below")
			log.Print(vars)
			return nil
		},
		"concatMapKeyVal": func(vars map[string]interface{}, keys []string, seprator string) string {
			str := ""
			for _, k := range keys {
				str = fmt.Sprint(str, k, "=", vars[k], "|")
			}
			return str
		},
		"concatMapKeyValUnordered": func(vars map[string]interface{}, seprator string) string {
			str := ""
			for k, _ := range vars {
				str = fmt.Sprint(str, k, "=", vars[k], seprator)
			}
			return str
		},
		"makeMapKeyValUnordered": func(str string, seprator string) (vars map[string]interface{}) {
			vars = make(map[string]interface{})
			tmpStr := strings.Split(str, seprator)
			for _, v := range tmpStr {
				vSplit := strings.Split(v, "=")
				splitStr := ""
				if len(vSplit) == 2 {
					splitStr = vSplit[1]
				}
				vars[vSplit[0]] = splitStr
			}
			return vars
		},
		"overwriteMap": func(orgMap map[string]interface{}, b []byte) (d interface{}, err error) {
			newMap := make(map[string]interface{})
			err = json.Unmarshal(b, &newMap)
			for k, v := range newMap {
				orgMap[k] = v
			}
			d, err = json.Marshal(orgMap)
			return
		},
		"removeMapKey": func(orgMap map[string]interface{}, key string) (d map[string]interface{}, err error) {
			v, err := json.Marshal(orgMap)
			if err != nil {
				return orgMap, err
			}
			newMap := make(map[string]interface{})
			err = json.Unmarshal(v, &newMap)
			if err != nil {
				return orgMap, err
			}
			delete(newMap, key)
			return newMap, nil
		},
		"logobject": func(v interface{}) (err error) {
			vobj, err := json.Marshal(v)
			if err != nil {
				log.Println(err)
				return
			}
			log.Print("logobject printed below")
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
		"current_date": func() (dt string, err error) {
			dt = time.Now().Format("2006-01-02")
			return
		},
		"date_part": func(dtStr string, dtPart string) (datePart string, err error) {
			dt, err := time.Parse("2006-01-02", dtStr)
			switch dtPart {
			case "DAY":
				datePart = string(dt.Day())
			case "MONTHN":
				log.Print(int(dt.Month()))
				mStr := strconv.Itoa(int(dt.Month()))
				datePart = strings.Repeat("0", 2-len(mStr)) + mStr
			case "MONTH":
				datePart = dt.Month().String()
			case "YEAR":
				datePart = strconv.Itoa(dt.Year())
			default:
				datePart = ""
			}
			return
		},
		"concat": func(sep string, inStr ...string) (str string, err error) {
			str = strings.Join(inStr, sep)
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
		//log.Println("buf.String()")
		//log.Println("-------------------")
		//log.Println(buf.String())
		if err = json.Unmarshal([]byte(buf.String()), &output); err != nil {
			return nil, errors.New(fmt.Sprintf("Unable to marhsal templated output to JSON : ", buf.String(), " ", err))
		} else {
			return
		}
	}
	return nil, errors.New(fmt.Sprint("Unknown output format : ", outputFormat))
}
