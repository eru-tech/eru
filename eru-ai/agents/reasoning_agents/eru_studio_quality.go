package reasoning_agents

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/eru-tech/eru/eru-ai/agents"
	"github.com/eru-tech/eru/eru-ai/agents/eru_studio"
	"github.com/eru-tech/eru/eru-ai/agents/eru_studio/catalog"
	ruleset "github.com/eru-tech/eru/eru-ai/agents/ruleset"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

// JudgeQuality answers the question conformance cannot: the page is valid, but
// is it worth handing over?
//
// The rubric is not written here. It is catalog.QualityRules - the same kind of
// declaration the validator reads, marked SeverityQuality so that nothing can
// reject a page with it. That keeps this function to the three decisions that
// are genuinely its own: which page to judge, what to forgive, and how a set of
// findings becomes one rating.
func (eruStudioAgent *EruStudioAgent) JudgeQuality(ctx context.Context, output map[string]interface{}) (agents.QualityVerdict, error) {
	if output == nil {
		return agents.QualityVerdict{}, nil
	}
	basePage := effectiveBasePage(ctx)
	pages := pagesToJudge(ctx, output, basePage)
	if len(pages) == 0 {
		return agents.QualityVerdict{}, nil
	}

	findings := qualityFindings(pages)

	// An edit is judged on what it introduced, never on what it inherited.
	// Asked to rename one tile on a page whose twelve grids have always shown
	// raw headings, the agent must not be sent back to relabel all of them: it
	// would spend its one attempt rewriting work nobody asked about, and the
	// rename - the thing actually requested - is what gets lost in the diff.
	// This is the same forgiveness introducedWiringIssues applies, for the same
	// reason.
	//
	// It is forgiveness against the page the USER has, never against an attempt
	// of our own. Once a quality repair is running, basePage IS the page this
	// gate just objected to, and forgiving what was already wrong there forgives
	// exactly the thing the repair was called to fix - the gate would then rate
	// an unchanged page "good" and record a fix that never happened.
	forgiven := basePage
	if eru_studio.RepairStateFrom(ctx).Active() {
		forgiven = eru_studio.BasePageFrom(ctx)
	}
	if len(forgiven) > 0 {
		findings = introducedOnly(findings, qualityFindings([]map[string]interface{}{forgiven}))
	}
	if len(findings) == 0 {
		return agents.QualityVerdict{Rating: agents.QualityGood}, nil
	}

	logs.WithContext(ctx).Info(fmt.Sprintf("eru studio quality gate found %d issue(s) on a page that passed validation", len(findings)))
	return agents.QualityVerdict{
		Rating: agents.QualityPoor,
		Reason: formatQualityFindings(findings),
	}, nil
}

// pagesToJudge is the root page plus anything the answer emitted alongside it.
// A board's card page is a page the user sees, so it is held to the same bar.
//
// The two output modes carry the page differently and the split has to be made
// here, exactly as eruStudioPageIssues makes it: an envelope holds the page (or
// a patch to resolve against the base) under "page", while a bare answer IS the
// page. Reading an envelope's shape out of a bare answer silently falls back to
// judging the base page instead of the new one - the gate then passes anything,
// because every fault it finds is one the base already had.
func pagesToJudge(ctx context.Context, output, basePage map[string]interface{}) []map[string]interface{} {
	var pages []map[string]interface{}
	root := output
	if eru_studio.EnvelopeEnabled(ctx) {
		root = resolvedRootPage(output, basePage)
	}
	if len(root) > 0 {
		pages = append(pages, root)
	}
	return append(pages, nestedPagesForPreflight(output, basePage)...)
}

// qualityFindings walks every component of every page and applies the rubric.
//
// Properties are read at base. Every quality rule is about whether a property
// was set AT ALL - a label, a title, a binding - and that is a base-bag
// question: checking each breakpoint would report a component once per
// breakpoint that does not override the property, which is all of them.
func qualityFindings(pages []map[string]interface{}) []ruleset.Finding {
	var subjects []ruleset.Subject
	for _, page := range pages {
		pageId, _ := page["id"].(string)
		eru_studio.WalkPage(page, func(component map[string]interface{}, _ string) {
			componentType, _ := component["type"].(string)
			if componentType == "" {
				return
			}
			properties, _ := component["properties"].(map[string]interface{})
			base, _ := properties["base"].(map[string]interface{})
			bag := make(map[string]interface{}, len(base)+1)
			for key, value := range base {
				bag[key] = value
			}
			// The rules select on "type", which lives on the component rather
			// than in its property bag.
			bag["type"] = componentType

			id, _ := component["id"].(string)
			label := fmt.Sprintf("%s %q", componentType, id)
			if pageId != "" {
				label = fmt.Sprintf("page %q, %s", pageId, label)
			}
			subjects = append(subjects, ruleset.Subject{Path: label, Name: componentType, Properties: bag})
		})
	}
	return ruleset.Check(catalog.GenericQualityRules(), subjects)
}

func fingerprint(finding ruleset.Finding) string {
	return finding.Path + "|" + finding.Code + "|" + finding.Property
}

