package aes

import (
	"encoding/hex"
	"github.com/eru-tech/eru/eru-crypto/md5"
	"log"
	"testing"
)

func TestAESEncryptCBC(t *testing.T) {
	plainTextBeforePadding := "merchant_id=1067144&order_id=12345&amount=1.00&currency=INR&redirect_url=www.abc.com&cancel_url=www.abc.com&billing_name=test&billing_address=test&billing_city=Mumbai&billing_state=Maharashtra&billing_zip=123456&billing_country=India&billing_tel=1234567890&billing_email=test@hdfcbank.com&delivery_name=test&delivery_address=test&delivery_city=Mumbai&delivery_state=Maharashtra&delivery_zip=123456&delivery_country=India&delivery_tel=1234567890&merchant_param1=udf1&merchant_param2=udf2&merchant_param3=udf3&merchant_param4=udf4&merchant_param5=udf5"
	//plainTextBeforePadding := "Hello PHP Hello PHP Hello PHP Hello P Hello PHP Hello PHP Hello PHP Hello P Hello PHP Hello PHP Hello PHP Hello P Hello PHP Hello PHP Hello PHP Hello P"
	org_aes_key := "8509504EDC99A001A3A8C912ABEE1050"
	aes_key, _ := md5.Md5(org_aes_key, "string")
	iv := "\u0000\u0001\u0002\u0003\u0004\u0005\u0006\u0007\u0008\u0009\u000a\u000b\u000c\u000d\u000e\u000f"
	plainTextBytes := Pad([]byte(plainTextBeforePadding), 16)
	plainText := string(plainTextBytes)
	log.Print("plaintext after padding = ", plainText)
	encryptedString, _ := EncryptCBC([]byte(plainText), []byte(aes_key), []byte(iv))
	log.Print("encryptedString")
	log.Print(encryptedString)
	log.Print(len(encryptedString))

	encryptedHexString := hex.EncodeToString([]byte(encryptedString))
	log.Print(encryptedHexString)
	encryptedHexString = "99022951d0bd337cce78c9166465258bee5feea39def6ddb60ad94d855fe60261594059ee70062e9ac456b66b99ebcf1c8e68c3b05c7f7cdd05c53e737f2076f4f32042e1e765770606142cc25ccdeab54e94348263f151a5446a9a45023f7df6d0b5f973735146951ea007c4a5fb508eae907f1e2d711b0aa7948b0df41b8c246575f084c91dea678266189cba2c77087f56aecf1328545047324ff23aa42ee1406206fe51303d5bd7d3db2e03ff0af2ff1861d66475b259e608ac9407fa245ed9768bc9efffd8d2b144c24a270007e853ebe16efca897c00a2abf8949df3ad"

	toDecrypt, _ := hex.DecodeString(encryptedHexString)
	decryptedString, _ := DecryptCBC(toDecrypt, []byte(aes_key), []byte(iv))
	log.Print("decryptedString")
	decryptedStringUnpadded, _ := Unpad([]byte(decryptedString))
	log.Print(string(decryptedStringUnpadded))

}
