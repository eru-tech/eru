package ruleset

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// The frozen kernel, enforced rather than asserted.
//
// The package doc has said all day that a Kind is a closed set of compiled
// predicates, that configuration selects a predicate and never defines one, and
// that an expression language here would hand authorship of the judging code to
// the thing being judged. That was a sentence in a comment, which is exactly the
// kind of protection this project has spent the day learning not to trust -
// "enforcement is the process boundary, never a sentence in a prompt", and a
// comment is no better than a prompt.
//
// These are the teeth. They will fail if someone adds dynamic evaluation to the
// rule engine, and failing is the point: a rule set arrives from configuration,
// configuration can be written by an agent, and the Darwin Godel Machine's
// Appendix H is what happens when the thing being scored can reach the scorer -
// a variant took a perfect 2.0 by deleting the instrumentation its grader read.

// evaluators are the ways Go turns a string into behaviour. None of them belongs
// in a package whose inputs are authored elsewhere.
var evaluators = []string{
	"text/template", "html/template", // template execution
	"os/exec", "plugin", // running code
	"reflect.Call", "reflect.MakeFunc", // calling by name
	"govaluate/", "antonmedv/expr", "cel-go", "starlark", "otto", "goja", // expression languages
}

// Matched as import paths rather than bare words: "expr" alone also matches the
// word "expression", which this package's own doc uses throughout, and a guard
// that cries wolf gets deleted.

func TestTheRuleEngineCannotEvaluateWhatItIsGiven(t *testing.T) {
	files, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		name := file.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		source, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		text := string(source)
		for _, evaluator := range evaluators {
			if strings.Contains(text, evaluator) {
				t.Errorf("%s references %q. A rule arrives from configuration, and configuration can be written "+
					"by an agent: the moment this package can EVALUATE what it is handed, the thing being judged "+
					"can author its own judgement. If this is deliberate, the package doc's closed-set boundary "+
					"has to be rewritten first, and the reason recorded.", name, evaluator)
			}
		}
	}
}

// Every Kind must be dispatched by Check, and every dispatched Kind must be a
// declared constant. A predicate reachable without a constant is a predicate
// nobody declared; a constant with no predicate is inert and silently passes
// everything it is asked to judge.
func TestEveryKindIsDeclaredAndDispatched(t *testing.T) {
	declared := map[string]bool{}
	source, err := os.ReadFile("ruleset.go")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parser.ParseFile(token.NewFileSet(), "ruleset.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	ast.Inspect(parsed, func(node ast.Node) bool {
		spec, ok := node.(*ast.ValueSpec)
		if !ok {
			return true
		}
		if ident, ok := spec.Type.(*ast.Ident); !ok || ident.Name != "Kind" {
			return true
		}
		for _, name := range spec.Names {
			declared[name.Name] = true
		}
		return true
	})
	if len(declared) < 5 {
		t.Fatalf("expected the Kind constants to be found by name; got %v", declared)
	}

	dispatched := string(source)
	for kind := range declared {
		// KindRequires is the zero value and is dispatched as the default case.
		if kind == "KindRequires" {
			continue
		}
		if !strings.Contains(dispatched, "case "+kind+":") {
			t.Errorf("%s is declared but never dispatched by Check - a rule using it would pass silently, "+
				"which is worse than being rejected", kind)
		}
	}
}

// A Kind nobody recognises must be inert, not treated as the zero value. A typo
// in configuration should judge nothing rather than judge the wrong thing.
func TestAnUnrecognisedKindJudgesNothing(t *testing.T) {
	rule := Rule{Kind: Kind("requires_typo"), Property: "x", Code: "c", Message: "m"}
	if out := rule.Check(subject("tile", map[string]interface{}{})); len(out) != 0 {
		t.Errorf("a misspelled kind must be inert, not silently the default: %+v", out)
	}
}
