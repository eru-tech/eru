package validator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"strings"
)

type BooleanField struct {
	Field
	CheckValue bool `json:"check_value"`
	Value      bool `json:"value"`
}

func (f *BooleanField) Validate(ctx context.Context, v interface{}) (err error) {
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

	value, ok := v.(bool)
	if !ok {
		errs = append(errs, fmt.Sprint("'", f.Name, "' has to be a boolean"))
		return
	}

	if f.CheckValue && f.Value != value {
		errs = append(errs, fmt.Sprint("invalid value for field '", f.Name, "'"))
	}

	return
}

func (f *BooleanField) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &f)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}
