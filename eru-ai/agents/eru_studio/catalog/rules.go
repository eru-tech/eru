package catalog

import (
	"fmt"
	"sort"
	"strings"

	ruleset "github.com/eru-tech/eru/eru-ai/agents/ruleset"
)

// Rule is one constraint on a component's properties, stated as data.
//
// These used to live in three places at once - a sentence in the component
// description, a paragraph in the system prompt, and a hand-written check in the
// mount validator - and only the last of them was enforced. Wording drifted from
// behaviour, and the model was being told something slightly different from what
// it would be judged on. A rule here is the single statement of the constraint:
// the validator raises it and the prompt is generated from it, so the two cannot
// disagree.
//
// # Why a Kind rather than more Go functions
//
// The table started with one shape - "set this, and that becomes required" - and
// everything that did not fit became a hand-written method on the validator.
// Within a week there were two such methods encoding constraints no more complex
// than the table's, expressed in a way the prompt generator could not read and an
// eval could not share. A closed set of kinds keeps the table expressive enough
// to absorb them while keeping a rule DATA: that is what lets the same
// declaration drive the in-loop validator, the system prompt, and later an eval
// assertion, without three chances to disagree.
//
// A kind is deliberately a small closed set, not an expression language. The
// predicate stays compiled; only the choice of predicate and its arguments are
// data. That is the boundary the package doc describes: agent-authored rules
// could select a kind, never define one.
// Kind and its constants are the ruleset package's, aliased so the table below
// reads unchanged. The predicates live there because an agent configured through
// the product needs them too, and two copies of a predicate is two chances for
// the validator and the prompt to drift apart - the very thing the table exists
// to prevent.
type Kind = ruleset.Kind

const (
	KindRequires   = ruleset.KindRequires
	KindForbidden  = ruleset.KindForbidden
	KindPathHead   = ruleset.KindPathHead
	KindNotRowPath = ruleset.KindNotRowPath
	// The two kinds the quality rules below need. RequiresAny is "attached to
	// something, the rule does not care which"; NotWithNameLike is a
	// presentation choice contradicting the data behind it.
	KindRequiresAny     = ruleset.KindRequiresAny
	KindNotWithNameLike = ruleset.KindNotWithNameLike
)

// Severity says whether a broken rule rejects the answer or only marks it down.
// Aliased from the engine for the same reason Kind is.
type Severity = ruleset.Severity

const (
	SeverityError   = ruleset.SeverityError
	SeverityQuality = ruleset.SeverityQuality
)

// AnyComponent applies a rule to every component type.
const AnyComponent = "*"

// Scope says which property bag a rule is checked against.
//
// Properties are stored per breakpoint, and the two are not interchangeable. A
// rule about whether a property was SET AT ALL belongs on the base bag, or a
// component that sets it only at base would be reported broken once per
// breakpoint that omits it. A rule about whether a VALUE is well formed belongs
// on every bag, or a bad path hidden in an "md" override goes unchecked.
//
// Getting this wrong is silent in both directions, which is why it is explicit.
type Scope string

const (
	// ScopeBase checks the base bag once per component. The default.
	ScopeBase Scope = ""
	// ScopeEveryBreakpoint checks each breakpoint's bag.
	ScopeEveryBreakpoint Scope = "every_breakpoint"
)

// Rule is a constraint that the validator raises and the prompt states.
type Rule struct {
	// Component is the type the rule applies to, or AnyComponent for all.
	Component string
	// When is the property that arms the rule, and Equals the values that arm
	// it. An empty When makes the rule unconditional.
	When   string
	Equals []string
	// Kind selects the predicate. Empty means KindRequires.
	Kind Kind
	// Scope selects the property bag. Empty means ScopeBase.
	Scope Scope
	// Requires is the property the rule is about.
	Requires string
	// Suffix selects properties by name instead of Requires - every property
	// ending in it. Used by KindNotRowPath, where the rule is about a family of
	// properties (primary_value_path, title_path, badge_text_path...) rather
	// than one.
	Suffix string
	// Except names properties the Suffix match must skip.
	Except []string
	// AllowedHeads are the first path segments KindPathHead accepts, besides a
	// bare array index.
	AllowedHeads []string
	// WhenSet arms the rule only while every property named here carries a
	// value - "this only matters when the component is doing the thing".
	WhenSet []string
	// Properties is the set KindRequiresAny is satisfied by any one of.
	Properties []string
	// Field and Patterns are KindNotWithNameLike's: the property whose value is
	// read, and the substrings in it that make the choice wrong. DefaultOn says
	// the checked property is on when the page does not mention it.
	Field     string
	Patterns  []string
	DefaultOn bool
	// Severity is what breaking the rule costs. Empty rejects the answer.
	Severity Severity
	Code     Code
	// Message is what the model is told when the rule is broken. It has to read
	// as an instruction: it is the only thing the model gets to act on.
	//
	// Placeholders, substituted by the predicate: {subject} {property} {value}
	// {head} {field}. Formatting lives here rather than in Go so the whole rule
	// is one readable declaration.
	Message string
	// Guidance is the same rule stated before the fact, for the system prompt.
	Guidance string
}

