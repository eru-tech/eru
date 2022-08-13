package md5

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
)

func Md5(str string, output string) (string, error) {
	h := md5.Sum([]byte(str))
	log.Println(string(h[:]))
	if output == "hex" {
		return hex.EncodeToString(h[:]), nil
	} else if output == "string" {
		return string(h[:]), nil
	}
	return "", errors.New(fmt.Sprint("error - unknow output", output))
}
