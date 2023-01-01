package routes

import (
	"bytes"
	b64 "encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	erujwt "github.com/eru-tech/eru/eru-crypto/jwt"
	"github.com/eru-tech/eru/eru-templates/gotemplate"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"strconv"
	"strings"
)

func fetchClaimsFromToken(strToken string, jwkUrl string) (claims interface{}, err error) {
	return erujwt.DecryptTokenJWK(strToken, jwkUrl)
}

func createFormFileCopy(w *multipart.Writer, part *multipart.Part) (io.Writer, error) {
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, part.FormName(), part.FileName()))
	log.Println(part.Header.Get("Content-Type"))
	h.Set("Content-Type", part.Header.Get("Content-Type"))
	return w.CreatePart(h)
}

func createFormFile(w *multipart.Writer, contentType string, fieldName string, fileName string) (io.Writer, error) {
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, fieldName, fileName))
	h.Set("Content-Type", contentType)
	return w.CreatePart(h)
}

func loadRequestVars(vars *TemplateVars, request *http.Request) (err error) {
	log.Println("inside loadRequestVars")
	vars.Headers = make(map[string]interface{})
	for k, v := range request.Header {
		vars.Headers[k] = v
	}
	vars.Params = make(map[string]interface{})
	for k, v := range request.URL.Query() {
		vars.Params[k] = v
	}

	// if formData is found, no need to add body to vars
	log.Println("if formData is found, no need to add body to vars")
	log.Println(len(vars.FormData))
	if len(vars.FormData) <= 0 {
		log.Println("inside len(vars.FormData) <= 0 ")
		tmplBodyFromReq := json.NewDecoder(request.Body)
		tmplBodyFromReq.DisallowUnknownFields()
		if err = tmplBodyFromReq.Decode(&vars.Body); err != nil {
			log.Println("error decode request body")
			log.Println(err)
			err = nil
			//return err
		}
		body, err := json.Marshal(vars.Body)
		if err != nil {
			log.Println(err)
		}
		request.Header.Set("Content-Length", strconv.Itoa(len(body)))
		request.ContentLength = int64(len(body))
		request.Body = ioutil.NopCloser(bytes.NewReader(body))

	}
	vars.Vars = make(map[string]interface{})
	vars.OrgBody = vars.Body
	return
}

