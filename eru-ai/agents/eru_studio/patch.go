// Package eru_studio carries the page protocol the Eru Studio agent speaks: a
// full EruPage, or a component-level patch against the page the client already
// holds.
//
// A patch exists because a full page is the wrong unit of work for an edit. Ask
// for a different button colour and a full-page agent regenerates every
// component, spends the tokens to do it, and risks dropping something on the way.
// A patch names the components that changed and nothing else, and - because its
// components are a flat list addressed by id, the way A2UI models a surface -
// each one can be rendered the moment it arrives instead of after the whole page
// has been generated.
package eru_studio

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// ProtocolVersion is the version of the response envelope. It changes only when
// a consumer would have to change with it.
const ProtocolVersion = "1.0"

const (
	// ModeFull carries a complete EruPage, as the agent has always answered.
	ModeFull = "full"
	// ModePatch carries the changed components only, against a base revision.
	ModePatch = "patch"
	// ModeAuto asks the agent to patch when it was handed an existing page and
	// to send a full page when it is building one from scratch.
	ModeAuto = "auto"
)

// pageRootArrays are the EruPage keys a patch never carries wholesale: the
// components and the state have their own targeted patch sections.
var pageRootArrays = map[string]bool{"components": true, "state": true}

// StatePatch changes the page's declared state variables by key.
type StatePatch struct {
	Upsert []map[string]interface{} `json:"upsert,omitempty"`
	Delete []string                 `json:"delete,omitempty"`
}

// Patch is what the model emits when it is editing rather than authoring: the
// components that changed, the ones that went away, and any page-level keys that
// moved with them.
type Patch struct {
	PageId string `json:"page_id,omitempty"`
	// PageProps carries changed EruPage root keys (title, route, entity_name,
	// display_mode, ...). components and state are not accepted here.
	PageProps map[string]interface{} `json:"page_props,omitempty"`
	// Upsert is a flat list of components. A component that already exists is
	// merged into; one that does not is created. Parenthood comes from
	// children_ids on the parent, or from parent_id + index on the child.
	Upsert []map[string]interface{} `json:"upsert,omitempty"`
	// Delete removes a component and everything under it.
	Delete []string    `json:"delete,omitempty"`
	State  *StatePatch `json:"state,omitempty"`
}

// IsEmpty reports a patch that would change nothing, which is a bug in the
// agent's output rather than a legitimate no-op.
func (p *Patch) IsEmpty() bool {
	if p == nil {
		return true
	}
	return len(p.PageProps) == 0 && len(p.Upsert) == 0 && len(p.Delete) == 0 &&
		(p.State == nil || (len(p.State.Upsert) == 0 && len(p.State.Delete) == 0))
}

