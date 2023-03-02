package module_store

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/eru-tech/eru/eru-alerts/channel"
	"log"
)

func (ms *ModuleStore) checkProjectExists(projectId string) error {
	_, ok := ms.Projects[projectId]
	if !ok {
		return errors.New(fmt.Sprint("project ", projectId, " not found"))
	}
	return nil
}

func UnMarshalStore(b []byte, msi ModuleStoreI) error {
	var storeMap map[string]*json.RawMessage
	err := json.Unmarshal(b, &storeMap)
	if err != nil {
		log.Print("error json.Unmarshal(storeData, ms)")
		log.Print(err)
		return err
	}

	var prjs map[string]*json.RawMessage
	if _, ok := storeMap["projects"]; ok {

		err = json.Unmarshal(*storeMap["projects"], &prjs)
		if err != nil {
			log.Print("error json.Unmarshal(*storeMap[\"projects\"], &prjs)")
			log.Print(err)
			return err
		}

		for prj, prjJson := range prjs {
			msi.SaveProject(prj, nil, false)

			var prjObjs map[string]*json.RawMessage
			err = json.Unmarshal(*prjJson, &prjObjs)
			if err != nil {
				log.Print("error json.Unmarshal(*prgJson, &prgObjs)")
				log.Print(err)
				return err
			}
			p, e := msi.GetProjectConfig(prj)
			if e != nil {
				log.Print(err)
				return err
			}

			var messageTemplates map[string]channel.MessageTemplate
			err = json.Unmarshal(*prjObjs["MessageTemplates"], &messageTemplates)
			if err != nil {
				log.Print("error json.Unmarshal(*prjObjs[\"MessageTemplates\"], &messageTemplates)")
				log.Print(err)
				return err
			}
			p.MessageTemplates = messageTemplates

			var channels map[string]*json.RawMessage
			err = json.Unmarshal(*prjObjs["Channels"], &channels)
			if err != nil {
				log.Print("error json.Unmarshal(*prgObjs[\"channels\"], &channels)")
				log.Print(err)
				return err
			}
			log.Println(channels)
			for channelKey, channelJson := range channels {
				log.Println("channelKey === ", channelKey)
				var channelObj map[string]*json.RawMessage
				err = json.Unmarshal(*channelJson, &channelObj)
				if err != nil {
					log.Print("error json.Unmarshal(*channelJson, &channelObj)")
					log.Print(err)
					return err
				}
				var channelType string
				err = json.Unmarshal(*channelObj["ChannelType"], &channelType)
				if err != nil {
					log.Print("error json.Unmarshal(*storageObj[\"ChannelType\"], &channelType)")
					log.Print(err)
					return err
				}
				channelI := channel.GetChannel(channelType)
				err = channelI.MakeFromJson(channelJson)
				if err == nil {
					err = msi.SaveChannel(channelI, prj, nil, false)
					if err != nil {
						log.Println(err)
						return err
					}
				} else {
					log.Println(err)
					return err
				}
			}
		}
	}
	return nil
}
