package eru_studio

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	catalog "github.com/eru-tech/eru/eru-ai/agents/eru_studio/catalog"
)

// PagePlan is what a design is before any of it is drawn: which pages it takes,
// what each is for, and where each one is mounted.
//
// It exists because "generate the whole design in one answer" left the number of
// pages up to whatever the model felt like doing. A board view whose card page
// is the actual design would come back as a single page with a default card, and
// nothing had noticed, because nothing had ever asked how many pages there
// should be. Asking first - cheaply, in a small schema, against checks that run
// before a single component is written - makes the answer to that question
// something the run can be held to.
type PagePlan struct {
	RootPageId string     `json:"root_page_id"`
	Pages      []PlanPage `json:"pages"`
	Summary    string     `json:"summary,omitempty"`
}

// PlanPage is one page of the design.
type PlanPage struct {
	PageId  string `json:"page_id"`
	Role    string `json:"role"`
	Purpose string `json:"purpose"`
	Entity  string `json:"entity_name,omitempty"`
	// MountedAt is the id of the component ON THE ROOT PAGE that mounts this
	// page, and MountProperty the property that points at it. Empty on the root.
	MountedAt     string `json:"mounted_at,omitempty"`
	MountProperty string `json:"mount_property,omitempty"`
	// Skeleton is one line describing the layout, so the page that gets built is
	// the page that was planned.
	Skeleton string `json:"skeleton,omitempty"`
}

const (
	// PlanRoleRoot is the page the user is editing.
	PlanRoleRoot = "root"
	// PlanRoleCard is a record drawn inside a board, a tile or a list.
	PlanRoleCard = "card"
	// PlanRoleDetail is a form or detail view mounted in a panel.
	PlanRoleDetail = "detail"
)

// IsMultiPage reports whether the plan calls for more than the root.
func (p PagePlan) IsMultiPage() bool { return len(p.Nested()) > 0 }

// Nested is every page but the root, in a stable order.
func (p PagePlan) Nested() []PlanPage {
	out := []PlanPage{}
	for _, page := range p.Pages {
		if page.PageId != "" && page.PageId != p.RootPageId {
			out = append(out, page)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].PageId < out[j].PageId })
	return out
}

// Root is the root page's entry.
func (p PagePlan) Root() (PlanPage, bool) {
	for _, page := range p.Pages {
		if page.PageId == p.RootPageId {
			return page, true
		}
	}
	return PlanPage{}, false
}

// PlanSchema is the shape the planning step answers in. It is small on purpose:
// the whole value of the step is that it costs a fraction of writing a page.
func PlanSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":     "object",
		"required": []interface{}{"root_page_id", "pages"},
		"properties": map[string]interface{}{
			"root_page_id": map[string]interface{}{
				"type":        "string",
				"description": "The id of the page the user is editing. It must appear in pages with role \"root\".",
			},
			"summary": map[string]interface{}{
				"type":        "string",
				"description": "One sentence describing the design you are about to build.",
			},
			"pages": map[string]interface{}{
				"type":        "array",
				"description": "Every page this design takes, the root first.",
				"items": map[string]interface{}{
					"type":     "object",
					"required": []interface{}{"page_id", "role", "purpose"},
					"properties": map[string]interface{}{
						"page_id": map[string]interface{}{"type": "string", "description": "A stable id for this page."},
						"role": map[string]interface{}{
							"type":        "string",
							"enum":        []interface{}{PlanRoleRoot, PlanRoleCard, PlanRoleDetail},
							"description": "root: the page being edited. card: one record drawn inside a board, tile or list. detail: a form or detail view mounted in a panel.",
						},
						"purpose":     map[string]interface{}{"type": "string", "description": "One line: what this page is for."},
						"entity_name": map[string]interface{}{"type": "string", "description": "The entity this page reads, if any."},
						"mounted_at": map[string]interface{}{
							"type":        "string",
							"description": "The id of the component on the ROOT page that mounts this page. Leave empty on the root itself.",
						},
						"mount_property": map[string]interface{}{
							"type":        "string",
							"description": "The property on that component that points at this page: \"card_page_id\" for a grid in board view or a tile, \"page\" for a page_ref.",
						},
						"skeleton": map[string]interface{}{
							"type":        "string",
							"description": "One line describing the layout, e.g. \"two-pane row: list 35%, detail 100%\".",
						},
					},
				},
			},
		},
	}
}

// ParsePlan reads a plan from a structured_output payload.
func ParsePlan(payload map[string]interface{}) (PagePlan, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return PagePlan{}, err
	}
	var plan PagePlan
	if err := json.Unmarshal(raw, &plan); err != nil {
		return PagePlan{}, fmt.Errorf("the page plan could not be read: %w", err)
	}
	return plan, nil
}

