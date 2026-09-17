package eru_studio

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	catalog "github.com/eru-tech/eru/eru-ai/agents/eru_studio/catalog"
)

// LayoutIssues finds pages that are valid and still render wrong.
//
// Every other check in this package asks whether the page is well formed. These
// ask whether it will look like anything: a pane squeezed to the width of its
// text, columns that add up to more than the row they sit in, a fixed grid in a
// flexible container. The renderer accepts all of it, which is exactly why
// nothing caught it - the only previous signal was someone opening the page and
// saying it looked wrong.
//
// The rules are deliberately few and each is grounded in a page we actually
// shipped. A layout rule that fires on a design someone meant is worse than no
// rule, because it teaches the model to ignore the whole class.
func LayoutIssues(page map[string]interface{}) []catalog.Issue {
	issues := []catalog.Issue{}
	if len(page) == 0 {
		return issues
	}
	WalkPage(page, func(component map[string]interface{}, _ string) {
		switch componentType(component) {
		case "flex_container":
			issues = append(issues, flexRowIssues(component)...)
		case "grid_container":
			issues = append(issues, gridColumnIssues(component)...)
		}
		if componentType(component) == "flex_container" {
			issues = append(issues, rowWidthOverflow(component)...)
		}
	})
	return issues
}

// flexRowIssues checks a horizontal flex row for panes that will collapse.
func flexRowIssues(container map[string]interface{}) []catalog.Issue {
	if !isRow(container) {
		return nil
	}
	children := childList(container)
	if len(children) < 2 {
		return nil
	}

	// Nothing grows, so nothing is taking space away from anything else: the
	// children sit at their content width on purpose. That is what a header or a
	// button bar looks like, and it is not a mistake.
	anyGrows := false
	for _, child := range children {
		if growsHorizontally(child) {
			anyGrows = true
			break
		}
	}
	if !anyGrows {
		return nil
	}

	issues := []catalog.Issue{}
	for _, child := range children {
		// Only a container that holds a layout can be "squeezed" in a way that
		// matters. An icon or a label next to a growing sibling is the normal
		// way to build a header.
		if !holdsLayout(child) || growsHorizontally(child) || declaredWidth(child) != "" {
			continue
		}
		id, _ := child["id"].(string)
		issues = append(issues, catalog.Issue{
			Path:        fmt.Sprintf("components (id %q)", id),
			Code:        catalog.CodeLayoutSqueezedPane,
			ComponentId: id,
			Message: fmt.Sprintf(
				"this pane sits in a row where a sibling grows, and it sets neither a width nor a flex_grow of its own, " +
					"so it collapses to the width of its contents instead of taking its share of the row. " +
					"Give it a Width style (a two-pane split is built on the panes: the narrow one gets a percentage width, the wide one 100%%) " +
					"- setting flex_grow on the parent container will not do it, because that value applies to every child equally"),
		})
	}
	return issues
}

// gridColumnIssues catches a grid whose columns cannot use the space it is given.
func gridColumnIssues(container map[string]interface{}) []catalog.Issue {
	template := strings.TrimSpace(propertyString(container, "grid_template_columns"))
	if template == "" || strings.Contains(template, "@") {
		return nil
	}
	lower := strings.ToLower(template)
	for _, flexible := range []string{"fr", "auto", "minmax", "%", "repeat"} {
		if strings.Contains(lower, flexible) {
			return nil
		}
	}
	columns := strings.Fields(template)
	if len(columns) < 2 {
		return nil
	}
	fixed := 0
	for _, column := range columns {
		if strings.HasSuffix(strings.ToLower(column), "px") {
			fixed++
		}
	}
	if fixed != len(columns) {
		return nil
	}
	id, _ := container["id"].(string)
	return []catalog.Issue{{
		Path:        fmt.Sprintf("components (id %q)", id),
		Code:        catalog.CodeLayoutFixedColumns,
		ComponentId: id,
		Message: fmt.Sprintf("grid_template_columns is %q - every column is a fixed pixel width, so the grid ignores the width it is given "+
			"and the content crowds to one side on any screen wider than the total. Use fractions (e.g. \"1fr 1fr\") so the columns share the row", template),
	}}
}

var percentWidth = regexp.MustCompile(`^([0-9]+(?:\.[0-9]+)?)%$`)

