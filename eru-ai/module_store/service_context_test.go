package module_store

import (
	"context"
	"os"
	"testing"

	tools "github.com/eru-tech/eru/eru-ai/tools"
	function_module_store "github.com/eru-tech/eru/eru-functions/module_store"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

func TestMain(m *testing.M) {
	logs.LogInit("test", "service-context-test")
	os.Exit(m.Run())
}

// every downstream eru call resolves its target from the context, and eru-functions reads a
// different key type than the eru-ai tools do - both have to be seeded or one path silently
// talks to localhost.
func TestWithEruServiceContextSeedsBothKeyFamilies(t *testing.T) {
	Eruqlbaseurl = "http://eruql:8087"
	Eruaibaseurl = "http://eruai:8088"
	Erufuncbaseurl = "http://erufunc:8083"
	Erufilesbaseurl = "http://erufiles:8082"

	ctx := WithEruServiceContext(context.Background())

	if got, _ := ctx.Value("eruqlbaseurl").(string); got != Eruqlbaseurl {
		t.Errorf("tools eruqlbaseurl = %q, want %q", got, Eruqlbaseurl)
	}
	if got, _ := ctx.Value(tools.EruFuncBaseUrlKey).(string); got != Erufuncbaseurl {
		t.Errorf("tools erufuncbaseurl = %q, want %q", got, Erufuncbaseurl)
	}
	if got, _ := ctx.Value(function_module_store.ContextKeyEruqlbaseurl).(string); got != Eruqlbaseurl {
		t.Errorf("eru-functions eruqlbaseurl = %q, want %q", got, Eruqlbaseurl)
	}
	if got, _ := ctx.Value(function_module_store.ContextKeyEruaibaseurl).(string); got != Eruaibaseurl {
		t.Errorf("eru-functions eruaibaseurl = %q, want %q", got, Eruaibaseurl)
	}
}

// the claims key is the project's, and a project that configures none keeps the header the
// claims arrived on.
func TestWithProjectContextClaimsKey(t *testing.T) {
	ctx := WithProjectContext(context.Background(), "", nil)
	if got := tools.ClaimsKeyFromContext(ctx); got != tools.ClaimsHeaderKey {
		t.Errorf("claims key without a project = %q, want %q", got, tools.ClaimsHeaderKey)
	}
	if got, _ := ctx.Value(function_module_store.ContextKeyEruqlbaseurl).(string); got == "" {
		t.Error("WithProjectContext did not seed the service base urls")
	}
}
