package eru_studio

import (
	"fmt"
	"sort"
	"strings"

	catalog "github.com/eru-tech/eru/eru-ai/agents/eru_studio/catalog"
)

// A page is not always one page. A repeated row needs a template page, a side
// panel or a popup is a page mounted by a page_ref, a board card and a tile card
// are pages, and a nested array is a page looped over a record set. The renderer
// mounts them by id; the store saves them separately; the user opens each in its
// own tab to accept it.
//
// So an edit can produce several pages, and each one has to come back as its own
// unit of persistence - not folded into the page that mounts it.

// PageMount is a property through which one component mounts another page.
type PageMount struct {
	// Property is the property key holding the mounted page's id.
	Property string
	// Purpose says what the mounted page is for, which is what the agent has to
	// reason about when it decides whether it needs a nested page at all.
	Purpose string
	// Note is the guidance the prompt carries for this mount.
	Note string
}

// pageMounts is the map of component type -> the properties that mount a page.
// The property names are checked against the generated catalog by a test, so a
// rename in the library breaks the build rather than silently unteaching the
// agent how to nest.
var pageMounts = map[string][]PageMount{
	"page_ref": {{
		Property: "page",
		Purpose:  PurposeFromDisplay,
		Note:     "the page this page_ref mounts - inline, in a popup, in a side panel, or once per record when it loops",
	}},
	"grid": {{
		Property: "card_page_id",
		Purpose:  PurposeBoardCard,
		Note:     "the page used as the layout of one card in board view; blank falls back to the built-in card",
	}},
	"tile": {{
		Property: "card_page_id",
		Purpose:  PurposeTileCard,
		Note:     "the page used as this tile's card layout",
	}},
	"nav_outlet": {{
		Property: "default_page",
		Purpose:  PurposeRoutedPage,
		Note:     "the page shown when no route is selected; routed pages are whole pages, not nested ones",
	}},
}

// What a mounted page is for. The agent picks a purpose before it picks a
// component, because the purpose is what forces a separate page to exist.
const (
	PurposeFromDisplay  = "from_display_type"
	PurposeLoopTemplate = "loop_template"
	PurposeSidePanel    = "side_panel"
	PurposePopup        = "popup"
	PurposeInline       = "inline"
	PurposeBoardCard    = "board_card"
	PurposeTileCard     = "tile_card"
	PurposeRoutedPage   = "routed_page"
)

// PersistUserAction is the only persistence mode the agent offers: the client
// saves the page when the user accepts it. The agent never writes to the store -
// its output is a proposal, and a proposal that has already overwritten the
// user's page is not one.
const PersistUserAction = "user_action"

// NestedPage is one page an edit produced besides the root page.
type NestedPage struct {
	PageId string `json:"page_id"`
	Name   string `json:"name,omitempty"`
	// Purpose is why this page exists as a page.
	Purpose string `json:"purpose,omitempty"`
	// ParentPageId is the page that mounts it, and MountedAt / MountProperty the
	// component and property that do the mounting - what the client needs to
	// open the right tab and to check the wiring.
	ParentPageId  string `json:"parent_page_id,omitempty"`
	MountedAt     string `json:"mounted_at,omitempty"`
	MountProperty string `json:"mount_property,omitempty"`
	// IsNew distinguishes a page the edit created from one it changed.
	IsNew bool `json:"is_new,omitempty"`
	// Persist is always PersistUserAction: the client owns the save.
	Persist  string                 `json:"persist,omitempty"`
	Revision string                 `json:"revision,omitempty"`
	Page     map[string]interface{} `json:"page,omitempty"`
	Warnings []string               `json:"warnings,omitempty"`
}

// PageManifestEntry is how the root page's envelope advertises a nested page:
// enough to find it and to know why it is there, without repeating it.
type PageManifestEntry struct {
	PageId        string `json:"page_id"`
	Name          string `json:"name,omitempty"`
	Purpose       string `json:"purpose,omitempty"`
	MountedAt     string `json:"mounted_at,omitempty"`
	MountProperty string `json:"mount_property,omitempty"`
	IsNew         bool   `json:"is_new,omitempty"`
	Revision      string `json:"revision,omitempty"`
	Persist       string `json:"persist,omitempty"`
}

// Manifest summarises the nested pages of an edit.
func Manifest(pages []*NestedPage) []PageManifestEntry {
	if len(pages) == 0 {
		return nil
	}
	out := make([]PageManifestEntry, 0, len(pages))
	for _, page := range pages {
		out = append(out, PageManifestEntry{
			PageId:        page.PageId,
			Name:          page.Name,
			Purpose:       page.Purpose,
			MountedAt:     page.MountedAt,
			MountProperty: page.MountProperty,
			IsNew:         page.IsNew,
			Revision:      page.Revision,
			Persist:       page.Persist,
		})
	}
	return out
}

