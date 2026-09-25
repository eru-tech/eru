package auth

import (
	"context"
	"encoding/hex"
	"testing"

	erusha "github.com/eru-tech/eru/eru-crypto/sha"
)

func TestHashClientSecretIsSha512Hex(t *testing.T) {
	want := hex.EncodeToString(erusha.NewSHA512([]byte("s3cret")))
	if got := hashClientSecret("s3cret"); got != want {
		t.Errorf("unexpected hash %s", got)
	}
}

func TestClientFromRowReadsJsonColumns(t *testing.T) {
	row := map[string]interface{}{
		"client_id":                  "cid",
		"client_name":                "Claude",
		"token_endpoint_auth_method": "none",
		"scope":                      "openid offline_access",
		"redirect_uris":              `["https://claude.ai/api/mcp/auth_callback"]`,
		"grant_types":                []byte(`["authorization_code","refresh_token"]`),
		"response_types":             []interface{}{"code"},
		"client_secret_hash":         "should-not-escape",
	}

	client, err := clientFromRow(context.Background(), row)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if len(client.RedirectURIs) != 1 || client.RedirectURIs[0] != "https://claude.ai/api/mcp/auth_callback" {
		t.Errorf("redirect_uris as a string column not read: %v", client.RedirectURIs)
	}
	if len(client.GrantTypes) != 2 {
		t.Errorf("grant_types as bytes not read: %v", client.GrantTypes)
	}
	if len(client.ResponseTypes) != 1 || client.ResponseTypes[0] != "code" {
		t.Errorf("response_types as a decoded list not read: %v", client.ResponseTypes)
	}
	if client.ClientSecret != "" {
		t.Error("the stored secret hash must never be carried out of the store")
	}
}

func TestRowStringListHandlesEmptyAndBadJson(t *testing.T) {
	if got := rowStringList(context.Background(), ""); got != nil {
		t.Errorf("empty column must read as nil, got %v", got)
	}
	if got := rowStringList(context.Background(), "not json"); got != nil {
		t.Errorf("unreadable column must read as nil rather than panic, got %v", got)
	}
}

func TestClientRegistryBackendSelection(t *testing.T) {
	hydraAuth := &Auth{Hydra: HydraConfig{PublicScheme: "https", PublicHost: "hydra.example"}}
	registry, err := hydraAuth.ClientRegistry(context.Background(), "smartvalues")
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if _, ok := registry.(HydraClientRegistry); !ok {
		t.Errorf("hydra must stay the default backend, got %T", registry)
	}

	eruAuth := &Auth{AuthName: "eru-oauth", OAuthServerConfig: OAuthServerConfig{Backend: OAuthBackendEru}}
	registry, err = eruAuth.ClientRegistry(context.Background(), "smartvalues")
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	eruRegistry, ok := registry.(EruClientRegistry)
	if !ok {
		t.Fatalf("expected the eru registry, got %T", registry)
	}
	if eruRegistry.ProjectId != "smartvalues" || eruRegistry.AuthName != "eru-oauth" {
		t.Errorf("registry not scoped to project and auth: %+v", eruRegistry)
	}
}

// Without a connection every operation must fail rather than silently behave as an empty store.
func TestEruClientRegistryFailsWithoutConnection(t *testing.T) {
	registry := EruClientRegistry{ProjectId: "smartvalues", AuthName: "eru-oauth"}
	if _, err := registry.CreateClient(context.Background(), OAuthClient{}); err == nil {
		t.Error("expected create to fail with no connection")
	}
	if _, err := registry.GetClient(context.Background(), "cid"); err == nil {
		t.Error("expected get to fail with no connection")
	}
	if err := registry.DeleteClient(context.Background(), "cid"); err == nil {
		t.Error("expected delete to fail with no connection")
	}
	if err := registry.VerifyClientSecret(context.Background(), "cid", "s"); err == nil {
		t.Error("expected secret verification to fail with no connection")
	}
}
