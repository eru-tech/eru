package vectorstore

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"

	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

type VectorStoreI interface {
	Search(ctx context.Context, query []float64, topK int, filter map[string]interface{}) ([]VectorResult, error)
	Insert(ctx context.Context, vectors []Vector) error
	Update(ctx context.Context, vectors []Vector) error
	Delete(ctx context.Context, ids []string) error
	CreateIndex(ctx context.Context) error
	DeleteIndex(ctx context.Context, indexName string) error
	GetStats(ctx context.Context) (VectorStats, error)
	GetAttribute(ctx context.Context, attributeName string) string
	MakeFromJson(ctx context.Context, rj *json.RawMessage) error
	EditIndex(ctx context.Context) error
	UpdateVectorStore(ctx context.Context, updatedVectorStore VectorStoreI) error
	GetBytes(ctx context.Context) ([]byte, error)
	BytesToVectorStore(ctx context.Context, vectorStoreObjJson []byte) (VectorStoreI, error)
}

type Vector struct {
	VectorType string                 `json:"vector_type" eru:"required"`
	Name       string                 `json:"name" eru:"required"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

type VectorIndexI interface {
	GetAttribute(ctx context.Context, attributeName string) string
}

type VectorResult struct {
	Vector
	Values []float64 `json:"values"`
	Score  float64   `json:"score"`
}

type VectorStats struct {
	TotalVectors int64  `json:"total_vectors"`
	Dimension    int    `json:"dimension"`
	IndexName    string `json:"index_name"`
}

func GetVectorStore(vectorType string) VectorStoreI {
	switch strings.ToLower(vectorType) {
	case "pinecone":
		return new(PineconeVectorStore)
	case "chromadb":
		return new(ChromaDBVectorStore)
	case "s3vector":
		return new(S3VectorStore)
	case "pgvector":
		return new(PGVectorStore)
	default:
		return new(VectorStore)
	}
}

type VectorStore struct {
	VectorName string `json:"vector_name" eru:"required"`
	VectorType string `json:"vector_type" eru:"required"`
}

func (vs *VectorStore) Search(ctx context.Context, query []float64, topK int, filter map[string]interface{}) ([]VectorResult, error) {
	logs.WithContext(ctx).Info("Search method not implemented")
	return nil, nil
}

func (vs *VectorStore) Insert(ctx context.Context, vectors []Vector) error {
	logs.WithContext(ctx).Info("Insert method not implemented")
	return nil
}

func (vs *VectorStore) Update(ctx context.Context, vectors []Vector) error {
	logs.WithContext(ctx).Info("Update method not implemented")
	return nil
}

func (vs *VectorStore) Delete(ctx context.Context, ids []string) error {
	logs.WithContext(ctx).Info("Delete method not implemented")
	return nil
}

func (vs *VectorStore) CreateIndex(ctx context.Context) error {
	logs.WithContext(ctx).Info("CreateIndex method not implemented")
	return nil
}

func (vs *VectorStore) DeleteIndex(ctx context.Context, indexName string) error {
	logs.WithContext(ctx).Info("DeleteIndex method not implemented")
	return nil
}

func (vs *VectorStore) GetStats(ctx context.Context) (VectorStats, error) {
	logs.WithContext(ctx).Info("GetStats method not implemented")
	return VectorStats{
		//IndexName: vs.IndexName,
	}, nil
}

func (vs *VectorStore) GetAttribute(ctx context.Context, attributeName string) string {
	switch attributeName {
	case "vector_name":
		return vs.VectorName
	case "vector_type":
		return vs.VectorType
	default:
		return ""
	}
}

func (vs *VectorStore) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &vs)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return err
	}
	return nil
}

func (vs *VectorStore) EditIndex(ctx context.Context) error {
	logs.WithContext(ctx).Error("EditIndex not implemented")

	return nil
}

func (vs *VectorStore) UpdateVectorStore(ctx context.Context, updatedVectorStore VectorStoreI) error {
	logs.WithContext(ctx).Error("UpdateVectorStore not implemented")

	return nil
}

func (vs *VectorStore) GetBytes(ctx context.Context) ([]byte, error) {
	vectorStoreJson, err := json.Marshal(vs)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	return vectorStoreJson, nil
}
func (vs *VectorStore) BytesToVectorStore(ctx context.Context, vectorStoreObjJson []byte) (VectorStoreI, error) {
	iCloneI := reflect.New(reflect.TypeOf(vs))
	vectorStoreObjCloneErr := json.Unmarshal(vectorStoreObjJson, iCloneI.Interface())
	if vectorStoreObjCloneErr != nil {
		err := logs.Err(ctx, vectorStoreObjCloneErr, "")
		return nil, err
	}
	return iCloneI.Elem().Interface().(VectorStoreI), nil
}
