package gateway

import (
	"encoding/json"
	"log"
)

type SmtpGateway struct {
	Gateway
	SmtpHost string
	SmtpPort string
}

func (smtpGateway *SmtpGateway) MakeFromJson(rj *json.RawMessage) error {
	log.Println("inside SmtpGateway MakeFromJson")
	err := json.Unmarshal(*rj, &smtpGateway)
	if err != nil {
		log.Print("error json.Unmarshal(*rj, &emailGateway)")
		log.Print(err)
		return err
	}
	return nil
}
