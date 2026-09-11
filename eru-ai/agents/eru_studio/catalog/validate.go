package catalog

import (
	"fmt"
	"sort"
	"strings"
)

// Issue is one thing wrong with a page the agent produced, addressed by a JSON
// path so the retry prompt can point the model at the exact node.
type Issue struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

func (i Issue) String() string {
	if i.Path == "" {
		return i.Message
	}
	return i.Path + ": " + i.Message
}

// FormatIssues renders issues for the model. The list is capped because a
// wholesale-wrong page produces hundreds and the model only needs the shape of
// the mistake.
func FormatIssues(issues []Issue, max int) string {
	if len(issues) == 0 {
		return ""
	}
	if max <= 0 || max > len(issues) {
		max = len(issues)
	}
	var b strings.Builder
	for _, issue := range issues[:max] {
		b.WriteString("- ")
		b.WriteString(issue.String())
		b.WriteString("\n")
	}
	if len(issues) > max {
		fmt.Fprintf(&b, "- ... and %d more\n", len(issues)-max)
	}
	return strings.TrimRight(b.String(), "\n")
}

const styleKeysHint = "classes, responsive_classes, responsive_styles, custom"

var pageStyleKeys = map[string]bool{"classes": true, "responsive_classes": true, "responsive_styles": true, "custom": true}

// componentEventRequirements names the field an action cannot run without.
var componentEventRequirements = map[string]string{
	"call-function":    "function_name",
	"call-query":       "query_name",
	"navigate-to-page": "page_id",
	"update-property":  "property_key",
	"update-state":     "state_key",
	"emit-to-parent":   "state_key",
}

// componentEventTargets names the actions that address other components through
// fieldNames, so an empty list is a silently dead subscription.
var componentEventTargets = map[string]bool{
	"hide-fields": true, "unhide-fields": true, "disable-field": true, "enable-field": true,
	"hide-component": true, "show-component": true, "enable-component": true, "disable-component": true,
	"start-loading": true, "stop-loading": true, "start-timer": true, "stop-timer": true,
	"refresh-grid": true, "download-grid": true, "refresh-page-ref": true, "set-field": true,
	"toggle-side-panel": true, "open-side-panel": true, "close-side-panel": true,
}

type validator struct {
	c            *Catalog
	issues       []Issue
	seenIds      map[string]string
	breakpoints  map[string]bool
	pageKeys     map[string]bool
	compKeys     map[string]bool
	eventKeys    map[string]bool
	stateKeys    map[string]bool
	ruleKeys     map[string]bool
	actions      map[string]bool
	ruleTypes    map[string]bool
	allowChildId bool
	// partial marks a component list that patches an existing page, where a
	// component legitimately carries only the keys that changed.
	partial bool
}

func (c *Catalog) newValidator(allowChildIds bool) *validator {
	set := func(values []string) map[string]bool {
		out := make(map[string]bool, len(values))
		for _, v := range values {
			out[v] = true
		}
		return out
	}
	v := &validator{
		c:            c,
		seenIds:      map[string]string{},
		breakpoints:  set(c.Breakpoints()),
		pageKeys:     set(c.InterfaceFieldNames("EruPage")),
		compKeys:     set(c.InterfaceFieldNames("EruComponent")),
		eventKeys:    set(c.InterfaceFieldNames("ComponentEventSubscription")),
		stateKeys:    set(c.InterfaceFieldNames("PageStateVariable")),
		ruleKeys:     set(c.InterfaceFieldNames("ValidationRule")),
		actions:      set(c.EventActions()),
		ruleTypes:    set(c.InterfaceEnumStrings("ValidationRule", "type")),
		allowChildId: allowChildIds,
	}
	if allowChildIds {
		v.compKeys["children_ids"] = true
	}
	return v
}

func (v *validator) add(path, format string, args ...interface{}) {
	v.issues = append(v.issues, Issue{Path: path, Message: fmt.Sprintf(format, args...)})
}

// ValidatePage checks a full EruPage against the component library.
func (c *Catalog) ValidatePage(page map[string]interface{}) []Issue {
	v := c.newValidator(false)
	v.page(page)
	return v.issues
}

// ValidateComponents checks a flat list of components, as a patch carries them.
// Child references by id are allowed here: a patch names children that live in
// other entries or already sit on the page.
func (c *Catalog) ValidateComponents(components []interface{}, path string) []Issue {
	v := c.newValidator(true)
	v.partial = true
	for i, raw := range components {
		component, ok := raw.(map[string]interface{})
		if !ok {
			v.add(fmt.Sprintf("%s[%d]", path, i), "must be an object")
			continue
		}
		v.component(fmt.Sprintf("%s[%d]", path, i), component)
	}
	return v.issues
}

