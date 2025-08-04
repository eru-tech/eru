package vectorstore

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	utils "github.com/eru-tech/eru/eru-utils"
)

const baseUrl = "https://api.pinecone.io"

type PineconeVectorStore struct {
	VectorStore
	APIKey string              `json:"api_key" eru:"required"`
	Index  PineconeVectorIndex `json:"index"`
}
type PineconeVectorIndex struct {
	Name      string `json:"name"`
	Dimension int    `json:"dimension"`
	Metric    string `json:"metric"`
	Status    struct {
		Ready bool   `json:"ready"`
		State string `json:"state"`
	} `json:"status"`
	Host           string `json:"host"`
	ServerlessSpec struct {
		Region           string `json:"region"`
		Cloud            string `json:"cloud"`
		SourceCollection string `json:"source_collection"`
	} `json:"serverless_spec"`
	PodSpec struct {
		Environment      string                 `json:"environment"`
		PodType          string                 `json:"pod_type"`
		Replicas         int                    `json:"replicas"`
		Shards           int                    `json:"shards"`
		Pods             int                    `json:"pods"`
		MetadataConfig   map[string]interface{} `json:"metadata_config"`
		SourceCollection string                 `json:"source_collection"`
	} `json:"pod_spec"`
	DeletionProtection string            `json:"deletion_protection"`
	Tags               map[string]string `json:"tags"`
	VectorType         string            `json:"vector_type"`
}

func (pvs *PineconeVectorStore) initClient(ctx context.Context) error {
	// Client initialization for future SDK integration
	return nil
}

func (pvs *PineconeVectorStore) Search(ctx context.Context, query []float64, topK int, filter map[string]interface{}) ([]VectorResult, error) {
	logs.WithContext(ctx).Debug("PineconeVectorStore Search - Start")

	return nil, nil
}

func (pvs *PineconeVectorStore) Insert(ctx context.Context, vectors []Vector) error {
	logs.WithContext(ctx).Debug("PineconeVectorStore Insert - Start")

	return nil
}

func (pvs *PineconeVectorStore) Update(ctx context.Context, vectors []Vector) error {
	logs.WithContext(ctx).Debug("PineconeVectorStore Update - Start")

	return nil
}

func (pvs *PineconeVectorStore) Delete(ctx context.Context, ids []string) error {
	logs.WithContext(ctx).Debug("PineconeVectorStore Delete - Start")

	return nil
}

func (pvs *PineconeVectorStore) CreateIndex(ctx context.Context) error {
	logs.WithContext(ctx).Debug("CreateIndex - Start")

	// Initialize Pinecone client
	if err := pvs.initClient(ctx); err != nil {
		return err
	}

	// Create index using Pinecone API directly
	createIndexPayload := map[string]interface{}{
		"name":      pvs.Index.Name,
		"dimension": pvs.Index.Dimension,
		"metric":    pvs.Index.Metric,
		"spec": map[string]interface{}{
			"serverless": map[string]interface{}{
				"cloud":  pvs.Index.ServerlessSpec.Cloud,
				"region": pvs.Index.ServerlessSpec.Region,
			},
		},
	}
	if pvs.Index.PodSpec.PodType != "" {
		createIndexPayload["spec"] = map[string]interface{}{
			"pod": map[string]interface{}{
				"pod_type":        pvs.Index.PodSpec.PodType,
				"replicas":        pvs.Index.PodSpec.Replicas,
				"shards":          pvs.Index.PodSpec.Shards,
				"pods":            pvs.Index.PodSpec.Pods,
				"environment":     pvs.Index.PodSpec.Environment,
				"metadata_config": pvs.Index.PodSpec.MetadataConfig,
			},
		}
	}

	// Use Pinecone API endpoint directly
	url := fmt.Sprintf("%s/indexes", baseUrl)
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	headers.Set("Api-Key", pvs.APIKey)

	_, _, _, _, err := utils.CallHttp(ctx, "POST", url, headers, nil, nil, nil, createIndexPayload)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return err
	}
	return nil
}

func (pvs *PineconeVectorStore) DeleteIndex(ctx context.Context, indexName string) error {
	logs.WithContext(ctx).Debug("PineconeVectorStore DeleteIndex - Start")

	// Initialize Pinecone client
	if err := pvs.initClient(ctx); err != nil {
		return err
	}

	// Delete index using Pinecone API directly
	url := fmt.Sprintf("%s/indexes/%s", baseUrl, indexName)
	headers := http.Header{}
	headers.Set("Api-Key", pvs.APIKey)

	_, _, _, _, err := utils.CallHttp(ctx, "DELETE", url, headers, nil, nil, nil, nil)
	if err != nil {
		if !strings.Contains(err.Error(), "NOT_FOUND") {
			err = logs.Err(ctx, err, "")
			return err
		} else {
			err = nil
		}
	}
	return nil
}

func (pvs *PineconeVectorStore) GetStats(ctx context.Context) (VectorStats, error) {
	logs.WithContext(ctx).Debug("PineconeVectorStore GetStats - Start")

	return VectorStats{
		//IndexName: pvs.IndexName,
	}, nil
}
func (pvs *PineconeVectorStore) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &pvs)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return err
	}
	return nil
}

func (pvs *PineconeVectorStore) EditIndex(ctx context.Context) error {
	logs.WithContext(ctx).Debug("PineconeVectorStore Edit - Start")

	logs.WithContext(ctx).Info("PineconeVectorStore edit completed")
	return nil
}
func (pvs *PineconeVectorStore) GetBytes(ctx context.Context) ([]byte, error) {
	logs.WithContext(ctx).Debug("PineconeVectorStore GetBytes - Start")
	vectorStoreJson, err := json.Marshal(pvs)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	return vectorStoreJson, nil
}
func (pvs *PineconeVectorStore) BytesToVectorStore(ctx context.Context, vectorStoreObjJson []byte) (VectorStoreI, error) {
	logs.WithContext(ctx).Debug("PineconeVectorStore BytesToVectorStore - Start")
	iCloneI := reflect.New(reflect.TypeOf(pvs))
	vectorStoreObjCloneErr := json.Unmarshal(vectorStoreObjJson, iCloneI.Interface())
	if vectorStoreObjCloneErr != nil {
		err := logs.Err(ctx, vectorStoreObjCloneErr, "error while cloning vectorStoreObj(unmarshal)")
		return nil, err
	}
	return iCloneI.Elem().Interface().(VectorStoreI), nil
}
func (pvs *PineconeVectorStore) GetAttribute(ctx context.Context, attributeName string) string {
	switch attributeName {
	case "vector_name":
		return pvs.VectorName
	case "vector_type":
		return pvs.VectorType
	case "index_name":
		return pvs.Index.Name
	default:
		return ""
	}
}
func (pvs *PineconeVectorStore) ValidateEditIndex(ctx context.Context, updatedVectorStore VectorStoreI) error {
	logs.WithContext(ctx).Debug("ValidateEditIndex - Start")

	_, ok := updatedVectorStore.(*PineconeVectorStore)
	if !ok {
		return logs.Err(ctx, fmt.Errorf("invalid vector store type"), "")
	}

	return nil
}
