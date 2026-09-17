package eru_studio

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	catalog "github.com/eru-tech/eru/eru-ai/agents/eru_studio/catalog"
)

// A page's chrome - its header bar, its hero strip, the bar across the top of a
// panel - is where the agent reached for decoration, and it reached for the same
// decoration every time: linear-gradient(135deg, ...) across two hard-coded
// colours. Six of nine saved pages had one, three carried no theme token at all.
//
// Two things are wrong with that. A gradient header is a house style nobody
// asked for, and a hard-coded pair of colours is frozen: it ignores the host
// app's theme and looks wrong the moment the theme or dark mode changes. The
// tokens exist precisely so page chrome follows the app it lives in.
//
// So chrome gets a solid token background unless there is a reason not to, and
// "a reason" means the user asked - which is why this stands down when they do.

var chromeNamePattern = regexp.MustCompile(`(?i)(header|hero|banner|toolbar|topbar|title_?bar|app_?bar)`)

// A small chip inside a header - the tinted square behind an icon - is an
// ornament, not the surface framing the page. It carries "header" in its name
// only because of where it sits, so excluding it keeps the rule about the thing
// the user actually sees as the header.
var chromeOrnamentPattern = regexp.MustCompile(`(?i)(icon_?bg|icon_?wrap|icon_?box|icon_?chip|avatar|badge|dot|accent_?bar)`)

var gradientPattern = regexp.MustCompile(`(?i)(linear|radial|conic)-gradient\s*\(`)

var gradientClassPattern = regexp.MustCompile(`(?i)\bbg-gradient-to-|\bbg-\[(linear|radial|conic)-gradient`)

var hexPattern = regexp.MustCompile(`#[0-9a-fA-F]{3,8}\b`)

// StyleIntent is what the user asked for that the style rules should respect.
//
// A rule that cannot be overridden by the person whose page it is stops being a
// default and becomes a restriction. These are read from the request, once, so
// "give it a gradient header" is honoured rather than argued with.
type StyleIntent struct {
	// AllowGradient is set when the user asked for a gradient.
	AllowGradient bool
	// AllowOwnPalette is set when the user named colours of their own - a hex,
	// a brand, a specific shade - so a hard-coded colour is the point.
	AllowOwnPalette bool
}

var paletteWords = regexp.MustCompile(`(?i)\b(brand|palette|colou?r[- ]?scheme|corporate colou?rs?|house style)\b`)

// DetectStyleIntent reads the request for the things that make a hard-coded
// colour or a gradient the right answer.
func DetectStyleIntent(content string) StyleIntent {
	if strings.TrimSpace(content) == "" {
		return StyleIntent{}
	}
	intent := StyleIntent{}
	if strings.Contains(strings.ToLower(content), "gradient") {
		intent.AllowGradient = true
	}
	if hexPattern.MatchString(content) || paletteWords.MatchString(content) {
		intent.AllowOwnPalette = true
	}
	// Asking for a gradient is asking for the colours in it.
	if intent.AllowGradient {
		intent.AllowOwnPalette = true
	}
	return intent
}

type styleIntentKey struct{}

func WithStyleIntent(ctx context.Context, intent StyleIntent) context.Context {
	return context.WithValue(ctx, styleIntentKey{}, intent)
}

func StyleIntentFrom(ctx context.Context) StyleIntent {
	if intent, ok := ctx.Value(styleIntentKey{}).(StyleIntent); ok {
		return intent
	}
	return StyleIntent{}
}

