package ruleset

import (
	"encoding/json"
	"strings"
	"testing"
)

func f64(v float64) *float64 { return &v }

// The whole point of the package, finally reachable: an agent declares this in
// configuration and gets a working correction loop without Go.
func TestAWholeRuleSetArrivesAsConfiguration(t *testing.T) {
	raw := []byte(`{
      "subjects": "invoice.line_items",
      "name_key": "description",
      "rules": [
        {"property": "description", "code": "line_unlabelled",
         "message": "{subject} has no {property}",
         "guidance": "Every line item needs a description."},
        {"kind": "range", "property": "quantity", "min": 1,
         "code": "bad_quantity", "message": "{subject} has quantity {value}, which must be {field}"},
        {"kind": "one_of", "property": "unit", "allowed": ["hour", "day", "item"],
         "code": "bad_unit", "message": "{subject} has unit {value}; allowed: {field}"}
      ]
    }`)
	var set RuleSet
	if err := json.Unmarshal(raw, &set); err != nil {
		t.Fatal(err)
	}

	answer := map[string]interface{}{
		"invoice": map[string]interface{}{
			"line_items": []interface{}{
				map[string]interface{}{"description": "Consulting", "quantity": 3.0, "unit": "day"},
				map[string]interface{}{"quantity": 0.0, "unit": "fortnight"},
			},
		},
	}
	findings := set.Check(answer)
	codes := map[string]bool{}
	for _, finding := range findings {
		codes[finding.Code] = true
	}
	for _, want := range []string{"line_unlabelled", "bad_quantity", "bad_unit"} {
		if !codes[want] {
			t.Errorf("expected %s among %v", want, findings)
		}
	}
	if len(findings) != 3 {
		t.Errorf("the sound line item must raise nothing; got %d findings: %v", len(findings), findings)
	}
}

func TestSubjectsWalkListsWithoutBracketSyntax(t *testing.T) {
	set := RuleSet{Subjects: "sections.fields", Rules: []Rule{
		{Property: "label", Code: "c", Message: "{subject} needs a label"},
	}}
	answer := map[string]interface{}{
		"sections": []interface{}{
			map[string]interface{}{"fields": []interface{}{
				map[string]interface{}{"id": "a", "label": "Name"},
				map[string]interface{}{"id": "b"},
			}},
			map[string]interface{}{"fields": []interface{}{
				map[string]interface{}{"id": "c"},
			}},
		},
	}
	findings := set.Check(answer)
	if len(findings) != 2 {
		t.Fatalf("both unlabelled fields must be found: %v", findings)
	}
	if !strings.Contains(findings[0].Path, "sections[0].fields[1]") {
		t.Errorf("the path must locate the offender: %s", findings[0].Path)
	}
	if !strings.Contains(findings[0].Message, `"b"`) && !strings.Contains(findings[0].Message, "b ") {
		t.Errorf("{subject} should fall back to id: %s", findings[0].Message)
	}
}

// An agent that did not produce the section a rule is about has not broken that
// rule. Requiring the section is a separate statement, on the parent.
func TestAMissingSectionYieldsNoSubjects(t *testing.T) {
	set := RuleSet{Subjects: "invoice.line_items", Rules: []Rule{
		{Property: "x", Code: "c", Message: "m"},
	}}
	if findings := set.Check(map[string]interface{}{"invoice": map[string]interface{}{}}); len(findings) != 0 {
		t.Errorf("no subjects means no findings: %v", findings)
	}
	if findings := set.Check(map[string]interface{}{}); len(findings) != 0 {
		t.Errorf("an absent parent must not fire either: %v", findings)
	}
}

func TestAnEmptyPathJudgesTheAnswerItself(t *testing.T) {
	set := RuleSet{Rules: []Rule{{Property: "summary", Code: "c", Message: "the answer needs a {property}"}}}
	if findings := set.Check(map[string]interface{}{"other": 1}); len(findings) != 1 {
		t.Fatalf("the whole answer is the subject: %v", findings)
	}
	if findings := set.Check(map[string]interface{}{"summary": "done"}); len(findings) != 0 {
		t.Errorf("and it can be satisfied: %v", findings)
	}
}

