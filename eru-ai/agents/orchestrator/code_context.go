package orchestrator

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	agents "github.com/eru-tech/eru/eru-ai/agents"
)

const (
	codeParamKey       = "code"
	codePreviewLimit   = 800
	codeFingerprintLen = 48
)

var sqlPrefixPattern = regexp.MustCompile(`(?is)^\s*(with|select|insert\s+into|update|delete\s+from|create\s+(table|view|index|function)|alter\s+table|drop\s+(table|view))\b`)

// codeContext describes the existing structured output the caller sent in
// params.code. The orchestrator plans with this description only - never with
// the artifact itself - so the planner can decide which sub-agents need it
// without the whole blob entering the planning prompt.
type codeContext struct {
	Present   bool
	Kind      string
	Size      int
	TopKeys   []string
	Preview   string
	Truncated bool
	code      string
}

// describeCodeParam characterises params.code: what kind of artifact it is, how
// big it is, its top-level keys and a short preview. A blank, empty or null
// artifact is treated as absent, matching how the sub-agents themselves ignore it.
func describeCodeParam(params map[string]interface{}) codeContext {
	if params == nil {
		return codeContext{}
	}
	raw, found := params[codeParamKey]
	if !found {
		return codeContext{}
	}
	code := strings.TrimSpace(stringifyCodeParam(raw))
	switch code {
	case "", "{}", "[]", "null", `""`:
		return codeContext{}
	}
	kind, topKeys := classifyCode(code)
	preview, truncated := previewCode(code)
	return codeContext{
		Present:   true,
		Kind:      kind,
		Size:      len(code),
		TopKeys:   topKeys,
		Preview:   preview,
		Truncated: truncated,
		code:      code,
	}
}

func stringifyCodeParam(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}

// classifyCode labels the artifact and, for JSON objects, returns its top-level
// keys - the strongest cheap signal of which agent produced it.
func classifyCode(code string) (string, []string) {
	if strings.HasPrefix(code, "{") {
		var obj map[string]interface{}
		if json.Unmarshal([]byte(code), &obj) == nil {
			keys := sortedMapKeys(obj)
			switch {
			case hasAnyKey(obj, "components", "eru_page", "page"):
				return "eru_page_json", keys
			case hasAnyKey(obj, "func_steps", "func_group_name"):
				return "func_group_json", keys
			}
			return "json_object", keys
		}
	}
	if strings.HasPrefix(code, "[") {
		var arr []interface{}
		if json.Unmarshal([]byte(code), &arr) == nil {
			return "json_array", nil
		}
	}
	if sqlPrefixPattern.MatchString(code) {
		return "sql", nil
	}
	if strings.Contains(code, "{{") && strings.Contains(code, "}}") {
		return "go_template", nil
	}
	if strings.HasPrefix(code, "<") {
		return "markup", nil
	}
	return "text", nil
}

func previewCode(code string) (string, bool) {
	if len(code) <= codePreviewLimit {
		return code, false
	}
	return code[:codePreviewLimit], true
}

func sortedMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func hasAnyKey(m map[string]interface{}, keys ...string) bool {
	for _, key := range keys {
		if _, found := m[key]; found {
			return true
		}
	}
	return false
}

