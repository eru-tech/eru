package sql

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	tools "github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eru_models "github.com/eru-tech/eru/eru-models"
	vectorstore "github.com/eru-tech/eru/eru-vectorstore/vectorstore"
)

type SqlAccount struct {
	tools.Tool
	DatabaseType    string                   `json:"database_type"`
	VectorStore     vectorstore.VectorStoreI `json:"-"`
	VectorStoreName string                   `json:"vectorstore_name"`
}

const (
	GenerateSql = "generate_sql"
	Train       = "train"
)

func (sqlAccount *SqlAccount) GetActionsList() []string {
	actions := []string{}
	actions = append(actions, GenerateSql)
	actions = append(actions, Train)
	return actions
}

func (sqlAccount *SqlAccount) GetSpec() tools.Tooling {
	return sqlAccount
}

func (sqlAccount *SqlAccount) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &sqlAccount)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (sqlAccount *SqlAccount) Execute(ctx context.Context, projectId string, tenantId string, actionName string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("SqlAccount Execute - Start")
	switch actionName {
	case GenerateSql:
		return sqlAccount.GenerateSql(ctx, params)
	case Train:
		return sqlAccount.Train(ctx, params)
	default:
		return nil, false, fmt.Errorf("action %s not found", actionName)
	}
}

func (sqlAccount *SqlAccount) BytesToTool(ctx context.Context, toolObjJson []byte) (tools.Tooling, error) {
	sqlAccountWithToken := SqlAccount{}
	err := json.Unmarshal(toolObjJson, &sqlAccountWithToken)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	sqlAccount = &SqlAccount{
		DatabaseType:    sqlAccountWithToken.DatabaseType,
		VectorStoreName: sqlAccountWithToken.VectorStoreName,
	}
	return sqlAccount, nil
}
func (sqlAccount *SqlAccount) GenerateSql(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("SqlAccount GenerateSql - Start")
	return nil, false, nil
}

func (sqlAccount *SqlAccount) Train(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("SqlAccount Train - Start")

	if sqlAccount.VectorStore == nil {
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
	err = sqlAccount.VectorStore.SaveVectors(ctx, vectorRecords)
	if err != nil {
		return nil, false, fmt.Errorf("error saving vectors: %w", err)
	}
	toolResult = map[string]interface{}{
		"message": "vectors saved successfully",
	}
	return toolResult, false, nil
}
func (sqlAccount *SqlAccount) GetAttribute(ctx context.Context, attributeName string) (attributeValue interface{}, err error) {
	switch attributeName {
	case "tool_name":
		return sqlAccount.ToolName, nil
	case "tool_type":
		return sqlAccount.ToolType, nil
	case "system_prompt":
		return sqlAccount.SystemPrompt, nil
	case "output_schema":
		return sqlAccount.OutputSchema, nil
	case "parameters":
		return sqlAccount.Parameters, nil
	case "description":
		return sqlAccount.Description, nil
	case "vectorstore_name":
		return sqlAccount.VectorStoreName, nil
	default:
		err := errors.New("attribute not found")
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
}
func (sqlAccount *SqlAccount) SetAttribute(ctx context.Context, attributeName string, attributeValue interface{}) (err error) {
	switch attributeName {
	case "tool_name":
		sqlAccount.ToolName = attributeValue.(string)
	case "tool_type":
		sqlAccount.ToolType = attributeValue.(string)
	case "system_prompt":
		sqlAccount.SystemPrompt = attributeValue.(string)
	case "output_schema":
		sqlAccount.OutputSchema = attributeValue.(eru_models.JSONSchema)
	case "parameters":
		sqlAccount.Parameters = attributeValue.(eru_models.JSONSchema)
	case "description":
		sqlAccount.Description = attributeValue.(string)
	case "vectorstore":
		sqlAccount.VectorStore = attributeValue.(vectorstore.VectorStoreI)
	default:
		err = errors.New("attribute not found")
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}
func (sqlAccount *SqlAccount) GetBytes(ctx context.Context) ([]byte, error) {

	toolJson, err := json.Marshal(sqlAccount)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	return toolJson, nil
}
