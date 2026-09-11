package eru_studio

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// A scoped edit is the common case: move this button, restyle the right panel,
// work on tab 1. The page it happens in is large - a real one runs to 119
// components and 119KB - while the part in play is a handful of them.
//
// Scope is how a client says which part that is. The full page still travels
// (the client holds the only copy that includes unsaved edits, and the patch is
// applied and validated against it), but only the components in scope, their
// descendants and their ancestors reach the model in full. Everything else is
// reduced to structure: id, type, children. Same page, a tenth of the tokens.

// ScopeParam is the request param carrying the edit scope.
const ScopeParam = "scope"

// skeletonKeys are what a component keeps when it is out of scope: enough to
// place a component next to it, address it, or reparent it, and nothing else.
var skeletonKeys = map[string]bool{"id": true, "type": true, "parent_id": true, "index": true}

// Scope names the part of the page an edit is allowed to touch.
type Scope struct {
	// ComponentIds are the components in play. A container brings its subtree
	// with it unless IncludeDescendants is explicitly false.
	ComponentIds []string `json:"component_ids,omitempty"`
	// IncludeDescendants defaults to true: scoping a tab means the tab's contents.
	IncludeDescendants *bool `json:"include_descendants,omitempty"`
	// AllowPageProps lets the edit change page-level keys (title, route). Off by
	// default: "restyle this panel" has no business renaming the page.
	AllowPageProps bool `json:"allow_page_props,omitempty"`
}

func (s *Scope) IsEmpty() bool {
	return s == nil || len(s.ComponentIds) == 0
}

func (s *Scope) descendants() bool {
	return s == nil || s.IncludeDescendants == nil || *s.IncludeDescendants
}

// ParseScope reads the scope param. A client that sends a bare id, a
// comma-separated list, or a JSON array instead of the object still gets the
// scoping it asked for.
func ParseScope(raw interface{}) (*Scope, error) {
	switch value := raw.(type) {
	case nil:
		return nil, nil
	case string:
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || trimmed == "{}" || trimmed == "null" {
			return nil, nil
		}
		if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
			var decoded interface{}
			if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
				return nil, fmt.Errorf("scope is not valid JSON: %w", err)
			}
			return ParseScope(decoded)
		}
		return &Scope{ComponentIds: toStringSlice(trimmed)}, nil
	case []interface{}, []string:
		ids := toStringSlice(value)
		if len(ids) == 0 {
			return nil, nil
		}
		return &Scope{ComponentIds: ids}, nil
	case map[string]interface{}:
		// component_ids may arrive as an array or as a comma-separated string;
		// normalise before decoding so the strict field type does not reject the
		// sloppier form outright.
		normalised := make(map[string]interface{}, len(value))
		for key, nested := range value {
			normalised[key] = nested
		}
		if ids, ok := normalised["component_ids"]; ok {
			normalised["component_ids"] = toStringSlice(ids)
		}
		encoded, err := json.Marshal(normalised)
		if err != nil {
			return nil, err
		}
		var scope Scope
		if err := json.Unmarshal(encoded, &scope); err != nil {
			return nil, fmt.Errorf("scope is not a scope object: %w", err)
		}
		if len(scope.ComponentIds) == 0 {
			return nil, nil
		}
		return &scope, nil
	default:
		return nil, fmt.Errorf("scope must be an object, an array of ids or a comma-separated string, got %T", raw)
	}
}

// ResolvedScope is a scope resolved against a page: which components the edit
// may change, and which ones reach the model in full.
type ResolvedScope struct {
	// Writable are the components whose content the edit may change.
	Writable map[string]bool
	// Detailed are the components shown to the model in full: the writable set
	// plus the ancestors that give them their layout context.
	Detailed map[string]bool
	// Missing are scoped ids the page does not contain. They are not an error -
	// a client may scope a component the agent is about to create - but they are
	// worth reporting.
	Missing []string
	// Omitted counts the components reduced to structure only.
	Omitted int
}

