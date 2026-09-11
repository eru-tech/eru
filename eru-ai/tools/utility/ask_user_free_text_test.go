package utiltiy

import (
	"encoding/json"
	"testing"
)

func normalize(t *testing.T, params map[string]interface{}) askUserRequest {
	t.Helper()
	out, err := normalizeClarificationRequest(params)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(out)
	var req askUserRequest
	if err := json.Unmarshal(b, &req); err != nil {
		t.Fatal(err)
	}
	return req
}

// The question the user was shown offered three single-page compromises and
// allow_free_text: false - so there was no way to say "no, build it properly".
// Whether the options cover the situation is not the asker's call to make.
func TestTheUserCanAlwaysTypeAnAnswer(t *testing.T) {
	req := normalize(t, map[string]interface{}{
		"questions": []interface{}{
			map[string]interface{}{
				"question":        "How would you like to handle the repeatable Addresses and Contact Details sections?",
				"allow_free_text": false,
				"required":        true,
				"options": []interface{}{
					map[string]interface{}{"value": "fixed_slots", "label": "Use a fixed number of slots"},
					map[string]interface{}{"value": "single_entry", "label": "Show just one block"},
					map[string]interface{}{"value": "nested_pages", "label": "Note where nested pages are needed"},
				},
			},
		},
	})

	q := req.Questions[0]
	if !q.AllowFreeText {
		t.Error("the asker was allowed to close off the free-text answer")
	}
	if q.FreeTextLabel == "" {
		t.Error("there is no label for the free-text box, so a client has nothing to render")
	}
	// Everything the asker did decide is untouched.
	if len(q.Options) != 3 || !q.Required || q.Id != "q1" {
		t.Errorf("question = %+v", q)
	}
}

func TestAFreeTextLabelTheAskerChoseIsKept(t *testing.T) {
	req := normalize(t, map[string]interface{}{
		"questions": []interface{}{
			map[string]interface{}{"question": "Which entity?", "free_text_label": "Type the entity name"},
		},
	})
	if req.Questions[0].FreeTextLabel != "Type the entity name" {
		t.Errorf("label = %q", req.Questions[0].FreeTextLabel)
	}
}

// The model is no longer asked to decide it, so the field is gone from the
// contract it writes against.
func TestTheAskerIsNotOfferedTheChoice(t *testing.T) {
	question := *AskUserToolSchema().Properties["questions"].Items
	if _, offered := question.Properties["allow_free_text"]; offered {
		t.Error("allow_free_text is still in the ask_user schema - the model will keep setting it")
	}
	if _, named := question.Properties["free_text_label"]; !named {
		t.Error("the asker can no longer name the free-text box")
	}
}
