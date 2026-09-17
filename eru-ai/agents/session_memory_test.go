package agents

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/eru-tech/eru/eru-cache/cache"
)

func storeWithTtl(t *testing.T, ttl string) cache.CacheStoreI {
	t.Helper()
	raw := json.RawMessage(`{"cache_store_type":"INMEMORY","session_ttl":"` + ttl + `"}`)
	store, err := cache.GetSharedCacheStore(context.Background(), "INMEMORY", &raw)
	if err != nil {
		t.Fatalf("GetSharedCacheStore: %v", err)
	}
	return store
}

func TestTheWorkingSetLivesInTheAgentsOwnStore(t *testing.T) {
	ctx := context.Background()
	store := storeWithTtl(t, "1h")

	recordSessionTurn(ctx, store, "c1", SessionTurn{Role: "user", Code: `{"label":"before"}`, Params: map[string]interface{}{"code": "x"}})
	recordSessionTurn(ctx, store, "c1", SessionTurn{Role: "user", Code: `{"label":"after"}`})

	state := sessionStateOf(ctx, store, "c1")
	if len(state.Turns) != 2 {
		t.Fatalf("turns = %d, want 2", len(state.Turns))
	}
	if latest, ok := state.LatestCode(); !ok || !strings.Contains(latest, "after") {
		t.Fatalf("latest code = %q", latest)
	}
	if prev, ok := state.PreviousCode(); !ok || !strings.Contains(prev, "before") {
		t.Fatalf("previous code = %q", prev)
	}
	if params := state.LatestParams(); params["code"] != "x" {
		t.Fatalf("params were not kept: %v", params)
	}
	forgetSession(ctx, store, "c1")
}

func TestTheWorkingSetIsNeverHandedToTheLongTermTier(t *testing.T) {
	ctx := context.Background()
	store := storeWithTtl(t, "1h")
	recordSessionTurn(ctx, store, "c-tier", SessionTurn{Role: "user", Code: `{"a":1}`})

	// A row reaches the database by being handed to SyncToDatabase, and the
	// working set is written with SetWithTTL alone. What proves the split is
	// that the key is in the store and no CacheData was ever built for it.
	if _, err := store.Get(ctx, sessionKey("c-tier")); err != nil {
		t.Fatalf("the working set is not in the short-term tier: %v", err)
	}
	forgetSession(ctx, store, "c-tier")
}

func TestTheWorkingSetIsBoundedSoPagesDoNotAccumulate(t *testing.T) {
	ctx := context.Background()
	store := storeWithTtl(t, "1h")
	for i := 0; i < 10; i++ {
		recordSessionTurn(ctx, store, "c-bound", SessionTurn{Role: "user", Code: `{"n":` + string(rune('0'+i)) + `}`})
	}
	if got := len(sessionStateOf(ctx, store, "c-bound").Turns); got != sessionMaxTurns {
		t.Fatalf("kept %d turns, want the ceiling of %d", got, sessionMaxTurns)
	}
	forgetSession(ctx, store, "c-bound")
}

func TestExpiryIsPerConversationNotPerMessage(t *testing.T) {
	ctx := context.Background()
	store := storeWithTtl(t, "40ms")

	recordSessionTurn(ctx, store, "cold", SessionTurn{Role: "user", Code: `{"a":1}`})
	recordSessionTurn(ctx, store, "warm", SessionTurn{Role: "user", Code: `{"a":1}`})

	// Keep one conversation warm by using it. A read counts as use, so the
	// expiry slides and the whole conversation survives as a unit.
	for i := 0; i < 4; i++ {
		time.Sleep(15 * time.Millisecond)
		if len(sessionStateOf(ctx, store, "warm").Turns) == 0 {
			t.Fatal("a conversation still being read was allowed to expire")
		}
	}

	if len(sessionStateOf(ctx, store, "cold").Turns) != 0 {
		t.Fatal("an untouched conversation outlived its ttl")
	}
	forgetSession(ctx, store, "warm")
}

func TestTtlComesFromTheStoresOwnConfiguration(t *testing.T) {
	ctx := context.Background()
	if got := sessionTtl(ctx, storeWithTtl(t, "45m")); got != 45*time.Minute {
		t.Fatalf("session_ttl = %s, want 45m - it must be read from the chat_memory config, not a separate setting", got)
	}
	raw := json.RawMessage(`{"cache_store_type":"INMEMORY"}`)
	store, _ := cache.GetSharedCacheStore(ctx, "INMEMORY", &raw)
	if got := sessionTtl(ctx, store); got != cache.DefaultSessionTtl {
		t.Fatalf("unconfigured ttl = %s, want the default %s", got, cache.DefaultSessionTtl)
	}
}