func TestOneOf(t *testing.T) {
	rule := Rule{Kind: KindOneOf, Property: "status", Allowed: []string{"draft", "sent", "paid"},
		Code: "c", Message: "{value} is not one of {field}"}
	if out := rule.Check(subject("invoice", map[string]interface{}{"status": "SENT"})); len(out) != 0 {
		t.Errorf("case must not matter: %v", out)
	}
	out := rule.Check(subject("invoice", map[string]interface{}{"status": "cancelled"}))
	if len(out) != 1 {
		t.Fatalf("an unknown value must be reported: %v", out)
	}
	if !strings.Contains(out[0].Message, "draft, sent, paid") {
		t.Errorf("the message must list what IS allowed, or the model guesses again: %s", out[0].Message)
	}
	// Absent is KindRequires' business, not this rule's.
	if out := rule.Check(subject("invoice", map[string]interface{}{})); len(out) != 0 {
		t.Errorf("an absent property is not a wrong one: %v", out)
	}
}

func TestRange(t *testing.T) {
	rule := Rule{Kind: KindRange, Property: "quantity", Min: f64(1), Max: f64(100),
		Code: "c", Message: "{value} must be {field}"}
	for _, ok := range []interface{}{1.0, 50.0, 100.0, "42"} {
		if out := rule.Check(subject("line", map[string]interface{}{"quantity": ok})); len(out) != 0 {
			t.Errorf("%v should pass: %v", ok, out)
		}
	}
	for _, bad := range []interface{}{0.0, 101.0, -5.0} {
		if out := rule.Check(subject("line", map[string]interface{}{"quantity": bad})); len(out) != 1 {
			t.Errorf("%v should fail: %v", bad, out)
		}
	}
	// One open end.
	atLeast := Rule{Kind: KindRange, Property: "q", Min: f64(0), Code: "c", Message: "m"}
	if out := atLeast.Check(subject("line", map[string]interface{}{"q": 99999.0})); len(out) != 0 {
		t.Errorf("no maximum means no upper bound: %v", out)
	}
	// A non-number is the schema's complaint, not this rule's.
	if out := rule.Check(subject("line", map[string]interface{}{"quantity": "many"})); len(out) != 0 {
		t.Errorf("a type error is not a range error: %v", out)
	}
}

func TestAgreesWith(t *testing.T) {
	rule := Rule{Kind: KindAgreesWith, Property: "line_currency", Field: "invoice_currency",
		Code: "c", Message: "{property} is {value} but the invoice is in {field}"}
	if out := rule.Check(subject("line", map[string]interface{}{
		"line_currency": "GBP", "invoice_currency": "GBP"})); len(out) != 0 {
		t.Errorf("agreement passes: %v", out)
	}
	out := rule.Check(subject("line", map[string]interface{}{
		"line_currency": "USD", "invoice_currency": "GBP"}))
	if len(out) != 1 {
		t.Fatalf("disagreement must be reported: %v", out)
	}
	if !strings.Contains(out[0].Message, "GBP") {
		t.Errorf("the message must name what it should agree with: %s", out[0].Message)
	}
	// Nothing to disagree with.
	if out := rule.Check(subject("line", map[string]interface{}{"line_currency": "USD"})); len(out) != 0 {
		t.Errorf("an absent counterpart is not a conflict: %v", out)
	}
}

// A quality rule in config must reach the gate and NOT the validator.
func TestSeveritySplitsAConfiguredSet(t *testing.T) {
	sets := []RuleSet{{Rules: []Rule{
		{Property: "total", Code: "hard", Message: "m"},
		{Property: "notes", Code: "soft", Message: "m", Severity: SeverityQuality},
	}}}
	answer := map[string]interface{}{}
	if hard := CheckSets(sets, answer, SeverityError); len(hard) != 1 || hard[0].Code != "hard" {
		t.Errorf("the validator must see only the error rule: %v", hard)
	}
	if soft := CheckSets(sets, answer, SeverityQuality); len(soft) != 1 || soft[0].Code != "soft" {
		t.Errorf("the gate must see only the quality rule: %v", soft)
	}
}

func TestPromptForSetsStatesOnlyItsOwnSeverity(t *testing.T) {
	sets := []RuleSet{{Rules: []Rule{
		{Code: "a", Message: "m", Guidance: "Totals must be positive."},
		{Code: "b", Message: "m", Severity: SeverityQuality, Guidance: "Notes read better in full sentences."},
	}}}
	hard := PromptForSets(sets, "RULES", SeverityError)
	if !strings.Contains(hard, "Totals must be positive.") || strings.Contains(hard, "full sentences") {
		t.Errorf("the error prompt must carry only error guidance:\n%s", hard)
	}
	soft := PromptForSets(sets, "QUALITY", SeverityQuality)
	if !strings.Contains(soft, "full sentences") {
		t.Errorf("the quality prompt must carry quality guidance:\n%s", soft)
	}
}
