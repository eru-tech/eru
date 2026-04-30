package reflex_agents

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	agents "github.com/eru-tech/eru/eru-ai/agents"
	models "github.com/eru-tech/eru/eru-ai/models"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	gotemplate "github.com/eru-tech/eru/eru-templates/gotemplate"
	eru_utils "github.com/eru-tech/eru/eru-utils"
	//eru_models "github.com/eru-tech/eru/eru-models"
)

type GoTemplateAgent struct {
	agents.Agent
}

func (goTemplateAgent *GoTemplateAgent) GetSpec() agents.AgentI {
	return goTemplateAgent
}

func (goTemplateAgent *GoTemplateAgent) Execute(ctx context.Context, agentMessage agents.AgentMessage, conversationId string, projectId string, tenantId string) (agents.AgentMessage, error) {
	logs.WithContext(ctx).Debug("Agent Execute - Start")
	contextMap := make(map[string]interface{})
	contextStringI, contextStringIOk := agentMessage.Params["context"]
	if contextStringIOk {
		if contextString, contextStringOk := contextStringI.(string); contextStringOk {
			err := json.Unmarshal([]byte(contextString), &contextMap)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return agents.AgentMessage{}, err
			}
			agentMessage.Params["context"] = contextMap
		} else if contextMapI, contextMapOk := contextStringI.(map[string]interface{}); contextMapOk {
			contextMap = contextMapI
		}
	}
	contextJsonSchema := eru_utils.GenerateJSONSchema(ctx, contextMap)
	jsonSchemaString, err := json.Marshal(contextJsonSchema)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return agents.AgentMessage{}, err
	}

	templateCode, templateCodeOk := agentMessage.Params["code"]
	if !templateCodeOk {
		logs.WithContext(ctx).Info("code is not present in the params")

	}

	templateCodeString, templateCodeStringOk := templateCode.(string)
	if !templateCodeStringOk {
		logs.WithContext(ctx).Info("code is not a string")
	}

	templateCodeString = fmt.Sprintf("This is existing go template code and you need to build on top of this incorporating user's new instructions. If this code is blank, write a new go template code. \n\n %s \n\n", templateCodeString)

	contextVariableString := fmt.Sprintf("Use this json as context variable to be used in the gotemplate \n\n %s \n\n", jsonSchemaString)

	contextVariableString = fmt.Sprint(templateCodeString, contextVariableString, agents.GoTemplateContextVariablePrompt, agents.TemplateVarsSchemaString)

	/* msg := models.Message{
		Role:    "assistant",
		Content: agentMessage.Content,
		Name:    goTemplateAgent.AgentName,
	} */
	msg := models.Message{
		Role:    "system",
		Content: contextVariableString,
		Name:    goTemplateAgent.AgentName,
	}

	chatRequest, conversation, err := goTemplateAgent.LoadConversations(ctx, conversationId, agentMessage, projectId, tenantId)
	if err != nil {
		return agents.AgentMessage{}, err
	}

	chatRequest.Messages = append(chatRequest.Messages, msg)
	agentOutput, err := goTemplateAgent.execute(ctx, chatRequest, contextStringI, goTemplateAgent.AgentTools, goTemplateAgent.AgentName, goTemplateAgent.SystemPrompt, 1, projectId, tenantId)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return agents.AgentMessage{}, err
	}

	agentOutput.MessageId = agentMessage.MessageId
	conversation.Messages = append(conversation.Messages, agentOutput)
	conversation.NewMessages = append(conversation.NewMessages, agentOutput)
	err = goTemplateAgent.SaveConversation(ctx, conversation, projectId, tenantId)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("Failed to save conversation: %v", err))
		return agents.AgentMessage{}, err
	}
	return agentOutput, nil
}
func (goTemplateAgent *GoTemplateAgent) execute(ctx context.Context, chatRequest models.ChatRequest, contextStringI interface{}, agentTools []agents.AgentTools, agentName string, systemPrompt string, currentTry int, projectId string, tenantId string) (agentOutput agents.AgentMessage, err error) {
	toolResults, err := goTemplateAgent.ExecuteTools(ctx, chatRequest, goTemplateAgent.AgentTools, projectId, tenantId)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return agents.AgentMessage{}, err
	}
	logs.WithContext(ctx).Info(fmt.Sprintf("Tool results: %+v", toolResults))
	agentOutputAction := agents.AgentOutputAction{}

	if gotemplate, gotemplateOk := toolResults["gotemplate"].(map[string]interface{}); gotemplateOk {
		if code, codeOk := gotemplate["code"]; codeOk {
			agentOutputAction.Action = map[string]interface{}{"code": code}
		} else {
			agentOutputAction.Action = map[string]interface{}{"code": ""}
		}
	}
	templateCode, templateCodeOk := agentOutputAction.Action["code"].(string)
	if !templateCodeOk {
		logs.WithContext(ctx).Error("code is not present in the params")
		return agents.AgentMessage{}, fmt.Errorf("code is not present in the params")
	}
	var output interface{}
	output, err = goTemplateAgent.validate(ctx, templateCode, contextStringI, "json", currentTry)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		//TODO make retry configurable and part of agent config
		if currentTry < goTemplateAgent.RetryCount {
			errMsgString := fmt.Sprintf("There was error in previous attempt to generate go template **START OF PREVIOUS TEMPLATE** %s **END OF PREVIOUS TEMPLATE**. \n Error: %s \n Please try again and ensure not to generate same template code again and improvize to resolve the error", templateCode, err.Error())
			chatRequest.Messages[1].Content = fmt.Sprint(errMsgString, "\n\n", chatRequest.Messages[1].Content)
			return goTemplateAgent.execute(ctx, chatRequest, contextStringI, agentTools, goTemplateAgent.AgentName, goTemplateAgent.SystemPrompt, currentTry+1, projectId, tenantId)
		}
		return agents.AgentMessage{}, err
	}
	agentOutputAction.Action["output"] = output
	agentOutputActions := []agents.AgentOutputAction{agentOutputAction}
	agentOutput = agents.AgentMessage{
		Role:             "assistant",
		Actions:          agentOutputActions,
		MessageTimestamp: time.Now(),
		RetryCount:       currentTry,
	}
	return agentOutput, nil
}
func (goTemplateAgent *GoTemplateAgent) validate(ctx context.Context, templateCode string, contextVars interface{}, outputFormat string, currentTry int) (interface{}, error) {
	logs.WithContext(ctx).Debug("validate - Start")

	output, err := processTemplate(ctx, "template", templateCode, &contextVars, outputFormat, "")
	if err != nil {
		return nil, err
	}
	return output, nil
}

func (goTemplateAgent *GoTemplateAgent) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &goTemplateAgent)
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("json.Unmarshal error : %w", err), "")
		return err
	}
	return nil
}

func processTemplate(ctx context.Context, templateName string, templateString string, vars *interface{}, outputType string, tokenHeaderKey string) (output interface{}, err error) {
	logs.WithContext(ctx).Debug("processTemplate - Start")
	goTmpl := gotemplate.GoTemplate{Name: templateName, Template: templateString}
	output, err = goTmpl.ExecuteWithErrors(ctx, vars, outputType)
	if err != nil {
		return nil, err
	}
	return
}
