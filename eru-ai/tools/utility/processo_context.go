package utiltiy

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	tools "github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eru_models "github.com/eru-tech/eru/eru-models"
	utils "github.com/eru-tech/eru/eru-utils"
)

// Every processo write carries org_id, process_id and process_name in its body -
// the processo tool's own actions put no tenant segment in the url at all, so
// those ids are the only thing that says which workspace is being changed.
//
// A model asked to supply them will supply something. It has seen ids in the
// conversation, in a page it was handed, in an error message; any of them will
// look plausible and one of them will be another customer's. So the ids are not
// the model's to choose: this tool resolves them from the tenant the request is
// already running as, and the prompt tells the model they come from here and
// from nowhere else.

const ProcessoContextToolName = "get_processo_context"

// ProcessoContextQuery is the stored eru-ql query that maps an org process id to
// the ids that identify it. It is the same query the eru-ql tool uses to fill
// mandatory variables, so the two cannot disagree about what a tenant means.
const ProcessoContextQuery = "get_org_processes"

// The query is graphql, so a column arrives either under its alias or under the
// generated `<schema>___<table>__<column>` name, depending on whether whoever
// wrote the query aliased it. Both spellings are accepted rather than requiring
// the stored query to be edited.
var (
	orgIdCandidates       = []string{"org_id", "public___orgs__org_id"}
	processIdCandidates   = []string{"process_id", "public___org_processes__process_id"}
	processNameCandidates = []string{"process_name", "public___org_processes__process_name"}
	orgNameCandidates     = []string{"org_name", "public___orgs__org_name"}
	ownerIdCandidates     = []string{"owner_id", "public___orgs__owner_id"}
)

var processoContextActions = []tools.ToolAction{
	{
		ActionName:   ProcessoContextToolName,
		Description:  "Resolve the ids of the workspace this request is running against",
		SystemPrompt: "Resolve the ids of the workspace this request is running against",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
	},
}

// ProcessoContextToolSchema takes no parameters on purpose. There is exactly one
// correct answer for a given request and the model has no part in choosing it.
func ProcessoContextToolSchema() eru_models.JSONSchema {
	return eru_models.JSONSchema{Type: "object", Properties: map[string]eru_models.JSONSchema{}}
}

func ProcessoContextToolDescription() string {
	return "Return the ids of the processo workspace this request is running against: `org_id`, `process_id`, " +
		"`process_name` and `org_process_id`. Almost every processo write needs `org_id`, `process_id` and " +
		"`process_name` in its body, and they are NOT interchangeable: `process_id` identifies the process " +
		"definition, `org_process_id` identifies this organisation's instance of it, and `process_name` is the " +
		"slug. Call this once before your first write and reuse the answer for the rest of the turn. Never take " +
		"these ids from the conversation, from a page you were handed, or from an error message, and never guess " +
		"one - a wrong id writes to somebody else's workspace."
}

// ProcessoContextTool answers "which workspace am I in" by running one stored
// eru-ql query against the tenant of the current request.
type ProcessoContextTool struct {
	tools.Tool
	// Delegate is the configured eru-ql-capable tool that runs the query. It is
	// wired per request by the agent offering this tool, because the connection
	// details belong to that agent's configuration.
	Delegate tools.Tooling `json:"-"`
	// QueryName defaults to ProcessoContextQuery and is here so a tenant on a
	// differently-named query does not need a code change.
	QueryName string `json:"query_name,omitempty"`
}

func (pcTool *ProcessoContextTool) GetActionsList() []tools.ActionInfo {
	infos := make([]tools.ActionInfo, len(processoContextActions))
	for i, action := range processoContextActions {
		infos[i] = tools.ActionInfo{Name: action.ActionName, Description: action.Description}
	}
	return infos
}

func (pcTool *ProcessoContextTool) GetActions() []tools.ToolAction {
	return processoContextActions
}