func introducedOnly(findings, preExisting []ruleset.Finding) []ruleset.Finding {
	if len(preExisting) == 0 || len(findings) == 0 {
		return findings
	}
	known := make(map[string]bool, len(preExisting))
	for _, finding := range preExisting {
		known[fingerprint(finding)] = true
	}
	out := make([]ruleset.Finding, 0, len(findings))
	for _, finding := range findings {
		if known[fingerprint(finding)] {
			continue
		}
		out = append(out, finding)
	}
	return out
}

// maxReportedQualityFindings caps the feedback. One wrong idea applied to eight
// tiles is one thing to fix, and a wall of near-identical sentences crowds out
// the part of the prompt that says what to do with them.
const maxReportedQualityFindings = 10

// formatQualityFindings is what the model is handed. Grouped by code, because
// these arrive in batches and "six grids show raw column headings" is a single
// instruction where six sentences are six chances to fix one and stop.
func formatQualityFindings(findings []ruleset.Finding) string {
	byCode := map[string][]ruleset.Finding{}
	order := []string{}
	for _, finding := range findings {
		if _, seen := byCode[finding.Code]; !seen {
			order = append(order, finding.Code)
		}
		byCode[finding.Code] = append(byCode[finding.Code], finding)
	}
	sort.Strings(order)

	var b strings.Builder
	shown := 0
	for _, code := range order {
		group := byCode[code]
		fmt.Fprintf(&b, "- %s\n", group[0].Message)
		for _, finding := range group {
			if shown >= maxReportedQualityFindings {
				fmt.Fprintf(&b, "  ...and %d more like it\n", len(findings)-shown)
				return strings.TrimRight(b.String(), "\n")
			}
			fmt.Fprintf(&b, "  %s\n", finding.Path)
			shown++
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// QualityRepairTurn answers the quality verdict with a patch against the page
// that just passed.
//
// It reuses the repair machinery wholesale - the accepted page becomes the base
// and the model answers in patch mode - with one thing changed that matters
// more than all the plumbing: the prompt must not say the page was rejected. It
// was not. A model told its correct page was refused starts looking for what is
// wrong with it, and what it finds is the reason both live runs came back with a
// page in worse shape than the one they replaced.
func (eruStudioAgent *EruStudioAgent) QualityRepairTurn(ctx context.Context, accepted map[string]interface{}, verdict agents.QualityVerdict) (agents.RepairTurn, bool) {
	// The same preconditions as a repair: without patch mode there is no
	// vocabulary to answer a diff in, and a scoped edit's promise is made
	// against the user's page, not against a page we just produced.
	// Declining is safe but never silent: a gate that quietly stops asking for a
	// diff looks exactly like one that asked and was ignored, and the two want
	// opposite fixes.
	decline := func(why string) (agents.RepairTurn, bool) {
		logs.WithContext(ctx).Info("eru studio: improving the accepted page as a patch is not possible (" + why + "); asking for the whole answer again")
		return agents.RepairTurn{}, false
	}
	if !eru_studio.EnvelopeEnabled(ctx) {
		return decline("bare-page mode has no patch vocabulary")
	}
	if eru_studio.ScopeFrom(ctx) != nil {
		return decline("the edit is scoped, and a scope is a promise about the user's page")
	}
	state := eru_studio.RepairStateFrom(ctx)
	if state == nil {
		return decline("no repair state on the request")
	}
	page := resolvedRootPage(accepted, effectiveBasePage(ctx))
	if len(page) == 0 {
		return decline("the accepted answer has no readable root page")
	}
	if _, ok := page["components"].([]interface{}); !ok {
		return decline("the accepted page has no components list")
	}
	pageId, _ := page["id"].(string)
	nested, err := parseNestedPages(accepted, pageId)
	if err != nil {
		return decline("the nested pages cannot be parsed: " + err.Error())
	}

	state.Begin(page, nested)
	logs.WithContext(ctx).Info("eru studio: improving the accepted page as a patch rather than regenerating it")

	return agents.RepairTurn{
		Schema: buildEruPageUpdateOutputSchema(),
		Prompt: qualityRepairPrompt(verdict, len(nested)),
	}, true
}

func qualityRepairPrompt(verdict agents.QualityVerdict, nestedCount int) string {
	carried := ""
	if nestedCount > 0 {
		carried = fmt.Sprintf("\nThe %d page(s) you emitted in \"pages\" are kept as they are. Re-send one in \"pages\" only if it is itself one of the things to improve.\n", nestedCount)
	}
	return fmt.Sprintf(`Your page is VALID and has been accepted. Nothing about it is wrong, nothing has been sent back to you, and it is now the base - everything in it is already correct and already saved.

Read as the person receiving it will read it, one thing is not good enough yet:

%s

Answer with structured_output using mode "patch", carrying ONLY the change that puts that right. Do NOT write the page again: re-sending components that are already correct is how a working page gets broken, and anything you leave out is kept exactly as it is.
%s
You have ONE attempt. After it the page is delivered as it stands, so make the named change and nothing else.`, verdict.Reason, carried)
}
