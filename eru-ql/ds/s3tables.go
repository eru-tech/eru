package ds

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3tables"
	"github.com/aws/aws-sdk-go/aws"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-ql/module_model"
	sqlengine "github.com/eru-tech/eru/eru-ql/sql_engine"
)

const (
	AuthTypeSecret = "SECRET"
	AuthTypeIAM    = "IAM"
)

func (ib *IcebergSqlMaker) S3TablesInit(ctx context.Context, s3TablesConfig *module_model.S3TablesConfig) (err error) {
	logs.WithContext(ctx).Debug("S3TablesInit - Start")
	awsConf, awsConfErr := config.LoadDefaultConfig(ctx,
		config.WithRegion(s3TablesConfig.Region),
	)
	if awsConfErr != nil {
		err = logs.Err(ctx, awsConfErr, "error while loading AWS config")
		return
	}

	if s3TablesConfig.Authentication == AuthTypeSecret {
		awsConf.Credentials = credentials.NewStaticCredentialsProvider(
			s3TablesConfig.Key,
			s3TablesConfig.Secret,
			"", // a token will be created when the session is used.
		)
	} else if s3TablesConfig.Authentication == AuthTypeIAM {
		logs.WithContext(ctx).Info("connecting AWS S3tables with IAM role")
		// do nothing - no new attributes to set in config
	}
	s3TablesConfig.Session = s3tables.NewFromConfig(awsConf)

	return err

}

func (ib *IcebergSqlMaker) S3TablesCreateConn(ctx context.Context, dataSource *module_model.DataSource) (err error) {
	logs.WithContext(ctx).Debug("S3TablesCreateConn - Start")
	if dataSource.IcebergConfig.S3TablesConfig.BucketName == "" {
		err = logs.Err(ctx, errors.New("S3TablesConfig.BucketName is nil"), "bucket name cannot be blank")
		return err
	}
	if dataSource.IcebergConfig.S3TablesConfig.Session == nil {
		err = ib.S3TablesInit(ctx, &dataSource.IcebergConfig.S3TablesConfig)
		if err != nil {
			return err
		}
	}
	// Step 1. Create a bucket once (infra bootstrap)
	bucketExists := false
	buckets, bucketsErr := dataSource.IcebergConfig.S3TablesConfig.Session.ListTableBuckets(ctx, &s3tables.ListTableBucketsInput{})
	if bucketsErr != nil {
		err = logs.Err(ctx, bucketsErr, "error while listing S3Tables buckets")
		return err
	}
	for _, bucket := range buckets.TableBuckets {
		if *bucket.Name == dataSource.IcebergConfig.S3TablesConfig.BucketName {
			dataSource.IcebergConfig.S3TablesConfig.BucketArn = *bucket.Arn
			bucketExists = true
			break
		}
	}

	if !bucketExists {
		bucketOut, bucketOutErr := dataSource.IcebergConfig.S3TablesConfig.Session.CreateTableBucket(ctx, &s3tables.CreateTableBucketInput{
			Name: aws.String(dataSource.IcebergConfig.S3TablesConfig.BucketName),
		})
		if bucketOutErr != nil {
			err = logs.Err(ctx, bucketOutErr, "error while creating new S3Tables bucket")
			return err
		}
		bucketArn := *bucketOut.Arn
		dataSource.IcebergConfig.S3TablesConfig.BucketArn = bucketArn
	}
	// Step 2. Create a namespace (database)
	namespace := fmt.Sprintf("%s_%s", dataSource.IcebergConfig.TenantId, dataSource.IcebergConfig.Database)

	namespaceExists := false
	namespaces, namespaceErr := dataSource.IcebergConfig.S3TablesConfig.Session.ListNamespaces(ctx, &s3tables.ListNamespacesInput{
		TableBucketARN: aws.String(dataSource.IcebergConfig.S3TablesConfig.BucketArn),
	})
	if namespaceErr != nil {
		err = logs.Err(ctx, namespaceErr, "error while listing S3Tables namespaces")
		return err
	}

	for _, ns := range namespaces.Namespaces {
		// ns is []string, flatten with dot for comparison
		joined := strings.Join(ns.Namespace, ".")
		if joined == namespace {
			namespaceExists = true
			break
		}
	}

	if !namespaceExists {
		namespaceOut, namespaceOutErr := dataSource.IcebergConfig.S3TablesConfig.Session.CreateNamespace(ctx, &s3tables.CreateNamespaceInput{
			TableBucketARN: aws.String(dataSource.IcebergConfig.S3TablesConfig.BucketArn),
			Namespace:      []string{namespace},
		})
		if namespaceOutErr != nil {
			err = logs.Err(ctx, namespaceOutErr, "error while creating new S3Tables namespace")
			return err
		}
		logs.WithContext(ctx).Info(fmt.Sprintf("namespace created successfully with arn: %s", namespaceOut.Namespace))
	}

	if dataSource.SqlEngine == nil {
		if dataSource.SqlEngineType == "athena" {
			dataSource.SqlEngine = &sqlengine.AthenaSQLEngine{
				Authentication: dataSource.IcebergConfig.S3TablesConfig.Authentication,
				Key:            dataSource.IcebergConfig.S3TablesConfig.Key,
				Secret:         dataSource.IcebergConfig.S3TablesConfig.Secret,
				Region:         dataSource.IcebergConfig.S3TablesConfig.Region,
			}
			err = dataSource.SqlEngine.Init(ctx)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
