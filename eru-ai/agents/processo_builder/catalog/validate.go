package catalog

import (
	"fmt"
	"sort"
	"strings"
)

// What this validator checks and what it deliberately does not.
//
// It checks what the catalog knows: that a datatype exists and is still offered,
// that a name matches the pattern the editor enforces, that a reserved name is
// not being written, that a key has some business on a field of that datatype.
// All of it derived - nothing here is a restatement of a rule written elsewhere.
//
// It does NOT check required-ness or the per-datatype combinations that make a
// payload valid (a STATIC dropdown needs options, a status needs its two lists).
// Those live on the Go param structs in eru-ai/tools/eru/processo.go and are
// enforced by ValidateStruct at call time. Copying them here would give us two
// rule sets to keep in step, and the copy would be the one that went stale.

type Code string

const (
	CodeDatatypeUnknown   Code = "datatype_unknown"
	CodeDatatypeRetired   Code = "datatype_retired"
	CodeDatatypeMissing   Code = "datatype_missing"
	CodeFieldNameInvalid  Code = "field_name_invalid"
	CodeFieldNameSystem   Code = "field_name_reserved"
	CodeFieldNameMissing  Code = "field_name_missing"
	CodeKeyNotOnDatatype  Code = "key_not_on_datatype"
	CodeEntityNameInvalid Code = "entity_name_invalid"
	CodeEntityNameMissing Code = "entity_name_missing"
)

type Issue struct {
	Path    string
	Code    Code
	Message string
}

func (i Issue) String() string {
	if i.Path == "" {
		return fmt.Sprintf("[%s] %s", i.Code, i.Message)
	}
	return fmt.Sprintf("%s: [%s] %s", i.Path, i.Code, i.Message)
}

// FormatIssues renders at most max issues as instructions, because the text goes
// back to the model as the reason to try again.
func FormatIssues(issues []Issue, max int) string {
	if len(issues) == 0 {
		return ""
	}
	shown := issues
	if max > 0 && len(issues) > max {
		shown = issues[:max]
	}
	lines := make([]string, 0, len(shown)+1)
	for _, issue := range shown {
		lines = append(lines, "  - "+issue.String())
	}
	if len(shown) < len(issues) {
		lines = append(lines, fmt.Sprintf("  ... and %d more", len(issues)-len(shown)))
	}
	return strings.Join(lines, "\n")
}

// ValidateFieldIdentity checks the two things that identify a field: its name
// and its datatype.
//
// It is separate from ValidateField because a field is described in two shapes.
// The payload that goes to save_field carries only field keys, so a key that
// belongs to no datatype is a real fault. A report that a field was written
// carries reporting keys too - which entity, what happened - and holding those
// to the payload's key set would reject every honest report.
func (c *Catalog) ValidateFieldIdentity(path string, field map[string]interface{}) []Issue {
	var issues []Issue
	add := func(code Code, format string, args ...interface{}) {
		issues = append(issues, Issue{Path: path, Code: code, Message: fmt.Sprintf(format, args...)})
	}

	name, _ := field["name"].(string)
	switch {
	case strings.TrimSpace(name) == "":
		add(CodeFieldNameMissing, "a field needs a name")
	case c.IsSystemFieldName(name):
		add(CodeFieldNameSystem, "%q is a system field created automatically on every entity - it must not be written", name)
	case !c.ValidFieldName(name):
		add(CodeFieldNameInvalid, "field name %q does not match %s - lower case and underscore only, no digits", name, c.Field.NamePattern)
	}

	datatype, _ := field["datatype"].(string)
	switch {
	case strings.TrimSpace(datatype) == "":
		add(CodeDatatypeMissing, "a field needs a datatype - one of: %s", strings.Join(c.DatatypeNames(), ", "))
		return issues // without a datatype there is nothing to check the keys against
	case c.IsRetiredDatatype(datatype):
		add(CodeDatatypeRetired, "%q is retired - the backend still accepts it but the editor no longer offers it. Use one of: %s", datatype, strings.Join(c.DatatypeNames(), ", "))
		return issues
	case !c.IsDatatype(datatype):
		if suggestion := c.SuggestDatatype(datatype); suggestion != "" {
			add(CodeDatatypeUnknown, "%q is not a datatype - did you mean %q?", datatype, suggestion)
		} else {
			add(CodeDatatypeUnknown, "%q is not a datatype. Use one of: %s", datatype, strings.Join(c.DatatypeNames(), ", "))
		}
		return issues
	}
	return issues
}

// ValidateField checks one field payload - the object that goes under `field` on
// save_field. It is the identity check plus the keys, and it assumes every key
// present is meant as a field key.
func (c *Catalog) ValidateField(path string, field map[string]interface{}) []Issue {
	issues := c.ValidateFieldIdentity(path, field)
	datatype, _ := field["datatype"].(string)
	if !c.IsDatatype(datatype) {
		// The identity check already said why; without a known datatype there is
		// nothing to check the keys against.
		return issues
	}

	// A key that belongs to no datatype, or to a different one, is dropped on
	// save - so the field arrives looking plain and nothing says why.
	stray := make([]string, 0)
	for key := range field {
		if !c.AllowsKey(datatype, key) {
			stray = append(stray, key)
		}
	}
	sort.Strings(stray)
	for _, key := range stray {
		if owners := c.datatypesWithKey(key); len(owners) > 0 {
			issues = append(issues, Issue{Path: path, Code: CodeKeyNotOnDatatype,
				Message: fmt.Sprintf("%q is not written for a %s field - it belongs to %s. It would be dropped on save.", key, datatype, strings.Join(owners, ", "))})
			continue
		}
		issues = append(issues, Issue{Path: path, Code: CodeKeyNotOnDatatype,
			Message: fmt.Sprintf("%q is not a field key - it would be dropped on save.", key)})
	}
	return issues
}

// ValidateEntity checks one entity of the entity_data list.
func (c *Catalog) ValidateEntity(path string, entity map[string]interface{}) []Issue {
	var issues []Issue
	name, _ := entity["name"].(string)
	switch {
	case strings.TrimSpace(name) == "":
		issues = append(issues, Issue{Path: path, Code: CodeEntityNameMissing, Message: "an entity needs a name"})
	case !c.ValidEntityName(name):
		issues = append(issues, Issue{Path: path, Code: CodeEntityNameInvalid,
			Message: fmt.Sprintf("entity name %q does not match %s", name, c.Entity.NamePattern)})
	}
	return issues
}

// datatypesWithKey names the datatypes a key does belong to, so the message can
// point somewhere instead of only refusing.
func (c *Catalog) datatypesWithKey(key string) []string {
	owners := make([]string, 0, 4)
	for name, keys := range c.Field.PerDatatype {
		if c.notOffered[name] {
			continue
		}
		for _, k := range keys {
			if k == key {
				owners = append(owners, name)
				break
			}
		}
	}
	sort.Strings(owners)
	if len(owners) > 4 {
		return append(owners[:4], "...")
	}
	return owners
}
