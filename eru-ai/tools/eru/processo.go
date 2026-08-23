package eru

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strconv"

	tools "github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eru_models "github.com/eru-tech/eru/eru-models"
	server "github.com/eru-tech/eru/eru-server/server"
	utils "github.com/eru-tech/eru/eru-utils"
)

type ProcessoSaveApiParams struct {
	ApiName     string                 `json:"api_name" eru:"required" desc:"unique name of the api to save"`
	ApiCategory string                 `json:"api_category" eru:"required" desc:"category of the api"`
	OrgId       string                 `json:"org_id" eru:"required" desc:"organization id"`
	ProcessId   string                 `json:"process_id" eru:"required" desc:"process id"`
	ApiDef      map[string]interface{} `json:"api_def" eru:"required" desc:"api definition (function group json with func_category_name, func_group_name, func_steps)"`
}

type ProcessoExecuteApiParams struct {
	ApiName   string                 `json:"api_name" eru:"required" desc:"name of the api to execute"`
	OrgId     string                 `json:"org_id" eru:"required" desc:"organization id"`
	ProcessId string                 `json:"process_id" eru:"required" desc:"process id"`
	Body      map[string]interface{} `json:"body" desc:"additional key value pairs required by the api" default:"{}"`
}

type ProcessoGetApiParams struct {
	OrgId     string `json:"org_id" eru:"required" desc:"organization id"`
	ProcessId string `json:"process_id" eru:"required" desc:"process id"`
	ApiId     string `json:"api_id" desc:"api id to fetch (either api_id or api_name must be provided)"`
	ApiName   string `json:"api_name" desc:"api name to fetch (either api_id or api_name must be provided)"`
}

type ProcessoExecuteQueryParams struct {
	OrgId     string                 `json:"org_id" eru:"required" desc:"organization id"`
	ProcessId string                 `json:"process_id" eru:"required" desc:"process id"`
	QueryName string                 `json:"query_name" eru:"required" desc:"name of the query to execute"`
	Body      map[string]interface{} `json:"body" desc:"additional key value pairs (query variables) required by the query" default:"{}"`
}

type ProcessoSaveQueryParams struct {
	QueryId     string `json:"query_id" desc:"query id (provided when updating an existing query)"`
	OrgId       string `json:"org_id" eru:"required" desc:"organization id"`
	ProcessId   string `json:"process_id" eru:"required" desc:"process id"`
	QueryName   string `json:"query_name" eru:"required" desc:"name of the query"`
	QueryString string `json:"query_string" eru:"required" desc:"the query string to save"`
	DbAlias     string `json:"db_alias" eru:"required" desc:"database alias to execute the query against"`
	QueryVars   string `json:"query_vars" desc:"query variables as a json string" default:"{}"`
	QueryType   string `json:"query_type" eru:"required" desc:"type of the query (e.g. sql)"`
}

type ProcessoGetQueryParams struct {
	OrgId     string `json:"org_id" eru:"required" desc:"organization id"`
	ProcessId string `json:"process_id" eru:"required" desc:"process id"`
	QueryId   string `json:"query_id" desc:"query id to fetch (either query_id or query_name must be provided)"`
	QueryName string `json:"query_name" desc:"query name to fetch (either query_id or query_name must be provided)"`
}

type ProcessoEntityData struct {
	Name        string `json:"name" eru:"required" desc:"unique name of the entity"`
	Index       int    `json:"index" desc:"display order index of the entity"`
	IsPeople    string `json:"is_people" desc:"whether the entity represents people - \"true\" or \"false\""`
	DisplayName string `json:"display_name" desc:"display name of the entity"`
	Description string `json:"description" desc:"description of what the entity stores"`
	HideEntity  string `json:"hide_entity" desc:"whether the entity is hidden - \"true\" or \"false\""`
	IsConv      bool   `json:"is_conv" desc:"whether the entity is a conversation entity"`
	Exet        bool   `json:"exet" desc:"whether the entity is an external entity"`
}

type ProcessoSaveEntityParams struct {
	OrgId       string               `json:"org_id" eru:"required" desc:"organization id"`
	ProcessId   string               `json:"process_id" eru:"required" desc:"process id"`
	ProcessName string               `json:"process_name" eru:"required" desc:"name of the process"`
	EntityData  []ProcessoEntityData `json:"entity_data" eru:"required" desc:"list of entities metadata to add or edit"`
}

type ProcessoFieldOption struct {
	OptionName string `json:"name" eru:"required" desc:"option value shown to the user"`
}

type ProcessoFieldStatus struct {
	StatusName  string `json:"name" eru:"required" desc:"status name"`
	StatusColor string `json:"color" eru:"required" desc:"status color as a hex code e.g. #ffea16"`
	StatusDf    *bool  `json:"df" desc:"true if this is the default status of the list, else null"`
}

type ProcessoFieldDef struct {
	Name             string                `json:"name" eru:"required" desc:"unique name of the field"`
	Label            string                `json:"label" eru:"required" desc:"display label of the field"`
	Datatype         string                `json:"datatype" eru:"required" desc:"datatype of the field - textbox, date, email, dropdown_single_select, dropdown_multi_select, status"`
	TabName          string                `json:"tab_name" eru:"required" desc:"name of the tab under which the field is displayed"`
	Description      string                `json:"description" desc:"description of the field"`
	Default          string                `json:"default" desc:"default value of the field"`
	ToolTip          string                `json:"tool_tip" desc:"tool tip displayed for the field"`
	CanEdit          bool                  `json:"can_edit" desc:"whether the field definition can be edited"`
	ShowMobile       bool                  `json:"show_mobile" desc:"whether the field is shown on mobile"`
	CanGroup         bool                  `json:"can_group" desc:"whether records can be grouped by this field"`
	Mandatory        bool                  `json:"mandatory" desc:"whether the field is mandatory"`
	IsHidden         bool                  `json:"is_hidden" desc:"whether the field is hidden"`
	Editable         bool                  `json:"editable" desc:"whether the field value can be edited after creation"`
	IsUnique         bool                  `json:"is_unique" desc:"whether the field value must be unique"`
	UnqSa            bool                  `json:"unq_sa" desc:"whether uniqueness is checked across all statuses"`
	ShowGrid         string                `json:"show_grid" desc:"whether the field is shown in the grid - yes or no"`
	GridIndex        int                   `json:"grid_index" desc:"position of the field in the grid"`
	DefaultGroup     bool                  `json:"default_group" desc:"whether the field is the default grouping field"`
	IsPii            bool                  `json:"is_pii" desc:"whether the field holds personally identifiable information"`
	ToEncrypt        bool                  `json:"to_encrypt" desc:"whether the field value is encrypted at rest"`
	IsEphi           bool                  `json:"is_ephi" desc:"whether the field holds electronic protected health information"`
	IsPf             bool                  `json:"is_pf" desc:"whether the field holds financial information"`
	Dfc              bool                  `json:"dfc" desc:"whether the field is a derived/computed field"`
	Ce               string                `json:"ce" desc:"conditional expression applied on the field"`
	Cef              string                `json:"cef" desc:"conditional expression field"`
	Cefa             string                `json:"cefa" desc:"conditional expression field aggregation e.g. min, max"`
	TfIdx            int                   `json:"tf_idx" desc:"position of the field in the tab/form"`
	UniqueFn         []string              `json:"unique_fn" desc:"list of field names forming a composite uniqueness check - applicable for textbox and email"`
	DataLength       string                `json:"data_length" desc:"allowed data length as a number - applicable for textbox"`
	DataLengthCheck  string                `json:"data_length_check" desc:"how data_length is enforced - applicable for textbox"`
	DateFormat       string                `json:"date_format" desc:"date format e.g. dd-MM-YYYY - mandatory for datatype date"`
	AllowDays        []string              `json:"allow_days" desc:"days allowed for selection - Mon, Tue, Wed, Thu, Fri, Sat, Sun - applicable for datatype date"`
	SystemValidate   string                `json:"system_validate" desc:"whether the email is validated by the system - true or false - applicable for datatype email"`
	SsvApi           string                `json:"ssv_api" desc:"api used to validate the email when system_validate is true - applicable for datatype email"`
	FieldOptionType  string                `json:"option_type" desc:"source of the dropdown options - STATIC, ENTITY_DATA or API - mandatory for dropdown datatypes"`
	ApiName          string                `json:"api_name" desc:"api providing the options when option_type is API"`
	ApiField         string                `json:"api_field" desc:"api response field providing the options when option_type is API"`
	OptionEntityName string                `json:"entity_name" desc:"entity providing the options when option_type is ENTITY_DATA"`
	OptionFieldName  string                `json:"field_name" desc:"field of the option entity providing the options when option_type is ENTITY_DATA"`
	Options          []ProcessoFieldOption `json:"options" desc:"static list of options when option_type is STATIC"`
	OpenStatus       []ProcessoFieldStatus `json:"open_status" desc:"open statuses - mandatory for datatype status"`
	CloseStatus      []ProcessoFieldStatus `json:"close_status" desc:"close statuses - mandatory for datatype status"`
	Sts              string                `json:"_sts" desc:"reserved status attribute - applicable for datatype status"`
}

