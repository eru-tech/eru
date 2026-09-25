package eval

import (
	"errors"
	"strings"
	"testing"

	agents "github.com/eru-tech/eru/eru-ai/agents"
)

func pass(name string) Attempt {
	return Attempt{Result: Result{Fixture: name, Passed: 3}}
}

func fail(name, check, reason string) Attempt {
	return Attempt{Result: Result{
		Fixture:  name,
		Passed:   2,
		Failures: []Failure{{Expectation: check, Reason: reason}},
	}}
}

// Two out of three is not a pass. Rounding it up is how an intermittent defect
// becomes a permanent one.
func TestFlakyIsNotAPass(t *testing.T) {
	s := SampledResult{Fixture: "dash", Attempts: []Attempt{
		pass("dash"), fail("dash", "binds soundly", "tile_a unbound"), pass("dash"),
	}}
	if s.OK() {
		t.Fatal("a fixture that passed 2 of 3 must not report OK")
	}
	if got := s.String(); !strings.Contains(got, "FLAKY 2/3") {
		t.Errorf("the report must say it was flaky, got: %s", got)
	}
}

// A consistent failure and an intermittent one need different remediation, so
// they must not render the same way.
func TestConsistentFailureReadsDifferentlyFromFlaky(t *testing.T) {
	s := SampledResult{Fixture: "dash", Attempts: []Attempt{
		fail("dash", "binds soundly", "tile_a unbound"),
		fail("dash", "binds soundly", "tile_a unbound"),
	}}
	got := s.String()
	if !strings.Contains(got, "FAIL 0/2") {
		t.Errorf("a consistent failure must read as FAIL, got: %s", got)
	}
	if strings.Contains(got, "FLAKY") {
		t.Errorf("a consistent failure is not flaky: %s", got)
	}
}

func TestAllPassing(t *testing.T) {
	s := SampledResult{Fixture: "dash", Attempts: []Attempt{pass("dash"), pass("dash"), pass("dash")}}
	if !s.OK() {
		t.Fatal("three passes is a pass")
	}
	if got := s.String(); !strings.Contains(got, "PASS 3/3") {
		t.Errorf("got: %s", got)
	}
}

// An attempt that never ran is not the agent's failure. Excluded from the rate,
// reported so it cannot hide.
func TestInfraFailuresAreExcludedButReported(t *testing.T) {
	s := SampledResult{Fixture: "dash", Attempts: []Attempt{
		pass("dash"),
		{Infra: "dial tcp: connection refused"},
		pass("dash"),
	}}
	if !s.OK() {
		t.Fatal("two passes and one unreachable stack is not a failure of the agent")
	}
	if s.Scored() != 2 || s.InfraFailures() != 1 {
		t.Fatalf("scored=%d infra=%d, want 2 and 1", s.Scored(), s.InfraFailures())
	}
	if got := s.String(); !strings.Contains(got, "PASS 2/2") || !strings.Contains(got, "1 infra failure(s) excluded") {
		t.Errorf("the infra failure must be visible in the report, got: %s", got)
	}
}

// If nothing ran, nothing is known — and a suite that reports green because it
// never executed is the worst outcome available.
func TestEveryAttemptInfraIsBlockedNotGreen(t *testing.T) {
	s := SampledResult{Fixture: "dash", Attempts: []Attempt{
		{Infra: "dial tcp: connection refused"},
		{Infra: "dial tcp: connection refused"},
	}}
	got := s.String()
	if !strings.Contains(got, "BLOCKED") {
		t.Fatalf("a fixture with no scored attempt must be BLOCKED, got: %s", got)
	}
	if !strings.Contains(got, "connection refused") {
		t.Errorf("the reason must be carried: %s", got)
	}
	if rate, ok := s.Rate(); ok {
		t.Errorf("there is no rate to report, got %v", rate)
	}
}

// The scenario report has to name the check that wobbled, not just the fixture.
func TestTheFlakyCheckIsNamed(t *testing.T) {
	s := SampledResult{Fixture: "dash", Attempts: []Attempt{
		pass("dash"),
		fail("dash", "every component binds to data that can resolve", "tile_a unbound"),
		pass("dash"),
	}}
	got := s.String()
	if !strings.Contains(got, "every component binds to data that can resolve — failed 1/3") {
		t.Errorf("the per-check rate must appear, got: %s", got)
	}
}

// Infra classification: an unreachable stack is infrastructure; an agent that
// answered badly is not.
func TestInfraClassification(t *testing.T) {
	infra := []string{
		"dash: dial tcp 127.0.0.1:8088: connect: connection refused",
		"dash: agent answered 502",
		"dash: agent answered 401",
		"dash: decoding the reply: unexpected EOF",
	}
	for _, text := range infra {
		if infraFailure(errors.New(text)) == "" {
			t.Errorf("should be infrastructure: %s", text)
		}
	}
	notInfra := []string{
		"dash: agent answered 400",
		"dash: the reply carried no traces and no actions - was raw=true honoured?",
		"dash: context deadline exceeded",
	}
	for _, text := range notInfra {
		if reason := infraFailure(errors.New(text)); reason != "" {
			t.Errorf("should NOT be infrastructure: %s (got %q)", text, reason)
		}
	}
}

