package eval

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// Pages recorded from a real tenant are not ours to commit, so they live in the
// module's one ignored folder and the committed fixtures beside this test are
// synthetic. Whichever is present is what gets scored.
const (
	localFixtureDir  = "../../../local-testdata/studio_pages"
	localBaseline    = "../../../local-testdata/studio_baseline.json"
	sampleFixtureDir = "testdata/studio_pages"
	sampleBaseline   = "testdata/studio_baseline.json"
)

// fixtureSet is where this run's pages and their baseline are read from.
func fixtureSet() (pages string, baseline string, real bool) {
	if entries, err := os.ReadDir(localFixtureDir); err == nil && len(entries) > 0 {
		return localFixtureDir, localBaseline, true
	}
	return sampleFixtureDir, sampleBaseline, false
}

func loadPage(t *testing.T, dir string, name string) map[string]interface{} {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, name+".json"))
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	var page map[string]interface{}
	if err := json.Unmarshal(raw, &page); err != nil {
		t.Fatalf("parsing fixture %s: %v", name, err)
	}
	return page
}

// TestFixtureBaseline prints what every recorded page scores.
//
// It does not assert a number: the fixtures are pages the agent really produced,
// warts and all, and freezing today's warts as "expected" would make the harness
// defend them. It fails only when a page gets WORSE than the baseline recorded
// in baseline.json, which is what a regression looks like.
func TestFixtureBaseline(t *testing.T) {
	pagesDir, baselinePath, real := fixtureSet()
	baseline := loadBaseline(t, baselinePath)
	updated := map[string][]string{}

	entries, err := os.ReadDir(pagesDir)
	if err != nil {
		t.Skipf("no pages to score in %s (%v) - record some with golden_run.py", pagesDir, err)
	}
	if len(entries) == 0 {
		// Scoring nothing and reporting success is worse than saying there was
		// nothing to score.
		t.Skipf("no pages to score in %s - record some with golden_run.py", pagesDir)
	}
	if real {
		t.Logf("scoring pages recorded from a live tenant (%s)", pagesDir)
	} else {
		t.Logf("scoring the committed sample pages (%s)", pagesDir)
	}
	names := []string{}
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".json" {
			names = append(names, entry.Name()[:len(entry.Name())-len(".json")])
		}
	}
	sort.Strings(names)

	for _, name := range names {
		page := loadPage(t, pagesDir, name)
		// Each fixture is scored on its own, so a nested page is also checked as
		// a page in its own right.
		result := Evaluate(name, page, nil, everyMountKnown(page))
		t.Log(result.String())
		updated[name] = result.Codes()

		want, recorded := baseline[name]
		if !recorded {
			t.Errorf("fixture %q has no baseline - run with -update to record one", name)
			continue
		}
		known := map[string]bool{}
		for _, code := range want {
			known[code] = true
		}
		for _, code := range result.Codes() {
			if !known[code] {
				t.Errorf("%s: new issue kind %q not in the baseline - this page got worse", name, code)
			}
		}
	}

	if os.Getenv("UPDATE_BASELINE") == "1" {
		writeBaseline(t, baselinePath, updated)
		t.Log("baseline.json rewritten")
	}
}

// everyMountKnown treats the pages a fixture points at as existing, because a
// saved page mounts pages that live in the database rather than alongside it.
// The check that matters here is that the mount is SET, not that we can read it.
func everyMountKnown(page map[string]interface{}) map[string]bool {
	known := map[string]bool{}
	for _, mount := range mountsOf(page) {
		if mount != "" {
			known[mount] = true
		}
	}
	return known
}

func loadBaseline(t *testing.T, path string) map[string][]string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string][]string{}
	}
	if err != nil {
		t.Fatalf("reading baseline: %v", err)
	}
	var baseline map[string][]string
	if err := json.Unmarshal(raw, &baseline); err != nil {
		t.Fatalf("parsing baseline: %v", err)
	}
	return baseline
}

func writeBaseline(t *testing.T, path string, baseline map[string][]string) {
	t.Helper()
	raw, err := json.MarshalIndent(baseline, "", "  ")
	if err != nil {
		t.Fatalf("encoding baseline: %v", err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		t.Fatalf("writing baseline: %v", err)
	}
}
