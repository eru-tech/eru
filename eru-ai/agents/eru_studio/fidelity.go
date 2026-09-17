package eru_studio

import (
	"fmt"
	"sort"
)

// An edit that answers with the whole page tends to lose things on the way.
//
// Asked to restyle a header and return the full page, the model re-emits every
// component it was given - and re-emitting is not copying. Properties it has no
// reason to change quietly fail to come back: an icon on a priority option, an
// empty events list, a blank name. Nothing reports it. The catalog has no
// complaint, because an absent optional property is not an error; the
// introduced-issues check has no complaint, because a leaf that vanished
// produces no issue on either side to compare. The page simply comes back
// slightly poorer than it went in, and does so again on the next turn, and the
// next.
//
// The honest reading is that the model told the truth about its intent - "all
// other components remain unchanged" - and then failed to carry it out. So this
// holds it to its own claim: a component the edit did not otherwise touch gets
// back whatever it had before.
//
// It is a repair and not a complaint on purpose. A validation failure that
// survives the retries aborts the whole run, so reporting this as an issue would
// turn a request the model very nearly got right into nothing at all. Putting
// the leaf back costs no model call, cannot fail, and needs no second attempt.
//
// The limit worth stating: a deliberate "remove the icon" on a component the
// request does not name is indistinguishable from drift, and will be undone. It
// is recoverable - the user says it again, more specifically, and a scoped edit
// is exempt - where silent loss on every full-page edit is not.

// restoration is one leaf that can be put back.
type restoration struct {
	path  string
	apply func()
}

// RestoreInheritedLeaves puts back the leaves that vanished from components the
// edit did not otherwise change, and reports what it restored.
//
// writable names the components the edit was licensed to change - the resolved
// scope, when the request carried one. Those are left entirely alone: a scoped
// edit that removes something removed it on purpose.
func RestoreInheritedLeaves(basePage, livePage map[string]interface{}, writable map[string]bool) []string {
	if len(basePage) == 0 || len(livePage) == 0 {
		return nil
	}
	baseById := indexComponentsById(basePage)
	liveById := indexComponentsById(livePage)

	var restored []string
	for id, baseComp := range baseById {
		if id == "" || writable[id] {
			continue
		}
		liveComp, ok := liveById[id]
		if !ok {
			// The component is gone altogether. That is a structural change, not
			// a lost leaf, and putting it back here would fight a deletion the
			// edit may well have been asked for.
			continue
		}

		var proposals []restoration
		if compareForRestore(id, baseComp, liveComp, &proposals) {
			// Something on this component was added or changed, so the edit was
			// working here and is entitled to have dropped things too.
			continue
		}
		for _, p := range proposals {
			p.apply()
			restored = append(restored, p.path)
		}
	}
	sort.Strings(restored)
	return restored
}

// indexComponentsById maps every component in a page by its id.
func indexComponentsById(page map[string]interface{}) map[string]map[string]interface{} {
	out := map[string]map[string]interface{}{}
	WalkPage(page, func(component map[string]interface{}, _ string) {
		id, _ := component["id"].(string)
		if id != "" {
			out[id] = component
		}
	})
	return out
}

// compareForRestore collects the leaves present in base and missing live, and
// reports whether live carries any addition or modification of its own.
//
// children are skipped throughout: a child is a component in its own right and
// is compared on its own terms.
func compareForRestore(path string, base, live map[string]interface{}, out *[]restoration) bool {
	changed := false
	for key, baseValue := range base {
		if key == "children" {
			continue
		}
		liveValue, present := live[key]
		if !present {
			k, v, target := key, baseValue, live
			*out = append(*out, restoration{
				path:  joinPath(path, key),
				apply: func() { target[k] = deepCopyValue(v) },
			})
			continue
		}
		if compareValueForRestore(joinPath(path, key), baseValue, liveValue, out) {
			changed = true
		}
	}
	for key := range live {
		if key == "children" {
			continue
		}
		if _, inBase := base[key]; !inBase {
			changed = true
		}
	}
	return changed
}

func compareValueForRestore(path string, base, live interface{}, out *[]restoration) bool {
	switch b := base.(type) {
	case map[string]interface{}:
		l, ok := live.(map[string]interface{})
		if !ok {
			return true
		}
		return compareForRestore(path, b, l, out)
	case []interface{}:
		l, ok := live.([]interface{})
		if !ok || len(l) != len(b) {
			// A list of a different length was rewritten, and matching its
			// entries up again would be guesswork.
			return true
		}
		changed := false
		for i := range b {
			if compareValueForRestore(fmt.Sprintf("%s[%d]", path, i), b[i], l[i], out) {
				changed = true
			}
		}
		return changed
	default:
		return fmt.Sprintf("%v", base) != fmt.Sprintf("%v", live)
	}
}

func joinPath(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}
