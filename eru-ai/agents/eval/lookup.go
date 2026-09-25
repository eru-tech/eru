package eval

import "strings"

// lookup reads a dotted path out of a tool input: "field.datatype" reaches into
// the nested object a save payload wraps its work in.
func lookup(bag map[string]interface{}, path string) interface{} {
	var node interface{} = bag
	for _, segment := range strings.Split(path, ".") {
		current, ok := node.(map[string]interface{})
		if !ok {
			return nil
		}
		node, ok = current[segment]
		if !ok {
			return nil
		}
	}
	return node
}

// valuesAt collects every value stored under key anywhere in a nested body.
//
// An answer nests differently per agent - the builder reports fields under
// "fields", the page agent under "pages" - and an expectation that had to know
// each shape would be the agent-specific thing this package exists to avoid.
func valuesAt(body interface{}, key string) []interface{} {
	var found []interface{}
	var walk func(interface{})
	walk = func(node interface{}) {
		switch typed := node.(type) {
		case map[string]interface{}:
			if value, ok := typed[key]; ok {
				found = append(found, value)
			}
			for _, child := range typed {
				walk(child)
			}
		case []interface{}:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(body)
	return found
}

// boundQueries is every saved query a produced page reads from: a component
// naming a query AND sourcing its values from one.
func boundQueries(body interface{}) []string {
	var names []string
	var walk func(interface{})
	walk = func(node interface{}) {
		switch typed := node.(type) {
		case map[string]interface{}:
			source, _ := typed["value_source"].(string)
			dataSource, _ := typed["data_source"].(string)
			if source == "query" || dataSource == "query" {
				if name, _ := typed["query"].(string); strings.TrimSpace(name) != "" {
					names = append(names, strings.TrimSpace(name))
				}
			}
			for _, child := range typed {
				walk(child)
			}
		case []interface{}:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(body)
	return names
}
