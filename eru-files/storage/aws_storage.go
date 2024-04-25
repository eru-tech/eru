package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
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
		byteContainer, err = awsStorage.decrypt(ctx, byteContainer, keyName)
		if err != nil {
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
		enc = ".enc"
		byteContainer, err = awsStorage.encrypt(ctx, byteContainer, keyName)
		if err != nil {
			return
		}
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
		enc = ".enc"
		file, err = awsStorage.encrypt(ctx, file, keyName)
		if err != nil {
			return
		}
	}
	docId = ksuid.New().String()
	if docType != "" {
		docType = fmt.Sprint(docType, "_")
	}
	finalFileName := fmt.Sprint(docType, docId, "_", fileName, enc)

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

func (awsStorage *AwsStorage) CreateStorage(ctx context.Context) (err error) {

	if awsStorage.session == nil {
		logs.WithContext(ctx).Info("creating AWS session")
		err = awsStorage.Init(ctx)
		if err != nil {
			return
		}
	}
	if !awsStorage.bucketExists(ctx) {
		_, err = awsStorage.session.CreateBucket(ctx, &s3.CreateBucketInput{
			Bucket: aws.String(awsStorage.BucketName),
			CreateBucketConfiguration: &s3types.CreateBucketConfiguration{
				LocationConstraint: s3types.BucketLocationConstraint(awsStorage.Region),
			},
		})
		if err != nil {
			logs.WithContext(ctx).Info(err.Error())
			err = errors.New("error while creating new AWS bucket")
			return
		}
	} else {
		logs.WithContext(ctx).Info("skipping bucket creation in AWS as it already exists")
	}
	return
}

func (awsStorage *AwsStorage) bucketExists(ctx context.Context) bool {
	_, err := awsStorage.session.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(awsStorage.BucketName),
	})
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			switch apiErr.ErrorCode() {
			case "NotFound", "NoSuchBucket":
				return false // Bucket does not exist
			}
		}
		logs.WithContext(ctx).Info(err.Error())
		err = errors.New(fmt.Sprint("error occurred while checking bucket : ", awsStorage.BucketName))
		return false
	}
	return true // Bucket exists
}
func (awsStorage *AwsStorage) DeleteStorage(ctx context.Context, forceDelete bool) (err error) {

	if awsStorage.session == nil {
		logs.WithContext(ctx).Info("creating AWS session")
		err = awsStorage.Init(ctx)
		if err != nil {
			return
		}
	}
	if forceDelete {
		err = awsStorage.emptyBucket()
		if err != nil {
			return
		}
	}
	_, err = awsStorage.session.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: aws.String(awsStorage.BucketName),
	})
	if err != nil {
		logs.WithContext(ctx).Info(err.Error())
		err = errors.New("error while deleting AWS bucket")
		return
	}

	return
}

func (awsStorage *AwsStorage) emptyBucket() error {
	paginator := s3.NewListObjectsV2Paginator(awsStorage.session, &s3.ListObjectsV2Input{
		Bucket: aws.String(awsStorage.BucketName),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.Background())
		if err != nil {
			return err
		}
		for _, obj := range page.Contents {
			_, delErr := awsStorage.session.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
				Bucket: aws.String(awsStorage.BucketName),
				Key:    obj.Key,
			})
			if delErr != nil {
				return delErr
			}
		}
	}

	return nil
}

func (awsStorage AwsStorage) encrypt(ctx context.Context, byteContainer []byte, keyName eruaes.AesKey) (eByteContainer []byte, err error) {
	if awsStorage.KeyPair != "" {
		byteContainer = eruaes.Pad(byteContainer, 16)
		eByteContainer, err = eruaes.EncryptCBC(ctx, byteContainer, keyName.Key, keyName.Vector)
		if err != nil {
			return
		}
	} else if awsStorage.KmsKey != nil {
		//byteContainerStr := b64.StdEncoding.EncodeToString(byteContainer)
		//byteContainer = eruaes.Pad(byteContainer, 512)
		eByteContainer, err = awsStorage.KmsKey.Encrypt(ctx, byteContainer)
		if err != nil {
			return
		}
	} else {
		err = errors.New("encryption key not found")
		return
	}
	return
}

func (awsStorage AwsStorage) decrypt(ctx context.Context, eByteContainer []byte, keyName eruaes.AesKey) (byteContainer []byte, err error) {
	if awsStorage.KeyPair != "" {
		byteContainer, err = eruaes.DecryptCBC(ctx, eByteContainer, keyName.Key, keyName.Vector)
		if err != nil {
			return
		}
		byteContainer, err = eruaes.Unpad(byteContainer)
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
			return
		}
	} else if awsStorage.KmsKey != nil {
		byteContainer, err = awsStorage.KmsKey.Decrypt(ctx, eByteContainer)
		if err != nil {
			return
		}
	} else {
		err = errors.New("encryption key not found")
		return
	}
	return
}
