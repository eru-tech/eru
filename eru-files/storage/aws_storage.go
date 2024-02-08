package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	eruaes "github.com/eru-tech/eru/eru-crypto/aes"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/segmentio/ksuid"
	"io"
	"mime/multipart"
)

const (
	AuthTypeSecret = "SECRET"
	AuthTypeIAM    = "IAM"
)

type AwsStorage struct {
	Storage
	Region         string `json:"region" eru:"required"`
	BucketName     string `json:"bucket_name" eru:"required"`
	Authentication string `json:"authentication" eru:"required"`
	Key            string `json:"key" eru:"required"`
	Secret         string `json:"secret" eru:"required"`
	session        *s3.Client
}

func (awsStorage *AwsStorage) DownloadFile(ctx context.Context, folderPath string, fileName string, keyName eruaes.AesKey) (file []byte, err error) {
	logs.WithContext(ctx).Debug("DownloadFile - Start")
	if awsStorage.session == nil {
		err = awsStorage.Init(ctx)
		if err != nil {
			return
		}
	}

	result, err := awsStorage.session.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(awsStorage.BucketName),
		Key:    aws.String(fmt.Sprint(folderPath, "/", fileName)),
	})
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
	defer result.Body.Close()

	byteContainer, err := io.ReadAll(result.Body)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}

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
	_, err = awsStorage.session.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(awsStorage.BucketName),
		Key:    aws.String(fmt.Sprint(folderPath, "/", finalFileName)),
		Body:   bytes.NewReader(byteContainer),
	})
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return
	}
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
	logs.WithContext(ctx).Info("file encryption completed")
	docId = ksuid.New().String()
	if docType != "" {
		docType = fmt.Sprint(docType, "_")
	}
	finalFileName := fmt.Sprint(docType, docId, "_", fileName, enc)
	logs.WithContext(ctx).Info(fmt.Sprint("awsStorage.BucketName = ", awsStorage.BucketName))
	logs.WithContext(ctx).Info(fmt.Sprint("fmt.Sprint(folderPath, \"/\", finalFileName) = ", fmt.Sprint(folderPath, "/", finalFileName)))

	_, err = awsStorage.session.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(awsStorage.BucketName),
		Key:    aws.String(fmt.Sprint(folderPath, "/", finalFileName)),
		Body:   bytes.NewReader(file),
	})
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return
	}
	logs.WithContext(ctx).Info("file upload completed")
	return
}

func (awsStorage *AwsStorage) Init(ctx context.Context) (err error) {
	logs.WithContext(ctx).Debug("Init - Start")
	awsConf, awsConfErr := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(awsStorage.Region),
	)
	if awsConfErr != nil {
		err = awsConfErr
		logs.WithContext(ctx).Error(err.Error())
		return
	}

	if awsStorage.Authentication == AuthTypeSecret {
		awsConf.Credentials = credentials.NewStaticCredentialsProvider(
			awsStorage.Key,
			awsStorage.Secret,
			"", // a token will be created when the session is used.
		)
	} else if awsStorage.Authentication == AuthTypeIAM {
		logs.WithContext(ctx).Info("connecting AWS S3 with IAM role")
		// do nothing - no new attributes to set in config
	}
	awsStorage.session = s3.NewFromConfig(awsConf)

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
