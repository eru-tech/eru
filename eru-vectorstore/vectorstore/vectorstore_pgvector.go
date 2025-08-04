package vectorstore

import (
	"context"
	"encoding/json"

	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/jmoiron/sqlx"
)

type PGVectorStore struct {
	VectorStore
	DB       *sqlx.DB
	Host     string
	Port     string
	Database string
	User     string
	Password string
	Indexes  []PGVectorIndex
}

type PGVectorIndex struct {
	Name string `json:"name"`
}

func (pgvs *PGVectorStore) Search(ctx context.Context, query []float64, topK int, filter map[string]interface{}) ([]VectorResult, error) {
	logs.WithContext(ctx).Info("Searching vectors in PGVector")

	return nil, nil
}

func (pgvs *PGVectorStore) Insert(ctx context.Context, vectors []Vector) error {
	logs.WithContext(ctx).Info("Inserting vectors to PGVector")

	return nil
}

func (pgvs *PGVectorStore) Update(ctx context.Context, vectors []Vector) error {
	logs.WithContext(ctx).Info("Updating vectors in PGVector")

	return nil
}

func (pgvs *PGVectorStore) Delete(ctx context.Context, ids []string) error {
	logs.WithContext(ctx).Info("Deleting vectors from PGVector")

	return nil
}

func (pgvs *PGVectorStore) CreateIndex(ctx context.Context) error {
	logs.WithContext(ctx).Info("Creating PGVector index")
	//pgvs.IndexName = indexName

	return nil
}

func (pgvs *PGVectorStore) DeleteIndex(ctx context.Context, indexName string) error {
	logs.WithContext(ctx).Info("Deleting PGVector index")

	return nil
}

func (pgvs *PGVectorStore) GetStats(ctx context.Context) (VectorStats, error) {
	logs.WithContext(ctx).Info("Getting PGVector stats")

	return VectorStats{
		//IndexName: pgvs.IndexName,
	}, nil
}
func (pgvs *PGVectorStore) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &pgvs)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}