// A timeout is the agent failing its budget, and it is the failure most likely
// to be caused by the change under test — so it must never be excused as infra.
func TestATimeoutIsACapabilityFailure(t *testing.T) {
	if reason := infraFailure(errors.New("dash: Client.Timeout exceeded while awaiting headers")); reason != "" {
		t.Fatalf("a timeout must count against the agent, got %q", reason)
	}
}

func TestReportSampledFailsTheSuiteOnFlake(t *testing.T) {
	_, ok := ReportSampled([]SampledResult{
		{Fixture: "a", Attempts: []Attempt{pass("a"), pass("a")}},
		{Fixture: "b", Attempts: []Attempt{pass("b"), fail("b", "check", "why")}},
	})
	if ok {
		t.Fatal("a flaky fixture must turn the suite red")
	}
}

func TestBlockedDoesNotTurnTheSuiteRed(t *testing.T) {
	_, ok := ReportSampled([]SampledResult{
		{Fixture: "a", Attempts: []Attempt{pass("a")}},
		{Fixture: "b", Blocked: "the workspace already holds it"},
	})
	if !ok {
		t.Fatal("blocked is not failure - nothing is known to be wrong")
	}
}

// The number of attempts is a property of the scenario, with a suite default
// and a one-off override above it.
func TestSampleCountPrecedence(t *testing.T) {
	cheap := Fixture{Name: "cheap", Samples: 5}
	costly := Fixture{Name: "costly", Samples: 1}
	unset := Fixture{Name: "unset"}

	if got := cheap.SampleCount(3, 0); got != 5 {
		t.Errorf("the fixture's own count beats the suite default: got %d", got)
	}
	if got := costly.SampleCount(3, 0); got != 1 {
		t.Errorf("an expensive fixture must be able to ask for fewer: got %d", got)
	}
	if got := unset.SampleCount(3, 0); got != 3 {
		t.Errorf("unset falls back to the suite default: got %d", got)
	}
	if got := unset.SampleCount(0, 0); got != 1 {
		t.Errorf("with nothing set at all, one: got %d", got)
	}
	if got := cheap.SampleCount(3, 2); got != 2 {
		t.Errorf("an explicit override beats everything: got %d", got)
	}
}

// A scenario acting on an entity the workspace does not have is not a failure
// of the agent — and, on its paired negative, not a pass either.
func TestAMissingEntityBlocksRatherThanFails(t *testing.T) {
	empty := Outcome{Read: true, Entities: map[string]EntityState{}}
	f := Fixture{Name: "attachment", RequiresPresent: []string{"test"}}
	reason := f.CheckPreconditions(empty)
	if reason == "" {
		t.Fatal("a missing required entity must block the scenario")
	}
	if !strings.Contains(reason, "test") {
		t.Errorf("the missing entity must be named: %s", reason)
	}
}

func TestPresentEntityDoesNotBlock(t *testing.T) {
	have := Outcome{Read: true, Entities: map[string]EntityState{"test": {Name: "test"}}}
	f := Fixture{Name: "attachment", RequiresPresent: []string{"test"}}
	if reason := f.CheckPreconditions(have); reason != "" {
		t.Fatalf("the entity is there, nothing should block: %s", reason)
	}
}

// RequiresAbsent still works, and the two can be declared on one fixture.
func TestBothPreconditionsApply(t *testing.T) {
	have := Outcome{Read: true, Entities: map[string]EntityState{"business_card": {Name: "business_card"}}}
	f := Fixture{Name: "build", RequiresAbsent: []string{"business_card"}}
	if reason := f.CheckPreconditions(have); reason == "" {
		t.Fatal("an already-built workspace must block a build scenario")
	}
}

// One tool, many actions: the record must name what the model saw, or a
// per-action assertion has nothing to match. The processo tool carries
// save_entity, save_field and save_page, and recording all three as "processo"
// made every one of them unassertable.
func TestTheRecordNamesTheActionNotTheTool(t *testing.T) {
	reply := agents.AgentMessage{Metrics: &agents.ExecutionMetrics{
		ToolRecord: []agents.ToolInvocation{
			{Tool: "processo", Action: "save_field", Args: map[string]interface{}{
				"field": map[string]interface{}{"storage_name": "sample_storage"},
			}},
			{Tool: "get_entity_metadata", Action: "get_entity_metadata"},
		},
	}}
	traj := FromRun("prompt", reply)
	if got := traj.CallCount("save_field"); got != 1 {
		t.Fatalf("save_field must be findable by its action name, got %d", got)
	}
	if reason := CalledWith("save_field", "field.storage_name", "sample_storage").Check(traj); reason != "" {
		t.Errorf("the arguments must be reachable: %s", reason)
	}
	if got := traj.CallCount("get_entity_metadata"); got != 1 {
		t.Errorf("a tool whose action matches its name still resolves, got %d", got)
	}
}

