package ruleset

import (
	"encoding/json"
	"strings"
	"testing"
)

func invoices(total interface{}, amounts ...interface{}) map[string]interface{} {
	list := make([]interface{}, 0, len(amounts))
	for i, amount := range amounts {
		list = append(list, map[string]interface{}{
			"inv_no": string(rune('a' + i)), "inv_amount": amount,
		})
	}
	return map[string]interface{}{"inv_total": total, "inv": list}
}

func sumRule() Rule {
	return Rule{
		Kind: KindSumOf, Property: "inv_total", Over: "inv", Field: "inv_amount",
		Code:    "total_wrong",
		Message: "inv_total is {value} but the invoices listed add up to {sum}",
	}
}

// The error a financial answer never shows on its face: every invoice right,
// every amount right, and the total wrong.
func TestAWrongTotalIsCaught(t *testing.T) {
	out := sumRule().Check(subject("answer", invoices(500000.0, 371070.0, 139052.0)))
	if len(out) != 1 {
		t.Fatalf("510122 was stated as 500000: %+v", out)
	}
	if !strings.Contains(out[0].Message, "510122") {
		t.Errorf("the message must show the computed sum so the reader can see both: %s", out[0].Message)
	}
	if strings.Contains(out[0].Message, `"510122"`) {
		t.Errorf("the sum should read as a number, not a quoted string: %s", out[0].Message)
	}
}

func TestARightTotalPasses(t *testing.T) {
	if out := sumRule().Check(subject("answer", invoices(510122.0, 371070.0, 139052.0))); len(out) != 0 {
		t.Errorf("%+v", out)
	}
}

// Comparing currency with exact float equality accuses honest answers.
func TestFloatingPointSlackIsAllowedByDefault(t *testing.T) {
	// 0.1 + 0.2 is famously not 0.3 in binary floating point.
	s := Subject{Path: "a", Name: "answer", Properties: invoices(0.3, 0.1, 0.2)}
	if out := sumRule().Check(s); len(out) != 0 {
		t.Errorf("0.1 + 0.2 must not be reported as failing to equal 0.3: %+v", out)
	}
	// A real discrepancy is still caught.
	off := Subject{Path: "a", Name: "answer", Properties: invoices(0.5, 0.1, 0.2)}
	if out := sumRule().Check(off); len(out) != 1 {
		t.Errorf("a genuine difference must still be reported: %+v", out)
	}
}

func TestAnExplicitToleranceIsHonoured(t *testing.T) {
	// The invoices add up to 510122, so a stated 510000 is out by 122.
	rule := sumRule()

	loose := 200.0
	rule.Tolerance = &loose
	if out := rule.Check(subject("answer", invoices(510000.0, 371070.0, 139052.0))); len(out) != 0 {
		t.Errorf("122 is inside a tolerance of 200 and must be allowed: %+v", out)
	}

	tight := 100.0
	rule.Tolerance = &tight
	if out := rule.Check(subject("answer", invoices(510000.0, 371070.0, 139052.0))); len(out) != 1 {
		t.Errorf("122 is outside a tolerance of 100 and must be reported: %+v", out)
	}

	// Explicit zero demands exactness, which is available to anyone who wants it
	// on values that are not floating-point money.
	exact := 0.0
	rule.Tolerance = &exact
	if out := rule.Check(subject("answer", invoices(3.0, 1.0, 2.0))); len(out) != 0 {
		t.Errorf("whole numbers must still compare equal at zero tolerance: %+v", out)
	}
	if out := rule.Check(subject("answer", invoices(3.5, 1.0, 2.0))); len(out) != 1 {
		t.Errorf("zero tolerance must reject a real difference: %+v", out)
	}
}

// An empty list is a real claim - "no invoices, so nothing owed" - and can be
// checked. A path that resolves to nothing cannot.
func TestAnEmptyListSumsToZeroButAMissingPathIsNotJudged(t *testing.T) {
	empty := map[string]interface{}{"inv_total": 0.0, "inv": []interface{}{}}
	if out := sumRule().Check(subject("answer", empty)); len(out) != 0 {
		t.Errorf("an empty list sums to zero: %+v", out)
	}
	wrong := map[string]interface{}{"inv_total": 99.0, "inv": []interface{}{}}
	if out := sumRule().Check(subject("answer", wrong)); len(out) != 1 {
		t.Errorf("a total with no invoices behind it must be reported: %+v", out)
	}
	absent := map[string]interface{}{"inv_total": 99.0}
	if out := sumRule().Check(subject("answer", absent)); len(out) != 0 {
		t.Errorf("nothing to add up means nothing to disagree with: %+v", out)
	}
}

// Requiring the total at all is KindRequires' job; reporting it here too would
// show one fault as two.
func TestAnAbsentTotalIsNotThisRulesComplaint(t *testing.T) {
	noTotal := map[string]interface{}{"inv": []interface{}{map[string]interface{}{"inv_amount": 5.0}}}
	if out := sumRule().Check(subject("answer", noTotal)); len(out) != 0 {
		t.Errorf("%+v", out)
	}
}

// Amounts arrive as strings often enough that refusing them would be pedantry.
func TestAmountsWrittenAsStringsStillAddUp(t *testing.T) {
	if out := sumRule().Check(subject("answer", invoices("510122", "371070", "139052"))); len(out) != 0 {
		t.Errorf("%+v", out)
	}
}

func TestSumOverWalksNestedLists(t *testing.T) {
	rule := sumRule()
	rule.Over = "sections.lines"
	rule.Field = "amount"
	answer := map[string]interface{}{
		"inv_total": 60.0,
		"sections": []interface{}{
			map[string]interface{}{"lines": []interface{}{
				map[string]interface{}{"amount": 10.0}, map[string]interface{}{"amount": 20.0}}},
			map[string]interface{}{"lines": []interface{}{
				map[string]interface{}{"amount": 30.0}}},
		},
	}
	if out := rule.Check(subject("answer", answer)); len(out) != 0 {
		t.Errorf("a nested path must be walked: %+v", out)
	}
}

// The whole point: it arrives as configuration.
func TestSumOfRoundTripsThroughJSON(t *testing.T) {
	var rule Rule
	raw := []byte(`{"kind":"sum_of","property":"inv_total","over":"inv","field":"inv_amount",
	  "tolerance":0.5,"code":"total_wrong","message":"stated {value}, computed {sum}"}`)
	if err := json.Unmarshal(raw, &rule); err != nil {
		t.Fatal(err)
	}
	if rule.Kind != KindSumOf || rule.Over != "inv" || rule.Tolerance == nil || *rule.Tolerance != 0.5 {
		t.Fatalf("decoded wrong: %+v", rule)
	}
	out := rule.Check(subject("answer", invoices(1.0, 2.0, 3.0)))
	if len(out) != 1 || !strings.Contains(out[0].Message, "computed 5") {
		t.Errorf("a configured sum rule must enforce and report both figures: %+v", out)
	}
}
