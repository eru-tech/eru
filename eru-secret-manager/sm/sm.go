package sm

import (
	"context"
	"encoding/json"
	"errors"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

const (
	AuthTypeSecret = "SECRET"
	AuthTypeIAM    = "IAM"
)

type SmStore struct {
	SmStoreType string `json:"sm_store_type" eru:"required"`
}

type SmStoreI interface {
	Init(ctx context.Context) (err error)
	FetchSmValue(ctx context.Context, smKey string) (smVal interface{}, err error)
	MakeFromJson(ctx context.Context, rj *json.RawMessage) error
}

func GetSm(storageType string) SmStoreI {
	switch storageType {
	case "AWS":
		return new(AwsSmStore)
	case "GCP":
		return new(GcpSmStore)

	default:
		return nil
	}
	return nil
}

func (smStore *SmStore) Init(ctx context.Context) (err error) {
	err = errors.New("method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return
}

func (smStore *SmStore) FetchSmValue(ctx context.Context, smKey string) (smVal interface{}, err error) {
	err = errors.New("method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return
}

func (smStore *SmStore) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &smStore)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}
