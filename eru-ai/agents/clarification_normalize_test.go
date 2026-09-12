package agents

import "testing"

// The failure this guards against: a question with no options and
// allow_free_text false renders no choices and no text box, so the wizard has
// nothing to answer and can never be submitted.
func TestParseClarificationRequestMakesAQuestionAnswerable(t *testing.T) {
	action := map[string]interface{}{
		"questions": []interface{}{
			map[string]interface{}{
				"id":              "q1",
				"question":        "which grid container?",
				"allow_free_text": false,
				"free_text_label": "Paste the component ID here",
				"required":        true,
			},
		},
	}

	req, err := ParseClarificationRequest(action)
	if err != nil {
		t.Fatal(err)
	}
	q := req.Questions[0]
	if !q.AllowFreeText {
		t.Error("a question with no options must accept free text, or it cannot be answered")
	}
	if q.FreeTextLabel != "Paste the component ID here" {
		t.Errorf("the asker's own label was replaced: %q", q.FreeTextLabel)
	}
}

func TestNormalizeFillsIdsAndTheDefaultLabel(t *testing.T) {
	req := ClarificationRequest{Questions: []ClarificationQuestion{{Question: "first?"}, {Question: "second?"}}}
	req.Normalize()
	if req.Questions[0].Id != "q1" || req.Questions[1].Id != "q2" {
		t.Errorf("ids not filled: %q %q", req.Questions[0].Id, req.Questions[1].Id)
	}
	if req.Questions[0].FreeTextLabel != DefaultFreeTextLabel {
		t.Errorf("default label not applied: %q", req.Questions[0].FreeTextLabel)
	}
}
