package eval

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

// Outcome is the workspace after a run: what is actually there, as opposed to
// what the agent did or what it said it did.
//
// Asserting actions turned out to be state-dependent in a way that produced a
// false failure on its third live run. The business-card scenario demanded three
// save_entity calls; the entities already existed from an earlier run, so the
// agent correctly extended them instead of duplicating - obeying the
// extend-vs-create rule - and made zero save_entity calls. The run was right and
// the assertion was wrong.
//
// An outcome assertion does not care how the workspace got that way, which is
// the only kind that survives being run twice.
type Outcome struct {
	Entities map[string]EntityState `json:"entities"`
	// Read says the workspace was actually looked at.
	//
	// An explicit flag rather than "are there any entities", because the two are
	// genuinely different states and confusing them cost a day: an unread
	// workspace was reported as one that did not contain the entity, for an
	// entity that was sitting right there. A workspace CAN legitimately be
	// empty, so emptiness cannot stand in for this.
	Read bool `json:"read,omitempty"`
}

// EntityState is one entity as the workspace holds it.
type EntityState struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"display_name"`
	Fields      []string `json:"fields"`
	// FieldAttrs is each field's stored definition, keyed by field name.
	//
	// Names alone answer "is there somewhere to put this", which is all the
	// build scenarios needed. They cannot answer "is it the RIGHT field" - an
	// attachment scenario cares that the field is an attachment pointed at the
	// storage the user named, and a field of the wrong datatype passes a
	// names-only check while being useless.
	FieldAttrs map[string]map[string]interface{} `json:"field_attrs,omitempty"`
}

// ReadOutcome loads the workspace's entities through the same stored query the
// product reads them with.
func (r *Runner) ReadOutcome(ctx context.Context, eruqlURL, orgId, processId string) (Outcome, error) {
	body, err := json.Marshal(map[string]string{"org_id": orgId, "process_id": processId})
	if err != nil {
		return Outcome{}, err
	}
	url := fmt.Sprintf("%s/store/%s/myquery/execute/get_process_metadata", strings.TrimRight(eruqlURL, "/"), r.Project)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return Outcome{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("claims", r.Claims)

	response, err := r.Client.Do(request)
	if err != nil {
		return Outcome{}, err
	}
	defer response.Body.Close()

	var decoded interface{}
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		return Outcome{}, fmt.Errorf("decoding the workspace: %w", err)
	}
	return outcomeFrom(decoded), nil
}

// outcomeFrom picks the entity list out of the response, wherever it is nested.
func outcomeFrom(body interface{}) Outcome {
	out := Outcome{Entities: map[string]EntityState{}, Read: true}
	var walk func(interface{})
	walk = func(node interface{}) {
		switch typed := node.(type) {
		case map[string]interface{}:
			if list, ok := typed["entities"].([]interface{}); ok {
				for _, raw := range list {
					entity, ok := raw.(map[string]interface{})
					if !ok {
						continue
					}
					name, _ := entity["name"].(string)
					if name == "" {
						continue
					}
					state := EntityState{Name: name}
					state.DisplayName, _ = entity["display_name"].(string)
					if fields, ok := entity["fields"].([]interface{}); ok {
						state.FieldAttrs = map[string]map[string]interface{}{}
						for _, f := range fields {
							if field, ok := f.(map[string]interface{}); ok {
								if fieldName, _ := field["name"].(string); fieldName != "" {
									state.Fields = append(state.Fields, fieldName)
									state.FieldAttrs[fieldName] = field
								}
							}
						}
						sort.Strings(state.Fields)
					}
					out.Entities[name] = state
				}
				return
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
	return out
}

// Has reports whether an entity exists, by name or by display name - an agent
// choosing "BusinessCard" over "business_card" is a naming choice, not a failure.
func (o Outcome) Has(name string) (EntityState, bool) {
	if state, ok := o.Entities[name]; ok {
		return state, true
	}
	want := normalise(name)
	for _, state := range o.Entities {
		if normalise(state.Name) == want || normalise(state.DisplayName) == want {
			return state, true
		}
	}
	return EntityState{}, false
}

func normalise(name string) string {
	return strings.ReplaceAll(strings.ReplaceAll(strings.ToLower(strings.TrimSpace(name)), "_", ""), " ", "")
}

// EntitiesExist requires each named entity to be present afterwards, however it
// got there.
func EntitiesExist(names ...string) OutcomeExpectation {
	return OutcomeExpectation{
		Describe: fmt.Sprintf("entities exist: %s", strings.Join(names, ", ")),
		Check: func(before, after Outcome) string {
			var missing []string
			for _, name := range names {
				if _, ok := after.Has(name); !ok {
					missing = append(missing, name)
				}
			}
			if len(missing) > 0 {
				return fmt.Sprintf("not in the workspace: %s", strings.Join(missing, ", "))
			}
			return ""
		},
	}
}

// CoversTopics requires the workspace to have gained, during this run, an entity
// for each topic named.
//
// What an entity is CALLED is the agent's choice, and it varies run to run: the
// same prompt produced "Address" one day and "business_card_address" the next.
// Both are right, and a fixture naming one of them fails the other. What the
// user actually asked for is somewhere to put addresses.
//
// Matched against what the run ADDED, not against the whole workspace, because a
// bare substring would happily match the "Contacts" entity that has been there
// for years and report success for work nobody did.
func CoversTopics(topics ...string) OutcomeExpectation {
	return OutcomeExpectation{
		Describe: fmt.Sprintf("this run adds somewhere to hold: %s", strings.Join(topics, ", ")),
		Check: func(before, after Outcome) string {
			added := map[string]EntityState{}
			for name, state := range after.Entities {
				if _, existed := before.Entities[name]; !existed {
					added[name] = state
				}
			}
			var missing []string
			for _, topic := range topics {
				want := normalise(topic)
				found := false
				for _, state := range added {
					if strings.Contains(normalise(state.Name), want) || strings.Contains(normalise(state.DisplayName), want) {
						found = true
						break
					}
				}
				if !found {
					missing = append(missing, topic)
				}
			}
			if len(missing) > 0 {
				return fmt.Sprintf("nothing added for %s (added: %s)", strings.Join(missing, ", "), listNames(added))
			}
			return ""
		},
	}
}

func listNames(entities map[string]EntityState) string {
	if len(entities) == 0 {
		return "nothing"
	}
	names := make([]string, 0, len(entities))
	for name := range entities {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// EntityHasFields requires an entity to carry these fields.
func EntityHasFields(entity string, fields ...string) OutcomeExpectation {
	return OutcomeExpectation{
		Describe: fmt.Sprintf("%s has %s", entity, strings.Join(fields, ", ")),
		Check: func(before, after Outcome) string {
			state, ok := after.Has(entity)
			if !ok {
				return fmt.Sprintf("%s is not in the workspace", entity)
			}
			present := map[string]bool{}
			for _, field := range state.Fields {
				present[normalise(field)] = true
			}
			var missing []string
			for _, field := range fields {
				if !present[normalise(field)] {
					missing = append(missing, field)
				}
			}
			if len(missing) > 0 {
				return fmt.Sprintf("%s is missing %s (has %s)", entity, strings.Join(missing, ", "), strings.Join(state.Fields, ", "))
			}
			return ""
		},
	}
}

// OutcomeExpectation is an assertion over the workspace rather than the run.
type OutcomeExpectation struct {
	Describe string
	// Check sees the workspace before the run and after it. The diff is what
	// makes "this run added somewhere for addresses" expressible without naming
	// the entity the agent will choose.
	Check func(before, after Outcome) string
}

// EntityFieldHas requires a field to exist on an entity and to carry a
// particular attribute value.
//
// This is the assertion an "add a field like this" scenario actually wants.
// Asserting that save_field was CALLED is state-dependent in a way that bites on
// the second run: the field is already there from the first, and the correct
// behaviour becomes doing nothing, which is indistinguishable from failing. The
// user does not care whether this run wrote it. They care that it is right.
func EntityFieldHas(entity, field, attribute string, want interface{}) OutcomeExpectation {
	return OutcomeExpectation{
		Describe: fmt.Sprintf("%s.%s has %s=%v", entity, field, attribute, want),
		Check: func(before, after Outcome) string {
			state, ok := after.Has(entity)
			if !ok {
				return fmt.Sprintf("the workspace has no entity %q", entity)
			}
			attrs, ok := state.FieldAttrs[field]
			if !ok {
				return fmt.Sprintf("%s has no field %q (it has: %s)", entity, field, listOrNothing(state.Fields))
			}
			got, present := attrs[attribute]
			if !present {
				return fmt.Sprintf("%s.%s has no %s at all", entity, field, attribute)
			}
			if fmt.Sprint(got) != fmt.Sprint(want) {
				return fmt.Sprintf("%s.%s has %s=%v, want %v", entity, field, attribute, got, want)
			}
			return ""
		},
	}
}

func listOrNothing(names []string) string {
	if len(names) == 0 {
		return "nothing"
	}
	return strings.Join(names, ", ")
}