// ValidatePlan checks a plan before anything is built on it.
//
// Every check here is one that would otherwise have been discovered after a page
// was written - which is the whole point. Catching "this board has no card page"
// in a plan costs one cheap retry of a twenty-line answer; catching it after
// generation costs a page.
func ValidatePlan(plan PagePlan) []catalog.Issue {
	issues := []catalog.Issue{}

	if strings.TrimSpace(plan.RootPageId) == "" {
		issues = append(issues, catalog.Issue{Path: "root_page_id", Code: catalog.CodePlanNoRoot,
			Message: "the plan has no root_page_id - name the page the user is editing"})
	}
	if len(plan.Pages) == 0 {
		issues = append(issues, catalog.Issue{Path: "pages", Code: catalog.CodePlanNoPages,
			Message: "the plan lists no pages - every design has at least the page being edited"})
		return issues
	}
	if _, ok := plan.Root(); !ok && plan.RootPageId != "" {
		issues = append(issues, catalog.Issue{Path: "pages", Code: catalog.CodePlanNoRoot,
			Message: fmt.Sprintf("root_page_id is %q but no page in \"pages\" has that id", plan.RootPageId)})
	}

	seen := map[string]bool{}
	for i, page := range plan.Pages {
		path := fmt.Sprintf("pages[%d]", i)
		if strings.TrimSpace(page.PageId) == "" {
			issues = append(issues, catalog.Issue{Path: path, Code: catalog.CodePlanPageNoId,
				Message: "every planned page needs a page_id"})
			continue
		}
		if seen[page.PageId] {
			issues = append(issues, catalog.Issue{Path: path, Code: catalog.CodePlanDuplicateId, ComponentId: page.PageId,
				Message: fmt.Sprintf("two planned pages share the id %q", page.PageId)})
		}
		seen[page.PageId] = true

		if page.PageId == plan.RootPageId {
			continue
		}
		if strings.TrimSpace(page.MountedAt) == "" {
			issues = append(issues, catalog.Issue{Path: path, Code: catalog.CodePlanPageUnmounted, ComponentId: page.PageId,
				Message: fmt.Sprintf("page %q is not mounted anywhere - say which component on the root page mounts it, or drop the page", page.PageId)})
		}
		if strings.TrimSpace(page.MountProperty) == "" {
			issues = append(issues, catalog.Issue{Path: path, Code: catalog.CodePlanPageUnmounted, ComponentId: page.PageId,
				Message: fmt.Sprintf("page %q does not say which property mounts it - \"card_page_id\" for a grid in board view or a tile, \"page\" for a page_ref", page.PageId)})
		}
	}
	return issues
}

// PlanPrompt is what the planning step is asked.
func PlanPrompt() string {
	return `Before you build anything, decide how many pages this design takes.

Most requests are one page. Some are not, and those are the ones that go wrong when they are treated as one:

- A grid in BOARD view draws every record as a card. That card is a page you author, mounted through the grid's
  "card_page_id". A board without one falls back to a built-in card the user cannot change - so if this design has a
  board, it has at least two pages.
- A tile that opens a record mounts that record's page through "card_page_id".
- A detail panel, a side panel, or a form that opens beside a list is its own page, mounted through a page_ref's "page".
- A layout that repeats a record - a list of identical blocks - is one page used many times, not many copies.

Use the lookups available to you first: read the entity metadata so each page names a real entity, and list the existing
pages if the user referred to one. Then answer with the plan.

Keep it to the pages the design genuinely needs. An extra page is a page the user has to save and maintain.`
}

// RebaseRoot holds the plan to the page id the request already decided on. The
// client names the page it is editing; a plan that invented a different id would
// have every mount pointing at a page that is not the one on screen.
func RebaseRoot(plan PagePlan, rootPageId string) PagePlan {
	if rootPageId == "" || plan.RootPageId == rootPageId {
		return plan
	}
	old := plan.RootPageId
	plan.RootPageId = rootPageId
	for i, page := range plan.Pages {
		if page.PageId == old {
			plan.Pages[i].PageId = rootPageId
		}
	}
	return plan
}

// PlanCommitment tells the root generation what was decided, and - the part that
// makes the split work - that the pages it mounts are being written elsewhere.
//
// Without the second half the model helpfully writes them too, and the design
// ends up with two versions of the same card: the one it inlined and the one
// built from the plan.
func PlanCommitment(plan PagePlan) string {
	nested := plan.Nested()
	var b strings.Builder
	b.WriteString("============================================================\n")
	b.WriteString("THE DESIGN HAS ALREADY BEEN PLANNED\n")
	b.WriteString("============================================================\n")
	if plan.Summary != "" {
		fmt.Fprintf(&b, "%s\n\n", plan.Summary)
	}
	fmt.Fprintf(&b, "You are building the ROOT page, id %q.\n", plan.RootPageId)
	if root, ok := plan.Root(); ok {
		if root.Purpose != "" {
			fmt.Fprintf(&b, "Its purpose: %s\n", root.Purpose)
		}
		if root.Skeleton != "" {
			fmt.Fprintf(&b, "Its layout: %s\n", root.Skeleton)
		}
		if root.Entity != "" {
			fmt.Fprintf(&b, "It reads entity: %s\n", root.Entity)
		}
	}

	if len(nested) == 0 {
		b.WriteString("\nThis design is a single page. Do not emit anything in \"pages\".\n")
		return b.String()
	}

	fmt.Fprintf(&b, "\nThis design also takes %d other page(s). THEY ARE BEING BUILT SEPARATELY, so do NOT author them and do NOT put them in \"pages\":\n", len(nested))
	for _, page := range nested {
		fmt.Fprintf(&b, "- %q (%s): %s\n", page.PageId, page.Role, page.Purpose)
		fmt.Fprintf(&b, "  Mount it: set %s.%s = %q on the component with id %q, which you must create on this page.\n",
			page.MountedAt, page.MountProperty, page.PageId, page.MountedAt)
	}
	b.WriteString("\nYour page MUST contain every component named above, with those exact ids, carrying those mount properties. ")
	b.WriteString("A mount that points at nothing renders an empty panel, and a mount you leave out means the page that was built for it never appears.\n")
	return b.String()
}