type ProcessoSaveFieldParams struct {
	OrgId        string           `json:"org_id" eru:"required" desc:"organization id"`
	ProcessId    string           `json:"process_id" eru:"required" desc:"process id"`
	EntityName   string           `json:"entity_name" eru:"required" desc:"name of the entity the field belongs to"`
	ProcessName  string           `json:"process_name" eru:"required" desc:"name of the process"`
	ParentEntity string           `json:"parent_entity" desc:"option source entity - set when the field options come from another entity"`
	ChildField   string           `json:"child_field" desc:"name of the field being saved - defaults to field.name"`
	OptionType   string           `json:"option_type" desc:"option source of the field - STATIC, ENTITY_DATA or API - defaults to field.option_type"`
	ParentField  string           `json:"parent_field" desc:"option source field of the parent entity - set when option_type is ENTITY_DATA"`
	Field        ProcessoFieldDef `json:"field" eru:"required" desc:"field metadata to add or edit"`
}

type ProcessoSaveEntityDataParams struct {
	OrgId         string                 `json:"org_id" eru:"required" desc:"organization id"`
	ProcessId     string                 `json:"process_id" eru:"required" desc:"process id"`
	ProcessName   string                 `json:"process_name" eru:"required" desc:"name of the process"`
	EntityName    string                 `json:"entity_name" eru:"required" desc:"name of the entity whose record is being saved"`
	EntityData    map[string]interface{} `json:"entity_data" eru:"required" desc:"record to save as field name and value pairs"`
	OldEntityData map[string]interface{} `json:"old_entity_data" desc:"existing record as field name and value pairs - mandatory when is_edit is true"`
	Emd           map[string]interface{} `json:"emd" desc:"extra metadata to be saved along with the record"`
	PeopleUsers   []interface{}          `json:"people_users" desc:"users to be tagged against the people fields of the record"`
	IsEdit        bool                   `json:"is_edit" desc:"true when an existing record is being edited, false when a new record is being added"`
	EntityId      string                 `json:"entity_id" desc:"id of the record being edited - mandatory when is_edit is true"`
	PEntities     []interface{}          `json:"p_entities" desc:"parent entities linked to this record"`
	PCreatedBy    []interface{}          `json:"p_created_by" desc:"created by details of the parent entity records"`
	DfcPEntities  []interface{}          `json:"dfc_p_entities" desc:"parent entities of the derived/computed fields of this record"`
	HasAm         bool                   `json:"has_am" desc:"whether an authorization matrix is configured for this entity"`
	HasWfAm       bool                   `json:"has_wf_am" desc:"whether a workflow with an authorization matrix is configured for this entity"`
	HasWfOth      bool                   `json:"has_wf_oth" desc:"whether any other workflow is configured for this entity"`
}

type ProcessoDeleteEntityDataParams struct {
	OrgId         string        `json:"org_id" eru:"required" desc:"organization id"`
	ProcessId     string        `json:"process_id" eru:"required" desc:"process id"`
	ProcessName   string        `json:"process_name" eru:"required" desc:"name of the process"`
	EntityName    string        `json:"entity_name" eru:"required" desc:"name of the entity whose records are being deleted"`
	EntityId      []string      `json:"entity_id" eru:"required" desc:"list of record ids to delete"`
	ChildEntities []interface{} `json:"child_entities" desc:"child entities whose linked records are to be deleted along with the record"`
}

type ProcessoSaveEntityVisibilityParams struct {
	OrgId          string   `json:"org_id" eru:"required" desc:"organization id"`
	ProcessId      string   `json:"process_id" eru:"required" desc:"process id"`
	ProcessName    string   `json:"process_name" eru:"required" desc:"name of the process"`
	EntityName     string   `json:"entity_name" eru:"required" desc:"name of the entity whose visibility is being saved"`
	VisibilityType string   `json:"visibility_type" eru:"required" desc:"visibility of the entity - public makes it visible to everyone, private restricts it to the mapped users and roles"`
	MapUsers       []string `json:"map_users" desc:"user ids the entity is visible to - applicable when visibility_type is private"`
	MapRoles       []string `json:"map_roles" desc:"roles the entity is visible to - applicable when visibility_type is private"`
	EnvMapUsers    []string `json:"env_map_users" desc:"user ids the entity is visible to in the environment - must be a subset of map_users"`
	EnvMapRoles    []string `json:"env_map_roles" desc:"roles the entity is visible to in the environment - must be a subset of map_roles"`
}

type ProcessoSaveEntityRecordVisibilityParams struct {
	OrgId           string                 `json:"org_id" eru:"required" desc:"organization id"`
	ProcessId       string                 `json:"process_id" eru:"required" desc:"process id"`
	ProcessName     string                 `json:"process_name" eru:"required" desc:"name of the process"`
	EntityName      string                 `json:"entity_name" eru:"required" desc:"name of the entity whose record visibility is being saved"`
	VisibilityType  string                 `json:"visibility_type" eru:"required" desc:"record visibility of the entity - public makes all records visible to everyone, private restricts records to the mapped users and roles"`
	MapRoles        []string               `json:"map_roles" desc:"roles the records are visible to - applicable when visibility_type is private"`
	MapUsers        []string               `json:"map_users" desc:"user ids the records are visible to - applicable when visibility_type is private"`
	UserOpFn        []interface{}          `json:"user_op_fn" desc:"operations allowed to the mapped users and roles on the visible records"`
	UserAttFilter   map[string]interface{} `json:"user_att_filter" desc:"record filter on the logged in user attributes as attribute name and value pairs"`
	Cond            string                 `json:"cond" desc:"condition joining the user_att_filter attributes e.g. and, or"`
	ParentEntities  []interface{}          `json:"parent_entities" desc:"parent entities through which the record visibility is derived"`
	ParentAttFilter map[string]interface{} `json:"parent_att_filter" desc:"record filter on the parent entity attributes as attribute name and value pairs"`
	PjCond          string                 `json:"pj_cond" desc:"condition joining the parent_att_filter attributes e.g. and, or"`
}

