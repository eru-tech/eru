package catalog

import (
	"fmt"
	"sort"
	"strings"
)

// categoryOrder keeps the index reading like the eru-studio palette rather than
// like a map iteration.
var categoryOrder = []string{"basic", "layout", "form", "eru", "navigation", "data", "loading", "legacy"}

// TypeIndex is the always-on half of the component contract: every type the
// agent may emit, what it is called, whether it takes children, and how many
// properties it has. It is deliberately terse - the full property list for a
// type comes from get_component_spec, on demand.
func (c *Catalog) TypeIndex() string {
	byCategory := map[string][]string{}
	for _, name := range c.types {
		component := c.Components[name]
		byCategory[component.Category] = append(byCategory[component.Category], name)
	}

	var b strings.Builder
	for _, category := range orderedCategories(byCategory) {
		names := byCategory[category]
		sort.Strings(names)
		fmt.Fprintf(&b, "%s:\n", strings.ToUpper(category))
		for _, name := range names {
			component := c.Components[name]
			marks := []string{}
			if component.AllowChildren {
				marks = append(marks, "container")
			}
			if count := len(c.EffectiveProperties(name)); count > 0 {
				marks = append(marks, fmt.Sprintf("%d props", count))
			}
			if len(component.Events) > 0 {
				marks = append(marks, "emits "+strings.Join(component.Events, "/"))
			}
			fmt.Fprintf(&b, "  %-16s %s", name, component.Name)
			if len(marks) > 0 {
				fmt.Fprintf(&b, " (%s)", strings.Join(marks, "; "))
			}
			b.WriteString("\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func orderedCategories(byCategory map[string][]string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, category := range categoryOrder {
		if len(byCategory[category]) > 0 {
			out = append(out, category)
			seen[category] = true
		}
	}
	rest := []string{}
	for category := range byCategory {
		if !seen[category] {
			rest = append(rest, category)
		}
	}
	sort.Strings(rest)
	return append(out, rest...)
}

// Spec renders the authoritative property contract for the named types: every
// property key, its allowed values and its runtime default, straight from the
// library. This is what get_component_spec returns, and it is the only
// description of a component the agent should trust over its own memory.
func (c *Catalog) Spec(types []string) string {
	var b strings.Builder
	for i, componentType := range types {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(c.specOne(componentType))
	}
	return strings.TrimRight(b.String(), "\n")
}

func (c *Catalog) specOne(componentType string) string {
	component, ok := c.Components[componentType]
	if !ok {
		suggestion := ""
		if guess := c.SuggestType(componentType); guess != "" {
			suggestion = fmt.Sprintf(" Did you mean %q?", guess)
		}
		return fmt.Sprintf("%s: NOT A COMPONENT TYPE.%s\n", componentType, suggestion)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "=== %s (%s", componentType, component.Name)
	if component.Deprecated {
		fmt.Fprintf(&b, "; LEGACY ALIAS - emit %q instead", component.AliasOf)
	}
	b.WriteString(") ===\n")

	if component.AllowChildren {
		b.WriteString("children: allowed (container)\n")
	} else {
		b.WriteString("children: NOT allowed (leaf)\n")
	}

	own := component.Properties
	overrides := component.CommonPropertyOverrides
	if len(own) == 0 && len(overrides) == 0 {
		b.WriteString("own properties: none - only the common properties apply\n")
	}
	if len(overrides) > 0 {
		b.WriteString("properties (this type's version of a common property):\n")
		writeProperties(&b, overrides)
	}
	if len(own) > 0 {
		b.WriteString("properties (specific to this type):\n")
		writeProperties(&b, own)
	}
	if len(component.ExcludedCommonProperties) > 0 {
		excluded := append([]string{}, component.ExcludedCommonProperties...)
		sort.Strings(excluded)
		fmt.Fprintf(&b, "common properties this type does NOT have: %s\n", strings.Join(excluded, ", "))
	}
	if len(component.Events) > 0 {
		fmt.Fprintf(&b, "events it emits (beyond the common DOM events): %s\n", strings.Join(component.Events, ", "))
	}
	return b.String()
}

func writeProperties(b *strings.Builder, properties []Property) {
	for _, property := range properties {
		if property.Visible != nil && !*property.Visible {
			continue
		}
		fmt.Fprintf(b, "  %s", property.Key)
		if values := property.EnumValues(); len(values) > 0 {
			fmt.Fprintf(b, " (%s", strings.Join(quoteAll(values), "|"))
		} else {
			fmt.Fprintf(b, " (%s", propertyKind(property))
		}
		if property.Default != nil && property.Default != "" {
			fmt.Fprintf(b, "; default %s", formatValue(property.Default))
		}
		if property.Min != nil || property.Max != nil {
			fmt.Fprintf(b, "; range %s..%s", formatBound(property.Min), formatBound(property.Max))
		}
		b.WriteString(")")
		if property.Description != "" {
			fmt.Fprintf(b, " - %s", collapse(property.Description))
		}
		b.WriteString("\n")
	}
}

// propertyKind translates a property-panel editor type into what the JSON value
// must actually be.
func propertyKind(property Property) string {
	switch property.Type {
	case "boolean":
		return "bool"
	case "number", "spacing":
		return "number"
	case "color":
		return "color string"
	case "status_options":
		return "array of {label,color}"
	case "color_ranges":
		return "array of {from,to,color,background}"
	case "tabs_list":
		return "array of tab definitions"
	case "dependent_fields", "pivot_aggregations", "chart_dimensions", "chart_measures", "chart_extra_options", "chart_value_colors":
		return "array of " + strings.ReplaceAll(property.Type, "_", " ")
	case "multiselect":
		return "comma-separated string"
	case "logic_editor":
		return "logic expression string"
	case "select", "radio", "autocomplete":
		// A select whose options are resolved at runtime (entity or api sourced).
		return "string"
	case "":
		return "value"
	default:
		return property.Type
	}
}

func quoteAll(values []string) []string {
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = fmt.Sprintf("%q", value)
	}
	return out
}

func formatValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		return fmt.Sprintf("%q", v)
	case float64:
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v))
		}
		return fmt.Sprintf("%g", v)
	case bool:
		return fmt.Sprintf("%t", v)
	case nil:
		return "null"
	default:
		return fmt.Sprintf("%v", v)
	}
}

