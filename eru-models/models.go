package eru_models

type Queries struct {
	Query string
	Vals  []interface{}
	Rank  int
}

type QueriesSorter []*Queries

func (a QueriesSorter) Len() int {
	return len(a)
}
func (a QueriesSorter) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}
func (a QueriesSorter) Less(i, j int) bool {
	return a[i].Rank < a[j].Rank
}

// JSONSchema represents a JSON schema definition
type JSONSchema struct {
	Type        string                `json:"type"`
	Properties  map[string]JSONSchema `json:"properties,omitempty"`
	Required    []string              `json:"required,omitempty"`
	Items       *JSONSchema           `json:"items,omitempty"`  // For arrays
	Enum        []interface{}         `json:"enum,omitempty"`   // For enums
	Format      string                `json:"format,omitempty"` // For strings
	Description string                `json:"description,omitempty"`
}
