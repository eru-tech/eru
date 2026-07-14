package reasoning_agents

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	agents "github.com/eru-tech/eru/eru-ai/agents"
	models "github.com/eru-tech/eru/eru-ai/models"
	tools "github.com/eru-tech/eru/eru-ai/tools"
	utility "github.com/eru-tech/eru/eru-ai/tools/utility"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eru_models "github.com/eru-tech/eru/eru-models"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
)

type ReasoningAgent struct {
	agents.Agent
	MaxIterations      int  `json:"max_iterations"`
	ThinkingBudget     int  `json:"thinking_budget"`
	EnableClarification bool `json:"enable_clarification"`
}

const clarificationGuidance = `

HUMAN-IN-THE-LOOP CLARIFICATION:
You may ask the user for clarification using the ask_user tool. Use it ONLY when the request is genuinely ambiguous or missing information you cannot infer or reasonably default — never to offload reasoning you can do yourself.
When you ask:
- Keep it to the fewest questions needed (ideally one).
- For each question provide 2-4 concrete, mutually exclusive options (value + human label).
- Set allow_free_text=true whenever the options may not be exhaustive, so the user can type their own answer.
- Set multi_select=true only when more than one option can legitimately be chosen.
Calling ask_user ends your turn; the user's answers will arrive as a follow-up message in the same conversation, after which you continue.`

func (ra *ReasoningAgent) GetSpec() agents.AgentI {
	return ra
}

func (ra *ReasoningAgent) UnmarshalJSON(b []byte) error {
	if err := json.Unmarshal(b, &ra.Agent); err != nil {
		return err
	}
	type reasoningFields struct {
		MaxIterations      int  `json:"max_iterations"`
		ThinkingBudget     int  `json:"thinking_budget"`
		EnableClarification bool `json:"enable_clarification"`
	}
	var rf reasoningFields
	if err := json.Unmarshal(b, &rf); err != nil {
		return err
	}
	ra.MaxIterations = rf.MaxIterations
	ra.ThinkingBudget = rf.ThinkingBudget
	ra.EnableClarification = rf.EnableClarification
	return nil
}

func (ra *ReasoningAgent) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("ReasoningAgent MakeFromJson - Start")
	err := json.Unmarshal(*rj, ra)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	if ra.MaxIterations <= 0 {
		ra.MaxIterations = 10
	}
	if ra.ThinkingBudget <= 0 {
		ra.ThinkingBudget = 10000
	}
	return nil
}

