package eru_studio

import (
	"encoding/json"
	"strings"
	"testing"
)

func pageJSON(t *testing.T, raw string) map[string]interface{} {
	t.Helper()
	var page map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &page); err != nil {
		t.Fatalf("bad fixture: %v", err)
	}
	return page
}

// The real er_form row: two selects and an arrow in a three-column grid, inside
// a body with 24px padding, with a collapse threshold too low to ever fire.
const erFormRow = `{"components":[{"id":"root","type":"flex_container","properties":{"base":{"flex_direction":"column"}},"children":[
  {"id":"body","type":"flex_container","properties":{"base":{"flex_direction":"column"}},"styles":{"responsive_styles":{"base":{"padding":24}}},"children":[
    {"id":"pair_row","type":"grid_container","properties":{"base":{"grid_template_columns":"1fr 48px 1fr","gap":12,"collapse_below_width":320}},"children":[
      {"id":"fc","type":"select-eru"},{"id":"arrow","type":"icon"},{"id":"tc","type":"select-eru"}]}]}]}]}`

func TestTheCurrencyPairRowIsReportedAsOverflowing(t *testing.T) {
	overflows := SmallScreenOverflows(pageJSON(t, erFormRow))
	if len(overflows) != 1 {
		t.Fatalf("expected the pair row to be reported, got %v", overflows)
	}
	if overflows[0].ComponentId != "pair_row" {
		t.Fatalf("wrong component: %v", overflows[0])
	}
	if overflows[0].Has != SmallFrameWidth-48 {
		t.Errorf("the body's padding was not taken off: %v", overflows[0])
	}
	if !strings.Contains(overflows[0].Reason, "collapse_below_width") {
		t.Errorf("the reason does not name the threshold that failed: %s", overflows[0].Reason)
	}
}

func TestAGridThatStacksItselfIsNotReported(t *testing.T) {
	fixed := strings.Replace(erFormRow, `"collapse_below_width":320`, `"collapse_below_width":480`, 1)
	if overflows := SmallScreenOverflows(pageJSON(t, fixed)); len(overflows) != 0 {
		t.Fatalf("a grid that collapses on a phone was still reported: %v", overflows)
	}
}

func TestAnSmOverrideToOneColumnClearsIt(t *testing.T) {
	fixed := strings.Replace(erFormRow,
		`"properties":{"base":{"grid_template_columns":"1fr 48px 1fr","gap":12,"collapse_below_width":320}}`,
		`"properties":{"base":{"grid_template_columns":"1fr 48px 1fr","gap":12},"sm":{"grid_template_columns":"1fr"}}`, 1)
	if overflows := SmallScreenOverflows(pageJSON(t, fixed)); len(overflows) != 0 {
		t.Fatalf("an sm override was ignored: %v", overflows)
	}
}

func TestARowOfTextAndIconsIsNotReported(t *testing.T) {
	// Text and icons shrink; only form controls hold a floor.
	page := pageJSON(t, `{"components":[{"id":"hint","type":"flex_container","properties":{"base":{"flex_direction":"row","gap":12}},"children":[
	  {"id":"i","type":"icon"},{"id":"t","type":"text"}]}]}`)
	if overflows := SmallScreenOverflows(page); len(overflows) != 0 {
		t.Fatalf("a shrinkable row was reported: %v", overflows)
	}
}

func TestAWrappingRowIsNotReported(t *testing.T) {
	page := pageJSON(t, `{"components":[{"id":"row","type":"flex_container","properties":{"base":{"flex_direction":"row","flex_wrap":"wrap"}},"children":[
	  {"id":"a","type":"select-eru"},{"id":"b","type":"date"}]}]}`)
	if overflows := SmallScreenOverflows(page); len(overflows) != 0 {
		t.Fatalf("a wrapping row was reported: %v", overflows)
	}
}

func TestAColumnOfControlsIsNotReported(t *testing.T) {
	page := pageJSON(t, `{"components":[{"id":"col","type":"flex_container","properties":{"base":{"flex_direction":"column","gap":20}},"children":[
	  {"id":"a","type":"date"},{"id":"b","type":"number"}]}]}`)
	if overflows := SmallScreenOverflows(page); len(overflows) != 0 {
		t.Fatalf("a stacked column was reported: %v", overflows)
	}
}

func TestTheNoteNamesTheFixRatherThanJustTheFault(t *testing.T) {
	note := SmallScreenNote(SmallScreenOverflows(pageJSON(t, erFormRow)))
	for _, want := range []string{"pair_row", "properties.sm", "grid_template_columns", "collapse_below_width"} {
		if !strings.Contains(note, want) {
			t.Errorf("the note does not mention %q:\n%s", want, note)
		}
	}
}

func TestAPageWithNoRowsProducesNoNote(t *testing.T) {
	if note := SmallScreenNote(nil); note != "" {
		t.Fatalf("got %q", note)
	}
}