// MergeNestedPages combines the pages the model declared with the ones it
// inlined under their hosts. A declared page wins: it carries the purpose and
// the mount the model meant, while an inlined one was only inferred.
func MergeNestedPages(declared []*NestedPage, inlined []*NestedPage, rootPageId string) []*NestedPage {
	out := make([]*NestedPage, 0, len(declared)+len(inlined))
	seen := map[string]bool{}
	for _, page := range declared {
		if page == nil || page.PageId == "" || seen[page.PageId] {
			continue
		}
		seen[page.PageId] = true
		if page.ParentPageId == "" {
			page.ParentPageId = rootPageId
		}
		if page.Persist == "" {
			page.Persist = PersistUserAction
		}
		if page.Revision == "" {
			page.Revision = Revision(page.Page)
		}
		out = append(out, page)
	}
	for _, page := range inlined {
		if page == nil || page.PageId == "" || seen[page.PageId] {
			continue
		}
		seen[page.PageId] = true
		if page.Revision == "" {
			page.Revision = Revision(page.Page)
		}
		page.Warnings = append(page.Warnings,
			"this page arrived inlined under the component that mounts it; it is returned as its own page because that is how it is saved")
		out = append(out, page)
	}
	return out
}

// MountsOf returns the page-mounting properties of a component type.
func MountsOf(componentType string) []PageMount {
	return pageMounts[componentType]
}

// IsPageHost reports whether a component type mounts a page, and so may legally
// carry an inlined nested page under "children".
func IsPageHost(componentType string) bool {
	return len(pageMounts[componentType]) > 0
}

// PageHostTypes lists the component types that mount a page.
func PageHostTypes() []string {
	out := make([]string, 0, len(pageMounts))
	for componentType := range pageMounts {
		out = append(out, componentType)
	}
	sort.Strings(out)
	return out
}

// Mount is one component's reference to another page, found on a page.
type Mount struct {
	ComponentId   string
	ComponentType string
	Property      string
	PageId        string
	Purpose       string
	// Inlined are the components the client sent under the host, which are the
	// mounted page's own components rendered in place rather than the host's
	// children.
	Inlined []interface{}
}

// FindMounts walks a page and returns every reference it makes to another page.
func FindMounts(page map[string]interface{}) []Mount {
	mounts := []Mount{}
	walkComponents(page, func(component map[string]interface{}, _ string) {
		componentType, _ := component["type"].(string)
		for _, mount := range pageMounts[componentType] {
			id, _ := component["id"].(string)
			properties := baseProperties(component)
			mountedId, _ := properties[mount.Property].(string)
			purpose := mount.Purpose
			if purpose == PurposeFromDisplay {
				purpose = purposeFromProperties(properties)
			}
			inlined, _ := component["children"].([]interface{})
			mounts = append(mounts, Mount{
				ComponentId:   id,
				ComponentType: componentType,
				Property:      mount.Property,
				PageId:        strings.TrimSpace(mountedId),
				Purpose:       purpose,
				Inlined:       inlined,
			})
		}
	})
	return mounts
}

// purposeFromProperties reads a page_ref's own properties to say what the page
// it mounts is for.
func purposeFromProperties(properties map[string]interface{}) string {
	if loop, _ := properties["loop_source"].(string); strings.TrimSpace(loop) != "" && loop != "none" {
		return PurposeLoopTemplate
	}
	if nesting, _ := properties["nesting_type"].(string); nesting == "array" || nesting == "nested_array" {
		return PurposeLoopTemplate
	}
	switch display, _ := properties["display_type"].(string); display {
	case "side_panel":
		return PurposeSidePanel
	case "popup":
		return PurposePopup
	case "inline":
		return PurposeInline
	}
	return PurposeInline
}

func baseProperties(component map[string]interface{}) map[string]interface{} {
	properties, _ := component["properties"].(map[string]interface{})
	if properties == nil {
		return map[string]interface{}{}
	}
	base, _ := properties["base"].(map[string]interface{})
	if base == nil {
		return map[string]interface{}{}
	}
	return base
}

