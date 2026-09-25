package reasoning_agents

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/eru-tech/eru/eru-ai/agents"
	"github.com/eru-tech/eru/eru-ai/agents/eru_studio"
	utility "github.com/eru-tech/eru/eru-ai/tools/utility"
)

// The four faults below all shipped to a user on one dashboard. Every one of
// them passed validation: the page was well formed, the properties were real,
// the queries had been probed. These tests are the record that each is now
// caught before the answer is handed over.

func component(id, componentType string, base map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"id": id, "type": componentType,
		"properties": map[string]interface{}{"base": base},
	}
}

func page(id string, components ...map[string]interface{}) map[string]interface{} {
	list := make([]interface{}, 0, len(components))
	for _, c := range components {
		list = append(list, c)
	}
	return map[string]interface{}{"id": id, "components": list}
}

func judge(t *testing.T, ctx context.Context, output map[string]interface{}) agents.QualityVerdict {
	t.Helper()
	verdict, err := (&EruStudioAgent{}).JudgeQuality(ctx, output)
	if err != nil {
		t.Fatalf("the gate must not error on a well-formed page: %v", err)
	}
	return verdict
}

func TestAGridWithRawColumnHeadingsIsSentBack(t *testing.T) {
	verdict := judge(t, context.Background(), page("dash",
		component("g1", "grid", map[string]interface{}{"data_source": "query", "query": "db_avg_bal"}),
	))
	if verdict.Rating != agents.QualityPoor {
		t.Fatalf("rating = %q, want poor", verdict.Rating)
	}
	if !strings.Contains(verdict.Reason, "column_overrides") {
		t.Errorf("the reason must name the fix: %s", verdict.Reason)
	}
}

func TestALabelledGridPasses(t *testing.T) {
	verdict := judge(t, context.Background(), page("dash",
		component("g1", "grid", map[string]interface{}{
			"data_source": "query", "query": "db_avg_bal",
			"column_overrides": map[string]interface{}{"os_amt": map[string]interface{}{"label": "Outstanding"}},
		}),
	))
	if verdict.Rating != agents.QualityGood {
		t.Errorf("rating = %q (%s), want good", verdict.Rating, verdict.Reason)
	}
}

func TestATileWithNothingBehindItIsSentBack(t *testing.T) {
	verdict := judge(t, context.Background(), page("dash",
		component("filler", "tile", map[string]interface{}{"title": "Portfolio"}),
	))
	if verdict.Rating != agents.QualityPoor {
		t.Fatalf("an unbound tile must be caught: %+v", verdict)
	}
	if !strings.Contains(verdict.Reason, "filler") {
		t.Errorf("the reason must say which tile: %s", verdict.Reason)
	}
}

func TestAnUntitledChartIsSentBack(t *testing.T) {
	for _, chart := range []string{"bar_chart", "line_chart", "pie_chart"} {
		verdict := judge(t, context.Background(), page("dash",
			component("c1", chart, map[string]interface{}{"query": "db_disb"}),
		))
		if verdict.Rating != agents.QualityPoor {
			t.Errorf("%s with no title must be caught: %+v", chart, verdict)
		}
	}
}

func TestATitledChartPasses(t *testing.T) {
	verdict := judge(t, context.Background(), page("dash",
		component("c1", "bar_chart", map[string]interface{}{"query": "db_disb", "title": "Disbursement by month"}),
	))
	if verdict.Rating != agents.QualityGood {
		t.Errorf("rating = %q (%s), want good", verdict.Rating, verdict.Reason)
	}
}

// secondary_is_currency defaults to TRUE, so the page that never mentions it is
// the page that shows a rupee symbol in front of a count of loans.
func TestACurrencySymbolOnACountIsSentBack(t *testing.T) {
	verdict := judge(t, context.Background(), page("dash",
		component("kpi", "tile", map[string]interface{}{
			"data_source": "query", "query": "db_tiles",
			"primary_value_field": "os_amt", "currency_symbol": "GBP",
			"secondary_value_field": "loan_cnt",
		}),
	))
	if verdict.Rating != agents.QualityPoor {
		t.Fatalf("money formatting on a count must be caught: %+v", verdict)
	}
	if !strings.Contains(verdict.Reason, "secondary_is_currency") {
		t.Errorf("the reason must name the property to turn off: %s", verdict.Reason)
	}
}