// promptSection is appended to the planning system prompt only when
// params.code is present. It tells the planner what the artifact is and that it
// must be routed - by reference, to the steps that actually revise it - rather
// than broadcast to every step.
func (cc codeContext) promptSection(discovered []agents.DiscoveredAgent) string {
	if !cc.Present {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(`

============================================================
RULE #2c - INCOMING params.code : ROUTE IT, DO NOT BROADCAST IT
============================================================

The caller sent an EXISTING structured output (produced by an earlier run) in the
request payload. It is available to every step as .Vars.Body.params.code, but NO
step receives it unless your transform_request passes it explicitly.

What was sent:
`)
	sb.WriteString(fmt.Sprint("  Detected kind : ", cc.Kind, "\n"))
	sb.WriteString(fmt.Sprint("  Size          : ", cc.Size, " characters\n"))
	if len(cc.TopKeys) > 0 {
		sb.WriteString(fmt.Sprint("  Top-level keys: ", strings.Join(cc.TopKeys, ", "), "\n"))
	}
	if candidates := cc.candidateAgents(discovered); len(candidates) > 0 {
		sb.WriteString(fmt.Sprint("  Agents that produce this shape of output: ", strings.Join(candidates, ", "),
			" (a hint, not an instruction - confirm against the user's request)\n"))
	}
	if cc.Truncated {
		sb.WriteString(fmt.Sprint("  Preview (first ", codePreviewLimit, " characters, TRUNCATED - the step you route it to receives the whole artifact):\n"))
	} else {
		sb.WriteString("  Full value:\n")
	}
	sb.WriteString(indentBlock(cc.Preview, "    "))
	sb.WriteString(`

HOW TO DECIDE, per step:
- Route params.code ONLY to a step whose job is to produce a NEW VERSION of THIS
  SAME artifact - i.e. the step's output is the same kind of thing as above (page
  JSON to the page/widget agent, SQL to the SQL-generating agent, a go template to
  the template agent, and so on). Such a step must build on the existing artifact
  instead of starting from scratch.
- Usually that is EXACTLY ONE step. Do NOT pass it to steps that fetch or compute
  data, classify, route, validate, summarise, or produce a different kind of
  output: it is irrelevant noise to them, costs tokens, and misleads them.
- If NO step in your plan revises this artifact (the user's request is about
  something else entirely), do not reference params.code anywhere. That is a valid
  and expected outcome.
- If the artifact was clearly produced by an agent you are NOT using in this plan,
  do not route it at all.

HOW TO PASS IT (by reference - never paste the artifact into the template):
  "transform_request": "{{stringify (dict \"content\" .Vars.Body.content \"params\" (dict \"code\" .Vars.Body.params.code))}}"

Combined with fetched data for the same step (Rule #2b), both keys go in one params dict:
  "transform_request": "{{stringify (dict \"content\" .Vars.Body.content \"params\" (dict \"code\" .Vars.Body.params.code \"context\" (stringify .ResVars.<data_step>.Body)))}}"

WRONG:
  pasting the artifact's text/JSON literally into transform_request  -> bloats the plan, breaks the template on quotes
  adding params.code to every step                                  -> information overload, wasted tokens
  "params" (dict "code" .Vars.Body.content)                          -> that is the user's instruction, not the artifact

CHECKLIST ADDITION:
[ ] params.code is passed - by .Vars.Body.params.code reference - only to the step(s) that revise the artifact described above, and to no other step`)
	return sb.String()
}

// candidateAgents names the discovered agents whose declared output looks like
// the incoming artifact, either by output-field overlap with its top-level keys
// or by a keyword match on the detected kind.
func (cc codeContext) candidateAgents(discovered []agents.DiscoveredAgent) []string {
	keySet := make(map[string]bool)
	for _, key := range cc.TopKeys {
		keySet[strings.ToLower(key)] = true
	}
	var candidates []string
	for _, ad := range discovered {
		fields := outputFieldNames(ad.OutputSchema)
		overlap := 0
		kindMatch := false
		for _, field := range fields {
			lower := strings.ToLower(field)
			if keySet[lower] {
				overlap++
			}
			if cc.Kind == "sql" && lower == "sql" {
				kindMatch = true
			}
			if cc.Kind == "go_template" && (lower == "template" || lower == "code") {
				kindMatch = true
			}
		}
		if overlap >= 2 || kindMatch {
			candidates = append(candidates, ad.AgentName)
		}
	}
	sort.Strings(candidates)
	return candidates
}

func indentBlock(text string, indent string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = indent + line
	}
	return strings.Join(lines, "\n")
}

// fingerprint returns a normalised slice of the artifact used to detect a plan
// that pasted the artifact into a template instead of referencing it. Quotes,
// escapes and whitespace are stripped so the match survives JSON escaping and
// reformatting.
func (cc codeContext) fingerprint() string {
	if !cc.Present {
		return ""
	}
	normalised := normaliseForFingerprint(cc.code)
	if len(normalised) < codeFingerprintLen {
		return ""
	}
	start := len(normalised) / 4
	if start+codeFingerprintLen > len(normalised) {
		start = len(normalised) - codeFingerprintLen
	}
	return normalised[start : start+codeFingerprintLen]
}

func normaliseForFingerprint(s string) string {
	var sb strings.Builder
	sb.Grow(len(s))
	for _, c := range s {
		switch c {
		case ' ', '\t', '\n', '\r', '"', '\'', '\\':
			continue
		}
		sb.WriteRune(c)
	}
	return sb.String()
}
