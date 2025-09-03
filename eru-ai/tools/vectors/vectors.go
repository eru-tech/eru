package vectors

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	tools "github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eru_models "github.com/eru-tech/eru/eru-models"
	utils "github.com/eru-tech/eru/eru-utils"
	vectorstore "github.com/eru-tech/eru/eru-vectorstore/vectorstore"
)

type VectorstoreAccount struct {
	tools.Tool
	VectorStore     vectorstore.VectorStoreI `json:"-"`
	VectorStoreName string                   `json:"vectorstore_name"`
}

const (
	SaveVectors   = "save_vectors"
	SearchVectors = "search_vectors"
)

var vectorstoreAccountActions = []tools.ToolAction{
	{
		ActionName:   SaveVectors,
		Description:  "Train the model",
		SystemPrompt: "Train the model",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(vectorstore.VectorRecords{}))
		},
	},
	{
		ActionName:   SearchVectors,
		Description:  "Search the model",
		SystemPrompt: "Search the model",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(vectorstore.VectorRecordsSearch{}))
		},
	},
}

func (vectorstoreAccount *VectorstoreAccount) GetActionsList() []string {
	actionNames := []string{}
	for _, action := range vectorstoreAccountActions {
		actionNames = append(actionNames, action.ActionName)
	}
	return actionNames
}

func (vectorstoreAccount *VectorstoreAccount) GetSpec() tools.Tooling {
	return vectorstoreAccount
}

func (vectorstoreAccount *VectorstoreAccount) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &vectorstoreAccount)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (vectorstoreAccount *VectorstoreAccount) Execute(ctx context.Context, projectId string, tenantId string, actionName string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("vectorstoreAccount Execute - Start")
	switch actionName {
	case SaveVectors:
		return vectorstoreAccount.SaveVectors(ctx, params)
	case SearchVectors:
		return vectorstoreAccount.SearchVectors(ctx, params)
	default:
		return nil, false, fmt.Errorf("action %s not found", actionName)
	}
}

func (vectorstoreAccount *VectorstoreAccount) BytesToTool(ctx context.Context, toolObjJson []byte) (tools.Tooling, error) {
	err := json.Unmarshal(toolObjJson, &vectorstoreAccount)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	return vectorstoreAccount, nil
}

func (vectorstoreAccount *VectorstoreAccount) SaveVectors(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("vectorstoreAccount Train - Start")

	if vectorstoreAccount.VectorStore == nil {
		return nil, false, fmt.Errorf("vectorstore not found")
	}

	vectorRecordsBytes, err := json.Marshal(params)
	if err != nil {
		return nil, false, fmt.Errorf("error marshalling vectorrecords: %w", err)
	}
	vectorRecords := vectorstore.VectorRecords{}
	err = json.Unmarshal(vectorRecordsBytes, &vectorRecords)
	if err != nil {
		return nil, false, fmt.Errorf("error unmarshalling vectorrecords: %w", err)
	}
	err = vectorstoreAccount.VectorStore.SaveVectors(ctx, vectorRecords)
	if err != nil {
		return nil, false, fmt.Errorf("error saving vectors: %w", err)
	}
	toolResult = map[string]interface{}{
		"message": "vectors saved successfully",
	}
	return toolResult, false, nil
}
func (vectorstoreAccount *VectorstoreAccount) SearchVectors(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("vectorstoreAccount Search - Start")
	if vectorstoreAccount.VectorStore == nil {
		return nil, false, fmt.Errorf("vectorstore not found")
	}
	if vectorSearchParams, vectorSearchParamsOk := params["params"]; vectorSearchParamsOk {
		vectorSearchBytes, err := json.Marshal(vectorSearchParams)
		if err != nil {
			return nil, false, fmt.Errorf("error marshalling vectorrecords: %w", err)
		}
		vectorSearch := vectorstore.VectorRecordsSearch{}
		err = json.Unmarshal(vectorSearchBytes, &vectorSearch)
		if err != nil {
			return nil, false, fmt.Errorf("error unmarshalling vectorrecords: %w", err)
		}
		vectorResults, err := vectorstoreAccount.VectorStore.SearchVectors(ctx, vectorSearch)
		if err != nil {
			return nil, false, fmt.Errorf("error searching vectors: %w", err)
		}
		toolResult = map[string]interface{}{
			"vector_search": vectorResults,
		}
	} else {
		return nil, false, fmt.Errorf("params not found")
	}
	return toolResult, false, nil
}
func (vectorstoreAccount *VectorstoreAccount) GetAttribute(ctx context.Context, attributeName string) (attributeValue interface{}, err error) {
	switch attributeName {
	case "tool_name":
		return vectorstoreAccount.ToolName, nil
	case "tool_type":
		return vectorstoreAccount.ToolType, nil
	case "system_prompt":
		return vectorstoreAccount.SystemPrompt, nil
	case "output_schema":
		return vectorstoreAccount.OutputSchema, nil
	case "parameters":
		return vectorstoreAccount.Parameters, nil
	case "description":
		return vectorstoreAccount.Description, nil
	case "vectorstore_name":
		return vectorstoreAccount.VectorStoreName, nil
	default:
		err := errors.New("attribute not found")
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
}
func (vectorstoreAccount *VectorstoreAccount) SetAttribute(ctx context.Context, attributeName string, attributeValue interface{}) (err error) {
	switch attributeName {
	case "tool_name":
		vectorstoreAccount.ToolName = attributeValue.(string)
	case "tool_type":
		vectorstoreAccount.ToolType = attributeValue.(string)
	case "system_prompt":
		vectorstoreAccount.SystemPrompt = attributeValue.(string)
	case "output_schema":
		vectorstoreAccount.OutputSchema = attributeValue.(eru_models.JSONSchema)
	case "parameters":
		vectorstoreAccount.Parameters = attributeValue.(eru_models.JSONSchema)
	case "description":
		vectorstoreAccount.Description = attributeValue.(string)
	case "vectorstore":
		vectorstoreAccount.VectorStore = attributeValue.(vectorstore.VectorStoreI)
	default:
		err = errors.New("attribute not found")
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}
func (vectorstoreAccount *VectorstoreAccount) GetBytes(ctx context.Context) ([]byte, error) {

	toolJson, err := json.Marshal(vectorstoreAccount)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	return toolJson, nil
}

func (vectorstoreAccount *VectorstoreAccount) SetToolAction(actionName string) {
	for _, action := range vectorstoreAccountActions {
		if action.ActionName == actionName {
			vectorstoreAccount.ToolAction = action
			return
		}
	}
	vectorstoreAccount.ToolAction = tools.ToolAction{}
}

/* func (vectorstoreAccount *VectorstoreAccount) GetParameters() eru_models.JSONSchema {
	return vectorstoreAccount.ToolAction.GetParameters()
} */