func TestTurningItOffExplicitlyPasses(t *testing.T) {
	verdict := judge(t, context.Background(), page("dash",
		component("kpi", "tile", map[string]interface{}{
			"data_source": "query", "query": "db_tiles",
			"primary_value_field": "os_amt", "currency_symbol": "GBP",
			"secondary_value_field": "loan_cnt", "secondary_is_currency": false,
		}),
	))
	if verdict.Rating != agents.QualityGood {
		t.Errorf("rating = %q (%s), want good", verdict.Rating, verdict.Reason)
	}
}

// An amount beside an amount is exactly what the currency symbol is for.
func TestTwoAmountsAreNotAFault(t *testing.T) {
	verdict := judge(t, context.Background(), page("dash",
		component("kpi", "tile", map[string]interface{}{
			"data_source": "query", "query": "db_tiles",
			"primary_value_field": "os_amt", "currency_symbol": "GBP",
			"secondary_value_field": "sanctioned_amt",
		}),
	))
	if verdict.Rating != agents.QualityGood {
		t.Errorf("rating = %q (%s), want good", verdict.Rating, verdict.Reason)
	}
}

// Asked to rename one tile, the agent must not be sent back to relabel twelve
// grids it was never asked about - it would spend its one attempt on work
// nobody requested and lose the rename in the diff.
func TestAnEditIsNotSentBackForFaultsItInherited(t *testing.T) {
	existing := page("dash",
		component("g1", "grid", map[string]interface{}{"data_source": "query", "query": "db_avg_bal"}),
	)
	ctx := eru_studio.WithBasePage(context.Background(), existing)

	edited := page("dash",
		component("g1", "grid", map[string]interface{}{"data_source": "query", "query": "db_avg_bal"}),
		component("t1", "tile", map[string]interface{}{"primary_value_field": "os_amt", "title": "Outstanding"}),
	)
	if verdict := judge(t, ctx, edited); verdict.Rating != agents.QualityGood {
		t.Errorf("an inherited fault must be forgiven: %q %s", verdict.Rating, verdict.Reason)
	}
}

// But what the edit ADDS is its own, and forgiveness does not extend to it.
func TestAnEditIsStillSentBackForWhatItIntroduces(t *testing.T) {
	existing := page("dash",
		component("g1", "grid", map[string]interface{}{"data_source": "query", "query": "db_avg_bal"}),
	)
	ctx := eru_studio.WithBasePage(context.Background(), existing)

	edited := page("dash",
		component("g1", "grid", map[string]interface{}{"data_source": "query", "query": "db_avg_bal"}),
		component("g2", "grid", map[string]interface{}{"data_source": "query", "query": "db_disb"}),
	)
	verdict := judge(t, ctx, edited)
	if verdict.Rating != agents.QualityPoor {
		t.Fatalf("the newly added grid is the agent's own work: %+v", verdict)
	}
	if strings.Contains(verdict.Reason, `"g1"`) {
		t.Errorf("the inherited grid must not be mentioned: %s", verdict.Reason)
	}
	if !strings.Contains(verdict.Reason, `"g2"`) {
		t.Errorf("the introduced one must be: %s", verdict.Reason)
	}
}