func processTemplate(templateName string, templateString string, vars *FuncTemplateVars, outputType string, tokenHeaderKey string, jwkUrl string) (output []byte, err error) {
	log.Println("inside processTemplate")
	if strings.Contains(templateString, "{{.token") {
		strToken := vars.Vars.Headers[tokenHeaderKey]
		log.Println("strToken = ", strToken)
		log.Println("JwkUrl = ", jwkUrl)
		vars.Vars.Token, err = fetchClaimsFromToken(strToken.(string), jwkUrl)
		if err != nil {
			return
		}
	}
	goTmpl := gotemplate.GoTemplate{templateName, templateString}
	outputObj, err := goTmpl.Execute(vars, outputType)
	log.Print(outputObj)
	if err != nil {
		log.Println(err)
		return nil, err
	} else {
		buffer := &bytes.Buffer{}
		encoder := json.NewEncoder(buffer)
		encoder.SetEscapeHTML(false)
		err = encoder.Encode(outputObj)
		//output, err = json.Marshal(outputObj)
		if err != nil {
			log.Println(err)
			return nil, err
		}
		//json encoder adds new line at the end by default - removing the same
		output = []byte(strings.TrimSuffix(buffer.String(), "\n"))
		return
	}
}
func makeMultipart(request *http.Request, formData []Headers, fileData []FilePart, vars *TemplateVars, reqVars map[string]*TemplateVars, resVars map[string]*TemplateVars, tokenSecretKey string, jwkUrl string) (varsFormData map[string]interface{}, varsFormDataKeyArray []string, err error) {
	reqContentType := strings.Split(request.Header.Get("Content-type"), ";")[0]
	log.Print("reqContentType from makeMultipart = ", reqContentType)
	log.Print("fileData")
	log.Print(fileData)
	varsFormData = make(map[string]interface{})
	if reqContentType == encodedForm || reqContentType == multiPartForm {
		log.Println("===========================")
		log.Println("inside makeMultipart encodedForm || multiPartForm")
		var reqBody bytes.Buffer
		multipartWriter := multipart.NewWriter(&reqBody)
		log.Println(formData)
		for _, fd := range formData {
			log.Println("inside loop of formData")
			fieldWriter, errfw := multipartWriter.CreateFormField(fd.Key)
			if errfw != nil {
				err = errfw
				log.Println(err)
				return nil, nil, err
			}
			if fd.IsTemplate {
				log.Println("processTemplate called for header value ", fd.Key)
				fvars := &FuncTemplateVars{}
				fvars.Vars = vars
				fvars.ResVars = resVars
				fvars.ReqVars = reqVars

				output, errop := processTemplate(fd.Key, fd.Value, fvars, "string", tokenSecretKey, jwkUrl)
				if errop != nil {
					err = errop
					log.Println(err)
					return
				}
				outputStr := string(output)
				if str, err := strconv.Unquote(outputStr); err == nil {
					log.Println("inside HasPrefix")
					outputStr = str
				}
				_, err = fieldWriter.Write([]byte(outputStr))

			} else {
				_, err = fieldWriter.Write([]byte(fd.Value))
			}
			if err != nil {
				log.Println(err)
				return nil, nil, err
			}
			varsFormData[fd.Key] = fd.Value
			varsFormDataKeyArray = append(varsFormDataKeyArray, fd.Key)
		}
		log.Println("len(fileData) = ", len(fileData))
		for _, fl := range fileData {
			log.Println("inside loop of fileData")
			fvars := &FuncTemplateVars{}
			fvars.Vars = vars
			filename, errop := processTemplate("filename", fl.FileName, fvars, "string", tokenSecretKey, jwkUrl)
			if errop != nil {
				err = errop
				log.Println(err)
				return
			}
			filenameStr := string(filename)
			if str, err := strconv.Unquote(filenameStr); err == nil {
				log.Println("inside HasPrefix")
				filenameStr = str
			}
			f2vars := &FuncTemplateVars{}
			f2vars.Vars = vars
			filevarname, errop := processTemplate("filevarname", fl.FileVarName, f2vars, "string", tokenSecretKey, jwkUrl)
			if errop != nil {
				err = errop
				log.Println(err)
				return
			}
			filevarnameStr := string(filevarname)
			if str, err := strconv.Unquote(filevarnameStr); err == nil {
				log.Println("inside HasPrefix")
				filevarnameStr = str
			}
			f3vars := &FuncTemplateVars{}
			f3vars.Vars = vars
			filecontent, errop := processTemplate("filecontent", fl.FileContent, f3vars, "string", tokenSecretKey, jwkUrl)
			if errop != nil {
				err = errop
				log.Println(err)
				return
			}
			filecontentStr := string(filecontent)
			str := ""
			if str, err = strconv.Unquote(filecontentStr); err == nil {
				log.Println("inside HasPrefix")
				filecontentStr = str
			}
			//log.Println(filecontentStr)
			decodeBytes := []byte("")
			//TODO make temp file name unique
			decodeBytes, err = b64.StdEncoding.DecodeString(filecontentStr)
			if err != nil {
				log.Println(err)
				return
			}

			var tempFile *os.File
			tempFile, err = ioutil.TempFile(os.TempDir(), "spa")
			defer tempFile.Close()
			if err != nil {
				log.Println("Temp file creation failed")
				return
			}
			log.Println("filevarnameStr = ", filevarnameStr)
			log.Println("filenameStr = ", filenameStr)
			fileWriter, err := createFormFile(multipartWriter, "application/pdf", filevarnameStr, filenameStr)
			if err != nil {
				log.Println(err)
				return nil, nil, err
			}
			//_, err = fileWriter.Write()
			_, err = io.Copy(fileWriter, bytes.NewBuffer(decodeBytes))
			if err != nil {
				log.Println(err)
				return nil, nil, err
			}
		}
		multipartWriter.Close()
		request.Body = ioutil.NopCloser(&reqBody)
		//request.Header.Set("Content-Type","application/pdf" )
		log.Println("--------------------multipartWriter.FormDataContentType()--------------------")
		log.Println(multipartWriter.FormDataContentType())
		request.Header.Set("Content-Type", multipartWriter.FormDataContentType())
		request.Header.Set("Content-Length", strconv.Itoa(reqBody.Len()))
		request.ContentLength = int64(reqBody.Len())
		log.Println("request.Header.Get(\"Content-Type\")")
		log.Println(request.Header.Get("Content-Type"))
		defer request.Body.Close()
	}
	//printRequestBody(request, "request body from makemultipart")
	return
}

