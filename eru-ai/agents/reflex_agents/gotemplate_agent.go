package reflex_agents

import (
	"context"
	"encoding/json"
	"fmt"

	agents "github.com/eru-tech/eru/eru-ai/agents"
	models "github.com/eru-tech/eru/eru-ai/models"
	tools "github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
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

	contextVariableMap, contextVariableMapOk := agentMessage.Params["context"]
	if !contextVariableMapOk {
		logs.WithContext(ctx).Info("context_variable is not present in the params")

	}

	contextVariable, contextVariableErr := json.Marshal(contextVariableMap)
	if contextVariableErr != nil {
		logs.WithContext(ctx).Error(contextVariableErr.Error())
		return nil, contextVariableErr
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

	contextVariableString := fmt.Sprintf("Use this json as context variable to be used in the gotemplate \n\n %s \n\n", string(contextVariable))

	contextVariablePrompt := `\n\nThere are three attributes in the context variable : \n 
				1. vars : this is JSON object for the current function step and its type is of TemplateVars  \n
				2. req_vars : this is map of string as key and JSON object of type TemplateVars as value. The map key is the name of previous function steps. This holds all the previous REQUEST objects of previous function steps\n
				3. res_vars : this is map of string as key and JSON object of type TemplateVars as value. The map key is the name of previous function steps. This holds all the previous RESPONSE objects of previous function steps\n
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
	response, err := goTemplateAgent.Model.QueryModelWithTool(ctx, chatRequest, goTemplateAgent.Tools, goTemplateAgent.AgentName, goTemplateAgent.SystemPrompt)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
	return response.Content, nil
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
