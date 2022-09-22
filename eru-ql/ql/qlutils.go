package ql

func GetQL(queryType string) QL {
	switch queryType {
	case "graphql":
		return new(GraphQLData)
	case "sql":
		return new(SQLData)
	default:
		return nil
		//do nothing
	}
	return nil
}

type OrderedMap struct {
	Rank int
	Obj  map[string]interface{}
}

type MapSorter []*OrderedMap

func (a MapSorter) Len() int {
	return len(a)
}
func (a MapSorter) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}
func (a MapSorter) Less(i, j int) bool {
	return a[i].Rank < a[j].Rank
}
