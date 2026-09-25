package module_model

import (
	"context"
	"encoding/json"
	"testing"
)

// A project saved before keys carried metadata stored a plain map of name to name. It still has to
// load, or every existing project loses its keys on upgrade.
func TestKidsDecodesLegacyShape(t *testing.T) {
	var kids Kids
	if err := json.Unmarshal([]byte(`{"ERUAUTH_KID_smartvalues":"ERUAUTH_KID_smartvalues"}`), &kids); err != nil {
		t.Fatalf("legacy kids must still decode: %v", err)
	}
	kid, ok := kids["ERUAUTH_KID_smartvalues"]
	if !ok {
		t.Fatal("legacy kid missing after decode")
	}
	if kid.Status != KidStatusActive {
		t.Errorf("a legacy kid must read as active, got %q", kid.Status)
	}
	if !kid.CreatedAt.IsZero() {
		t.Error("a legacy kid has no recorded creation date and must not invent one")
	}
}

func TestKidsDecodesCurrentShape(t *testing.T) {
	var kids Kids
	body := `{"ERUAUTH_KID_a":{"kid":"ERUAUTH_KID_a","status":"RETIRED","created_at":"2026-01-02T03:04:05Z"}}`
	if err := json.Unmarshal([]byte(body), &kids); err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	kid := kids["ERUAUTH_KID_a"]
	if !kid.Retired() || kid.CreatedAt.IsZero() {
		t.Errorf("unexpected kid %+v", kid)
	}
}

func TestKidsRoundTrip(t *testing.T) {
	prj := &Project{}
	if err := prj.AddKid(context.Background(), "ERUAUTH_KID_a"); err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if prj.Kids["ERUAUTH_KID_a"].CreatedAt.IsZero() || prj.Kids["ERUAUTH_KID_a"].Retired() {
		t.Error("a new kid must be active and carry a creation date")
	}

	body, err := json.Marshal(prj.Kids)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	var decoded Kids
	if err = json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if decoded["ERUAUTH_KID_a"].Status != KidStatusActive {
		t.Error("status lost in round trip")
	}
}

func TestSetKidStatus(t *testing.T) {
	prj := &Project{}
	_ = prj.AddKid(context.Background(), "ERUAUTH_KID_a")

	if err := prj.SetKidStatus(context.Background(), "ERUAUTH_KID_a", "retired"); err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if !prj.Kids["ERUAUTH_KID_a"].Retired() {
		t.Error("expected the kid to be retired")
	}
	if err := prj.SetKidStatus(context.Background(), "ERUAUTH_KID_missing", KidStatusRetired); err == nil {
		t.Error("an unknown kid must be reported rather than created")
	}
}
