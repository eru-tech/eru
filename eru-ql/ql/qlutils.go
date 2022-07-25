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