func (v *validator) page(page map[string]interface{}) {
	for key := range page {
		if !v.pageKeys[key] {
			v.add("", "unknown EruPage key %q - allowed keys are %s", key, strings.Join(sortedKeys(v.pageKeys), ", "))
		}
	}
	for _, required := range []string{"id", "name", "components", "styles"} {
		if _, ok := page[required]; !ok {
			v.add("", "EruPage is missing the required key %q", required)
		}
	}
	if styles, ok := page["styles"]; ok {
		v.styles("styles", styles, "")
	}
	if state, ok := page["state"]; ok {
		v.state("state", state)
	}
	if events, ok := page["events"]; ok {
		v.events("events", events, "")
	}
	components, ok := page["components"].([]interface{})
	if !ok {
		if page["components"] != nil {
			v.add("components", "must be a JSON array of components, not %T", page["components"])
		}
		return
	}
	for i, raw := range components {
		component, ok := raw.(map[string]interface{})
		if !ok {
			v.add(fmt.Sprintf("components[%d]", i), "must be an object")
			continue
		}
		v.component(fmt.Sprintf("components[%d]", i), component)
	}
}

func (v *validator) component(path string, component map[string]interface{}) {
	componentType, _ := component["type"].(string)
	id, _ := component["id"].(string)
	label := path
	if id != "" {
		label = fmt.Sprintf("%s (id %q)", path, id)
	}

	if id == "" {
		v.add(path, "component is missing a string \"id\"")
	} else if previous, clash := v.seenIds[id]; clash {
		v.add(label, "duplicate component id - already used at %s", previous)
	} else {
		v.seenIds[id] = path
	}

	if componentType == "" {
		v.add(label, "component is missing a string \"type\"")
		return
	}
	definition, known := v.c.Components[componentType]
	if !known {
		if suggestion := v.c.SuggestType(componentType); suggestion != "" {
			v.add(label, "unknown component type %q - did you mean %q?", componentType, suggestion)
		} else {
			v.add(label, "unknown component type %q - it is not in the eru-studio component library", componentType)
		}
		return
	}
	if definition.Deprecated {
		v.add(label, "component type %q is a legacy alias - use %q instead", componentType, definition.AliasOf)
	}

	for key := range component {
		if !v.compKeys[key] {
			v.add(label, "unknown component key %q - allowed keys are %s", key, strings.Join(sortedKeys(v.compKeys), ", "))
		}
	}

	if _, ok := component["properties"]; !ok {
		if !v.partial {
			v.add(label, "component is missing \"properties\" (use {\"base\": {...}})")
		}
	} else {
		v.properties(label, componentType, component["properties"])
	}
	if _, ok := component["styles"]; !ok {
		if !v.partial {
			v.add(label, "component is missing \"styles\" (%s)", styleKeysHint)
		}
	} else {
		v.styles(label+".styles", component["styles"], componentType)
	}
	if events, ok := component["events"]; ok {
		v.events(label+".events", events, componentType)
	}
	if rules, ok := component["validation_rules"]; ok {
		v.validationRules(label+".validation_rules", rules)
	}

	children, hasChildren := component["children"]
	childIds, hasChildIds := component["children_ids"]
	// A page host's "children" are the mounted page's components, rendered in
	// place. They are that page's, not this component's, so they are legal here
	// and are checked as components - but the agent must edit them through the
	// nested page, not through the host.
	hostsNestedPage := v.c.MayHoldNestedPage(componentType)
	if (hasChildren || hasChildIds) && !definition.AllowChildren && !hostsNestedPage {
		if list, ok := children.([]interface{}); !ok || len(list) > 0 || hasChildIds {
			v.add(label, "component type %q cannot have children - only %s can", componentType, strings.Join(v.c.Containers(), ", "))
		}
	}
	if hostsNestedPage && hasChildIds {
		v.add(label, "%q mounts another page by id (%s); it has no children of its own, so children_ids does not apply",
			componentType, strings.Join(v.c.PageHostProperties(componentType), " / "))
	}
	if hasChildIds {
		if _, ok := childIds.([]interface{}); !ok && childIds != nil {
			v.add(label, "\"children_ids\" must be an array of component ids")
		}
	}
	if hasChildren {
		list, ok := children.([]interface{})
		if !ok {
			if children != nil {
				v.add(label, "\"children\" must be a JSON array of components, not %T", children)
			}
			return
		}
		for i, raw := range list {
			child, ok := raw.(map[string]interface{})
			if !ok {
				v.add(fmt.Sprintf("%s.children[%d]", label, i), "must be an object")
				continue
			}
			v.component(fmt.Sprintf("%s.children[%d]", label, i), child)
		}
	}
}

