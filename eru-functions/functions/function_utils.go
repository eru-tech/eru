package functions

import (
	"bytes"
	"context"
	b64 "encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-templates/gotemplate"
	"github.com/google/uuid"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
)

//func fetchClaimsFromToken(ctx context.Context, strToken string, jwkUrl string) (claims interface{}, err error) {
//	return erujwt.DecryptTokenJWK(ctx, strToken, jwkUrl)
//}

func createFormFileCopy(w *multipart.Writer, part *multipart.Part) (io.Writer, error) {
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, part.FormName(), part.FileName()))
	h.Set("Content-Type", part.Header.Get("Content-Type"))
	return w.CreatePart(h)
}

func createFormFile(w *multipart.Writer, contentType string, fieldName string, fileName string) (io.Writer, error) {
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, fieldName, fileName))
	h.Set("Content-Type", contentType)
	return w.CreatePart(h)
}

func loadRequestVars(ctx context.Context, vars *TemplateVars, request *http.Request, tokenHeaderKey string) (err error) {
	logs.WithContext(ctx).Debug("loadRequestVars - Start")
	vars.Headers = make(map[string]interface{})
	for k, v := range request.Header {
		vars.Headers[k] = v
	}
	tokenStr := request.Header.Get(tokenHeaderKey)
	if tokenStr != "" {
		err = json.Unmarshal([]byte(tokenStr), &vars.Token)
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
			return
		}
	}
	vars.Params = make(map[string]interface{})
	for k, v := range request.URL.Query() {
		vars.Params[k] = v
	}

	reqContentType := strings.Split(request.Header.Get("Content-type"), ";")[0]
	if reqContentType == applicationjson && request.ContentLength > 0 {

		tmplBodyFromReq := json.NewDecoder(request.Body)
		tmplBodyFromReq.DisallowUnknownFields()
		if err = tmplBodyFromReq.Decode(&vars.Body); err != nil {
			logs.WithContext(ctx).Error(fmt.Sprint("error decode request body : ", err.Error()))
			return err
		}
		body, err := json.Marshal(vars.Body)
		if err != nil {
			logs.WithContext(ctx).Error(fmt.Sprint("json.Marshal(vars.Body) error : ", err.Error()))
			return err
		}
		request.Body = io.NopCloser(bytes.NewReader(body))
		request.Header.Set("Content-Length", strconv.Itoa(len(body)))
		request.ContentLength = int64(len(body))
	}
	if vars.Vars == nil {
		vars.Vars = make(map[string]interface{})
	}
	vars.OrgBody = vars.Body
	return
}

