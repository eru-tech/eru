package eru_utils

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/google/go-cmp/cmp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"io"
	"mime/multipart"
	"net/http"
	httpurl "net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

const (
	encodedForm     = "application/x-www-form-urlencoded"
	multiPartForm   = "multipart/form-data"
	applicationJson = "application/json"
)

var httpClient = http.Client{
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func getAttr(ctx context.Context, obj interface{}, fieldName string) reflect.Value {
	logs.WithContext(ctx).Debug("getAttr - Start")
	pointToStruct := reflect.ValueOf(obj) // addressable
	curStruct := pointToStruct.Elem()
	if curStruct.Kind() != reflect.Struct {
		logs.WithContext(ctx).Error("not a struct")
	}
	curField := curStruct.FieldByName(fieldName) // type: reflect.Value
	if !curField.IsValid() {
		logs.WithContext(ctx).Error(fmt.Sprint(("not found:" + fieldName)))
	}
	return curField
}

func SetStructValue(ctx context.Context, obj interface{}, propName string, propValue interface{}) {
	logs.WithContext(ctx).Debug("SetStructValue - Start")
	v := getAttr(ctx, obj, propName)
	v.Set(reflect.ValueOf(propValue))
}

/*func GetSha512(s string) string {
	h := sha512.New()
	h.Write([]byte(s))
	sha := base64.URLEncoding.EncodeToString(h.Sum(nil))
	return sha
}*/

func ValidateStruct(ctx context.Context, s interface{}, parentKey string) error {
	logs.WithContext(ctx).Debug("ValidateStruct - Start")
	if parentKey != "" {
		parentKey = parentKey + "."
	}
	f := reflect.ValueOf(s)
	if f.Type().Kind().String() == "ptr" {
		f = reflect.Indirect(reflect.ValueOf(s))
	}
	var errs []string
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
				e := ValidateStruct(ctx, f.Field(i).Interface(), fmt.Sprint(parentKey, f.Type().Field(i).Name))
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
								e := ValidateStruct(ctx, ff.Index(ii).Interface(), fmt.Sprint(parentKey, f.Type().Field(i).Name, "[", ii, "]"))
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
	err := errors.New(strings.Join(errs, " , "))
	logs.WithContext(ctx).Error(err.Error())
	return err
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

func PrintResponseBody(ctx context.Context, response *http.Response, msg string) {
	logs.WithContext(ctx).Debug("PrintResponseBody - Start")
	body, err := io.ReadAll(response.Body)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
	}
	logs.WithContext(ctx).Info(msg)
	logs.WithContext(ctx).Info(fmt.Sprint(len(string(body))))
	cl, _ := strconv.Atoi(response.Header.Get("Content-Length"))
	if cl > 1000 {
		logs.WithContext(ctx).Info(string(body)[1:1000])
	} else if len(string(body)) > 1000 {
		logs.WithContext(ctx).Info(string(body)[1:1000])
	} else {
		logs.WithContext(ctx).Info(string(body))
	}
	response.Body = io.NopCloser(bytes.NewReader(body))
}

func PrintRequestBody(ctx context.Context, request *http.Request, msg string) {
	logs.WithContext(ctx).Debug("PrintRequestBody - Start")
	body, err := io.ReadAll(request.Body)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
	}
	logs.WithContext(ctx).Info(msg)
	logs.WithContext(ctx).Info(fmt.Sprint(len(string(body))))
	cl, _ := strconv.Atoi(request.Header.Get("Content-Length"))
	if cl > 1000 && len(string(body)) > 1000 {
		logs.WithContext(ctx).Info(string(body)[1:1000])
	} else {
		logs.WithContext(ctx).Info(string(body))
	}
	request.Body = io.NopCloser(bytes.NewReader(body))
}

func CallParallelHttp(ctx context.Context, method string, url string, headers http.Header, formData map[string]string, reqCookies []*http.Cookie, params map[string]string, postBody interface{}, rc chan *http.Response) (err error) {
	logs.WithContext(ctx).Debug("CallParallelHttp - Start")
	resp, err := callHttp(ctx, method, url, headers, formData, reqCookies, params, postBody)
	if err == nil {
		rc <- resp
	}
	return err
}

func ExecuteParallelHttp(ctx context.Context, req *http.Request, rc chan *http.Response) (err error) {
	logs.WithContext(ctx).Debug("ExecuteParallelHttp - Start")
	resp, err := ExecuteHttp(ctx, req)
	if err == nil {
		rc <- resp
	}
	return err
}

func ExecuteHttp(ctx context.Context, req *http.Request) (resp *http.Response, err error) {
	logs.WithContext(ctx).Debug("ExecuteHttp - Start")
	//req = req.WithContext(ctx)
	//resp, err = httpClient.Do(req)
	resp, err = HTTPClientTransporter(http.DefaultTransport).RoundTrip(req)
	return
}

func HTTPClientTransporter(rt http.RoundTripper) http.RoundTripper {
	return otelhttp.NewTransport(rt)
}