func (v *validator) properties(label, componentType string, raw interface{}) {
	properties, ok := raw.(map[string]interface{})
	if !ok {
		v.add(label, "\"properties\" must be an object keyed by breakpoint (%s)", strings.Join(v.c.Breakpoints(), ", "))
		return
	}
	responsive := false
	for key := range properties {
		if v.breakpoints[key] {
			responsive = true
			break
		}
	}
	if !responsive {
		v.add(label, "\"properties\" must nest values under a breakpoint - put defaults in properties.base")
		v.propertyBag(label+".properties", componentType, properties)
		return
	}
	for breakpoint, value := range properties {
		if !v.breakpoints[breakpoint] {
			v.add(label, "unknown breakpoint %q in \"properties\" - use %s", breakpoint, strings.Join(v.c.Breakpoints(), ", "))
			continue
		}
		bag, ok := value.(map[string]interface{})
		if !ok {
			if value != nil {
				v.add(fmt.Sprintf("%s.properties.%s", label, breakpoint), "must be an object of property values")
			}
			continue
		}
		v.propertyBag(fmt.Sprintf("%s.properties.%s", label, breakpoint), componentType, bag)
	}
}

func (v *validator) propertyBag(path, componentType string, bag map[string]interface{}) {
	for key, value := range bag {
		property, known := v.c.Property(componentType, key)
		if !known {
			v.add(path, "%q has no property %q%s", componentType, key, v.nearestProperty(componentType, key))
			continue
		}
		v.propertyValue(path, componentType, property, value)
	}
}

func (v *validator) propertyValue(path, componentType string, property Property, value interface{}) {
	allowed := property.EnumValues()
	if len(allowed) == 0 {
		return
	}
	text, ok := value.(string)
	if !ok || text == "" || isExpression(text) {
		return
	}
	for _, candidate := range allowed {
		if candidate == text {
			return
		}
	}
	v.add(path, "%s.%s = %q is not allowed - use one of %s", componentType, property.Key, text, strings.Join(allowed, " | "))
}

// isExpression spots a value the logic evaluator resolves at runtime, which no
// static option list can constrain.
func isExpression(value string) bool {
	return strings.HasPrefix(value, "@") || strings.Contains(value, "{{")
}

func (v *validator) nearestProperty(componentType, key string) string {
	best, bestScore := "", 0
	needle := normalizeTypeName(key)
	for _, property := range v.c.EffectiveProperties(componentType) {
		if score := similarity(needle, normalizeTypeName(property.Key)); score > bestScore {
			best, bestScore = property.Key, score
		}
	}
	if best == "" || bestScore < 3 {
		return ""
	}
	return fmt.Sprintf(" - did you mean %q?", best)
}

// styles checks the four style buckets and their breakpoints. Individual style
// keys are deliberately not checked: the renderer's StyleProperties carries an
// index signature, so property-to-tailwind accepts any CSS-ish key, and the
// style schema only describes what the property panel offers.
func (v *validator) styles(path string, raw interface{}, componentType string) {
	styles, ok := raw.(map[string]interface{})
	if !ok {
		v.add(path, "\"styles\" must be an object with %s", styleKeysHint)
		return
	}
	for key, value := range styles {
		if !pageStyleKeys[key] {
			v.add(path, "unknown styles key %q - allowed keys are %s", key, styleKeysHint)
			continue
		}
		if key != "responsive_styles" && key != "responsive_classes" {
			continue
		}
		perBreakpoint, ok := value.(map[string]interface{})
		if !ok {
			continue
		}
		for breakpoint := range perBreakpoint {
			if !v.breakpoints[breakpoint] {
				v.add(fmt.Sprintf("%s.%s", path, key), "unknown breakpoint %q - use %s", breakpoint, strings.Join(v.c.Breakpoints(), ", "))
			}
		}
	}
}

func (v *validator) events(path string, raw interface{}, componentType string) {
	v.eventList(path, raw, componentType, false)
}

func (v *validator) eventList(path string, raw interface{}, componentType string, nested bool) {
	list, ok := raw.([]interface{})
	if !ok {
		if raw != nil {
			v.add(path, "\"events\" must be a JSON array of event subscriptions")
		}
		return
	}
	for i, item := range list {
		event, ok := item.(map[string]interface{})
		if !ok {
			v.add(fmt.Sprintf("%s[%d]", path, i), "must be an object")
			continue
		}
		v.event(fmt.Sprintf("%s[%d]", path, i), event, componentType, nested)
	}
}