func CloneRequest(ctx context.Context, request *http.Request) (req *http.Request, err error) {
	logs.WithContext(ctx).Debug("CloneRequest - Start")
	req = request.Clone(request.Context())

	//Only request.clone does not work - need to handle multipart request as under

	reqContentType := strings.Split(req.Header.Get("Content-type"), ";")[0]
	logs.WithContext(ctx).Info(fmt.Sprint("reqContentType = ", reqContentType))
	if reqContentType == multiPartForm {
		logs.WithContext(ctx).Info("inside multiPartForm")
		var reqBody bytes.Buffer
		var reqOldBody bytes.Buffer
		multipartWriter := multipart.NewWriter(&reqBody)
		multiPart, err1 := request.MultipartReader()
		if err1 != nil {
			logs.WithContext(ctx).Error(err1.Error())
		} else {
			for {
				part, errPart := multiPart.NextRawPart()
				if errPart == io.EOF {
					logs.WithContext(ctx).Error("inside EOF error")
					break
				}
				if part.FileName() != "" {
					logs.WithContext(ctx).Info(part.FileName())
					var tempFile *os.File
					tempFile, err = ioutil.TempFile(os.TempDir(), "spa")
					defer tempFile.Close()
					if err != nil {
						logs.WithContext(ctx).Error(fmt.Sprint("Temp file creation failed : ", err.Error()))
					}
					fileWriter, err2 := createFormFileCopy(multipartWriter, part)
					if err2 != nil {
						err = err2
						logs.WithContext(ctx).Error(err.Error())
						return
					}
					//_, err = fileWriter.Write()
					_, err = io.Copy(fileWriter, part)
					if err != nil {
						logs.WithContext(ctx).Error(err.Error())
						return
					}

				} else {
					logs.WithContext(ctx).Info(part.FormName())
					buf := new(bytes.Buffer)
					buf.ReadFrom(part)
					fieldWriter, err3 := multipartWriter.CreateFormField(part.FormName())
					if err3 != nil {
						err = err3
						logs.WithContext(ctx).Error(err.Error())
						return
					}
					_, err = fieldWriter.Write(buf.Bytes())
					if err != nil {
						logs.WithContext(ctx).Error(err.Error())
						return
					}
				}
			}
		}
		multipartWriter.Close()
		reqOldBody = reqBody
		req.Body = io.NopCloser(&reqBody)
		req.Header.Set("Content-Type", multipartWriter.FormDataContentType())
		req.Header.Set("Content-Length", strconv.Itoa(reqBody.Len()))
		req.ContentLength = int64(reqBody.Len())
		request.Body = io.NopCloser(&reqOldBody)
		request.Header.Set("Content-Type", multipartWriter.FormDataContentType())
		request.Header.Set("Content-Length", strconv.Itoa(reqOldBody.Len()))
		request.ContentLength = int64(reqOldBody.Len())

	} else if reqContentType == encodedForm {
		formData := url.Values{}
		rpfErr := request.ParseForm()
		if rpfErr != nil {
			err = rpfErr
			logs.WithContext(ctx).Info(fmt.Sprint("error from request.ParseForm() = ", err.Error()))
			return
		}
		if request.Form != nil {
			for k, v := range request.Form {
				formData.Set(k, strings.Join(v, ","))
			}
		}
		req.Body = io.NopCloser(strings.NewReader(formData.Encode()))
		req.Header.Add("Content-Length", strconv.Itoa(len(formData.Encode())))

		request.Body = io.NopCloser(strings.NewReader(formData.Encode()))
		request.Header.Add("Content-Length", strconv.Itoa(len(formData.Encode())))
	} else {
		body, err3 := io.ReadAll(req.Body)
		if err3 != nil {
			err = err3
			logs.WithContext(ctx).Error(err.Error())
			return
		}
		request.Body = io.NopCloser(bytes.NewReader(body))
		req.Body = io.NopCloser(bytes.NewReader(body))
	}

	return
}