// SplitInlinedPages separates a page the client sent into the root page and the
// nested pages inlined under their hosts.
//
// The client renders a nested page in place, so it arrives as "children" of the
// page_ref that mounts it. Those components belong to another page: they are
// saved separately, and an edit to them has to come back as an edit to that
// page. Splitting them out is what makes that possible.
func SplitInlinedPages(page map[string]interface{}) (root map[string]interface{}, nested []*NestedPage) {
	if page == nil {
		return nil, nil
	}
	rootId, _ := page["id"].(string)

	root = map[string]interface{}{}
	for key, value := range page {
		if key == "components" {
			continue
		}
		root[key] = value
	}

	var strip func(list []interface{}) []interface{}
	strip = func(list []interface{}) []interface{} {
		out := make([]interface{}, 0, len(list))
		for _, raw := range list {
			component, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			componentType, _ := component["type"].(string)
			copied := map[string]interface{}{}
			for key, value := range component {
				if key == "children" {
					continue
				}
				copied[key] = value
			}

			children, _ := component["children"].([]interface{})
			if IsPageHost(componentType) {
				// The host's "children" are the mounted page's components.
				if len(children) > 0 {
					properties := baseProperties(component)
					mountedId := ""
					for _, mount := range pageMounts[componentType] {
						if id, _ := properties[mount.Property].(string); strings.TrimSpace(id) != "" {
							mountedId = strings.TrimSpace(id)
							break
						}
					}
					hostId, _ := component["id"].(string)
					if mountedId == "" {
						mountedId = hostId + "_page"
					}
					nested = append(nested, &NestedPage{
						PageId:       mountedId,
						ParentPageId: rootId,
						MountedAt:    hostId,
						Purpose:      purposeFromProperties(properties),
						Persist:      PersistUserAction,
						Page: map[string]interface{}{
							"id":             mountedId,
							"parent_page_id": rootId,
							"components":     children,
						},
					})
				}
			} else if len(children) > 0 {
				copied["children"] = strip(children)
			}
			out = append(out, copied)
		}
		return out
	}

	components, _ := page["components"].([]interface{})
	root["components"] = strip(components)
	return root, nested
}

// InlineNestedPages puts each nested page's components back under the host that
// mounts it, so a client can render the whole thing in one pass without
// resolving mounts itself. The nested pages still travel separately: this is a
// rendering convenience, not the unit of persistence.
func InlineNestedPages(page map[string]interface{}, nested []*NestedPage) map[string]interface{} {
	if page == nil || len(nested) == 0 {
		return page
	}
	byHost := map[string]*NestedPage{}
	byPageId := map[string]*NestedPage{}
	for _, nestedPage := range nested {
		if nestedPage.MountedAt != "" {
			byHost[nestedPage.MountedAt] = nestedPage
		}
		if nestedPage.PageId != "" {
			byPageId[nestedPage.PageId] = nestedPage
		}
	}

	var inline func(list []interface{}) []interface{}
	inline = func(list []interface{}) []interface{} {
		out := make([]interface{}, 0, len(list))
		for _, raw := range list {
			component, ok := raw.(map[string]interface{})
			if !ok {
				out = append(out, raw)
				continue
			}
			copied := map[string]interface{}{}
			for key, value := range component {
				copied[key] = value
			}
			componentType, _ := copied["type"].(string)
			hostId, _ := copied["id"].(string)

			if IsPageHost(componentType) {
				match := byHost[hostId]
				if match == nil {
					properties := baseProperties(copied)
					for _, mount := range pageMounts[componentType] {
						if id, _ := properties[mount.Property].(string); id != "" {
							match = byPageId[strings.TrimSpace(id)]
							break
						}
					}
				}
				if match != nil {
					if components, ok := match.Page["components"].([]interface{}); ok && len(components) > 0 {
						copied["children"] = components
					}
				}
			} else if children, ok := copied["children"].([]interface{}); ok && len(children) > 0 {
				copied["children"] = inline(children)
			}
			out = append(out, copied)
		}
		return out
	}

	withNested := map[string]interface{}{}
	for key, value := range page {
		withNested[key] = value
	}
	components, _ := page["components"].([]interface{})
	withNested["components"] = inline(components)
	return withNested
}

