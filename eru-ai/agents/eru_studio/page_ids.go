package eru_studio

import "sort"

// ComponentIds lists every component id on a page, at any depth, sorted.
//
// It exists so an answer can be compared with the page it claims to be a new
// version of: a full-mode answer is supposed to carry the whole page back, and
// the only way to tell that it did not is to look at what is missing.
func ComponentIds(page map[string]interface{}) []string {
	flat := Flatten(page)
	if flat == nil {
		return nil
	}
	ids := make([]string, 0, len(flat.nodes))
	for id := range flat.nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
