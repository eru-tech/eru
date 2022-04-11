package module_store

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/eru-tech/eru/eru-auth/gateway"
	"github.com/eru-tech/eru/eru-auth/module_model"
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

			var messageTemplates map[string]module_model.MessageTemplate
			err = json.Unmarshal(*prjObjs["MessageTemplates"], &messageTemplates)
			if err != nil {
				log.Print("error json.Unmarshal(*prjObjs[\"MessageTemplates\"], &messageTemplates)")
				log.Print(err)
				return err
			}
			p.MessageTemplates = messageTemplates

			var gateways map[string]*json.RawMessage
			err = json.Unmarshal(*prjObjs["Gateways"], &gateways)
			if err != nil {
				log.Print("error json.Unmarshal(*prgObjs[\"gateways\"], &gateways)")
				log.Print(err)
				return err
			}
			log.Println(gateways)
			for gatewayKey, gatewayJson := range gateways {
				log.Println("gatewayKey === ", gatewayKey)
				var gatewayObj map[string]*json.RawMessage
				err = json.Unmarshal(*gatewayJson, &gatewayObj)
				if err != nil {
					log.Print("error json.Unmarshal(*gatewayJson, &gatewayObj)")
					log.Print(err)
					return err
				}
				var gatewayType string
				err = json.Unmarshal(*gatewayObj["GatewayType"], &gatewayType)
				if err != nil {
					log.Print("error json.Unmarshal(*storageObj[\"GatewayType\"], &gatewayType)")
					log.Print(err)
					return err
				}
				gatewayI := gateway.GetGateway(gatewayType)
				err = gatewayI.MakeFromJson(gatewayJson)
				if err == nil {
					err = msi.SaveGateway(gatewayI, prj, nil, false)
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
	log.Println(msi)
	return nil
}
