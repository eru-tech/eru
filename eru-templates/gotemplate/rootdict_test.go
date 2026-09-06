package gotemplate

import (
	"context"
	"os"
	"testing"

	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

func TestMain(m *testing.M) {
	logs.LogInit("test", "test-instance")
	os.Exit(m.Run())
}

func rootDictFor(t *testing.T, template string) *TemplateDict {
	t.Helper()
	goTmpl := GoTemplate{Name: "test", Template: template}
	root, err := goTmpl.RootDict(context.Background())
	if err != nil {
		t.Fatalf("unexpected parse error for %q : %v", template, err)
	}
	return root
}

func TestRootDictNestedParams(t *testing.T) {
	root := rootDictFor(t, `{{stringify (dict "content" .Vars.Body.content "params" (dict "context" (stringify .ResVars.step1.Body) "code" .Vars.Body.params.code))}}`)
	if root == nil {
		t.Fatal("expected a root dict")
	}
	if !root.HasKey("content") || !root.HasKey("params") {
		t.Fatalf("unexpected root keys : %v", root.Keys)
	}
	params := root.Child("params")
	if params == nil {
		t.Fatal("expected a params child dict")
	}
	if !params.HasKey("context") || !params.HasKey("code") {
		t.Fatalf("unexpected params keys : %v", params.Keys)
	}
	if root.Dynamic || params.Dynamic {
		t.Error("expected fully literal keys")
	}
}

func TestRootDictPipedStringify(t *testing.T) {
	root := rootDictFor(t, `{{dict "params" (dict "query" "select 1") | stringify}}`)
	if root == nil || !root.HasKey("params") {
		t.Fatalf("expected params root dict, got %+v", root)
	}
	if child := root.Child("params"); child == nil || !child.HasKey("query") {
		t.Fatalf("expected query in params, got %+v", child)
	}
}

func TestRootDictNoDict(t *testing.T) {
	if root := rootDictFor(t, `{{.Vars.Body.content}}`); root != nil {
		t.Fatalf("expected no dict, got %+v", root)
	}
}

func TestRootDictPassedByReference(t *testing.T) {
	root := rootDictFor(t, `{{stringify (dict "params" .Vars.Body.params)}}`)
	if root == nil || !root.HasKey("params") {
		t.Fatalf("expected params root dict, got %+v", root)
	}
	if root.Child("params") != nil {
		t.Error("expected no child dict when params is passed by reference")
	}
}

func TestRootDictDynamicKey(t *testing.T) {
	root := rootDictFor(t, `{{stringify (dict .Vars.Body.key "x" "content" "y")}}`)
	if root == nil || !root.Dynamic {
		t.Fatalf("expected dynamic dict, got %+v", root)
	}
}

func TestRootDictParseError(t *testing.T) {
	goTmpl := GoTemplate{Name: "test", Template: `{{stringify (dict "content" .Vars.Body.content))}}`}
	if _, err := goTmpl.RootDict(context.Background()); err == nil {
		t.Fatal("expected a parse error")
	}
}
