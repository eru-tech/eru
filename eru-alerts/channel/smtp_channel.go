package channel

import (
	"encoding/json"
	gomail "gopkg.in/gomail.v2"
	"log"
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

func (smtpChannel *SmtpChannel) MakeFromJson(rj *json.RawMessage) error {
	log.Println("inside SmtpChannel MakeFromJson")
	err := json.Unmarshal(*rj, &smtpChannel)
	if err != nil {
		log.Print("error json.Unmarshal(*rj, &smtpChannel)")
		log.Print(err)
		return err
	}
	return nil
}

func (smtpChannel *SmtpChannel) Execute(r *http.Request, mt MessageTemplate) (response *http.Response, err error) {

	smtpMsg, msgErr := smtpChannel.ProcessMessageTemplate(r, mt)
	if msgErr != nil {
		log.Println("msgErr = ", msgErr)
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
	//msg.Attach("/home/User/cat.jpg")

	log.Println(msg)

	n := gomail.NewDialer(smtpChannel.SmtpHost, smtpChannel.SmtpPort, smtpChannel.SmtpUser, smtpChannel.SmtpPassword)
	// Send the email
	if smtpErr := n.DialAndSend(msg); smtpErr != nil {
		log.Println(smtpErr)
		return nil, smtpErr
	}
	return response, nil
}
