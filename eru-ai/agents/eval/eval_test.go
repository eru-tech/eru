package eval

import (
	"strings"
	"testing"

	agents "github.com/eru-tech/eru/eru-ai/agents"
	models "github.com/eru-tech/eru/eru-ai/models"
)

// run builds a trajectory the way a real reply would - including the stop
// reason, because a real reply always carries one and a helper that omits it
// quietly exempts every assertion about how a run ended.
func run(actions []agents.AgentOutputAction, traces ...models.StepTrace) Trajectory {
	return runStopping(agents.StopEndTurn, actions, traces...)
}

func runStopping(reason agents.StopReason, actions []agents.AgentOutputAction, traces ...models.StepTrace) Trajectory {
	return FromRun("p", agents.AgentMessage{Traces: traces, Actions: actions, StopReason: reason})
}

func call(iteration int, tool string, input map[string]interface{}) models.StepTrace {
	return models.StepTrace{Iteration: iteration, ToolName: tool, ToolInput: input, Agent: "a"}
}

func answer(body map[string]interface{}) []agents.AgentOutputAction {
	return []agents.AgentOutputAction{{ActionType: agents.ActionTypeAnswer, Action: body}}
}

// ─── the storage_name defect ────────────────────────────────────────────────

// What actually happened: no question, a save_field carrying storage_name
// "default", and a field left pointing at storage that does not exist.
func TestInventedStorageIsCaught(t *testing.T) {
	// It finished instead of asking - which is the fault, and is now also
	// visible in how the run ended.
	bad := runStopping(agents.StopEndTurn, answer(map[string]interface{}{"summary": "done"}),
		call(2, "get_field_spec", map[string]interface{}{"datatypes": []interface{}{"attachment"}}),
		call(3, "save_field", map[string]interface{}{
			"field": map[string]interface{}{"name": "test_scan", "storage_name": "default"},
		}),
	)
	fixture, _ := Scenarios().ByName("attachment_without_storage_asks")
	result := fixture.Score(bad)
	if result.OK() {
		t.Fatal("a run that invented a storage name should fail this fixture")
	}
	// Three now: it did not ask, it wrote anyway, and it ended as though it had
	// finished the job.
	if len(result.Failures) != 3 {
		t.Errorf("expected the missing question, the write and the wrong ending, got %+v", result.Failures)
	}
}

// What we want: a question, and nothing written.
func TestAskingInsteadOfGuessingPasses(t *testing.T) {
	good := runStopping(agents.StopAskedUser, []agents.AgentOutputAction{{ActionType: "question"}},
		call(2, "get_field_spec", map[string]interface{}{"datatypes": []interface{}{"attachment"}}),
	)
	fixture, _ := Scenarios().ByName("attachment_without_storage_asks")
	if result := fixture.Score(good); !result.OK() {
		t.Errorf("asking should pass: %s", result)
	}
}

// Argument correctness: the storage the user gave must be the one that is sent.
func TestStorageFromTheUserMustBeTheOneSent(t *testing.T) {
	fixture, _ := Scenarios().ByName("attachment_with_storage_writes")

	wrong := run(answer(map[string]interface{}{"summary": "done"}),
		call(3, "save_field", map[string]interface{}{
			"field": map[string]interface{}{"storage_name": "default"},
		}),
	)
	if fixture.Score(wrong).OK() {
		t.Error("a save that substituted its own storage name should fail")
	}

	right := run(answer(map[string]interface{}{"summary": "done"}),
		call(3, "save_field", map[string]interface{}{
			"field": map[string]interface{}{"storage_name": "sample_storage"},
		}),
	)
	if result := fixture.Score(right); !result.OK() {
		t.Errorf("the correct storage should pass: %s", result)
	}
}

// ─── the dropped-steps defect ───────────────────────────────────────────────