func processTemplate(ctx context.Context, templateName string, templateString string, vars *FuncTemplateVars, outputType string, tokenHeaderKey string) (output []byte, err error) {
	logs.WithContext(ctx).Debug("processTemplate - Start")
	goTmpl := gotemplate.GoTemplate{templateName, templateString}
	outputObj, err := goTmpl.Execute(ctx, vars, outputType)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	} else {
		buffer := &bytes.Buffer{}
		encoder := json.NewEncoder(buffer)
		encoder.SetEscapeHTML(false)
		err = encoder.Encode(outputObj)
		//output, err = json.Marshal(outputObj)
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
			return nil, err
		}
		output = []byte(strings.TrimSuffix(buffer.String(), "\n"))
		logs.WithContext(ctx).Info(fmt.Sprint("output ===== ", string(output)))
		if string(output) == "null" || string(output) == `"null"` {
			logs.WithContext(ctx).Info("inside string(output) == \"null\"")
			logs.WithContext(ctx).Info(templateString)
			output = []byte("")
		}
		return
	}
}
func makeMultipart(ctx context.Context, request *http.Request, formData []Headers, fileData []FilePart, vars *TemplateVars, reqVars map[string]*TemplateVars, resVars map[string]*TemplateVars, tokenSecretKey string) (varsFormData map[string]interface{}, varsFormDataKeyArray []string, err error) {
	logs.WithContext(ctx).Debug("makeMultipart - Start")

	reqContentType := strings.Split(request.Header.Get("Content-type"), ";")[0]
	varsFormData = make(map[string]interface{})
	if reqContentType == encodedForm || reqContentType == multiPartForm {
		logs.WithContext(ctx).Info("inside makeMultipart encodedForm || multiPartForm")
		var reqBody bytes.Buffer
		multipartWriter := multipart.NewWriter(&reqBody)
		for _, fd := range formData {
			fieldWriter, errfw := multipartWriter.CreateFormField(fd.Key)
			if errfw != nil {
				err = errfw
				logs.WithContext(ctx).Error(err.Error())
				return nil, nil, err
			}
			if fd.IsTemplate {
				fvars := &FuncTemplateVars{}
				fvars.Vars = vars
				fvars.ResVars = resVars
				fvars.ReqVars = reqVars

				output, errop := processTemplate(ctx, fd.Key, fd.Value, fvars, "string", tokenSecretKey)
				if errop != nil {
					err = errop
					return
				}
				outputStr := string(output)
				if str, err := strconv.Unquote(outputStr); err == nil {
					outputStr = str
				}
				_, err = fieldWriter.Write([]byte(outputStr))

			} else {
				_, err = fieldWriter.Write([]byte(fd.Value))
			}
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return nil, nil, err
			}
			varsFormData[fd.Key] = fd.Value
			varsFormDataKeyArray = append(varsFormDataKeyArray, fd.Key)
		}
		for _, fl := range fileData {
			fvars := &FuncTemplateVars{}
			fvars.Vars = vars
			filename, errop := processTemplate(ctx, "filename", fl.FileName, fvars, "string", tokenSecretKey)
			if errop != nil {
				err = errop
				return
			}
			filenameStr := string(filename)
			if str, err := strconv.Unquote(filenameStr); err == nil {
				filenameStr = str
			}
			f2vars := &FuncTemplateVars{}
			f2vars.Vars = vars
			filevarname, errop := processTemplate(ctx, "filevarname", fl.FileVarName, f2vars, "string", tokenSecretKey)
			if errop != nil {
				err = errop
				return
			}
			filevarnameStr := string(filevarname)
			if str, err := strconv.Unquote(filevarnameStr); err == nil {
				filevarnameStr = str
			}
			f3vars := &FuncTemplateVars{}
			f3vars.Vars = vars
			filecontent, errop := processTemplate(ctx, "filecontent", fl.FileContent, f3vars, "string", tokenSecretKey)
			if errop != nil {
				err = errop
				return
			}
			filecontentStr := string(filecontent)
			str := ""
			if str, err = strconv.Unquote(filecontentStr); err == nil {
				filecontentStr = str
			}
			decodeBytes := []byte("")

			decodeBytes, err = b64.StdEncoding.DecodeString(filecontentStr)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return
			}

			var tempFile *os.File
			fn, _ := uuid.NewUUID()
			tempFile, err = ioutil.TempFile(os.TempDir(), fn.String())
			defer tempFile.Close()
			if err != nil {
				logs.WithContext(ctx).Error(fmt.Sprint("Temp file creation failed : ", err.Error()))
				return
			}
			//TODO - hard coded pdf content type to be removed
			fileWriter, err := createFormFile(multipartWriter, "application/pdf", filevarnameStr, filenameStr)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return nil, nil, err
			}
			_, err = io.Copy(fileWriter, bytes.NewBuffer(decodeBytes))
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return nil, nil, err
			}
		}
		multipartWriter.Close()
		request.Body = io.NopCloser(&reqBody)
		request.Header.Set("Content-Type", multipartWriter.FormDataContentType())
		request.Header.Set("Content-Length", strconv.Itoa(reqBody.Len()))
		request.ContentLength = int64(reqBody.Len())
	}
	return
}

