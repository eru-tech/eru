package validator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"regexp"
	"strings"
)

const (
	LengthCheckExact = "EXACT"
	LengthCheckMin   = "MIN"
	LengthCheckMax   = "MAX"
)

type StringField struct {
	Field
	Length      int      `json:"length"`
	LengthCheck string   `json:"length_check"`
	FormatCheck string   `json:"format_check"`
	Values      []string `json:"values"`
}

func (f *StringField) Validate(ctx context.Context, v interface{}) (err error) {
	var errs []string
	defer func() {
		if len(errs) > 0 {
			err = errors.New(strings.Join(errs, " ; "))
			logs.WithContext(ctx).Error(fmt.Sprint(errs))
		} else {
			err = nil
		}
	}()

	if v == nil {
		if f.Required {
			errs = append(errs, fmt.Sprint("'", f.Name, "' cannot be blank"))
		}
		return
	}

	value := ""
	switch vv := v.(type) {
	case int, float64, string:
		value = fmt.Sprintf("%v", vv)
	default:
		errs = append(errs, fmt.Sprint("'", f.Name, "' has to be a string"))
		return
	}

	if value == "" {
		if f.Required {
			errs = append(errs, fmt.Sprint("'", f.Name, "' cannot be blank"))
		}
		return
	}

	if len(f.Values) > 0 && value != "" {
		vFound := false
		logs.WithContext(ctx).Info(fmt.Sprint(f.Values))
		logs.WithContext(ctx).Info(fmt.Sprint(value))
		for _, val := range f.Values {
			if value == val {
				vFound = true
			}
		}
		if !vFound {
			errs = append(errs, fmt.Sprint("invalid value for field '", f.Name, "'"))
			return
		}
	}

	if f.FormatCheck != "" {
		r, rErr := regexp.Compile(f.FormatCheck)
		if rErr != nil {
			errs = append(errs, fmt.Sprint("invalid format checker for field '", f.Name, "'"))
			return
		}

		logs.WithContext(ctx).Info(fmt.Sprint(r))
		logs.WithContext(ctx).Info(fmt.Sprint(value))

		isValidFormat := r.MatchString(value)

		if !isValidFormat {
			errs = append(errs, fmt.Sprint("invalid string format for field '", f.Name, "'"))
			return
		}
	}

	if f.LengthCheck != "" {
		switch f.LengthCheck {
		case LengthCheckExact:
			if f.Length != len(value) {
				errs = append(errs, fmt.Sprint("length of '", f.Name, "' has to be ", f.Length))
			}
			break
		case LengthCheckMin:
			if f.Length > len(value) {
				errs = append(errs, fmt.Sprint("minimum length of '", f.Name, "' has to be ", f.Length))
			}
			break
		case LengthCheckMax:
			if f.Length < len(value) {
				errs = append(errs, fmt.Sprint("maximum length of '", f.Name, "' has to be ", f.Length))
			}
			break
		default:
			//do nothing
		}
	}
	return
}

func (f *StringField) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &f)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}
