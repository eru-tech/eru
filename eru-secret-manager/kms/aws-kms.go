package kms

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

type AwsKmsStore struct {
	KmsStore
	Region         string `json:"region" eru:"required"`
	KmsName        string `json:"kms_name" eru:"required"`
	Authentication string `json:"authentication" eru:"required"`
	Key            string `json:"key" eru:"required"`
	Secret         string `json:"secret" eru:"required"`
	client         *kms.Client
}

func (awsKmsStore *AwsKmsStore) Init(ctx context.Context) (err error) {
	logs.WithContext(ctx).Debug("Init - Start")
	awsConf, awsConfErr := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(awsKmsStore.Region),
	)
	if awsConfErr != nil {
		err = awsConfErr
		logs.WithContext(ctx).Error(err.Error())
		return
	}

	if awsKmsStore.Authentication == AuthTypeSecret {
		awsConf.Credentials = credentials.NewStaticCredentialsProvider(
			awsKmsStore.Key,
			awsKmsStore.Secret,
			"", // a token will be created when the session it's used. //TODO to check this
		)
	}
	awsKmsStore.client = kms.NewFromConfig(awsConf)
	return err
}

func (awsKmsStore *AwsKmsStore) CreateKey(ctx context.Context, aliasName string, keyDesc string) (err error) {
	if awsKmsStore.client == nil {
		err = awsKmsStore.Init(ctx)
		if err != nil {
			return
		}
	}
	keyInput := &kms.CreateKeyInput{
		Description: aws.String(keyDesc),
		KeySpec:     types.KeySpecSymmetricDefault,
		KeyUsage:    types.KeyUsageTypeEncryptDecrypt,
	}
	keyResult, err := awsKmsStore.client.CreateKey(context.TODO(), keyInput)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return
	}

	keyId := keyResult.KeyMetadata.KeyId

	// Create an alias for the newly created key
	aliasInput := &kms.CreateAliasInput{
		AliasName:   aws.String(aliasName),
		TargetKeyId: keyId,
	}
	_, err = awsKmsStore.client.CreateAlias(context.TODO(), aliasInput)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return
	}
	return
}

func (awsKmsStore *AwsKmsStore) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &awsKmsStore)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (awsKmsStore *AwsKmsStore) ListKeys(ctx context.Context) (keyList []string, err error) {
	if awsKmsStore.client == nil {
		err = awsKmsStore.Init(ctx)
		if err != nil {
			return
		}
	}
	paginator := kms.NewListKeysPaginator(awsKmsStore.client, &kms.ListKeysInput{})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.TODO())
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
		}

		for _, key := range page.Keys {

			aliasesPaginator := kms.NewListAliasesPaginator(awsKmsStore.client, &kms.ListAliasesInput{
				KeyId: key.KeyId,
			})

			for aliasesPaginator.HasMorePages() {
				aliasesPage, err := aliasesPaginator.NextPage(context.TODO())
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
				}

				for _, alias := range aliasesPage.Aliases {
					keyList = append(keyList, fmt.Sprint(*key.KeyId, " (", *alias.AliasName, ")"))
				}
			}
		}
	}
	return
}

func (awsKmsStore *AwsKmsStore) Encrypt(ctx context.Context, keyId string, plainText []byte) (encryptedText []byte, err error) {
	if awsKmsStore.client == nil {
		err = awsKmsStore.Init(ctx)
		if err != nil {
			return
		}
	}
	encryptInput := &kms.EncryptInput{
		KeyId:     aws.String(keyId),
		Plaintext: plainText,
	}
	encryptOutput, err := awsKmsStore.client.Encrypt(context.TODO(), encryptInput)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
	}
	encryptedText = encryptOutput.CiphertextBlob
	return
}

func (awsKmsStore *AwsKmsStore) Decrypt(ctx context.Context, keyId string, encryptedText []byte) (plainText []byte, err error) {
	if awsKmsStore.client == nil {
		err = awsKmsStore.Init(ctx)
		if err != nil {
			return
		}
	}
	decryptInput := &kms.DecryptInput{
		CiphertextBlob: encryptedText,
	}
	decryptOutput, err := awsKmsStore.client.Decrypt(context.TODO(), decryptInput)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
	}
	plainText = decryptOutput.Plaintext
	return
}