func processMultipart(ctx context.Context, reqContentType string, request *http.Request, formDataRemove []string, formData map[string]interface{}, fileData []FilePart) (varsFormData map[string]interface{}, varsFormDataKeyArray []string, varsFileData []FilePart, err error) {
	logs.WithContext(ctx).Debug("processMultipart - Start")
	logs.WithContext(ctx).Info(fmt.Sprint("reqContentType = ", reqContentType))
	varsFormData = make(map[string]interface{})
	if reqContentType == encodedForm || reqContentType == multiPartForm {
		logs.WithContext(ctx).Info("inside encodedForm || multiPartForm of processMultipart")
		var reqBody bytes.Buffer
		multipartWriter := multipart.NewWriter(&reqBody)
		multiPart, mErr := request.MultipartReader()
		requestHasMultipart := true
		if mErr != nil {
			//err = mErr
			logs.WithContext(ctx).Error(fmt.Sprint("error from request.MultipartReader() : ", mErr.Error()))
			requestHasMultipart = false
		}
		logs.WithContext(ctx).Info(fmt.Sprint("requestHasMultipart = ", requestHasMultipart))
		i := 0
		if requestHasMultipart {
			for {
				i++
				removeFlag := false
				part, errPart := multiPart.NextRawPart()
				if errPart == io.EOF {
					logs.WithContext(ctx).Error("breaking becuase of eof")
					break
				}
				if formDataRemove != nil {
					for _, v := range formDataRemove {
						if part.FormName() == v {
							removeFlag = true
							break
						}
					}
				}
				if !removeFlag && part != nil {
					logs.WithContext(ctx).Debug("inside !removeFlag")
					if part.FileName() != "" {
						logs.WithContext(ctx).Info(fmt.Sprint("inside part.FileName() != \"\"", part.FileName()))
						fileWriter, err := createFormFileCopy(multipartWriter, part)
						if err != nil {
							logs.WithContext(ctx).Error(err.Error())
							return nil, nil, nil, err
						}
						buf := new(bytes.Buffer)
						_, err = buf.ReadFrom(part)
						if err != nil {
							logs.WithContext(ctx).Error(err.Error())
							return nil, nil, nil, err
						}
						_, ferr := fileWriter.Write(buf.Bytes())
						if ferr != nil {
							err = ferr
							logs.WithContext(ctx).Error(err.Error())
							return nil, nil, nil, err
						}
						//fk := fmt.Sprint("file_", i)
						formName := strings.Replace(strings.Replace(part.FormName(), "[", "", -1), "]", "", -1)
						logs.WithContext(ctx).Info(formName)
						filePart := FilePart{}
						filePart.FileName = part.FileName()
						filePart.FileVarName = formName
						filePart.FileContent = b64.StdEncoding.EncodeToString(buf.Bytes())

						varsFileData = append(varsFileData, filePart)
						//varsFormData[fk] = b64.StdEncoding.EncodeToString(buf.Bytes())
						_, err = io.Copy(fileWriter, part)
						if err != nil {
							logs.WithContext(ctx).Error(err.Error())
							return nil, nil, nil, err
						}

					} else {
						logs.WithContext(ctx).Info(fmt.Sprint("inside else of part.FileName() != \"\"", part.FormName()))
						buf := new(bytes.Buffer)
						buf.ReadFrom(part)
						fieldWriter, err := multipartWriter.CreateFormField(part.FormName())
						if err != nil {
							logs.WithContext(ctx).Error(err.Error())
							return nil, nil, nil, err
						}
						_, err = fieldWriter.Write(buf.Bytes())
						if err != nil {
							logs.WithContext(ctx).Error(err.Error())
							return nil, nil, nil, err
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
		for fk, fd := range formData {
			toIgnore := false
			for _, k := range varsFormDataKeyArray {
				if k == fk {
					toIgnore = true
					break
				}
			}
			if !toIgnore {
				fieldWriter, err := multipartWriter.CreateFormField(fk)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					return nil, nil, nil, err
				}
				_, err = fieldWriter.Write([]byte(fd.(string)))
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					return nil, nil, nil, err
				}
				varsFormData[fk] = fd
				varsFormDataKeyArray = append(varsFormDataKeyArray, fk)
			}
		}
		for _, fl := range fileData {
			filenameStr := string(fl.FileName)
			if str, err := strconv.Unquote(filenameStr); err == nil {
				filenameStr = str
			}
			filevarnameStr := string(fl.FileVarName)
			if str, err := strconv.Unquote(filevarnameStr); err == nil {
				filevarnameStr = str
			}
			filecontentStr := string(fl.FileContent)
			str := ""
			if str, err = strconv.Unquote(filecontentStr); err == nil {
				filecontentStr = str
			}
			decodeBytes := []byte("")
			decodeBytes, err = b64.StdEncoding.DecodeString(filecontentStr)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return
			}

			var tempFile *os.File
			fn, _ := uuid.NewUUID()

			tempFile, err = ioutil.TempFile(os.TempDir(), fn.String())
			defer tempFile.Close()
			if err != nil {
				logs.WithContext(ctx).Error(fmt.Sprint("Temp file creation failed : ", err.Error()))
				return
			}
			//TODO - hard coded pdf content type to be removed

			fileWriter, err := createFormFile(multipartWriter, "application/pdf", filevarnameStr, filenameStr)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return nil, nil, nil, err
			}
			_, err = io.Copy(fileWriter, bytes.NewBuffer(decodeBytes))
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return nil, nil, nil, err
			}
		}
		multipartWriter.Close()
		request.Body = io.NopCloser(&reqBody)
		request.Header.Set("Content-Type", multipartWriter.FormDataContentType())
		request.Header.Set("Content-Length", strconv.Itoa(reqBody.Len()))
		request.ContentLength = int64(reqBody.Len())
		//defer request.Body.Close()
	}
	return
}

