package model

import (
	"context"
	"encoding/json"

	//"errors"
	"fmt"
	"net/http"

	tools "github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eru_models "github.com/eru-tech/eru/eru-models"
	utils "github.com/eru-tech/eru/eru-utils"
)

const (
	OpenAIApiUrl          = "https://api.openai.com/v1/chat/completions"
	OpenAIEmbeddingApiUrl = "https://api.openai.com/v1/embeddings"
)

type OpenAIModel struct {
	Model
}

type OpenAIRequestMessage struct {
	Content    []OpenAIRequestMessageContent `json:"content" `
	Role       string                        `json:"role" `
	Name       string                        `json:"name"`
	ToolCallId string                        `json:"tool_call_id"`
	Refusal    string                        `json:"refusal"`
}

type OpenAIRequestMessageContent interface {
	GetType() string
}

type TextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (t TextContent) GetType() string {
	return "text"
}

type FileContent struct {
	Type string                          `json:"type"`
	File OpenAIRequestMessageContentFile `json:"file"`
}

func (f FileContent) GetType() string {
	return "file"
}

type ImageContent struct {
	Type     string                           `json:"type"`
	ImageUrl OpenAIRequestMessageContentImage `json:"image_url"`
}

func (i ImageContent) GetType() string {
	return "image_url"
}

type OpenAIRequestMessageContentFile struct {
	FileId   string `json:"file_id,omitempty"`
	Filename string `json:"filename,omitempty"`
	FileData string `json:"file_data,omitempty"`
}

type OpenAIRequestMessageContentImage struct {
	Url string `json:"url,omitempty"`
}

type OpenAIAudioRequestMessage struct {
	Content    string `json:"content" `
	Role       string `json:"role" `
	Name       string `json:"name"`
	ToolCallId string `json:"tool_call_id"`
	Refusal    string `json:"refusal"`
	Audio      struct {
		Id string `json:"id"`
	} `json:"audio"`
}

type OpenAIToolChoice struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type ToolChoiceFunction struct {
	Name string `json:"name" eru:"required"`
}
type ToolFunction struct {
	Name        string                `json:"name" eru:"required"`
	Description string                `json:"description" eru:"required"`
	Parameters  eru_models.JSONSchema `json:"parameters" eru:"required"`
	Strict      bool                  `json:"strict"`
}

type OpenAIRequestToolChoice struct {
	Type     string             `json:"type" eru:"required"`
	Function ToolChoiceFunction `json:"function" eru:"required"`
}
type OpenAIRequestTools struct {
	Type     string       `json:"type" eru:"required"`
	Function ToolFunction `json:"function" eru:"required"`
}
type OpenAIChatRequest struct {
	MaxCompletionTokens int                    `json:"max_completion_tokens"`
	Messages            []OpenAIRequestMessage `json:"messages" eru:"required"`
	Model               string                 `json:"model" eru:"required"`
	Store               bool                   `json:"store"`
	//ReasoningEffort     string                 `json:"reasoning_effort"` //availble in o1 models only
	Metadata         map[string]interface{} `json:"metadata"`
	FrequencyPenalty float64                `json:"frequency_penalty"`
	LogitBias        map[string]interface{} `json:"logit_bias"`

	N          int      `json:"n"`
	Modalities []string `json:"modalities"`

	PresencePenalty float64 `json:"presence_penalty"`

	Seed        int    `json:"seed"`
	ServiceTier string `json:"service_tier"`
	//Stop        []string `json:"stop"`
	Temperature float64 `json:"temperature"`
	TopP        float64 `json:"top_p"`
	User        string  `json:"user"`
}

type OpenAIChatToolRequest struct {
	OpenAIChatRequest
	//ToolChoice        OpenAIRequestToolChoice `json:"tool_choice"`
	ToolChoice        string               `json:"tool_choice"`
	Tools             []OpenAIRequestTools `json:"tools"`
	ParallelToolCalls bool                 `json:"parallel_tool_calls"`
}

type OpenAIChatAudioRequest struct {
	OpenAIAudioRequestMessage
	Audio struct {
		Voice  string `json:"voice"`
		Format string `json:"format"`
	} `json:"audio"`
}

