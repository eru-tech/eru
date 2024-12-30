package model

import (
	"context"
	"encoding/json"
	"errors"

	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

type ModelI interface {
	GetAttribute(ctx context.Context, attributeName string) (attributeValue interface{}, err error)
	MakeFromJson(ctx context.Context, rj *json.RawMessage) (err error)
	PerformPreSaveTask(ctx context.Context) (err error)
	PerformPreDeleteTask(ctx context.Context) (err error)
	QueryModel(ctx context.Context, chatRequest ChatRequest) (response Message, err error)
	QueryModelWithTool(ctx context.Context, chatRequest ChatRequest, toolId string) (response JsonMessage, err error)
}

type Model struct {
	ModelId    string  `json:"model_id" eru:"required"`
	LLMName    string  `json:"llm_name" eru:"required"`
	ModelName  string  `json:"model_name" eru:"required"`
	LLMSecret  string  `json:"llm_secret" eru:"required"`
	Temprature float64 `json:"temprature"`
}
type ChatRequest struct {
	Messages []Message `json:"messages"`
}
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Name    string `json:"name"`
}
type JsonMessage struct {
	Role    string                 `json:"role"`
	Content map[string]interface{} `json:"content"`
	Name    string                 `json:"name"`
}

func (model *Model) GetAttribute(ctx context.Context, attributeName string) (attributeValue interface{}, err error) {
	switch attributeName {
	case "model_id":
		return model.ModelId, nil
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

func GetModel(llmName string) ModelI {
	switch llmName {
	case "OPENAI":
		return new(OpenAIModel)
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

func (model *Model) QueryModelWithTool(ctx context.Context, chatRequest ChatRequest, toolId string) (response JsonMessage, err error) {
	err = errors.New("QueryModelWithTool Method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return
}