// ValidateMounts checks that the pages an edit produced and the components that
// mount them agree. A page_ref whose page property is blank, or points at a page
// nobody sent, renders as an empty panel - the failure mode that looks like the
// agent did nothing.
func ValidateMounts(root map[string]interface{}, nested []*NestedPage, knownPageIds map[string]bool) []catalog.Issue {
	issues := []catalog.Issue{}

	sent := map[string]*NestedPage{}
	for _, page := range nested {
		if page.PageId != "" {
			sent[page.PageId] = page
		}
	}

	mountedIds := map[string]bool{}
	for _, mount := range FindMounts(root) {
		path := fmt.Sprintf("components (id %q)", mount.ComponentId)
		if mount.PageId == "" {
			if mount.ComponentType == "page_ref" {
				issues = append(issues, catalog.Issue{
					Path:    path,
					Message: fmt.Sprintf("page_ref has no %q - it will render an empty panel. Emit the page it should mount in \"pages\" and set %s to that page's id", mount.Property, mount.Property),
				})
			}
			continue
		}
		mountedIds[mount.PageId] = true
		if sent[mount.PageId] == nil && !knownPageIds[mount.PageId] {
			issues = append(issues, catalog.Issue{
				Path: path,
				Message: fmt.Sprintf("%s.%s points at page %q, which is neither in \"pages\" nor an existing page - either emit that page in \"pages\" or point at a page that exists",
					mount.ComponentType, mount.Property, mount.PageId),
			})
		}
	}

	for _, page := range nested {
		if page.PageId == "" {
			issues = append(issues, catalog.Issue{Path: "pages", Message: "a nested page was sent without an \"id\""})
			continue
		}
		if mountedIds[page.PageId] {
			continue
		}
		// A page nothing mounts is dead weight the user would have to save for
		// no reason.
		issues = append(issues, catalog.Issue{
			Path: fmt.Sprintf("pages (id %q)", page.PageId),
			Message: fmt.Sprintf("nothing on the page mounts %q. Set a page_ref's \"page\" (or a grid's \"card_page_id\") to it, or drop the page",
				page.PageId),
		})
	}

	return issues
}

// NestingGuidance is the prompt section that teaches the agent when a design
// needs more than one page. It is the gap that matters most in practice: an
// agent that does not reach for a nested page hard-codes three copies of a row
// instead of one template, or renders a "side panel" inline, and the result
// cannot do what the user asked for however good the components are.
func NestingGuidance() string {
	var b strings.Builder
	b.WriteString("MORE THAN ONE PAGE (nested pages)\n")
	b.WriteString("Some designs cannot be one page. A component mounts another page by id, and that page is\n")
	b.WriteString("authored, saved and versioned on its own. These are the mount points:\n\n")
	b.WriteString(MountReference())
	b.WriteString("\n\nA SEPARATE PAGE IS REQUIRED - not a preference - when:\n")
	b.WriteString("- Something repeats once per record: a list of record cards, the line items of an invoice,\n")
	b.WriteString("  a nested array. The repeating unit is a TEMPLATE page, mounted by one page_ref with\n")
	b.WriteString("  loop_source (or nesting_type array / nested_array) set. NEVER hard-code N copies of the\n")
	b.WriteString("  same block: the record count is not known when you design the page.\n")
	b.WriteString("- The user asks for a side panel or a drawer: page_ref with display_type \"side_panel\".\n")
	b.WriteString("- The user asks for a popup, modal or dialog: page_ref with display_type \"popup\".\n")
	b.WriteString("- A grid needs a board / kanban view: the card layout is a page, set as the grid's\n")
	b.WriteString("  card_page_id. The built-in card is the only alternative, and it is not designable.\n")
	b.WriteString("- A tile needs a custom card layout: a page, set as the tile's card_page_id.\n")
	b.WriteString("- A row opens a detail view: a page mounted by a page_ref (popup or side_panel), wired\n")
	b.WriteString("  through the parent page's master_detail_config.\n")
	b.WriteString("- The same layout appears in several places: author it once and mount it from each.\n")
	b.WriteString("\nA SEPARATE PAGE IS WRONG when:\n")
	b.WriteString("- It is just a section of this page: use flex_container, grid_container or card.\n")
	b.WriteString("- It is a tab whose content is used nowhere else: use the tabs container's children.\n")
	b.WriteString("- It is a destination the user navigates to: that is a page of its own, reached with\n")
	b.WriteString("  navigate-to-page - not mounted inside this one.\n")
	b.WriteString("\nRULES FOR EVERY MOUNT\n")
	b.WriteString("- A page_ref with a blank \"page\" renders an empty panel. Every mount must name a page.\n")
	b.WriteString("- The mounted page is NOT the host's children. A page host has no children of its own:\n")
	b.WriteString("  its contents live on the page it mounts. When the message you were given shows\n")
	b.WriteString("  components underneath a page_ref, those are that page's components rendered in place -\n")
	b.WriteString("  to change them, emit that page, not the host's children.\n")
	b.WriteString("- A nested page is a page: same EruPage shape, its own id and name, its own components\n")
	b.WriteString("  and styles. Keep it small - it is a template or a panel, not a second application.\n")
	return b.String()
}

// MountReference renders the generated part of the nesting guidance: which
// component types mount a page, and through which property. Generated so it
// cannot drift from the library.
func MountReference() string {
	var b strings.Builder
	for _, componentType := range PageHostTypes() {
		for _, mount := range pageMounts[componentType] {
			fmt.Fprintf(&b, "  %s.properties.base.%s - %s\n", componentType, mount.Property, mount.Note)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}
