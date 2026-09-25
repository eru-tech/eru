package orchestrator

import (
	"strings"
	"testing"

	agents "github.com/eru-tech/eru/eru-ai/agents"
	functions "github.com/eru-tech/eru/eru-functions/functions"
)

func pageBody(pages ...map[string]interface{}) map[string]interface{} {
	list := make([]interface{}, 0, len(pages))
	for _, p := range pages {
		list = append(list, p)
	}
	return map[string]interface{}{"actions": []interface{}{
		map[string]interface{}{"action_type": "answer", "action": map[string]interface{}{"pages": list}},
	}}
}

func TestCollectGeneratedPagesFindsNestedPages(t *testing.T) {
	resVars := map[string]*functions.TemplateVars{
		"eru_studio": {Body: pageBody(
			map[string]interface{}{"page_id": "business_cards", "page": map[string]interface{}{"title": "Business Cards"}},
			map[string]interface{}{"page_id": "card_form", "page": map[string]interface{}{"name": "Card Form"}},
		)},
		"processo_builder": {Body: map[string]interface{}{"summary": "made an entity"}},
	}

	pages := collectGeneratedPages(resVars)
	if len(pages) != 2 {
		t.Fatalf("expected both pages, got %d: %+v", len(pages), pages)
	}
	if pageName(pages[0]) != "Business Cards" || pageName(pages[1]) != "Card Form" {
		t.Errorf("unexpected names: %q, %q", pageName(pages[0]), pageName(pages[1]))
	}
}

// An entry without a definition is not a page - a step that merely mentions a
// page id must not produce a save prompt for something that does not exist.
func TestCollectGeneratedPagesIgnoresIdWithoutDefinition(t *testing.T) {
	resVars := map[string]*functions.TemplateVars{
		"step": {Body: map[string]interface{}{"page_id": "just_a_reference"}},
	}
	if pages := collectGeneratedPages(resVars); len(pages) != 0 {
		t.Errorf("expected no pages, got %+v", pages)
	}
}

func TestCollectGeneratedPagesDeduplicates(t *testing.T) {
	page := map[string]interface{}{"page_id": "p1", "page": map[string]interface{}{"title": "One"}}
	resVars := map[string]*functions.TemplateVars{
		"a": {Body: pageBody(page)},
		"b": {Body: pageBody(page)},
	}
	if pages := collectGeneratedPages(resVars); len(pages) != 1 {
		t.Errorf("the same page must be offered once, got %d", len(pages))
	}
}

func TestPageSaveRequestNamesThePagesAndWarns(t *testing.T) {
	req := pageSaveRequest([]map[string]interface{}{
		{"page_id": "p1", "page": map[string]interface{}{"title": "Business Cards"}},
	})
	if len(req.Questions) != 1 {
		t.Fatalf("expected one question, got %d", len(req.Questions))
	}
	q := req.Questions[0]
	if q.Id != pageSaveQuestionId {
		t.Errorf("question id = %q", q.Id)
	}
	if !strings.Contains(q.Question, "Business Cards") {
		t.Errorf("the question should name the page: %q", q.Question)
	}
	if !strings.Contains(q.Question, "1 page was built") {
		t.Errorf("a single page should read as singular: %q", q.Question)
	}
	if !strings.Contains(q.Question, "lost when you navigate away") {
		t.Errorf("the question should say what happens if they decline: %q", q.Question)
	}
	if len(q.Options) != 2 {
		t.Errorf("expected save/don't-save options, got %+v", q.Options)
	}
}

func TestAnsweredYesToPageSave(t *testing.T) {
	yes := []agents.ClarificationAnswer{{QuestionId: pageSaveQuestionId, Selected: []string{pageSaveYes}}}
	if !answeredYesToPageSave(yes) {
		t.Error("an explicit save must be honoured")
	}
	no := []agents.ClarificationAnswer{{QuestionId: pageSaveQuestionId, Selected: []string{pageSaveNo}}}
	if answeredYesToPageSave(no) {
		t.Error("declining must not save")
	}
	// A step-prefixed id still refers to the same question.
	prefixed := []agents.ClarificationAnswer{{QuestionId: "eru_studio::" + pageSaveQuestionId, Selected: []string{pageSaveYes}}}
	if !answeredYesToPageSave(prefixed) {
		t.Error("a prefixed question id must still match")
	}
	if answeredYesToPageSave(nil) {
		t.Error("no answer must not be read as consent to write")
	}
}

// The resume guard used to require a paused branch, so a checkpoint carrying
// only pages was ignored: the answer became message text and the whole request
// was planned again instead of the pages being saved.
func TestPageOnlyCheckpointIsResumable(t *testing.T) {
	pr := PendingResume{PagesToSave: []map[string]interface{}{
		{"page_id": "p1", "page": map[string]interface{}{"title": "Dash"}},
	}}
	if len(pr.PausedBranches) != 0 {
		t.Fatal("a page-save checkpoint has no paused branch by construction")
	}
	resumable := len(pr.PausedBranches) > 0 || len(pr.PagesToSave) > 0
	if !resumable {
		t.Error("a checkpoint with pages waiting must be resumable")
	}
}

// The studio agent answers {mode, page} with the id on the page. A build that
// arrived this way was collected as zero pages: nothing was offered, nothing
// was said, and the dashboard was lost on the next navigation.
func TestCollectGeneratedPagesTakesTheIdFromThePage(t *testing.T) {
	resVars := map[string]*functions.TemplateVars{
		"eru_studio": {Body: map[string]interface{}{"actions": []interface{}{
			map[string]interface{}{"action_type": "answer", "action": map[string]interface{}{
				"mode": "full",
				"page": map[string]interface{}{
					"id": "3b757edb", "name": "Management Dashboard",
					"components": []interface{}{},
				},
			}},
		}}},
	}
	pages := collectGeneratedPages(resVars)
	if len(pages) != 1 {
		t.Fatalf("expected the page to be found, got %d: %+v", len(pages), pages)
	}
	if pages[0]["page_id"] != "3b757edb" {
		t.Errorf("the id must come from the page, got %v", pages[0]["page_id"])
	}
	if pageName(pages[0]) != "Management Dashboard" {
		t.Errorf("unexpected name %q", pageName(pages[0]))
	}
}

// A page with no id anywhere cannot be addressed by save_page, so it is not
// offered rather than offered and failed.
func TestCollectGeneratedPagesSkipsAPageWithNoId(t *testing.T) {
	resVars := map[string]*functions.TemplateVars{
		"eru_studio": {Body: map[string]interface{}{
			"page": map[string]interface{}{"name": "Nameless"},
		}},
	}
	if pages := collectGeneratedPages(resVars); len(pages) != 0 {
		t.Errorf("expected no pages, got %+v", pages)
	}
}