// rules is the table. Adding one here adds it to the validator and to the
// prompt at the same time; there is nowhere else to put it.
var rules = []Rule{
	{
		Component: "grid",
		When:      "view_mode",
		Equals:    []string{"board"},
		Requires:  "card_page_id",
		Code:      CodeMountBoardCardUnset,
		Message: "grid is in board view but has no \"card_page_id\", so every record draws with the built-in card, which cannot be designed. " +
			"Author the card as a small page - the fields that belong on a card, not the whole record - emit it in \"pages\" and set card_page_id to its id",
		Guidance: "A grid with view_mode \"board\" MUST have card_page_id pointing at a page you author and emit in \"pages\". " +
			"A board draws every record as a card, so the card is the design; left blank it falls back to a built-in card the user cannot change.",
	},
	{
		Component: "page_ref",
		Requires:  "page",
		Code:      CodeMountPageRefUnset,
		Message:   "page_ref has no \"page\" - it will render an empty panel. Emit the page it should mount in \"pages\" and set \"page\" to that page's id",
		Guidance: "A page_ref MUST have \"page\" set to the id of a page that exists or that you emit in \"pages\". " +
			"A page_ref with no page renders an empty panel, which looks to the user like nothing happened.",
	},
	{
		// Was validator.queryResultPath. The prefix "result." is the one that
		// keeps getting invented: eru-ql answers with an unnamed array, so there
		// is no "result" key to step into.
		Component:    AnyComponent,
		Kind:         KindPathHead,
		Scope:        ScopeEveryBreakpoint,
		Requires:     "query_result_path",
		AllowedHeads: []string{"Results", "results", "rows", "data"},
		Code:         CodeQueryResultPathWrong,
		Message: "{subject}.query_result_path = {value} starts with {head}, which is not in the response - a saved query answers with an unnamed array, so there is no such key to step into. " +
			"Leave query_result_path out entirely: a bare array and an eru-ql `[{\"Results\": [...]}]` envelope are both unwrapped automatically. " +
			"Set it only to reach deeper than that, and then start it with the array index, as in \"0.Results\"",
		Guidance: "query_result_path walks the query RESPONSE, which is an unnamed array - so it starts with an index, as in \"0.Results\", or is left out entirely. " +
			"There is no \"result\" key to step into; a path that starts with one renders an empty component behind a 200.",
	},
	{
		// Was validator.fieldPaths.
		Component: AnyComponent,
		When:      "data_source",
		Equals:    []string{"query"},
		Kind:      KindNotRowPath,
		Scope:     ScopeEveryBreakpoint,
		Suffix:    "_path",
		Except:    []string{"query_result_path", "state_result_path"},
		Code:      CodeFieldPathIsRowPath,
		Message: "{subject}.{property} = {value} is a path into the query RESULT, but it is read as a path into the value of {field} - which with data_source \"query\" is already that column's value, " +
			"so it walks into nothing and the component renders empty. Leave {property} out; set it only when that column itself holds JSON, and then start it at a key inside that JSON",
		Guidance: "A *_path property is read against the value of its matching *_field, NOT against the query response. With data_source \"query\" the row is already picked out, so *_field is a column " +
			"name and its *_path stays empty unless that column holds JSON. Writing the result path there - primary_value_path \"0.amt\" beside primary_value_field \"amt\" - walks into a number.",
	},
}

