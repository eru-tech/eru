package utiltiy

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	studio "github.com/eru-tech/eru/eru-ai/agents/eru_studio"
	tools "github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eru_models "github.com/eru-tech/eru/eru-models"
)

const RunQueryToolName = "run_query"

// MaxProbeRows is how many rows come back as a sample. The point of this tool is
// the SHAPE of the answer, not its contents, so a couple of rows is enough and a
// dashboard query over months of data does not flood the context.
const MaxProbeRows = 2

var runQueryToolActions = []tools.ToolAction{
	{
		ActionName:   RunQueryToolName,
		Description:  "Run a saved query and report the shape of its result",
		SystemPrompt: "Run a saved query and report the shape of its result",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
	},
}

func RunQueryToolSchema() eru_models.JSONSchema {
	return eru_models.JSONSchema{
		Type: "object",
		Properties: map[string]eru_models.JSONSchema{
			"query_name": {
				Type:        "string",
				Description: "Name of a SAVED query to run. Use this for a query the page will bind to.",
			},
			"sql": {
				Type:        "string",
				Description: "SQL to run directly, for a query that is not saved yet - to check what a statement returns before saving it. Give this or query_name, not both.",
			},
			"vars": {
				Type:        "object",
				Description: "Variables to run it with, as the page would send them. Give realistic values - a query filtered on a currency pair returns nothing for a pair that has no rows, and an empty result tells you nothing about its shape.",
			},
		},
	}
}

func RunQueryToolDescription() string {
	return "Run a query and see what it actually returns: the path to the row array, the column names, and " +
		"a sample row. Give `query_name` for a saved query, or `sql` to try a statement that is not saved yet. " +
		"Call it before you bind anything to a query. The response shape is NOT something to assume " +
		"- it differs between sql and graphql queries, between eru-ql and a function, and with whatever the query " +
		"itself selects. Take `result_path` for a `payload_path` / `query_result_path`, and take `columns` for a " +
		"tile's *_field, a chart's dimensions and measures, and a grid's fields. A binding written from a guessed " +
		"column name renders blank and reports nothing."
}

// RunQueryTool lets the page author run a query and read its real answer.
//
// Without it the shape of a query result is a guess: the agent has to invent
// both the path into the response and the column names, and a page bound to
// either guess renders empty while every call succeeds. Nothing in the page
// validates those two, because a JSON path that resolves to nothing is not an
// error - it is a blank tile.
type RunQueryTool struct {
	tools.Tool
	// Delegate is the configured eru-ql tool that runs the query, wired per
	// request by the agent that offers this tool.
	Delegate tools.Tooling `json:"-"`
}

func (rqTool *RunQueryTool) GetActionsList() []tools.ActionInfo {
	infos := make([]tools.ActionInfo, len(runQueryToolActions))
	for i, action := range runQueryToolActions {
		infos[i] = tools.ActionInfo{Name: action.ActionName, Description: action.Description}
	}
	return infos
}

func (rqTool *RunQueryTool) GetActions() []tools.ToolAction {
	return runQueryToolActions
}

func (rqTool *RunQueryTool) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("RunQueryTool MakeFromJson - Start")
	if err := json.Unmarshal(*rj, &rqTool); err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (rqTool *RunQueryTool) Execute(ctx context.Context, projectId string, tenantId string, actionName string, params map[string]interface{}) (map[string]interface{}, bool, error) {
	logs.WithContext(ctx).Debug("RunQueryTool Execute - Start")
	if rqTool.Delegate == nil {
		return nil, false, fmt.Errorf("%s is not available: no eru-ql tool is attached to this agent", RunQueryToolName)
	}

	queryName := strings.TrimSpace(asString(params["query_name"]))
	sql := strings.TrimSpace(asString(params["sql"]))
	switch {
	case queryName != "" && sql != "":
		return nil, false, fmt.Errorf("%s takes query_name for a saved query OR sql for an unsaved one, not both", RunQueryToolName)
	case queryName == "" && sql == "":
		return nil, false, fmt.Errorf("%s needs query_name (a saved query) or sql (a statement to try)", RunQueryToolName)
	}

	vars, _ := params["vars"].(map[string]interface{})
	if vars == nil {
		vars = map[string]interface{}{}
	}

	action := "execute_query"
	call := map[string]interface{}{"query_name": queryName, "vars": vars}
	subject := queryName
	if sql != "" {
		action = "execute_sql"
		call = map[string]interface{}{"query": sql, "vars": vars}
		subject = "the supplied sql"
	}

	result, _, err := rqTool.Delegate.Execute(ctx, projectId, tenantId, action, call)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("%s failed for %s: %v", RunQueryToolName, subject, err))
		studio.LedgerFrom(ctx).RecordFailure(RunQueryToolName, err.Error())
		// A failing query is an answer, not a dead end: it is usually the query
		// that needs fixing, and the agent can say so.
		out := map[string]interface{}{
			"query_name": queryName,
			"ran":        action,
			"ok":         false,
			"error":      err.Error(),
			"note":       "the query did not run. Do not bind anything to it until it does - and do not retry with guessed variable names, the declared ones are reported here.",
		}
		if declared := rqTool.declaredVars(ctx, projectId, tenantId, queryName); declared != nil {
			out["declared_variables"] = declared
		}
		return out, false, nil
	}
	studio.LedgerFrom(ctx).Record(RunQueryToolName)

	path, rows := locateRows(result)
	out := map[string]interface{}{
		"query_name":  queryName,
		"ran":         action,
		"ok":          true,
		"result_path": path,
		"row_count":   len(rows),
	}
	if len(rows) == 0 {
		out["note"] = "the query ran and returned no rows, so the columns are unknown. Run it again with variable values that match real data before binding to it."
		if declared := rqTool.declaredVars(ctx, projectId, tenantId, queryName); declared != nil {
			out["declared_variables"] = declared
		}
		return out, false, nil
	}
	out["columns"] = rowKeys(rows[0])
	sample := rows
	if len(sample) > MaxProbeRows {
		sample = sample[:MaxProbeRows]
	}
	out["sample_rows"] = sample
	logs.WithContext(ctx).Info(fmt.Sprintf("%s: %s returned %d row(s) at %q with columns %v", RunQueryToolName, subject, len(rows), path, rowKeys(rows[0])))
	return out, false, nil
}

