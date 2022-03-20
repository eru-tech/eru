package rsa

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"log"
)

type RsaKeyPair struct {
	PrivateKey string `json:"private_key" eru:"required"`
	PublicKey  string `json:"public_key" eru:"required"`
	Bits       int    `json:"bits" eru:"required"`
}

func GenerateKeyPair(bits int) (rsaKeyPair RsaKeyPair, err error) {
	log.Println("inside GenerateKeyPair of crypto lib")
	rsaKeyPair.Bits = bits
	// generate key
	privateKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		log.Println("Failed to generate RSA key")
		return
	}
	publicKey := &privateKey.PublicKey

	// save private key to a file
	var privateKeyBytes []byte = x509.MarshalPKCS1PrivateKey(privateKey)

	privateKeyBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	}

	//privatePem, err := os.Create("private.pem")
	//if err != nil {
	//	log.Println("failed to create private.pem: ", err)
	//	return
	//}
	pk := pem.EncodeToMemory(privateKeyBlock)
	//if err != nil {
	//	log.Println("failed to encode private.pem: ", err)
	//	return
	//}

	rsaKeyPair.PrivateKey = string(pk)

	/*block, _ := pem.Decode(pk)
	if block == nil {
		log.Println("failed to parse PEM block containing the key")
	}
	log.Println(string(pk))
	privateKey2,err2 := x509.ParsePKCS1PrivateKey(block.Bytes)
	log.Println("err2 below")
	log.Println(err2)
	log.Println("privateKey2 below")
	log.Println(privateKey2)
	*/

	// save public key to file
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		log.Println("failed to dump public key: ", err)
		return
	}

	publicKeyBlock := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	}
	//publicPem, err := os.Create("public.pem")
	//if err != nil {
	//	log.Println("failed to create public.pem: ", err)
	//	return
	//}
	pubk := pem.EncodeToMemory(publicKeyBlock)
	//if err != nil {
	//	log.Println("failed to encode public.pem: ", err)
	//	return
	//}
	rsaKeyPair.PublicKey = string(pubk)

	/*block1, _ := pem.Decode(pubk)
	if block1 == nil {
		log.Println("failed to parse PEM block containing the key")
	}
	log.Println(string(pubk))
	//publicKey1,err1 := x509.ParsePKCS1PublicKey(block1.Bytes)
	publicKey1,err1 := x509.ParsePKIXPublicKey(block1.Bytes)
	log.Println("err1 below")
	log.Println(err1)
	log.Println("publicKey1 below")
	log.Println(publicKey1)
	*/

	return
}

func Encrypt(plainBytes []byte, publicKeyStr string) (encryptedBytes []byte, err error) {
	block, _ := pem.Decode([]byte(publicKeyStr))
	if block == nil {
		err = errors.New("failed to parse PEM block containing the key")
		log.Println(err)
		return
	}
	publicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	rsaPublicKey, ok := publicKey.(*rsa.PublicKey)
	if !ok {
		err = errors.New("Value returned from ParsePKIXPublicKey was not an RSA public key")
		log.Println(err)
		return
	}
	encryptedBytes, err = rsa.EncryptOAEP(
		sha256.New(),
		rand.Reader,
		rsaPublicKey,
		plainBytes,
		nil)
	if err != nil {
		log.Println(err)
		return
	}
	return
}

func Decrypt(encryptedBytes []byte, privateKeyStr string) (decryptedBytes []byte, err error) {
	block, _ := pem.Decode([]byte(privateKeyStr))
	if block == nil {
		err = errors.New("failed to parse PEM block containing the key")
		log.Println(err)
		return
	}
	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		log.Println(err)
		return
	}

	decryptedBytes, err = privateKey.Decrypt(nil, encryptedBytes, &rsa.OAEPOptions{Hash: crypto.SHA256})
	if err != nil {
		log.Println(err)
		return
	}
	return
}