// rowWidthIssues is folded into flexRowIssues' caller: percentage widths in one
// row that add up to more than the row can hold will wrap or overflow.
func rowWidthOverflow(container map[string]interface{}) []catalog.Issue {
	if !isRow(container) || wraps(container) {
		return nil
	}
	children := childList(container)
	if len(children) < 2 {
		return nil
	}
	total := 0.0
	counted := 0
	for _, child := range children {
		match := percentWidth.FindStringSubmatch(declaredWidth(child))
		if match == nil {
			// A child with no percentage width makes the sum meaningless.
			return nil
		}
		value, err := strconv.ParseFloat(match[1], 64)
		if err != nil {
			return nil
		}
		total += value
		counted++
	}
	if counted < 2 || total <= 100.5 {
		return nil
	}
	id, _ := container["id"].(string)
	return []catalog.Issue{{
		Path:        fmt.Sprintf("components (id %q)", id),
		Code:        catalog.CodeLayoutWidthsOverflow,
		ComponentId: id,
		Message: fmt.Sprintf("the %d children of this row declare widths adding up to %.0f%%, so the last one wraps or spills out of the row. "+
			"Make them add up to 100%% or less", counted, total),
	}}
}

func componentType(component map[string]interface{}) string {
	value, _ := component["type"].(string)
	return value
}

func childList(component map[string]interface{}) []map[string]interface{} {
	raw, _ := component["children"].([]interface{})
	out := make([]map[string]interface{}, 0, len(raw))
	for _, item := range raw {
		if child, ok := item.(map[string]interface{}); ok {
			out = append(out, child)
		}
	}
	return out
}

// isRow reports whether a flex container lays its children out horizontally.
// Row is the renderer's default, so an unset direction is a row.
func isRow(component map[string]interface{}) bool {
	direction := strings.ToLower(strings.TrimSpace(propertyString(component, "flex_direction")))
	return direction == "" || direction == "row"
}

func wraps(component map[string]interface{}) bool {
	value := strings.ToLower(strings.TrimSpace(propertyString(component, "flex_wrap")))
	return value == "wrap" || value == "wrap-reverse"
}

// holdsLayout reports whether a component is a pane rather than a leaf. Only a
// pane can be squeezed in a way a user would call a bug.
func holdsLayout(component map[string]interface{}) bool {
	switch componentType(component) {
	case "flex_container", "grid_container", "card", "grid", "page_ref", "tile", "nav_outlet":
		return true
	}
	return false
}

// growsHorizontally reports whether a component claims a share of the leftover
// space, whether through the flex_grow property or a utility class.
func growsHorizontally(component map[string]interface{}) bool {
	if value := propertyString(component, "flex_grow"); value != "" && value != "0" {
		return true
	}
	if raw, ok := propertyValue(component, "flex_grow").(float64); ok && raw > 0 {
		return true
	}
	for _, class := range classList(component) {
		if class == "flex-1" || class == "grow" || strings.HasPrefix(class, "flex-grow") || strings.HasPrefix(class, "basis-") {
			return true
		}
	}
	return false
}

// declaredWidth is the width the component asks for, from whichever place it was
// written: a style, a custom style, or a tailwind width class.
func declaredWidth(component map[string]interface{}) string {
	styles, _ := component["styles"].(map[string]interface{})
	if styles != nil {
		if responsive, ok := styles["responsive_styles"].(map[string]interface{}); ok {
			keys := make([]string, 0, len(responsive))
			for key := range responsive {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range append([]string{"base"}, keys...) {
				bag, ok := responsive[key].(map[string]interface{})
				if !ok {
					continue
				}
				if width, _ := bag["width"].(string); strings.TrimSpace(width) != "" {
					return strings.TrimSpace(width)
				}
			}
		}
		if custom, ok := styles["custom"].(map[string]interface{}); ok {
			if width, _ := custom["width"].(string); strings.TrimSpace(width) != "" {
				return strings.TrimSpace(width)
			}
		}
	}
	for _, class := range classList(component) {
		if strings.HasPrefix(class, "w-") && class != "w-auto" && class != "w-fit" {
			return class
		}
	}
	return ""
}

func classList(component map[string]interface{}) []string {
	styles, _ := component["styles"].(map[string]interface{})
	if styles == nil {
		return nil
	}
	out := []string{}
	if classes, _ := styles["classes"].(string); classes != "" {
		out = append(out, strings.Fields(classes)...)
	}
	if responsive, ok := styles["responsive_classes"].(map[string]interface{}); ok {
		for _, value := range responsive {
			if classes, _ := value.(string); classes != "" {
				out = append(out, strings.Fields(classes)...)
			}
		}
	}
	return out
}

func propertyValue(component map[string]interface{}, key string) interface{} {
	properties, ok := component["properties"].(map[string]interface{})
	if !ok {
		return nil
	}
	if base, ok := properties["base"].(map[string]interface{}); ok {
		if value, present := base[key]; present {
			return value
		}
	}
	return nil
}

func propertyString(component map[string]interface{}, key string) string {
	switch typed := propertyValue(component, key).(type) {
	case string:
		return typed
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(typed)
	}
	return ""
}

// WalkPage visits every component of a page with the id of its parent.
func WalkPage(page map[string]interface{}, visit func(component map[string]interface{}, parentId string)) {
	walkComponents(page, visit)
}
