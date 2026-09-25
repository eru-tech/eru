package reasoning_agents

import (
	"context"
	"fmt"
	"sort"
	"strings"

	studio "github.com/eru-tech/eru/eru-ai/agents/eru_studio"
	catalog "github.com/eru-tech/eru/eru-ai/agents/eru_studio/catalog"
	utility "github.com/eru-tech/eru/eru-ai/tools/utility"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

// maxNamedBindings caps how many components an issue lists. The model needs to
// see that the complaint is about real bindings, not to read all forty of them.
const maxNamedBindings = 6

// preflightIssues holds the page to the lookups behind it.
//
// A form field binds through a field name the model cannot know, and the prompt
// has always said to read the entity metadata first. Prose alone made that a
// suggestion: the model often skipped it, invented plausible field names, and
// produced a page that renders correctly and binds to nothing - the failure the
// user only discovers after saving. This makes the instruction checkable.
//
// It is deliberately narrow. It fires only when the lookup was actually offered
// this run, only when it has not already failed, and only for bindings this edit
// introduced: an edit that moves a button is not answerable for bindings the
// page already had.
func preflightIssues(ctx context.Context, resolved, basePage map[string]interface{}, nested []map[string]interface{}) []catalog.Issue {
	ledger := studio.LedgerFrom(ctx)
	if !ledger.Enforceable(utility.EntityMetadataToolName) || ledger.Called(utility.EntityMetadataToolName) {
		return nil
	}

	claims := studio.IntroducedBindings(basePage, resolved)
	for _, page := range nested {
		// A nested page is new in its entirety, so every binding on it is one
		// this edit introduced.
		claims = append(claims, studio.BindingClaims(page)...)
	}
	if len(claims) == 0 {
		return nil
	}

	named := make([]string, 0, len(claims))
	seen := map[string]bool{}
	for _, claim := range claims {
		label := claim.Identifier
		if label == "" {
			label = claim.EntityName
		}
		if claim.ComponentId != "" {
			label = fmt.Sprintf("%s (%s)", label, claim.ComponentId)
		}
		if seen[label] {
			continue
		}
		seen[label] = true
		named = append(named, label)
	}
	sort.Strings(named)
	shown := named
	suffix := ""
	if len(shown) > maxNamedBindings {
		shown = shown[:maxNamedBindings]
		suffix = fmt.Sprintf(" and %d more", len(named)-maxNamedBindings)
	}

	return []catalog.Issue{{
		Code: catalog.CodeBindingsWithoutMetadata,
		Message: fmt.Sprintf(
			"this page claims to bind to real entity fields (%s%s) but %s was never called, so those names are guesses. "+
				"Call %s, then set each component's \"name\"/\"identifier\" to a field it returns and the label to that field's display name. "+
				"If the field you want is genuinely not in the metadata, leave \"identifier\" off that component so it does not claim a binding, and say so in your summary",
			strings.Join(shown, ", "), suffix, utility.EntityMetadataToolName, utility.EntityMetadataToolName),
	}}
}

// nestedPagesForPreflight returns the pages an edit emitted alongside the root.
// A malformed "pages" block is already reported by validateNestedPages, so this
// simply has nothing to check in that case.
func nestedPagesForPreflight(output, basePage map[string]interface{}) []map[string]interface{} {
	rootPageId := ""
	if page, ok := output["page"].(map[string]interface{}); ok {
		rootPageId, _ = page["id"].(string)
	} else if basePage != nil {
		rootPageId, _ = basePage["id"].(string)
	}
	nested, err := parseNestedPages(output, rootPageId)
	if err != nil {
		return nil
	}
	pages := make([]map[string]interface{}, 0, len(nested))
	for _, page := range nested {
		pages = append(pages, page.Page)
	}
	return pages
}

// entityBindingIssues catches a page bound to the physical table instead of the
// entity.
//
// The two differ only by a process prefix - entity "fn" lives in table "scf_fn"
// - and the metadata answer carries both, so the model picks whichever looks
// more like an identifier. Telling it the difference in the prompt did not stop
// it. This does, and it can say exactly which name to use, because the lookup
// that returned the table also returned the entity it belongs to.
func entityBindingIssues(ctx context.Context, resolved, basePage map[string]interface{}, nested []map[string]interface{}) []catalog.Issue {
	ledger := studio.LedgerFrom(ctx)
	if !ledger.KnowsEntities() {
		return nil
	}

	issues := []catalog.Issue{}
	seen := map[string]bool{}
	report := func(claim studio.BindingClaim) {
		if claim.EntityName == "" {
			return
		}
		entity, isTable := ledger.EntityForTable(claim.EntityName)
		if !isTable {
			return
		}
		key := claim.ComponentId + "|" + claim.EntityName
		if seen[key] {
			return
		}
		seen[key] = true
		issues = append(issues, catalog.Issue{
			Path:        fmt.Sprintf("components (id %q)", claim.ComponentId),
			Code:        catalog.CodeEntityIsATableName,
			ComponentId: claim.ComponentId,
			Message: fmt.Sprintf("entity_name is %q, which is the physical TABLE, not the entity. Set it to %q. "+
				"The table is the process name joined to the entity name and exists only for generating SQL; a page bound to it renders nothing",
				claim.EntityName, entity),
		})
	}

	for _, claim := range studio.IntroducedBindings(basePage, resolved) {
		report(claim)
	}
	for _, page := range nested {
		for _, claim := range studio.BindingClaims(page) {
			report(claim)
		}
	}
	return issues
}

// unprobedQueryIssues holds a page to the queries behind it.
//
// A saved query's response shape is not fixed - the path into the rows and the
// column names differ per query and per source - so binding to one without
// running it is a guess. The guess fails silently: a wrong path resolves to
// nothing rather than erroring, so the component renders blank while its query
// returns 200 with rows in it, which is the hardest kind of blank screen to
// explain. run_query reports the real path and columns, which is why the prompt
// requires it before any binding.
//
// It was not enough. A dashboard bound seven tiles, two charts and two grids to
// eight queries, called run_query zero times, and reported "all columns bound
// from confirmed run_query results" - so the claim cannot be taken on trust and
// is checked here instead.
//
// Narrow on purpose, like the metadata check: only when run_query was offered
// this run and has not already failed.
func unprobedQueryIssues(ctx context.Context, resolved map[string]interface{}, nested []map[string]interface{}) []catalog.Issue {
	ledger := studio.LedgerFrom(ctx)
	pages := append([]map[string]interface{}{resolved}, nested...)

	// A query the tool said does not exist is always an error to bind, whether
	// or not the probe rule is enforceable. Enforceability is about being fair
	// to an agent whose tools are broken; nothing here is broken, the question
	// was asked and answered.
	// What the check is working from, every time. Three separate false
	// diagnoses of this rule have come from assuming one of these three lists
	// rather than reading it.
	logs.WithContext(ctx).Info(fmt.Sprintf("query bindings: page binds %v, probed %v, reported missing %v",
		allBoundQueryNames(pages), ledger.ProbedQueries(), ledger.MissingQueries()))

	if missing := missingQueryIssues(ledger, pages); len(missing) > 0 {
		return missing
	}

	if !ledger.Enforceable(utility.RunQueryToolName) {
		return nil
	}

	var unprobed []string
	seen := map[string]bool{}
	for _, page := range pages {
		for _, name := range boundQueryNames(page) {
			if seen[name] || ledger.ProbedQuery(name) {
				continue
			}
			seen[name] = true
			unprobed = append(unprobed, name)
		}
	}
	if len(unprobed) == 0 {
		return nil
	}
	sort.Strings(unprobed)
	shown := unprobed
	suffix := ""
	if len(shown) > maxNamedBindings {
		shown = shown[:maxNamedBindings]
		suffix = fmt.Sprintf(" and %d more", len(unprobed)-maxNamedBindings)
	}

	return []catalog.Issue{{
		Code: catalog.CodeQueryNotProbed,
		Message: fmt.Sprintf(
			"this page binds to %s (%s%s) without running %s first, so the row path and every column name on those components are guesses. "+
				"Call %s for each one: it reports \"result_path\" and \"columns\". Use the columns for tile *_field, chart dimensions/measures and grid fields, "+
				"and leave query_result_path out unless result_path is deeper than the usual envelope. A wrong path or column renders an empty component while the query still answers 200",
			queryWord(len(unprobed)), strings.Join(shown, ", "), suffix,
			utility.RunQueryToolName, utility.RunQueryToolName),
	}}
}

// unboundProbedQueryIssues is the other half of unprobedQueryIssues.
//
// That rule says a component may not bind a query the model never ran. It can
// be satisfied two ways: by running the query, or by not binding it. The second
// is cheaper, and it is what a rejected page came back having done - seven KPI
// tiles that had asked for db_tiles_rff turned into data_source "static" with
// nothing behind them, which the rule was happy with and the user was not.
//
// A query is not probed idly. Running one costs a tool call and is only worth
// making to find out what to bind to it, so a query that was probed and then
// left unbound is a binding that was dropped rather than corrected.
func unboundProbedQueryIssues(ctx context.Context, resolved map[string]interface{}, nested []map[string]interface{}) []catalog.Issue {
	ledger := studio.LedgerFrom(ctx)
	probed := ledger.ProbedQueries()
	if len(probed) == 0 {
		return nil
	}

	bound := map[string]bool{}
	for _, page := range append([]map[string]interface{}{resolved}, nested...) {
		for _, name := range boundQueryNames(page) {
			bound[name] = true
		}
	}
	var dropped []string
	for _, name := range probed {
		if !bound[name] {
			dropped = append(dropped, name)
		}
	}
	if len(dropped) == 0 {
		return nil
	}
	shown := dropped
	suffix := ""
	if len(shown) > maxNamedBindings {
		shown = shown[:maxNamedBindings]
		suffix = fmt.Sprintf(" and %d more", len(dropped)-maxNamedBindings)
	}

	return []catalog.Issue{{
		Code: catalog.CodeQueryProbedNotBound,
		Message: fmt.Sprintf(
			"you ran %s for %s (%s%s) and then bound nothing to %s. Unbinding is not a way to satisfy \"probe before you bind\" - it leaves the user with a component that shows nothing "+
				"and no error. Bind each of them to the component it was run for, using the columns %s reported; if one genuinely has no place on this page, remove the component instead of leaving it empty",
			utility.RunQueryToolName, queryWord(len(dropped)), strings.Join(shown, ", "), suffix, itOrThem(len(dropped)), utility.RunQueryToolName),
	}}
}

// missingQueryIssues faults a page bound to a query the tool reported as absent.
//
// It is stated separately from "you never probed this" because it is a
// different mistake and wants a different instruction. Not probing is an
// omission the agent can put right by probing. Binding to a name that does not
// exist cannot be put right by probing - the query has to be written, or the
// component has to go - and telling the agent to "call run_query for it" would
// send it round a loop it cannot leave.
func missingQueryIssues(ledger *studio.Ledger, pages []map[string]interface{}) []catalog.Issue {
	var missing []string
	seen := map[string]bool{}
	for _, page := range pages {
		for _, name := range boundQueryNames(page) {
			if seen[name] || !ledger.MissingQuery(name) {
				continue
			}
			seen[name] = true
			missing = append(missing, name)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	sort.Strings(missing)
	return []catalog.Issue{{
		Code: catalog.CodeQueryMissing,
		Message: fmt.Sprintf(
			"this page binds to %s (%s) that %s reported does not exist in this workspace. Binding to a name that is not there renders an empty component behind a 200, "+
				"and no amount of probing will fix it. Either drop the component, or bind it to a query that does exist - call list_queries to see them - "+
				"and say in your report that the one the request named is not there",
			queryWord(len(missing)), strings.Join(missing, ", "), utility.RunQueryToolName),
	}}
}

func itOrThem(n int) string {
	if n == 1 {
		return "it"
	}
	return "them"
}

func queryWord(n int) string {
	if n == 1 {
		return "a query"
	}
	return "queries"
}

// boundQueryNames is every saved query a page's components read from.
func boundQueryNames(page map[string]interface{}) []string {
	var names []string
	var walk func(node interface{})
	walk = func(node interface{}) {
		switch typed := node.(type) {
		case map[string]interface{}:
			if props, ok := typed["properties"].(map[string]interface{}); ok {
				for _, bag := range props {
					values, ok := bag.(map[string]interface{})
					if !ok {
						continue
					}
					// Only a component actually reading from a query counts: a
					// query named on a component wired to page data or state is
					// not a binding this can hold it to.
					//
					// Absence of a source is not evidence against, though. A tile
					// and a grid declare data_source "query"; a line_chart and a
					// bar_chart have no such property and are bound by naming a
					// query and nothing else. Requiring the declaration made every
					// chart on a page invisible here - so the charts, the very
					// components that had invented their column names, were the
					// ones the probe rule never looked at.
					source, _ := values["value_source"].(string)
					dataSource, _ := values["data_source"].(string)
					if (source != "" && source != "query") || (dataSource != "" && dataSource != "query") {
						continue
					}
					if name, _ := values["query"].(string); strings.TrimSpace(name) != "" {
						names = append(names, strings.TrimSpace(name))
					}
				}
			}
			for _, child := range typed {
				walk(child)
			}
		case []interface{}:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(page)
	return names
}

// allBoundQueryNames is every query name bound across a set of pages, deduped.
func allBoundQueryNames(pages []map[string]interface{}) []string {
	seen := map[string]bool{}
	var names []string
	for _, page := range pages {
		for _, name := range boundQueryNames(page) {
			if seen[name] {
				continue
			}
			seen[name] = true
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}
