package eru_studio

import (
	"encoding/json"
	"strings"
	"testing"

	catalog "github.com/eru-tech/eru/eru-ai/agents/eru_studio/catalog"
)

func layoutPage(t *testing.T, body string) map[string]interface{} {
	t.Helper()
	var page map[string]interface{}
	if err := json.Unmarshal([]byte(body), &page); err != nil {
		t.Fatalf("bad test page: %v", err)
	}
	return page
}

func codesOf(issues []catalog.Issue) []string {
	out := make([]string, 0, len(issues))
	for _, issue := range issues {
		out = append(out, string(issue.Code)+"@"+issue.ComponentId)
	}
	return out
}

// The shape this rule exists for: a two-pane body where the right pane grows and
// the left pane was given nothing, so it collapses to its text width. This is
// the page the user looked at and called a narrow layout.
func TestLayoutCatchesAPaneWithNothingToSizeIt(t *testing.T) {
	page := layoutPage(t, `{
	  "id":"p","name":"p","styles":{},
	  "components":[{"id":"main_body","type":"flex_container",
	    "properties":{"base":{"flex_direction":"row"}},"styles":{},
	    "children":[
	      {"id":"left_panel","type":"flex_container","properties":{"base":{}},"styles":{},"children":[]},
	      {"id":"right_panel","type":"flex_container","properties":{"base":{"flex_grow":1}},"styles":{},"children":[]}
	    ]}]}`)

	issues := LayoutIssues(page)
	if len(issues) != 1 {
		t.Fatalf("expected exactly the left pane to be flagged, got %v", codesOf(issues))
	}
	if issues[0].Code != catalog.CodeLayoutSqueezedPane || issues[0].ComponentId != "left_panel" {
		t.Errorf("wrong issue: %v", codesOf(issues))
	}
	if !strings.Contains(issues[0].Message, "applies to every child equally") {
		t.Errorf("the message should say why setting it on the parent does not work: %s", issues[0].Message)
	}
}

func TestLayoutAcceptsAPaneThatSetsItsOwnWidth(t *testing.T) {
	page := layoutPage(t, `{
	  "id":"p","name":"p","styles":{},
	  "components":[{"id":"main_body","type":"flex_container",
	    "properties":{"base":{"flex_direction":"row"}},"styles":{},
	    "children":[
	      {"id":"left_panel","type":"flex_container","properties":{"base":{}},
	       "styles":{"responsive_styles":{"base":{"width":"35%"}}},"children":[]},
	      {"id":"right_panel","type":"flex_container","properties":{"base":{"flex_grow":1}},"styles":{},"children":[]}
	    ]}]}`)
	if issues := LayoutIssues(page); len(issues) != 0 {
		t.Fatalf("a pane with an explicit width is fine, got %v", codesOf(issues))
	}
}

// A header is a row of things at their natural width. Nothing grows, so nothing
// is being squeezed, and flagging it would teach the model to ignore the rule.
func TestLayoutLeavesAHeaderRowAlone(t *testing.T) {
	page := layoutPage(t, `{
	  "id":"p","name":"p","styles":{},
	  "components":[{"id":"top_header","type":"flex_container",
	    "properties":{"base":{"flex_direction":"row"}},"styles":{},
	    "children":[
	      {"id":"brand","type":"flex_container","properties":{"base":{}},"styles":{},"children":[]},
	      {"id":"subtitle","type":"text","properties":{"base":{}},"styles":{}}
	    ]}]}`)
	if issues := LayoutIssues(page); len(issues) != 0 {
		t.Fatalf("nothing grows in a header row, got %v", codesOf(issues))
	}
}

// An icon and a label beside a spacer that grows is the standard section header.
// Only a pane can be squeezed in a way anyone would call a bug.
func TestLayoutIgnoresLeavesBesideAGrowingSpacer(t *testing.T) {
	page := layoutPage(t, `{
	  "id":"p","name":"p","styles":{},
	  "components":[{"id":"section_header","type":"flex_container",
	    "properties":{"base":{"flex_direction":"row"}},"styles":{},
	    "children":[
	      {"id":"icon1","type":"icon","properties":{"base":{}},"styles":{}},
	      {"id":"title1","type":"text","properties":{"base":{}},"styles":{}},
	      {"id":"rule1","type":"divider","properties":{"base":{}},"styles":{"classes":"flex-1"}}
	    ]}]}`)
	if issues := LayoutIssues(page); len(issues) != 0 {
		t.Fatalf("leaves next to a growing divider are not panes, got %v", codesOf(issues))
	}
}

