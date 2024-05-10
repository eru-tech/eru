package gotemplate

import (
	"bytes"
	"context"
	b64 "encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	sprig "github.com/Masterminds/sprig/v3"
	eruaes "github.com/eru-tech/eru/eru-crypto/aes"
	"github.com/eru-tech/eru/eru-crypto/jwt"
	erumd5 "github.com/eru-tech/eru/eru-crypto/md5"
	erursa "github.com/eru-tech/eru/eru-crypto/rsa"
	erusha "github.com/eru-tech/eru/eru-crypto/sha"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eruutils "github.com/eru-tech/eru/eru-utils"
	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"
)

type GoTemplate struct {
	Name     string
	Template string
}

type OrderedMap struct {
	Rank float64
	Obj  map[string]interface{}
}

type sorter []*OrderedMap

func (a sorter) Len() int {
	return len(a)
}
func (a sorter) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}
func (a sorter) Less(i, j int) bool {
	return a[i].Rank < a[j].Rank
}
func GenericFuncMap(ctx context.Context) map[string]interface{} {
	return map[string]interface{}{
		"inc": func(n int) int {
			return n + 1
		},
		"marshalJSON": func(j interface{}) ([]byte, error) {
			d, err := json.Marshal(j)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
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
				logs.WithContext(ctx).Error(err.Error())
				//return empty string with nil error to silently proceed even if base64 conversion fails
				return "", nil
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
			dst, err := eruaes.EncryptECB(ctx, pb, k)
			return dst, err
		},
		"aesDecryptECB": func(eb []byte, k []byte) ([]byte, error) {
			return eruaes.DecryptECB(ctx, eb, k)
		},
		"aesEncryptCBC": func(pb []byte, k []byte, iv []byte) ([]byte, error) {
			dst, err := eruaes.EncryptCBC(ctx, pb, k, iv)
			return []byte(dst), err
		},
		"aesDecryptCBC": func(eb []byte, k []byte, iv []byte) ([]byte, error) {
			return eruaes.DecryptCBC(ctx, eb, k, iv)
		},
		"encryptRSACert": func(j []byte, pubK string) ([]byte, error) {
			return erursa.EncryptWithCert(ctx, j, pubK)
		},
		"bytesToString": func(b []byte) string {
			return string(b)
		},
		"stringToByte": func(s string) []byte {
			return []byte(s)
		},
		"stringify": func(j interface{}) (string, error) {
			d, err := json.Marshal(j)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
			}
			return string(d), err
		},
		"unquote": func(s string) string {
			str, uerr := strconv.Unquote(string(s))
			if uerr != nil {
				logs.WithContext(ctx).Error(uerr.Error())
			}
			return str
		},
		"doubleQuote": func(s string) string {
			return strconv.Quote(fmt.Sprint("\"", s, "\""))
		},
		"generateAesKey": func(bits int) ([]byte, error) {
			aesObj, err := eruaes.GenerateKey(ctx, bits)
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
				err := errors.New(fmt.Sprint("SHA function not defined for ", bits, "bits"))
				logs.WithContext(ctx).Error(err.Error())
				return "", err
			}
		},
		"md5": func(str string, output string) (string, error) {
			return erumd5.Md5(ctx, str, output)
		},
		"PKCS7Pad": func(buf []byte, size int) []byte {
			return eruaes.Pad(buf, size)
		},
		"PKCS7Unpad": func(buf []byte) ([]byte, error) {
			return eruaes.Unpad(buf)
		},
		"saveVar": func(vars map[string]interface{}, ketToSave string, valueToSave interface{}) error {
			if vars == nil {
				vars = make(map[string]interface{})
			}
			vars[ketToSave] = valueToSave
			return nil
		},
		"concatMapKeyVal": func(vars map[string]interface{}, keys []string, seprator string) string {
			str := ""
			for _, k := range keys {
				str = fmt.Sprint(str, k, "=", vars[k], "|")
			}
			return str
		},
		"concatMapKeyValUnordered": func(vars map[string]interface{}, seprator string, keyFirst bool, varSeprator string) string {
			str := ""
			for k, _ := range vars {
				if keyFirst {
					str = fmt.Sprint(str, k, seprator, vars[k], varSeprator)
				} else {
					str = fmt.Sprint(str, vars[k], seprator, k, varSeprator)
				}

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
		"getMapValue": func(orgMap map[string]interface{}, key string) (d interface{}, err error) {
			d = make(map[string]interface{})
			ok := false
			d, ok = orgMap[key]
			if !ok {
				return orgMap, err
			}
			return d, nil
		},
		"getMapKeys": func(orgMap map[string]interface{}) (d []string, err error) {
			for k, _ := range orgMap {
				d = append(d, k)
			}
			return d, nil
		},
		"arrayLen": func(arr interface{}) (d int, err error) {
			d = 0
			if o, oOk := arr.([]interface{}); oOk {
				return len(o), err
			}
			if o, oOk := arr.([]string); oOk {
				return len(o), err
			}
			return d, errors.New("not an array")
		},
		"getMapPointerValue": func(orgMap map[string]*interface{}, key string) (d interface{}, err error) {
			d = make(map[string]interface{})
			ok := false
			d, ok = orgMap[key]
			if !ok {
				return orgMap, err
			}
			return d, nil
		},
		"getArrayValue": func(orgArray []interface{}, index int, emptyValue interface{}) (d interface{}) {
			if emptyValue == "object" {
				d = make(map[string]interface{})
			} else if emptyValue == "string" {
				d = ""
			} else if emptyValue == "number" {
				d = 0
			}
			if len(orgArray) <= index {
				return d
			}
			d = orgArray[index]
			return d
		},
		"sortMapArray": func(mapArray []interface{}, sortKey string) (mapArraySorted []interface{}, err error) {
			var tmpArray []*OrderedMap
			for _, v := range mapArray {
				if vMap, vMapOk := v.(map[string]interface{}); vMapOk {
					logs.WithContext(ctx).Error(fmt.Sprint(reflect.TypeOf(vMap[sortKey])))
					if r, rOk := vMap[sortKey].(float64); !rOk {
						err = errors.New("sortKey is not an int")
						return
					} else {
						o := OrderedMap{
							Rank: r,
							Obj:  vMap,
						}
						tmpArray = append(tmpArray, &o)
					}
				} else {
					err = errors.New("not a map array")
					return
				}
			}
			sort.Sort(sorter(tmpArray))
			for _, nv := range tmpArray {
				mapArraySorted = append(mapArraySorted, nv.Obj)
			}
			return
		},
		"logobject": func(v interface{}) (err error) {
			vobj, err := json.Marshal(v)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return
			}
			logs.WithContext(ctx).Info(fmt.Sprint("logobject = ", string(vobj)))
			return
		},
		"logstring": func(str interface{}) (err error) {
			logs.WithContext(ctx).Info(fmt.Sprint("logstring = ", str))
			return
		},
		"logerror": func(str interface{}) (err error) {
			logs.WithContext(ctx).Error(fmt.Sprint("logstring = ", str))
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
		"date_diff": func(indtstr string, n int, t string) (dt string, err error) {
			indt, err := time.Parse("2006-01-02", indtstr)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				err = errors.New("Invalid date format - expected formatted 2006-01-02")
				logs.WithContext(ctx).Error(err.Error())
				return "", err
			}
			y := 0
			m := 0
			d := 0
			switch t {
			case "days":
				d = n
			case "months":
				m = n
			case "years":
				y = n
			default:
				err = errors.New("Invalid type - expected values are years months and days")
				logs.WithContext(ctx).Error(err.Error())
				return "", err
			}
			dt = indt.AddDate(y, m, d).Format("2006-01-02")
			return dt, nil
		},
		"date_part": func(dtStr string, dtPart string) (datePart string, err error) {
			dt, err := time.Parse("2006-01-02", dtStr)
			switch dtPart {
			case "DAY":
				datePart = string(dt.Day())
			case "MONTHN":
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
		"date_format": func(dtStr string, srcLayout string, newLayout string) (datePart string, err error) {
			vDate, vErr := time.Parse(srcLayout, dtStr)
			if vErr != nil {
				err = vErr
				return
			}
			datePart = vDate.Format(newLayout)
			return
		},
		"str_concat": func(sep string, inStr ...string) (str string, err error) {
			str = strings.Join(inStr, sep)
			return
		},
		"str_replace": func(txt string, oldStr string, newStr string, num int) (str string, err error) {
			str = strings.Replace(txt, oldStr, newStr, num)
			return
		},
		"removenull": func(txt string) (str string, err error) {
			str = strings.Replace(txt, "\u0000", "", -1)
			str = strings.Replace(str, "\\u0000", "", -1)
			return
		},
		"math_add": func(args ...interface{}) (result float64, err error) {
			num := 0.0
			for _, a := range args {
				switch v := a.(type) {
				case int, float64:
					_ = v
					num, err = strconv.ParseFloat(fmt.Sprintf("%v", a), 64)
					if err != nil {
						logs.WithContext(ctx).Error(err.Error())
						return
					}
					result = result + num
				default:
					err = errors.New("Non Numeric Input")
					return
				}
			}
			return result, nil
		},
		"math_sub": func(a interface{}, b interface{}) (result float64, err error) {
			var n1, n2 float64
			switch v := a.(type) {
			case int, float64:
				_ = v
				n1, err = strconv.ParseFloat(fmt.Sprintf("%v", a), 64)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					return
				}
			default:
				err = errors.New("Non Numeric Input")
				return
			}
			switch v := b.(type) {
			case int, float64:
				_ = v
				n2, err = strconv.ParseFloat(fmt.Sprintf("%v", b), 64)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					return
				}
			default:
				err = errors.New("Non Numeric Input")
				return
			}
			result = n1 - n2
			return result, nil
		},
		"math_div": func(a interface{}, b interface{}) (result float64, err error) {
			var n1, n2 float64
			switch v := a.(type) {
			case int, float64:
				_ = v
				n1, err = strconv.ParseFloat(fmt.Sprintf("%v", a), 64)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					return
				}
			default:
				err = errors.New("Non Numeric Input")
				return
			}
			switch v := b.(type) {
			case int, float64:
				_ = v
				n2, err = strconv.ParseFloat(fmt.Sprintf("%v", b), 64)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					return
				}
			default:
				err = errors.New("Non Numeric Input")
				return
			}
			result = n1 / n2
			return result, nil
		},
		"math_mul": func(a float64, b float64) (result float64) {
			return a * b
		},
		"math_round": func(a interface{}, r float64) (result float64, err error) {
			var n1 float64
			m := 1.0
			switch v := a.(type) {
			case int, float64:
				_ = v
				n1, err = strconv.ParseFloat(fmt.Sprintf("%v", a), 64)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					return
				}
			default:
				err = errors.New("Non Numeric Input")
				return
			}
			result = math.Round(n1*m*r) / (m * r)
			return result, nil
		},
		"excelToJson": func(fData string, sheetNames string, firstRowHeader string, headers string, keys string) (fJson interface{}, err error) {
			return excelToJson(ctx, fData, sheetNames, firstRowHeader, headers, keys)
		},
		"null": func() interface{} {
			return nil
		},
		"char_index": func(s string, c string) int {
			return strings.Index(s, c)
		},
		"new_jwt": func(privateKeyStr string, claimsMap map[string]interface{}) (tokenString string, err error) {
			return jwt.CreateJWT(ctx, privateKeyStr, claimsMap)
		},
		"evalFilter": func(filter map[string]interface{}, record map[string]interface{}) (result bool, err error) {
			logs.WithContext(ctx).Info("----------------------------------- evalFilter starting --------------------------------")
			logs.WithContext(ctx).Info(fmt.Sprint(record))
			return evalFilter(ctx, filter, record)
		},
		"execTemplate": func(obj interface{}, templateString string, outputFormat string) (output interface{}, err error) {
			goTmpl := GoTemplate{"subtemplate", templateString}
			return goTmpl.Execute(ctx, obj, outputFormat)
		},
	}
}