// qualityRules are the same kind of declaration, marked down rather than
// rejected.
//
// Every one of these is a fault that reached a user. The dashboard they came
// from passed every conformance check on its first attempt: nothing was
// malformed, no property was invented, every query had been probed. It was
// simply not good - a grid headed `an`, `cl` and `os_amt`, a tile with nothing
// behind it, two charts both called "Line Chart", and a rupee symbol on a count
// of loans. No rule that may REJECT an answer should contain any of these, since
// each has a legitimate exception; but an answer carrying them is worth one more
// attempt before it is handed over.
//
// The separation matters in both directions. Make these errors and a dashboard
// fails over a missing chart title. Leave them out and they ship, every time,
// because the model has no reason to think anyone minds.
var qualityRules = []Rule{
	{
		// The fault the whole gate was designed around.
		Component: "grid",
		When:      "data_source",
		Equals:    []string{"query", "entity", "nested_entity", "function"},
		Requires:  "column_overrides",
		Severity:  SeverityQuality,
		Code:      CodeQualityRawColumnHeadings,
		Message: "{subject} derives its columns from the data, so its headings are the raw column names - `an`, `cl`, `os_amt` - which mean nothing to the person reading the dashboard. " +
			"Set column_overrides to label them, keyed by column name: {\"os_amt\": {\"label\": \"Outstanding\"}}",
		Guidance: "A grid bound to a query or an entity takes its headings from the column names, which are usually terse database codes. Set column_overrides to give every column a label " +
			"a reader would recognise. A dashboard headed `an` and `os_amt` is correct and useless.",
	},
	{
		Component:  "tile",
		Kind:       KindRequiresAny,
		Properties: []string{"primary_value_field", "title_field", "graph_data_field", "card_page_id"},
		Severity:   SeverityQuality,
		Code:       CodeQualityUnboundTile,
		Message: "{subject} has no field behind it - none of {property} is set - so it renders as an empty card with a heading and no number. " +
			"Either bind it to a value or leave it out; a tile that shows nothing is worse than one fewer tile",
		Guidance: "Every tile must have something behind it: primary_value_field, title_field, graph_data_field or card_page_id. Do not pad a layout with an empty tile to square off a row - " +
			"an unbound tile reads to the user as data that failed to load.",
	},
	{
		Component: "bar_chart",
		Requires:  "title",
		Severity:  SeverityQuality,
		Code:      CodeQualityChartUntitled,
		Message:   "{subject} has no title, so it falls back to the component's default - two of them on one page both read the same generic words. Set title to what this chart actually shows",
		Guidance: "Every chart - bar, line or pie - needs a title saying what it shows. Left blank it falls back to a generic default, so a page with two bar charts shows the words " +
			"\"Bar Chart\" twice and tells the reader nothing.",
	},
	{
		Component: "line_chart",
		Requires:  "title",
		Severity:  SeverityQuality,
		Code:      CodeQualityChartUntitled,
		Message:   "{subject} has no title, so it falls back to the component's default - two of them on one page both read the same generic words. Set title to what this chart actually shows",
	},
	{
		Component: "pie_chart",
		Requires:  "title",
		Severity:  SeverityQuality,
		Code:      CodeQualityChartUntitled,
		Message:   "{subject} has no title, so it falls back to the component's default - two of them on one page both read the same generic words. Set title to what this chart actually shows",
	},
	{
		Component: "tile",
		WhenSet:   []string{"currency_symbol", "secondary_value_field"},
		Kind:      KindNotWithNameLike,
		Requires:  "secondary_is_currency",
		DefaultOn: true,
		Field:     "secondary_value_field",
		Patterns:  []string{"cnt", "count", "_qty", "qty_", "_num", "num_", "_no", "total_records", "records"},
		Severity:  SeverityQuality,
		Code:      CodeQualityCurrencyOnACount,
		Message: "{subject} shows {field} = {value}, which reads as a count, but secondary_is_currency is on (it defaults to on), so the tile prints a currency symbol in front of it. " +
			"Set secondary_is_currency to false",
		Guidance: "secondary_is_currency DEFAULTS TO TRUE. On the commonest KPI tile of all - an amount beside a count of records - that puts a currency symbol in front of the count. " +
			"Set it to false whenever the secondary value is a count rather than money.",
	},
}

// Rules is the conformance table, for callers that render it.
func Rules() []Rule { return rules }

// QualityRules is the table the quality gate reads. It is separate from Rules
// so that no caller can enforce a quality rule by accident: reaching for it is
// a deliberate act, and the only thing entitled to act on it is a gate that
// cannot fail the request.
func QualityRules() []Rule { return qualityRules }

// GenericRules is the table in the shared engine's terms, for a caller outside
// this package that wants to judge components by the same declarations.
//
// This is what makes a rule declared ONCE hold in two places. The eval used to
// re-implement these constraints by hand - resultPathFaults and fieldPathFaults
// in agents/eval were the same two checks as CodeQueryResultPathWrong and
// CodeFieldPathIsRowPath, written a day apart in a different file. Two copies of
// a rule is two chances to drift, and the one that drifts silently is the eval:
// it would keep passing a page the validator would now reject, or fail one it
// would now allow, and either way the suite stops describing the product.
//
// Scope is dropped deliberately. It says WHICH property bag to read, which is a
// question about the live page structure; a caller judging a finished page
// flattens the breakpoints first and wants every rule applied to the result.
func GenericRules() []ruleset.Rule { return project(rules) }

