package models

import (
	"context"
	"encoding/json"
	"errors"

	chunking "github.com/eru-tech/eru/eru-ai/chunking"
	tools "github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

type ModelI interface {
	GetAttribute(ctx context.Context, attributeName string) (attributeValue interface{}, err error)
	MakeFromJson(ctx context.Context, rj *json.RawMessage) (err error)
	PerformPreSaveTask(ctx context.Context) (err error)
	PerformPreDeleteTask(ctx context.Context) (err error)
	QueryModel(ctx context.Context, chatRequest ChatRequest) (response Message, err error)
	QueryModelWithTool(ctx context.Context, chatRequest ChatRequest, tools map[string]tools.Tooling, agentName string, agentPrompt string) (response JsonMessage, err error)
	GenerateEmbeddings(ctx context.Context, inputs []EmbeddingInput, config chunking.ChunkingConfig, dimension int) (outputs []EmbeddingOutput, err error)
}

type Model struct {
	Provider   string  `json:"provider" eru:"required"`
	LLMName    string  `json:"llm_name" eru:"required"`
	ModelName  string  `json:"model_name" eru:"required"`
	LLMSecret  string  `json:"llm_secret" eru:"required"`
	Temprature float64 `json:"temprature"`
}
type ChatRequest struct {
	Messages []Message `json:"messages"`
}
type Message struct {
	Role    string        `json:"role"`
	Content string        `json:"content,omitempty"`
	Name    string        `json:"name"`
	Files   []FileMessage `json:"files,omitempty"`
}
type FileMessage struct {
	FileData  string `json:"file_data,omitempty"`
	FileName  string `json:"file_name,omitempty"`
	FileId    string `json:"file_id,omitempty"`
	ImageData string `json:"image_data,omitempty"`
	FileType  string `json:"file_type,omitempty"`
}
type JsonMessage struct {
	Role    string                 `json:"role"`
	Content map[string]interface{} `json:"content"`
	Name    string                 `json:"name"`
}

type EmbeddingInputRequest struct {
	Inputs      []EmbeddingInput        `json:"inputs" eru:"required"`
	ChunkConfig chunking.ChunkingConfig `json:"chunk_config"`
	Dimension   int                     `json:"dimension"`
}

// Batch embedding input/output types
type EmbeddingInput struct {
	Id   string `json:"id"`
	Text string `json:"text"`
}

type EmbeddingOutput struct {
	Id     string    `json:"id"`
	Text   string    `json:"text"`
	Vector []float64 `json:"vector"`
}

func (model *Model) GetAttribute(ctx context.Context, attributeName string) (attributeValue interface{}, err error) {
	switch attributeName {
	case "provider":
		return model.Provider, nil
	case "model_name":
		return model.ModelName, nil
	case "llm_name":
		return model.LLMName, nil
	case "llm_secret":
		return model.LLMSecret, nil
	case "temprature":
		return model.Temprature, nil
	default:
		err := errors.New("attribute not found")
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
}

func GetModel(provider string) ModelI {
	switch provider {
	case "OPENAI":
		return new(OpenAIModel)
	case "ANTHROPIC":
		return new(AnthropicModel)
	case "BEDROCK":
		return new(BedrockModel)
	case "GEMINI":
		return new(GeminiModel)
	default:
		return new(Model)
	}
}

func (model *Model) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	err := errors.New("MakeFromJson Method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return err
}

func (model *Model) PerformPreSaveTask(ctx context.Context) (err error) {
	err = errors.New("PerformPreSaveTask Method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return
}

func (model *Model) PerformPreDeleteTask(ctx context.Context) (err error) {
	err = errors.New("PerformPreDeleteTask Method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return
}

func (model *Model) QueryModel(ctx context.Context, chatRequest ChatRequest) (response Message, err error) {
	err = errors.New("QueryModel Method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return
}

func (model *Model) QueryModelWithTool(ctx context.Context, chatRequest ChatRequest, tools map[string]tools.Tooling, agentName string, agentPrompt string) (response JsonMessage, err error) {
	err = errors.New("QueryModelWithTool Method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return
}

func (model *Model) GenerateEmbedding(ctx context.Context, text string) (embedding []float64, err error) {
	err = errors.New("GenerateEmbedding Method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return
}

func (model *Model) GenerateEmbeddings(ctx context.Context, inputs []EmbeddingInput, config chunking.ChunkingConfig, dimension int) (outputs []EmbeddingOutput, err error) {
	err = errors.New("GenerateEmbeddings Method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return
}

// averageEmbeddings averages multiple embeddings
func (model *Model) averageEmbeddings(embeddings [][]float64) []float64 {
	if len(embeddings) == 0 {
		return nil
	}

	if len(embeddings) == 1 {
		return embeddings[0]
	}

	// All embeddings should have the same dimension
	dimension := len(embeddings[0])
	result := make([]float64, dimension)

	for _, embedding := range embeddings {
		for i, value := range embedding {
			result[i] += value
		}
	}

	// Average the values
	for i := range result {
		result[i] /= float64(len(embeddings))
	}

	return result
}
