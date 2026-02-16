package eru_studio

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	agents "github.com/eru-tech/eru/eru-ai/agents"
	models "github.com/eru-tech/eru/eru-ai/models"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eru_utils "github.com/eru-tech/eru/eru-utils"
	"github.com/google/uuid"
	//eru_models "github.com/eru-tech/eru/eru-models"
)

const templateVarsSchemaString = `{"type":"object","properties":{"Headers":{"type":"object"},"FormData":{"type":"object"},"FileData":{"type":"object"},"Params":{"type":"object"},"Vars":{"type":"object","properties":{"Body":{"type":"object"},"OrgBody":{"type":"object"}},"required":[]},"Body":{"type":"object"},"OrgBody":{"type":"object"},"Token":{"type":"object"},"FormDataKeyArray":{"type":"array","items":[{"type":"string"}]},"LoopVars":{"type":"array","items":[{"type":"object"}]},"LoopVar":{"type":"object"},"Cookies":{"type":"object"},"ResponseStatus":{"type":"integer"}},"required":[]}`

type EruWidgetAgent struct {
	agents.Agent
}

func (EruWidgetAgent *EruWidgetAgent) GetSpec() agents.AgentI {
	return EruWidgetAgent
}

func (EruWidgetAgent *EruWidgetAgent) Execute(ctx context.Context, agentMessage agents.AgentMessage, conversationId string, projectId string, tenantId string) (agents.AgentMessage, error) {
	logs.WithContext(ctx).Debug("Agent Execute - Start")
	agentContextString := ""
	contextStringI, contextStringIOk := agentMessage.Params["context"]
	if contextStringIOk {
		if contextString, contextStringOk := contextStringI.(string); contextStringOk {
			agentContextString = contextString
			contextMap := make(map[string]interface{})
			err := json.Unmarshal([]byte(contextString), &contextMap)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return agents.AgentMessage{}, err
			}
			agentMessage.Params["context"] = contextMap
		} else {
			agentContextbytes, err := json.Marshal(agentMessage.Params["context"])
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return agents.AgentMessage{}, err
			}
			agentContextString = string(agentContextbytes)
		}
	}
	contextJsonSchema := eru_utils.GenerateJSONSchema(ctx, agentMessage.Params)
	jsonSchemaString, err := json.Marshal(contextJsonSchema)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return agents.AgentMessage{}, err
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

	/* msg := models.Message{
		Role:    "assistant",
		Content: agentMessage.Content,
		Name:    EruWidgetAgent.AgentName,
	} */
	widgetId := conversationId
	if widgetId == "" {
		widgetId = uuid.New().String()
	}

	if agentContextString != "" {
		agentContextString = fmt.Sprintf("this is the actual data that has been fetched based on user prompt. Analyse the best possible way to display this and is in lines with any specific user's prompt. There could be nil data or empty json, handle these cases gracefully by populating default value provided for data key of properties in respective component. \n\n START OF DATA %s \n\n END OF DATA \n\n", agentContextString)
	}
	contextVariableString = fmt.Sprintf("%s Based on componenet selection, you will have to add the 'data' property into component specific format as required in the component's properties. Ensure that the data is always stringified.\n\n %s \n\n use %s as widget id", agentContextString, contextVariableString, widgetId)

	msg := models.Message{
		Role:    "assistant",
		Content: contextVariableString,
		Name:    EruWidgetAgent.AgentName,
	}
	chatRequest, conversation, err := EruWidgetAgent.LoadConversations(ctx, conversationId, agentMessage, projectId, tenantId)
	if err != nil {
		return agents.AgentMessage{}, err
	}

	chatRequest.Messages = append(chatRequest.Messages, msg)
	agentOutput, err := EruWidgetAgent.execute(ctx, chatRequest, EruWidgetAgent.AgentTools, EruWidgetAgent.AgentName, EruWidgetAgent.SystemPrompt, 1, projectId, tenantId)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return agents.AgentMessage{}, err
	}
	agentOutput.MessageId = agentMessage.MessageId

	conversation.Messages = append(conversation.Messages, agentOutput)
	conversation.NewMessages = append(conversation.NewMessages, agentOutput)
	err = EruWidgetAgent.SaveConversation(ctx, conversation, projectId, tenantId)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("Failed to save conversation: %v", err))
		return agents.AgentMessage{}, err
	}
	return agentOutput, nil
}
func (EruWidgetAgent *EruWidgetAgent) execute(ctx context.Context, chatRequest models.ChatRequest, agentTools []agents.AgentTools, agentName string, systemPrompt string, currentTry int, projectId string, tenantId string) (agentOutput agents.AgentMessage, err error) {

	toolResults, err := EruWidgetAgent.ExecuteTools(ctx, chatRequest, EruWidgetAgent.AgentTools, projectId, tenantId)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return agents.AgentMessage{}, err
	}
	logs.WithContext(ctx).Info(fmt.Sprintf("Tool results: %+v", toolResults))

	agentOutputAction := agents.AgentOutputAction{
		ActionName: "eru_widget",
		Action:     toolResults,
	}
	agentOutputActions := []agents.AgentOutputAction{agentOutputAction}

	agentOutput = agents.AgentMessage{
		Role:             "assistant",
		Actions:          agentOutputActions,
		MessageTimestamp: time.Now(),
	}
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
	agentOutput.RetryCount = currentTry
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
