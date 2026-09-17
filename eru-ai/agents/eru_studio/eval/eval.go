// Package eval scores a page the agent produced.
//
// It is deliberately a reporter, not a second validator: every judgement it
// makes comes from the same checks the agent is held to at runtime, so a page
// that scores clean here is a page the agent would have been allowed to return.
// What eval adds is the ability to run those checks over a saved page, off the
// request path, and to compare two runs - which is the thing we could not do
// while "is this page better?" was answered by looking at it.
package eval

import (
	"fmt"
	"sort"

	studio "github.com/eru-tech/eru/eru-ai/agents/eru_studio"
	catalog "github.com/eru-tech/eru/eru-ai/agents/eru_studio/catalog"
)

// Result is what one page scored.
type Result struct {
	PageId string `json:"page_id"`
	Name   string `json:"name"`
	// Issues is every complaint, root page and nested pages together.
	Issues []catalog.Issue `json:"issues"`
	// Counts tallies them by code, which is what a run-to-run comparison reads.
	Counts map[catalog.Code]int `json:"counts"`
	// Components is the size of what was produced, so a drop in issues that is
	// really a drop in ambition is visible.
	Components  int `json:"components"`
	NestedPages int `json:"nested_pages"`
}

func (r Result) Clean() bool { return len(r.Issues) == 0 }

// Codes lists the distinct codes, sorted, for a compact assertion.
func (r Result) Codes() []string {
	out := make([]string, 0, len(r.Counts))
	for code := range r.Counts {
		out = append(out, string(code))
	}
	sort.Strings(out)
	return out
}

func (r Result) String() string {
	if r.Clean() {
		return fmt.Sprintf("%s: clean (%d components, %d nested pages)", r.Name, r.Components, r.NestedPages)
	}
	return fmt.Sprintf("%s: %d issue(s) across %d code(s) (%d components, %d nested pages)\n%s",
		r.Name, len(r.Issues), len(r.Counts), r.Components, r.NestedPages, catalog.FormatIssues(r.Issues, 40))
}

// Evaluate scores a root page together with the pages it mounts.
//
// nested is keyed by page id. Anything a mount points at that is neither in
// nested nor in known is reported as a dangling mount, which is the failure that
// renders as an empty panel.
func Evaluate(name string, root map[string]interface{}, nested map[string]map[string]interface{}, known map[string]bool) Result {
	c := catalog.Get()
	pageId, _ := root["id"].(string)
	result := Result{
		PageId:      pageId,
		Name:        name,
		Counts:      map[catalog.Code]int{},
		Components:  countComponents(root),
		NestedPages: len(nested),
	}

	result.Issues = append(result.Issues, c.ValidatePage(root)...)

	pages := make([]*studio.NestedPage, 0, len(nested))
	ids := make([]string, 0, len(nested))
	for id := range nested {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		page := nested[id]
		result.Components += countComponents(page)
		for _, issue := range c.ValidatePage(page) {
			issue.Path = fmt.Sprintf("pages (id %q).%s", id, issue.Path)
			issue.ComponentId = id + "/" + issue.ComponentId
			result.Issues = append(result.Issues, issue)
		}
		mountedAt := ""
		for _, mount := range studio.FindMounts(root) {
			if mount.PageId == id {
				mountedAt = mount.ComponentId
				break
			}
		}
		pages = append(pages, &studio.NestedPage{PageId: id, Page: page, MountedAt: mountedAt})
	}

	result.Issues = append(result.Issues, studio.ValidateMounts(root, pages, known)...)
	result.Issues = append(result.Issues, studio.LayoutIssues(root)...)
	for _, id := range ids {
		for _, issue := range studio.LayoutIssues(nested[id]) {
			issue.Path = fmt.Sprintf("pages (id %q).%s", id, issue.Path)
			issue.ComponentId = id + "/" + issue.ComponentId
			result.Issues = append(result.Issues, issue)
		}
	}

	for _, issue := range result.Issues {
		result.Counts[issue.Code]++
	}
	return result
}

func countComponents(page map[string]interface{}) int {
	n := 0
	studio.WalkPage(page, func(map[string]interface{}, string) { n++ })
	return n
}

// mountsOf lists the page ids a page mounts, for callers that only need the
// targets rather than the full mount records.
func mountsOf(page map[string]interface{}) []string {
	out := []string{}
	for _, mount := range studio.FindMounts(page) {
		out = append(out, mount.PageId)
	}
	return out
}