func processParams(ctx context.Context, request *http.Request, queryParamsRemove []string, queryParams []Headers, vars *TemplateVars, reqVars map[string]*TemplateVars, resVars map[string]*TemplateVars, tokenHeaderKey string) (err error) {
	logs.WithContext(ctx).Debug("processParams - Start")
	pvars := &FuncTemplateVars{}
	pvars.Vars = vars
	pvars.ReqVars = reqVars
	pvars.ResVars = resVars
	params := request.URL.Query()
	for _, p := range queryParams {
		if p.IsTemplate {
			valueBytes, terr := processTemplate(ctx, p.Key, p.Value, pvars, "string", tokenHeaderKey)
			if terr != nil {
				err = terr
				return
			}
			valueStr, uerr := strconv.Unquote(string(valueBytes))
			if uerr != nil {
				err = uerr
				logs.WithContext(ctx).Error(err.Error())
				return
			}
			params.Set(p.Key, valueStr)
			vars.Params[p.Key] = valueStr
		} else {
			params.Set(p.Key, p.Value)
			vars.Params[p.Key] = p.Value
		}
	}

	if queryParamsRemove != nil {
		for _, v := range queryParamsRemove {
			params.Del(v)
		}
	}
	request.URL.RawQuery = params.Encode()
	return
}

func processHeaderTemplates(ctx context.Context, request *http.Request, headersToRemove []string, headers []Headers, reqVarsLoaded bool, vars *TemplateVars, tokenSecretKey string, reqVars map[string]*TemplateVars, resVars map[string]*TemplateVars) (err error) {
	logs.WithContext(ctx).Debug("processHeaderTemplates - Start")
	//TODO remove reqVarsLoaded unused parameter
	for _, h := range headers {
		if h.IsTemplate {

			//TODO check if commenting below block as an impact elsewhere as we are loading vars only once before transform request is called.
			/*
				if !reqVarsLoaded {
					err = loadRequestVars(vars, request)
					if err != nil {
						return
					}
					reqVarsLoaded = true
				}
			*/

			logs.WithContext(ctx).Info(fmt.Sprint("processTemplate called for header value ", h.Key))
			fvars := &FuncTemplateVars{}
			fvars.Vars = vars
			fvars.ResVars = resVars
			fvars.ReqVars = reqVars

			koutputStr := h.Key
			if strings.HasPrefix(h.Key, "{{") {
				koutput, err := processTemplate(ctx, "headerkey", h.Key, fvars, "string", tokenSecretKey)
				if err != nil {
					return err
				}
				koutputStr = string(koutput)
				if str, err := strconv.Unquote(koutputStr); err == nil {
					koutputStr = str
				}
			}

			output, err := processTemplate(ctx, h.Key, h.Value, fvars, "string", tokenSecretKey)
			if err != nil {
				return err
			}
			outputStr := string(output)
			if str, err := strconv.Unquote(outputStr); err == nil {
				outputStr = str
			}
			request.Header.Set(koutputStr, outputStr)
			logs.WithContext(ctx).Info(fmt.Sprint("string(output) = ", outputStr))
		}
	}
	if headersToRemove != nil {
		for _, v := range headersToRemove {
			request.Header.Del(v)
		}
	}
	return
}