type OpenAIChatStreamRequest struct {
	OpenAIChatRequest
	Stream        bool `json:"stream"`
	StreamOptions struct {
		IncludeUsage bool `json:"include_usage"`
	} `json:"stream_options"`
}

// mostly not to be used as  we will using tools
//ResponseFormat  OpenAIResponseFormat `json:"response_format"`

// To decide its usage
//Logprobs         bool                   `json:"logprobs"`
//	TopLogprobs      int                    `json:"top_logprobs"`

type OpenAIRequestPrediction struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

type OpenAIResponseFormat struct {
	Type       string            `json:"type" eru:"required"`
	JsonSchema map[string]string `json:"json_schema"`
}
type JsonSchema struct {
	Description string            `json:"description" `
	Name        string            `json:"name" eru:"required"`
	Schema      map[string]string `json:"schema" `
	Strict      bool              `json:"strict"`
}

// OpenAI Request Response //
// @docs https://platform.openai.com/docs/api-reference/chat/create

// OpenAI Response //
type OpenAIResponseFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type OpenAIResponseToolCall struct {
	Id       string                 `json:"id"`
	Type     string                 `json:"type"`
	Function OpenAIResponseFunction `json:"function"`
}

type OpenAIResponseMessage struct {
	Content   string                     `json:"content" eru:"required"`
	Refusal   string                     `json:"refusal"`
	Role      string                     `json:"role" eru:"required"`
	ToolCalls []OpenAIResponseToolCall   `json:"tool_calls"`
	Audio     OpenAIResponseMessageAudio `json:"audio"`
}

type OpenAIResponseMessageAudio struct {
	Id         string `json:"id"`
	Transcript string `json:"transcript"`
	Data       string `json:"data"`
	ExpiresAt  int    `json:"expires_at"`
}

type OpenAIChatResponse struct {
	Id                string                     `json:"id"`
	Object            string                     `json:"object"`
	Created           int                        `json:"created"`
	Model             string                     `json:"model"`
	Choices           []OpenAIChatResponseChoice `json:"choices"`
	ServiceTier       string                     `json:"service_tier"`
	SystemFingerprint string                     `json:"system_fingerprint"`
	Usage             OpenAIChatUsage            `json:"usage"`
}
type OpenAIChatResponseChoice struct {
	FinishReason string                `json:"finish_reason" enum:"stop,length,tool_calls,content_filter,function_call"`
	Index        int                   `json:"index"`
	Message      OpenAIResponseMessage `json:"message"`
	Logprobs     OpenAILogprobs        `json:"logprobs"`
}

type OpenAILogprobs struct {
	Content OpenAILogprobsContent `json:"content"`
	Refusal OpenAILogprobsContent `json:"refusal"`
}
type OpenAILogprobsContentObj struct {
	Token   string  `json:"token"`
	Logprob float64 `json:"logprob"`
	Bytes   []byte  `json:"bytes"`
}
type OpenAILogprobsContent struct {
	OpenAILogprobsContentObj
	TopLogprobs []OpenAILogprobsContentObj `json:"top_logprobs"`
}
type OpenAIChatUsage struct {
	PromptTokens            int                           `json:"prompt_tokens"`
	CompletionTokens        int                           `json:"completion_tokens"`
	TotalTokens             int                           `json:"total_tokens"`
	PromptTokensDetails     OpenAIPromptTokensDetails     `json:"prompt_tokens_details"`
	CompletionTokensDetails OpenAICompletionTokensDetails `json:"completion_tokens_details"`
}
type OpenAICompletionTokensDetails struct {
	AcceptedPredictionTokens int `json:"accepted_prediction_tokens"`
	RejectedPredictionTokens int `json:"rejected_prediction_tokens"`
	AudioTokens              int `json:"audio_tokens"`
	ReasoningTokens          int `json:"reasoning_tokens"`
}
type OpenAIPromptTokensDetails struct {
	AudioTokens  int `json:"audio_tokens"`
	CachedTokens int `json:"cached_tokens"`
}

