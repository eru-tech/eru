package aes

import (
	"crypto/sha256"
	b64 "encoding/base64"
	"fmt"
	"log"
	"strings"
	"testing"
)

func NewSHA256(data []byte) []byte {
	hash := sha256.Sum256(data)
	//log.Println(string(hash[:]))
	return hash[:]
}

func TestAESDecryptCBC(t *testing.T) {
	aes_key := fmt.Sprint("CRDRIAF", strings.Repeat("\000", 9))
	iv := "lksdasfsaskllopl"
	//encyptedBase64Str := "bGtzZGFzZnNhc2tsbG9wbBpU1Y0wlp3dYU9muV5XTQCwg+vYcdM/fZkFM3mhAUVqlDM/5ulZnt/aqsfGRgQKrA=="
	encyptedBase64Str := "bGtzZGFzZnNhc2tsbG9wbG7tZD/+qEbWLRnSE2MB1I+UmlUqj7QxhHzoDVnfbopnDs0n17PoINyV9IZyGOa/Fg=="
	encyptedBytes, err := b64.StdEncoding.DecodeString(encyptedBase64Str)
	if err != nil {
		log.Println(err)
	}
	encyptedStr := string(encyptedBytes)
	log.Print(len(encyptedStr))
	actualEncyptedStr := encyptedStr[48:len(encyptedStr)]
	decryptedString := DecryptCBC(actualEncyptedStr, aes_key, iv)
	log.Print(strings.TrimSpace(decryptedString))
}
func TestAESEncryptCBC(t *testing.T) {
	org_aes_key := "CRDRIAF"
	aes_key := fmt.Sprint(org_aes_key, strings.Repeat("\000", 9))
	//aes_key := fmt.Sprint("CRDRIAF", strings.Repeat((" "), 9))
	iv := "lksdasfsaskllopl"
	plainTextBeforePadding := "Hello World"
	plainTextBytes, _ := Pkcs7Pad([]byte(plainTextBeforePadding), 16)
	plainText := string(plainTextBytes)
	log.Print("plaintext after padding = ", plainText)
	encryptedString := EncryptCBC(plainText, aes_key, iv)
	log.Print("encryptedString")
	log.Print(encryptedString)
	log.Print(len(encryptedString))
	encryptedStringb64 := b64.StdEncoding.EncodeToString([]byte(encryptedString))
	log.Print(encryptedStringb64)
	//just checking if decrypting well
	decryptedString := DecryptCBC(encryptedString, aes_key, iv)
	log.Print(decryptedString)

	//plainTextBytes, err := b64.StdEncoding.DecodeString(decryptedString)
	//if err != nil {
	//	log.Println(err)
	//}
	//log.Print(string(plainTextBytes))
	//

	hash_map := string(NewSHA256([]byte(fmt.Sprint(encryptedString, org_aes_key))))
	log.Print("hash_map")
	log.Print(hash_map)
	log.Print(len(hash_map))
	encodedFinalValue := b64.StdEncoding.EncodeToString([]byte(fmt.Sprint(iv, hash_map, encryptedString)))
	log.Print(encodedFinalValue)
	//checking with actual decryption process
	encyptedBytes, err := b64.StdEncoding.DecodeString(encodedFinalValue)
	if err != nil {
		log.Println(err)
	}
	encyptedStr := string(encyptedBytes)
	log.Print(len(encyptedStr))
	log.Print(encyptedStr)
	actualEncyptedStr := encyptedStr[48:len(encyptedStr)]
	log.Print(actualEncyptedStr)
	decryptedFinalString := DecryptCBC(actualEncyptedStr, aes_key, iv)
	log.Print("decryptedFinalString")
	log.Print(decryptedFinalString)
}

