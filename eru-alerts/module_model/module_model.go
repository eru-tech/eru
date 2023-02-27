package module_model

import (
	"fmt"
	"github.com/eru-tech/eru/eru-alerts/channel"
	"log"
)

type ModuleProjectI interface {
}

type Project struct {
	ProjectId        string `eru:"required"`
	MessageTemplates map[string]channel.MessageTemplate
	Channels         map[string]channel.ChannelI
}

func (prg *Project) AddChannel(channelObjI channel.ChannelI) error {
	log.Println("inside AddChannel")
	channelName, err := channelObjI.GetAttribute("ChannelName")
	if err != nil {
		return err
	}

	cKey := fmt.Sprint(channelName.(string))
	log.Print(cKey)
	if prg.Channels == nil {
		prg.Channels = make(map[string]channel.ChannelI)
	}
	prg.Channels[cKey] = channelObjI
	return nil
}
