package model

import (
	"context"
	"encoding/json"
	"errors"

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
