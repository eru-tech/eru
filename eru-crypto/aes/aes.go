package aes

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
)

type AesKey struct {
	Key  string `json:"key" eru:"required"`
	Bits int    `json:"bits" eru:"required"`
}

func GenerateKey(bits int) (aesKey AesKey, err error) {
	aesKey.Bits = bits
	bytes := make([]byte, bits/8) //generate a random xx byte key for AES-256
	if _, err := rand.Read(bytes); err != nil {
		panic(err.Error())
	}
	aesKey.Key = hex.EncodeToString(bytes)
	return
}

func Encrypt(plainBytes []byte, aesKeyStr string) (encryptedBytes []byte, err error) {
	key, _ := hex.DecodeString(aesKeyStr)
	block, err := aes.NewCipher(key)
	if err != nil {
		log.Println(err.Error())
		return
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		log.Println(err.Error())
		return
	}
	nonce := make([]byte, aesGCM.NonceSize())
	encryptedBytes = aesGCM.Seal(nonce, nonce, plainBytes, nil)
	return
}

func Decrypt(encryptedBytes []byte, aesKeyStr string) (decryptedBytes []byte, err error) {
	key, _ := hex.DecodeString(aesKeyStr)
	//enc, err := hex.DecodeString(string(encryptedBytes))
	if err != nil {
		log.Println(err.Error())
		return
	}

	//Create a new Cipher Block from the key
	block, err := aes.NewCipher(key)
	if err != nil {
		log.Println(err.Error())
		return
	}

	//Create a new GCM
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		log.Println(err.Error())
		return
	}

	//Get the nonce size
	nonceSize := aesGCM.NonceSize()
	log.Println("len(encryptedBytes) = ", len(encryptedBytes))
	if len(encryptedBytes) < nonceSize {
		err = errors.New("length of encryptedBytes is less then nonce size")
		log.Println(err)
		return
	}
	log.Println("nonceSize = ", nonceSize)
	//Extract the nonce from the encrypted data
	nonce, ciphertext := encryptedBytes[:nonceSize], encryptedBytes[nonceSize:]

	//Decrypt the data
	decryptedBytes, err = aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		log.Println(err.Error())
		return
	}
	return
}
