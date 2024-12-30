package model

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	tools "github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	utils "github.com/eru-tech/eru/eru-utils"
)

const (
	OpenAIApiUrl = "https://api.openai.com/v1/chat/completions"
)

type OpenAIModel struct {
	Model
}

type OpenAIRequestMessage struct {
	Content    string `json:"content" `
	Role       string `json:"role" `
	Name       string `json:"name"`
	ToolCallId string `json:"tool_call_id"`
	Refusal    string `json:"refusal"`
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
	Name        string                 `json:"name" eru:"required"`
	Description string                 `json:"description" eru:"required"`
	Parameters  map[string]interface{} `json:"parameters" eru:"required"`
	Strict      bool                   `json:"strict"`
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

	Seed        int      `json:"seed"`
	ServiceTier string   `json:"service_tier"`
	Stop        []string `json:"stop"`
	Temperature float64  `json:"temperature"`
	TopP        float64  `json:"top_p"`
	User        string   `json:"user"`
}

type OpenAIChatToolRequest struct {
	OpenAIChatRequest
	ToolChoice        OpenAIRequestToolChoice `json:"tool_choice"`
	Tools             []OpenAIRequestTools    `json:"tools"`
	ParallelToolCalls bool                    `json:"parallel_tool_calls"`
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
func (openaiModel *OpenAIModel) makeOpenAIChatRequest(ctx context.Context, chatRequest ChatRequest) (openAIChatRequest OpenAIChatRequest, err error) {
	logs.WithContext(ctx).Debug("makeOpenAIChatRequest - Start")
	openAIChatRequest = OpenAIChatRequest{
		Model:               openaiModel.ModelName,
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
			Content: message.Content,
			Name:    message.Name,
		})
	}
	return
}
func (openaiModel *OpenAIModel) makeOpenAIChatToolRequest(ctx context.Context, chatRequest ChatRequest) (openAIChatToolRequest OpenAIChatToolRequest, err error) {
	logs.WithContext(ctx).Debug("makeOpenAIChatToolRequest - Start")

	jsonSchema := `{"properties":{"addresses":{"default":[],"items":{"type":"string"},"title":"Addresses","type":"array"},"contact_numbers":{"default":[],"items":{"type":"string"},"title":"Contact Numbers","type":"array"},"designation":{"anyOf":[{"type":"string"},{"type":"null"}],"default":null,"title":"Designation"},"emails":{"default":[],"items":{"type":"string"},"title":"Emails","type":"array"},"first_name":{"anyOf":[{"type":"string"},{"type":"null"}],"default":null,"title":"First Name"},"id":{"anyOf":[{"type":"integer"},{"type":"null"}],"default":null,"title":"Id"},"last_name":{"anyOf":[{"type":"string"},{"type":"null"}],"default":null,"title":"Last Name"},"middle_name":{"anyOf":[{"type":"string"},{"type":"null"}],"default":null,"title":"Middle Name"},"organization":{"anyOf":[{"type":"string"},{"type":"null"}],"default":null,"title":"Organization"},"salutation":{"anyOf":[{"type":"string"},{"type":"null"}],"default":null,"title":"Salutation"}},"required":[],"type":"object"}`
	jsonSchemaObject, err := utils.GetJsonSchemaObject(ctx, jsonSchema)
	if err != nil {
		return
	}
	tool := tools.Tool{
		Name:         "BusinessCard",
		Description:  "Correctly extracted `BusinessCard` with all the required parameters with correct types",
		SystemPrompt: "\nYou are a helpful JSON configuration generator to extract relevant fields from text pertaining to Business Cards. \nBusiness Cards usually will have entiites such as Salutation, First Name, Middle Name, Last Name, Addresses, Contact Numbers, Emails, Organization, Designation etc.\nIf  the user message is a greeting (unless the greeting contains details) like Hi, Hello, How are you, Thank you etc. then you need to return an error message  in the response (This is because the LLM won't be able to generate config).\n",
		OutputSchema: jsonSchemaObject,
	}

	openAIChatToolRequest = OpenAIChatToolRequest{
		OpenAIChatRequest: OpenAIChatRequest{
			Model:               openaiModel.ModelName,
			N:                   1,
			Temperature:         openaiModel.Temprature,
			TopP:                1,
			Modalities:          []string{"text"},
			MaxCompletionTokens: 150,
			ServiceTier:         "auto",
		},
		ToolChoice: OpenAIRequestToolChoice{
			Type: "function",
			Function: ToolChoiceFunction{
				Name: tool.Name,
			},
		},
		Tools: []OpenAIRequestTools{
			{
				Type: "function",
				Function: ToolFunction{
					Name:        tool.Name,
					Description: tool.Description,
					Parameters:  tool.OutputSchema,
				},
			},
		},
	}
	openAIChatToolRequest.Messages = append(openAIChatToolRequest.Messages, OpenAIRequestMessage{
		Role:    "system",
		Content: tool.SystemPrompt,
		Name:    tool.Name,
	})
	for _, message := range chatRequest.Messages {
		openAIChatToolRequest.Messages = append(openAIChatToolRequest.Messages, OpenAIRequestMessage{
			Role:    message.Role,
			Content: message.Content,
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

	logs.WithContext(ctx).Info(fmt.Sprint(chatToolRequest))
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

func (openaiModel *OpenAIModel) QueryModelWithTool(ctx context.Context, chatRequest ChatRequest, toolId string) (queryResponse JsonMessage, err error) {
	logs.WithContext(ctx).Debug("QueryModelWithTool - Start")
	openAIChatToolRequest, err := openaiModel.makeOpenAIChatToolRequest(ctx, chatRequest)
	if err != nil {
		return
	}
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
	queryResponse = JsonMessage{
		Content: outputJson,
		Role:    "assistant",
	}
	return
}
