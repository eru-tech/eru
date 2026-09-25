package utiltiy

import "testing"

// A read used to verify a write must not be served from cache.
//
// The metadata query is cached for 500 seconds, which is right for the lookups
// an agent makes while working and exactly wrong for a read-back taken seconds
// after a save. confirmWrites spent a day telling the agent it had failed to
// write fields it had written.
func TestAFreshReadIsOptIn(t *testing.T) {
	if freshRequired(nil) {
		t.Error("the default must stay cached - the working lookups are the common case")
	}
	if freshRequired(map[string]interface{}{"entity_names": []string{"test"}}) {
		t.Error("an ordinary lookup must stay cached")
	}
	if !freshRequired(map[string]interface{}{FreshReadParam: true}) {
		t.Error("a caller verifying its own write must be able to ask for a fresh read")
	}
	if freshRequired(map[string]interface{}{FreshReadParam: "true"}) {
		t.Error("only a real bool counts - a model echoing a string must not switch caching off")
	}
}

// The parameter is underscore-prefixed and absent from the tool's declared
// schema, so the model cannot set it. A model that could turn caching off would
// turn it off always.
func TestTheFreshReadParamIsNotModelFacing(t *testing.T) {
	if FreshReadParam[0] != '_' {
		t.Errorf("FreshReadParam = %q; an internal parameter should not look like a model-facing one", FreshReadParam)
	}
	// It is also absent from the parameter schema the tool declares, which is
	// what actually keeps it out of the model's reach - the underscore is only
	// the convention that makes that visible to a reader.
}
