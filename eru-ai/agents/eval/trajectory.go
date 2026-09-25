// # The frozen kernel
//
// This package is part of the code that JUDGES an agent's work, and nothing an
// agent produces may modify it. The others are agents/eval (the assertions and
// fixtures), agents/tool_record.go and agents/eru_studio/preflight.go (the
// instrumentation the judgement reads).
//
// The reason is measured, not theoretical. The Darwin Godel Machine
// (arXiv:2505.22954, Appendix H) had a variant score a perfect 2.0 by deleting
// the marker tokens its hallucination detector grepped for - "despite
// instructions not to change the special tokens". The detector found nothing,
// the score was perfect, the defect untouched, and a human reading diffs caught
// it. Their scope restriction was an instruction; HarnessX enforced write scope
// in code and it held.
//
// The same shape exists here already, with no self-modification anywhere:
// CodeQueryNotProbed is satisfiable by not recording the probe, and
// unboundProbedQueryIssues by not recording the query. Nothing can do that
// because this is compiled Go, which is the enforcement.
//
// If agent-authored validation is ever wanted, the rules become DATA that this
// package interprets - the interpreter stays here, and a rule never becomes
// code. That boundary needs a test the day it is built; today it is held by the
// fact that there is no path from a model's output to this file.
// Package eval scores what an agent run actually did.
//
// The validators an agent is held to at runtime judge its ANSWER. They cannot
// judge the run: whether it looked something up before binding to it, whether
// every step of a plan executed, whether the report it wrote matches the work it
// performed. Those live in the trajectory, and every defect found in a day of
// manual testing lived there - a tile bound to a query nobody ran, a four-step
// plan that silently completed one step, an entity reported as failed that was
// sitting in the data model.
//
// Nothing here is specific to one agent or one prompt. A Trajectory is the facts
// of a run; an Expectation is an assertion over those facts; a Fixture pairs a
// prompt with the assertions it should satisfy. The assertions are code and are
// written once; the fixtures are data and grow with the product.
package eval

import (
	"encoding/json"
	"sort"
	"strings"

	agents "github.com/eru-tech/eru/eru-ai/agents"
	models "github.com/eru-tech/eru/eru-ai/models"
)

// ToolCall is one tool invocation, reduced to what an assertion can stand on.
type ToolCall struct {
	Name      string                 `json:"name"`
	Agent     string                 `json:"agent,omitempty"`
	Iteration int                    `json:"iteration"`
	Input     map[string]interface{} `json:"input,omitempty"`
	Result    map[string]interface{} `json:"result,omitempty"`
}

// Trajectory is what one run did.
type Trajectory struct {
	Agent      string                     `json:"agent,omitempty"`
	Prompt     string                     `json:"prompt,omitempty"`
	ToolCalls  []ToolCall                 `json:"tool_calls"`
	Actions    []agents.AgentOutputAction `json:"actions,omitempty"`
	Iterations int                        `json:"iterations,omitempty"`
	DurationMs int64                      `json:"duration_ms,omitempty"`

	// ToolTally is the per-tool count the run reported in its metrics.
	//
	// It is NOT a second opinion: BuildMetrics derives it from these same traces,
	// so where the traces are incomplete the tally is incomplete identically.
	// That is why the page agent showed one run_query in a run where the service
	// log showed four - and why no assertion here can be trusted about an agent
	// whose tool calls go unrecorded.
	//
	// The independent record is the ledger: the tools write to it themselves, at
	// the point of execution, so it sees every call whichever loop made it. It is
	// not carried in the reply today; until it is, fixtures that read tool calls
	// declare NeedsToolVisibility and report BLOCKED rather than guessing.
	ToolTally map[string]int `json:"tool_tally,omitempty"`

	// Quality is every verdict the in-loop quality gate reached, in order.
	//
	// It is the only record of a judgement the run acted on, and the only way to
	// ask afterwards whether the judge was right: a verdict that shipped
	// unenforced is a fault the gate SAW and could not stop, which is a
	// different thing from a fault it missed, and the two need telling apart
	// before anyone tunes the rubric.
	Quality []agents.QualityVerdict `json:"quality,omitempty"`

	// StopReason is why the run ended, from the loop's closed set. A scenario
	// that passes every content assertion while stopping on a token limit is not
	// a passing scenario, and without this nothing could tell.
	StopReason agents.StopReason `json:"stop_reason,omitempty"`
}

