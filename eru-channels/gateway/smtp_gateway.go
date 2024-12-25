package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

type SmtpGateway struct {
	Gateway
	SmtpHost string `json:"smtp_host"`
	SmtpPort string `json:"smtp_port"`
}

func (smtpGateway *SmtpGateway) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &smtpGateway)
	if err != nil {
		err = errors.New(fmt.Sprint("error json.Unmarshal(*rj, &emailGateway) ", err.Error()))
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}
