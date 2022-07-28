package aes

import (
	b64 "encoding/base64"
	"fmt"
	"log"
	"strings"
	"testing"
)

func TestAESDecryptCBC(t *testing.T) {
	//encyptedBase64Str := "bGtzZGFzZnNhc2tsbG9wbBpU1Y0wlp3dYU9muV5XTQCwg+vYcdM/fZkFM3mhAUVqlDM/5ulZnt/aqsfGRgQKrA=="
	encyptedBase64Str := "bGtzZGFzZnNhc2tsbG9wbPgjjiyZf45DW58MW+L/dQCDWftYOKP66r4Q1Y77tREvOwXAZCtkaQfahCHQnPnAkg==\n"

	encyptedBytes, err := b64.StdEncoding.DecodeString(encyptedBase64Str)
	if err != nil {
		log.Println(err)
	}
	encyptedStr := string(encyptedBytes)
	log.Print(len(encyptedStr))
	actualEncyptedStr := encyptedStr[48:len(encyptedStr)]
	decryptedString := DecryptCBC(actualEncyptedStr, fmt.Sprint("CRDRIAF", strings.Repeat("\000", 9)), "lksdasfsaskllopl")
	log.Print(decryptedString)
}