func formatBound(bound *float64) string {
	if bound == nil {
		return ""
	}
	return formatValue(*bound)
}

func collapse(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func shortFingerprint(fingerprint string) string {
	if len(fingerprint) > 8 {
		return fingerprint[:8]
	}
	if fingerprint == "" {
		return "unknown"
	}
	return fingerprint
}

// CommonPropertyReference renders the properties every component shares, for the
// one place in the prompt that has to state them.
func (c *Catalog) CommonPropertyReference() string {
	var b strings.Builder
	writeProperties(&b, c.CommonProperties)
	return strings.TrimRight(b.String(), "\n")
}

// EventActionReference lists the runtime actions an event subscription may run.
func (c *Catalog) EventActionReference() string {
	actions := c.EventActions()
	sort.Strings(actions)
	return strings.Join(actions, ", ")
}

// DeprecatedAliases lists the type names the renderer still loads for old pages
// but that the agent must never emit, each with the name to use instead.
func (c *Catalog) DeprecatedAliases() [][2]string {
	out := [][2]string{}
	for name, component := range c.Components {
		if component.Deprecated && component.AliasOf != "" {
			out = append(out, [2]string{name, component.AliasOf})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i][0] < out[j][0] })
	return out
}

// Contract is the block the agent's system prompt carries: the type index plus
// how to get the rest. Short by design - the detail is a tool call away.
//
// This is the ONLY place the prompt states which types exist, whether each takes
// children, or what a type is called. Anything that repeats those facts in prose
// will eventually contradict the library, and the model will believe whichever
// version it read last.
func (c *Catalog) Contract() string {
	var b strings.Builder
	fmt.Fprintf(&b, "COMPONENT LIBRARY (generated from eru-studio %s, catalog %s)\n",
		shortFingerprint(c.Source.Fingerprint), c.CatalogVersion)
	b.WriteString("This block is generated from the component library itself. It is the single source of\n")
	b.WriteString("truth for which components exist: these are the ONLY component types, spelled exactly\n")
	b.WriteString("as shown (they are case-sensitive, and the hyphens and underscores are part of the name).\n")
	b.WriteString("Never invent a type. Where anything else in this prompt disagrees with this block, this\n")
	b.WriteString("block is right.\n\n")
	b.WriteString(c.TypeIndex())
	b.WriteString("\n\nContainers (the only types that may carry children[]): ")
	b.WriteString(strings.Join(c.Containers(), ", "))
	b.WriteString("\nEvery other type is a leaf and MUST NOT carry a children array - including the ones that\n")
	b.WriteString("look like containers: they take their contents from a property instead.\n")

	if aliases := c.DeprecatedAliases(); len(aliases) > 0 {
		b.WriteString("\nRetired names the renderer still accepts but you must NOT emit: ")
		pairs := make([]string, 0, len(aliases))
		for _, alias := range aliases {
			pairs = append(pairs, fmt.Sprintf("%s -> use %s", alias[0], alias[1]))
		}
		b.WriteString(strings.Join(pairs, ", "))
		b.WriteString("\n")
	}

	b.WriteString("\nBefore you set a property on a component type you have not already been shown in this\n")
	b.WriteString("conversation, call get_component_spec with those types. It returns the authoritative\n")
	b.WriteString("property keys, allowed values and defaults for each one. A property key or value that\n")
	b.WriteString("get_component_spec does not list will be rejected, however plausible it looks.\n")
	return b.String()
}