func (goTmpl *GoTemplate) Execute(ctx context.Context, obj interface{}, outputFormat string) (output interface{}, err error) {
	logs.WithContext(ctx).Debug("Execute - Start")

	buf := &bytes.Buffer{}

	t := template.Must(template.New(goTmpl.Name).Funcs(sprig.FuncMap()).Funcs(GenericFuncMap(ctx)).Parse(goTmpl.Template))

	if err := t.Execute(buf, obj); err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return "", err
	}
	str := buf.String()
	switch outputFormat {
	case "string":
		if str == "<no value>" {
			err = errors.New("Template returned <no value>")
			logs.WithContext(ctx).Error(err.Error())
			return nil, err
		}
		return str, nil
	case "json":
		if str == "<no value>" {
			return nil, err
		}
		if err = json.Unmarshal([]byte(str), &output); err != nil {
			err = errors.New(fmt.Sprintf("Unable to marhsal templated output to JSON : ", buf.String(), " ", err))
			logs.WithContext(ctx).Error(err.Error())
			return nil, err
		} else {
			return
		}
	}
	err = errors.New(fmt.Sprint("Unknown output format : ", outputFormat))
	logs.WithContext(ctx).Error(err.Error())
	return nil, err
}
func evalFilter(ctx context.Context, filter map[string]interface{}, record map[string]interface{}) (result bool, err error) {
	for k, v := range filter {
		kk := fetchKey(k)
		if kk == "$or" {
			if vArray, vArrayOk := v.([]interface{}); vArrayOk {
				result, err = evalOrFilter(ctx, vArray, record)
				if !result {
					return false, nil
				}
			} else {
				err = errors.New("$or needs an array")
				return false, err
			}
		} else {
			if recordValue, recordValueOk := record[kk]; recordValueOk {
				if vMap, vMapOk := v.(map[string]interface{}); vMapOk {
					result, err = evalCondition(ctx, vMap, recordValue)
					if !result {
						return false, err
					}
				} else {
					result = eruutils.ImplCompare(recordValue, v)
					if !result {
						return false, err
					}
				}
			} else {
				err = errors.New(fmt.Sprint("key : ", kk, " not found in data"))
				logs.WithContext(ctx).Info(err.Error())
				return false, err
			}
		}
	}
	return true, nil
}
func evalCondition(ctx context.Context, cond map[string]interface{}, recordValue interface{}) (result bool, err error) {
	for ck, cv := range cond {
		switch ck {
		case "$in":
			if cvArray, cvArrayOk := cv.([]interface{}); cvArrayOk {
				return eruutils.ImplContains(cvArray, recordValue), nil
			} else {
				err = errors.New("$in operator requires an array")
				logs.WithContext(ctx).Error(err.Error())
				return false, err
			}
		case "$nin":
			if cvArray, cvArrayOk := cv.([]interface{}); cvArrayOk {
				return !(eruutils.ImplContains(cvArray, recordValue)), nil
			} else {
				err = errors.New("$nin operator requires an array")
				logs.WithContext(ctx).Error(err.Error())
				return false, err
			}
		case "$like":
			if rvStr, rvStrOk := recordValue.(string); rvStrOk {
				if cvStr, cvStrOk := cv.(string); cvStrOk {
					return strings.Contains(rvStr, cvStr), nil
				} else {
					err = errors.New("$like operator requires a string")
					logs.WithContext(ctx).Error(err.Error())
					return false, err
				}
			} else {
				err = errors.New("$like operator requires a string to compare")
				logs.WithContext(ctx).Error(err.Error())
				return false, err
			}
		case "$nlike":
			if rvStr, rvStrOk := recordValue.(string); rvStrOk {
				if cvStr, cvStrOk := cv.(string); cvStrOk {
					return !(strings.Contains(rvStr, cvStr)), nil
				} else {
					err = errors.New("$nlike operator requires a string")
					logs.WithContext(ctx).Error(err.Error())
					return false, err
				}
			} else {
				err = errors.New("$nlike operator requires a string to compare")
				logs.WithContext(ctx).Error(err.Error())
				return false, err
			}
		case "$gt":
			if rvF, rvFOk := recordValue.(float64); rvFOk {
				if cvF, cvFOk := cv.(float64); cvFOk {
					return (rvF > cvF), nil
				} else {
					err = errors.New("$gt operator requires a number")
					logs.WithContext(ctx).Error(err.Error())
					return false, err
				}
			} else {
				err = errors.New("$gt operator requires a number to compare")
				logs.WithContext(ctx).Error(err.Error())
				return false, err
			}
		case "$gte":
			if rvF, rvFOk := recordValue.(float64); rvFOk {
				if cvF, cvFOk := cv.(float64); cvFOk {
					return (rvF >= cvF), nil
				} else {
					err = errors.New("$gte operator requires a number")
					logs.WithContext(ctx).Error(err.Error())
					return false, err
				}
			} else {
				err = errors.New("$gte operator requires a number to compare")
				logs.WithContext(ctx).Error(err.Error())
				return false, err
			}
		case "$lt":
			if rvF, rvFOk := recordValue.(float64); rvFOk {
				if cvF, cvFOk := cv.(float64); cvFOk {
					return (rvF < cvF), nil
				} else {
					err = errors.New("$lt operator requires a number")
					logs.WithContext(ctx).Error(err.Error())
					return false, err
				}
			} else {
				err = errors.New("$lt operator requires a number to compare")
				logs.WithContext(ctx).Error(err.Error())
				return false, err
			}
		case "$lte":
			if rvF, rvFOk := recordValue.(float64); rvFOk {
				if cvF, cvFOk := cv.(float64); cvFOk {
					return (rvF <= cvF), nil
				} else {
					err = errors.New("$lte operator requires a number")
					logs.WithContext(ctx).Error(err.Error())
					return false, err
				}
			} else {
				err = errors.New("$lte operator requires a number to compare")
				logs.WithContext(ctx).Error(err.Error())
				return false, err
			}
		case "$ne":
			return !(eruutils.ImplCompare(cv, recordValue)), nil
		case "$eq":
			return eruutils.ImplCompare(cv, recordValue), nil
		case "$jin":
			//todo implement jin and jnin
			err = errors.New("$jin not implemented")
			logs.WithContext(ctx).Error(err.Error())
			return
		case "$jnin":
			err = errors.New("jnin not implemented")
			logs.WithContext(ctx).Error(err.Error())
			return
		default:
			logs.WithContext(ctx).Info("operator not found")
			return false, nil
		}
	}
	return
}
func fetchKey(k string) (key string) {
	key = k
	kArray := strings.Split(k, "___")
	if len(kArray) > 1 {
		key = kArray[1]
	}
	return
}
func evalOrFilter(ctx context.Context, filter []interface{}, record map[string]interface{}) (result bool, err error) {
	for i, v := range filter {
		if vMap, vMapOk := v.(map[string]interface{}); vMapOk {
			result, err = evalFilter(ctx, vMap, record)
		} else {
			err = errors.New("$or needs array of objects")
		}
		logs.WithContext(ctx).Info(fmt.Sprint("result of OR element no ", i))
		logs.WithContext(ctx).Info(fmt.Sprint(result))
		if result {
			return true, nil
		}
	}
	return
}
func excelToJson(ctx context.Context, fData string, sheetNames string, firstRowHeader string, headers string, mapKeys string) (fJson interface{}, err error) {
	sheetNameArray := strings.Split(sheetNames, ",")
	sheetHeadersArray := strings.Split(headers, ",")
	firstRowHeaderArray := strings.Split(firstRowHeader, ",")
	mapKeysArray := strings.Split(mapKeys, ",")

	result := make(map[string][]map[string]interface{})
	fDataDecoded, err := b64.StdEncoding.DecodeString(fData)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return "", nil
	}
	f, err := excelize.OpenReader(bytes.NewReader(fDataDecoded))
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return "", err
	}
	defer func() {
		// Close the spreadsheet.
		if err := f.Close(); err != nil {
			logs.WithContext(ctx).Error(err.Error())
		}
	}()
	sheetFound := false
	if sheetNameArray[0] == "" {
		sheetFound = true
	}
	for sNo, sheetName := range f.GetSheetList() {
		outputSheetName := sheetName
		for _, sn := range sheetNameArray {
			if sn == sheetName {
				sheetFound = true
				break
			}
		}
		if sheetFound {
			if sNo < len(mapKeysArray) {
				if mapKeysArray[sNo] != "" {
					outputSheetName = mapKeysArray[sNo]
				}
			}
			resultRow := make(map[string]interface{})
			isFirstRowHeader := false
			if sNo < len(firstRowHeaderArray) {
				isFirstRowHeader, err = strconv.ParseBool(firstRowHeaderArray[sNo])
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					isFirstRowHeader = false
				}
			}
			if sNo >= len(sheetHeadersArray) && !isFirstRowHeader {
				resultRow["error"] = "header information missing"
				result[outputSheetName] = append(result[outputSheetName], resultRow)
			} else if sheetHeadersArray[0] == "" && !isFirstRowHeader {
				resultRow["error"] = "header information missing"
				result[outputSheetName] = append(result[outputSheetName], resultRow)
			} else {
				var keys []string
				if sNo < len(sheetHeadersArray) {
					keys = strings.Split(sheetHeadersArray[sNo], "|")
				}

				rows, rErr := f.GetRows(sheetName)
				if rErr != nil {
					err = rErr
					logs.WithContext(ctx).Error(err.Error())
					return
				}
				for rNo, row := range rows {
					resultRow = make(map[string]interface{})
					skipRow := false
					for cNo, colCell := range row {
						skipRow = false
						if rNo == 0 && isFirstRowHeader {
							skipRow = true
							if cNo >= len(keys) {
								keys = append(keys, colCell)
							} else if keys[cNo] == "" {
								keys[cNo] = colCell
							}
						} else {
							k := ""
							if cNo >= len(keys) {
								k = fmt.Sprint("C", cNo)
							} else if keys[cNo] == "" {
								k = fmt.Sprint("C", cNo)
							} else {
								k = keys[cNo]
							}
							resultRow[k] = colCell
						}
					}
					if !skipRow {
						result[outputSheetName] = append(result[outputSheetName], resultRow)
					}
				}
			}
		}
	}
	return result, nil
}