func TestLayoutIgnoresAColumn(t *testing.T) {
	page := layoutPage(t, `{
	  "id":"p","name":"p","styles":{},
	  "components":[{"id":"stack","type":"flex_container",
	    "properties":{"base":{"flex_direction":"column"}},"styles":{},
	    "children":[
	      {"id":"a","type":"flex_container","properties":{"base":{}},"styles":{},"children":[]},
	      {"id":"b","type":"flex_container","properties":{"base":{"flex_grow":1}},"styles":{},"children":[]}
	    ]}]}`)
	if issues := LayoutIssues(page); len(issues) != 0 {
		t.Fatalf("a column stacks vertically, width is not in play, got %v", codesOf(issues))
	}
}

func TestLayoutCatchesWidthsThatOverflowTheRow(t *testing.T) {
	page := layoutPage(t, `{
	  "id":"p","name":"p","styles":{},
	  "components":[{"id":"row","type":"flex_container",
	    "properties":{"base":{"flex_direction":"row"}},"styles":{},
	    "children":[
	      {"id":"a","type":"flex_container","properties":{"base":{}},"styles":{"responsive_styles":{"base":{"width":"70%"}}},"children":[]},
	      {"id":"b","type":"flex_container","properties":{"base":{}},"styles":{"responsive_styles":{"base":{"width":"50%"}}},"children":[]}
	    ]}]}`)
	issues := LayoutIssues(page)
	if len(issues) != 1 || issues[0].Code != catalog.CodeLayoutWidthsOverflow {
		t.Fatalf("expected an overflow issue, got %v", codesOf(issues))
	}
}

func TestLayoutAllowsWidthsThatFit(t *testing.T) {
	page := layoutPage(t, `{
	  "id":"p","name":"p","styles":{},
	  "components":[{"id":"row","type":"flex_container",
	    "properties":{"base":{"flex_direction":"row"}},"styles":{},
	    "children":[
	      {"id":"a","type":"flex_container","properties":{"base":{}},"styles":{"responsive_styles":{"base":{"width":"35%"}}},"children":[]},
	      {"id":"b","type":"flex_container","properties":{"base":{}},"styles":{"responsive_styles":{"base":{"width":"65%"}}},"children":[]}
	    ]}]}`)
	if issues := LayoutIssues(page); len(issues) != 0 {
		t.Fatalf("35 + 65 fits, got %v", codesOf(issues))
	}
}

func TestLayoutCatchesAGridOfFixedColumns(t *testing.T) {
	page := layoutPage(t, `{
	  "id":"p","name":"p","styles":{},
	  "components":[{"id":"g","type":"grid_container",
	    "properties":{"base":{"grid_template_columns":"120px 120px"}},"styles":{},"children":[]}]}`)
	issues := LayoutIssues(page)
	if len(issues) != 1 || issues[0].Code != catalog.CodeLayoutFixedColumns {
		t.Fatalf("expected a fixed-columns issue, got %v", codesOf(issues))
	}
}

// The shape every real page uses.
func TestLayoutAllowsFractionalColumns(t *testing.T) {
	page := layoutPage(t, `{
	  "id":"p","name":"p","styles":{},
	  "components":[{"id":"g","type":"grid_container",
	    "properties":{"base":{"grid_template_columns":"1fr 1fr","gap":16,"collapse_below_width":480}},"styles":{},"children":[]}]}`)
	if issues := LayoutIssues(page); len(issues) != 0 {
		t.Fatalf("1fr 1fr is the right way to do it, got %v", codesOf(issues))
	}
}

// A runtime expression cannot be read statically, so the rule must stand down
// rather than guess.
func TestLayoutIgnoresExpressionDrivenColumns(t *testing.T) {
	page := layoutPage(t, `{
	  "id":"p","name":"p","styles":{},
	  "components":[{"id":"g","type":"grid_container",
	    "properties":{"base":{"grid_template_columns":"@app.cols"}},"styles":{},"children":[]}]}`)
	if issues := LayoutIssues(page); len(issues) != 0 {
		t.Fatalf("an expression is not checkable, got %v", codesOf(issues))
	}
}
