package aes

// this file implements PKCS#7 padding, as defined in RFC 5652.
//
// https://tools.ietf.org/html/rfc5652#section-6.3

import (
	"bytes"
	"crypto/subtle"
	"errors"
	"fmt"
	"log"
)

var errPKCS7Padding = errors.New("bad padding for pkcs7pad")

func Pad(buf []byte, size int) []byte {
	if size < 1 || size > 255 {
		panic(fmt.Sprintf("inappropriate pkcs7pad block size %d", size))
	}
	i := size - (len(buf) % size)
	log.Println("i = ", i)
	return append(buf, bytes.Repeat([]byte{byte(i)}, i)...)
}

func Unpad(buf []byte) ([]byte, error) {
	if len(buf) == 0 {
		return nil, errPKCS7Padding
	}

	padLen := buf[len(buf)-1]
	toChk := 255
	gd := 1
	if toChk > len(buf) {
		toChk = len(buf)
	}
	for i := 0; i < toChk; i++ {
		b := buf[len(buf)-1-i]

		outOfRange := subtle.ConstantTimeLessOrEq(int(padLen), i)
		eq := subtle.ConstantTimeByteEq(padLen, b)
		gd &= subtle.ConstantTimeSelect(outOfRange, 1, eq)
	}

	gd &= subtle.ConstantTimeLessOrEq(1, int(padLen))
	gd &= subtle.ConstantTimeLessOrEq(int(padLen), len(buf))

	if gd != 1 {
		return nil, errPKCS7Padding
	}

	return buf[:len(buf)-int(padLen)], nil
}
