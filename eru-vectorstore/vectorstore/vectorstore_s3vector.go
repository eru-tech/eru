package vectorstore

import (
	"context"
	"encoding/json"

	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

type S3VectorStore struct {
	VectorStore
	BucketName   string
	Region       string
	AccessKeyID  string
	SecretKey    string
	SessionToken string
	Indexes      []S3VectorIndex
}

type S3VectorIndex struct {
	Name string `json:"name"`
}

func (svs *S3VectorStore) Search(ctx context.Context, query []float64, topK int, filter map[string]interface{}) ([]VectorResult, error) {
	logs.WithContext(ctx).Info("Searching vectors in S3 Vector Store")

	return nil, nil
}

func (svs *S3VectorStore) Insert(ctx context.Context, vectors []Vector) error {
	logs.WithContext(ctx).Info("Inserting vectors to S3 Vector Store")

	return nil
}

func (svs *S3VectorStore) Update(ctx context.Context, vectors []Vector) error {
	logs.WithContext(ctx).Info("Updating vectors in S3 Vector Store")

	return nil
}

func (svs *S3VectorStore) Delete(ctx context.Context, ids []string) error {
	logs.WithContext(ctx).Info("Deleting vectors from S3 Vector Store")

	return nil
}

func (svs *S3VectorStore) CreateIndex(ctx context.Context) error {
	logs.WithContext(ctx).Info("Creating S3 Vector index")
	//svs.IndexName = indexName

	return nil
}

func (svs *S3VectorStore) DeleteIndex(ctx context.Context, indexName string) error {
	logs.WithContext(ctx).Info("Deleting S3 Vector index")

	return nil
}

func (svs *S3VectorStore) GetStats(ctx context.Context) (VectorStats, error) {
	logs.WithContext(ctx).Info("Getting S3 Vector stats")

	return VectorStats{
		//IndexName: svs.IndexName,
	}, nil
}
func (svs *S3VectorStore) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &svs)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}