func processMultipart(request *http.Request, formDataRemove []string, formData []Headers) (varsFormData map[string]interface{}, varsFormDataKeyArray []string, err error) {
	log.Print("formData printed from processMultipart")
	log.Print(formData)
	reqContentType := strings.Split(request.Header.Get("Content-type"), ";")[0]
	log.Print("reqContentType = ", reqContentType)
	varsFormData = make(map[string]interface{})
	if reqContentType == encodedForm || reqContentType == multiPartForm {
		log.Println("===========================")
		log.Println("inside encodedForm || multiPartForm")
		var reqBody bytes.Buffer
		multipartWriter := multipart.NewWriter(&reqBody)
		log.Println("|||||||||||||||||||||||||||||||||||||||")
		log.Println("multipart read here from processMultipart")
		multiPart, err := request.MultipartReader()
		requestHasMultipart := true
		if err != nil {
			log.Println(err)
			log.Print("error from request.MultipartReader()")
			requestHasMultipart = false
			//return nil, nil, err
			log.Print(request.MultipartForm.Value)
		}
		log.Print("requestHasMultipart = ", requestHasMultipart)
		i := 0
		if requestHasMultipart {
			for {
				i++
				log.Print(i)
				log.Println("inside for loop - multiPart.NextRawPart() ")
				removeFlag := false
				part, errPart := multiPart.NextRawPart()
				if errPart == io.EOF {
					log.Print("breaking becuase of eof")
					break
				}
				log.Print(errPart)
				log.Print(part)

				log.Println("formDataRemove = ", formDataRemove)
				if formDataRemove != nil {
					for _, v := range formDataRemove {
						if part.FormName() == v {
							removeFlag = true
							break
						}
					}
				}
				if !removeFlag && part != nil {
					log.Println("inside !removeFlag")
					if part.FileName() != "" {
						log.Println(part.FileName())
						log.Println(part)
						var tempFile *os.File
						tempFile, err = ioutil.TempFile(os.TempDir(), "spa")
						defer tempFile.Close()
						if err != nil {
							log.Println("Temp file creation failed")
						}
						//_, err = io.Copy(tempFile, part)
						//if err != nil {
						//	log.Println(err)
						//	return
						//}
						fileWriter, err := createFormFileCopy(multipartWriter, part)
						//fileWriter, err := multipartWriter.CreateFormFile(part.FormName(), part.FileName())
						if err != nil {
							log.Println(err)
							return nil, nil, err
						}
						//_, err = fileWriter.Write()
						_, err = io.Copy(fileWriter, part)
						if err != nil {
							log.Println(err)
							return nil, nil, err
						}
					} else {
						log.Println("inside else of part.FileName() != \"\"", part.FormName())
						buf := new(bytes.Buffer)
						buf.ReadFrom(part)
						fieldWriter, err := multipartWriter.CreateFormField(part.FormName())
						if err != nil {
							log.Println(err)
							return nil, nil, err
						}
						_, err = fieldWriter.Write(buf.Bytes())
						if err != nil {
							log.Println(err)
							return nil, nil, err
						}
						formName := strings.Replace(strings.Replace(part.FormName(), "[", "", -1), "]", "", -1)
						varsFormData[formName] = buf.String()
						varsFormDataKeyArray = append(varsFormDataKeyArray, formName)
					}
				} else {
					//break the for loop
					break
				}
			}
		}
		log.Println(" ++++++++++++++++++ formData ++++++++++++++++")
		log.Println(formData)
		for _, fd := range formData {

			fieldWriter, err := multipartWriter.CreateFormField(fd.Key)
			if err != nil {
				log.Println(err)
				return nil, nil, err
			}
			_, err = fieldWriter.Write([]byte(fd.Value))
			if err != nil {
				log.Println(err)
				return nil, nil, err
			}
			varsFormData[fd.Key] = fd.Value
			varsFormDataKeyArray = append(varsFormDataKeyArray, fd.Key)
		}
		multipartWriter.Close()
		request.Body = ioutil.NopCloser(&reqBody)
		//request.Header.Set("Content-Type","application/pdf" )
		log.Println(multipartWriter.FormDataContentType())
		log.Print("multipartWriter.Boundary() = ", multipartWriter.Boundary())
		log.Println(" ++++++++++++++++++ varsFormData ++++++++++++++++")
		log.Println(varsFormData)
		log.Println(varsFormDataKeyArray)
		request.Header.Set("Content-Type", multipartWriter.FormDataContentType())
		request.Header.Set("Content-Length", strconv.Itoa(reqBody.Len()))
		request.ContentLength = int64(reqBody.Len())
		log.Print("Content-Type = ", request.Header.Get("Content-Type"))
		log.Print("Content-Length = ", request.Header.Get("Content-Length"))
	}
	return
}

