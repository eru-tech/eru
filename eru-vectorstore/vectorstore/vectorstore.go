package vectorstore

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"

	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

type VectorStoreI interface {
	ListVectors(ctx context.Context, vectorRecordsList VectorRecordsList) (VectorResults, error)
	SearchVectors(ctx context.Context, vectorRecordsSearch VectorRecordsSearch) (VectorResults, error)
	SaveVectors(ctx context.Context, vectorRecords VectorRecords) error
	DeleteVectors(ctx context.Context, vectorRecordsDelete VectorRecordsDelete) error
	CreateIndex(ctx context.Context, cloneVectorStore VectorStoreI) error
	DeleteIndex(ctx context.Context, indexName string) error
	GetStats(ctx context.Context) (VectorStats, error)
	GetAttribute(ctx context.Context, attributeName string) string
	MakeFromJson(ctx context.Context, rj *json.RawMessage) error
	EditIndex(ctx context.Context, cloneVectorStore VectorStoreI) error
	UpdateVectorStore(ctx context.Context, updatedVectorStore VectorStoreI) error
	GetBytes(ctx context.Context) ([]byte, error)
	BytesToVectorStore(ctx context.Context, vectorStoreObjJson []byte) (VectorStoreI, error)
	SyncIndexDefinition(ctx context.Context, cloneVectorStore VectorStoreI) error
	CheckRemoteStoreExists(ctx context.Context) (exists bool, err error)
}

type Vector struct {
	Id           string    `json:"id" eru:"required"`
	Values       []float64 `json:"values,omitempty"`
	SparceValues struct {
		Indices []int     `json:"indices"`
		Values  []float64 `json:"values"`
	} `json:"sparce_values,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}
type VectorRecords struct {
	Vectors   []Vector `json:"vectors" eru:"required"`
	Namespace string   `json:"namespace" eru:"required"`
}

type VectorRecordsDelete struct {
	Ids       []string               `json:"ids"`
	Namespace string                 `json:"namespace"`
	Filter    map[string]interface{} `json:"filter,omitempty"`
	DeleteAll bool                   `json:"delete_all,omitempty"`
}
type VectorRecordsList struct {
	Ids            []string `json:"ids"`
	ReturnVector   bool     `json:"return_vector,omitempty"`
	ReturnMetadata bool     `json:"return_metadata,omitempty"`
}

type VectorRecordsSearch struct {
	Id              string                 `json:"query_id,omitempty"`
	Namespace       string                 `json:"namespace" eru:"required"`
	Filter          map[string]interface{} `json:"filter,omitempty"`
	Fields          []string               `json:"fields,omitempty"`
	Inputs          map[string]string      `json:"inputs,omitempty"`
	TopK            int                    `json:"top_k,omitempty"`
	IncludeValues   bool                   `json:"include_values,omitempty"`
	IncludeMetadata bool                   `json:"include_metadata,omitempty"`
	ReturnDistance  bool                   `json:"return_distance,omitempty"`
	Vector          []float64              `json:"vector,omitempty"`
	SparceVector    struct {
		Indices []int     `json:"indices"`
		Values  []float64 `json:"values"`
	} `json:"sparce_vector,omitempty"`
	ReRank struct {
		Model      string                 `json:"model,omitempty"`
		RankFields []string               `json:"rank_fields,omitempty"`
		TopN       int                    `json:"top_n,omitempty"`
		Parameters map[string]interface{} `json:"parameters,omitempty"`
		Query      string                 `json:"query,omitempty"`
	} `json:"rerank,omitempty"`
}

type VectorIndexI interface {
	GetAttribute(ctx context.Context, attributeName string) string
}
type VectorResults struct {
	Records []VectorResult    `json:"records"`
	Usage   map[string]string `json:"usage"`
}
type VectorResult struct {
	Id       string                 `json:"id"`
	Values   []float64              `json:"values,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
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

func (vs *VectorStore) SearchVectors(ctx context.Context, vectorRecordsSearch VectorRecordsSearch) (VectorResults, error) {
	logs.WithContext(ctx).Info("SearchVectors method not implemented")
	return VectorResults{
		Records: []VectorResult{},
		Usage:   map[string]string{},
	}, nil
}
func (vs *VectorStore) ListVectors(ctx context.Context, vectorRecordsList VectorRecordsList) (VectorResults, error) {
	logs.WithContext(ctx).Info("ListVectors method not implemented")
	return VectorResults{
		Records: []VectorResult{},
		Usage:   map[string]string{},
	}, nil
}

func (vs *VectorStore) SaveVectors(ctx context.Context, vectorRecords VectorRecords) error {
	logs.WithContext(ctx).Info("SaveVectors method not implemented")
	return nil
}

func (vs *VectorStore) DeleteVectors(ctx context.Context, vectorRecordsDelete VectorRecordsDelete) error {
	logs.WithContext(ctx).Info("DeleteVectors method not implemented")
	return nil
}

func (vs *VectorStore) CreateIndex(ctx context.Context, cloneVectorStore VectorStoreI) error {
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

func (vs *VectorStore) EditIndex(ctx context.Context, cloneVectorStore VectorStoreI) error {
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
func (vs *VectorStore) SyncIndexDefinition(ctx context.Context, cloneVectorStore VectorStoreI) error {
	logs.WithContext(ctx).Info("SyncIndexDefinition method not implemented")
	return nil
}
func (vs *VectorStore) CheckRemoteStoreExists(ctx context.Context) (exists bool, err error) {
	logs.WithContext(ctx).Info("CheckRemoteStoreExists method not implemented")
	return false, nil
}
