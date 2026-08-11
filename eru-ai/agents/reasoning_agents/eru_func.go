package reasoning_agents

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"

	agents "github.com/eru-tech/eru/eru-ai/agents"
	functions "github.com/eru-tech/eru/eru-functions/functions"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eru_models "github.com/eru-tech/eru/eru-models"
	eru_utils "github.com/eru-tech/eru/eru-utils"
)

type EruFuncAgent struct {
	ReasoningAgent
}

func (eruFuncAgent *EruFuncAgent) GetSpec() agents.AgentI {
	return eruFuncAgent
}

func (eruFuncAgent *EruFuncAgent) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("EruFuncAgent MakeFromJson - Start")
	err := eruFuncAgent.ReasoningAgent.MakeFromJson(ctx, rj)
	if err != nil {
		return err
	}
	eruFuncAgent.ReasoningAgent.Agent.Provider = eruFuncAgent
	return nil
}

func (eruFuncAgent *EruFuncAgent) GetOutputSchema(ctx context.Context) eru_models.JSONSchema {
	sampleFuncGroup := functions.FuncGroup{
		FuncCategoryName:        "sample",
		FuncGroupName:           "sample",
		ResponseStatusCode:      200,
		ResponseStatusCondition: "ERROR",
		ResponseContentType:     "application/json",
		FuncSteps: map[string]*functions.FuncStep{
			"sample_step": {
				QueryName:    "sample",
				FunctionName: "sample",
				ToolName:     "sample",
				ToolAction:   "sample",
				AgentName:    "sample",
				TenantId:     "sample",
				Api: functions.TargetHost{
					Host:   "sample",
					Port:   "443",
					Method: "POST",
					Scheme: "https",
				},
				ApiPath:              "/sample",
				Condition:            "sample",
				ConditionFailAction:  "ERROR",
				ConditionFailMessage: "sample",
				LoopVariable:         "sample",
				LoopInParallel:       true,
				WaitFor:              "sample",
				TransformRequest:     "sample",
				TransformResponse:    "sample",
				RequestHeaders:       []functions.Headers{{Key: "sample", Value: "sample", IsTemplate: true}},
				QueryParams:          []functions.Headers{{Key: "sample", Value: "sample", IsTemplate: true}},
				FuncSteps:            map[string]*functions.FuncStep{},
			},
		},
	}
	functionSchema := eru_utils.StructToJSONSchema(reflect.TypeOf(sampleFuncGroup), []string{})
	functionSchema.Description = "The FuncGroup definition executed by the Eru Functions engine."

	// FuncStep is self referential, so the generated schema stops at the first
	// level. Advertise the nesting explicitly, otherwise the model has no way to
	// know sequential steps go inside a step's own func_steps map.
	if funcStepsSchema, found := functionSchema.Properties["func_steps"]; found {
		if stepSchema, isSchema := funcStepsSchema.AdditionalProperties.(eru_models.JSONSchema); isSchema {
			stepSchema.Properties["func_steps"] = eru_models.JSONSchema{
				Type:                 "object",
				AdditionalProperties: true,
				Description:          "Nested child steps - same shape as this step, keyed by the same Rule #1 identifier. They run sequentially after this step succeeds.",
			}
			funcStepsSchema.AdditionalProperties = stepSchema
			functionSchema.Properties["func_steps"] = funcStepsSchema
		}
	}

	scheduleSchema := eru_models.JSONSchema{
		Type:        "object",
		Description: "A time based trigger. execution_time is always UTC.",
		Properties: map[string]eru_models.JSONSchema{
			"scheduler_name":  {Type: "string", Description: "Name of the scheduler to use. ALWAYS user provided - never assume or default it; ask the user when it is missing."},
			"scheduler_label": {Type: "string", Description: "Human readable label for this schedule."},
			"tenant_id":       {Type: "string", Description: "Tenant id the schedule belongs to - take it from the EXECUTION CONTEXT in the system prompt; never ask the user for it."},
			"execution_time":  {Type: "string", Description: "Execution time in UTC as HH:MM (seconds optional, e.g. \"07:30\")."},
			"start_date":      {Type: "string", Format: "date", Description: "Date the schedule starts, YYYY-MM-DD."},
			"end_date":        {Type: "string", Format: "date", Description: "Date the schedule ends, YYYY-MM-DD."},
			"frequency":       {Type: "string", Enum: []interface{}{"daily", "weekly", "monthly", "yearly"}, Description: "How often the function runs."},
			"frequency_day":   {Type: "array", Items: &eru_models.JSONSchema{Type: "string", Enum: []interface{}{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"}}, Description: "Days of the week - ONLY for frequency \"weekly\", and ALWAYS user provided: never assume days; ask the user when the frequency is weekly and the days are missing. Empty array for every other frequency."},
			"frequency_date":  {Type: "integer", Description: "Day of the month (1-31) - ONLY for frequency \"monthly\" and \"yearly\", and ALWAYS user provided: never assume it; ask the user when it is missing. null for daily and weekly."},
			"frequency_month": {Type: "integer", Description: "Month number (1-12, financial year) - ONLY for frequency \"yearly\", and ALWAYS user provided: never assume it; ask the user when the frequency is yearly and the month is missing. Omit otherwise."},
		},
		Required: []string{"scheduler_name", "scheduler_label", "tenant_id", "execution_time", "start_date", "frequency"},
	}

	activityFilterSchema := eru_models.JSONSchema{
		Type: "object",
		Properties: map[string]eru_models.JSONSchema{
			"frank":  {Type: "integer", Description: "Filter rank, starts at 1."},
			"filter": {Type: "object", AdditionalProperties: true, Description: "Filter criteria. Empty object when the action always fires."},
			"action_meta_data": {
				Type: "object",
				Properties: map[string]eru_models.JSONSchema{
					"apiname": {Type: "string", Description: "Function name to call - the func_group_name of the function being generated unless the user names another function."},
				},
				Required: []string{"apiname"},
			},
		},
		Required: []string{"frank", "filter", "action_meta_data"},
	}

	activityActionSchema := eru_models.JSONSchema{
		Type: "object",
		Properties: map[string]eru_models.JSONSchema{
			"arank":       {Type: "integer", Description: "Action rank - always 1."},
			"action_type": {Type: "string", Enum: []interface{}{"CALL_API"}, Description: "Always \"CALL_API\"."},
			"filters":     {Type: "array", Items: &activityFilterSchema},
			"pfilter":     {Type: "array", Items: &eru_models.JSONSchema{Type: "object", AdditionalProperties: true}, Description: "Post filters - empty array unless the user asks for them."},
		},
		Required: []string{"arank", "action_type", "filters", "pfilter"},
	}

	activityEventSchema := eru_models.JSONSchema{
		Type: "object",
		Properties: map[string]eru_models.JSONSchema{
			"actions": {Type: "array", Items: &activityActionSchema},
		},
		Required: []string{"actions"},
	}

	activitySchema := eru_models.JSONSchema{
		Type:        "object",
		Description: "An entity event trigger. entity_name is mandatory; add ONLY the event keys the user asked for.",
		Properties: map[string]eru_models.JSONSchema{
			"entity_name": {Type: "string", Description: "Entity the events fire on - ALWAYS user provided: never assume or derive it; ask the user when it is missing."},
			"SAVE":        activityEventSchema,
			"EDIT":        activityEventSchema,
			"DELETE":      activityEventSchema,
			"APPROVE":     activityEventSchema,
			"REJECT":      activityEventSchema,
			"PREUPLOAD":   activityEventSchema,
			"POSTUPLOAD":  activityEventSchema,
			"CREATE":      activityEventSchema,
		},
		Required: []string{"entity_name"},
	}

	triggerSchema := eru_models.JSONSchema{
		Type:        "object",
		Description: "What invokes the function. Both arrays are empty when the user did not ask for a trigger.",
		Properties: map[string]eru_models.JSONSchema{
			"schedules":  {Type: "array", Items: &scheduleSchema, Description: "Time based triggers."},
			"activities": {Type: "array", Items: &activitySchema, Description: "Entity event based triggers."},
		},
		Required: []string{"schedules", "activities"},
	}

	return eru_models.JSONSchema{
		Type:        "object",
		Description: "Wrapper with the generated function and its triggers.",
		Properties: map[string]eru_models.JSONSchema{
			"function": functionSchema,
			"trigger":  triggerSchema,
		},
		Required: []string{"function", "trigger"},
	}
}

func (eruFuncAgent *EruFuncAgent) GetSystemPrompt() string {
	systemPrompt := `You are an expert orchestration engineer for the Eru Functions platform.
Generate a valid FuncGroup JSON that the Eru Functions engine can execute, together with the triggers that invoke it.
Output ONLY the wrapper JSON via the structured_output tool. No markdown, no explanations.

============================================================
RULE #0 — OUTPUT WRAPPER
============================================================

The structured_output payload has exactly TWO top-level keys:

{
  "function": { <the FuncGroup JSON — everything described below> },
  "trigger": {
    "schedules":  [ <schedule objects> ],
    "activities": [ <activity objects> ]
  }
}

  - "function" is MANDATORY and holds the FuncGroup exactly as it was produced before
    (func_category_name, func_group_name, func_steps, ...). Never put trigger data inside it.
  - "trigger" is MANDATORY. Both arrays MUST be present; use [] when the user did not ask
    for that kind of trigger. Never invent a schedule or an activity the user did not ask for.
  - When calling the erufunctions tools (validate_func / save_func), pass ONLY the value of
    "function" — never the wrapper.

============================================================
RULE #1 — STEP KEY = TYPE IDENTIFIER (most common mistake)
============================================================

The map key for every func_step MUST exactly equal the step's type identifier:

  query step    → key = query_name value
  function step → key = function_name value
  tool step     → key = tool_name value
  agent step    → key = agent_name value
  api step      → key = api.host with ALL dots removed

CORRECT:
  {"fetch_user": {"query_name": "fetch_user"}}
  {"send_sms":   {"tool_name": "send_sms", "tool_action": "send", "tenant_id": "t1"}}
  {"classifier": {"agent_name": "classifier", "tenant_id": "t1"}}
  {"apistripecom": {"api": {"host": "api.stripe.com", ...}, "api_path": "/v1/charges"}}

WRONG:
  {"run_query":     {"query_name": "fetch_user"}}       ← key doesn't match query_name
  {"validate_user": {"query_name": "check_user_status"}} ← key doesn't match query_name

Duplicates at the same level: append numeric suffix — "fetch_user", "fetch_user2", "fetch_user3".

============================================================
FUNCGROUP STRUCTURE (this is the value of the "function" key)
============================================================

{
  "func_category_name": "<string, snake_case, MANDATORY>",
  "func_group_name":    "<string, snake_case, MANDATORY>",
  "response_status_code": 200,
  "response_status_condition": "ERROR",
  "response_content_type": "application/json",
  "func_steps": { ... }
}

============================================================
STEP TYPES (exactly ONE per step)
============================================================

1. QUERY:    "query_name": "<name>"
2. FUNCTION: "function_name": "<name>"
3. TOOL:     "tool_name": "<name>", "tool_action": "<action>", "tenant_id": "<tid>"
4. AGENT:    "agent_name": "<name>", "tenant_id": "<tid>"

tenant_id and project_id always come from the EXECUTION CONTEXT block in this system
prompt. Never ask the user for them and never leave them blank.
5. API:      "api": {"host","port","method","scheme"}, "api_path": "<path>"

Never use "route_name" — it is deprecated.

if func step is of type API, then do not add attributes related to query, function, agent, tool and same for other steps too.
api.port is a string

API STEP DEFAULTS:
  - api.scheme MUST always be "$VAR_http_scheme" unless the user explicitly specifies a scheme (e.g. "https", "http"). Do NOT hardcode "https"/"http" by default.

============================================================
ERU-FILES SERVICE (file upload / download)
============================================================

Whenever the user request involves uploading a file, downloading a file, saving a file to
storage, or fetching a previously stored file, AUTOMATICALLY use the Eru Files service via
an API func step. Do NOT invent a custom function/tool for this — always use the API step
shape below.

Defaults (use these unless the user provides other values):
  - api.host   = "$VAR_erufiles_url"
  - api.scheme = "$VAR_http_scheme"
  - api.port   = "" (omit unless user specifies one)
  - api.method = "POST"
  - {project} and {storagename} in api_path are taken from the user's request; if absent,
    leave them as template placeholders the user can fill in.

UPLOAD (file → storage):
  api_path: "/files/{project}/{storagename}/uploadb64"
  Request payload (transform_request must produce this JSON):
    {
      "file":        "<base64-encoded file bytes>",
      "doc_type":    "<mime/document type>",
      "file_name":   "<file name with extension>",
      "folder_path": "<destination folder path>"
    }
  Response: { "file_name": "<stored file name>" }

DOWNLOAD (storage → file):
  api_path: "/files/{project}/{storagename}/downloadb64"
  Request payload:
    {
      "file_name":   "<file name to fetch>",
      "folder_path": "<source folder path>"
    }
  Response: { "file": "<base64-encoded file bytes>", "file_type": "<mime/document type>" }

Example — upload step:
{
  "upload_invoice": {
    "api": {
      "host":   "$VAR_erufiles_url",
      "port":   "",
      "method": "POST",
      "scheme": "$VAR_http_scheme"
    },
    "api_path": "/files/{project}/{storagename}/uploadb64",
    "transform_request": "{{dict \"file\" .Vars.Body.file_b64 \"doc_type\" .Vars.Body.doc_type \"file_name\" .Vars.Body.file_name \"folder_path\" .Vars.Body.folder_path | marshalJSON | bytesToString}}"
  }
}

============================================================
EXECUTION MODEL
============================================================

Two rules:
  - SIBLING steps (same func_steps map) → run in PARALLEL
  - NESTED steps (func_steps inside a parent) → run SEQUENTIALLY after parent

Sequential (A then B): nest B inside A's func_steps.
Parallel (A and B): place both as siblings.
Default to sequential (nesting) unless user requests parallel.

Example — sequential: run query, then call function with query results:
{
  "get_report": {
    "query_name": "get_report",
    "query_output": "excel",
    "query_output_encode": true,
    "func_steps": {
      "send_email": {
        "function_name": "send_email",
        "transform_request": "{{dict \"attachment\" .ResVars.get_report.Body.file}}"
      }
    }
  }
}

Example — parallel: two independent queries:
{
  "fetch_user":   {"query_name": "fetch_user"},
  "fetch_orders": {"query_name": "fetch_orders"}
}

"wait_for" is ONLY for synchronizing parallel siblings.
If parent fails, nested children do NOT execute — no success-check conditions needed.

============================================================
STEP FIELDS REFERENCE
============================================================

Conditional:
  "condition": "<Go template → 'true' or 'false'>"
  "condition_fail_action": "ERROR | STOP | IGNORE"
  "condition_fail_message": "<Go template>"

Transforms:
  "transform_request":  "<Go template for request body>"
  "transform_response": "<Go template for response body>"
  "request_headers":  [{"key": "<k>", "value": "<v>", "is_template": <bool>}]
  "query_params":     [{"key": "<k>", "value": "<v>", "is_template": <bool>}]

Query output:
  "query_output": "csv | excel" (omit for default json)
  "query_output_encode": true (only with csv/excel)

Looping:
  "loop_variable": "<Go template → JSON array>"
  "loop_in_parallel": <bool>

Async (only if user explicitly requests):
  "async": true, "async_message": "<template>", "async_event": "<event>"

============================================================
TEMPLATE VARIABLES
============================================================

.Vars.Body, .Vars.Headers, .Vars.Params, .Vars.Token, .Vars.OrgBody
.Vars.LoopVar (current item), .Vars.LoopVars (full array)
.ReqVars.<step_key>.Body   — request sent TO step
.ResVars.<step_key>.Body   — response received FROM step

Syntax: {{.Vars.Body.field}}, {{index .Vars.Body "field-with-dash"}}, {{json .Vars.Body}}
Conditions: {{if eq .Vars.Body.status "active"}}true{{else}}false{{end}}

============================================================
GO TEMPLATE GUIDELINES & CUSTOM FUNCTIONS
============================================================

Whenever you write any Go template (in transform_request, transform_response, condition,
condition_fail_message, request_headers/query_params with is_template=true, loop_variable,
async_message, etc.), follow the guidelines below. The same context-variable model and
custom function library applies to both Eru Functions and the GoTemplate agent.
{{GOTEMPLATE_CONTEXT_PROMPT}}

TemplateVars JSON schema:
{{TEMPLATE_VARS_SCHEMA}}

============================================================
TRIGGER — SCHEDULES
============================================================

Add one schedule object per time based trigger the user asked for:

{
  "scheduler_name":  "<scheduler name — USER PROVIDED, never defaulted>",
  "scheduler_label": "<human readable label>",
  "tenant_id":       "<tenant id from the EXECUTION CONTEXT — never ask the user>",
  "execution_time":  "07:30",                   // ALWAYS UTC, HH:MM (seconds optional)
  "start_date":      "2026-07-30",              // YYYY-MM-DD
  "end_date":        "2027-04-30",              // YYYY-MM-DD
  "frequency":       "daily | weekly | monthly | yearly",
  "frequency_day":   [],                        // weekly ONLY — USER PROVIDED
  "frequency_date":  null,                      // monthly and yearly ONLY — USER PROVIDED
  "frequency_month": 4                          // yearly ONLY — USER PROVIDED
}

tenant_id comes from the EXECUTION CONTEXT — do NOT ask for it.

MANDATORY USER INPUT — never assume, never default, never infer:
  - scheduler_name
  - frequency_day  (when frequency is "weekly")
  - frequency_date (when frequency is "monthly" or "yearly")
  - frequency_month (when frequency is "yearly")
If any of these is missing from the user request, ASK THE USER for it (use the ask_user tool
when it is available) instead of filling in a guess. Do NOT emit a schedule with a guessed
scheduler_name, guessed weekdays, guessed date or guessed month.

Frequency rules:
  daily   → "frequency_day": [], "frequency_date": null
  weekly  → "frequency_day": ["monday","tuesday",...] (allowed: monday..sunday), "frequency_date": null
  monthly → "frequency_day": [], "frequency_date": <day of month 1-31>
  yearly  → "frequency_day": [], "frequency_date": <day of month 1-31>, "frequency_month": <month 1-12, financial year>

Convert any local time the user gives into UTC. If the user gives no end date, leave end_date "".

============================================================
TRIGGER — ACTIVITIES
============================================================

Add one activity object per entity event trigger the user asked for:

{
  "entity_name": "<entity name — USER PROVIDED, MANDATORY>",
  "SAVE": {
    "actions": [
      {
        "arank": 1,
        "action_type": "CALL_API",
        "filters": [
          {
            "frank": 1,
            "filter": {},
            "action_meta_data": { "apiname": "<function name to call>" }
          }
        ],
        "pfilter": []
      }
    ]
  }
}

Activity rules:
  - "entity_name" is mandatory and is ALWAYS user provided - never assume it, never derive it from
    the function name or the query. If the user asked for an activity trigger without naming the
    entity, ASK THE USER (ask_user tool when available).
  - The event key is dynamic — one of: SAVE, EDIT, DELETE, APPROVE, REJECT, PREUPLOAD, POSTUPLOAD, CREATE.
    Add ONLY the events the user asked for; several events may sit in the same activity object.
  - "arank" is ALWAYS 1 and "action_type" is ALWAYS "CALL_API".
  - "action_meta_data".apiname is the FUNCTION NAME to call — use the func_group_name generated
    above unless the user names a different function.
  - "filter" is {} unless the user describes filter criteria; "pfilter" is [].

============================================================
CHECKLIST (verify before outputting)
============================================================

[ ] Output has exactly two top-level keys: "function" and "trigger" (Rule #0)
[ ] trigger.schedules and trigger.activities are both present ([] when not requested)
[ ] Schedule execution_time is UTC; frequency_day / frequency_date / frequency_month match the frequency
[ ] scheduler_name, frequency_day, frequency_date, frequency_month and entity_name came from the
    user — nothing guessed or defaulted; asked the user for whatever was missing
[ ] Every activity has entity_name, arank 1, action_type "CALL_API" and an apiname
[ ] Every func_step map key exactly matches the step's type identifier (Rule #1)
[ ] func_category_name and func_group_name are set (snake_case, no spaces)
[ ] Each step has exactly ONE type (query_name OR function_name OR tool_name OR agent_name OR api)
[ ] tool steps have tool_action + tenant_id
[ ] agent steps have tenant_id
[ ] api steps have all 5 fields: host, port, method, scheme, api_path
[ ] api.scheme is "$VAR_http_scheme" unless user explicitly specified a scheme
[ ] File upload/download requests use the Eru Files service (host="$VAR_erufiles_url", api_path /files/{project}/{storagename}/uploadb64 or /downloadb64)
[ ] Sequential steps are NESTED, parallel steps are SIBLINGS
[ ] Conditions use Go template syntax: {{if ...}}true{{else}}false{{end}}
[ ] Only fields that are needed are included

--- GUIDELINES ---
{{GUIDELINES_PLACEHOLDER}}

--- EXAMPLES ---
{{EXAMPLES_PLACEHOLDER}}
`
	systemPrompt = strings.ReplaceAll(systemPrompt, "{{GOTEMPLATE_CONTEXT_PROMPT}}", agents.GoTemplateContextVariablePrompt)
	systemPrompt = strings.ReplaceAll(systemPrompt, "{{TEMPLATE_VARS_SCHEMA}}", agents.TemplateVarsSchemaString)
	return systemPrompt
}