func (pcTool *ProcessoContextTool) GetSpec() tools.Tooling {
	return pcTool
}

func (pcTool *ProcessoContextTool) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("ProcessoContextTool MakeFromJson - Start")
	if err := json.Unmarshal(*rj, &pcTool); err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (pcTool *ProcessoContextTool) BytesToTool(ctx context.Context, toolObjJson []byte) (tools.Tooling, error) {
	newTool := &ProcessoContextTool{}
	if err := json.Unmarshal(toolObjJson, newTool); err != nil {
		return nil, logs.Err(ctx, err, "")
	}
	return newTool, nil
}

func (pcTool *ProcessoContextTool) Execute(ctx context.Context, projectId string, tenantId string, actionName string, params map[string]interface{}) (map[string]interface{}, bool, error) {
	logs.WithContext(ctx).Debug("ProcessoContextTool Execute - Start")
	if pcTool.Delegate == nil {
		return nil, false, fmt.Errorf("%s is not available: no eru-ql tool is attached to this agent", ProcessoContextToolName)
	}
	if strings.TrimSpace(tenantId) == "" {
		return nil, false, fmt.Errorf("%s cannot resolve a workspace: this request carries no tenant", ProcessoContextToolName)
	}

	queryName := pcTool.QueryName
	if strings.TrimSpace(queryName) == "" {
		queryName = ProcessoContextQuery
	}

	// The tenant is the org process id, and it comes from the execution context.
	result, _, err := pcTool.Delegate.Execute(ctx, projectId, tenantId, "execute_query", map[string]interface{}{
		"query_name": queryName,
		"vars":       map[string]interface{}{"org_process_id": tenantId},
	})
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("%s failed: %v", ProcessoContextToolName, err))
		return nil, false, fmt.Errorf("the processo context query failed: %w", err)
	}

	rows := extractRows(result)
	if len(rows) == 0 {
		// Returning empty ids would be worse than failing: the model would write
		// with blank org_id and the write would land somewhere unintended or be
		// rejected with a message that says nothing about the real cause.
		return nil, false, fmt.Errorf(
			"no active org process matches this tenant (%s) - the workspace cannot be resolved, so do not attempt any write that needs org_id or process_id",
			tenantId)
	}
	row := rows[0]

	orgId := firstString(row, orgIdCandidates)
	processId := firstString(row, processIdCandidates)
	processName := firstString(row, processNameCandidates)
	if orgId == "" || processId == "" || processName == "" {
		logs.WithContext(ctx).Error(fmt.Sprintf("%s got a row with keys %v", ProcessoContextToolName, rowKeys(row)))
		return nil, false, fmt.Errorf(
			"the processo context query answered without org_id, process_id or process_name - the workspace cannot be resolved, so do not attempt any write that needs them")
	}

	out := map[string]interface{}{
		"org_process_id": tenantId,
		"org_id":         orgId,
		"process_id":     processId,
		"process_name":   processName,
	}
	if orgName := firstString(row, orgNameCandidates); orgName != "" {
		out["org_name"] = orgName
	}
	if ownerId := firstString(row, ownerIdCandidates); ownerId != "" {
		out["owner_id"] = ownerId
	}

	logs.WithContext(ctx).Info(fmt.Sprintf("%s resolved org %s process %s (%s)", ProcessoContextToolName, orgId, processId, processName))
	return out, false, nil
}

func init() {
	tools.RegisterToolCatalog(tools.ToolCatalogEntry{
		Public:       false,
		ToolType:     "ProcessoContext",
		Category:     "Utility",
		Description:  "The ids of the processo workspace a request is running against: org_id, process_id, process_name, org_process_id",
		Actions:      []tools.ActionInfo{{Name: ProcessoContextToolName}},
		OAuthEnabled: false,
		Icon:         "",
		IconType:     "svg",
		ToolSchema:   utils.StructToJSONSchema(reflect.TypeOf(ProcessoContextTool{}), []string{}),
	})
}
