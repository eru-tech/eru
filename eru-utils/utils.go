package eru_utils

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	models "github.com/eru-tech/eru/eru-models"
	"github.com/google/go-cmp/cmp"
	"github.com/jmoiron/sqlx"
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
		isOptional := false
		projectTags := f.Type().Field(i).Tag.Get("eru")
		if strings.Contains(projectTags, "required") {
			isRequired = true
			if f.Field(i).IsZero() && f.Field(i).Kind() != reflect.Bool {
				errs = append(errs, fmt.Sprint(parentKey, f.Type().Field(i).Name))
				isError = true
			}
		}
		if strings.Contains(projectTags, "optional") {
			isOptional = true
		}
		if !isError && !isOptional {
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
	logs.WithContext(ctx).Info(msg)
	if response != nil {
		body, err := io.ReadAll(response.Body)
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
		}
		if response.Request != nil {
			logs.WithContext(ctx).Info(fmt.Sprint(response.Request.URL))
		}
		cl, _ := strconv.Atoi(response.Header.Get("Content-Length"))
		if cl > 1000 {
			logs.WithContext(ctx).Info(string(body)[0:1000])
		} else if len(string(body)) > 1000 {
			logs.WithContext(ctx).Info(string(body)[0:1000])
		} else {
			logs.WithContext(ctx).Info(string(body))
		}
		response.Body = io.NopCloser(bytes.NewReader(body))
	} else {
		logs.WithContext(ctx).Info("response is nil")

	}
}

func PrintRequestBody(ctx context.Context, request *http.Request, msg string) {
	logs.WithContext(ctx).Debug("PrintRequestBody - Start")
	body, err := io.ReadAll(request.Body)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
	}
	logs.WithContext(ctx).Info(msg)
	logs.WithContext(ctx).Info(fmt.Sprint(request.URL))
	cl, _ := strconv.Atoi(request.Header.Get("Content-Length"))
	if cl > 1000 && len(string(body)) > 1000 {
		logs.WithContext(ctx).Info(string(body)[0:1000])
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
	//for _, c := range req.Cookies() {
	//	logs.WithContext(ctx).Info(c.String())
	//}

	resp, err = HTTPClientTransporter(http.DefaultTransport).RoundTrip(req)

	allowedOriginsI := ctx.Value("allowed_origins")
	originI := ctx.Value("origin")

	allowedOrigins := ""
	if allowedOriginsI != nil {
		allowedOrigins = allowedOriginsI.(string)
	}

	origin := ""
	if originI != nil {
		origin = originI.(string)
	}
	if req.Header.Get("Origin") == "" && origin != "" && allowedOrigins != "" {
		logs.WithContext(ctx).Info(fmt.Sprint("setting cors headers as origin is blank"))
		envOrigin := strings.Split(allowedOrigins, ",")
		for _, o := range envOrigin {
			oo := strings.Replace(o, "*.", "", -1)
			if strings.Contains(origin, oo) {
				resp.Header.Set("Access-Control-Allow-Origin", origin)
				resp.Header.Set("Access-Control-Allow-Credentials", "true")
				resp.Header.Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
				resp.Header.Set("Access-Control-Expose-Headers", "Authorization, Content-Type")
				break
			}
		}
	}
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
	//else {
	//	req.Header.Set("Content-Length", strconv.Itoa(bytes.NewReader(reqBody).Len()))
	//}
	return ExecuteHttp(ctx, req)
}