type ProcessoSaveEntityDownloadVisibilityParams struct {
	OrgId          string   `json:"org_id" eru:"required" desc:"organization id"`
	ProcessId      string   `json:"process_id" eru:"required" desc:"process id"`
	ProcessName    string   `json:"process_name" eru:"required" desc:"name of the process"`
	EntityName     string   `json:"entity_name" eru:"required" desc:"name of the entity whose download visibility is being saved"`
	VisibilityType string   `json:"visibility_type" eru:"required" desc:"download visibility of the entity - public allows everyone to download, private restricts the download to the mapped users and roles"`
	MapUsers       []string `json:"map_users" desc:"user ids allowed to download - applicable when visibility_type is private"`
	MapRoles       []string `json:"map_roles" desc:"roles allowed to download - applicable when visibility_type is private"`
}

type ProcessoTool struct {
	tools.Tool
	ProjectId string `json:"project_id" desc:"processo project id used in the url path" default:"processo"`
}

const (
	SaveApi                              = "save_api"
	ExecuteApi                           = "execute_api"
	GetApi                               = "get_api"
	ProcessoExecQuery                    = "execute_query"
	ProcessoSaveQueryGr                  = "save_query"
	ProcessoGetQuery                     = "get_query"
	ProcessoSaveEntity                   = "save_entity"
	ProcessoSaveField                    = "save_field"
	ProcessoSaveEntityData               = "save_entity_data"
	ProcessoDeleteEntityData             = "delete_entity_data"
	ProcessoSaveEntityVisibility         = "save_entity_visibility"
	ProcessoSaveEntityRecordVisibility   = "save_entity_record_visibility"
	ProcessoSaveEntityDownloadVisibility = "save_entity_download_visibility"
)

var processoToolActions = []tools.ToolAction{
	{
		ActionName:   SaveApi,
		Description:  "Save an api definition (function group) under processo for an org and process",
		SystemPrompt: "This tool saves an api definition (function group) under processo for an org and process. The api_def follows the eru function group json structure with func_category_name, func_group_name and func_steps.",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(ProcessoSaveApiParams{}), []string{})
		},
	},
	{
		ActionName:   ExecuteApi,
		Description:  "Execute a previously saved processo api by name with additional key value pairs as required by the api",
		SystemPrompt: "This tool executes a previously saved processo api by name. Pass api_name, org_id, process_id and any other key value pairs required by the api via the body attribute.",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(ProcessoExecuteApiParams{}), []string{})
		},
	},
	{
		ActionName:   GetApi,
		Description:  "Fetch a saved api definition for an org and process by api_id or api_name",
		SystemPrompt: "This tool fetches a saved api definition for an org and process. Pass org_id, process_id and either api_id or api_name.",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(ProcessoGetApiParams{}), []string{})
		},
	},
	{
		ActionName:   ProcessoExecQuery,
		Description:  "Execute a saved processo query by name for an org and process",
		SystemPrompt: "This tool executes a saved processo query by name. Pass org_id, process_id, query_name and any query variables via the body attribute.",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(ProcessoExecuteQueryParams{}), []string{})
		},
	},
	{
		ActionName:   ProcessoSaveQueryGr,
		Description:  "Save a query (query group) under processo for an org and process",
		SystemPrompt: "This tool saves a query under processo for an org and process. Pass query_name, query_string, db_alias, query_type and optionally query_id and query_vars.",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(ProcessoSaveQueryParams{}), []string{})
		},
	},
	{
		ActionName:   ProcessoGetQuery,
		Description:  "Fetch a saved query for an org and process by query_id or query_name",
		SystemPrompt: "This tool fetches a saved query for an org and process. Pass org_id, process_id and either query_id or query_name.",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(ProcessoGetQueryParams{}), []string{})
		},
	},
	{
		ActionName:   ProcessoSaveEntity,
		Description:  "allows user to add/edit entities metadata",
		SystemPrompt: "This tool adds or edits entities metadata under processo for an org and process. Pass org_id, process_id, process_name and entity_data as a list of entities with name, index, is_people, display_name, description, hide_entity, is_conv and exet.",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(ProcessoSaveEntityParams{}), []string{})
		},
	},
	{
		ActionName:   ProcessoSaveField,
		Description:  "allows user to define new field or edit exisitng field meta data for a given entity under a particular organization and process",
		SystemPrompt: "This tool adds or edits a field metadata of an entity under processo for an org and process. Pass org_id, process_id, process_name, entity_name and the field object. The field attributes depend on datatype - date needs date_format and allow_days, email needs system_validate and ssv_api, dropdown datatypes need option_type with options (STATIC) or entity_name and field_name (ENTITY_DATA) or api_name and api_field (API), status needs open_status and close_status.",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(ProcessoSaveFieldParams{}), []string{})
		},
	},
	{
		ActionName:   ProcessoSaveEntityData,
		Description:  "allows user to add / edit an entity record - on successfull save, it will trigger authorization matrix and/or workflow as configured",
		SystemPrompt: "This tool adds or edits a record of an entity under processo for an org and process. Pass org_id, process_id, process_name, entity_name and entity_data as field name and value pairs. For an edit pass is_edit as true along with entity_id and old_entity_data. On a successful save the configured authorization matrix and/or workflow is triggered based on has_am, has_wf_am and has_wf_oth.",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(ProcessoSaveEntityDataParams{}), []string{})
		},
	},
	{
		ActionName:   ProcessoDeleteEntityData,
		Description:  "allows user to delete one or more entity records along with their linked child entity records",
		SystemPrompt: "This tool deletes one or more records of an entity under processo for an org and process. Pass org_id, process_id, process_name, entity_name and entity_id as a list of record ids. Pass child_entities to also delete the records linked to this record in those child entities.",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(ProcessoDeleteEntityDataParams{}), []string{})
		},
	},
	{
		ActionName:   ProcessoSaveEntityVisibility,
		Description:  "allows user to define who can view an entity by mapping users and roles to it",
		SystemPrompt: "This tool saves the visibility of an entity under processo for an org and process. Pass org_id, process_id, process_name, entity_name and visibility_type. When visibility_type is private, pass map_users and/or map_roles with the users and roles the entity is visible to, and env_map_users and env_map_roles as their environment level subsets.",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(ProcessoSaveEntityVisibilityParams{}), []string{})
		},
	},
	{
		ActionName:   ProcessoSaveEntityRecordVisibility,
		Description:  "allows user to define which records of an entity are visible to the mapped users and roles",
		SystemPrompt: "This tool saves the record level visibility of an entity under processo for an org and process. Pass org_id, process_id, process_name, entity_name and visibility_type. When visibility_type is private, pass map_users and/or map_roles along with user_op_fn for the allowed operations, user_att_filter with cond to filter records by the logged in user attributes and parent_entities with parent_att_filter and pj_cond to filter records through the parent entities.",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(ProcessoSaveEntityRecordVisibilityParams{}), []string{})
		},
	},
	{
		ActionName:   ProcessoSaveEntityDownloadVisibility,
		Description:  "allows user to define who can download the records of an entity by mapping users and roles to it",
		SystemPrompt: "This tool saves the download visibility of an entity under processo for an org and process. Pass org_id, process_id, process_name, entity_name and visibility_type. When visibility_type is private, pass map_users and/or map_roles with the users and roles allowed to download the entity records.",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(ProcessoSaveEntityDownloadVisibilityParams{}), []string{})
		},
	},
}

