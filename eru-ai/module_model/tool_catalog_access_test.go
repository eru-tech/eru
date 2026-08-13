package module_model

import (
	"context"
	"testing"
)

func boolPtr(b bool) *bool {
	return &b
}

func TestIsToolVisibleNoOverride(t *testing.T) {
	ps := ProjectSettings{}
	if !ps.IsToolVisible("Eruql", true, "t1") {
		t.Error("expected code level public tool to stay visible")
	}
	if ps.IsToolVisible("Eruql", false, "t1") {
		t.Error("expected code level private tool to stay hidden")
	}
}

func TestIsToolVisibleAllowedTenant(t *testing.T) {
	ps := ProjectSettings{ToolCatalogAccess: map[string]ToolCatalogAccess{
		"Eruql": {Public: boolPtr(false), AllowedTenants: []string{"t1"}},
	}}
	if !ps.IsToolVisible("Eruql", true, "t1") {
		t.Error("expected allowed tenant to see private tool")
	}
	if ps.IsToolVisible("Eruql", true, "t2") {
		t.Error("expected override to hide tool from other tenants")
	}
	if ps.IsToolVisible("Eruql", true, "") {
		t.Error("expected private tool to stay hidden when tenant is unknown")
	}
}

func TestIsToolVisiblePublicOverride(t *testing.T) {
	ps := ProjectSettings{ToolCatalogAccess: map[string]ToolCatalogAccess{
		"Eruql": {Public: boolPtr(true)},
	}}
	if !ps.IsToolVisible("Eruql", false, "t2") {
		t.Error("expected public override to expose tool to every tenant")
	}
}

func TestSetToolCatalogAccessDeltas(t *testing.T) {
	ctx := context.Background()
	ps := ProjectSettings{}
	if err := ps.SetToolCatalogAccess(ctx, ToolCatalogAccessRequest{ToolType: "Eruql", Public: boolPtr(false), AddTenants: []string{"t1", "t2", "t1"}}); err != nil {
		t.Fatal(err)
	}
	if got := ps.ToolCatalogAccess["Eruql"].AllowedTenants; len(got) != 2 || got[0] != "t1" || got[1] != "t2" {
		t.Fatalf("unexpected allowed tenants %v", got)
	}

	if err := ps.SetToolCatalogAccess(ctx, ToolCatalogAccessRequest{ToolType: "Eruql", AddTenants: []string{"t3"}, RemoveTenants: []string{"t1"}}); err != nil {
		t.Fatal(err)
	}
	access := ps.ToolCatalogAccess["Eruql"]
	if got := access.AllowedTenants; len(got) != 2 || got[0] != "t2" || got[1] != "t3" {
		t.Fatalf("unexpected allowed tenants after delta %v", got)
	}
	if access.Public == nil || *access.Public {
		t.Error("expected public override to be retained when not sent")
	}

	if err := ps.SetToolCatalogAccess(ctx, ToolCatalogAccessRequest{}); err == nil {
		t.Error("expected error for blank tool_type")
	}
}