func processParams(request *http.Request, queryParamsRemove []string, queryParams []Headers, vars *TemplateVars, reqVars map[string]*TemplateVars, resVars map[string]*TemplateVars) (err error) {

	pvars := &FuncTemplateVars{}
	pvars.Vars = vars
	pvars.ReqVars = reqVars
	pvars.ResVars = resVars
	params := request.URL.Query()
	for _, p := range queryParams {
		if p.IsTemplate {
			log.Print(pvars)
			valueBytes, terr := processTemplate(p.Key, p.Value, pvars, "string", "", "")
			if terr != nil {
				err = terr
				log.Print(err)
				return
			}
			valueStr, uerr := strconv.Unquote(string(valueBytes))
			if terr != nil {
				err = uerr
				log.Print(err)
				return
			}
			log.Print("valueStr = ", valueStr)
			params.Set(p.Key, valueStr)
		} else {
			params.Set(p.Key, p.Value)
		}
	}

	if queryParamsRemove != nil {
		for _, v := range queryParamsRemove {
			params.Del(v)
		}
	}
	log.Print(params)
	request.URL.RawQuery = params.Encode()
	return
}

func processHeaderTemplates(request *http.Request, headersToRemove []string, headers []Headers, reqVarsLoaded bool, vars *TemplateVars, tokenSecretKey string, jwkUrl string, reqVars map[string]*TemplateVars, resVars map[string]*TemplateVars) (err error) {
	//TODO remove reqVarsLoaded unused parameter
	for _, h := range headers {
		if h.IsTemplate {

			//TODO check if commenting below block as an impact elsewhere as we are loading vars only once before transform request is called.
			/*
				if !reqVarsLoaded {
					err = loadRequestVars(vars, request)
					if err != nil {
						log.Println(err)
						return
					}
					reqVarsLoaded = true
				}
			*/

			log.Println("processTemplate called for header value ", h.Key)
			fvars := &FuncTemplateVars{}
			fvars.Vars = vars
			fvars.ResVars = resVars
			fvars.ReqVars = reqVars
			output, err := processTemplate(h.Key, h.Value, fvars, "string", tokenSecretKey, jwkUrl)
			if err != nil {
				log.Println(err)
				return err
			}
			outputStr := string(output)
			if str, err := strconv.Unquote(outputStr); err == nil {
				log.Println("inside HasPrefix")
				outputStr = str
			}
			request.Header.Set(h.Key, outputStr)
		}
	}
	if headersToRemove != nil {
		for _, v := range headersToRemove {
			request.Header.Del(v)
		}
	}
	return
}

func printRequestBody(request *http.Request, msg string) {
	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		log.Println(err)
	}
	log.Println(msg)
	cl, _ := strconv.Atoi(request.Header.Get("Content-Length"))
	if cl > 1000 {
		log.Println(string(body)[1:1000])
	} else {
		log.Println(string(body))
	}
	log.Println(request.Header.Get("Content-Length"))
	request.Body = ioutil.NopCloser(bytes.NewReader(body))
}

