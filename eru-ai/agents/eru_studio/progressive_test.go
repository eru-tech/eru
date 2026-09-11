package eru_studio

import (
	"reflect"
	"strings"
	"testing"
)

// feed splits a payload into chunks of n bytes, the way a model streams it, and
// collects everything the scanner emits.
func feed(payload string, n int) []ScannedComponent {
	scanner := NewComponentScanner()
	out := []ScannedComponent{}
	for i := 0; i < len(payload); i += n {
		end := i + n
		if end > len(payload) {
			end = len(payload)
		}
		out = append(out, scanner.Write(payload[i:end])...)
	}
	return out
}

const patchPayload = `{"mode":"patch","patch":{"page_id":"p1","upsert":[` +
	`{"id":"root","type":"flex_container","children_ids":["a","b"],"properties":{"base":{"gap":8}}},` +
	`{"id":"a","type":"text","parent_id":"root","properties":{"base":{"label":"Hi"}}},` +
	`{"id":"b","type":"button","parent_id":"root","properties":{"base":{"label":"Go"}}}` +
	`],"delete":["old_btn"]}}`

func ids(components []ScannedComponent) []string {
	out := make([]string, 0, len(components))
	for _, component := range components {
		out = append(out, component.Id)
	}
	return out
}

func TestScannerEmitsEachComponentAsItCloses(t *testing.T) {
	// Every chunk size must give the same answer: the scanner cannot depend on
	// where the network happened to split the stream.
	for _, size := range []int{1, 3, 7, 32, 500} {
		got := feed(patchPayload, size)
		if want := []string{"root", "a", "b"}; !reflect.DeepEqual(ids(got), want) {
			t.Errorf("chunk size %d emitted %v, want %v", size, ids(got), want)
		}
		if got[0].Source != "upsert" {
			t.Errorf("chunk size %d: source = %q", size, got[0].Source)
		}
		if !reflect.DeepEqual(got[0].ChildrenIds, []string{"a", "b"}) {
			t.Errorf("chunk size %d: children_ids = %v", size, got[0].ChildrenIds)
		}
		if got[1].ParentId != "root" || got[1].Type != "text" {
			t.Errorf("chunk size %d: adjacency was lost: %+v", size, got[1])
		}
		for i, component := range got {
			if component.Sequence != i+1 {
				t.Errorf("chunk size %d: component %d has sequence %d", size, i, component.Sequence)
			}
		}
	}
}

func TestScannerEmitsBeforeTheResponseIsComplete(t *testing.T) {
	// The whole point: a component is usable before the JSON is closed.
	truncated := patchPayload[:strings.Index(patchPayload, `{"id":"b"`)]
	got := feed(truncated, 5)
	if want := []string{"root", "a"}; !reflect.DeepEqual(ids(got), want) {
		t.Errorf("a truncated stream emitted %v, want %v", ids(got), want)
	}
}

func TestScannerDoesNotEmitNestedChildrenAsSiblings(t *testing.T) {
	payload := `{"page":{"components":[` +
		`{"id":"root","type":"flex_container","children":[` +
		`{"id":"inner","type":"text","properties":{"base":{}}}` +
		`]}` +
		`]}}`
	got := feed(payload, 4)
	if want := []string{"root"}; !reflect.DeepEqual(ids(got), want) {
		t.Errorf("emitted %v, want just the root - a nested child is part of its parent", ids(got))
	}
	// The nested child still travels inside its parent.
	children, _ := got[0].Component["children"].([]interface{})
	if len(children) != 1 {
		t.Errorf("the parent lost its children: %v", got[0].Component)
	}
}

func TestScannerIgnoresArrayNamesInsideStrings(t *testing.T) {
	// A label that happens to contain the word must not arm the scanner.
	payload := `{"page":{"title":"upsert: [not a component]","components":[` +
		`{"id":"only","type":"text","properties":{"base":{"label":"components\":[\"x\"]"}}}` +
		`]}}`
	got := feed(payload, 3)
	if want := []string{"only"}; !reflect.DeepEqual(ids(got), want) {
		t.Errorf("emitted %v, want %v", ids(got), want)
	}
}

func TestScannerHandlesEscapesAndBracesInsideStrings(t *testing.T) {
	payload := `{"patch":{"upsert":[` +
		`{"id":"a","type":"text","properties":{"base":{"label":"a \"}\" and a \\\\ backslash"}}},` +
		`{"id":"b","type":"text","properties":{"base":{"label":"{[}]"}}}` +
		`]}}`
	got := feed(payload, 2)
	if want := []string{"a", "b"}; !reflect.DeepEqual(ids(got), want) {
		t.Errorf("emitted %v, want %v", ids(got), want)
	}
}

func TestScannerSkipsComponentsWithNoId(t *testing.T) {
	payload := `{"patch":{"upsert":[{"type":"text"},{"id":"real","type":"text"}]}}`
	got := feed(payload, 6)
	if want := []string{"real"}; !reflect.DeepEqual(ids(got), want) {
		t.Errorf("emitted %v, want %v - a component with no id cannot be keyed by a renderer", ids(got), want)
	}
}

func TestScannerStopsAtTheEndOfTheComponentArray(t *testing.T) {
	// A second array later in the payload must not restart the scan; the page has
	// one component list and the rest is scaffolding.
	payload := `{"patch":{"upsert":[{"id":"a","type":"text"}],"state":{"upsert":[{"key":"k"}]}}}`
	got := feed(payload, 4)
	if want := []string{"a"}; !reflect.DeepEqual(ids(got), want) {
		t.Errorf("emitted %v, want %v", ids(got), want)
	}
}

func TestScannerCountsWhatItEmitted(t *testing.T) {
	scanner := NewComponentScanner()
	scanner.Write(patchPayload)
	if got := scanner.Count(); got != 3 {
		t.Errorf("Count() = %d, want 3", got)
	}
}

func TestScannerIsSafeWhenAbsent(t *testing.T) {
	var scanner *ComponentScanner
	if got := scanner.Write("{}"); got != nil {
		t.Errorf("a nil scanner emitted %v", got)
	}
	if got := scanner.Count(); got != 0 {
		t.Errorf("a nil scanner counted %d", got)
	}
	if got := ComponentScannerFrom(nil); got != nil {
		t.Error("a nil context produced a scanner")
	}
}

func TestScannerHandlesAnEmptyComponentArray(t *testing.T) {
	if got := feed(`{"patch":{"upsert":[]}}`, 3); len(got) != 0 {
		t.Errorf("an empty array emitted %v", got)
	}
}