// A write scenario must target something different each attempt, or attempt 2
// meets what attempt 1 created and tests a different thing.
func TestEachAttemptGetsItsOwnTarget(t *testing.T) {
	f := Fixture{Name: "w", Prompt: "add a field named test_scan_{{sample}} to test"}
	// Letters, not digits: a field name may not contain a digit, and substituting
	// the attempt number made every attempt fail on a name the agent refused.
	if got := f.PromptFor(1); got != "add a field named test_scan_a to test" {
		t.Errorf("attempt 1: %s", got)
	}
	if got := f.PromptFor(3); got != "add a field named test_scan_c to test" {
		t.Errorf("attempt 3: %s", got)
	}
	if got := f.PromptFor(27); got != "add a field named test_scan_aa to test" {
		t.Errorf("attempt 27 must keep going past z: %s", got)
	}
	if strings.ContainsAny(f.PromptFor(5), "0123456789") {
		t.Error("the substitution must never put a digit in a field name")
	}
	// Stable, not random: a second run reuses the same names rather than
	// accumulating a new field every time the suite is run.
	if f.PromptFor(3) != f.PromptFor(3) {
		t.Error("the substitution must be deterministic")
	}
}

func TestAPromptWithoutThePlaceholderIsUntouched(t *testing.T) {
	f := Fixture{Name: "r", Prompt: "list the entities"}
	if got := f.PromptFor(4); got != "list the entities" {
		t.Errorf("got %q", got)
	}
}

// "If you wrote, you wrote the right thing" must not require a write - that is
// the whole difference from CalledWith, and it is what lets the assertion
// survive a second run against a workspace that already holds the result.
func TestIfCalledWithPassesWhenTheToolWasNotCalled(t *testing.T) {
	quiet := Trajectory{ToolCalls: []ToolCall{{Name: "get_entity_metadata"}}}
	if reason := IfCalledWith("save_field", "field.storage_name", "sample_storage").Check(quiet); reason != "" {
		t.Errorf("no call means nothing to contradict: %s", reason)
	}
}

func TestIfCalledWithStillCatchesAWrongArgument(t *testing.T) {
	wrong := Trajectory{ToolCalls: []ToolCall{{
		Name:  "save_field",
		Input: map[string]interface{}{"field": map[string]interface{}{"storage_name": "default"}},
	}}}
	reason := IfCalledWith("save_field", "field.storage_name", "sample_storage").Check(wrong)
	if reason == "" {
		t.Fatal("an invented storage name must still be reported")
	}
	if !strings.Contains(reason, "default") {
		t.Errorf("what was actually sent must be named: %s", reason)
	}
}

// The per-attempt outcome assertion has to name the field that attempt wrote,
// not a fixed one - otherwise five attempts all check attempt 1's work.
func TestOutcomeExpectationsFollowTheAttempt(t *testing.T) {
	f := Fixture{
		Name: "w",
		ExpectOutcomeFor: func(sample string) []OutcomeExpectation {
			return []OutcomeExpectation{EntityFieldHas("test", "test_scan_"+sample, "datatype", "attachment")}
		},
	}
	first := f.OutcomeExpectations(1)
	third := f.OutcomeExpectations(3)
	if len(first) != 1 || len(third) != 1 {
		t.Fatalf("one expectation each, got %d and %d", len(first), len(third))
	}
	if !strings.Contains(first[0].Describe, "test_scan_a") {
		t.Errorf("attempt 1 must assert on test_scan_a: %s", first[0].Describe)
	}
	if !strings.Contains(third[0].Describe, "test_scan_c") {
		t.Errorf("attempt 3 must assert on test_scan_c: %s", third[0].Describe)
	}
}

// A field of the wrong datatype passes a names-only check while being useless,
// which is why the outcome reads the stored definition.
func TestEntityFieldHasReadsTheStoredDefinition(t *testing.T) {
	after := Outcome{Read: true, Entities: map[string]EntityState{"test": {
		Name:   "test",
		Fields: []string{"test_scan_a"},
		FieldAttrs: map[string]map[string]interface{}{
			"test_scan_a": {"datatype": "textbox", "storage_name": "sample_storage"},
		},
	}}}
	wrong := EntityFieldHas("test", "test_scan_a", "datatype", "attachment").Check(Outcome{}, after)
	if wrong == "" {
		t.Error("a textbox where an attachment was asked for must fail")
	}
	right := EntityFieldHas("test", "test_scan_a", "storage_name", "sample_storage").Check(Outcome{}, after)
	if right != "" {
		t.Errorf("the storage matches, so this must pass: %s", right)
	}
	missing := EntityFieldHas("test", "test_scan_z", "datatype", "attachment").Check(Outcome{}, after)
	if !strings.Contains(missing, "test_scan_z") {
		t.Errorf("a missing field must be named: %s", missing)
	}
}