func (ra *ReasoningAgent) Execute(ctx context.Context, agentMessage agents.AgentMessage, conversationId string, projectId string, tenantId string) (agents.AgentMessage, error) {
	logs.WithContext(ctx).Debug("ReasoningAgent Execute - Start")
	ctx, span := otel.Tracer("eru-ai").Start(ctx, "ReasoningAgent.Execute",
		oteltrace.WithAttributes(attribute.String("agent_name", ra.AgentName), attribute.String("conversation_id", conversationId)),
	)
	defer span.End()
	startTime := time.Now()

	if answers, ok := agentMessage.ClarificationAnswers(); ok {
		var req agents.ClarificationRequest
		if priorConv, lerr := ra.LoadConversationHistory(ctx, conversationId, projectId, tenantId); lerr == nil && priorConv != nil {
			if _, qa, found := agents.PendingQuestion(priorConv); found {
				req, _ = agents.ParseClarificationRequest(qa.Action)
			}
		}
		answerText := agents.FormatAnswersForModel(req, answers)
		if strings.TrimSpace(agentMessage.Content) != "" {
			agentMessage.Content = agentMessage.Content + "\n\n" + answerText
		} else {
			agentMessage.Content = answerText
		}
	}

	if augment := buildCodeAugmentation(agentMessage.Params); augment != "" {
		if strings.TrimSpace(agentMessage.Content) == "" {
			agentMessage.Content = augment
		} else {
			agentMessage.Content = fmt.Sprintf("%s\n\n--- USER PROMPT ---\n%s", augment, agentMessage.Content)
		}
	}

	chatRequest, conversation, err := ra.LoadConversations(ctx, conversationId, agentMessage, projectId, tenantId)
	if err != nil {
		return agents.AgentMessage{}, err
	}

	if ra.Function.FuncGroupName != "" {
		return ra.executeWithFunction(ctx, agentMessage, conversation, projectId, tenantId)
	}

	toolsMap := make(map[string]tools.Tooling)
	for _, at := range ra.AgentTools {
		if at.Tool != nil {
			key := at.ToolKey
			if key == "" {
				key = at.ToolName
			}
			toolsMap[key] = at.Tool
		}
	}

	outputSchema := ra.getOutputSchema(ctx)
	if outputSchema.Type != "" {
		outputTool := &utility.StructuredOutputTool{}
		outputTool.SetAttribute(ctx, "output_schema", outputSchema)
		outputTool.SetAttribute(ctx, "parameters", outputSchema)
		outputTool.SetAttribute(ctx, "description", "Output the final result as structured JSON. Call this tool when you have your final answer ready.")
		outputTool.SetAttribute(ctx, "tool_name", "structured_output")
		outputTool.SetAttribute(ctx, "tool_type", "STRUCTURED_OUTPUT")
		outputTool.SetToolAction("structured_output")
		toolsMap["structured_output"] = outputTool
	}

	if ra.EnableClarification {
		askTool := &utility.AskUserTool{}
		askTool.SetAttribute(ctx, "parameters", utility.AskUserToolSchema())
		askTool.SetAttribute(ctx, "description", "Ask the user clarifying questions when the request is ambiguous or missing information needed to proceed. Provide 2-4 concrete, mutually exclusive options per question and allow free text when the options may not be exhaustive.")
		askTool.SetAttribute(ctx, "system_prompt", "")
		askTool.SetAttribute(ctx, "tool_name", utility.AskUserToolName)
		askTool.SetAttribute(ctx, "tool_type", "ASK_USER")
		askTool.SetToolAction(utility.AskUserToolName)
		toolsMap[utility.AskUserToolName] = askTool
	}

	toolExecutor := func(ctx context.Context, toolName string, input map[string]interface{}) (map[string]interface{}, error) {
		for _, at := range ra.AgentTools {
			if at.Tool == nil {
				continue
			}
			tnI, _ := at.Tool.GetAttribute(ctx, "tool_name")
			if tn, ok := tnI.(string); ok && tn == toolName {
				result, _, execErr := at.Tool.Execute(ctx, projectId, tenantId, at.ActionName, input)
				return result, execErr
			}
		}
		return nil, fmt.Errorf("tool %s not found", toolName)
	}

	sp := ra.SystemPrompt
	if ra.GetProvider() != nil {
		sp = ra.GetProvider().GetSystemPrompt() + "\n" + sp
	}
	if ra.EnableClarification {
		sp = sp + clarificationGuidance
	}

	// When the agent produces structured output, the final answer is the
	// structured_output tool payload, delivered to the client in the terminal
	// `done` event — NOT as streamed text. If the model instead emits the answer
	// as plain text (e.g. a markdown ```json block), those text_delta events must
	// NOT be forwarded, otherwise the whole answer leaks into the stream. Only
	// thinking is streamed for structured-output agents.
	suppressTextStream := outputSchema.Type != ""
	streamCb := agents.GetStreamCallback(ctx)
	runModel := func() (models.Message, []models.StepTrace, error) {
		if streamCb != nil {
			if streamingModel, ok := ra.Model.(models.StreamingModelI); ok {
				modelCb := func(me models.ModelStreamEvent) {
					if suppressTextStream && me.Type == models.StreamTextDelta {
						return
					}
					streamCb(agents.StreamEvent{
						Event:     string(me.Type),
						Data:      me,
						Iteration: me.Iteration,
					})
				}
				return streamingModel.RunToolLoopStreaming(ctx, chatRequest, toolsMap, sp, ra.MaxIterations, ra.ThinkingBudget, toolExecutor, modelCb)
			}
		}
		return ra.Model.RunToolLoop(ctx, chatRequest, toolsMap, sp, ra.MaxIterations, ra.ThinkingBudget, toolExecutor)
	}

	var response models.Message
	var traces []models.StepTrace
	var agentResponse map[string]interface{}
	attempt := 0
	for {
		response, traces, err = runModel()
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
			return agents.AgentMessage{}, err
		}

		agentResponse = parseAgentResponse(response.Content)
		if normalized, ok := deepUnstringifyJSON(agentResponse, "").(map[string]interface{}); ok {
			agentResponse = normalized
		}

		if response.TerminalTool == models.TerminalToolAskUser {
			break
		}

		valErr := validateAgainstSchema(agentResponse, outputSchema, "")
		if valErr == nil {
			break
		}
		logs.WithContext(ctx).Error(fmt.Sprintf("agent %s output validation failed (attempt %d of %d): %v", ra.AgentName, attempt+1, ra.RetryCount+1, valErr))
		if attempt >= ra.RetryCount {
			return agents.AgentMessage{}, fmt.Errorf("agent output failed JSON validation after %d attempt(s): %w", attempt+1, valErr)
		}
		chatRequest.Messages = append(chatRequest.Messages, models.Message{
			Role:    "user",
			Content: fmt.Sprintf(agentValidationRetryPrompt, valErr.Error()),
			Name:    ra.AgentName,
		})
		attempt++
	}

	metrics := agents.BuildMetrics(traces, startTime, response.Usage)

	actionType := agents.ActionTypeAnswer
	if response.TerminalTool == models.TerminalToolAskUser {
		actionType = agents.ActionTypeQuestion
	}

	agentOutput := agents.AgentMessage{
		Role: "assistant",
		Actions: []agents.AgentOutputAction{{
			ActionType: actionType,
			ActionName: ra.AgentName,
			Action:     agentResponse,
		}},
		Traces:           traces,
		Metrics:          metrics,
		MessageId:        agentMessage.MessageId,
		MessageTimestamp: time.Now(),
		RetryCount:       attempt,
	}

	conversation.Messages = append(conversation.Messages, agentOutput)
	conversation.NewMessages = append(conversation.NewMessages, agentOutput)
	err = ra.SaveConversation(ctx, conversation, projectId, tenantId)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("Failed to save conversation: %v", err))
		return agents.AgentMessage{}, err
	}

	return agentOutput, nil
}