func (v *validator) event(path string, event map[string]interface{}, componentType string, nested bool) {
	for key := range event {
		if !v.eventKeys[key] {
			v.add(path, "unknown event key %q - allowed keys are %s", key, strings.Join(sortedKeys(v.eventKeys), ", "))
		}
	}
	action, _ := event["action"].(string)
	if action == "" {
		v.add(path, "event subscription is missing a string \"action\"")
	} else if !v.actions[action] {
		v.add(path, "unknown action %q - allowed actions are %s", action, strings.Join(v.c.EventActions(), " | "))
	} else {
		if required, ok := componentEventRequirements[action]; ok {
			if text, _ := event[required].(string); strings.TrimSpace(text) == "" {
				v.add(path, "action %q requires %q", action, required)
			}
		}
		if componentEventTargets[action] {
			if names, ok := event["fieldNames"].([]interface{}); !ok || len(names) == 0 {
				v.add(path, "action %q needs \"fieldNames\" naming the components it acts on", action)
			}
		}
	}
	if name, _ := event["event"].(string); name == "" {
		if !nested {
			v.add(path, "event subscription is missing a string \"event\"")
		}
	} else if componentType != "" {
		if known := v.c.Events(componentType); len(known) > 0 && !contains(known, name) && !isPageEvent(name) {
			v.add(path, "%q does not emit %q - it emits %s", componentType, name, strings.Join(known, " | "))
		}
	}
	if id, _ := event["id"].(string); id == "" && !nested {
		v.add(path, "event subscription is missing a string \"id\"")
	}
	for _, branch := range []string{"on_success", "on_error"} {
		if followUp, ok := event[branch]; ok {
			v.eventList(path+"."+branch, followUp, "", true)
		}
	}
}

// isPageEvent covers the lifecycle and action-outcome events the runtime raises
// on a component rather than the component emitting them itself.
func isPageEvent(name string) bool {
	switch name {
	case "on_load", "on_api_success", "on_api_error", "on_upload", "on_complete", "timeout", "timer_start", "custom_action", "on_drill":
		return true
	}
	return false
}

func (v *validator) validationRules(path string, raw interface{}) {
	list, ok := raw.([]interface{})
	if !ok {
		if raw != nil {
			v.add(path, "\"validation_rules\" must be a JSON array")
		}
		return
	}
	for i, item := range list {
		rule, ok := item.(map[string]interface{})
		if !ok {
			v.add(fmt.Sprintf("%s[%d]", path, i), "must be an object")
			continue
		}
		for key := range rule {
			if !v.ruleKeys[key] {
				v.add(fmt.Sprintf("%s[%d]", path, i), "unknown validation rule key %q - allowed keys are %s", key, strings.Join(sortedKeys(v.ruleKeys), ", "))
			}
		}
		ruleType, _ := rule["type"].(string)
		if ruleType == "" {
			v.add(fmt.Sprintf("%s[%d]", path, i), "validation rule is missing a string \"type\"")
		} else if !v.ruleTypes[ruleType] {
			v.add(fmt.Sprintf("%s[%d]", path, i), "unknown validation type %q - allowed types are %s", ruleType, strings.Join(v.c.InterfaceEnumStrings("ValidationRule", "type"), " | "))
		}
		if message, _ := rule["message"].(string); strings.TrimSpace(message) == "" {
			v.add(fmt.Sprintf("%s[%d]", path, i), "validation rule needs a \"message\"")
		}
	}
}

func (v *validator) state(path string, raw interface{}) {
	list, ok := raw.([]interface{})
	if !ok {
		if raw != nil {
			v.add(path, "\"state\" must be a JSON array of state variables")
		}
		return
	}
	for i, item := range list {
		variable, ok := item.(map[string]interface{})
		if !ok {
			v.add(fmt.Sprintf("%s[%d]", path, i), "must be an object")
			continue
		}
		for key := range variable {
			if !v.stateKeys[key] {
				v.add(fmt.Sprintf("%s[%d]", path, i), "unknown state key %q - allowed keys are %s", key, strings.Join(sortedKeys(v.stateKeys), ", "))
			}
		}
		if key, _ := variable["key"].(string); strings.TrimSpace(key) == "" {
			v.add(fmt.Sprintf("%s[%d]", path, i), "state variable needs a \"key\"")
		}
	}
}

func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func contains(values []string, needle string) bool {
	for _, v := range values {
		if v == needle {
			return true
		}
	}
	return false
}
