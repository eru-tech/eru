package module_store

import (
	"encoding/json"
	eruaes "github.com/eru-tech/eru/eru-crypto/aes"
	erursa "github.com/eru-tech/eru/eru-crypto/rsa"
	"github.com/eru-tech/eru/eru-files/storage"
	"log"
)

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
			var aeskeys map[string]eruaes.AesKey
			err = json.Unmarshal(*prjObjs["aes_keys"], &aeskeys)
			if err != nil {
				log.Print("error json.Unmarshal(*prjObjs[\"aes_keys\"], &aeskeys)")
				log.Print(err)
				return err
			}
			p.AesKeys = aeskeys

			var keypairs map[string]*json.RawMessage
			err = json.Unmarshal(*prjObjs["rsa_keypairs"], &keypairs)
			if err != nil {
				log.Print("error json.Unmarshal(*prgObjs[\"rsa_keypairs\"], &keypairs)")
				log.Print(err)
				return err
			}
			log.Println(keypairs)
			for keypairKey, keypairJson := range keypairs {
				log.Println("keypairKey === ", keypairKey)
				var keypairObj erursa.RsaKeyPair
				err = json.Unmarshal(*keypairJson, &keypairObj)
				if err != nil {
					log.Print("error json.Unmarshal(*keypairJson, &keypairObj)")
					log.Print(err)
					return err
				}

				p.RsaKeyPairs[keypairKey] = keypairObj
			}
			var storages map[string]*json.RawMessage
			err = json.Unmarshal(*prjObjs["storages"], &storages)
			if err != nil {
				log.Print("error json.Unmarshal(*prgObjs[\"storages\"], &storages)")
				log.Print(err)
				return err
			}
			for storageKey, storageJson := range storages {
				log.Println("storageKey === ", storageKey)
				var storageObj map[string]*json.RawMessage
				err = json.Unmarshal(*storageJson, &storageObj)
				if err != nil {
					log.Print("error json.Unmarshal(*storageJson, &storageObj)")
					log.Print(err)
					return err
				}
				var storageType string
				err = json.Unmarshal(*storageObj["storage_type"], &storageType)
				if err != nil {
					log.Print("error json.Unmarshal(*storageObj[\"storage_type\"], &storageType)")
					log.Print(err)
					return err
				}
				storageI := storage.GetStorage(storageType)
				err = storageI.MakeFromJson(storageJson)
				if err == nil {
					err = msi.SaveStorage(storageI, prj, nil, false)
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
