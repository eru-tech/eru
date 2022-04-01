package eru_utils

import (
	"crypto/sha512"
	"encoding/base64"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

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
