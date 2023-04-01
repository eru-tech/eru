package rsa

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

type RsaKeyPair struct {
	PrivateKey string `json:"private_key" eru:"required"`
	PublicKey  string `json:"public_key" eru:"required"`
	Bits       int    `json:"bits" eru:"required"`
}

func GenerateKeyPair(ctx context.Context, bits int) (rsaKeyPair RsaKeyPair, err error) {
	logs.WithContext(ctx).Debug("GenerateKeyPair - Start")
	rsaKeyPair.Bits = bits
	// generate key
	privateKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprint("Failed to generate RSA key : ", err.Error()))
		return
	}
	publicKey := &privateKey.PublicKey

	// save private key to a file
	var privateKeyBytes []byte = x509.MarshalPKCS1PrivateKey(privateKey)

	privateKeyBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	}

	pk := pem.EncodeToMemory(privateKeyBlock)

	rsaKeyPair.PrivateKey = string(pk)

	// save public key to file
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprint("failed to dump public key: ", err))
		return
	}

	publicKeyBlock := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	}

	pubk := pem.EncodeToMemory(publicKeyBlock)

	rsaKeyPair.PublicKey = string(pubk)

	return
}

func EncryptWithCert(ctx context.Context, plainBytes []byte, publicCert string) (encryptedBytes []byte, err error) {
	logs.WithContext(ctx).Debug("EncryptWithCert - Start")
	block, _ := pem.Decode([]byte(publicCert))
	if block == nil {
		err = errors.New("failed to parse PEM block containing the key")
		logs.WithContext(ctx).Error(err.Error())
		return
	}

	var cert *x509.Certificate
	cert, _ = x509.ParseCertificate(block.Bytes)
	rsaPublicKey := cert.PublicKey.(*rsa.PublicKey)
	return Encrypt(ctx, plainBytes, rsaPublicKey)
}

func EncryptWithKey(ctx context.Context, plainBytes []byte, publicKeyStr string) (encryptedBytes []byte, err error) {
	logs.WithContext(ctx).Debug("EncryptWithKey - Start")
	block, _ := pem.Decode([]byte(publicKeyStr))
	if block == nil {
		err = errors.New("failed to parse PEM block containing the key")
		logs.WithContext(ctx).Error(err.Error())
		return
	}
	publicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	rsaPublicKey, ok := publicKey.(*rsa.PublicKey)
	if !ok {
		err = errors.New("Value returned from ParsePKIXPublicKey was not an RSA public key")
		logs.WithContext(ctx).Error(err.Error())
		return
	}
	return Encrypt(ctx, plainBytes, rsaPublicKey)
}

func Encrypt(ctx context.Context, plainBytes []byte, rsaPublicKey *rsa.PublicKey) (encryptedBytes []byte, err error) {
	logs.WithContext(ctx).Debug("Encrypt - Start")

	encryptedBytes = make([]byte, 0, len(plainBytes))
	for i := 0; i < len(plainBytes); i += 117 {
		if i+117 < len(plainBytes) {
			partial, err1 := rsa.EncryptPKCS1v15(rand.Reader, rsaPublicKey, plainBytes[i:i+117])
			if err1 != nil {
				logs.WithContext(ctx).Error(err1.Error())
			}
			encryptedBytes = append(encryptedBytes, partial...)
		} else {
			partial, err1 := rsa.EncryptPKCS1v15(rand.Reader, rsaPublicKey, plainBytes[i:])
			if err1 != nil {
				logs.WithContext(ctx).Error(err1.Error())
			}
			encryptedBytes = append(encryptedBytes, partial...)
		}
	}
	return
}

func Decrypt(ctx context.Context, encryptedBytes []byte, privateKeyStr string) (decryptedBytes []byte, err error) {
	logs.WithContext(ctx).Debug("Decrypt - Start")
	block, _ := pem.Decode([]byte(privateKeyStr))
	if block == nil {
		err = errors.New("failed to parse PEM block containing the key")
		logs.WithContext(ctx).Error(err.Error())
		return
	}
	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return
	}

	decryptedBytes, err = privateKey.Decrypt(nil, encryptedBytes, &rsa.OAEPOptions{Hash: crypto.SHA256})
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return
	}
	return
}