func buildCodeAugmentation(params map[string]interface{}) string {
	if params == nil {
		return ""
	}
	codeRaw, ok := params["code"]
	if !ok {
		return ""
	}
	codeStr := stringifyParam(codeRaw)
	if strings.TrimSpace(codeStr) == "" || strings.TrimSpace(codeStr) == "{}" || strings.TrimSpace(codeStr) == "null" {
		return ""
	}
	var b strings.Builder
	b.WriteString("--- EXISTING STRUCTURED OUTPUT FROM PREVIOUS ATTEMPT ---\n")
	b.WriteString("This is the structured output (e.g. JSON or SQL) produced in a previous attempt. Build on top of it and improvize incorporating the user's new instructions: preserve what still applies unless the user prompt clearly requires changing it. If this is blank, generate a fresh output.\n\n")
	b.WriteString(codeStr)
	b.WriteString("\n--- END EXISTING STRUCTURED OUTPUT ---\n\n")
	return b.String()
}

func (ra *ReasoningAgent) getOutputSchema(ctx context.Context) eru_models.JSONSchema {
	outputSchema := ra.OutputSchema
	if ra.GetProvider() != nil {
		providerSchema := ra.GetProvider().GetOutputSchema(ctx)
		if providerSchema.Type != "" {
			outputSchema = providerSchema
		}
	}
	return outputSchema
}

func (ra *ReasoningAgent) executeWithFunction(ctx context.Context, agentMessage agents.AgentMessage, conversation *agents.Conversation, projectId string, tenantId string) (agents.AgentMessage, error) {
	response, err := ra.ExecuteAgentFunction(ctx, agentMessage, projectId, tenantId)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("Failed to execute agent function: %v", err))
		return agents.AgentMessage{}, err
	}

	responseBytes, err := json.Marshal(response)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("Failed to marshal agent function response: %v", err))
		return agents.AgentMessage{}, err
	}

	var agentMsg agents.AgentMessage
	err = json.Unmarshal(responseBytes, &agentMsg)
	if err != nil {
		agentMsg = agents.AgentMessage{
			Role: "assistant",
			Actions: []agents.AgentOutputAction{{
				Action: response,
			}},
			MessageId:        agentMessage.MessageId,
			MessageTimestamp: time.Now(),
		}
	}

	conversation.Messages = append(conversation.Messages, agentMsg)
	conversation.NewMessages = append(conversation.NewMessages, agentMsg)
	err = ra.SaveConversation(ctx, conversation, projectId, tenantId)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("Failed to save conversation: %v", err))
		return agents.AgentMessage{}, err
	}
	return agentMsg, nil
}