// OpenAI Embedding Request
type OpenAIEmbeddingRequest struct {
	Input          interface{} `json:"input" eru:"required"`
	Model          string      `json:"model" eru:"required"`
	EncodingFormat string      `json:"encoding_format,omitempty"`
	Dimensions     int         `json:"dimensions,omitempty"`
	User           string      `json:"user,omitempty"`
}

// OpenAI Embedding Response
type OpenAIEmbeddingResponse struct {
	Object string                `json:"object"`
	Data   []OpenAIEmbeddingData `json:"data"`
	Model  string                `json:"model"`
	Usage  OpenAIEmbeddingUsage  `json:"usage"`
}

type OpenAIEmbeddingData struct {
	Object    string    `json:"object"`
	Embedding []float64 `json:"embedding"`
	Index     int       `json:"index"`
}

type OpenAIEmbeddingUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

func (openaiModel *OpenAIModel) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &openaiModel)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}
func (openaiModel *OpenAIModel) QueryModel(ctx context.Context, chatRequest ChatRequest) (queryResponse Message, err error) {
	logs.WithContext(ctx).Debug("QueryModel - Start")
	openAIChatRequest, err := openaiModel.makeOpenAIChatRequest(ctx, chatRequest)
	//_, err = openaiModel.makeOpenAIChatRequest(ctx, chatRequest)

	if err != nil {
		return
	}
	openAIChatResponse, err := openaiModel.queryModel(ctx, openAIChatRequest)
	if err != nil {
		return
	}
	queryResponse = Message{
		Content: openAIChatResponse.Choices[0].Message.Content,
		Role:    "assistant",
	}
	return
}
func (openaiModel *OpenAIModel) makeOpenAIChatRequestContent(ctx context.Context, message Message) (openAIRequestMessageContent []OpenAIRequestMessageContent) {
	logs.WithContext(ctx).Debug("makeOpenAIChatRequestContent - Start")
	if message.Content != "" {
		openAIRequestMessageContent = append(openAIRequestMessageContent, TextContent{
			Type: "text",
			Text: message.Content,
		})
	}
	if len(message.Files) > 0 {
		for _, file := range message.Files {
			if file.FileData != "" {
				openAIRequestMessageContent = append(openAIRequestMessageContent, FileContent{
					Type: "file",
					File: OpenAIRequestMessageContentFile{
						Filename: file.FileName,
						FileData: file.FileData,
					},
				})
			} else if file.FileId != "" {
				openAIRequestMessageContent = append(openAIRequestMessageContent, FileContent{
					Type: "file",
					File: OpenAIRequestMessageContentFile{
						FileId: file.FileId,
					},
				})
			} else if file.ImageData != "" {
				openAIRequestMessageContent = append(openAIRequestMessageContent, ImageContent{
					Type: "image_url",
					ImageUrl: OpenAIRequestMessageContentImage{
						Url: fmt.Sprintf("data:%s;base64,%s", file.FileType, file.ImageData),
					},
				})
			}
		}
	}

	return
}
func (openaiModel *OpenAIModel) makeOpenAIChatRequest(ctx context.Context, chatRequest ChatRequest) (openAIChatRequest OpenAIChatRequest, err error) {
	logs.WithContext(ctx).Debug("makeOpenAIChatRequest - Start")
	openAIChatRequest = OpenAIChatRequest{
		Model:               openaiModel.LLMName,
		N:                   1,
		Temperature:         openaiModel.Temprature,
		TopP:                1,
		Modalities:          []string{"text"},
		MaxCompletionTokens: 150,
		ServiceTier:         "auto",
	}

	for _, message := range chatRequest.Messages {
		openAIChatRequest.Messages = append(openAIChatRequest.Messages, OpenAIRequestMessage{
			Role:    message.Role,
			Content: openaiModel.makeOpenAIChatRequestContent(ctx, message),
			Name:    message.Name,
		})
	}
	return
}

