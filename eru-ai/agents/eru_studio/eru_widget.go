package eru_studio

import (
	"context"
	"encoding/json"
	"fmt"

	agents "github.com/eru-tech/eru/eru-ai/agents"
	models "github.com/eru-tech/eru/eru-ai/models"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eru_utils "github.com/eru-tech/eru/eru-utils"
	//eru_models "github.com/eru-tech/eru/eru-models"
)

const templateVarsSchemaString = `{"type":"object","properties":{"Headers":{"type":"object"},"FormData":{"type":"object"},"FileData":{"type":"object"},"Params":{"type":"object"},"Vars":{"type":"object","properties":{"Body":{"type":"object"},"OrgBody":{"type":"object"}},"required":[]},"Body":{"type":"object"},"OrgBody":{"type":"object"},"Token":{"type":"object"},"FormDataKeyArray":{"type":"array","items":[{"type":"string"}]},"LoopVars":{"type":"array","items":[{"type":"object"}]},"LoopVar":{"type":"object"},"Cookies":{"type":"object"},"ResponseStatus":{"type":"integer"}},"required":[]}`

type EruWidgetAgent struct {
	agents.Agent
}

func (EruWidgetAgent *EruWidgetAgent) GetSpec() agents.AgentI {
	return EruWidgetAgent
}

func (EruWidgetAgent *EruWidgetAgent) Execute(ctx context.Context, agentMessage agents.AgentMessage, conversationId string, projectId string, tenantId string) (map[string]interface{}, error) {
	logs.WithContext(ctx).Debug("Agent Execute - Start")
	agentOutput := make(map[string]interface{})
	contextStringI, contextStringIOk := agentMessage.Params["context"]
	if contextStringIOk {
		if contextString, contextStringOk := contextStringI.(string); contextStringOk {
			contextMap := make(map[string]interface{})
			err := json.Unmarshal([]byte(contextString), &contextMap)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return nil, err
			}
			agentMessage.Params["context"] = contextMap
		}
	}
	contextJsonSchema := eru_utils.GenerateJSONSchema(ctx, agentMessage.Params)
	jsonSchemaString, err := json.Marshal(contextJsonSchema)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
	_ = jsonSchemaString
	eruComponentString := ""
	eruComponentStringOk := false
	templateCode, templateCodeOk := agentMessage.Params["code"]
	if !templateCodeOk {
		logs.WithContext(ctx).Info("code is not present in the params")
	} else {
		eruComponentString, eruComponentStringOk = templateCode.(string)
		if !eruComponentStringOk {
			logs.WithContext(ctx).Info("code is not a string")
		} else {
			eruComponentString = fmt.Sprintf("This is existing Eru Component json and you need to build on top of this incorporating user's new instructions. If this code is blank, write a new Eru Component json. \n\n %s \n\n", eruComponentString)
		}
	}

	//contextVariableString := fmt.Sprintf("Use this json as context variable to be used in the gotemplate \n\n %s \n\n", jsonSchemaString)

	contextVariableString := fmt.Sprint(contextVariablePrompt, eruComponentString)

	logs.WithContext(ctx).Info(contextVariableString)

	msg := models.Message{
		Role:    "assistant",
		Content: agentMessage.Content,
		Name:    EruWidgetAgent.AgentName,
	}
	msg1 := models.Message{
		Role:    "assistant",
		Content: contextVariableString,
		Name:    EruWidgetAgent.AgentName,
	}
	chatRequest := models.ChatRequest{
		Messages: []models.Message{
			msg,
			msg1,
		},
	}
	_ = chatRequest
	agentOutput, err = EruWidgetAgent.execute(ctx, chatRequest, contextStringI, EruWidgetAgent.AgentTools, EruWidgetAgent.AgentName, EruWidgetAgent.SystemPrompt, 1, projectId, tenantId)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
	return agentOutput, nil
}
func (EruWidgetAgent *EruWidgetAgent) execute(ctx context.Context, chatRequest models.ChatRequest, contextStringI interface{}, agentTools []agents.AgentTools, agentName string, systemPrompt string, currentTry int, projectId string, tenantId string) (map[string]interface{}, error) {
	agentOutput := make(map[string]interface{})

	toolResults, err := EruWidgetAgent.ExecuteTools(ctx, chatRequest, EruWidgetAgent.AgentTools, projectId, tenantId)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
	logs.WithContext(ctx).Info(fmt.Sprintf("Tool results: %+v", toolResults))

	agentOutput = toolResults

	/*templateCode, templateCodeOk := agentOutput["code"].(string)
	if !templateCodeOk {
		logs.WithContext(ctx).Error("code is not present in the params")
		return nil, fmt.Errorf("code is not present in the params")
	}
	 var output interface{}
	output, err = EruWidgetAgent.validate(ctx, templateCode, contextStringI, "json", currentTry)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		//TODO make retry configurable and part of agent config
		if currentTry < EruWidgetAgent.RetryCount {
			errMsgString := fmt.Sprintf("Error in the gotemplate code. Please try again. \n Error: %s \n Erroneous Tenplate Code generated in previous try: %s", err.Error(), templateCode)
			chatRequest.Messages[1].Content = fmt.Sprint(chatRequest.Messages[1].Content, "\n", errMsgString)
			return EruWidgetAgent.execute(ctx, chatRequest, contextStringI, agentTools, EruWidgetAgent.AgentName, EruWidgetAgent.SystemPrompt, currentTry+1, projectId, tenantId)
		}
		return nil, err
	} */
	agentOutput["retry_count"] = currentTry
	return agentOutput, nil
}

/* func (EruWidgetAgent *EruWidgetAgent) validate(ctx context.Context, templateCode string, contextVars interface{}, outputFormat string, currentTry int) (interface{}, error) {
	logs.WithContext(ctx).Debug("validate - Start")

	output, err := processTemplate(ctx, "template", templateCode, &contextVars, outputFormat, "")
	if err != nil {
		return nil, err
	}
	return output, nil
} */

func (EruWidgetAgent *EruWidgetAgent) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &EruWidgetAgent)
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("json.Unmarshal error : %w", err), "")
		return err
	}
	return nil
}

/* func processTemplate(ctx context.Context, templateName string, templateString string, vars *interface{}, outputType string, tokenHeaderKey string) (output interface{}, err error) {
	logs.WithContext(ctx).Debug("processTemplate - Start")
	goTmpl := gotemplate.GoTemplate{Name: templateName, Template: templateString}
	output, err = goTmpl.ExecuteWithErrors(ctx, vars, outputType)
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("goTmpl.ExecuteWithErrors error : %w", err), "")
		return nil, err
	}
	return
} */
