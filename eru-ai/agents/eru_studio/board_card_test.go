package eru_studio

import (
	"strings"
	"testing"

	catalog "github.com/eru-tech/eru/eru-ai/agents/eru_studio/catalog"
)

// The board-needs-a-card-page rule moved into the catalog rule table, which the
// component validator enforces and the system prompt is generated from. These
// tests follow it there: the behaviour is unchanged, the home is not.
func gridIssues(page map[string]interface{}) []catalog.Issue {
	return append(catalog.Get().ValidatePage(page), ValidateMounts(page, nil, nil)...)
}

func gridPage(viewMode string, cardPageId string) map[string]interface{} {
	return map[string]interface{}{
		"id": "p1",
		"components": []interface{}{
			map[string]interface{}{
				"id":     "financier_grid",
				"type":   "grid",
				"styles": map[string]interface{}{},
				"properties": map[string]interface{}{
					"base": map[string]interface{}{
						"view_mode":    viewMode,
						"card_page_id": cardPageId,
					},
				},
			},
		},
	}
}

// A board draws every record as a card, so a board with no card page hands the
// user a default they cannot design - which is what "board view" was asking for.
func TestBoardGridWithoutACardPageIsReported(t *testing.T) {
	issues := gridIssues(gridPage("board", ""))
	found := false
	for _, issue := range issues {
		if strings.Contains(issue.Message, "board view") && strings.Contains(issue.Message, "card_page_id") {
			found = true
		}
	}
	if !found {
		t.Errorf("a board with no card page was not reported: %v", issues)
	}
}

// In table mode the property is not even shown in the designer, so requiring it
// would be nonsense.
func TestTableGridWithoutACardPageIsFine(t *testing.T) {
	for _, issue := range gridIssues(gridPage("table", "")) {
		if strings.Contains(issue.Message, "card_page_id") {
			t.Errorf("a table grid was asked for a card page: %s", issue.Message)
		}
	}
}

// A board that names its card page is complete; the page itself is checked by
// the mount rules already in place.
func TestBoardGridWithACardPageIsNotReportedForTheCard(t *testing.T) {
	page := gridPage("board", "card_pg")
	issues := append(catalog.Get().ValidatePage(page), ValidateMounts(page, nil, map[string]bool{"card_pg": true})...)
	for _, issue := range issues {
		if strings.Contains(issue.Message, "no \"card_page_id\"") {
			t.Errorf("a board WITH a card page was still reported: %s", issue.Message)
		}
	}
}

// The rule used to live in the mount validator as well as the prompt. Reporting
// it from both places would have shown the user the same complaint twice.
func TestTheBoardRuleIsReportedExactlyOnce(t *testing.T) {
	page := gridPage("board", "")
	count := 0
	for _, issue := range gridIssues(page) {
		if issue.Code == catalog.CodeMountBoardCardUnset {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected the board rule once, got it %d time(s)", count)
	}
}