// FromRun reduces an agent's reply to the facts worth asserting on.
func FromRun(prompt string, reply agents.AgentMessage) Trajectory {
	traj := Trajectory{Prompt: prompt, Actions: reply.Actions, StopReason: reply.StopReason}
	if reply.Metrics != nil {
		traj.Quality = reply.Metrics.Quality
		if reply.Metrics.TotalIterations > traj.Iterations {
			traj.Iterations = reply.Metrics.TotalIterations
		}
		// The record is written at the tool boundary, so it sees every call with
		// the arguments it was given - including calls a loop never traced. It
		// is the authority; traces only add what it lacks.
		for _, call := range reply.Metrics.ToolRecord {
			// The ACTION is what the model was shown and what an assertion names.
			// One tool can carry many actions - the processo tool alone offers
			// save_entity, save_field, save_page and thirty more - so recording
			// them all as "processo" made every per-action assertion unanswerable:
			// save_field appeared in the tally and the record held five calls
			// named "processo", and CalledWith reported it had never been called.
			name := strings.TrimSpace(call.Action)
			if name == "" {
				name = call.Tool
			}
			traj.ToolCalls = append(traj.ToolCalls, ToolCall{
				Name:  name,
				Input: call.Args,
			})
		}
		for _, metric := range reply.Metrics.ToolCalls {
			if metric.ToolName == "" {
				continue
			}
			if traj.ToolTally == nil {
				traj.ToolTally = map[string]int{}
			}
			traj.ToolTally[metric.ToolName] += metric.CallCount
		}
	}
	recorded := len(traj.ToolCalls) > 0
	for _, trace := range reply.Traces {
		if trace.Iteration > traj.Iterations {
			traj.Iterations = trace.Iteration
		}
		if strings.TrimSpace(trace.ToolName) == "" {
			continue
		}
		// With a record present the traces would double every call it already
		// holds, and a count assertion would read twice the truth.
		if recorded {
			if traj.Agent == "" {
				traj.Agent = trace.Agent
			}
			continue
		}
		traj.ToolCalls = append(traj.ToolCalls, ToolCall{
			Name:      trace.ToolName,
			Agent:     trace.Agent,
			Iteration: trace.Iteration,
			Input:     trace.ToolInput,
			Result:    trace.ToolResult,
		})
		if traj.Agent == "" {
			traj.Agent = trace.Agent
		}
	}
	return traj
}

// FromTraces builds a trajectory straight from traces, for a caller that has the
// steps but not a whole reply.
func FromTraces(prompt string, traces []models.StepTrace) Trajectory {
	return FromRun(prompt, agents.AgentMessage{Traces: traces})
}

// Calls returns every invocation of one tool, in order.
func (t Trajectory) Calls(tool string) []ToolCall {
	var out []ToolCall
	for _, call := range t.ToolCalls {
		if call.Name == tool {
			out = append(out, call)
		}
	}
	return out
}

// CallCount is how many times a tool ran: from the traces when they carry it,
// otherwise from the reported tally. An agent that traces nothing still counts.
func (t Trajectory) CallCount(tool string) int {
	if n := len(t.Calls(tool)); n > 0 {
		return n
	}
	return t.ToolTally[tool]
}

// Names lists the distinct tools used: those seen in the traces first, in
// call order, then any the tally knows about that the traces missed.
func (t Trajectory) Names() []string {
	var out []string
	seen := map[string]bool{}
	for _, call := range t.ToolCalls {
		if seen[call.Name] {
			continue
		}
		seen[call.Name] = true
		out = append(out, call.Name)
	}
	tallied := make([]string, 0, len(t.ToolTally))
	for name := range t.ToolTally {
		if !seen[name] {
			tallied = append(tallied, name)
		}
	}
	sort.Strings(tallied)
	return append(out, tallied...)
}

// reportsTools says whether this reply shows any tool use at all. A run that
// only ever "called" structured_output reported its answer and nothing else,
// which is the signature of an agent whose tool calls are not instrumented
// rather than one that used no tools.
func (t Trajectory) reportsTools() bool {
	for _, name := range t.Names() {
		if name != "structured_output" {
			return true
		}
	}
	return false
}

// firstIndex is where a tool was first called, or -1.
func (t Trajectory) firstIndex(tool string) int {
	for i, call := range t.ToolCalls {
		if call.Name == tool {
			return i
		}
	}
	return -1
}

// answer is the structured body the run reported, if it reported one.
func (t Trajectory) answer() map[string]interface{} {
	for _, action := range t.Actions {
		if action.ActionType == agents.ActionTypeAnswer && action.Action != nil {
			return action.Action
		}
	}
	for _, action := range t.Actions {
		if action.Action != nil {
			return action.Action
		}
	}
	return nil
}

// asked reports whether the run stopped to ask the user something.
func (t Trajectory) asked() bool {
	for _, action := range t.Actions {
		if action.ActionType == "question" {
			return true
		}
	}
	return len(t.Calls("ask_user")) > 0
}

// Marshal renders the trajectory, so a run can be recorded and replayed.
func (t Trajectory) Marshal() ([]byte, error) { return json.MarshalIndent(t, "", "  ") }
