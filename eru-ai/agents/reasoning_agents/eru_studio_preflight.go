package reasoning_agents

import (
	"context"
	"fmt"
	"sort"
	"strings"

	studio "github.com/eru-tech/eru/eru-ai/agents/eru_studio"
	catalog "github.com/eru-tech/eru/eru-ai/agents/eru_studio/catalog"
	utility "github.com/eru-tech/eru/eru-ai/tools/utility"
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