// declaredVars is the variable names the saved query actually declares.
//
// Reported whenever a run does not produce rows, because the alternative is what
// it replaced: the agent re-running the same query with invented names -
// dt_from, rate_date_from, rtdt_from - learning nothing from each failure.
func (rqTool *RunQueryTool) declaredVars(ctx context.Context, projectId string, tenantId string, queryName string) []string {
	if queryName == "" {
		return nil
	}
	result, _, err := rqTool.Delegate.Execute(ctx, projectId, tenantId, "get_query", map[string]interface{}{
		"query_name": queryName,
	})
	if err != nil {
		logs.WithContext(ctx).Info(fmt.Sprint("could not read the declared variables of ", queryName, ": ", err.Error()))
		return nil
	}
	vars := findDeclaredVars(result)
	if vars == nil {
		return nil
	}
	names := make([]string, 0, len(vars))
	for name := range vars {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// findDeclaredVars digs the saved query's "vars" out of a fetch response,
// recognising the definition by the keys that identify one rather than by the
// envelope around it.
func findDeclaredVars(value interface{}) map[string]interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		_, named := typed["query_name"]
		if vars, ok := typed["vars"].(map[string]interface{}); ok && named {
			return vars
		}
		for _, key := range sortedKeys(typed) {
			if found := findDeclaredVars(typed[key]); found != nil {
				return found
			}
		}
	case []interface{}:
		for _, item := range typed {
			if found := findDeclaredVars(item); found != nil {
				return found
			}
		}
	}
	return nil
}

// locateRows finds the row array inside a response and reports the path that
// reaches it, in the dotted form a page property takes.
//
// The path is derived rather than assumed because there is no single envelope:
// an eru-ql sql query answers [{"Results": [...]}], other sources answer a bare
// array or a named object, and the page has to be told which one it is.
func locateRows(value interface{}) (string, []map[string]interface{}) {
	// Look inside first. An eru-ql answer is [{"Results": [...]}], and the
	// envelope is itself an array of one object - indistinguishable from a row
	// set until you have looked at what it wraps.
	switch typed := value.(type) {
	case map[string]interface{}:
		for _, key := range sortedKeys(typed) {
			if path, rows := locateRows(typed[key]); rows != nil {
				return join(key, path), rows
			}
		}
	case []interface{}:
		for i, item := range typed {
			if path, rows := locateRows(item); rows != nil {
				return join(fmt.Sprint(i), path), rows
			}
		}
	}

	if rows, ok := asRowSlice(value); ok && !wrapsRows(rows) {
		return "", rows
	}
	return "", nil
}

// wrapsRows reports objects that carry a row set rather than being one, so an
// envelope around an EMPTY result is not reported as a single row whose only
// column is the envelope key.
func wrapsRows(rows []map[string]interface{}) bool {
	for _, row := range rows {
		for _, value := range row {
			items, ok := value.([]interface{})
			if !ok {
				continue
			}
			if len(items) == 0 {
				return true
			}
			if _, isRow := items[0].(map[string]interface{}); isRow {
				return true
			}
		}
	}
	return false
}

// asRowSlice reports a non-empty array of objects - the thing a page plots,
// lists or reads a value out of.
func asRowSlice(value interface{}) ([]map[string]interface{}, bool) {
	items, ok := value.([]interface{})
	if !ok || len(items) == 0 {
		return nil, false
	}
	rows := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		row, ok := item.(map[string]interface{})
		if !ok {
			return nil, false
		}
		rows = append(rows, row)
	}
	return rows, true
}

func asString(value interface{}) string {
	text, _ := value.(string)
	return text
}

func join(head string, tail string) string {
	if tail == "" {
		return head
	}
	return head + "." + tail
}

func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