// The clarification resume ran the step that asked and abandoned the rest: one
// entity of three, no page, and no word about it.
//
// Checked against the WORKSPACE, not the call log. The call-count version of
// this assertion failed a correct run: on a second pass the entities already
// existed and the agent rightly extended them, making no save_entity calls at
// all. What matters is that the three entities are there afterwards.
func TestAPlanThatStoppedEarlyIsCaught(t *testing.T) {
	stopped := run(answer(map[string]interface{}{
		"entities": []interface{}{map[string]interface{}{"action": "created", "name": "BusinessCard"}},
	}),
		call(1, "save_entity", map[string]interface{}{"entity_data": []interface{}{}}),
		call(2, "save_field", map[string]interface{}{}),
	)
	fixture, _ := Scenarios().ByName("capture_request_builds_model_and_page")

	// One of three built.
	before := Outcome{Read: true, Entities: map[string]EntityState{}}
	partial := Outcome{Read: true, Entities: map[string]EntityState{
		"business_card": {Name: "business_card", Fields: []string{"first_name", "last_name"}},
	}}
	result := fixture.ScoreOutcome(fixture.Score(stopped), before, partial)
	if result.OK() {
		t.Fatal("one entity of three should fail")
	}
	if !strings.Contains(result.String(), "address") || !strings.Contains(result.String(), "contact") {
		t.Errorf("the failure should name what is missing: %s", result)
	}
}

// The same fixture passes when all three are present, no matter whether this run
// created them or an earlier one did.
func TestAllThreeEntitiesPresentPasses(t *testing.T) {
	finished := run(answer(map[string]interface{}{"summary": "extended what was there"}),
		call(1, "save_field", map[string]interface{}{}),
	)
	before := Outcome{Read: true, Entities: map[string]EntityState{}}
	// Prefixed names, as a later run actually chose - the fixture must accept them.
	full := Outcome{Read: true, Entities: map[string]EntityState{
		"business_card":         {Name: "business_card", Fields: []string{"first_name", "last_name"}},
		"business_card_address": {Name: "business_card_address"},
		"business_card_contact": {Name: "business_card_contact"},
	}}
	fixture, _ := Scenarios().ByName("capture_request_builds_model_and_page")
	if result := fixture.ScoreOutcome(fixture.Score(finished), before, full); !result.OK() {
		t.Errorf("an already-built workspace should pass with no save_entity calls: %s", result)
	}
}

// Naming is a choice, not a failure: BusinessCard and business_card are the
// same entity, and a fixture that insisted on one spelling would be brittle.
func TestOutcomeMatchesOnNameOrDisplayName(t *testing.T) {
	outcome := Outcome{Read: true, Entities: map[string]EntityState{
		"BusinessCard": {Name: "BusinessCard", DisplayName: "Business Card"},
	}}
	for _, spelling := range []string{"business_card", "BusinessCard", "Business Card"} {
		if _, ok := outcome.Has(spelling); !ok {
			t.Errorf("%q should match", spelling)
		}
	}
	if _, ok := outcome.Has("invoices"); ok {
		t.Error("an unrelated entity must not match")
	}
}

// A reported failure that is really a success, and the reverse, both matter -
// this pins the direction we actually shipped wrong.
func TestReportingAFailureIsCaught(t *testing.T) {
	reported := run(answer(map[string]interface{}{
		"fields": []interface{}{map[string]interface{}{"action": "failed", "name": "test_scan"}},
	}),
		call(3, "save_field", map[string]interface{}{
			"field": map[string]interface{}{"storage_name": "sample_storage"},
		}),
	)
	fixture, _ := Scenarios().ByName("attachment_with_storage_writes")
	if fixture.Score(reported).OK() {
		t.Error("an answer reporting action=failed should not pass")
	}
}

// ─── the unprobed-query defect ──────────────────────────────────────────────

// Eleven components bound to eight queries, run_query never called, and a
// summary claiming the columns were confirmed by it.
func TestBindingQueriesWithoutProbingIsCaught(t *testing.T) {
	page := map[string]interface{}{"page": map[string]interface{}{"components": []interface{}{
		map[string]interface{}{"properties": map[string]interface{}{"base": map[string]interface{}{
			"query": "db_disb", "value_source": "query",
		}}},
		map[string]interface{}{"properties": map[string]interface{}{"base": map[string]interface{}{
			"query": "db_tiles_os", "data_source": "query",
		}}},
	}}}
	unprobed := run(answer(page), call(1, "get_component_spec", map[string]interface{}{}))

	// Exercised directly, not through the dashboard fixture: that fixture no
	// longer carries this assertion, because the page agent reports only some of
	// its tool calls and the check therefore fails runs that were correct. The
	// assertion itself is sound and stays covered here, ready for the day the
	// agent reports everything it does.
	reason := EveryBoundQueryWasProbed("run_query", "query_name").Check(unprobed)
	if reason == "" {
		t.Fatal("binding queries that were never run should fail")
	}
	if !strings.Contains(reason, "db_disb") || !strings.Contains(reason, "db_tiles_os") {
		t.Errorf("the failure should name every unprobed query: %s", reason)
	}
}

