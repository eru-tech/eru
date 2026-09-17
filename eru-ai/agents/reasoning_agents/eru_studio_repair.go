package reasoning_agents

import (
	"context"
	"fmt"

	agents "github.com/eru-tech/eru/eru-ai/agents"
	studio "github.com/eru-tech/eru/eru-ai/agents/eru_studio"
	catalog "github.com/eru-tech/eru/eru-ai/agents/eru_studio/catalog"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

// effectiveBasePage is the page an answer is a diff against: normally the page
// the client sent, and during a repair the page the rejected attempt produced.
func effectiveBasePage(ctx context.Context) map[string]interface{} {
	if state := studio.RepairStateFrom(ctx); state.Active() {
		return state.Base()
	}
	return studio.BasePageFrom(ctx)
}

// RepairTurn answers a rejection with a diff instead of a rewrite.
//
// It declines more often than it accepts, and that is deliberate. A diff is only
// meaningful when there is a coherent page to diff against, so an answer that
// failed on its own shape - an unreadable patch, a missing page, a mode nobody
// recognises - goes back through the ordinary retry. So does a scoped edit: a
// scope is a promise about which components an answer may touch, expressed
// against the page the user is looking at, and re-basing the diff onto the
// rejected attempt would quietly change what that promise means.
func (eruStudioAgent *EruStudioAgent) RepairTurn(ctx context.Context, failed map[string]interface{}, valErr error) (agents.RepairTurn, bool) {
	if !studio.EnvelopeEnabled(ctx) {
		// Bare-page mode has no patch vocabulary to answer in.
		return agents.RepairTurn{}, false
	}
	if studio.ScopeFrom(ctx) != nil {
		return agents.RepairTurn{}, false
	}
	state := studio.RepairStateFrom(ctx)
	if state == nil {
		return agents.RepairTurn{}, false
	}

	base := effectiveBasePage(ctx)
	page := resolvedRootPage(failed, base)
	if len(page) == 0 {
		return agents.RepairTurn{}, false
	}
	// A page we cannot even read the components of is not something to diff.
	if _, ok := page["components"].([]interface{}); !ok {
		return agents.RepairTurn{}, false
	}

	pageId, _ := page["id"].(string)
	nested, err := parseNestedPages(failed, pageId)
	if err != nil {
		// The nested pages are part of what was wrong; let the model rewrite.
		return agents.RepairTurn{}, false
	}

	issues := eruStudioIssuesOf(ctx, failed)
	state.Begin(page, nested)
	logs.WithContext(ctx).Info(fmt.Sprintf("eru studio: repairing %d issue(s) as a patch against the rejected page", len(issues)))

	return agents.RepairTurn{
		Schema: buildEruPageUpdateOutputSchema(),
		Prompt: repairPrompt(issues, valErr, len(nested)),
	}, true
}

// eruStudioIssuesOf re-runs the checks so the repair can address them by path.
// The error text the loop carries is already formatted for a human; the issues
// are what let the prompt point at components.
func eruStudioIssuesOf(ctx context.Context, output map[string]interface{}) []catalog.Issue {
	_, issues := eruStudioPageIssues(ctx, output)
	return issues
}

func repairPrompt(issues []catalog.Issue, valErr error, nestedCount int) string {
	detail := catalog.FormatIssues(issues, maxReportedPageIssues)
	if detail == "" && valErr != nil {
		detail = "- " + valErr.Error()
	}
	carried := ""
	if nestedCount > 0 {
		carried = fmt.Sprintf("\nThe %d page(s) you emitted in \"pages\" are kept as they are. Re-send one in \"pages\" only if it is "+
			"itself one of the things that has to change.\n", nestedCount)
	}
	return fmt.Sprintf(`Your page was rejected. Do NOT write it again - the page you produced is now the base, and everything not listed below is already correct and already saved.

Answer with structured_output using mode "patch", carrying ONLY the changes that fix these:

%s
%s
How to answer:
- "patch.upsert": the full component objects for the components you are changing, each with its existing "id". A component you list replaces the one with that id, so include all of its properties, not only the changed one.
- "patch.remove": ids to delete, if a fix means removing something.
- Touch nothing else. A component you do not mention stays exactly as it is.
- If a fix needs a component that does not exist yet, add it in "patch.upsert" with a new id and put that id in its parent's "children_ids".

Fix only what is listed. Adding improvements now is how a corrected page acquires a new fault.`, detail, carried)
}
