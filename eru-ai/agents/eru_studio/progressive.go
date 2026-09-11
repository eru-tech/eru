package eru_studio

import (
	"context"
	"encoding/json"
	"sync"
)

// A structured-output agent's answer is the argument of a tool call, so nothing
// reaches the client until the model has finished writing the whole page. The
// scanner below reads those arguments as they stream and hands back each
// component the moment its closing brace arrives, which is what lets a renderer
// draw a page while it is still being written.

// scannedArrays are the arrays whose elements are worth streaming: the patch's
// component list, and a full page's root components.
var scannedArrays = []string{"upsert", "components"}

// ScannedComponent is one component lifted out of the model's partial output.
type ScannedComponent struct {
	// Sequence counts components within one response, from 1.
	Sequence int `json:"sequence"`
	// Source is the array the component came from: "upsert" for a patch,
	// "components" for a full page.
	Source string `json:"source"`
	Id     string `json:"id,omitempty"`
	Type   string `json:"type,omitempty"`
	// ParentId and ChildrenIds are the adjacency the renderer needs to place the
	// component before the rest of the page exists.
	ParentId    string                 `json:"parent_id,omitempty"`
	ChildrenIds []string               `json:"children_ids,omitempty"`
	Component   map[string]interface{} `json:"component"`
}

// ComponentScanner turns a stream of partial JSON into complete components.
//
// It is a byte scanner rather than a JSON parser because the input is not valid
// JSON until the last chunk: it tracks string and escape state, finds the first
// component array, and yields each element of that array as it closes. Elements
// of nested arrays are not yielded - a component that arrives inside another's
// "children" is part of that component, not a sibling.
type ComponentScanner struct {
	mu sync.Mutex

	buf []byte

	inString bool
	escaped  bool
	depth    int

	// keyStart marks the opening quote of the string being read, so a closing
	// quote can be compared against the array names worth scanning.
	keyStart int
	// pendingKey is set once a component-array name has been read and cleared by
	// whatever value follows it.
	pendingKey bool

	// arrayDepth is the depth of the array being scanned, or -1 when the scanner
	// has not found one yet.
	arrayDepth int
	source     string
	elemStart  int

	sequence int
	done     bool
}

func NewComponentScanner() *ComponentScanner {
	return &ComponentScanner{arrayDepth: -1, elemStart: -1, keyStart: -1}
}

// Write feeds the next chunk of tool arguments and returns the components that
// became complete within it. Feeding the same stream twice, or feeding chunks out
// of order, produces nonsense - one scanner belongs to one tool call.
func (s *ComponentScanner) Write(chunk string) []ScannedComponent {
	if s == nil || chunk == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.done {
		return nil
	}

	start := len(s.buf)
	s.buf = append(s.buf, chunk...)

	var found []ScannedComponent
	for i := start; i < len(s.buf); i++ {
		c := s.buf[i]

		if s.inString {
			switch {
			case s.escaped:
				s.escaped = false
			case c == '\\':
				s.escaped = true
			case c == '"':
				s.inString = false
				if s.arrayDepth < 0 && s.keyStart >= 0 {
					if name := string(s.buf[s.keyStart+1 : i]); isScannedArray(name) {
						s.pendingKey = true
						s.source = name
					}
				}
				s.keyStart = -1
			}
			continue
		}

		switch c {
		case '"':
			s.inString = true
			s.keyStart = i
		case '[':
			s.depth++
			if s.pendingKey && s.arrayDepth < 0 {
				s.arrayDepth = s.depth
			}
			s.pendingKey = false
		case ']':
			if s.arrayDepth == s.depth {
				// The component array has closed; everything after it is page
				// scaffolding the client does not need streamed.
				s.arrayDepth = -1
				s.done = true
			}
			s.depth--
		case '{':
			s.depth++
			if s.arrayDepth >= 0 && s.depth == s.arrayDepth+1 {
				s.elemStart = i
			}
			s.pendingKey = false
		case '}':
			if s.arrayDepth >= 0 && s.depth == s.arrayDepth+1 && s.elemStart >= 0 {
				if component := s.emit(s.buf[s.elemStart : i+1]); component != nil {
					found = append(found, *component)
				}
				s.elemStart = -1
			}
			s.depth--
		case ' ', '\t', '\n', '\r', ':', ',':
			// Whitespace and separators leave a pending key alone: the array may
			// still be a few bytes away.
		default:
			s.pendingKey = false
		}
	}
	return found
}

func (s *ComponentScanner) emit(raw []byte) *ScannedComponent {
	var component map[string]interface{}
	if err := json.Unmarshal(raw, &component); err != nil {
		return nil
	}
	id, _ := component["id"].(string)
	if id == "" {
		// Without an id the renderer cannot key it, and a patch cannot address
		// it - not something worth streaming.
		return nil
	}
	s.sequence++
	componentType, _ := component["type"].(string)
	parentId, _ := component["parent_id"].(string)
	return &ScannedComponent{
		Sequence:    s.sequence,
		Source:      s.source,
		Id:          id,
		Type:        componentType,
		ParentId:    parentId,
		ChildrenIds: toStringSlice(component["children_ids"]),
		Component:   component,
	}
}

// Count reports how many components the scanner has emitted, which is what a
// caller needs to decide whether progressive rendering actually happened.
func (s *ComponentScanner) Count() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sequence
}

func isScannedArray(name string) bool {
	for _, candidate := range scannedArrays {
		if candidate == name {
			return true
		}
	}
	return false
}

const scannerKey contextKey = "eru_studio_component_scanner"

// WithComponentScanner attaches a scanner to the request. It lives in the
// context because a scanner belongs to one response, while the agent object is
// shared across them.
func WithComponentScanner(ctx context.Context, scanner *ComponentScanner) context.Context {
	return context.WithValue(ctx, scannerKey, scanner)
}

// ComponentScannerFrom returns this request's scanner, or nil when the request
// is not being streamed.
func ComponentScannerFrom(ctx context.Context) *ComponentScanner {
	if ctx == nil {
		return nil
	}
	scanner, _ := ctx.Value(scannerKey).(*ComponentScanner)
	return scanner
}