func (processoTool *ProcessoTool) GetActionsList() []tools.ActionInfo {
	infos := make([]tools.ActionInfo, len(processoToolActions))
	for i, action := range processoToolActions {
		infos[i] = tools.ActionInfo{Name: action.ActionName, Description: action.Description}
	}
	return infos
}

func (processoTool *ProcessoTool) GetActions() []tools.ToolAction {
	return processoToolActions
}

func (processoTool *ProcessoTool) GetSpec() tools.Tooling {
	return processoTool
}

func (processoTool *ProcessoTool) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	if err := json.Unmarshal(*rj, &processoTool); err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (processoTool *ProcessoTool) BytesToTool(ctx context.Context, toolObjJson []byte) (tools.Tooling, error) {
	newTool := &ProcessoTool{}
	if err := json.Unmarshal(toolObjJson, newTool); err != nil {
		return nil, logs.Err(ctx, err, "")
	}
	return newTool, nil
}

func (processoTool *ProcessoTool) GetBytes(ctx context.Context) ([]byte, error) {
	b, err := json.Marshal(processoTool)
	if err != nil {
		return nil, logs.Err(ctx, err, "")
	}
	return b, nil
}

func (processoTool *ProcessoTool) SetToolAction(actionName string) {
	for _, action := range processoToolActions {
		if action.ActionName == actionName {
			processoTool.ToolAction = action
			return
		}
	}
	processoTool.ToolAction = tools.ToolAction{}
}