/*

func md5hash (b string) string {
	hash := md5.Sum([]byte(b))
	//log.Print(b)
	//log.Print(string(hash[:]))
	//log.Print(hex.EncodeToString(hash[:]))
	return string(hash[:])
}

func stringToBin(s string) (binString string) {
	res := ""
	for _, c := range s {
		res = fmt.Sprintf("%s%.8b", res, c)
	}
	return res
}

func TestAESDecryptCBC(t *testing.T) {
	aes_key := fmt.Sprint("CRDRIAF", strings.Repeat("\000", 9))
	aes_key_hash := md5hash(aes_key)
	iv := "lksdasfsaskllopl"

	//encyptedBase64Str := "bGtzZGFzZnNhc2tsbG9wbBpU1Y0wlp3dYU9muV5XTQCwg+vYcdM/fZkFM3mhAUVqlDM/5ulZnt/aqsfGRgQKrA=="
	encyptedBase64Str := "bGtzZGFzZnNhc2tsbG9wbPgjjiyZf45DW58MW+L/dQCDWftYOKP66r4Q1Y77tREvOwXAZCtkaQfahCHQnPnAkg==\n"

	encyptedBytes, err := b64.StdEncoding.DecodeString(encyptedBase64Str)
	if err != nil {
		log.Println(err)
	}
	encyptedStr := string(encyptedBytes)
	log.Print(len(encyptedStr))
	actualEncyptedStr := encyptedStr[48:len(encyptedStr)]
	decryptedString := DecryptCBC(actualEncyptedStr, aes_key_hash,iv)
	log.Print(strings.TrimSpace(decryptedString))
}

func TestAESEncryptCBC(t *testing.T) {
	log.Print("inside TestAESEncryptCBC")
	//aes_key := fmt.Sprint("CRDRIAF", strings.Repeat("\000", 9))
	aes_key:="8509504EDC99A001A3A8C912ABEE1050";
	aes_key_hash := md5hash(aes_key)
	log.Print(aes_key_hash)
	//log.Print(stringToBin(aes_key_hash))
	iv := "lksdasfsaskllopl"
	plainText := "Hello World"

	encryptedString := EncryptCBC(plainText, aes_key_hash, iv)
	log.Print("encryptedString")
	log.Print(encryptedString)
	log.Print(len(encryptedString))

	log.Print("hex of encryption")
	log.Print(hex.EncodeToString([]byte(encryptedString)))

	//just checking if decrypting well
	decryptedString := DecryptCBC(encryptedString, aes_key_hash, iv)
	log.Print(decryptedString)
	plainTextBytes, err := b64.StdEncoding.DecodeString(decryptedString)
	if err != nil {
		log.Println(err)
	}
	log.Print(string(plainTextBytes))
	//

	hash_map := string(NewSHA256([]byte(fmt.Sprint(encryptedString,aes_key_hash))))
	log.Print("hash_map")
	log.Print(hash_map)
	log.Print(len(hash_map))

	encodedFinalValue := b64.StdEncoding.EncodeToString([]byte(fmt.Sprint(iv,hash_map,encryptedString)))
	log.Print(encodedFinalValue)

	//checking with actual decryption process
	encyptedBytes, err := b64.StdEncoding.DecodeString(encodedFinalValue)
	if err != nil {
		log.Println(err)
	}
	encyptedStr := string(encyptedBytes)
	log.Print(len(encyptedStr))
	log.Print(encyptedStr)
	actualEncyptedStr := encyptedStr[48:len(encyptedStr)]
	log.Print(actualEncyptedStr)

	decryptedFinalString := DecryptCBC(actualEncyptedStr, aes_key_hash,iv)
	log.Print("decryptedFinalString")
	log.Print(decryptedFinalString)

	phpEncryptedText := "8f250a1d9fa46a2f14a95cf6605a89cc"
	phpEncryptedText1 , _ := hex.DecodeString(phpEncryptedText)
	phpEncryptedText2 := string(phpEncryptedText1)
	phpDdecryptedFinalString := DecryptCBC(phpEncryptedText2, aes_key_hash,iv)
	log.Print("phpDdecryptedFinalString")
	log.Print(phpEncryptedText1)
	log.Print(phpEncryptedText2)
	log.Print(phpDdecryptedFinalString)

}

func TestAESEncryptCBC(t *testing.T) {
	aes_key:="8509504EDC99A001A3A8C912ABEE1050";
	aes_key_hash := md5hash(aes_key)
	//iv := "lksdasfsaskllopl"
	iv := "0123456789abcdef"
	//plainText := "Hello World"
	plainText := "language=EN&customer_identifier=cust1&merchant_id=1067144&order_id=1&currency=INR&amount=1&redirect_url=https://devgw.api.artfine.in/artfine/route/callback/&cancel_url=https://devgw.api.artfine.in/artfine/route/callback/&"
	finalPlainText := pad(plainText,16)
	log.Print(finalPlainText)
	goEnecryptedString := EncryptCBC(finalPlainText, aes_key_hash,iv)
	log.Print("goEnecryptedString")
	log.Print(goEnecryptedString)
	goEnecryptedString1 := hex.EncodeToString([]byte(goEnecryptedString))
	log.Print(goEnecryptedString1)

	goEncryptedText1 , _ := hex.DecodeString(goEnecryptedString1)
	goEncryptedText2 := string(goEncryptedText1)
	goDdecryptedFinalString := DecryptCBC(goEncryptedText2, aes_key_hash,iv)
log.Print(goDdecryptedFinalString)


	phpEncryptedText := "8f250a1d9fa46a2f14a95cf6605a89cc"
	phpEncryptedText1 , _ := hex.DecodeString(phpEncryptedText)
	phpEncryptedText2 := string(phpEncryptedText1)
	phpDdecryptedFinalString := DecryptCBC(phpEncryptedText2, aes_key_hash,iv)
	log.Print("phpDdecryptedFinalString")
	log.Print(phpEncryptedText1)
	log.Print(phpEncryptedText2)
	log.Print(phpDdecryptedFinalString)
}

func pad(plainText string, size int) (str string){
	//paddding with spaces
	plainBuf := []byte(plainText)
	i := size - (len(plainBuf) % size)
	spaceByte := []byte(fmt.Sprint(i))
	finalPlainBytes := append(plainBuf, bytes.Repeat(spaceByte, i)...)
	str = string(finalPlainBytes)
	//paddding with spaces ended
	return
}
*/