// parseAgentResponse turns the model's final Message.Content into a structured
// map suitable for AgentOutputAction.Action. It handles three cases:
//  1. Content is already valid JSON object → return parsed map.
//  2. Content is a JSON object wrapped in markdown fences (```json … ``` or
//     ``` … ```) → strip fences, parse, return parsed map. This is what
//     happens when the model returns its answer as plain text instead of
//     calling structured_output.
//  3. Anything else → return {"output": <raw content>}.
func parseAgentResponse(content string) map[string]interface{} {
	trimmed := strings.TrimSpace(content)

	if parsed, ok := tryUnmarshalObject(trimmed); ok {
		return parsed
	}

	if stripped, ok := stripMarkdownFences(trimmed); ok {
		if parsed, ok := tryUnmarshalObject(stripped); ok {
			return parsed
		}
	}

	if start, end, ok := findOuterJSONObject(trimmed); ok {
		if parsed, parsedOk := tryUnmarshalObject(trimmed[start : end+1]); parsedOk {
			return parsed
		}
	}

	return map[string]interface{}{"output": content}
}

// preserveStringJSONKeys lists object keys whose value is intentionally a
// stringified-JSON string that downstream consumers expect to stay a string.
// eru_studio component `data` properties are stringified JSON by design (see the
// eru_studio system prompt), so they must NOT be expanded into objects/arrays.
var preserveStringJSONKeys = map[string]bool{
	"data": true,
}

// deepUnstringifyJSON walks the entire parsed action and repairs values that a
// structured-output model emitted as stringified JSON instead of real JSON.
// Any string whose content is a JSON object/array is parsed back into that type
// and recursed into, so that a stringified `components` array, and deeply nested
// double/triple-encoded arrays inside free-form event payloads / entity_data
// (e.g. "fns":"[\"SEC-FIN-2\"]"), all become proper JSON. Strings that are not
// valid JSON are left untouched, as are keys in preserveStringJSONKeys. The key
// argument is the object key the value was found under ("" for array elements /
// the root).
func deepUnstringifyJSON(value interface{}, key string) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		for k, child := range v {
			v[k] = deepUnstringifyJSON(child, k)
		}
		return v
	case []interface{}:
		for i, item := range v {
			v[i] = deepUnstringifyJSON(item, "")
		}
		return v
	case string:
		if preserveStringJSONKeys[key] {
			return v
		}
		if parsed, ok := parseJSONStringFlexible(v); ok {
			return deepUnstringifyJSON(parsed, key)
		}
		return v
	default:
		return value
	}
}

// parseJSONStringFlexible parses a string that looks like a JSON object/array.
// It returns false for anything that is not JSON so plain strings are preserved.
// When a first parse fails because the model left raw control characters inside
// a string literal (a common truncation/formatting artifact that makes the whole
// payload unparseable), it escapes those control chars and retries once.
func parseJSONStringFlexible(s string) (interface{}, bool) {
	trimmed := strings.TrimSpace(s)
	if len(trimmed) == 0 {
		return nil, false
	}
	if c := trimmed[0]; c != '{' && c != '[' {
		return nil, false
	}
	var out interface{}
	if err := json.Unmarshal([]byte(trimmed), &out); err == nil {
		return out, true
	}
	sanitized := escapeControlCharsInStrings(trimmed)
	if sanitized == trimmed {
		return nil, false
	}
	if err := json.Unmarshal([]byte(sanitized), &out); err == nil {
		return out, true
	}
	return nil, false
}

// escapeControlCharsInStrings escapes raw control characters (<0x20) that appear
// INSIDE JSON string literals, leaving structural whitespace between tokens
// untouched so the result stays valid JSON.
func escapeControlCharsInStrings(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	inString := false
	escaped := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inString {
			if escaped {
				b.WriteByte(c)
				escaped = false
				continue
			}
			switch {
			case c == '\\':
				b.WriteByte(c)
				escaped = true
			case c == '"':
				b.WriteByte(c)
				inString = false
			case c < 0x20:
				switch c {
				case '\n':
					b.WriteString(`\n`)
				case '\t':
					b.WriteString(`\t`)
				case '\r':
					b.WriteString(`\r`)
				default:
					fmt.Fprintf(&b, `\u%04x`, c)
				}
			default:
				b.WriteByte(c)
			}
			continue
		}
		if c == '"' {
			inString = true
		}
		b.WriteByte(c)
	}
	return b.String()
}

