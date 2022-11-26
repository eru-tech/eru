package aes

import (
	"encoding/hex"
	"github.com/eru-tech/eru/eru-crypto/md5"
	"log"
	"testing"
)

func TestAESEncryptCBC(t *testing.T) {
	plainTextBeforePadding := "merchant_id=1067144&order_id=12345&amount=1.00&currency=INR&redirect_url=www.abc.com&cancel_url=www.abc.com&billing_name=test&billing_address=test&billing_city=Mumbai&billing_state=Maharashtra&billing_zip=123456&billing_country=India&billing_tel=1234567890&billing_email=test@hdfcbank.com&delivery_name=test&delivery_address=test&delivery_city=Mumbai&delivery_state=Maharashtra&delivery_zip=123456&delivery_country=India&delivery_tel=1234567890&merchant_param1=udf1&merchant_param2=udf2&merchant_param3=udf3&merchant_param4=udf4&merchant_param5=udf5"
	_ = plainTextBeforePadding
	//plainTextBeforePadding := "Hello PHP Hello PHP Hello PHP Hello P Hello PHP Hello PHP Hello PHP Hello P Hello PHP Hello PHP Hello PHP Hello P Hello PHP Hello PHP Hello PHP Hello P"
	org_aes_key := "8509504EDC99A001A3A8C912ABEE1050"
	aes_key, _ := md5.Md5(org_aes_key, "string")
	iv := "\u0000\u0001\u0002\u0003\u0004\u0005\u0006\u0007\u0008\u0009\u000a\u000b\u000c\u000d\u000e\u000f"
	/*
		plainTextBytes := Pad([]byte(plainTextBeforePadding), 16)
		plainText := string(plainTextBytes)
		log.Print("plaintext after padding = ", plainText)
		encryptedString, _ := EncryptCBC([]byte(plainText), []byte(aes_key), []byte(iv))
		log.Print("encryptedString")
		log.Print(encryptedString)
		log.Print(len(encryptedString))

		encryptedHexString := hex.EncodeToString([]byte(encryptedString))
		log.Print(encryptedHexString)
	*/

	encryptedHexString := "64b27a5c7751b8227e6f4c4bcc5abc4ea9d68ca41dc353c1a59d8dec07e2a75e85b705f13fe8d6cd900bd02d5eaac0f0349980014d6b0a6a996a9e387dc59346bb6c1ccdc8151acbba1979671d7e70227f9c80092a96d2808ce3e93175d63b2b584d3b6ebf5fcf138f47e58d1d5bc97eaf1d6f86a8c332fb45d58779d1a312af00010af6330f6e8122f497ce6856150816d0f56eac6aaa4b5cd104742c7b2098652847d9e68622dc64bd4e7039d15a2a5324c679b8084007bc61cc267824b47fcc85c466c8aa5d42d9dd0008ed0ee0fe44197f2867854f64f98129bccd9486c0d387c62145405e7efa73965f61da2dbb542966e2afd4670c74f040316012a595eef4ed4d5b8e28c4a71f1646b422d58b5f858f3f4e194069f2cfbea18cbb5fcc6d4bc407cffef049085f610c196b9dc5994df895e058482b766fb9e0a84d6ebe5cbcfba86a8ec36e430ac73c69d6e118669cd754b4a7760b0b6be4e821bdcc40a2c21cf7ef75acb93b4846842077a84a"
	toDecrypt, _ := hex.DecodeString(encryptedHexString)
	decryptedString, _ := DecryptCBC(toDecrypt, []byte(aes_key), []byte(iv))
	log.Print("decryptedString")
	decryptedStringUnpadded, _ := Unpad([]byte(decryptedString))
	log.Print(string(decryptedStringUnpadded))

}
