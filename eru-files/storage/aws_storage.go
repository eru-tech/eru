package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	eruaes "github.com/eru-tech/eru/eru-crypto/aes"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/segmentio/ksuid"
	"io"
	"io/ioutil"
	"mime/multipart"
	"os"
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

func (awsStorage *AwsStorage) DownloadFile(ctx context.Context, folderPath string, fileName string, keyName eruaes.AesKey) (file []byte, err error) {
	logs.WithContext(ctx).Debug("DownloadFile - Start")
	if awsStorage.session == nil {
		err = awsStorage.Init(ctx)
		if err != nil {
			return
		}
	}

	tmpfile, err := ioutil.TempFile("", fileName)
	if err != nil {
		return nil, err
	}

	downloader := s3manager.NewDownloader(awsStorage.session)
	_, err = downloader.Download(tmpfile, &s3.GetObjectInput{
		Bucket: aws.String(awsStorage.BucketName),
		Key:    aws.String(fmt.Sprint(folderPath, "/", fileName)),
	})
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		_ = tmpfile.Close()
		_ = os.Remove(tmpfile.Name())
		return nil, err
	}
	byteContainer, err := io.ReadAll(tmpfile)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		_ = tmpfile.Close()
		_ = os.Remove(tmpfile.Name())
		return
	}
	defer func() {
		_ = tmpfile.Close()
		_ = os.Remove(tmpfile.Name())
	}()

	if awsStorage.EncryptFiles {
		byteContainer, err = eruaes.DecryptCBC(ctx, byteContainer, keyName.Key, keyName.Vector)
		if err != nil {
			return
		}
		byteContainer, err = eruaes.Unpad(byteContainer)
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
			return
		}
	}
	return byteContainer, nil
}

func (awsStorage *AwsStorage) UploadFile(ctx context.Context, file multipart.File, header *multipart.FileHeader, docType string, folderPath string, keyName eruaes.AesKey) (docId string, err error) {
	logs.WithContext(ctx).Debug("UploadFile - Start")
	if awsStorage.session == nil {
		logs.WithContext(ctx).Info("creating AWS session")
		err = awsStorage.Init(ctx)
		if err != nil {
			return
		}
	}
	var byteContainer []byte
	enc := ""
	byteContainer, err = io.ReadAll(file)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return
	}

	if awsStorage.EncryptFiles {
		byteContainer = eruaes.Pad(byteContainer, 16)
		byteContainer, err = eruaes.EncryptCBC(ctx, byteContainer, keyName.Key, keyName.Vector)
		if err != nil {
			return
		}
		enc = ".enc"
	}
	docId = ksuid.New().String()
	if docType != "" {
		docType = fmt.Sprint(docType, "_")
	}
	finalFileName := fmt.Sprint(docType, docId, "_", header.Filename, enc)
	uploader := s3manager.NewUploader(awsStorage.session)
	_, err = uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(awsStorage.BucketName),
		Key:    aws.String(fmt.Sprint(folderPath, "/", finalFileName)),
		Body:   bytes.NewReader(byteContainer),
	})
	return
}

func (awsStorage *AwsStorage) UploadFileB64(ctx context.Context, file []byte, fileName string, docType string, folderPath string, keyName eruaes.AesKey) (docId string, err error) {
	logs.WithContext(ctx).Debug("UploadFileB64 - Start")
	if awsStorage.session == nil {
		logs.WithContext(ctx).Info("creating AWS session")
		err = awsStorage.Init(ctx)
		if err != nil {
			return
		}
	}
	enc := ""

	if awsStorage.EncryptFiles {
		file = eruaes.Pad(file, 16)
		file, err = eruaes.EncryptCBC(ctx, file, keyName.Key, keyName.Vector)
		if err != nil {
			return
		}
		enc = ".enc"
	}
	docId = ksuid.New().String()
	if docType != "" {
		docType = fmt.Sprint(docType, "_")
	}
	finalFileName := fmt.Sprint(docType, docId, "_", fileName, enc)
	uploader := s3manager.NewUploader(awsStorage.session)
	_, err = uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(awsStorage.BucketName),
		Key:    aws.String(fmt.Sprint(folderPath, "/", finalFileName)),
		Body:   bytes.NewReader(file),
	})
	return
}

func (awsStorage *AwsStorage) Init(ctx context.Context) (err error) {
	logs.WithContext(ctx).Debug("Init - Start")
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

func (awsStorage *AwsStorage) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &awsStorage)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}