func CallHttp(ctx context.Context, method string, url string, headers http.Header, formData map[string]string, reqCookies []*http.Cookie, params map[string]string, postBody interface{}) (res interface{}, respHeaders http.Header, respCookies []*http.Cookie, statusCode int, err error) {
	logs.WithContext(ctx).Debug("CallHttp - Start")
	resp, err := callHttp(ctx, method, url, headers, formData, reqCookies, params, postBody)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, resp.Header, resp.Cookies(), resp.StatusCode, err
	}
	statusCode = resp.StatusCode
	respHeaders = resp.Header
	respCookies = resp.Cookies()
	defer resp.Body.Close()
	//todo - check if below change from reqContentType to header.get breaks anything
	//todo - merge conflict - main had below first if commented
	contentType := strings.Split(headers.Get("Content-type"), ";")[0]
	respcontentType := strings.Split(resp.Header.Get("Content-type"), ";")[0]
	if resp.ContentLength > 0 || contentType == encodedForm || contentType == applicationJson {
		if respcontentType == applicationJson {
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
	Path   string `json:"path"`
	AddStr string `json:"add_str"`
	DelStr string `json:"del_str"`
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

func CloneInterface(ctx context.Context, i interface{}) (iClone interface{}, err error) {
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

func ExecuteDbFetch(ctx context.Context, db *sqlx.DB, query models.Queries) (output []map[string]interface{}, err error) {
	logs.WithContext(ctx).Debug("ExecuteDbFetch - Start")

	rows, err := db.Queryx(query.Query, query.Vals...)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
	mapping := make(map[string]interface{})
	colsType, ee := rows.ColumnTypes()
	if ee != nil {
		return nil, ee
	}
	for rows.Next() {
		innerResultRow := make(map[string]interface{})
		ee = rows.MapScan(mapping)
		if ee != nil {
			return nil, ee
		}
		for _, colType := range colsType {
			if colType.DatabaseTypeName() == "NUMERIC" && mapping[colType.Name()] != nil {
				f := 0.0
				if reflect.TypeOf(mapping[colType.Name()]).String() == "[]uint8" {
					f, err = strconv.ParseFloat(string(mapping[colType.Name()].([]byte)), 64)
					mapping[colType.Name()] = f
				} else if reflect.TypeOf(mapping[colType.Name()]).String() == "float64" {
					f = mapping[colType.Name()].(float64)
					mapping[colType.Name()] = f
				}
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					return nil, err
				}
			} else if (colType.DatabaseTypeName() == "JSONB" || colType.DatabaseTypeName() == "JSON") && mapping[colType.Name()] != nil {
				bytesToUnmarshal := mapping[colType.Name()].([]byte)
				var v map[string]interface{}
				err = json.Unmarshal(bytesToUnmarshal, &v)
				if err != nil {
					return nil, err
				}
				mapping[colType.Name()] = &v
			}
			innerResultRow[colType.Name()] = mapping[colType.Name()]
		}
		output = append(output, innerResultRow)
	}
	return
}

func ExecuteDbSave(ctx context.Context, db *sqlx.DB, queries []*models.Queries) (output [][]map[string]interface{}, err error) {
	logs.WithContext(ctx).Debug("ExecuteDbSave - Start")
	tx := db.MustBegin()
	for _, q := range queries {
		stmt, err := tx.PreparexContext(ctx, q.Query)
		if err != nil {
			logs.WithContext(ctx).Error(fmt.Sprint("Error in tx.PreparexContext : ", err.Error()))
			tx.Rollback()
			return nil, err
		}
		rw, err := stmt.QueryxContext(ctx, q.Vals...)
		if err != nil {
			logs.WithContext(ctx).Error(fmt.Sprint("Error in stmt.QueryxContext : ", err.Error()))
			tx.Rollback()
			return nil, err
		}
		var innerOutput []map[string]interface{}
		for rw.Rows.Next() {
			resDoc := make(map[string]interface{})
			err = rw.MapScan(resDoc)
			if err != nil {
				logs.WithContext(ctx).Error(fmt.Sprint("Error in rw.MapScan : ", err.Error()))
				tx.Rollback()
				return nil, err
			}
			innerOutput = append(innerOutput, resDoc)
		}
		output = append(output, innerOutput)
	}
	err = tx.Commit()
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprint("Error in tx.Commit : ", err.Error()))
		tx.Rollback()
	}
	return
}

func ImplContains[T comparable](s []T, e T) bool {
	for _, v := range s {
		if v == e {
			return true
		}
	}
	return false
}

func ImplArrayContains[T comparable](s []T, e []T) bool {
	for _, v := range s {
		result := false
		for _, vv := range e {
			if v == vv {
				result = true
			}
		}
		if !result {
			return false
		}
	}
	return true
}

func ImplArrayNotContains[T comparable](s []T, e []T) bool {
	for _, v := range s {
		for _, vv := range e {
			if v == vv {
				return false
			}
		}
	}
	return true
}

func ImplCompare[T comparable](s T, e T) bool {
	return s == e
}

func GetNestedFieldValue(ctx context.Context, data interface{}, fieldPath string) (interface{}, error) {
	logs.WithContext(ctx).Debug("GetNestedFieldValue - Start")
	fields := strings.Split(fieldPath, ".")
	val := reflect.ValueOf(data)
	for _, field := range fields {
		if val.Kind() == reflect.Map {
			val = val.MapIndex(reflect.ValueOf(field))
			if !val.IsValid() {
				err := errors.New("invalid value")
				logs.WithContext(ctx).Error(err.Error())
				return nil, nil
			}
			if val.Kind() == reflect.Interface {
				val = reflect.ValueOf(val.Interface())
			}
		} else {
			err := errors.New("not a map")
			logs.WithContext(ctx).Error(err.Error())
			return nil, err
		}
	}
	return val.Interface(), nil
}