// agentValidationRetryPrompt is fed back to the model when its output fails
// JSON validation, so it can self-correct on the next attempt.
const agentValidationRetryPrompt = `Your previous structured_output was NOT valid and was rejected: %s

Call structured_output again with a corrected result. Requirements:
- The ENTIRE output must be a single valid, parseable JSON object.
- Every field must match its declared type EXACTLY. In particular, array/object fields (e.g. ` + "`components`" + `) MUST be real JSON arrays/objects, NEVER a stringified JSON string.
- All control characters inside string values (newlines, tabs, quotes, backslashes) MUST be properly escaped.
Do not repeat the previous mistake.`

// validateAgainstSchema checks that value conforms to the type declared by
// schema wherever the schema is strict about arrays/objects. It is the
// validation step for reasoning agents (analogous to go-template execution for
// the gotemplate agent): if a field the schema declares as an array/object is
// still a string after normalization, the model emitted a stringified or
// malformed value that could not be repaired, and we return a descriptive error
// so the model can fix it on retry. Agents without an output schema
// (schema.Type == "") are not validated.
func validateAgainstSchema(value interface{}, schema eru_models.JSONSchema, path string) error {
	switch schema.Type {
	case "object":
		if s, isStr := value.(string); isStr {
			return jsonFieldError(path, "object", s)
		}
		m, ok := value.(map[string]interface{})
		if !ok {
			return nil
		}
		for key, propSchema := range schema.Properties {
			child, exists := m[key]
			if !exists {
				continue
			}
			if err := validateAgainstSchema(child, propSchema, joinSchemaPath(path, key)); err != nil {
				return err
			}
		}
	case "array":
		if s, isStr := value.(string); isStr {
			return jsonFieldError(path, "array", s)
		}
		arr, ok := value.([]interface{})
		if !ok {
			return nil
		}
		if schema.Items != nil {
			for i, item := range arr {
				if err := validateAgainstSchema(item, *schema.Items, fmt.Sprintf("%s[%d]", path, i)); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// jsonFieldError builds a descriptive validation error for a field that should
// be a JSON array/object but arrived as a string, including the underlying JSON
// parse error and a snippet around the offending byte to guide the model's fix.
func jsonFieldError(path, want, s string) error {
	field := path
	if field == "" {
		field = "<root>"
	}
	trimmed := strings.TrimSpace(s)
	var probe interface{}
	perr := json.Unmarshal([]byte(trimmed), &probe)
	if perr == nil {
		return fmt.Errorf("field %q must be a JSON %s, but it was sent as a stringified JSON string; emit it as a real %s, not a string", field, want, want)
	}
	snippet := ""
	if se, ok := perr.(*json.SyntaxError); ok {
		off := int(se.Offset)
		start := off - 60
		if start < 0 {
			start = 0
		}
		end := off + 60
		if end > len(trimmed) {
			end = len(trimmed)
		}
		snippet = fmt.Sprintf(" near byte %d: ...%s...", off, trimmed[start:end])
	}
	return fmt.Errorf("field %q must be a valid JSON %s but its value could not be parsed: %v%s", field, want, perr, snippet)
}

func joinSchemaPath(base, key string) string {
	if base == "" {
		return key
	}
	return base + "." + key
}

func tryUnmarshalObject(s string) (map[string]interface{}, bool) {
	if s == "" {
		return nil, false
	}
	out := map[string]interface{}{}
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil, false
	}
	return out, true
}

// stripMarkdownFences removes a single ```lang … ``` wrapper if present.
// Returns (stripped, true) when fences were found and removed.
func stripMarkdownFences(s string) (string, bool) {
	if !strings.HasPrefix(s, "```") {
		return s, false
	}
	rest := strings.TrimPrefix(s, "```")
	if nl := strings.IndexByte(rest, '\n'); nl >= 0 {
		rest = rest[nl+1:]
	}
	rest = strings.TrimSpace(rest)
	if idx := strings.LastIndex(rest, "```"); idx >= 0 {
		rest = strings.TrimSpace(rest[:idx])
	}
	return rest, true
}

// findOuterJSONObject locates the first balanced { … } object in the string,
// ignoring braces that appear inside JSON strings. This recovers JSON when
// the model precedes/follows it with prose.
func findOuterJSONObject(s string) (int, int, bool) {
	start := -1
	depth := 0
	inString := false
	escaped := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '{':
			if depth == 0 {
				start = i
			}
			depth++
		case '}':
			depth--
			if depth == 0 && start >= 0 {
				return start, i, true
			}
		}
	}
	return 0, 0, false
}