// GenericQualityRules is the quality table in the shared engine's terms, for the
// gate that runs after validation passes.
func GenericQualityRules() []ruleset.Rule { return project(qualityRules) }

func project(table []Rule) []ruleset.Rule {
	out := make([]ruleset.Rule, 0, len(table))
	for _, rule := range table {
		generic := rule.generic()
		if rule.Component != AnyComponent {
			generic.Match = map[string]string{"type": rule.Component}
		}
		out = append(out, generic)
	}
	return out
}

// armed reports whether the rule's condition holds for this component.
func (r Rule) armed(componentType string, properties map[string]interface{}) bool {
	if r.Component != AnyComponent && r.Component != componentType {
		return false
	}
	if r.When == "" {
		return true
	}
	value, _ := properties[r.When].(string)
	value = strings.TrimSpace(value)
	for _, candidate := range r.Equals {
		if strings.EqualFold(value, candidate) {
			return true
		}
	}
	return false
}

// checkRules raises an issue for every rule this component arms and then fails.
//
// A property written as a runtime expression satisfies every rule: no static
// check can know what it resolves to, and refusing it would ban a legitimate
// page.
func (v *validator) checkRules(scope Scope, label, componentType string, properties map[string]interface{}) {
	subject := ruleset.Subject{Path: label, Name: componentType, Properties: properties}
	for _, rule := range rules {
		if rule.Scope != scope || (rule.Component != AnyComponent && rule.Component != componentType) {
			continue
		}
		for _, finding := range rule.generic().Check(subject) {
			v.add(finding.Path, Code(finding.Code), "%s", finding.Message)
		}
	}
}

// generic projects a catalog rule onto the shared engine. Component becomes a
// Match on "type" - except AnyComponent, which the caller has already filtered,
// because a property bag does not carry its own type.
func (r Rule) generic() ruleset.Rule {
	return ruleset.Rule{
		When:         r.When,
		Equals:       r.Equals,
		WhenSet:      r.WhenSet,
		Kind:         r.Kind,
		Property:     r.Requires,
		Properties:   r.Properties,
		Suffix:       r.Suffix,
		Except:       r.Except,
		AllowedHeads: r.AllowedHeads,
		Field:        r.Field,
		Patterns:     r.Patterns,
		DefaultOn:    r.DefaultOn,
		Severity:     r.Severity,
		Code:         string(r.Code),
		Message:      r.Message,
		Guidance:     r.Guidance,
	}
}

// RulesPrompt renders both tables for the system prompt, so what the model is
// told and what it is judged on are the same sentences.
//
// The quality rules are stated here too, and under their own heading. A gate
// that only ever speaks after the fact teaches the model nothing: it pays for a
// second model call to say something the first call could have been told for
// free. Telling it up front is how the retry stays rare.
func RulesPrompt() string {
	out := renderRules(rules,
		"PROPERTIES THAT REQUIRE OTHER PROPERTIES",
		"These are checked. An answer that breaks one is rejected and sent back to you.")
	quality := renderRules(qualityRules,
		"WHAT MAKES A PAGE GOOD RATHER THAN MERELY VALID",
		"These are checked after the page is found valid. Breaking one does not make the page wrong, it makes it not worth handing over - "+
			"and it will be sent back to you once to put right.")
	if quality != "" {
		out = out + "\n\n" + quality
	}
	return out
}

func renderRules(table []Rule, heading, preamble string) string {
	byComponent := map[string][]Rule{}
	names := []string{}
	for _, rule := range table {
		if _, seen := byComponent[rule.Component]; !seen {
			names = append(names, rule.Component)
		}
		byComponent[rule.Component] = append(byComponent[rule.Component], rule)
	}
	sort.Strings(names)

	var b strings.Builder
	b.WriteString(heading + "\n")
	b.WriteString(preamble + "\n\n")
	wrote := false
	for _, name := range names {
		label := name
		if name == AnyComponent {
			label = "every component"
		}
		var stated []string
		for _, rule := range byComponent[name] {
			if strings.TrimSpace(rule.Guidance) == "" {
				continue
			}
			stated = append(stated, rule.Guidance)
		}
		if len(stated) == 0 {
			continue
		}
		fmt.Fprintf(&b, "%s:\n", label)
		for _, guidance := range stated {
			fmt.Fprintf(&b, "- %s\n", guidance)
		}
		wrote = true
	}
	if !wrote {
		return ""
	}
	return strings.TrimRight(b.String(), "\n")
}