func (processoTool *ProcessoTool) GetAttribute(ctx context.Context, attributeName string) (interface{}, error) {
	switch attributeName {
	case "tool_name":
		return processoTool.ToolName, nil
	case "tool_type":
		return processoTool.ToolType, nil
	case "system_prompt":
		return processoTool.SystemPrompt, nil
	case "output_schema":
		return processoTool.OutputSchema, nil
	case "parameters":
		return processoTool.Parameters, nil
	case "description":
		return processoTool.Description, nil
	case "project_id":
		return processoTool.ProjectId, nil
	default:
		err := errors.New("attribute not found")
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
}

func (processoTool *ProcessoTool) SetAttribute(ctx context.Context, attributeName string, attributeValue interface{}) error {
	switch attributeName {
	case "tool_name":
		processoTool.ToolName = attributeValue.(string)
	case "tool_type":
		processoTool.ToolType = attributeValue.(string)
	case "system_prompt":
		processoTool.SystemPrompt = attributeValue.(string)
	case "output_schema":
		processoTool.OutputSchema = attributeValue.(eru_models.JSONSchema)
	case "parameters":
		processoTool.Parameters = attributeValue.(eru_models.JSONSchema)
	case "description":
		processoTool.Description = attributeValue.(string)
	case "project_id":
		processoTool.ProjectId = attributeValue.(string)
	default:
		err := errors.New("attribute not found")
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (processoTool *ProcessoTool) getEruFuncBaseUrl(ctx context.Context) (string, error) {
	efurl := ctx.Value(tools.EruFuncBaseUrlKey)
	if efurl == nil {
		return "", errors.New("erufuncbaseurl not found in context")
	}
	efurlString, ok := efurl.(string)
	if !ok {
		return "", errors.New("erufuncbaseurl is not a string")
	}
	if efurlString == "" {
		return "", errors.New("erufuncbaseurl is not set")
	}
	return efurlString, nil
}

func (processoTool *ProcessoTool) getEruqlBaseUrl(ctx context.Context) (string, error) {
	v := ctx.Value("eruqlbaseurl")
	if v == nil {
		return "", errors.New("eruqlbaseurl not found in context")
	}
	s, ok := v.(string)
	if !ok {
		return "", errors.New("eruqlbaseurl is not a string")
	}
	if s == "" {
		return "", errors.New("eruqlbaseurl is not set")
	}
	return s, nil
}

func (processoTool *ProcessoTool) buildHeaders(ctx context.Context) http.Header {
	headers := http.Header{}
	claims := ctx.Value("claims")
	if claims != nil {
		headers.Add("claims", fmt.Sprint(claims))
	}
	headers.Add("Content-Type", "application/json")
	headers.Add("Accept", "application/json")
	return headers
}

func (processoTool *ProcessoTool) projectIdSegment() string {
	if processoTool.ProjectId == "" {
		return "processo"
	}
	return processoTool.ProjectId
}

func (processoTool *ProcessoTool) unmarshalParams(ctx context.Context, params map[string]interface{}, target interface{}) error {
	b, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("error marshalling params: %w", err)
	}
	if err := json.Unmarshal(b, target); err != nil {
		return logs.Err(ctx, err, "")
	}
	return nil
}

func (processoTool *ProcessoTool) Execute(ctx context.Context, projectId string, tenantId string, actionName string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("processoTool Execute - Start")
	var toolRequest interface{}
	switch actionName {
	case SaveApi:
		toolResult, toolRequest, persistStore, err = processoTool.SaveApi(ctx, projectId, tenantId, params)
	case ExecuteApi:
		toolResult, toolRequest, persistStore, err = processoTool.ExecuteApi(ctx, projectId, tenantId, params)
	case GetApi:
		toolResult, toolRequest, persistStore, err = processoTool.GetApi(ctx, projectId, tenantId, params)
	case ProcessoExecQuery:
		toolResult, toolRequest, persistStore, err = processoTool.ExecuteQuery(ctx, projectId, tenantId, params)
	case ProcessoSaveQueryGr:
		toolResult, toolRequest, persistStore, err = processoTool.SaveQuery(ctx, projectId, tenantId, params)
	case ProcessoGetQuery:
		toolResult, toolRequest, persistStore, err = processoTool.GetQuery(ctx, projectId, tenantId, params)
	case ProcessoSaveEntity:
		toolResult, toolRequest, persistStore, err = processoTool.SaveEntity(ctx, projectId, tenantId, params)
	case ProcessoSaveField:
		toolResult, toolRequest, persistStore, err = processoTool.SaveField(ctx, projectId, tenantId, params)
	case ProcessoSaveEntityData:
		toolResult, toolRequest, persistStore, err = processoTool.SaveEntityData(ctx, projectId, tenantId, params)
	case ProcessoDeleteEntityData:
		toolResult, toolRequest, persistStore, err = processoTool.DeleteEntityData(ctx, projectId, tenantId, params)
	case ProcessoSaveEntityVisibility:
		toolResult, toolRequest, persistStore, err = processoTool.SaveEntityVisibility(ctx, projectId, tenantId, params)
	case ProcessoSaveEntityRecordVisibility:
		toolResult, toolRequest, persistStore, err = processoTool.SaveEntityRecordVisibility(ctx, projectId, tenantId, params)
	case ProcessoSaveEntityDownloadVisibility:
		toolResult, toolRequest, persistStore, err = processoTool.SaveEntityDownloadVisibility(ctx, projectId, tenantId, params)
	default:
		return nil, false, fmt.Errorf("action %s not found", actionName)
	}

	gm := server.GetGlobalGoroutineManager(ctx)
	gm.SafeGoWithRestartBehavior("tool-post-execute-hook", func(bgCtx context.Context) {
		claims := ctx.Value("claims")
		if claims != nil {
			bgCtx = context.WithValue(bgCtx, "claims", claims)
		}
		efurl := ctx.Value(tools.EruFuncBaseUrlKey)
		if efurl == nil {
			logs.WithContext(ctx).Error("erufuncbaseurl not found in context")
			return
		}
		efurlString, ok := efurl.(string)
		if !ok {
			logs.WithContext(ctx).Error("erufuncbaseurl is not a string")
			return
		}
		bgCtx = context.WithValue(bgCtx, tools.EruFuncBaseUrlKey, efurlString)

		body := make(map[string]interface{})
		if toolRequest != nil {
			body["request"] = toolRequest
		}
		if toolResult != nil {
			body["response"] = toolResult
		}
		body["tenant_id"] = tenantId
		body["project_id"] = projectId

		if params["metadata"] != nil {
			body["metadata"] = params["metadata"]
		}

		hookResult, hookErr := processoTool.ExecuteHook(bgCtx, "poex", actionName, projectId, tenantId, body, nil)
		if hookErr != nil {
			logs.WithContext(bgCtx).Error(hookErr.Error())
			return
		}
		logs.WithContext(bgCtx).Info(fmt.Sprint(hookResult))
	}, server.ContinueOnMaxRetries)

	return toolResult, persistStore, err
}

func (processoTool *ProcessoTool) SaveApi(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("processoTool SaveApi - Start")
	p := ProcessoSaveApiParams{}
	if err = processoTool.unmarshalParams(ctx, params, &p); err != nil {
		return nil, nil, false, err
	}
	baseUrl, err := processoTool.getEruFuncBaseUrl(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	url := fmt.Sprint(baseUrl, "/", processoTool.projectIdSegment(), "/func/save_apis")
	body := map[string]interface{}{
		"api_name":     p.ApiName,
		"api_category": p.ApiCategory,
		"org_id":       p.OrgId,
		"process_id":   p.ProcessId,
		"api_def":      p.ApiDef,
	}
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, processoTool.buildHeaders(ctx), map[string]string{}, []*http.Cookie{}, map[string]string{}, body)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	toolResult = map[string]interface{}{"result": res}
	return toolResult, body, true, nil
}

func (processoTool *ProcessoTool) ExecuteApi(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("processoTool ExecuteApi - Start")
	p := ProcessoExecuteApiParams{}
	if err = processoTool.unmarshalParams(ctx, params, &p); err != nil {
		return nil, nil, false, err
	}
	baseUrl, err := processoTool.getEruFuncBaseUrl(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	url := fmt.Sprint(baseUrl, "/", processoTool.projectIdSegment(), "/func/exec_api")
	body := map[string]interface{}{
		"api_name":   p.ApiName,
		"org_id":     p.OrgId,
		"process_id": p.ProcessId,
	}
	for k, v := range p.Body {
		if _, exists := body[k]; exists {
			continue
		}
		body[k] = v
	}
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, processoTool.buildHeaders(ctx), map[string]string{}, []*http.Cookie{}, map[string]string{}, body)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	toolResult = map[string]interface{}{"result": res}
	return toolResult, body, true, nil
}

func (processoTool *ProcessoTool) GetApi(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("processoTool GetApi - Start")
	p := ProcessoGetApiParams{}
	if err = processoTool.unmarshalParams(ctx, params, &p); err != nil {
		return nil, nil, false, err
	}
	if p.ApiId == "" && p.ApiName == "" {
		return nil, nil, false, errors.New("either api_id or api_name must be provided")
	}
	baseUrl, err := processoTool.getEruqlBaseUrl(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	url := fmt.Sprint(baseUrl, "/store/", processoTool.projectIdSegment(), "/myquery/execute/fetch_api")
	body := map[string]interface{}{
		"org_id":     p.OrgId,
		"process_id": p.ProcessId,
	}
	if p.ApiId != "" {
		body["api_id"] = p.ApiId
	}
	if p.ApiName != "" {
		body["api_name"] = p.ApiName
	}
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, processoTool.buildHeaders(ctx), map[string]string{}, []*http.Cookie{}, map[string]string{}, body)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	toolResult = map[string]interface{}{"result": res}
	return toolResult, body, true, nil
}

func (processoTool *ProcessoTool) ExecuteQuery(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("processoTool ExecuteQuery - Start")
	p := ProcessoExecuteQueryParams{}
	if err = processoTool.unmarshalParams(ctx, params, &p); err != nil {
		return nil, nil, false, err
	}
	baseUrl, err := processoTool.getEruFuncBaseUrl(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	url := fmt.Sprint(baseUrl, "/", processoTool.projectIdSegment(), "/func/exec_queries")
	body := map[string]interface{}{
		"org_id":     p.OrgId,
		"process_id": p.ProcessId,
		"query_name": p.QueryName,
	}
	for k, v := range p.Body {
		if _, exists := body[k]; exists {
			continue
		}
		body[k] = v
	}
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, processoTool.buildHeaders(ctx), map[string]string{}, []*http.Cookie{}, map[string]string{}, body)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	toolResult = map[string]interface{}{"result": res}
	return toolResult, body, true, nil
}

func (processoTool *ProcessoTool) SaveQuery(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("processoTool SaveQuery - Start")
	p := ProcessoSaveQueryParams{}
	if err = processoTool.unmarshalParams(ctx, params, &p); err != nil {
		return nil, nil, false, err
	}
	baseUrl, err := processoTool.getEruFuncBaseUrl(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	url := fmt.Sprint(baseUrl, "/", processoTool.projectIdSegment(), "/func/save_queries_grp")
	body := map[string]interface{}{
		"org_id":       p.OrgId,
		"process_id":   p.ProcessId,
		"query_name":   p.QueryName,
		"query_string": p.QueryString,
		"db_alias":     p.DbAlias,
		"query_vars":   p.QueryVars,
		"query_type":   p.QueryType,
	}
	if p.QueryId != "" {
		body["query_id"] = p.QueryId
	}
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, processoTool.buildHeaders(ctx), map[string]string{}, []*http.Cookie{}, map[string]string{}, body)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	toolResult = map[string]interface{}{"result": res}
	return toolResult, body, true, nil
}

func (processoTool *ProcessoTool) GetQuery(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("processoTool GetQuery - Start")
	p := ProcessoGetQueryParams{}
	if err = processoTool.unmarshalParams(ctx, params, &p); err != nil {
		return nil, nil, false, err
	}
	if p.QueryId == "" && p.QueryName == "" {
		return nil, nil, false, errors.New("either query_id or query_name must be provided")
	}
	baseUrl, err := processoTool.getEruFuncBaseUrl(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	url := fmt.Sprint(baseUrl, "/", processoTool.projectIdSegment(), "/func/get_queries_def")
	body := map[string]interface{}{
		"org_id":     p.OrgId,
		"process_id": p.ProcessId,
	}
	if p.QueryId != "" {
		body["query_id"] = p.QueryId
	}
	if p.QueryName != "" {
		body["query_name"] = p.QueryName
	}
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, processoTool.buildHeaders(ctx), map[string]string{}, []*http.Cookie{}, map[string]string{}, body)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	toolResult = map[string]interface{}{"result": res}
	return toolResult, body, true, nil
}

func (processoTool *ProcessoTool) SaveEntity(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("processoTool SaveEntity - Start")
	p := ProcessoSaveEntityParams{}
	if err = processoTool.unmarshalParams(ctx, params, &p); err != nil {
		return nil, nil, false, err
	}
	if len(p.EntityData) == 0 {
		return nil, nil, false, errors.New("entity_data must have at least one entity")
	}
	baseUrl, err := processoTool.getEruFuncBaseUrl(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	url := fmt.Sprint(baseUrl, "/", processoTool.projectIdSegment(), "/func/save_process_entity_index")
	body := map[string]interface{}{
		"org_id":       p.OrgId,
		"process_id":   p.ProcessId,
		"process_name": p.ProcessName,
		"entity_data":  p.EntityData,
	}
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, processoTool.buildHeaders(ctx), map[string]string{}, []*http.Cookie{}, map[string]string{}, body)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	toolResult = map[string]interface{}{"result": res}
	return toolResult, body, true, nil
}

const (
	ProcessoDatatypeTextbox    = "textbox"
	ProcessoDatatypeDate       = "date"
	ProcessoDatatypeEmail      = "email"
	ProcessoDatatypeDropdownSs = "dropdown_single_select"
	ProcessoDatatypeDropdownMs = "dropdown_multi_select"
	ProcessoDatatypeStatus     = "status"
)

var processoAllowedDays = []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}

func processoIsDropdown(datatype string) bool {
	return datatype == ProcessoDatatypeDropdownSs || datatype == ProcessoDatatypeDropdownMs
}

func processoValidateStatusList(listName string, statusList []ProcessoFieldStatus) error {
	if len(statusList) == 0 {
		return fmt.Errorf("%s must have at least one status for datatype %s", listName, ProcessoDatatypeStatus)
	}
	defaultCount := 0
	for i, st := range statusList {
		if st.StatusName == "" {
			return fmt.Errorf("%s[%d].name is mandatory", listName, i)
		}
		if st.StatusColor == "" {
			return fmt.Errorf("%s[%d].color is mandatory", listName, i)
		}
		if st.StatusColor[0] != '#' {
			return fmt.Errorf("%s[%d].color must be a hex code starting with #", listName, i)
		}
		if st.StatusDf != nil && *st.StatusDf {
			defaultCount = defaultCount + 1
		}
	}
	if defaultCount > 1 {
		return fmt.Errorf("%s can have only one status with df as true", listName)
	}
	return nil
}

func (processoTool *ProcessoTool) buildFieldBody(f ProcessoFieldDef) (map[string]interface{}, error) {
	if f.Name == "" {
		return nil, errors.New("field.name is mandatory")
	}
	if f.Label == "" {
		return nil, errors.New("field.label is mandatory")
	}
	if f.Datatype == "" {
		return nil, errors.New("field.datatype is mandatory")
	}
	if f.TabName == "" {
		return nil, errors.New("field.tab_name is mandatory")
	}
	showGrid := f.ShowGrid
	if showGrid == "" {
		showGrid = "yes"
	}
	if showGrid != "yes" && showGrid != "no" {
		return nil, errors.New("field.show_grid must be either yes or no")
	}
	tfIdx := f.TfIdx
	if tfIdx == 0 {
		tfIdx = f.GridIndex
	}
	fieldBody := map[string]interface{}{
		"can_edit":      f.CanEdit,
		"show_mobile":   f.ShowMobile,
		"name":          f.Name,
		"label":         f.Label,
		"description":   f.Description,
		"default":       f.Default,
		"datatype":      f.Datatype,
		"tab_name":      f.TabName,
		"can_group":     f.CanGroup,
		"mandatory":     f.Mandatory,
		"is_hidden":     f.IsHidden,
		"editable":      f.Editable,
		"is_unique":     f.IsUnique,
		"unq_sa":        f.UnqSa,
		"show_grid":     showGrid,
		"grid_index":    f.GridIndex,
		"default_group": f.DefaultGroup,
		"tool_tip":      f.ToolTip,
		"is_pii":        f.IsPii,
		"to_encrypt":    f.ToEncrypt,
		"is_ephi":       f.IsEphi,
		"is_pf":         f.IsPf,
		"dfc":           f.Dfc,
		"ce":            f.Ce,
		"cef":           f.Cef,
		"cefa":          f.Cefa,
		"tf_idx":        tfIdx,
	}

	uniqueFn := f.UniqueFn
	if uniqueFn == nil {
		uniqueFn = []string{}
	}

	switch f.Datatype {
	case ProcessoDatatypeTextbox:
		if f.DataLength != "" {
			dataLength, atoiErr := strconv.Atoi(f.DataLength)
			if atoiErr != nil || dataLength <= 0 {
				return nil, errors.New("field.data_length must be a positive number")
			}
			if f.DataLengthCheck == "" {
				return nil, errors.New("field.data_length_check is mandatory when data_length is provided")
			}
		}
		fieldBody["unique_fn"] = uniqueFn
		fieldBody["data_length"] = f.DataLength
		fieldBody["data_length_check"] = f.DataLengthCheck
	case ProcessoDatatypeDate:
		if f.DateFormat == "" {
			return nil, fmt.Errorf("field.date_format is mandatory for datatype %s", ProcessoDatatypeDate)
		}
		allowDays := f.AllowDays
		if len(allowDays) == 0 {
			allowDays = processoAllowedDays
		}
		for _, day := range allowDays {
			isAllowed := false
			for _, allowedDay := range processoAllowedDays {
				if day == allowedDay {
					isAllowed = true
					break
				}
			}
			if !isAllowed {
				return nil, fmt.Errorf("field.allow_days has an invalid day %s - allowed values are %v", day, processoAllowedDays)
			}
		}
		fieldBody["date_format"] = f.DateFormat
		fieldBody["allow_days"] = allowDays
	case ProcessoDatatypeEmail:
		systemValidate := f.SystemValidate
		if systemValidate == "" {
			systemValidate = "false"
		}
		if systemValidate != "true" && systemValidate != "false" {
			return nil, errors.New("field.system_validate must be either true or false")
		}
		if systemValidate == "true" && f.SsvApi == "" {
			return nil, errors.New("field.ssv_api is mandatory when system_validate is true")
		}
		fieldBody["unique_fn"] = uniqueFn
		fieldBody["system_validate"] = systemValidate
		fieldBody["ssv_api"] = f.SsvApi
	case ProcessoDatatypeDropdownSs, ProcessoDatatypeDropdownMs:
		options := f.Options
		if options == nil {
			options = []ProcessoFieldOption{}
		}
		switch f.FieldOptionType {
		case "STATIC":
			if len(options) == 0 {
				return nil, errors.New("field.options must have at least one option when option_type is STATIC")
			}
			for i, option := range options {
				if option.OptionName == "" {
					return nil, fmt.Errorf("field.options[%d].name is mandatory", i)
				}
			}
		case "ENTITY_DATA":
			if f.OptionEntityName == "" || f.OptionFieldName == "" {
				return nil, errors.New("field.entity_name and field.field_name are mandatory when option_type is ENTITY_DATA")
			}
		case "API":
			if f.ApiName == "" || f.ApiField == "" {
				return nil, errors.New("field.api_name and field.api_field are mandatory when option_type is API")
			}
		case "":
			return nil, fmt.Errorf("field.option_type is mandatory for datatype %s", f.Datatype)
		default:
			return nil, errors.New("field.option_type must be one of STATIC, ENTITY_DATA or API")
		}
		fieldBody["option_type"] = f.FieldOptionType
		fieldBody["api_name"] = f.ApiName
		fieldBody["api_field"] = f.ApiField
		fieldBody["entity_name"] = f.OptionEntityName
		fieldBody["field_name"] = f.OptionFieldName
		fieldBody["options"] = options
	case ProcessoDatatypeStatus:
		if err := processoValidateStatusList("field.open_status", f.OpenStatus); err != nil {
			return nil, err
		}
		if err := processoValidateStatusList("field.close_status", f.CloseStatus); err != nil {
			return nil, err
		}
		fieldBody["open_status"] = f.OpenStatus
		fieldBody["close_status"] = f.CloseStatus
		fieldBody["_sts"] = f.Sts
	}
	return fieldBody, nil
}

func (processoTool *ProcessoTool) SaveField(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("processoTool SaveField - Start")
	p := ProcessoSaveFieldParams{}
	if err = processoTool.unmarshalParams(ctx, params, &p); err != nil {
		return nil, nil, false, err
	}
	if p.EntityName == "" {
		return nil, nil, false, errors.New("entity_name is mandatory")
	}
	fieldBody, err := processoTool.buildFieldBody(p.Field)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	baseUrl, err := processoTool.getEruFuncBaseUrl(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	childField := p.ChildField
	if childField == "" {
		childField = p.Field.Name
	}
	optionType := p.OptionType
	if optionType == "" {
		optionType = p.Field.FieldOptionType
	}
	parentEntity := p.ParentEntity
	if parentEntity == "" {
		parentEntity = p.Field.OptionEntityName
	}
	url := fmt.Sprint(baseUrl, "/", processoTool.projectIdSegment(), "/func/save_field")
	body := map[string]interface{}{
		"org_id":        p.OrgId,
		"process_id":    p.ProcessId,
		"entity_name":   p.EntityName,
		"field":         fieldBody,
		"process_name":  p.ProcessName,
		"parent_entity": parentEntity,
		"child_field":   childField,
		"option_type":   optionType,
	}
	if processoIsDropdown(p.Field.Datatype) {
		parentField := p.ParentField
		if parentField == "" {
			parentField = p.Field.OptionFieldName
		}
		body["parent_field"] = parentField
	}
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, processoTool.buildHeaders(ctx), map[string]string{}, []*http.Cookie{}, map[string]string{}, body)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	toolResult = map[string]interface{}{"result": res}
	return toolResult, body, true, nil
}

func processoSliceOrEmpty(list []interface{}) []interface{} {
	if list == nil {
		return []interface{}{}
	}
	return list
}

func processoMapOrEmpty(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return map[string]interface{}{}
	}
	return m
}