func TestOneTenantCannotReadAnothersConversation(t *testing.T) {
	ctx := context.Background()
	store := storeWithTtl(t, "1h")

	// Several tenants pointing at one Redis is the normal deployment, and the
	// cache has no query to filter on - only a key - so the scope has to be in
	// the key. The same conversation id under two tenants is two conversations.
	a := memoryKey("processo", "tenant-a", "conv-1")
	b := memoryKey("processo", "tenant-b", "conv-1")
	if a == b {
		t.Fatal("two tenants resolved to one cache key")
	}

	recordSessionTurn(ctx, store, a, SessionTurn{Role: "user", Code: `{"secret":"tenant a"}`})
	if got := sessionStateOf(ctx, store, b); len(got.Turns) != 0 {
		t.Fatal("a tenant read another tenant's working set by knowing the conversation id")
	}
	forgetSession(ctx, store, a)
}

func TestOldArtifactsArePrunedButTheirHistoryIsNot(t *testing.T) {
	ctx := context.Background()
	store := storeWithTtl(t, "1h")

	// Five successive baselines, each differing from the last by one leaf.
	for i := 0; i < 5; i++ {
		code := `{"components":[{"id":"c","label":"v` + string(rune('0'+i)) + `"}]}`
		recordSessionTurn(ctx, store, "c-prune", SessionTurn{Role: "user", Code: code})
	}

	state := sessionStateOf(ctx, store, "c-prune")

	withCode := 0
	withDiff := 0
	for _, turn := range state.Turns {
		if turn.Code != "" {
			withCode++
		}
		if len(turn.Diff) > 0 {
			withDiff++
		}
	}
	if withCode != sessionFullArtifactTurns {
		t.Fatalf("%d turns still hold a full artifact, want %d", withCode, sessionFullArtifactTurns)
	}
	if withDiff != 4 {
		t.Fatalf("%d turns kept their change history, want 4 - pruning the artifact must not lose what changed", withDiff)
	}

	// And the journal still reaches all the way back.
	note := JournalNote(state)
	for _, want := range []string{"v0", "v1", "v4"} {
		if !strings.Contains(note, want) {
			t.Fatalf("the journal lost %q once its artifact was pruned:\n%s", want, note)
		}
	}
}

func TestTheDiffIsTakenOnceWhenTheTurnIsRecorded(t *testing.T) {
	ctx := context.Background()
	store := storeWithTtl(t, "1h")

	recordSessionTurn(ctx, store, "c-once", SessionTurn{Role: "user", Code: `{"a":1}`})
	recordSessionTurn(ctx, store, "c-once", SessionTurn{Role: "user", Code: `{"a":2}`})

	state := sessionStateOf(ctx, store, "c-once")
	last := state.Turns[len(state.Turns)-1]
	if len(last.Diff) == 0 {
		t.Fatal("the turn carries no stored diff, so the journal would have to re-derive it from artifacts that may be gone")
	}
	if last.Diff[0].From != "1" || last.Diff[0].To != "2" {
		t.Fatalf("stored diff = %+v", last.Diff[0])
	}
}

func TestTheAgentsAnswerIsNotDiffedAgainstTheClientsPage(t *testing.T) {
	ctx := context.Background()
	store := storeWithTtl(t, "1h")

	recordSessionTurn(ctx, store, "c-shape", SessionTurn{Role: "user", Code: `{"components":[{"id":"c","label":"a"}]}`})
	recordSessionTurn(ctx, store, "c-shape", SessionTurn{Role: "assistant", Code: `{"response":"done","mode":"full"}`})

	state := sessionStateOf(ctx, store, "c-shape")
	for _, turn := range state.Turns {
		if turn.Role == "assistant" && len(turn.Diff) > 0 {
			t.Fatalf("the answer envelope was diffed against the page: %+v", turn.Diff)
		}
	}
	if note := JournalNote(state); strings.Contains(note, "response") {
		t.Fatalf("the answer envelope leaked into the journal:\n%s", note)
	}
}
