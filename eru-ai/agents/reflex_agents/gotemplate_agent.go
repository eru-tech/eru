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
				There are many custom functions written by us that we can use in the gotemplate : \n
				
				JSON Functions:
				1. stringify : this function takes a JSON object and returns a string representation of the JSON object\n
				2. unquote : this function takes a string and returns the unquoted string\n
				3. marshalJSON : marshals interface{} to JSON bytes\n
				4. unmarshalJSON : unmarshals JSON bytes to interface{}\n
				
				Encoding Functions:
				5. b64Encode : base64 encode bytes to string\n
				6. b64Decode : base64 decode string to string\n
				7. hexEncode : hex encode bytes to string\n
				8. hexDecode : hex decode string to string\n
				
				Crypto Functions:
				9. aesEncryptGCM : AES GCM encryption\n
				10. aesDecryptGCM : AES GCM decryption\n
				11. aesEncryptECB : AES ECB encryption\n
				12. aesDecryptECB : AES ECB decryption\n
				13. aesEncryptCBC : AES CBC encryption\n
				14. aesDecryptCBC : AES CBC decryption\n
				15. generateAesKey : generate AES key of specified bits\n
				16. generate_rsa_keypair : generate RSA key pair\n
				17. encryptRSACert : encrypt with RSA certificate\n
				18. hmac : HMAC with secret\n
				19. shaHash : SHA hash with specified bits (256, 512)\n
				20. md5 : MD5 hash\n
				21. PKCS7Pad : PKCS7 padding\n
				22. PKCS7Unpad : PKCS7 unpadding\n
				23. new_jwt : create JWT token\n
				
				String/Data Functions:
				24. bytesToString : convert bytes to string\n
				25. stringToByte : convert string to bytes\n
				26. doubleQuote : double quote a string\n
				27. len : get length of data\n
				28. str_concat : concatenate strings with separator\n
				29. str_replace : replace string occurrences\n
				30. removenull : remove null characters\n
				31. char_index : find character index in string\n
				
				Map/Array Functions:
				32. saveVar : save variable to map\n
				33. concatMapKeyVal : concatenate map key-value pairs\n
				34. concatMapKeyValUnordered : concatenate map unordered\n
				35. makeMapKeyValUnordered : create map from string\n
				36. overwriteMap : overwrite map with new data\n
				37. removeMapKey : remove key from map\n
				38. getMapValue : get value from map by key\n
				39. getMapKeys : get all keys from map\n
				40. getMapPointerValue : get pointer value from map\n
				41. getArrayValue : get array value by index\n
				42. arrayLen : get array length\n
				43. is_array : check if variable is array\n
				44. sortMapArray : sort array of maps by key\n
				
				Math Functions:
				45. math_add : add multiple numbers\n
				46. math_sub : subtract two numbers\n
				47. math_div : divide two numbers\n
				48. math_mul : multiply two numbers\n
				49. math_round : round number with precision\n
				
				Date/Time Functions:
				50. current_date : get current date\n
				51. date_diff : add/subtract days/months/years from date\n
				52. date_part : extract part from date (DAY, MONTH, YEAR)\n
				53. date_format : format date from one layout to another\n
				
				Utility Functions:
				54. uuid : generate UUID\n
				55. null : return null value\n
				56. inc : increment number by 1\n
				57. logobject : log object to console\n
				58. logstring : log string to console\n
				59. logerror : log error to console\n
				
				Template/Filter Functions:
				60. evalFilter : evaluate filter against record\n
				61. makeFilter : create SQL filter from JSON\n
				62. makeParentFilter : create parent SQL filter\n
				63. fetch_filter_keys : fetch filter keys\n
				64. execTemplate : execute sub-template\n
				
				File/Data Processing:
				65. excelToJson : convert Excel data to JSON\n
				66. jsonToCsv : convert JSON to CSV string\n
				67. jsonToCsvB64 : convert JSON to base64 CSV\n
				68. kmsDecrypt : decrypt using KMS\n
				69. getObjDiff : get difference between objects\n
				
				All Sprig template functions are also available (date, string, math, flow control functions)\n
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

func (goTemplateAgent *GoTemplateAgent) callTool(ctx context.Context, projectId string, tenantId string, tool tools.Tooling, params map[string]interface{}) (map[string]interface{}, bool, error) {
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
	output, err = goTmpl.ExecuteWithErrors(ctx, vars, outputType)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
	return
}