func clubResponses(ctx context.Context, responses []*http.Response, trResVars []*TemplateVars, errs []error) (response *http.Response, trResVar *TemplateVars, err error) {
	logs.WithContext(ctx).Debug("clubResponses - Start")
	if len(errs) > 0 {
		logs.WithContext(ctx).Error(fmt.Sprint(errs))
	}
	if len(responses) > 0 {
		if responses[0] != nil {
			reqContentType := strings.Split(responses[0].Header.Get("Content-type"), ";")[0]
			if reqContentType != applicationjson {
				response = responses[0]
				if len(trResVars) == 1 {
					trResVar = trResVars[0]
				}
				if len(errs) == 1 {
					err = errs[0]
				}
				return
			}
		}
	}

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
	logs.WithContext(ctx).Debug(fmt.Sprint("errorFound = ", errorFound))
	if errorFound {
		logs.WithContext(ctx).Error(strings.Join(errMsg, " , "))
		return nil, trResVar, errors.New(strings.Join(errMsg, " , "))
	}

	defer func(resps []*http.Response) {
		for _, resp := range resps {
			resp.Body.Close()
		}
	}(responses)
	respHeader := http.Header{}
	newR := &http.Request{}
	if len(responses) > 0 {
		newR = responses[0].Request
		for k, v := range responses[0].Header {
			if k != "Content-Length" { //TODO is this needed? || route.LoopVariable == ""
				for _, h := range v {
					respHeader.Set(k, h)
				}
			}
		}
	}
	var rJsonArray []interface{}
	statusCode := http.StatusOK
	for _, rp := range responses {
		var rJson interface{}
		reqContentTypeCheck := strings.Split(rp.Header.Get("Content-type"), ";")[0]
		if reqContentTypeCheck == applicationjson {
			err = json.NewDecoder(rp.Body).Decode(&rJson)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return nil, trResVar, err
			}
			rJson = stripSingleElement(rJson)
		} else {
			rJson, err = io.ReadAll(rp.Body)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return nil, trResVar, err
			}
		}
		rJsonArray = append(rJsonArray, rJson)
		//this will set status code of last response which will be passed
		statusCode = rp.StatusCode
	}
	rJsonArrayBytes, eee := json.Marshal(stripSingleElement(rJsonArray))
	if eee != nil {
		return nil, trResVar, eee
	}
	respHeader.Set("Content-Length", fmt.Sprint(len(rJsonArrayBytes)))
	response = &http.Response{
		StatusCode:    statusCode,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Body:          io.NopCloser(bytes.NewBuffer(rJsonArrayBytes)),
		ContentLength: int64(len(rJsonArrayBytes)),
		Request:       newR,
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
			logs.Logger.Panic(fmt.Sprint("Recovered: %v", err))
		}
	}()

	f()
}

func errorResponse(ctx context.Context, errMsg string, request *http.Request) (response *http.Response) {
	logs.WithContext(ctx).Debug("errorResponse - Start")
	errRespHeader := http.Header{}
	errRespHeader.Set("Content-Type", "application/json")
	response = &http.Response{
		StatusCode:    http.StatusBadRequest,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Body:          io.NopCloser(bytes.NewBufferString(errMsg)),
		ContentLength: int64(len(errMsg)),
		Request:       request,
		Header:        errRespHeader,
	}
	return
}

func cloneInterface(ctx context.Context, i interface{}) (iClone interface{}, err error) {
	logs.WithContext(ctx).Debug("cloneInterface - Start")
	iBytes, err := json.Marshal(i)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return
	}
	iCloneI := reflect.New(reflect.TypeOf(i))
	err = json.Unmarshal(iBytes, iCloneI.Interface())
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return
	}
	return iCloneI.Elem().Interface(), nil
}