func (openaiModel *OpenAIModel) makeOpenAIChatToolRequest(ctx context.Context, chatRequest ChatRequest, tools map[string]tools.Tooling, agentName string, agentPrompt string) (openAIChatToolRequest OpenAIChatToolRequest, err error) {
	logs.WithContext(ctx).Debug("makeOpenAIChatToolRequest - Start")

	var openAIRequestTools []OpenAIRequestTools
	toolPrompt := ""
	for _, tool := range tools {
		toolNameI, _ := tool.GetAttribute(ctx, "tool_name")
		toolDescriptionI, _ := tool.GetAttribute(ctx, "description")
		toolParametersI, _ := tool.GetAttribute(ctx, "parameters")
		toolSystemPromptI, _ := tool.GetAttribute(ctx, "system_prompt")
		toolName := toolNameI.(string)
		toolDescription := toolDescriptionI.(string)
		toolParameters := toolParametersI.(eru_models.JSONSchema)

		toolPrompt += fmt.Sprint("Tool prompt for Tool ", toolName, " is as follows :\n", toolSystemPromptI.(string))
		reqTool := OpenAIRequestTools{
			Type: "function",
			Function: ToolFunction{
				Name:        toolName,
				Description: toolDescription,
				Parameters:  toolParameters,
			},
		}
		openAIRequestTools = append(openAIRequestTools, reqTool)
	}

	openAIChatToolRequest = OpenAIChatToolRequest{
		OpenAIChatRequest: OpenAIChatRequest{
			Model:               openaiModel.LLMName,
			N:                   1,
			Temperature:         openaiModel.Temprature,
			TopP:                1,
			Modalities:          []string{"text"},
			MaxCompletionTokens: 1024,
			ServiceTier:         "auto",
		},

		ToolChoice: "required",
		/* ToolChoice: OpenAIRequestToolChoice{
			Type: "function",
			Function: ToolChoiceFunction{
				Name: toolName,
			},
		}, */
		Tools: openAIRequestTools,
	}
	openAIChatToolRequest.Messages = append(openAIChatToolRequest.Messages, OpenAIRequestMessage{
		Role: "system",
		Content: openaiModel.makeOpenAIChatRequestContent(ctx, Message{
			Content: fmt.Sprint(agentPrompt, "\n", toolPrompt),
		}),
		Name: agentName,
	})
	for _, message := range chatRequest.Messages {

		openAIChatToolRequest.Messages = append(openAIChatToolRequest.Messages, OpenAIRequestMessage{
			Role:    message.Role,
			Content: openaiModel.makeOpenAIChatRequestContent(ctx, message),
			Name:    message.Name,
		})
	}
	return
}

func (openaiModel *OpenAIModel) queryModel(ctx context.Context, chatRequest OpenAIChatRequest) (openAIChatResponse OpenAIChatResponse, err error) {
	logs.WithContext(ctx).Debug("QueryModel - Start")
	reqHeader := http.Header{}
	reqHeader.Add("Authorization", "Bearer "+openaiModel.LLMSecret)
	reqHeader.Add("Content-Type", "application/json")

	logs.WithContext(ctx).Info(fmt.Sprint(chatRequest))
	//response, respHeaders, respCookies, statusCode, err := utils.CallHttp(ctx, "POST", OpenAIApiUrl, reqHeader, nil, nil, nil, postBody)
	response, _, _, _, err := utils.CallHttp(ctx, "POST", OpenAIApiUrl, reqHeader, nil, nil, nil, chatRequest)

	if err != nil {
		return
	}
	// logs.WithContext(ctx).Info(fmt.Sprint(response))
	// logs.WithContext(ctx).Info(fmt.Sprint(respHeaders))
	// logs.WithContext(ctx).Info(fmt.Sprint(respCookies))
	// logs.WithContext(ctx).Info(fmt.Sprint(statusCode))

	responseJson, err := json.Marshal(response)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return
	}
	err = json.Unmarshal(responseJson, &openAIChatResponse)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return
	}
	logs.WithContext(ctx).Info(fmt.Sprint(openAIChatResponse.Choices[0].Message))
	return
}

