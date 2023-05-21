package module_model

import (
	"context"
	"fmt"
	"github.com/eru-tech/eru/eru-alerts/channel"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

type ModuleProjectI interface {
}

type Project struct {
	ProjectId        string `eru:"required"`
	MessageTemplates map[string]channel.MessageTemplate
	Channels         map[string]channel.ChannelI
}

func (prg *Project) AddChannel(ctx context.Context, channelObjI channel.ChannelI) error {
	logs.WithContext(ctx).Debug("AddChannel - Start")
	channelName, err := channelObjI.GetAttribute("ChannelName")
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	cKey := fmt.Sprint(channelName.(string))
	if prg.Channels == nil {
		prg.Channels = make(map[string]channel.ChannelI)
	}
	prg.Channels[cKey] = channelObjI
	return nil
}