// Resolve expands a scope over a page.
func (s *Scope) Resolve(page map[string]interface{}) *ResolvedScope {
	resolved := &ResolvedScope{Writable: map[string]bool{}, Detailed: map[string]bool{}}
	if s.IsEmpty() {
		return resolved
	}

	parents := map[string]string{}
	present := map[string]bool{}
	children := map[string][]string{}
	walkComponents(page, func(component map[string]interface{}, parentId string) {
		id, _ := component["id"].(string)
		if id == "" {
			return
		}
		present[id] = true
		parents[id] = parentId
		if parentId != "" {
			children[parentId] = append(children[parentId], id)
		}
	})

	for _, id := range s.ComponentIds {
		if !present[id] {
			resolved.Missing = append(resolved.Missing, id)
			// Still writable: the edit may be creating it.
			resolved.Writable[id] = true
			continue
		}
		resolved.Writable[id] = true
		if s.descendants() {
			addSubtree(children, id, resolved.Writable)
		}
	}

	for id := range resolved.Writable {
		resolved.Detailed[id] = true
		// Ancestors are shown but not writable: the model needs the card's
		// padding and the container's direction to make a sane decision inside
		// them, without licence to restyle them.
		for parent := parents[id]; parent != ""; parent = parents[parent] {
			resolved.Detailed[parent] = true
		}
	}
	sort.Strings(resolved.Missing)
	return resolved
}

func addSubtree(children map[string][]string, id string, into map[string]bool) {
	for _, child := range children[id] {
		if into[child] {
			continue
		}
		into[child] = true
		addSubtree(children, child, into)
	}
}

// walkComponents visits every component of a page with the id of its parent.
func walkComponents(page map[string]interface{}, visit func(component map[string]interface{}, parentId string)) {
	components, _ := page["components"].([]interface{})
	var walk func(list []interface{}, parentId string)
	walk = func(list []interface{}, parentId string) {
		for _, raw := range list {
			component, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			visit(component, parentId)
			id, _ := component["id"].(string)
			if nested, ok := component["children"].([]interface{}); ok {
				walk(nested, id)
			}
		}
	}
	walk(components, "")
}

// Prune returns the page as the model should see it for a scoped edit: full
// detail where the edit is happening, structure only everywhere else.
//
// The result is for the prompt alone. The patch is still applied to, and
// validated against, the unpruned page the client sent - so a component the
// model never saw in full cannot be damaged by an edit it was not part of.
func Prune(page map[string]interface{}, resolved *ResolvedScope) map[string]interface{} {
	if page == nil || resolved == nil || len(resolved.Detailed) == 0 {
		return page
	}

	pruned := map[string]interface{}{}
	for key, value := range page {
		if key == "components" {
			continue
		}
		pruned[key] = value
	}

	omitted := 0
	var prune func(list []interface{}) []interface{}
	prune = func(list []interface{}) []interface{} {
		out := make([]interface{}, 0, len(list))
		for _, raw := range list {
			component, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			id, _ := component["id"].(string)
			copied := map[string]interface{}{}
			if resolved.Detailed[id] {
				for key, value := range component {
					if key == "children" {
						continue
					}
					copied[key] = value
				}
			} else {
				for key, value := range component {
					if skeletonKeys[key] {
						copied[key] = value
					}
				}
				omitted++
			}
			if children, ok := component["children"].([]interface{}); ok && len(children) > 0 {
				copied["children"] = prune(children)
			}
			out = append(out, copied)
		}
		return out
	}

	components, _ := page["components"].([]interface{})
	pruned["components"] = prune(components)
	resolved.Omitted = omitted
	return pruned
}

