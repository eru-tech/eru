package reasoning_agents

import (
	"encoding/json"
	"strings"
	"testing"
)

func pageJSON(t *testing.T, raw string) map[string]interface{} {
	t.Helper()
	var page map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &page); err != nil {
		t.Fatalf("bad fixture: %v", err)
	}
	return page
}

const filterPage = `{"components":[{"id":"row","type":"flex_container","children":[
  {"id":"f_from","type":"date","properties":{"base":{"value_source":"state","state_key":"s_from","name":"rtdt_from"}}},
  {"id":"f_to","type":"date","properties":{"base":{"value_source":"state","state_key":"s_to","name":"rtdt_to","default_value_mode":"current_date"}}},
  {"id":"apply","type":"button","events":[{"action":"call-query","query_name":"q","api_payload_fields":["state:s_from=dt_from","state:s_to=dt_to"]}]}
]}]}`

func TestAFilterFeedingAQueryNeedsADefault(t *testing.T) {
	issues := wiringIssues(pageJSON(t, filterPage))
	if len(issues) != 1 {
		t.Fatalf("expected one issue, got %v", issues)
	}
	if !strings.Contains(issues[0].Path, "f_from") {
		t.Errorf("the wrong control was named: %s", issues[0].Path)
	}
	if !strings.Contains(issues[0].Message, "default_value_mode") {
		t.Errorf("the fix is not named: %s", issues[0].Message)
	}
}

func TestAFilterWithADefaultIsFine(t *testing.T) {
	page := pageJSON(t, strings.Replace(filterPage,
		`"state_key":"s_from","name":"rtdt_from"`,
		`"state_key":"s_from","name":"rtdt_from","default_value_mode":"current_date"`, 1))
	if issues := wiringIssues(page); len(issues) != 0 {
		t.Fatalf("a defaulted filter was faulted: %v", issues)
	}
}

func TestAControlNotSentToAQueryIsNotRequiredToHaveADefault(t *testing.T) {
	page := pageJSON(t, `{"components":[
	  {"id":"loose","type":"date","properties":{"base":{"value_source":"state","state_key":"s_other","name":"d"}}}
	]}`)
	if issues := wiringIssues(page); len(issues) != 0 {
		t.Fatalf("an unwired control was faulted: %v", issues)
	}
}

func TestTwoStateBoundControlsMayNotShareAName(t *testing.T) {
	page := pageJSON(t, `{"components":[
	  {"id":"d_from","type":"date","properties":{"base":{"value_source":"state","state_key":"a","name":"rtdt","default_value":"x"}}},
	  {"id":"d_to","type":"date","properties":{"base":{"value_source":"state","state_key":"b","name":"rtdt","default_value":"x"}}}
	]}`)
	issues := wiringIssues(page)
	if len(issues) != 1 {
		t.Fatalf("expected the shared name to be faulted, got %v", issues)
	}
	for _, want := range []string{"d_from", "d_to", "rtdt"} {
		if !strings.Contains(issues[0].Message, want) {
			t.Errorf("message does not mention %q: %s", want, issues[0].Message)
		}
	}
}

func TestDistinctNamesAreFine(t *testing.T) {
	page := pageJSON(t, `{"components":[
	  {"id":"d_from","type":"date","properties":{"base":{"value_source":"state","state_key":"a","name":"rtdt_from","default_value":"x"}}},
	  {"id":"d_to","type":"date","properties":{"base":{"value_source":"state","state_key":"b","name":"rtdt_to","default_value":"x"}}}
	]}`)
	if issues := wiringIssues(page); len(issues) != 0 {
		t.Fatalf("distinct names were faulted: %v", issues)
	}
}

func TestAPreExistingFaultIsNotTheEditsFault(t *testing.T) {
	base := pageJSON(t, filterPage)
	resolved := pageJSON(t, filterPage)
	if issues := introducedWiringIssues(base, resolved); len(issues) != 0 {
		t.Fatalf("a fault the page already had was reported: %v", issues)
	}
}

func TestAFaultThisEditAddedIsReported(t *testing.T) {
	base := pageJSON(t, `{"components":[
	  {"id":"apply","type":"button","events":[{"action":"call-query","query_name":"q","api_payload_fields":["state:s_to=dt_to"]}]}
	]}`)
	resolved := pageJSON(t, filterPage)
	issues := introducedWiringIssues(base, resolved)
	if len(issues) != 1 || !strings.Contains(issues[0].Path, "f_from") {
		t.Fatalf("the newly added filter was not reported: %v", issues)
	}
}

func TestPayloadEntryFormsAreRead(t *testing.T) {
	for _, entry := range []string{"state:s_from=dt_from", "dt_from=state:s_from", "state:s_from", " state:s_from.inner "} {
		if got := stateKeyOf(entry); got != "s_from" {
			t.Errorf("%q -> %q", entry, got)
		}
	}
	for _, entry := range []string{"s_from", "page:entity_data.fc", "app:x"} {
		if got := stateKeyOf(entry); got != "" {
			t.Errorf("%q should not name a page-state key, got %q", entry, got)
		}
	}
}
