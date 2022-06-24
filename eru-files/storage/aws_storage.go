package storage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	eruaes "github.com/eru-tech/eru/eru-crypto/aes"
	"github.com/segmentio/ksuid"
	"io/ioutil"
	"log"
	"mime/multipart"
)

type AwsStorage struct {
	Storage
	Region         string `json:"region" eru:"required"`
	BucketName     string `json:"bucket_name" eru:"required"`
	Authentication string `json:"authentication" eru:"required"`
	Key            string `json:"key" eru:"required"`
	Secret         string `json:"secret" eru:"required"`
	session        *session.Session
}

func (awsStorage *AwsStorage) UploadFile(file multipart.File, header *multipart.FileHeader, docType string, folderPath string, keyName eruaes.AesKey) (docId string, err error) {
	log.Println("inside AwsStorage UploadFile")
	if awsStorage.session == nil {
		err = awsStorage.Init()
		if err != nil {
			log.Println(err)
			return
		}
	}
	var byteContainer []byte
	enc := ""
	byteContainer, err = ioutil.ReadAll(file)
	if err != nil {
		log.Println(err)
		return
	}
	log.Println("size = ", len(byteContainer))
	log.Println("file.Read(byteContainer)")
	log.Println(file.Read(byteContainer))

	log.Println(awsStorage.EncryptFiles)

	if awsStorage.EncryptFiles {
		byteContainer, err = eruaes.Encrypt(byteContainer, string(keyName.Key))
		if err != nil {
			log.Println(err)
			return
		}
		enc = ".enc"
	}
	docId = ksuid.New().String()
	log.Println(docType)
	if docType != "" {
		docType = fmt.Sprint(docType, "_")
	}
	log.Println(docType)
	finalFileName := fmt.Sprint(docType, docId, "_", header.Filename, enc)
	uploader := s3manager.NewUploader(awsStorage.session)
	_, err = uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(awsStorage.BucketName),
		Key:    aws.String(fmt.Sprint(folderPath, "/", finalFileName)),
		Body:   bytes.NewReader(byteContainer),
	})
	return
}

func (awsStorage *AwsStorage) Init() (err error) {
	log.Println("inside AwsStorage Init")
	awsConf := &aws.Config{
		Region: aws.String(awsStorage.Region),
		Credentials: credentials.NewStaticCredentials(
			awsStorage.Key,
			awsStorage.Secret,
			"", // a token will be created when the session it's used. //TODO to check this
		),
		//TODO to check if below 2 attributes are required
		//DisableSSL: &disableSSL,
		//S3ForcePathStyle: &forcePathStyle,
	}
	awsStorage.session, err = session.NewSession(awsConf)
	return err
}

func (awsStorage *AwsStorage) MakeFromJson(rj *json.RawMessage) error {
	log.Println("inside AwsStorage MakeFromJson")
	err := json.Unmarshal(*rj, &awsStorage)
	if err != nil {
		log.Print("error json.Unmarshal(*rj, &awsStorage)")
		log.Print(err)
		return err
	}
	log.Println(awsStorage)
	return nil
}
