package channel

import (
	"context"
	"encoding/json"
	"errors"

	eru_db "github.com/eru-tech/eru/eru-db/db"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

type ChannelI interface {
	//Login(req *http.Request) (res interface{}, cookies []*http.Cookie, err error)
	SetAuthDb(dbI eru_db.DbI)
	GetAuthDb() (dbI eru_db.DbI)
	GetAttribute(ctx context.Context, attributeName string) (attributeValue interface{}, err error)
	MakeFromJson(ctx context.Context, rj *json.RawMessage) (err error)
}

const (
	OTP_PURPOSE_RECOVERY = "RECOVERY"
	OTP_PURPOSE_VERIFY   = "VERIFY"
)

type Channel struct {
	ChannelType string     `json:"channel_type" eru:"required"`
	ChannelName string     `json:"channel_name" eru:"required"`
	ChannelDb   eru_db.DbI `json:"-"`
}

func (channel *Channel) SetAuthDb(dbI eru_db.DbI) {
	channel.ChannelDb = dbI
}

func (channel *Channel) GetAuthDb() (dbI eru_db.DbI) {
	return channel.ChannelDb
}

func (channel *Channel) GetAttribute(ctx context.Context, attributeName string) (attributeValue interface{}, err error) {
	switch attributeName {
	case "ChannelType":
		return channel.ChannelType, nil
	case "ChannelName":
		return channel.ChannelName, nil
	default:
		return nil, errors.New("attribute not found")
	}
}

func (channel *Channel) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	err := errors.New("MakeFromJson Method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return err
}

func GetChannel(channelType string) ChannelI {
	switch channelType {
	/* case "SMTP":
		return new(SmtpChannel)
	case "MESSENGER":
		return new(MessengerChannel) */
	default:
		return new(Channel)
	}
}