// One wrong idea applied to six grids is one instruction, not six.
func TestFindingsAreGroupedByFault(t *testing.T) {
	var components []map[string]interface{}
	for _, id := range []string{"a", "b", "c"} {
		components = append(components, component(id, "grid", map[string]interface{}{
			"data_source": "query", "query": "q_" + id,
		}))
	}
	verdict := judge(t, context.Background(), page("dash", components...))
	if got := strings.Count(verdict.Reason, "column_overrides to label them"); got != 1 {
		t.Errorf("the instruction should appear once, appeared %d times:\n%s", got, verdict.Reason)
	}
	for _, id := range []string{`"a"`, `"b"`, `"c"`} {
		if !strings.Contains(verdict.Reason, id) {
			t.Errorf("every offending component must still be listed, %s missing:\n%s", id, verdict.Reason)
		}
	}
}

// Nothing to judge is not a poor verdict. A page the gate cannot read must ship
// unchanged rather than be sent back for a fault nobody can name.
func TestAnEmptyAnswerIsUnrated(t *testing.T) {
	if verdict := judge(t, context.Background(), nil); verdict.Rating != agents.QualityUnrated {
		t.Errorf("rating = %q, want unrated", verdict.Rating)
	}
	if verdict := judge(t, context.Background(), map[string]interface{}{}); verdict.Rating != agents.QualityUnrated {
		t.Errorf("an empty object is nothing to judge, got %q", verdict.Rating)
	}
}

// A card page is a page the user looks at, so it is held to the same bar.
func TestANestedPageIsJudgedToo(t *testing.T) {
	output := map[string]interface{}{
		"id": "dash",
		"components": []interface{}{
			component("g1", "grid", map[string]interface{}{
				"data_source": "query", "query": "q", "view_mode": "board", "card_page_id": "card",
				"column_overrides": map[string]interface{}{"x": map[string]interface{}{"label": "X"}},
			}),
		},
		"pages": []interface{}{
			map[string]interface{}{
				"page":       page("card", component("filler", "tile", map[string]interface{}{"title": "Nothing"})),
				"mounted_at": "g1",
			},
		},
	}
	verdict := judge(t, context.Background(), output)
	if verdict.Rating != agents.QualityPoor {
		t.Fatalf("a fault on the card page must be caught: %+v", verdict)
	}
	if !strings.Contains(verdict.Reason, `page "card"`) {
		t.Errorf("the reason must say which page: %s", verdict.Reason)
	}
}

// Improving a correct page by rewriting it whole is a large bet on a small
// change. Two live runs lost that bet the same way: asked to title one chart,
// the agent regenerated everything and the rewrite dropped required keys,
// invented properties and finally unbound every query.

func qualityVerdict(reason string) agents.QualityVerdict {
	return agents.QualityVerdict{Rating: agents.QualityPoor, Reason: reason}
}

func TestAQualityVerdictIsAnsweredWithAPatch(t *testing.T) {
	existing := page("dash", component("t1", "text", map[string]interface{}{}))
	ctx := eru_studio.WithOutputMode(context.Background(), eru_studio.ModePatch)
	ctx = eru_studio.WithBasePage(ctx, existing)
	ctx = eru_studio.WithRepairState(ctx, eru_studio.NewRepairState())

	accepted := map[string]interface{}{
		"page": page("dash", component("c1", "bar_chart", map[string]interface{}{"query": "q"})),
	}
	turn, ok := (&EruStudioAgent{}).QualityRepairTurn(ctx, accepted, qualityVerdict("the chart has no title"))
	if !ok {
		t.Fatal("patch mode with a readable page is exactly when a diff is possible")
	}
	if turn.Schema.Type == "" {
		t.Error("the turn must carry the patch schema, or the model answers in the wrong shape")
	}
	if !strings.Contains(turn.Prompt, "the chart has no title") {
		t.Errorf("the verdict must reach the model: %s", turn.Prompt)
	}
}

