package eru_studio

import (
	"encoding/json"
	"strings"
	"testing"

	catalog "github.com/eru-tech/eru/eru-ai/agents/eru_studio/catalog"
)

func chromePage(t *testing.T, body string) map[string]interface{} {
	t.Helper()
	var page map[string]interface{}
	if err := json.Unmarshal([]byte(body), &page); err != nil {
		t.Fatalf("bad test page: %v", err)
	}
	return page
}

func chromeCodes(issues []catalog.Issue) []string {
	out := make([]string, 0, len(issues))
	for _, issue := range issues {
		out = append(out, string(issue.Code)+"@"+issue.ComponentId)
	}
	return out
}

// The shape the agent produced on six of nine pages.
func TestAGradientHeaderIsReported(t *testing.T) {
	page := chromePage(t, `{"id":"p","name":"p","styles":{},"components":[
	  {"id":"top_header","type":"flex_container","properties":{"base":{}},
	   "styles":{"responsive_styles":{"base":{"background":"linear-gradient(135deg, #4f46e5 0%, #7c3aed 100%)"}}},"children":[]}]}`)
	issues := ChromeStyleIssues(page, StyleIntent{})
	if len(issues) != 1 || issues[0].Code != catalog.CodeChromeGradient {
		t.Fatalf("expected a gradient issue, got %v", chromeCodes(issues))
	}
	if !strings.Contains(issues[0].Message, "var(--studio-") {
		t.Error("the message should name the tokens to use instead")
	}
}

func TestAGradientClassOnAHeaderIsReported(t *testing.T) {
	page := chromePage(t, `{"id":"p","name":"p","styles":{},"components":[
	  {"id":"hero_banner","type":"flex_container","properties":{"base":{}},
	   "styles":{"classes":"bg-gradient-to-r from-indigo-500 to-purple-600"},"children":[]}]}`)
	if issues := ChromeStyleIssues(page, StyleIntent{}); len(issues) != 1 {
		t.Fatalf("a tailwind gradient should be caught too, got %v", chromeCodes(issues))
	}
}

// A rule the user cannot override is a restriction, not a default.
func TestAskingForAGradientStandsTheRuleDown(t *testing.T) {
	page := chromePage(t, `{"id":"p","name":"p","styles":{},"components":[
	  {"id":"top_header","type":"flex_container","properties":{"base":{}},
	   "styles":{"responsive_styles":{"base":{"background":"linear-gradient(135deg, #4f46e5 0%, #7c3aed 100%)"}}},"children":[]}]}`)
	intent := DetectStyleIntent("build me a page with a gradient header please")
	if !intent.AllowGradient {
		t.Fatal("the request plainly asks for a gradient")
	}
	if issues := ChromeStyleIssues(page, intent); len(issues) != 0 {
		t.Fatalf("the user asked for it, got %v", chromeCodes(issues))
	}
}

func TestAHardCodedChromeBackgroundIsReported(t *testing.T) {
	page := chromePage(t, `{"id":"p","name":"p","styles":{},"components":[
	  {"id":"app_bar","type":"flex_container","properties":{"base":{}},
	   "styles":{"responsive_styles":{"base":{"background_color":"#1e293b"}}},"children":[]}]}`)
	issues := ChromeStyleIssues(page, StyleIntent{})
	if len(issues) != 1 || issues[0].Code != catalog.CodeChromeHardCodedColour {
		t.Fatalf("expected a frozen-colour issue, got %v", chromeCodes(issues))
	}
	if !strings.Contains(issues[0].Message, "#1e293b") {
		t.Error("the message should name the colour it is complaining about")
	}
}

func TestATokenChromeBackgroundIsAccepted(t *testing.T) {
	page := chromePage(t, `{"id":"p","name":"p","styles":{},"components":[
	  {"id":"top_header","type":"flex_container","properties":{"base":{}},
	   "styles":{"responsive_styles":{"base":{"background_color":"var(--studio-surface-container)"}}},"children":[]}]}`)
	if issues := ChromeStyleIssues(page, StyleIntent{}); len(issues) != 0 {
		t.Fatalf("a token background is the whole point, got %v", chromeCodes(issues))
	}
}

// A colour the user named is a colour they meant.
func TestANamedColourStandsTheRuleDown(t *testing.T) {
	page := chromePage(t, `{"id":"p","name":"p","styles":{},"components":[
	  {"id":"top_header","type":"flex_container","properties":{"base":{}},
	   "styles":{"responsive_styles":{"base":{"background_color":"#ff0000"}}},"children":[]}]}`)
	for _, prompt := range []string{"make the header #ff0000", "use our brand colours"} {
		if issues := ChromeStyleIssues(page, DetectStyleIntent(prompt)); len(issues) != 0 {
			t.Errorf("%q should allow a chosen colour, got %v", prompt, chromeCodes(issues))
		}
	}
}

