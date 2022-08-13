package aes

import (
	caes "crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
)

type AesKey struct {
	KeyHex string `json:"key_string" eru:"required"`
	Key    []byte `json:"key" eru:"required"`
	Bits   int    `json:"bits" eru:"required"`
}

type ecb struct {
	block     cipher.Block
	blockSize int
}

func newECB(block cipher.Block) *ecb {
	return &ecb{
		block:     block,
		blockSize: block.BlockSize(),
	}
}

type ecbEncrypter ecb

func NewECBEncrypter(block cipher.Block) cipher.BlockMode {
	return (*ecbEncrypter)(newECB(block))
}

func (e *ecbEncrypter) BlockSize() int { return e.blockSize }

func (e *ecbEncrypter) CryptBlocks(dst, src []byte) {
	if len(src)%e.blockSize != 0 {
		panic("crypto/cipher: input blocks are not full")
	}
	if len(dst) < len(src) {
		panic("crypto/cipher: output is smaller than input")
	}
	for len(src) > 0 {
		e.block.Encrypt(dst, src[:e.blockSize])
		src = src[e.blockSize:]
		dst = dst[e.blockSize:]
	}
}

type ecbDecrypter ecb

func NewECBDecrypter(block cipher.Block) cipher.BlockMode {
	return (*ecbDecrypter)(newECB(block))
}

func (d *ecbDecrypter) BlockSize() int { return d.blockSize }

func (d *ecbDecrypter) CryptBlocks(dst, src []byte) {
	log.Println("len(src) = ", len(src))
	log.Println("d.blockSize = ", d.blockSize)
	if len(src)%d.blockSize != 0 {
		panic("crypto/cipher: input blocks are not full")
	}
	if len(dst) < len(src) {
		panic("crypto/cipher: output is smaller than input")
	}
	for len(src) > 0 {
		d.block.Decrypt(dst, src[:d.blockSize])
		src = src[d.blockSize:]
		dst = dst[d.blockSize:]
	}
}

func GenerateKey(bits int) (aesKey AesKey, err error) {
	log.Println("inside GenerateKey")

	aesKey.Bits = bits
	bytes := make([]byte, bits/8) //generate a random xx byte key for AES-256
	if _, err := rand.Read(bytes); err != nil {
		log.Println(err.Error())
	}
	aesKey.KeyHex = hex.EncodeToString(bytes)
	aesKey.Key = bytes
	log.Println(bytes)
	log.Println(string(bytes))
	return
}

func EncryptECB(plainBytes []byte, aesKey []byte) (encryptedBytes []byte, err error) {
	block, err := caes.NewCipher(aesKey)
	if err != nil {
		log.Println(err.Error())
		return
	}
	log.Println("len(plainBytes) = ", len(plainBytes))
	log.Println(fmt.Sprint(plainBytes))
	plaintext := Pad(plainBytes, block.BlockSize())
	log.Println("len(plaintext) = ", len(plaintext))
	log.Println(string(plaintext))

	ecbObj := NewECBEncrypter(block)
	encryptedBytes = make([]byte, len(plaintext))
	ecbObj.CryptBlocks(encryptedBytes, plaintext)
	log.Println("len(encryptedBytes) = ", len(encryptedBytes))
	log.Println("len(plaintext) = ", len(plaintext))
	/*ecbdObj := NewECBDecrypter(block)
	plainBytesBeforeUnpad := make([]byte,len(encryptedBytes))
	log.Println("len(plainBytesBeforeUnpad) = ",len(plainBytesBeforeUnpad))
	ecbdObj.CryptBlocks(plainBytesBeforeUnpad,encryptedBytes)
	plainBytes, err = Unpad(plainBytesBeforeUnpad)
	if err != nil {
		log.Println(err.Error())
		return
	}
	log.Println(string(plainBytes))
	*/
	return encryptedBytes, err
}

func DecryptECB(encryptedBytes []byte, aesKey []byte) (plainBytes []byte, err error) {
	log.Println("len(encryptedBytes) = ", len(encryptedBytes))
	block, err := caes.NewCipher(aesKey)
	if err != nil {
		log.Println(err.Error())
		return
	}

	ecbdObj := NewECBDecrypter(block)
	plainBytesBeforeUnpad := make([]byte, len(encryptedBytes))
	log.Println("len(plainBytesBeforeUnpad) = ", len(plainBytesBeforeUnpad))
	ecbdObj.CryptBlocks(plainBytesBeforeUnpad, encryptedBytes)
	plainBytes, err = Unpad(plainBytesBeforeUnpad)
	if err != nil {
		log.Println(err.Error())
		return
	}
	return plainBytes, err
}

func EncryptCBC(plainText []byte, bKey []byte, bIV []byte) (encryptedString []byte, err error) {
	block, err := caes.NewCipher(bKey)
	if err != nil {
		log.Print(err)
		return
	}

	bCipherText := make([]byte, len(plainText))
	mode := cipher.NewCBCEncrypter(block, bIV)
	mode.CryptBlocks(bCipherText, (plainText))
	return bCipherText, nil
}
func DecryptCBC(cipherText []byte, bKey []byte, bIV []byte) (decryptedString []byte, err error) {
	block, err := caes.NewCipher(bKey)
	if err != nil {
		log.Print(err)
		return
	}
	bPlaintext := make([]byte, len(cipherText))
	mode := cipher.NewCBCDecrypter(block, bIV)
	mode.CryptBlocks(bPlaintext, cipherText)
	return bPlaintext, nil
}

/*func EncryptCBC(plainText string, encKey string, iv string) (encryptedString string) {
	bKey := []byte(encKey)
	bIV := []byte(iv)
	//cipherTextDecoded, err := hex.DecodeString(cipherText)
	//if err != nil {
	//	panic(err)
	//}
	block, err := caes.NewCipher(bKey)
	if err != nil {
		panic(err)
	}

	log.Print("block..BlockSize() = ",block.BlockSize())

	bCipherText := make([]byte, len(plainText))
	mode := cipher.NewCBCEncrypter(block, bIV)
	mode.CryptBlocks([]byte(bCipherText), []byte(plainText))
	log.Print(bCipherText)
	log.Print(string(bCipherText))
	return string(bCipherText)
}

func DecryptCBC(cipherText string, encKey string, iv string) (decryptedString string) {
	bKey := []byte(encKey)
	bIV := []byte(iv)
	//cipherTextDecoded, err := hex.DecodeString(cipherText)
	//if err != nil {
	//	panic(err)
	//}

	block, err := caes.NewCipher(bKey)
	if err != nil {
		panic(err)
	}
	bPlaintext := make([]byte, len(cipherText))
	mode := cipher.NewCBCDecrypter(block, bIV)
	mode.CryptBlocks([]byte(bPlaintext), []byte(cipherText))
	return strings.Trim(string(bPlaintext), " ")
}

*/

func Encrypt(plainBytes []byte, aesKeyStr string) (encryptedBytes []byte, err error) {
	log.Print("aesKeyStr = ", aesKeyStr)
	//key, err := hex.DecodeString(aesKeyStr)
	//log.Print("error from hex.DecodeString(aesKeyStr) ", err.Error())
	//log.Print(key)
	key := []byte(aesKeyStr)
	block, err := caes.NewCipher(key)
	if err != nil {
		log.Println("error from caes.NewCipher(key)")
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
	log.Print(key)
	//enc, err := hex.DecodeString(string(encryptedBytes))
	if err != nil {
		log.Println(err.Error())
		return
	}

	//Create a new Cipher Block from the key
	block, err := caes.NewCipher(key)
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