// ScopeInstructions is the note that travels with a pruned page. Without it the
// model reads a skeleton component as a component with no properties and
// helpfully "completes" it, which is how a scoped edit turns into a rewrite.
func ScopeInstructions(resolved *ResolvedScope, scope *Scope) string {
	if resolved == nil || len(resolved.Writable) == 0 {
		return ""
	}
	writable := sortedSet(resolved.Writable)

	var b strings.Builder
	b.WriteString("--- EDIT SCOPE ---\n")
	fmt.Fprintf(&b, "This prompt is about these components only: %s\n", strings.Join(writable, ", "))
	b.WriteString("Patch those, and nothing else. Every other component on the page is somebody else's -\n")
	b.WriteString("leave it out of the patch entirely.\n\n")
	if resolved.Omitted > 0 {
		fmt.Fprintf(&b, "To keep this prompt small, %d component(s) outside the scope are shown as STRUCTURE ONLY:\n", resolved.Omitted)
		b.WriteString("just their id, type and nesting. Their properties, styles and events were removed from\n")
		b.WriteString("this message - they still exist on the real page. Never re-emit one of them, and never\n")
		b.WriteString("read a missing property as an absent property.\n\n")
	}
	b.WriteString("The components in scope, and the containers they sit inside, are shown in full.\n")
	b.WriteString("You may reparent or reorder a component by patching a container's children_ids, or by\n")
	b.WriteString("setting parent_id and index on the component being moved - that is allowed even for a\n")
	b.WriteString("container outside the scope, as long as you change nothing else about it.\n")
	if !scope.IsEmpty() && !scope.AllowPageProps {
		b.WriteString("Do not change page-level keys (title, route, name, state): this edit is not about the page.\n")
	}
	if len(resolved.Missing) > 0 {
		fmt.Fprintf(&b, "Scoped ids that the page does not have yet, so presumably yours to create: %s\n", strings.Join(resolved.Missing, ", "))
	}
	b.WriteString("--- END EDIT SCOPE ---\n")
	return b.String()
}

func sortedSet(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

const scopeKey contextKey = "eru_studio_scope"

// WithScope records the resolved scope so validation can hold the patch to it.
func WithScope(ctx context.Context, resolved *ResolvedScope) context.Context {
	return context.WithValue(ctx, scopeKey, resolved)
}

// ScopeFrom returns this request's resolved scope, nil when the edit is
// unscoped.
func ScopeFrom(ctx context.Context) *ResolvedScope {
	if ctx == nil {
		return nil
	}
	resolved, _ := ctx.Value(scopeKey).(*ResolvedScope)
	return resolved
}

// reparentOnlyKeys are what an out-of-scope component may carry in a patch: its
// identity, and where its children go. Moving a component into a container that
// is not itself being edited is a legitimate scoped edit; restyling that
// container on the way past is not.
var reparentOnlyKeys = map[string]bool{"id": true, "type": true, "children_ids": true, "index": true, "parent_id": true}

// ScopeViolations reports the parts of a patch that reach outside the scope. The
// text is what the model is told, so it says what to do instead.
func ScopeViolations(patch *Patch, resolved *ResolvedScope) []string {
	if patch == nil || resolved == nil || len(resolved.Writable) == 0 {
		return nil
	}
	violations := []string{}

	for _, component := range patch.Upsert {
		id, _ := component["id"].(string)
		if id == "" || resolved.Writable[id] {
			continue
		}
		extra := []string{}
		for key := range component {
			if !reparentOnlyKeys[key] {
				extra = append(extra, key)
			}
		}
		if len(extra) == 0 {
			continue
		}
		sort.Strings(extra)
		violations = append(violations, fmt.Sprintf(
			"patch.upsert changes %q, which this edit is not about - it may only carry children_ids/index/parent_id to move something, not %s. Drop it from the patch.",
			id, strings.Join(extra, ", ")))
	}

	for _, id := range patch.Delete {
		if !resolved.Writable[id] {
			violations = append(violations, fmt.Sprintf(
				"patch.delete removes %q, which is outside this edit's scope. Only %s may be deleted.",
				id, strings.Join(sortedSet(resolved.Writable), ", ")))
		}
	}

	return violations
}