func (processoTool *ProcessoTool) SaveEntityData(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("processoTool SaveEntityData - Start")
	p := ProcessoSaveEntityDataParams{}
	if err = processoTool.unmarshalParams(ctx, params, &p); err != nil {
		return nil, nil, false, err
	}
	if p.EntityName == "" {
		return nil, nil, false, errors.New("entity_name is mandatory")
	}
	if len(p.EntityData) == 0 {
		return nil, nil, false, errors.New("entity_data must have at least one field value")
	}
	if p.IsEdit {
		if p.EntityId == "" {
			return nil, nil, false, errors.New("entity_id is mandatory when is_edit is true")
		}
		if len(p.OldEntityData) == 0 {
			return nil, nil, false, errors.New("old_entity_data is mandatory when is_edit is true")
		}
	}
	baseUrl, err := processoTool.getEruFuncBaseUrl(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	url := fmt.Sprint(baseUrl, "/", processoTool.projectIdSegment(), "/func/save_entity_data")
	body := map[string]interface{}{
		"has_am":          p.HasAm,
		"org_id":          p.OrgId,
		"process_id":      p.ProcessId,
		"entity_name":     p.EntityName,
		"process_name":    p.ProcessName,
		"entity_data":     p.EntityData,
		"old_entity_data": processoMapOrEmpty(p.OldEntityData),
		"emd":             processoMapOrEmpty(p.Emd),
		"people_users":    processoSliceOrEmpty(p.PeopleUsers),
		"is_edit":         p.IsEdit,
		"p_entities":      processoSliceOrEmpty(p.PEntities),
		"entity_id":       p.EntityId,
		"p_created_by":    processoSliceOrEmpty(p.PCreatedBy),
		"dfc_p_entities":  processoSliceOrEmpty(p.DfcPEntities),
		"has_wf_am":       p.HasWfAm,
		"has_wf_oth":      p.HasWfOth,
	}
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, processoTool.buildHeaders(ctx), map[string]string{}, []*http.Cookie{}, map[string]string{}, body)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	toolResult = map[string]interface{}{"result": res}
	return toolResult, body, true, nil
}

