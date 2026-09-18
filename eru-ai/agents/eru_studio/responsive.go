package eru_studio

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// The designer renders a page at a fixed frame width per breakpoint, and the
// agent never sees that rendering. It reads JSON, in which a row of two fields
// looks the same whether the frame is 1536px or 390px - so nothing tells it
// that the row it just wrote spills off the phone.
//
// This measures the one thing a page cannot express and the model cannot see:
// how much width a row needs versus how much the small frame has. It is an
// estimate, deliberately conservative, and it is reported as context rather
// than enforced - the point is to give the model the perception it lacks, not
// to fail an edit over arithmetic.

// SmallFrameWidth is the designer's `sm` canvas: .canvas-viewport.viewport-sm.
// Note this is NOT the tailwind `sm` value (640) the toolbar tooltip shows -
// that is the breakpoint at which `sm` overrides apply; the frame drawn is a
// phone.
const SmallFrameWidth = 390

// MinFormControlWidth is what an eru form control occupies before it stops
// shrinking: the material form field's own min-width plus its label. A `1fr`
// track cannot go below it, which is why two of them in one row overflow long
// before the arithmetic on the track sizes suggests it.
const MinFormControlWidth = 200

// minIconColumnWidth is what a bare icon between two fields costs.
const minIconColumnWidth = 48

// formControlTypes are the components that carry that minimum.
var formControlTypes = map[string]bool{
	"select-eru": true, "select": true, "date": true, "date-range": true,
	"number": true, "text-field": true, "input": true, "textarea": true,
	"autocomplete": true, "chip-list": true, "time": true, "datetime": true,
}

// Overflow is one container that does not fit the small frame.
type Overflow struct {
	ComponentId string
	Type        string
	Needs       int
	Has         int
	Reason      string
}

func (o Overflow) String() string {
	return fmt.Sprintf("%s (%s) needs about %dpx but has %dpx at sm - %s", o.ComponentId, o.Type, o.Needs, o.Has, o.Reason)
}

// SmallScreenOverflows reports the containers whose children cannot fit the sm
// frame, walking the tree so each container is measured against the width its
// ancestors actually leave it.
func SmallScreenOverflows(page map[string]interface{}) []Overflow {
	components, _ := page["components"].([]interface{})
	if len(components) == 0 {
		return nil
	}
	var found []Overflow
	var walk func(list []interface{}, available int)
	walk = func(list []interface{}, available int) {
		for _, raw := range list {
			component, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			inner := available - horizontalPadding(component)
			if overflow := measureRow(component, inner); overflow != nil {
				found = append(found, *overflow)
			}
			children, _ := component["children"].([]interface{})
			walk(children, inner)
		}
	}
	walk(components, SmallFrameWidth)
	sort.Slice(found, func(i, j int) bool { return found[i].ComponentId < found[j].ComponentId })
	return found
}

// measureRow reports a container that keeps its children side by side at sm and
// cannot fit them.
func measureRow(component map[string]interface{}, available int) *Overflow {
	if available <= 0 {
		return nil
	}
	kind, _ := component["type"].(string)
	base := propsAt(component, "base")
	small := propsAt(component, "sm")

	children, _ := component["children"].([]interface{})
	if len(children) < 2 {
		return nil
	}

	var columns int
	var reason string
	switch kind {
	case "grid_container":
		template := resolvedString(small, base, "grid_template_columns")
		columns = len(strings.Fields(template))
		if columns < 2 {
			return nil
		}
		// A grid that stacks itself below a width wide enough to cover the small
		// frame has already handled this.
		if threshold := resolvedInt(small, base, "collapse_below_width"); threshold > available {
			return nil
		}
		reason = fmt.Sprintf("grid_template_columns %q keeps %d columns and collapse_below_width does not cover a %dpx container", template, columns, available)
	case "flex_container":
		direction := resolvedString(small, base, "flex_direction")
		if direction != "row" && direction != "row-reverse" {
			return nil
		}
		if wrap := resolvedString(small, base, "flex_wrap"); wrap == "wrap" || wrap == "wrap-reverse" {
			return nil
		}
		columns = len(children)
		reason = fmt.Sprintf("flex_direction is row with no flex_wrap, so its %d children stay on one line", columns)
	default:
		return nil
	}

	needs := rowMinimumWidth(children) + resolvedInt(small, base, "gap")*(columns-1)
	if needs <= available {
		return nil
	}
	id, _ := component["id"].(string)
	return &Overflow{ComponentId: id, Type: kind, Needs: needs, Has: available, Reason: reason}
}

