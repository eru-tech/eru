package gotemplate

import (
	"bytes"
	"context"
	b64 "encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	eruaes "github.com/eru-tech/eru/eru-crypto/aes"
	erumd5 "github.com/eru-tech/eru/eru-crypto/md5"
	erursa "github.com/eru-tech/eru/eru-crypto/rsa"
	erusha "github.com/eru-tech/eru/eru-crypto/sha"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/xuri/excelize/v2"
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

func (goTmpl *GoTemplate) Execute(ctx context.Context, obj interface{}, outputFormat string) (output interface{}, err error) {
	logs.WithContext(ctx).Debug("Execute - Start")
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
		"unquote": func(s string) string {
			str, uerr := strconv.Unquote(string(s))
			if uerr != nil {
				logs.WithContext(ctx).Error(uerr.Error())
			}
			return str
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
				err = errors.New(fmt.Sprint("SHA function not defined for ", bits, "bits"))
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
		"getMapValue": func(orgMap map[string]interface{}, key string) (d interface{}, err error) {
			d = make(map[string]interface{})
			ok := false
			d, ok = orgMap[key]
			if !ok {
				return orgMap, err
			}
			return d, nil
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
		"concat": func(sep string, inStr ...string) (str string, err error) {
			str = strings.Join(inStr, sep)
			return
		},
		"replace": func(txt string, oldStr string, newStr string, num int) (str string, err error) {
			str = strings.Replace(txt, oldStr, newStr, num)
			return
		},
		"removenull": func(txt string) (str string, err error) {
			str = strings.Replace(txt, "\u0000", "", -1)
			str = strings.Replace(str, "\\u0000", "", -1)
			return
		},
		"mul": func(a float64, b float64) (result float64) {
			return a * b
		},
		"excelToJson": func(fData string, sheetNames string, firstRowHeader string, headers string, keys string) (fJson interface{}, err error) {

			return excelToJson(ctx, fData, sheetNames, firstRowHeader, headers, keys)
		},
		"null": func() interface{} {
			return nil
		},
	}

	buf := &bytes.Buffer{}

	t := template.Must(template.New(goTmpl.Name).Funcs(funcs).Parse(goTmpl.Template))

	if err := t.Execute(buf, obj); err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return "", err
	}
	switch outputFormat {
	case "string":
		if buf.String() == "<no value>" {
			err = errors.New("Template returned <no value>")
			logs.WithContext(ctx).Error(err.Error())
			return nil, err
		}
		return buf.String(), nil
	case "json":
		if err = json.Unmarshal([]byte(buf.String()), &output); err != nil {
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