func (processoTool *ProcessoTool) DeleteEntityData(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("processoTool DeleteEntityData - Start")
	p := ProcessoDeleteEntityDataParams{}
	if err = processoTool.unmarshalParams(ctx, params, &p); err != nil {
		return nil, nil, false, err
	}
	if p.EntityName == "" {
		return nil, nil, false, errors.New("entity_name is mandatory")
	}
	if len(p.EntityId) == 0 {
		return nil, nil, false, errors.New("entity_id must have at least one record id")
	}
	for i, entityId := range p.EntityId {
		if entityId == "" {
			return nil, nil, false, fmt.Errorf("entity_id[%d] cannot be blank", i)
		}
	}
	baseUrl, err := processoTool.getEruFuncBaseUrl(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	url := fmt.Sprint(baseUrl, "/", processoTool.projectIdSegment(), "/func/delete_entity_data")
	body := map[string]interface{}{
		"child_entities": processoSliceOrEmpty(p.ChildEntities),
		"entity_name":    p.EntityName,
		"process_name":   p.ProcessName,
		"org_id":         p.OrgId,
		"process_id":     p.ProcessId,
		"entity_id":      p.EntityId,
	}
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, processoTool.buildHeaders(ctx), map[string]string{}, []*http.Cookie{}, map[string]string{}, body)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	toolResult = map[string]interface{}{"result": res}
	return toolResult, body, true, nil
}

func processoStringSliceOrEmpty(list []string) []string {
	if list == nil {
		return []string{}
	}
	return list
}

