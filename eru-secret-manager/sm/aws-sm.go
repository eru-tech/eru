package sm

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

type AwsSmStore struct {
	SmStore
	Region         string `json:"region" eru:"required"`
	SmName         string `json:"sm_name" eru:"required"`
	KmsName        string `json:"kms_name" eru:"required"`
	Authentication string `json:"authentication" eru:"required"`
	Key            string `json:"key" eru:"required"`
	Secret         string `json:"secret" eru:"required"`
	client         *secretsmanager.Client
}

func (awsSmStore *AwsSmStore) Init(ctx context.Context) (err error) {
	logs.WithContext(ctx).Debug("Init - Start")
	awsConf, awsConfErr := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(awsSmStore.Region),
	)
	if awsConfErr != nil {
		err = awsConfErr
		logs.WithContext(ctx).Error(err.Error())
		return
	}

	if awsSmStore.Authentication == AuthTypeIAM {
		appCreds := aws.NewCredentialsCache(ec2rolecreds.New())
		logs.WithContext(ctx).Error(fmt.Sprint(appCreds))
	} else if awsSmStore.Authentication == AuthTypeSecret {
		awsConf.Credentials = credentials.NewStaticCredentialsProvider(
			awsSmStore.Key,
			awsSmStore.Secret,
			"", // a token will be created when the session it's used. //TODO to check this
		)
	}
	awsSmStore.client = secretsmanager.NewFromConfig(awsConf)
	return err
}

func (awsSmStore *AwsSmStore) FetchSmValue(ctx context.Context) (resultJson map[string]string, err error) {
	logs.WithContext(ctx).Debug("FetchSmValue - Start")
	if awsSmStore.client == nil {
		err = awsSmStore.Init(ctx)
		if err != nil {
			return
		}
	}

	input := &secretsmanager.GetSecretValueInput{
		SecretId:     aws.String(awsSmStore.SmName),
		VersionStage: aws.String("AWSCURRENT"), // VersionStage defaults to AWSCURRENT if unspecified
	}

	result, err := awsSmStore.client.GetSecretValue(context.TODO(), input)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return
	}
	err = json.Unmarshal([]byte(*result.SecretString), &resultJson)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		resultJson["secret"] = *result.SecretString
		return
	}
	return
}

func (awsSmStore *AwsSmStore) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &awsSmStore)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}