// rowMinimumWidth is the narrowest the children go: form controls stop at their
// own minimum, anything else is assumed to be able to shrink.
func rowMinimumWidth(children []interface{}) int {
	total := 0
	for _, raw := range children {
		child, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		kind, _ := child["type"].(string)
		switch {
		case formControlTypes[kind]:
			total += MinFormControlWidth
		case kind == "icon":
			total += minIconColumnWidth
		}
	}
	return total
}

func horizontalPadding(component map[string]interface{}) int {
	styles, _ := component["styles"].(map[string]interface{})
	responsive, _ := styles["responsive_styles"].(map[string]interface{})
	base, _ := responsive["base"].(map[string]interface{})
	small, _ := responsive["sm"].(map[string]interface{})
	padding := firstNumber(small["padding"], base["padding"])
	// "16px 24px" - the second value is the horizontal one.
	if padding == 0 {
		if text := firstString(small["padding"], base["padding"]); text != "" {
			parts := strings.Fields(text)
			if len(parts) > 1 {
				padding = toPixels(parts[1])
			}
		}
	}
	return padding * 2
}

func propsAt(component map[string]interface{}, breakpoint string) map[string]interface{} {
	properties, _ := component["properties"].(map[string]interface{})
	at, _ := properties[breakpoint].(map[string]interface{})
	return at
}

// resolvedString reads a property the way the renderer does: the breakpoint's
// value when it has one, otherwise base.
func resolvedString(small, base map[string]interface{}, key string) string {
	if value, ok := small[key]; ok {
		return fmt.Sprint(value)
	}
	if value, ok := base[key]; ok {
		return fmt.Sprint(value)
	}
	return ""
}

func resolvedInt(small, base map[string]interface{}, key string) int {
	if value, ok := small[key]; ok {
		return toPixels(fmt.Sprint(value))
	}
	if value, ok := base[key]; ok {
		return toPixels(fmt.Sprint(value))
	}
	return 0
}

func firstNumber(values ...interface{}) int {
	for _, value := range values {
		if value == nil {
			continue
		}
		if px := toPixels(fmt.Sprint(value)); px > 0 {
			return px
		}
	}
	return 0
}

func firstString(values ...interface{}) string {
	for _, value := range values {
		if text, ok := value.(string); ok && text != "" {
			return text
		}
	}
	return ""
}

func toPixels(text string) int {
	trimmed := strings.TrimSuffix(strings.TrimSpace(text), "px")
	value, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return 0
	}
	return int(value)
}

// SmallScreenNote is what the agent is told about a page it is about to edit.
func SmallScreenNote(overflows []Overflow) string {
	if len(overflows) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("--- THIS PAGE AT sm (390px phone frame) ---\n")
	b.WriteString("Measured from the page you were given. You cannot see the rendering; this is it.\n")
	for _, overflow := range overflows {
		b.WriteString("- ")
		b.WriteString(overflow.String())
		b.WriteString("\n")
	}
	b.WriteString("Fix one of these only if the request is about small screens. The fix is a per-breakpoint override on the container ")
	b.WriteString("(properties.sm), not a change to base: a grid_container takes properties.sm.grid_template_columns \"1fr\", ")
	b.WriteString("a flex_container takes properties.sm.flex_direction \"column\"; collapse_below_width works too, but only if it is ")
	b.WriteString("ABOVE the container's own width at sm, which is why a value like 320 never fires in a 342px container.\n")
	b.WriteString("--- END THIS PAGE AT sm ---\n\n")
	return b.String()
}
