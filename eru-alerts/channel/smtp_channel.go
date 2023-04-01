package channel

import (
	"context"
	"encoding/json"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	gomail "gopkg.in/gomail.v2"
	"net/http"
)

type SmtpChannel struct {
	Channel
	SmtpHost        string
	SmtpPort        int
	SmtpUser        string
	SmtpDisplayName string
	SmtpPassword    string
}

const (
	HTML_EMAIL = "text/html"
	TEXT_EMAIL = "text/plain"
)

func (smtpChannel *SmtpChannel) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &smtpChannel)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (smtpChannel *SmtpChannel) Execute(ctx context.Context, r *http.Request, mt MessageTemplate) (response *http.Response, err error) {
	logs.WithContext(ctx).Debug("Execute - Start")
	smtpMsg, msgErr := smtpChannel.ProcessMessageTemplate(ctx, r, mt)
	if msgErr != nil {
		return nil, msgErr
	}
	msg := gomail.NewMessage()
	msg.SetHeader("From", smtpChannel.SmtpUser, smtpChannel.SmtpDisplayName)
	msg.SetHeader("To", smtpMsg.To...)
	msg.SetHeader("Cc", smtpMsg.Cc...)
	msg.SetHeader("Bcc", smtpMsg.Bcc...)
	msg.SetHeader("Subject", smtpMsg.Subject)
	contentType := HTML_EMAIL
	if mt.TemplateType == "HTML" {
		contentType = HTML_EMAIL
	} else if mt.TemplateType == "TEXT" {
		contentType = TEXT_EMAIL
	}
	msg.SetBody(contentType, smtpMsg.Msg)

	n := gomail.NewDialer(smtpChannel.SmtpHost, smtpChannel.SmtpPort, smtpChannel.SmtpUser, smtpChannel.SmtpPassword)
	if smtpErr := n.DialAndSend(msg); smtpErr != nil {
		logs.WithContext(ctx).Error(smtpErr.Error())
		return nil, smtpErr
	}
	return response, nil
}