func (openaiModel *OpenAIModel) queryModelTool(ctx context.Context, chatToolRequest OpenAIChatToolRequest) (openAIChatResponse OpenAIChatResponse, err error) {
	logs.WithContext(ctx).Debug("QueryModelTool - Start")
	reqHeader := http.Header{}
	reqHeader.Add("Authorization", "Bearer "+openaiModel.LLMSecret)
	reqHeader.Add("Content-Type", "application/json")

	//logs.WithContext(ctx).Info(fmt.Sprint(chatToolRequest))
	//response, respHeaders, respCookies, statusCode, err := utils.CallHttp(ctx, "POST", OpenAIApiUrl, reqHeader, nil, nil, nil, postBody)
	response, _, _, _, err := utils.CallHttp(ctx, "POST", OpenAIApiUrl, reqHeader, nil, nil, nil, chatToolRequest)

	if err != nil {
		return
	}
	// logs.WithContext(ctx).Info(fmt.Sprint(response))
	// logs.WithContext(ctx).Info(fmt.Sprint(respHeaders))
	// logs.WithContext(ctx).Info(fmt.Sprint(respCookies))
	// logs.WithContext(ctx).Info(fmt.Sprint(statusCode))

	responseJson, err := json.Marshal(response)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return
	}
	err = json.Unmarshal(responseJson, &openAIChatResponse)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return
	}
	logs.WithContext(ctx).Info(fmt.Sprint(openAIChatResponse.Choices[0].Message))
	return
}

func (openaiModel *OpenAIModel) QueryModelWithTool(ctx context.Context, chatRequest ChatRequest, tools map[string]tools.Tooling, agentName string, agentPrompt string) (queryResponse JsonMessage, err error) {
	logs.WithContext(ctx).Debug("QueryModelWithTool - Start")
	openAIChatToolRequest, err := openaiModel.makeOpenAIChatToolRequest(ctx, chatRequest, tools, agentName, agentPrompt)
	if err != nil {
		return
	}
	logs.WithContext(ctx).Info(fmt.Sprint(openAIChatToolRequest))
	openAIChatResponse, err := openaiModel.queryModelTool(ctx, openAIChatToolRequest)
	if err != nil {
		return
	}
	outputJson := make(map[string]interface{})
	err = json.Unmarshal([]byte(openAIChatResponse.Choices[0].Message.ToolCalls[0].Function.Arguments), &outputJson)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		outputJson["raw"] = openAIChatResponse.Choices[0].Message.ToolCalls[0].Function.Arguments
	}
	//outputJson["raw"] = openAIChatResponse
	queryResponse = JsonMessage{
		Content: outputJson,
		Role:    "assistant",
	}
	return
}

func (openaiModel *OpenAIModel) GenerateEmbedding(ctx context.Context, text string) (embedding []float64, err error) {
	logs.WithContext(ctx).Debug("GenerateEmbedding - Start")

	// Create embedding request
	embeddingRequest := OpenAIEmbeddingRequest{
		Input: text,
		Model: "text-embedding-ada-002", // Default OpenAI embedding model
	}

	// Make HTTP request to OpenAI embedding API
	reqHeader := http.Header{}
	reqHeader.Add("Authorization", "Bearer "+openaiModel.LLMSecret)
	reqHeader.Add("Content-Type", "application/json")

	response, _, _, _, err := utils.CallHttp(ctx, "POST", OpenAIEmbeddingApiUrl, reqHeader, nil, nil, nil, embeddingRequest)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("Failed to call OpenAI embedding API: %v", err))
		return nil, err
	}

	// Parse response
	responseJson, err := json.Marshal(response)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("Failed to marshal response: %v", err))
		return nil, err
	}

	var embeddingResponse OpenAIEmbeddingResponse
	err = json.Unmarshal(responseJson, &embeddingResponse)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("Failed to unmarshal embedding response: %v", err))
		return nil, err
	}

	// Check if we have embedding data
	if len(embeddingResponse.Data) == 0 {
		err = fmt.Errorf("no embedding data received from OpenAI")
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}

	embedding = embeddingResponse.Data[0].Embedding
	logs.WithContext(ctx).Debug(fmt.Sprintf("Generated embedding with %d dimensions", len(embedding)))
	return embedding, nil
}
