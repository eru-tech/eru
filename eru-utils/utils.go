package eru_utils

import (
	"bytes"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	httpurl "net/url"
	"reflect"
	"strconv"
	"strings"
)

const (
	encodedForm   = "application/x-www-form-urlencoded"
	multiPartForm = "multipart/form-data"
)

var httpClient = http.Client{
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func getAttr(obj interface{}, fieldName string) reflect.Value {
	pointToStruct := reflect.ValueOf(obj) // addressable
	curStruct := pointToStruct.Elem()
	if curStruct.Kind() != reflect.Struct {
		panic("not struct")
	}
	curField := curStruct.FieldByName(fieldName) // type: reflect.Value
	if !curField.IsValid() {
		panic("not found:" + fieldName)
	}
	return curField
}

func SetStructValue(obj interface{}, propName string, propValue interface{}) {
	v := getAttr(obj, propName)
	v.Set(reflect.ValueOf(propValue))
}

func GetSha512(s string) string {
	h := sha512.New()
	h.Write([]byte(s))
	sha := base64.URLEncoding.EncodeToString(h.Sum(nil))
	return sha
}

func ValidateStruct(s interface{}, parentKey string) error {

	if parentKey != "" {
		parentKey = parentKey + "."
	}
	f := reflect.ValueOf(s)
	if f.Type().Kind().String() == "ptr" {
		f = reflect.Indirect(reflect.ValueOf(s))
	}
	log.Println(f.Type())
	var errs []string
	//log.Println(f)
	for i := 0; i < f.NumField(); i++ {
		isError := false
		isRequired := false
		projectTags := f.Type().Field(i).Tag.Get("eru")
		if strings.Contains(projectTags, "required") {
			isRequired = true
			if f.Field(i).IsZero() {
				errs = append(errs, fmt.Sprint(parentKey, f.Type().Field(i).Name))
				isError = true
			}
		}
		if !isError {
			switch f.Field(i).Kind().String() {
			case "struct":
				e := ValidateStruct(f.Field(i).Interface(), fmt.Sprint(parentKey, f.Type().Field(i).Name))
				if e != nil {
					errs = append(errs, e.Error())
				}
			case "slice":
				ff := f.Field(i)
				if ff.Len() == 0 && isRequired {
					errs = append(errs, fmt.Sprint(parentKey, f.Type().Field(i).Name))
				} else {
					if ff.Len() > 0 {

						if ff.Index(0).Kind().String() == "struct" || ff.Index(0).Kind().String() == "slice" {
							for ii := 0; ii < ff.Len(); ii++ {
								e := ValidateStruct(ff.Index(ii).Interface(), fmt.Sprint(parentKey, f.Type().Field(i).Name, "[", ii, "]"))
								if e != nil {
									errs = append(errs, e.Error())
								}
							}
						}
					}
				}
			default:
				//do nothing
			}
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errors.New(strings.Join(errs, " , "))
}

func GetArrayPosition(s []string, value string) int {
	for p, v := range s {
		if v == value {
			return p
		}
	}
	return -1
}

func ReplaceUnderscoresWithDots(str string) string {
	return strings.Replace(strings.Replace(str, "___", ".", 1), "__", ".", 1)
}

func PrintResponseBody(response *http.Response, msg string) {
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Println(err)
	}
	log.Println(msg)
	log.Println(string(body))
	log.Println(response.Header.Get("Content-Length"))
	response.Body = ioutil.NopCloser(bytes.NewReader(body))
}

func PrintRequestBody(request *http.Request, msg string) {
	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		log.Println(err)
	}
	log.Println(msg)
	log.Println(string(body))
	log.Println(request.Header.Get("Content-Length"))
	request.Body = ioutil.NopCloser(bytes.NewReader(body))
}

func CallHttp(method string, url string, headers http.Header, formData map[string]string, reqCookies []*http.Cookie, params map[string]string, postBody interface{}) (res interface{}, respHeaders http.Header, respCookies []*http.Cookie, statusCode int, err error) {
	reqBody, err := json.Marshal(postBody)
	if err != nil {
		log.Print("error in json.Marshal(postBody)")
		log.Print(err)
		return nil, nil, nil, 0, err
	}
	req, err := http.NewRequest(method, url, bytes.NewBuffer(reqBody))
	if err != nil {
		log.Print(err)
		return
	}
	for _, v := range reqCookies {
		req.AddCookie(v)
	}
	for k, v := range headers {
		for _, vv := range v {
			req.Header.Add(k, vv)
		}
	}

	reqParams := req.URL.Query()
	for k, v := range params {
		reqParams.Add(k, v)
	}
	req.URL.RawQuery = reqParams.Encode()

	reqContentType := strings.Split(req.Header.Get("Content-type"), ";")[0]
	log.Print("reqContentType = ", reqContentType)
	if reqContentType == multiPartForm {
		log.Println("===========================")
		log.Println("inside encodedForm || multiPartForm")
		var reqBodyNew bytes.Buffer
		multipartWriter := multipart.NewWriter(&reqBodyNew)
		if err != nil {
			log.Println(err)
			return nil, nil, nil, 0, err
		}
		for fk, fd := range formData {
			fieldWriter, err := multipartWriter.CreateFormField(fk)
			if err != nil {
				log.Println(err)
				return nil, nil, nil, 0, err
			}
			_, err = fieldWriter.Write([]byte(fd))
			if err != nil {
				log.Println(err)
				return nil, nil, nil, 0, err
			}
		}
		multipartWriter.Close()
		req.Body = ioutil.NopCloser(&reqBodyNew)
		if reqContentType == multiPartForm {
			req.Header.Set("Content-Type", multipartWriter.FormDataContentType())
		}
		req.Header.Set("Content-Length", strconv.Itoa(reqBodyNew.Len()))
		req.ContentLength = int64(reqBodyNew.Len())
	}
	if reqContentType == encodedForm {
		data := httpurl.Values{}
		var reqBodyNew bytes.Buffer
		for fk, fd := range formData {
			data.Add(fk, fd)
		}
		encodedData := data.Encode()
		reqBodyNew.WriteString(encodedData)
		req.Body = ioutil.NopCloser(&reqBodyNew)
		req.Header.Set("Content-Length", strconv.Itoa(len(data.Encode())))
		req.ContentLength = int64(len(data.Encode()))
	}
	log.Println(req.Header)
	PrintRequestBody(req, "printing request body from file_utils before http call")
	resp, err := httpClient.Do(req)
	statusCode = resp.StatusCode

	if err != nil {
		log.Print("error in httpClient.Do")
		log.Print(err)
		return nil, nil, nil, 0, err
	}
	log.Println("resp from CallHttp")
	log.Println(resp.Header)
	log.Println(resp.Cookies())
	log.Println(resp.StatusCode)
	log.Println(resp.Status)
	respHeaders = resp.Header
	//respHeaders = make(map[string][]string)
	//for k, v := range resp.Header {
	//	respHeaders[k] = v
	//}
	respCookies = resp.Cookies()
	defer resp.Body.Close()
	log.Println("resp.ContentLength = ", resp.ContentLength)
	if resp.ContentLength > 0 || reqContentType == encodedForm {
		log.Println(resp.Header.Get("content-type"))
		if strings.Split(resp.Header.Get("content-type"), ";")[0] == "application/json" {
			if err = json.NewDecoder(resp.Body).Decode(&res); err != nil {
				log.Print("error in json.NewDecoder of resp.Body")
				log.Print(resp.Body)
				log.Print(err)
				return nil, nil, nil, 0, err
			}
		} else {
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Println(err)
			}
			//log.Println(body)
			resBody := make(map[string]interface{})
			resBody["body"] = string(body)
			res = resBody
		}
		//log.Println(res)
	}
	if resp.StatusCode >= 400 {
		log.Print("error in httpClient.Do - response status code >=400 ")
		statusCode = resp.StatusCode
		resBytes, bytesErr := json.Marshal(res)
		if bytesErr != nil {
			log.Print("error in json.Marshal of httpClient.Do response")
			log.Print(bytesErr)
			return nil, nil, nil, statusCode, bytesErr
		}
		err = errors.New(strings.Replace(string(resBytes), "\"", "", -1))
		return nil, nil, nil, statusCode, err
	}
	return
}

func CsvToMap(csvData [][]string) (jsonData []map[string]interface{}, err error) {
	for i, line := range csvData {
		if i > 0 {
			jsonMap := make(map[string]interface{})
			for j, field := range line {
				jsonMap[strings.Replace(csvData[0][j], " ", "_", -1)] = field
			}
			jsonData = append(jsonData, jsonMap)
		}
	}
	return
}