// The single most important difference from RepairTurn. A model told its
// correct page was refused goes looking for what is wrong with it, and what it
// finds is a rewrite.
func TestTheQualityPatchPromptDoesNotSayThePageWasRejected(t *testing.T) {
	prompt := qualityRepairPrompt(qualityVerdict("no chart title"), 0)
	// The conformance repair opens "Your page was rejected." This one must not,
	// and must not carry any other claim that something was wrong with it.
	for _, forbidden := range []string{"was rejected", "Your page was rejected", "was wrong", "failed", "is invalid"} {
		if strings.Contains(prompt, forbidden) {
			t.Errorf("the prompt must not suggest the page was refused, found %q:\n%s", forbidden, prompt)
		}
	}
	if strings.Contains(repairPrompt(nil, errors.New("x"), 0), "was rejected") == false {
		t.Error("guard assumption broken: the conformance repair prompt no longer says the page was rejected, so this test is comparing against nothing")
	}
	for _, want := range []string{"VALID and has been accepted", "nothing has been sent back", "ONLY the change", "ONE attempt"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the prompt must state %q:\n%s", want, prompt)
		}
	}
}

// Declining is always safe: the loop falls back to asking for the whole answer.
func TestTheQualityPatchDeclinesWhereADiffHasNoMeaning(t *testing.T) {
	accepted := map[string]interface{}{"page": page("dash", component("c1", "bar_chart", nil))}
	agent := &EruStudioAgent{}

	// Bare-page mode has no patch vocabulary.
	bare := eru_studio.WithRepairState(context.Background(), eru_studio.NewRepairState())
	if _, ok := agent.QualityRepairTurn(bare, accepted, qualityVerdict("r")); ok {
		t.Error("bare mode cannot answer with a patch")
	}

	// A scope is a promise about the user's page; re-basing it onto our own
	// answer would quietly change what it means.
	scoped := eru_studio.WithOutputMode(context.Background(), eru_studio.ModePatch)
	scoped = eru_studio.WithRepairState(scoped, eru_studio.NewRepairState())
	scoped = eru_studio.WithScope(scoped, &eru_studio.ResolvedScope{})
	if _, ok := agent.QualityRepairTurn(scoped, accepted, qualityVerdict("r")); ok {
		t.Error("a scoped edit must not be re-based")
	}
}

// Once the repair is running, the base IS the page the gate just objected to.
// Forgiving what was wrong there forgives exactly what the repair was called to
// fix, and the gate would record a fix that never happened.
func TestTheSecondJudgementIsNotForgivenAgainstTheRejectedPage(t *testing.T) {
	usersPage := page("dash", component("t1", "text", map[string]interface{}{}))
	rejected := page("dash",
		component("t1", "text", map[string]interface{}{}),
		component("c1", "bar_chart", map[string]interface{}{"query": "q"}),
	)

	ctx := eru_studio.WithOutputMode(context.Background(), eru_studio.ModePatch)
	ctx = eru_studio.WithBasePage(ctx, usersPage)
	state := eru_studio.NewRepairState()
	ctx = eru_studio.WithRepairState(ctx, state)
	state.Begin(rejected, nil)

	// The "improved" answer did not actually add the title.
	verdict := judge(t, ctx, map[string]interface{}{"page": rejected})
	if verdict.Rating != agents.QualityPoor {
		t.Fatalf("an unchanged page must still be rated poor, got %q", verdict.Rating)
	}
}

// A page built by the planner is a page the user sees, and was being held to a
// weaker standard: the compose path ran the catalog and entity checks only, so
// every query rule was skipped for it.
func TestAPlannedPageIsHeldToTheQueryRulesToo(t *testing.T) {
	ledger := eru_studio.NewLedger()
	ledger.Offer(utility.RunQueryToolName)
	ledger.RecordMissingQuery("db_not_there")
	ctx := eru_studio.WithLedger(context.Background(), ledger)

	built := page("card", component("c1", "line_chart", map[string]interface{}{
		"query": "db_not_there", "title": "Trend",
	}))

	issues := unprobedQueryIssues(ctx, built, nil)
	if len(issues) == 0 {
		t.Fatal("a nested page binding a query that does not exist must be reported")
	}
	if issues[0].Code != "query_missing" {
		t.Errorf("code = %q", issues[0].Code)
	}
}