func callHttp(ctx context.Context, method string, url string, headers http.Header, formData map[string]string, reqCookies []*http.Cookie, params map[string]string, postBody interface{}) (resp *http.Response, err error) {
	logs.WithContext(ctx).Debug("callHttp - Start")
	reqBody, err := json.Marshal(postBody)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
	req, err := http.NewRequest(method, url, bytes.NewBuffer(reqBody))
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
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
	if reqContentType == multiPartForm {
		var reqBodyNew bytes.Buffer
		multipartWriter := multipart.NewWriter(&reqBodyNew)
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
			return nil, err
		}
		for fk, fd := range formData {
			fieldWriter, err := multipartWriter.CreateFormField(fk)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return nil, err
			}
			_, err = fieldWriter.Write([]byte(fd))
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return nil, err
			}
		}
		multipartWriter.Close()
		req.Body = io.NopCloser(&reqBodyNew)
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
		req.Body = io.NopCloser(&reqBodyNew)
		req.Header.Set("Content-Length", strconv.Itoa(len(data.Encode())))
		req.ContentLength = int64(len(data.Encode()))
	}
	return ExecuteHttp(ctx, req)
}

func CallHttp(ctx context.Context, method string, url string, headers http.Header, formData map[string]string, reqCookies []*http.Cookie, params map[string]string, postBody interface{}) (res interface{}, respHeaders http.Header, respCookies []*http.Cookie, statusCode int, err error) {
	logs.WithContext(ctx).Debug("CallHttp - Start")
	resp, err := callHttp(ctx, method, url, headers, formData, reqCookies, params, postBody)
	statusCode = resp.StatusCode
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, resp.Header, resp.Cookies(), resp.StatusCode, err
	}

	respHeaders = resp.Header
	respCookies = resp.Cookies()
	defer resp.Body.Close()

	//todo - check if below change from reqContentType to header.get breaks anything
	//todo - merge conflict - main had below first if commented
	contentType := strings.Split(headers.Get("Content-type"), ";")[0]
	if resp.ContentLength > 0 || contentType == encodedForm || contentType == applicationJson {
		if strings.Split(resp.Header.Get("content-type"), ";")[0] == applicationJson {
			if err = json.NewDecoder(resp.Body).Decode(&res); err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return nil, nil, nil, resp.StatusCode, err
			}
		} else {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
			}
			resBody := make(map[string]interface{})
			resBody["body"] = string(body)
			res = resBody
		}
	} else {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
		}
		resBody := make(map[string]interface{})
		resBody["body"] = string(body)
		res = resBody
	}
	if resp.StatusCode >= 400 {
		statusCode = resp.StatusCode
		resBytes, bytesErr := json.Marshal(res)
		if bytesErr != nil {
			logs.WithContext(ctx).Error(bytesErr.Error())
			return nil, nil, nil, statusCode, bytesErr
		}
		err = errors.New(string(resBytes))
		logs.WithContext(ctx).Error(err.Error())
		return nil, resp.Header, resp.Cookies(), statusCode, err
	}
	return
}

func CsvToMap(ctx context.Context, csvData [][]string, lowerCaseHeader bool) (jsonData []map[string]interface{}, err error) {
	logs.WithContext(ctx).Debug("CsvToMap - Start")
	charsToRemove := []string{"."}
	for j, _ := range csvData[0] {
		csvData[0][j] = regexp.MustCompile(`[^a-zA-Z0-9 ]+`).ReplaceAllString(csvData[0][j], "")
		if lowerCaseHeader {
			csvData[0][j] = strings.ToLower(csvData[0][j])
		}
		csvData[0][j] = strings.Replace(csvData[0][j], " ", "_", -1)
		for _, v := range charsToRemove {
			csvData[0][j] = strings.Replace(csvData[0][j], v, "", -1)
		}
	}
	for i, line := range csvData {
		if i > 0 {
			jsonMap := make(map[string]interface{})
			for j, field := range line {
				jsonMap[csvData[0][j]] = field
			}
			jsonData = append(jsonData, jsonMap)
		}
	}
	return
}

type DiffOutput struct {
	Path   string
	AddStr string
	DelStr string
}

type DiffReporter struct {
	path    cmp.Path
	diffs   map[string]DiffOutput
	diffStr []string
}

func (r *DiffReporter) PushStep(ps cmp.PathStep) {
	r.path = append(r.path, ps)
}

func (r *DiffReporter) Report(rs cmp.Result) {
	if !rs.Equal() {
		vx, vy := r.path.Last().Values()
		do := DiffOutput{}
		path := fmt.Sprintf("%v ", r.path)
		//do.Path = fmt.Sprintf("%v", strings.Replace(r.path.Last().String(), "\"", "", -1))
		do.Path = fmt.Sprintf("%v", strings.Replace(r.path.GoString(), "\"", "", -1))
		do.AddStr = fmt.Sprintf("%+v", vy)
		do.DelStr = fmt.Sprintf("%+v", vx)
		if r.diffs == nil {
			r.diffs = make(map[string]DiffOutput)
		}
		r.diffs[path] = do
		r.diffStr = append(r.diffStr, fmt.Sprintf("%#v:\n\t-: %+v\n\t+: %+v\n", r.path, vx, vy))
	}
}

func (r *DiffReporter) PopStep() {
	r.path = r.path[:len(r.path)-1]
}

func (r *DiffReporter) String() string {
	return strings.Join(r.diffStr, "\n")
}

func (r *DiffReporter) Output() map[string]DiffOutput {
	return r.diffs
}