func TestProbingEveryQueryPasses(t *testing.T) {
	page := map[string]interface{}{"page": map[string]interface{}{"components": []interface{}{
		map[string]interface{}{"properties": map[string]interface{}{"base": map[string]interface{}{
			"query": "db_disb", "value_source": "query",
		}}},
	}}}
	probed := run(answer(page),
		call(1, "run_query", map[string]interface{}{"query_name": "db_disb"}),
		call(2, "save_page", map[string]interface{}{}),
	)
	if reason := EveryBoundQueryWasProbed("run_query", "query_name").Check(probed); reason != "" {
		t.Errorf("a run that probed first should pass: %s", reason)
	}
}

// Order matters: probing after the page is written is not probing.
func TestProbingAfterWritingIsCaught(t *testing.T) {
	late := run(answer(map[string]interface{}{}),
		call(1, "save_page", map[string]interface{}{}),
		call(2, "run_query", map[string]interface{}{"query_name": "db_disb"}),
	)
	if reason := CalledBefore("run_query", "save_page").Check(late); reason == "" {
		t.Error("a probe that happened after the write should be reported")
	}
}

// ─── the library itself ─────────────────────────────────────────────────────

func TestTrajectoryReadsToolCallsAndIterations(t *testing.T) {
	traj := run(nil,
		call(1, "get_processo_context", nil),
		call(3, "save_field", nil),
		call(3, "save_field", nil),
	)
	if got := len(traj.Calls("save_field")); got != 2 {
		t.Errorf("save_field calls = %d, want 2", got)
	}
	if traj.Iterations != 3 {
		t.Errorf("iterations = %d, want 3", traj.Iterations)
	}
	if got := strings.Join(traj.Names(), ","); got != "get_processo_context,save_field" {
		t.Errorf("names = %q", got)
	}
}

func TestBudgetIsAnAssertion(t *testing.T) {
	traj := run(nil, call(9, "save_field", nil))
	if reason := WithinIterations(4).Check(traj); reason == "" {
		t.Error("nine iterations against a budget of four should be reported")
	}
	if reason := WithinIterations(10).Check(traj); reason != "" {
		t.Errorf("inside budget should pass, got %q", reason)
	}
}

// Every scenario must be runnable and self-describing.
func TestScenariosAreWellFormed(t *testing.T) {
	for _, fixture := range Scenarios() {
		if fixture.Name == "" || fixture.Prompt == "" || len(fixture.Expect) == 0 {
			t.Errorf("incomplete fixture: %+v", fixture.Name)
		}
		if strings.TrimSpace(fixture.Why) == "" {
			t.Errorf("%s does not say what it protects against", fixture.Name)
		}
	}
}

// An entity that was already there cannot satisfy a topic: the run must have
// added it. Without this, "contact" matches the Contacts entity that has existed
// for years and the fixture reports success for work nobody did.
func TestCoversTopicsIgnoresWhatWasAlreadyThere(t *testing.T) {
	before := Outcome{Read: true, Entities: map[string]EntityState{
		"con": {Name: "con", DisplayName: "Contacts"},
	}}
	after := Outcome{Read: true, Entities: map[string]EntityState{
		"con":                   {Name: "con", DisplayName: "Contacts"},
		"business_card_address": {Name: "business_card_address"},
	}}
	reason := CoversTopics("address", "contact").Check(before, after)
	if reason == "" {
		t.Fatal("a pre-existing Contacts entity must not satisfy the contact topic")
	}
	// Only the contact topic is unmet; address was added and must not be listed
	// as missing. Checked on the "nothing added for ..." clause alone, because
	// the message also names what WAS added.
	unmet := strings.SplitN(strings.TrimPrefix(reason, "nothing added for "), " (", 2)[0]
	if unmet != "contact" {
		t.Errorf("unmet topics = %q, want just contact (full: %q)", unmet, reason)
	}
}
