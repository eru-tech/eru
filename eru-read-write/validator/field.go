package validator

import (
	"context"
	"encoding/json"
	"errors"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

const (
	DataTypeString  = "string"
	DataTypeBoolean = "boolean"
	DataTypeNumber  = "number"
)

type FieldI interface {
	GetName() string
	GetDatatype() string
	Validate(ctx context.Context, v interface{}) error
	MakeFromJson(ctx context.Context, rj *json.RawMessage) error
}

type Field struct {
	Name     string `json:"name" eru:"required"`
	Required bool   `json:"required" eru:"required"`
	DataType string `json:"data_type" eru:"required"`
}

func (f *Field) GetName() string {
	return f.Name
}

func (f *Field) GetDatatype() string {
	return f.DataType
}

func GetField(dt string) FieldI {
	switch dt {
	case DataTypeString:
		return new(StringField)
	case DataTypeNumber:
		return new(NumberField)
	case DataTypeBoolean:
		return new(BooleanField)
	default:
		return new(Field)
	}
}

func (f *Field) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson method not implemented")
	return nil
}

func (f *Field) Validate(ctx context.Context, v interface{}) error {
	logs.WithContext(ctx).Debug("Validate method not implemented")
	return errors.New("Validate method not implemented")
}