func printResponseBody(response *http.Response, msg string) {
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Println(err)
	}
	log.Println(msg)
	//log.Println(body)
	cl, _ := strconv.Atoi(response.Header.Get("Content-Length"))
	if cl > 1000 {
		log.Println(string(body)[1:1000])
	} else {
		log.Println(string(body))
	}
	log.Println(response.Header.Get("Content-Length"))
	response.Body = ioutil.NopCloser(bytes.NewReader(body))
}

func clubResponses(responses []*http.Response, trResVars []*TemplateVars, errs []error) (response *http.Response, trResVar *TemplateVars, err error) {
	//check error of first record only as it is same host
	log.Print("len(responses) = ", len(responses))
	//log.Print("len(trVars) = ", len(trVars))
	log.Print("len(errs) = ", len(errs))

	var errMsg []string
	errorFound := false
	for _, e := range errs {
		if e != nil {
			errorFound = true
			errMsg = append(errMsg, e.Error())
		} else {
			errMsg = append(errMsg, "-")
		}

	}
	//trResVar - copying all attributes of first element as it will be same except response body
	trResVar = &TemplateVars{}
	if trResVars != nil {
		if trResVars[0] != nil {
			trResVar.LoopVars = trResVars[0].LoopVars
			trResVar.Vars = trResVars[0].Vars
			trResVar.FormData = trResVars[0].FormData
			trResVar.Params = trResVars[0].Params
			trResVar.Headers = trResVars[0].Headers
			trResVar.FormDataKeyArray = trResVars[0].FormDataKeyArray
			trResVar.Token = trResVars[0].Token
			var resBody []interface{}
			for _, tr := range trResVars {
				resBody = append(resBody, tr.Body)
			}
			trResVar.Body = resBody
		}
	}

	if errorFound {
		log.Println(" httpClient.Do error ")
		log.Println(errMsg)
		return nil, trResVar, errors.New(strings.Join(errMsg, " , "))
	}

	defer func(resps []*http.Response) {
		for _, resp := range resps {
			resp.Body.Close()
		}
	}(responses)

	respHeader := http.Header{}
	for k, v := range responses[0].Header {
		// for loop, content length is calculcated below based on all responses
		if k != "Content-Length" { //TODO is this needed? || route.LoopVariable == ""
			for _, h := range v {
				respHeader.Set(k, h)
			}
		}
	}
	var rJsonArray []interface{}
	for _, rp := range responses {
		var rJson interface{}
		err = json.NewDecoder(rp.Body).Decode(&rJson)
		if err != nil {
			log.Println("================")
			log.Println(err)
			return nil, trResVar, err
		}
		rJson = stripSingleElement(rJson)
		rJsonArray = append(rJsonArray, rJson)
	}
	rJsonArrayBytes, eee := json.Marshal(rJsonArray)
	if eee != nil {
		return nil, trResVar, eee
	}
	respHeader.Set("Content-Length", fmt.Sprint(len(rJsonArrayBytes)))

	response = &http.Response{
		StatusCode:    http.StatusOK,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Body:          ioutil.NopCloser(bytes.NewBuffer(rJsonArrayBytes)),
		ContentLength: int64(len(rJsonArrayBytes)),
		Request:       responses[0].Request,
		Header:        respHeader,
	}
	return
}

func stripSingleElement(obj interface{}) interface{} {
	if objArray, ok := obj.([]interface{}); !ok {
		return obj
	} else if len(objArray) == 1 {
		return objArray[0]
	} else {
		return obj
	}
}

func protect(f func()) {
	defer func() {
		if err := recover(); err != nil {
			log.Printf("Recovered: %v", err)
		}
	}()

	f()
}

func errorResponse(errMsg string, request *http.Request) (response *http.Response) {
	errRespHeader := http.Header{}
	errRespHeader.Set("Content-Type", "application/json")
	response = &http.Response{
		StatusCode:    http.StatusBadRequest,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Body:          ioutil.NopCloser(bytes.NewBufferString(errMsg)),
		ContentLength: int64(len(errMsg)),
		Request:       request,
		Header:        errRespHeader,
	}
	return
}
