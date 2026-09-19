package catalog

import (
	"fmt"
	"sort"
	"strings"
)

// The prompt blocks are generated from the same catalog the validator reads, so
// what the model is told and what its answer is held to cannot drift apart.

// DatatypeIndex is the compact list the model gets up front: every datatype it
// may use, and the keys that datatype writes beyond the common set. A datatype
// with no extra keys is listed too - its absence would read as "not available".
func (c *Catalog) DatatypeIndex() string {
	var b strings.Builder
	b.WriteString("DATATYPES (these are the only values `datatype` may take)\n")
	names := c.DatatypeNames()
	width := 0
	for _, name := range names {
		if len(name) > width {
			width = len(name)
		}
	}
	for _, name := range names {
		keys := c.KeysFor(name)
		label := c.Label(name)
		if len(keys) == 0 {
			fmt.Fprintf(&b, "  %-*s  %-22s  (no keys of its own)\n", width, name, label)
			continue
		}
		fmt.Fprintf(&b, "  %-*s  %-22s  %s\n", width, name, label, strings.Join(keys, ", "))
	}
	return b.String()
}

// CommonKeyReference is what every field carries whatever its datatype.
func (c *Catalog) CommonKeyReference() string {
	var b strings.Builder
	b.WriteString("KEYS EVERY FIELD CARRIES\n  ")
	b.WriteString(strings.Join(c.Field.CommonKeys, ", "))
	b.WriteString("\n\nKEYS THAT DEPEND ON SOMETHING OTHER THAN THE DATATYPE\n  ")
	b.WriteString(strings.Join(c.Field.ConditionalKeys, ", "))
	b.WriteString("\n")
	if optionTypes := c.OptionTypes(); len(optionTypes) > 0 {
		fmt.Fprintf(&b, "\n  `option_type` is one of: %s\n", strings.Join(optionTypes, ", "))
	}
	return b.String()
}

// NamingRules covers the two patterns and the reserved names. They are the
// cheapest mistakes to make and the most annoying to undo, because a field
// cannot be renamed - only deleted and recreated.
func (c *Catalog) NamingRules() string {
	var b strings.Builder
	b.WriteString("NAMING\n")
	if p := c.Field.NamePattern; p != "" {
		fmt.Fprintf(&b, "  A field name must match %s - lower case and underscore only. No digits, no camelCase, no spaces.\n", p)
	}
	if p := c.Entity.NamePattern; p != "" {
		fmt.Fprintf(&b, "  An entity name must match %s - it may contain digits, a field name may not.\n", p)
	}
	b.WriteString("  `label` is what a person reads and may be anything; `name` is the identifier and is what\n")
	b.WriteString("  every binding, query and page refers to. A field cannot be renamed later - it can only be\n")
	b.WriteString("  deleted and recreated - so get the name right the first time.\n")
	if names := c.SystemFieldNames(); len(names) > 0 {
		fmt.Fprintf(&b, "\n  RESERVED - these are created automatically on every entity and must never be written,\n  renamed or deleted:\n    %s\n", strings.Join(names, ", "))
	}
	if retired := c.Field.PerDatatypeNotOffered; len(retired) > 0 {
		fmt.Fprintf(&b, "\n  RETIRED - still accepted by the backend, never to be emitted: %s\n", strings.Join(retired, ", "))
	}
	return b.String()
}

// Spec is the detail for named datatypes, which is what get_field_spec returns.
// Asking for it beats putting every datatype's full shape in the prompt, and it
// is the same text either way.
func (c *Catalog) Spec(datatypes []string) string {
	if len(datatypes) == 0 {
		return c.DatatypeIndex()
	}
	sorted := append([]string(nil), datatypes...)
	sort.Strings(sorted)

	var b strings.Builder
	for _, name := range sorted {
		if c.IsRetiredDatatype(name) {
			fmt.Fprintf(&b, "%s: RETIRED. The backend still accepts it but the editor no longer offers it - do not use it.\n\n", name)
			continue
		}
		if !c.IsDatatype(name) {
			if suggestion := c.SuggestDatatype(name); suggestion != "" {
				fmt.Fprintf(&b, "%s: not a datatype. Did you mean %s?\n\n", name, suggestion)
			} else {
				fmt.Fprintf(&b, "%s: not a datatype. Use one of: %s\n\n", name, strings.Join(c.DatatypeNames(), ", "))
			}
			continue
		}
		fmt.Fprintf(&b, "%s (%s)\n", name, c.Label(name))
		keys := c.KeysFor(name)
		if len(keys) == 0 {
			b.WriteString("  writes no keys of its own - only the common set\n")
		} else {
			fmt.Fprintf(&b, "  own keys: %s\n", strings.Join(keys, ", "))
		}
		fmt.Fprintf(&b, "  plus the common set: %s\n", strings.Join(c.Field.CommonKeys, ", "))
		b.WriteString("\n")
	}
	return b.String()
}

// Contract is the block the system prompt embeds.
func (c *Catalog) Contract() string {
	var b strings.Builder
	b.WriteString(c.DatatypeIndex())
	b.WriteString("\n")
	b.WriteString(c.CommonKeyReference())
	b.WriteString("\n")
	b.WriteString(c.NamingRules())
	b.WriteString("\nCall `get_field_spec` for the exact key set of a datatype before you write a field of a\n")
	b.WriteString("type you have not used in this conversation. A key that is not listed for a datatype is\n")
	b.WriteString("ignored on save, so a field that looks configured arrives plain.\n")
	return b.String()
}