func processoValidateSubset(subsetName string, subset []string, supersetName string, superset []string) error {
	for _, value := range subset {
		isPresent := false
		for _, supersetValue := range superset {
			if value == supersetValue {
				isPresent = true
				break
			}
		}
		if !isPresent {
			return fmt.Errorf("%s has %s which is not present in %s", subsetName, value, supersetName)
		}
	}
	return nil
}

func (processoTool *ProcessoTool) SaveEntityVisibility(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("processoTool SaveEntityVisibility - Start")
	p := ProcessoSaveEntityVisibilityParams{}
	if err = processoTool.unmarshalParams(ctx, params, &p); err != nil {
		return nil, nil, false, err
	}
	if p.EntityName == "" {
		return nil, nil, false, errors.New("entity_name is mandatory")
	}
	if p.VisibilityType == "" {
		return nil, nil, false, errors.New("visibility_type is mandatory")
	}
	mapUsers := processoStringSliceOrEmpty(p.MapUsers)
	mapRoles := processoStringSliceOrEmpty(p.MapRoles)
	envMapUsers := processoStringSliceOrEmpty(p.EnvMapUsers)
	envMapRoles := processoStringSliceOrEmpty(p.EnvMapRoles)
	if p.VisibilityType != "public" && len(mapUsers) == 0 && len(mapRoles) == 0 {
		return nil, nil, false, fmt.Errorf("map_users or map_roles is mandatory when visibility_type is %s", p.VisibilityType)
	}
	if err = processoValidateSubset("env_map_users", envMapUsers, "map_users", mapUsers); err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	if err = processoValidateSubset("env_map_roles", envMapRoles, "map_roles", mapRoles); err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	baseUrl, err := processoTool.getEruqlBaseUrl(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	url := fmt.Sprint(baseUrl, "/store/", processoTool.projectIdSegment(), "/myquery/execute/save_process_entity_visibility")
	body := map[string]interface{}{
		"org_id":          p.OrgId,
		"process_id":      p.ProcessId,
		"entity_name":     p.EntityName,
		"process_name":    p.ProcessName,
		"map_users":       mapUsers,
		"map_roles":       mapRoles,
		"env_map_users":   envMapUsers,
		"env_map_roles":   envMapRoles,
		"visibility_type": p.VisibilityType,
	}
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, processoTool.buildHeaders(ctx), map[string]string{}, []*http.Cookie{}, map[string]string{}, body)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	toolResult = map[string]interface{}{"result": res}
	return toolResult, body, true, nil
}

func (processoTool *ProcessoTool) SaveEntityRecordVisibility(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("processoTool SaveEntityRecordVisibility - Start")
	p := ProcessoSaveEntityRecordVisibilityParams{}
	if err = processoTool.unmarshalParams(ctx, params, &p); err != nil {
		return nil, nil, false, err
	}
	if p.EntityName == "" {
		return nil, nil, false, errors.New("entity_name is mandatory")
	}
	if p.VisibilityType == "" {
		return nil, nil, false, errors.New("visibility_type is mandatory")
	}
	mapUsers := processoStringSliceOrEmpty(p.MapUsers)
	mapRoles := processoStringSliceOrEmpty(p.MapRoles)
	userAttFilter := processoMapOrEmpty(p.UserAttFilter)
	parentEntities := processoSliceOrEmpty(p.ParentEntities)
	parentAttFilter := processoMapOrEmpty(p.ParentAttFilter)
	if p.VisibilityType != "public" && len(mapUsers) == 0 && len(mapRoles) == 0 {
		return nil, nil, false, fmt.Errorf("map_users or map_roles is mandatory when visibility_type is %s", p.VisibilityType)
	}
	if p.Cond != "" && len(userAttFilter) == 0 {
		return nil, nil, false, errors.New("user_att_filter is mandatory when cond is provided")
	}
	if (p.PjCond != "" || len(parentAttFilter) > 0) && len(parentEntities) == 0 {
		return nil, nil, false, errors.New("parent_entities is mandatory when parent_att_filter or pj_cond is provided")
	}
	baseUrl, err := processoTool.getEruFuncBaseUrl(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	url := fmt.Sprint(baseUrl, "/", processoTool.projectIdSegment(), "/func/save_process_entity_record_visibility")
	body := map[string]interface{}{
		"org_id":            p.OrgId,
		"process_id":        p.ProcessId,
		"entity_name":       p.EntityName,
		"process_name":      p.ProcessName,
		"visibility_type":   p.VisibilityType,
		"map_roles":         mapRoles,
		"map_users":         mapUsers,
		"user_op_fn":        processoSliceOrEmpty(p.UserOpFn),
		"user_att_filter":   userAttFilter,
		"cond":              p.Cond,
		"parent_entities":   parentEntities,
		"parent_att_filter": parentAttFilter,
		"pj_cond":           p.PjCond,
	}
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, processoTool.buildHeaders(ctx), map[string]string{}, []*http.Cookie{}, map[string]string{}, body)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	toolResult = map[string]interface{}{"result": res}
	return toolResult, body, true, nil
}

func (processoTool *ProcessoTool) SaveEntityDownloadVisibility(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("processoTool SaveEntityDownloadVisibility - Start")
	p := ProcessoSaveEntityDownloadVisibilityParams{}
	if err = processoTool.unmarshalParams(ctx, params, &p); err != nil {
		return nil, nil, false, err
	}
	if p.EntityName == "" {
		return nil, nil, false, errors.New("entity_name is mandatory")
	}
	if p.VisibilityType == "" {
		return nil, nil, false, errors.New("visibility_type is mandatory")
	}
	mapUsers := processoStringSliceOrEmpty(p.MapUsers)
	mapRoles := processoStringSliceOrEmpty(p.MapRoles)
	if p.VisibilityType != "public" && len(mapUsers) == 0 && len(mapRoles) == 0 {
		return nil, nil, false, fmt.Errorf("map_users or map_roles is mandatory when visibility_type is %s", p.VisibilityType)
	}
	baseUrl, err := processoTool.getEruqlBaseUrl(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	url := fmt.Sprint(baseUrl, "/store/", processoTool.projectIdSegment(), "/myquery/execute/save_download_visibility")
	body := map[string]interface{}{
		"org_id":          p.OrgId,
		"process_id":      p.ProcessId,
		"entity_name":     p.EntityName,
		"process_name":    p.ProcessName,
		"map_users":       mapUsers,
		"map_roles":       mapRoles,
		"visibility_type": p.VisibilityType,
	}
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, processoTool.buildHeaders(ctx), map[string]string{}, []*http.Cookie{}, map[string]string{}, body)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	toolResult = map[string]interface{}{"result": res}
	return toolResult, body, true, nil
}

func init() {
	tools.RegisterToolCatalog(tools.ToolCatalogEntry{
		Public:      false,
		ToolType:    "PROCESSO",
		Category:    "Data",
		Description: "Processo tool to save and execute apis (function groups) for an org and process via eru-functions service",
		Actions: func() []tools.ActionInfo {
			infos := make([]tools.ActionInfo, len(processoToolActions))
			for i, a := range processoToolActions {
				infos[i] = tools.ActionInfo{Name: a.ActionName, Description: a.Description}
			}
			return infos
		}(),
		OAuthEnabled: false,
		Icon:         "",
		IconType:     "svg",
		ToolSchema:   utils.StructToJSONSchema(reflect.TypeOf(ProcessoTool{}), []string{}),
	})
}
