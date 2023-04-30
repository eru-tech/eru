package module_store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/eru-tech/eru/eru-alerts/channel"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

func (ms *ModuleStore) checkProjectExists(ctx context.Context, projectId string) error {
	logs.WithContext(ctx).Debug("checkProjectExists - Start")
	_, ok := ms.Projects[projectId]
	if !ok {
		err := errors.New(fmt.Sprint("project ", projectId, " not found"))
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func UnMarshalStore(ctx context.Context, b []byte, msi ModuleStoreI) error {
	logs.WithContext(ctx).Debug("UnMarshalStore - Start")
	var storeMap map[string]*json.RawMessage
	err := json.Unmarshal(b, &storeMap)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}

	var prjs map[string]*json.RawMessage
	if _, ok := storeMap["projects"]; ok {

		err = json.Unmarshal(*storeMap["projects"], &prjs)
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
			return err
		}

		for prj, prjJson := range prjs {
			msi.SaveProject(ctx, prj, nil, false)

			var prjObjs map[string]*json.RawMessage
			err = json.Unmarshal(*prjJson, &prjObjs)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return err
			}
			p, e := msi.GetProjectConfig(ctx, prj)
			if e != nil {
				return err
			}

			var messageTemplates map[string]channel.MessageTemplate
			err = json.Unmarshal(*prjObjs["MessageTemplates"], &messageTemplates)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return err
			}
			p.MessageTemplates = messageTemplates

			var channels map[string]*json.RawMessage
			err = json.Unmarshal(*prjObjs["Channels"], &channels)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return err
			}
			for channelKey, channelJson := range channels {
				logs.WithContext(ctx).Info(fmt.Sprint("channelKey === ", channelKey))
				var channelObj map[string]*json.RawMessage
				err = json.Unmarshal(*channelJson, &channelObj)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					return err
				}
				var channelType string
				err = json.Unmarshal(*channelObj["ChannelType"], &channelType)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					return err
				}
				channelI := channel.GetChannel(channelType)
				err = channelI.MakeFromJson(ctx, channelJson)
				if err == nil {
					err = msi.SaveChannel(ctx, channelI, prj, nil, false)
					if err != nil {
						return err
					}
				} else {
					return err
				}
			}
		}
	}
	return nil
}