// Only chrome. A status chip, a chart series or a card body keeping its own
// colour is none of this rule's business.
func TestNonChromeComponentsAreLeftAlone(t *testing.T) {
	page := chromePage(t, `{"id":"p","name":"p","styles":{},"components":[
	  {"id":"status_chip","type":"status","properties":{"base":{}},
	   "styles":{"responsive_styles":{"base":{"background_color":"#16a34a"}}}},
	  {"id":"body_panel","type":"flex_container","properties":{"base":{}},
	   "styles":{"responsive_styles":{"base":{"background_color":"#1e293b"}}},"children":[]}]}`)
	if issues := ChromeStyleIssues(page, StyleIntent{}); len(issues) != 0 {
		t.Fatalf("nothing here is chrome, got %v", chromeCodes(issues))
	}
}

func TestARuntimeExpressionIsNotAFrozenColour(t *testing.T) {
	page := chromePage(t, `{"id":"p","name":"p","styles":{},"components":[
	  {"id":"top_header","type":"flex_container","properties":{"base":{}},
	   "styles":{"responsive_styles":{"base":{"background_color":"@app.header_colour"}}},"children":[]}]}`)
	if issues := ChromeStyleIssues(page, StyleIntent{}); len(issues) != 0 {
		t.Fatalf("an expression cannot be judged, got %v", chromeCodes(issues))
	}
}

func TestIntentIsOnlyReadFromWhatWasAsked(t *testing.T) {
	if DetectStyleIntent("build a page listing invoices").AllowGradient {
		t.Error("nothing here asks for a gradient")
	}
	if DetectStyleIntent("build a page listing invoices").AllowOwnPalette {
		t.Error("nothing here names a colour")
	}
}

// The tinted square behind a header's icon is an ornament, not the header. It
// only carries "header" in its name because of where it sits.
func TestAnOrnamentInsideAHeaderIsNotChrome(t *testing.T) {
	page := chromePage(t, `{"id":"p","name":"p","styles":{},"components":[
	  {"id":"header_icon_bg","type":"flex_container","properties":{"base":{}},
	   "styles":{"responsive_styles":{"base":{"background":"linear-gradient(135deg, var(--studio-primary) 0%, var(--studio-tertiary) 100%)"}}},"children":[]},
	  {"id":"form_header_icon_wrap","type":"flex_container","properties":{"base":{}},
	   "styles":{"responsive_styles":{"base":{"background_color":"#ede9fe"}}},"children":[]}]}`)
	if issues := ChromeStyleIssues(page, StyleIntent{}); len(issues) != 0 {
		t.Fatalf("an icon chip is not page chrome, got %v", chromeCodes(issues))
	}
}

// A header is often a named wrapper with the decorated band inside it. The
// gradient that prompted this rule sat on an unnamed child of "summary_header",
// and slipped through when only the named component was checked.
func TestAGradientInsideANamedHeaderIsReported(t *testing.T) {
	page := chromePage(t, `{"id":"p","name":"p","styles":{},"components":[
	  {"id":"summary_header","type":"flex_container","properties":{"base":{}},"styles":{},"children":[
	    {"id":"hero_band_inner","type":"flex_container","properties":{"base":{}},
	     "styles":{"responsive_styles":{"base":{"background":"linear-gradient(135deg, var(--studio-primary) 0%, var(--studio-primary-container) 100%)"}}},"children":[]}
	  ]}]}`)
	issues := ChromeStyleIssues(page, StyleIntent{})
	if len(issues) != 1 || issues[0].ComponentId != "hero_band_inner" {
		t.Fatalf("the band inside the header should be caught, got %v", chromeCodes(issues))
	}
}

// Inheritance must not drag ornaments back in.
func TestAnOrnamentInsideANamedHeaderStaysExempt(t *testing.T) {
	page := chromePage(t, `{"id":"p","name":"p","styles":{},"components":[
	  {"id":"summary_header","type":"flex_container","properties":{"base":{}},"styles":{},"children":[
	    {"id":"icon_bg","type":"flex_container","properties":{"base":{}},
	     "styles":{"responsive_styles":{"base":{"background":"linear-gradient(135deg, #a 0%, #b 100%)"}}},"children":[]}
	  ]}]}`)
	if issues := ChromeStyleIssues(page, StyleIntent{}); len(issues) != 0 {
		t.Fatalf("an icon chip stays an ornament wherever it sits, got %v", chromeCodes(issues))
	}
}

// Content below the header is not chrome just because the page has one.
func TestASiblingOfTheHeaderIsNotChrome(t *testing.T) {
	page := chromePage(t, `{"id":"p","name":"p","styles":{},"components":[
	  {"id":"summary_header","type":"flex_container","properties":{"base":{}},"styles":{},"children":[]},
	  {"id":"body_panel","type":"flex_container","properties":{"base":{}},
	   "styles":{"responsive_styles":{"base":{"background_color":"#1e293b"}}},"children":[]}]}`)
	if issues := ChromeStyleIssues(page, StyleIntent{}); len(issues) != 0 {
		t.Fatalf("a sibling is not inside the header, got %v", chromeCodes(issues))
	}
}
