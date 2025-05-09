package reflex_agents

import (
	"context"
	"encoding/json"
	"fmt"

	agents "github.com/eru-tech/eru/eru-ai/agents"
	models "github.com/eru-tech/eru/eru-ai/models"
	tools "github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	gotemplate "github.com/eru-tech/eru/eru-templates/gotemplate"
	eru_utils "github.com/eru-tech/eru/eru-utils"
	//eru_models "github.com/eru-tech/eru/eru-models"
)

const templateVarsSchemaString = `{"type":"object","properties":{"Headers":{"type":"object"},"FormData":{"type":"object"},"FileData":{"type":"object"},"Params":{"type":"object"},"Vars":{"type":"object","properties":{"Body":{"type":"object"},"OrgBody":{"type":"object"}},"required":[]},"Body":{"type":"object"},"OrgBody":{"type":"object"},"Token":{"type":"object"},"FormDataKeyArray":{"type":"array","items":[{"type":"string"}]},"LoopVars":{"type":"array","items":[{"type":"object"}]},"LoopVar":{"type":"object"},"Cookies":{"type":"object"},"ResponseStatus":{"type":"integer"}},"required":[]}`

type GoTemplateAgent struct {
	agents.Agent
}

func (reflex_agent *GoTemplateAgent) GetSpec() agents.AgentI {
	return reflex_agent
}

func (goTemplateAgent *GoTemplateAgent) Execute(ctx context.Context, agentMessage agents.AgentMessage) (map[string]interface{}, error) {
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

	contextVariablePrompt := `\n\nThere are three attributes in the context variable : \n 
				1. Vars or vars : this is JSON object for the current function step and its type is of TemplateVars  \n
				2. ReqVars or req_vars : this is map of string as key and JSON object of type TemplateVars as value. The map key is the name of previous function steps. This holds all the previous REQUEST objects of previous function steps\n
				3. ResVars or res_vars : this is map of string as key and JSON object of type TemplateVars as value. The map key is the name of previous function steps. This holds all the previous RESPONSE objects of previous function steps\n
				TemplateVars JSON schema is as follows : \n\n
				There some few custom functions written by us that we can use in the gotemplate : \n
				1. stringify : this function takes a JSON object and returns a string representation of the JSON object\n
				2. unquote : this function takes a string and returns the unquoted string\n
				`

	contextVariableString = fmt.Sprint(templateCodeString, contextVariableString, contextVariablePrompt, templateVarsSchemaString)

	logs.WithContext(ctx).Info(contextVariableString)

	msg := models.Message{
		Role:    "assistant",
		Content: agentMessage.Content,
		Name:    goTemplateAgent.AgentName,
	}
	msg1 := models.Message{
		Role:    "assistant",
		Content: contextVariableString,
		Name:    goTemplateAgent.AgentName,
	}
	chatRequest := models.ChatRequest{
		Messages: []models.Message{
			msg,
			msg1,
		},
	}
	agentOutput, err = goTemplateAgent.execute(ctx, chatRequest, contextStringI, goTemplateAgent.Tools, goTemplateAgent.AgentName, goTemplateAgent.SystemPrompt, 1)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
	return agentOutput, nil
}
func (goTemplateAgent *GoTemplateAgent) execute(ctx context.Context, chatRequest models.ChatRequest, contextStringI interface{}, tools map[string]tools.Tooling, agentName string, systemPrompt string, currentTry int) (map[string]interface{}, error) {
	agentOutput := make(map[string]interface{})
	response, err := goTemplateAgent.Model.QueryModelWithTool(ctx, chatRequest, goTemplateAgent.Tools, goTemplateAgent.AgentName, goTemplateAgent.SystemPrompt)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
	agentResponse := response.Content["raw"].(map[string]interface{})
	if gotemplate, gotemplateOk := agentResponse["gotemplate"].(map[string]interface{}); gotemplateOk {
		if code, codeOk := gotemplate["code"]; codeOk {
			agentOutput["code"] = code
		}
	}
	templateCode := agentOutput["code"].(string)
	var output interface{}
	output, err = goTemplateAgent.validate(ctx, templateCode, contextStringI, "json", currentTry)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		//TODO make retry configurable and part of agent config
		if currentTry < goTemplateAgent.RetryCount {
			errMsgString := fmt.Sprintf("Error in the gotemplate code. Please try again. \n Error: %s \n Erroneous Tenplate Code generated in previous try: %s", err.Error(), templateCode)
			chatRequest.Messages[1].Content = fmt.Sprint(chatRequest.Messages[1].Content, "\n", errMsgString)
			return goTemplateAgent.execute(ctx, chatRequest, contextStringI, goTemplateAgent.Tools, goTemplateAgent.AgentName, goTemplateAgent.SystemPrompt, currentTry+1)
		}
		return nil, err
	}
	agentOutput["output"] = output
	agentOutput["retry_count"] = currentTry
	return agentOutput, nil
}
func (goTemplateAgent *GoTemplateAgent) validate(ctx context.Context, templateCode string, contextVars interface{}, outputFormat string, currentTry int) (interface{}, error) {
	logs.WithContext(ctx).Debug("validate - Start")

	output, err := processTemplate(ctx, "template", templateCode, &contextVars, outputFormat, "")
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
	return output, nil
}

func (goTemplateAgent *GoTemplateAgent) callTool(ctx context.Context, projectId string, tenantId string, tool tools.Tooling, params map[string]interface{}) (map[string]interface{}, error) {
	logs.WithContext(ctx).Debug("callTool - Start")
	return tool.Execute(ctx, projectId, tenantId, "", params)
}

func (goTemplateAgent *GoTemplateAgent) callModel(ctx context.Context, model models.ModelI, params map[string]interface{}) (map[string]interface{}, error) {
	logs.WithContext(ctx).Debug("callModel - Start")
	return nil, nil
}

func (goTemplateAgent *GoTemplateAgent) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &goTemplateAgent)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func processTemplate(ctx context.Context, templateName string, templateString string, vars *interface{}, outputType string, tokenHeaderKey string) (output interface{}, err error) {
	logs.WithContext(ctx).Debug("processTemplate - Start")
	goTmpl := gotemplate.GoTemplate{Name: templateName, Template: templateString}
	output, err = goTmpl.Execute(ctx, vars, outputType)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
	return
}
