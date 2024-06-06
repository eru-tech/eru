package validator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"strings"
	"time"
)

type DateField struct {
	Field
	MinValue  string   `json:"min_value"`
	MinCheck  bool     `json:"min_check"`
	MaxValue  string   `json:"max_value"`
	MaxCheck  bool     `json:"max_check"`
	AllowDays []string `json:"allow_days"`
}

func (f *DateField) Validate(ctx context.Context, v interface{}) (err error) {
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
	var value time.Time
	value, err = time.Parse("2006-01-02", v.(string))
	if err != nil {
		errs = append(errs, fmt.Sprint("'", f.Name, "' has to be a date"))
		return
	}

	valueDtMin, valueDtMinErr := time.Parse("2006-01-02", f.MinValue)

	if valueDtMinErr != nil {
		errs = append(errs, fmt.Sprint("'incorrect minvalue for field ", f.Name))
		return
	}
	logs.WithContext(ctx).Info(fmt.Sprintln(value))
	logs.WithContext(ctx).Info(fmt.Sprintln(valueDtMin))
	logs.WithContext(ctx).Info(fmt.Sprintln(valueDtMin.Sub(value)))
	if f.MinCheck && valueDtMin.Sub(value) > 0 {
		errs = append(errs, fmt.Sprint("minimum date for '", f.Name, "' has to be ", f.MinValue))
	}

	valueDtMax, valueDtMaxErr := time.Parse("2006-01-02", f.MaxValue)

	if valueDtMaxErr != nil {
		errs = append(errs, fmt.Sprint("'incorrect maxvalue for field ", f.Name))
		return
	}
	logs.WithContext(ctx).Info(fmt.Sprintln(value))
	logs.WithContext(ctx).Info(fmt.Sprintln(valueDtMax))
	logs.WithContext(ctx).Info(fmt.Sprintln(valueDtMax.Sub(value)))

	if f.MaxCheck && valueDtMax.Sub(value) < 0 {
		errs = append(errs, fmt.Sprint("maximum date of '", f.Name, "' has to be ", f.MaxValue))
	}
	adErr := false
	wd := value.Weekday()
	logs.WithContext(ctx).Info(fmt.Sprint(wd))
	logs.WithContext(ctx).Info(fmt.Sprint(f.AllowDays))
	for _, ad := range f.AllowDays {
		logs.WithContext(ctx).Info(fmt.Sprint(ad))
		adErr = true
		switch ad {
		case "SUN":
			if wd == time.Sunday {
				adErr = false
				break
			}
		case "MON":
			if wd == time.Monday {
				adErr = false
				break
			}
		case "TUE":
			if wd == time.Tuesday {

				adErr = false
				break
			}
		case "WED":
			if wd == time.Wednesday {
				adErr = false
				break
			}
		case "THU":
			if wd == time.Thursday {
				adErr = false
				break
			}
		case "FRI":
			logs.WithContext(ctx).Info(fmt.Sprint(wd == time.Friday))
			logs.WithContext(ctx).Info(fmt.Sprint(wd))
			if wd == time.Friday {
				adErr = false
				break
			}
		case "SAT":
			if wd == time.Saturday {
				adErr = false
				break
			}
		}
		if !adErr {
			break
		}
	}
	if adErr {
		errs = append(errs, fmt.Sprint("invalid date value for '", f.Name))
	}
	return
}
func (f *DateField) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &f)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}