// TouchedIds lists the components a patch writes, in the order it writes them -
// the order a progressive renderer should apply them in.
func (p *Patch) TouchedIds() []string {
	if p == nil {
		return nil
	}
	ids := make([]string, 0, len(p.Upsert))
	for _, component := range p.Upsert {
		if id, ok := component["id"].(string); ok && id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

// Envelope is the agent's answer in patch mode. It always carries the resolved
// full page as well, so a consumer that cannot apply patches can ignore the
// patch and render `page` exactly as it does today.
type Envelope struct {
	ProtocolVersion string                 `json:"protocol_version"`
	Mode            string                 `json:"mode"`
	PageId          string                 `json:"page_id,omitempty"`
	Revision        string                 `json:"revision,omitempty"`
	BaseRevision    string                 `json:"base_revision,omitempty"`
	Page            map[string]interface{} `json:"page,omitempty"`
	Patch           *Patch                 `json:"patch,omitempty"`
	// Warnings records what the patch asked for that the base page could not
	// satisfy - a delete of something that was already gone, a parent that does
	// not exist. The patch still applies; these are for the log and the client.
	Warnings []string `json:"warnings,omitempty"`
	// Scope echoes the edit scope the request asked for, so a client can assert
	// that the agent stayed inside it.
	Scope *ScopeReport `json:"scope,omitempty"`
	// Role distinguishes the page that was asked for from the nested pages the
	// edit needed. Every page comes back as its own action; this is how a client
	// tells which action is which.
	Role string `json:"role,omitempty"`
	// Pages is the manifest of nested pages this edit produced - ids, why they
	// exist and where they mount. Each one also arrives as its own action,
	// carrying the page itself; this is the index, not the payload.
	Pages []PageManifestEntry `json:"pages,omitempty"`
	// Persist says who saves this page. Always "user_action": the agent proposes,
	// the user accepts, the client writes.
	Persist string `json:"persist,omitempty"`
	// MountedAt / MountProperty / ParentPageId are set on a nested page's
	// envelope: the component that mounts it, and the page that component is on.
	ParentPageId  string `json:"parent_page_id,omitempty"`
	MountedAt     string `json:"mounted_at,omitempty"`
	MountProperty string `json:"mount_property,omitempty"`
	Purpose       string `json:"purpose,omitempty"`
	IsNew         bool   `json:"is_new,omitempty"`
}

// Roles an envelope can carry.
const (
	// RoleRoot is the page the request was about.
	RoleRoot = "root"
	// RoleNested is a page the edit needed: a row template, a side panel, a
	// board card. It is saved separately, so the user accepts it separately.
	RoleNested = "nested"
)

// ScopeReport is what the envelope says about a scoped edit.
type ScopeReport struct {
	// ComponentIds are the components the edit was allowed to change.
	ComponentIds []string `json:"component_ids"`
	// OmittedFromPrompt counts the components that reached the model as
	// structure only - useful when working out why an edit did what it did.
	OmittedFromPrompt int `json:"omitted_from_prompt,omitempty"`
}

// ReportScope turns a resolved scope into the envelope's view of it.
func ReportScope(resolved *ResolvedScope) *ScopeReport {
	if resolved == nil || len(resolved.Writable) == 0 {
		return nil
	}
	return &ScopeReport{
		ComponentIds:      sortedSet(resolved.Writable),
		OmittedFromPrompt: resolved.Omitted,
	}
}

// node is one component while a page is held flat.
type node struct {
	component map[string]interface{}
	parentId  string
	order     int
	children  []string
}

// flatPage is a page held as an adjacency list: the shape a patch addresses and
// the shape a progressive renderer wants.
type flatPage struct {
	page  map[string]interface{}
	nodes map[string]*node
	roots []string
	next  int
}

// Flatten turns a nested EruPage into an adjacency list. The page map it returns
// shares nothing with the input: callers mutate it freely.
func Flatten(page map[string]interface{}) *flatPage {
	flat := &flatPage{
		page:  map[string]interface{}{},
		nodes: map[string]*node{},
	}
	for key, value := range page {
		if key == "components" {
			continue
		}
		flat.page[key] = deepCopyValue(value)
	}
	components, _ := page["components"].([]interface{})
	for _, raw := range components {
		if id := flat.addNode(raw, ""); id != "" {
			flat.roots = append(flat.roots, id)
		}
	}
	return flat
}

// addNode copies one component and its subtree into the flat map, returning its
// id. A component without a usable id is dropped: it could not be addressed by a
// patch, and the renderer cannot key it either.
func (f *flatPage) addNode(raw interface{}, parentId string) string {
	component, ok := raw.(map[string]interface{})
	if !ok {
		return ""
	}
	id, _ := component["id"].(string)
	if id == "" {
		return ""
	}

	copied := map[string]interface{}{}
	var children []interface{}
	for key, value := range component {
		if key == "children" {
			children, _ = value.([]interface{})
			continue
		}
		copied[key] = deepCopyValue(value)
	}

	entry := &node{component: copied, parentId: parentId, order: f.next}
	f.next++
	f.nodes[id] = entry
	for _, child := range children {
		if childId := f.addNode(child, id); childId != "" {
			entry.children = append(entry.children, childId)
		}
	}
	return id
}

// Nest rebuilds the nested EruPage a renderer consumes.
func (f *flatPage) Nest() map[string]interface{} {
	page := map[string]interface{}{}
	for key, value := range f.page {
		page[key] = value
	}
	components := make([]interface{}, 0, len(f.roots))
	for _, id := range f.roots {
		if built := f.nest(id, map[string]bool{}); built != nil {
			components = append(components, built)
		}
	}
	page["components"] = components
	return page
}

func (f *flatPage) nest(id string, seen map[string]bool) map[string]interface{} {
	entry, ok := f.nodes[id]
	if !ok || seen[id] {
		return nil
	}
	seen[id] = true

	out := map[string]interface{}{}
	for key, value := range entry.component {
		out[key] = value
	}
	if entry.parentId != "" {
		out["parent_id"] = entry.parentId
	} else {
		delete(out, "parent_id")
	}
	if len(entry.children) > 0 {
		children := make([]interface{}, 0, len(entry.children))
		for _, childId := range entry.children {
			if built := f.nest(childId, seen); built != nil {
				children = append(children, built)
			}
		}
		out["children"] = children
	}
	return out
}

// detach removes an id from whatever child list currently holds it.
func (f *flatPage) detach(id string) {
	entry, ok := f.nodes[id]
	if !ok {
		return
	}
	if entry.parentId == "" {
		f.roots = removeString(f.roots, id)
		return
	}
	if parent, ok := f.nodes[entry.parentId]; ok {
		parent.children = removeString(parent.children, id)
	}
}

// attach puts id under parentId at index, moving it off its previous parent.
// index < 0 appends.
func (f *flatPage) attach(id, parentId string, index int) {
	entry, ok := f.nodes[id]
	if !ok {
		return
	}
	f.detach(id)
	entry.parentId = parentId
	if parentId == "" {
		f.roots = insertString(f.roots, id, index)
		return
	}
	parent, ok := f.nodes[parentId]
	if !ok {
		// A parent the page does not have: keep the component addressable at the
		// root rather than losing it.
		entry.parentId = ""
		f.roots = insertString(f.roots, id, index)
		return
	}
	parent.children = insertString(parent.children, id, index)
}

// remove deletes a component and its whole subtree.
func (f *flatPage) remove(id string) bool {
	entry, ok := f.nodes[id]
	if !ok {
		return false
	}
	f.detach(id)
	for _, childId := range append([]string{}, entry.children...) {
		f.remove(childId)
	}
	delete(f.nodes, id)
	return true
}

// ApplyPatch resolves a patch against the page the client currently holds and
// returns the full page that results. base may be nil, which is how a patch that
// authors a page from scratch is applied.
func ApplyPatch(base map[string]interface{}, patch *Patch) (map[string]interface{}, []string, error) {
	if patch == nil {
		return nil, nil, fmt.Errorf("no patch to apply")
	}
	warnings := []string{}

	flat := Flatten(base)
	if flat.page == nil {
		flat.page = map[string]interface{}{}
	}
	if patch.PageId != "" {
		flat.page["id"] = patch.PageId
	}
	for key, value := range patch.PageProps {
		if pageRootArrays[key] {
			warnings = append(warnings, fmt.Sprintf("page_props.%s was ignored: %s has its own patch section", key, key))
			continue
		}
		flat.page[key] = deepCopyValue(value)
	}

	// Upserts first, so a parent named by children_ids can be created in the same
	// patch as its children whatever order they arrive in.
	type placement struct {
		id       string
		parentId string
		index    int
		explicit bool
	}
	placements := []placement{}
	childLists := map[string][]string{}

	for _, raw := range patch.Upsert {
		id, _ := raw["id"].(string)
		if id == "" {
			warnings = append(warnings, "an upsert entry without an \"id\" was dropped")
			continue
		}
		incoming := map[string]interface{}{}
		var declaredChildren []string
		parentId := ""
		index := -1
		hasParent := false
		for key, value := range raw {
			switch key {
			case "children_ids":
				declaredChildren = toStringSlice(value)
			case "children":
				// A nested subtree inside a patch is still accepted: it is just a
				// shorthand for the parent plus its descendants.
				if nested, ok := value.([]interface{}); ok {
					for _, child := range nested {
						childId := flat.addNode(child, id)
						if childId != "" {
							declaredChildren = append(declaredChildren, childId)
						}
					}
				}
			case "parent_id":
				parentId, _ = value.(string)
				hasParent = true
			case "index":
				index = toInt(value, -1)
			default:
				incoming[key] = deepCopyValue(value)
			}
		}

		if existing, ok := flat.nodes[id]; ok {
			mergeInto(existing.component, incoming)
		} else {
			entry := &node{component: incoming, order: flat.next}
			flat.next++
			flat.nodes[id] = entry
			// Until something claims it, a new component sits at the root; the
			// placement pass below moves it.
			flat.roots = append(flat.roots, id)
		}
		if declaredChildren != nil {
			childLists[id] = declaredChildren
		}
		if hasParent || index >= 0 {
			placements = append(placements, placement{id: id, parentId: parentId, index: index, explicit: hasParent})
		}
	}

	// children_ids is authoritative for that parent: it replaces the child list.
	parents := make([]string, 0, len(childLists))
	for parentId := range childLists {
		parents = append(parents, parentId)
	}
	sort.Strings(parents)
	for _, parentId := range parents {
		parent, ok := flat.nodes[parentId]
		if !ok {
			warnings = append(warnings, fmt.Sprintf("children_ids named parent %q, which the page does not have", parentId))
			continue
		}
		for _, childId := range parent.children {
			if !contains(childLists[parentId], childId) {
				// Dropped from the list but not deleted: keep it addressable.
				if child, ok := flat.nodes[childId]; ok {
					child.parentId = ""
					flat.roots = append(flat.roots, childId)
				}
			}
		}
		parent.children = nil
		for _, childId := range childLists[parentId] {
			if _, ok := flat.nodes[childId]; !ok {
				warnings = append(warnings, fmt.Sprintf("%q lists child %q, which no component in the patch or the page defines", parentId, childId))
				continue
			}
			flat.attach(childId, parentId, -1)
		}
	}

	for _, place := range placements {
		if _, ok := childLists[place.parentId]; ok && place.explicit {
			// The parent already declared its full child list; do not reorder it.
			continue
		}
		if place.explicit || place.index >= 0 {
			target := place.parentId
			if !place.explicit {
				if entry, ok := flat.nodes[place.id]; ok {
					target = entry.parentId
				}
			}
			flat.attach(place.id, target, place.index)
		}
	}

	for _, id := range patch.Delete {
		if !flat.remove(id) {
			warnings = append(warnings, fmt.Sprintf("delete named %q, which the page does not have", id))
		}
	}

	if patch.State != nil {
		flat.page["state"] = applyStatePatch(flat.page["state"], patch.State, &warnings)
	}

	resolved := flat.Nest()
	if len(warnings) == 0 {
		warnings = nil
	}
	return resolved, warnings, nil
}

func applyStatePatch(existing interface{}, patch *StatePatch, warnings *[]string) []interface{} {
	current, _ := existing.([]interface{})
	byKey := map[string]int{}
	out := make([]interface{}, 0, len(current))
	for _, raw := range current {
		variable, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		key, _ := variable["key"].(string)
		if key != "" {
			byKey[key] = len(out)
		}
		out = append(out, deepCopyValue(variable))
	}
	for _, raw := range patch.Upsert {
		key, _ := raw["key"].(string)
		if key == "" {
			*warnings = append(*warnings, "a state upsert without a \"key\" was dropped")
			continue
		}
		if index, ok := byKey[key]; ok {
			if variable, ok := out[index].(map[string]interface{}); ok {
				mergeInto(variable, raw)
				continue
			}
		}
		byKey[key] = len(out)
		out = append(out, deepCopyValue(raw))
	}
	for _, key := range patch.Delete {
		index, ok := byKey[key]
		if !ok {
			*warnings = append(*warnings, fmt.Sprintf("state delete named %q, which the page does not declare", key))
			continue
		}
		out = append(out[:index], out[index+1:]...)
		byKey = map[string]int{}
		for i, raw := range out {
			if variable, ok := raw.(map[string]interface{}); ok {
				if k, _ := variable["key"].(string); k != "" {
					byKey[k] = i
				}
			}
		}
	}
	return out
}

// mergeInto merges incoming over target, one level at a time so that a patch can
// set a single property without resending the rest of the bag. Arrays are
// replaced wholesale: a patch that sends events or children_ids means "these are
// now the events", not "add these".
func mergeInto(target, incoming map[string]interface{}) {
	for key, value := range incoming {
		if nested, ok := value.(map[string]interface{}); ok {
			if existing, ok := target[key].(map[string]interface{}); ok {
				mergeInto(existing, nested)
				continue
			}
		}
		target[key] = deepCopyValue(value)
	}
}

// Revision identifies the exact page a patch applies to, so a client can tell
// that its base is the one the agent patched. It is a content hash: same page,
// same revision, whoever computed it.
func Revision(page map[string]interface{}) string {
	if page == nil {
		return ""
	}
	canonical, err := json.Marshal(canonicalize(page))
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(canonical)
	return "r" + hex.EncodeToString(sum[:])[:12]
}

// canonicalize rewrites a decoded JSON value so that marshalling it is stable.
// Go already sorts map keys on marshal; this exists to strip the values that
// would otherwise make two identical pages hash differently.
func canonicalize(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(typed))
		for key, nested := range typed {
			if key == "created_at" || key == "updated_at" {
				continue
			}
			out[key] = canonicalize(nested)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(typed))
		for i, nested := range typed {
			out[i] = canonicalize(nested)
		}
		return out
	default:
		return value
	}
}

func deepCopyValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(typed))
		for key, nested := range typed {
			out[key] = deepCopyValue(nested)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(typed))
		for i, nested := range typed {
			out[i] = deepCopyValue(nested)
		}
		return out
	default:
		return value
	}
}

func toStringSlice(value interface{}) []string {
	switch typed := value.(type) {
	case []interface{}:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok && text != "" {
				out = append(out, text)
			}
		}
		return out
	case []string:
		return typed
	case string:
		out := []string{}
		for _, part := range strings.Split(typed, ",") {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				out = append(out, trimmed)
			}
		}
		return out
	default:
		return nil
	}
}

func toInt(value interface{}, fallback int) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case json.Number:
		if parsed, err := typed.Int64(); err == nil {
			return int(parsed)
		}
	}
	return fallback
}

func removeString(values []string, needle string) []string {
	for i, value := range values {
		if value == needle {
			return append(values[:i], values[i+1:]...)
		}
	}
	return values
}

func insertString(values []string, needle string, index int) []string {
	if index < 0 || index > len(values) {
		return append(values, needle)
	}
	values = append(values, "")
	copy(values[index+1:], values[index:])
	values[index] = needle
	return values
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

// ParsePatch reads the patch section out of the model's structured output.
func ParsePatch(output map[string]interface{}) (*Patch, error) {
	raw, ok := output["patch"]
	if !ok {
		return nil, fmt.Errorf("the output carries no \"patch\"")
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var patch Patch
	if err := json.Unmarshal(encoded, &patch); err != nil {
		return nil, fmt.Errorf("the \"patch\" is not a patch object: %w", err)
	}
	return &patch, nil
}