// ChromeStyleIssues checks the page's chrome against the two defaults: a solid
// background, and a theme token rather than a frozen colour.
func ChromeStyleIssues(page map[string]interface{}, intent StyleIntent) []catalog.Issue {
	issues := []catalog.Issue{}
	if len(page) == 0 || (intent.AllowGradient && intent.AllowOwnPalette) {
		return issues
	}

	// A header is often a named wrapper with the decorated band inside it, so
	// chrome-ness is inherited: the gradient that prompted this rule sat on an
	// unnamed child of "summary_header", not on the header itself.
	chromeIds := map[string]bool{}
	WalkPage(page, func(component map[string]interface{}, parentId string) {
		if isChromeNamed(component) || (parentId != "" && chromeIds[parentId] && !isOrnament(component)) {
			if id, _ := component["id"].(string); id != "" {
				chromeIds[id] = true
			}
		}
	})

	WalkPage(page, func(component map[string]interface{}, _ string) {
		if !isChrome(component, chromeIds) {
			return
		}
		id, _ := component["id"].(string)
		background := backgroundValues(component)

		if !intent.AllowGradient && hasGradient(background, classList(component)) {
			issues = append(issues, catalog.Issue{
				Path:        fmt.Sprintf("components (id %q)", id),
				Code:        catalog.CodeChromeGradient,
				ComponentId: id,
				Message: "this is page chrome and its background is a gradient. Use a solid background from a theme token instead " +
					"- var(--studio-primary), var(--studio-surface-container) or var(--studio-surface-variant) - so the header follows the " +
					"host app's theme. A gradient here is decoration nobody asked for, and it freezes two colours against a theme that moves",
			})
			return
		}

		if !intent.AllowOwnPalette {
			if frozen := firstHardCodedColour(background); frozen != "" {
				issues = append(issues, catalog.Issue{
					Path:        fmt.Sprintf("components (id %q)", id),
					Code:        catalog.CodeChromeHardCodedColour,
					ComponentId: id,
					Message: fmt.Sprintf("this is page chrome and its background is the fixed colour %s. Use a theme token "+
						"- var(--studio-primary), var(--studio-surface-container) or var(--studio-surface-variant) - so it follows the host "+
						"app's theme and its dark mode. Keep a literal colour only where the theme has no token for what it means: a status, "+
						"a chart series, a brand shade the user named", frozen),
				})
			}
		}
	})
	return issues
}

// isChrome reports whether a component is the page's framing rather than its
// content. Named by the model itself - it calls these things header_bar,
// top_header, detail_pane_header - which is the most reliable signal available
// without guessing from layout.
func isChrome(component map[string]interface{}, chromeIds map[string]bool) bool {
	switch componentType(component) {
	case "flex_container", "grid_container", "card":
	default:
		return false
	}
	if isOrnament(component) {
		return false
	}
	if isChromeNamed(component) {
		return true
	}
	id, _ := component["id"].(string)
	return id != "" && chromeIds[id]
}

func isChromeNamed(component map[string]interface{}) bool {
	if isOrnament(component) {
		return false
	}
	id, _ := component["id"].(string)
	name := propertyString(component, "name")
	return chromeNamePattern.MatchString(id) || chromeNamePattern.MatchString(name)
}

func isOrnament(component map[string]interface{}) bool {
	id, _ := component["id"].(string)
	name := propertyString(component, "name")
	return chromeOrnamentPattern.MatchString(id) || chromeOrnamentPattern.MatchString(name)
}

// backgroundValues collects every background-ish style a component sets.
func backgroundValues(component map[string]interface{}) []string {
	out := []string{}
	styles, _ := component["styles"].(map[string]interface{})
	if styles == nil {
		return out
	}
	collect := func(bag map[string]interface{}) {
		for key, value := range bag {
			if !strings.Contains(strings.ToLower(key), "background") {
				continue
			}
			if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
				out = append(out, text)
			}
		}
	}
	if responsive, ok := styles["responsive_styles"].(map[string]interface{}); ok {
		for _, value := range responsive {
			if bag, ok := value.(map[string]interface{}); ok {
				collect(bag)
			}
		}
	}
	if custom, ok := styles["custom"].(map[string]interface{}); ok {
		collect(custom)
	}
	return out
}

func hasGradient(values []string, classes []string) bool {
	for _, value := range values {
		if gradientPattern.MatchString(value) {
			return true
		}
	}
	for _, class := range classes {
		if gradientClassPattern.MatchString(class) {
			return true
		}
	}
	return false
}

// firstHardCodedColour returns a literal colour a background is pinned to, or
// "" when it is a token, an expression, or nothing recognisable.
func firstHardCodedColour(values []string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || strings.Contains(trimmed, "var(--") || strings.HasPrefix(trimmed, "@") || strings.Contains(trimmed, "{{") {
			continue
		}
		if match := hexPattern.FindString(trimmed); match != "" {
			return match
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "rgb(") || strings.HasPrefix(lower, "rgba(") || strings.HasPrefix(lower, "hsl(") {
			return trimmed
		}
	}
	return ""
}
