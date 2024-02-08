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
