package gojinja_test

import (
	"context"
	"os"
	"strings"
	"testing"

	gj "github.com/jryberg/gojinja"
	"github.com/jryberg/gojinja/pkg/lexer"
)

// TestPublicFacade ensures the top-level package re-exports stay
// usable: a caller can build an Environment with a loader + render a
// template purely through the gojinja namespace.
func TestPublicFacade(t *testing.T) {
	env, err := gj.New(
		gj.WithAutoescape(gj.AutoescapeNever{}),
		gj.WithLoader(gj.DictLoader{
			"hello.j2": "Hi, {{ name | upper }}!",
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	tpl, err := env.GetTemplate("hello.j2")
	if err != nil {
		t.Fatal(err)
	}
	out, err := tpl.RenderContext(context.Background(), map[string]any{"name": "world"})
	if err != nil {
		t.Fatal(err)
	}
	if out != "Hi, WORLD!" {
		t.Fatalf("got %q", out)
	}
}

// TestRenderStability_CTSGeneric exercises the production code path
// that historically leaked Go's randomised map iteration into rendered
// output: JSONVars (now ordered) → Template.RenderContext. The
// CTS-generic template iterates `template['containers']` directly, so
// before the OrderedDict fix two consecutive renders produced different
// orderings in the JANITOR_CONTAINERS list. Re-render N times and
// assert byte-equality across all runs.
func TestRenderStability_CTSGeneric(t *testing.T) {
	const (
		tmplPath = "tools/parity/corpus/CTS-generic.j2"
		varsPath = "tools/parity/corpus/CTS-generic.vars.json"
		runs     = 10
	)
	src, err := os.ReadFile(tmplPath)
	if err != nil {
		t.Fatalf("read template: %v", err)
	}
	raw, err := os.ReadFile(varsPath)
	if err != nil {
		t.Fatalf("read vars: %v", err)
	}
	vars, err := gj.JSONVars(raw)
	if err != nil {
		t.Fatalf("JSONVars: %v", err)
	}
	lexOpts := lexer.DefaultOptions()
	lexOpts.KeepTrailingNewline = true
	env, err := gj.New(
		gj.WithAutoescape(gj.AutoescapeNever{}),
		gj.WithLexerOptions(lexOpts),
	)
	if err != nil {
		t.Fatal(err)
	}
	tpl, err := env.FromString(string(src))
	if err != nil {
		t.Fatal(err)
	}
	first, err := tpl.RenderContext(context.Background(), vars)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	for i := 1; i < runs; i++ {
		out, err := tpl.RenderContext(context.Background(), vars)
		if err != nil {
			t.Fatalf("render rerun %d: %v", i, err)
		}
		if out != first {
			t.Fatalf("non-deterministic render at rerun %d", i)
		}
	}
}

// TestRenderAcceptsOrderedDict locks in the public-API contract that an
// OrderedDict can be passed straight to RenderContext (no conversion
// dance) and that iteration through the template sees insertion order.
func TestRenderAcceptsOrderedDict(t *testing.T) {
	od := gj.NewOrderedDict()
	od.Set("zeta", 1)
	od.Set("alpha", 2)
	od.Set("mu", 3)
	env, _ := gj.New(gj.WithAutoescape(gj.AutoescapeNever{}))
	tpl, err := env.FromString(`{% for k in d %}{{ k }} {% endfor %}`)
	if err != nil {
		t.Fatal(err)
	}
	out, err := tpl.RenderContext(context.Background(), map[string]any{"d": od})
	if err != nil {
		t.Fatal(err)
	}
	const want = "zeta alpha mu "
	if out != want {
		t.Fatalf("got %q, want %q (insertion order, not lex)", out, want)
	}
}

// TestRenderRawMapDeterministic asserts that a plain map[string]any
// (where Go can't preserve insertion order) iterates in lex-sorted
// order — the documented determinism fallback.
func TestRenderRawMapDeterministic(t *testing.T) {
	env, _ := gj.New(gj.WithAutoescape(gj.AutoescapeNever{}))
	tpl, err := env.FromString(`{% for k in d %}{{ k }} {% endfor %}`)
	if err != nil {
		t.Fatal(err)
	}
	vars := map[string]any{"d": map[string]any{"zeta": 1, "alpha": 2, "mu": 3}}
	first, err := tpl.RenderContext(context.Background(), vars)
	if err != nil {
		t.Fatal(err)
	}
	const want = "alpha mu zeta "
	if first != want {
		t.Fatalf("got %q, want %q (lex sort)", first, want)
	}
	for i := 0; i < 5; i++ {
		out, _ := tpl.RenderContext(context.Background(), vars)
		if out != first {
			t.Fatalf("non-deterministic rerun %d: %q vs %q", i, out, first)
		}
	}
}

func TestAutoescapeFacade(t *testing.T) {
	env, _ := gj.New() // defaults — autoescape on
	tpl, err := env.FromString(`{{ x }}`)
	if err != nil {
		t.Fatal(err)
	}
	out, err := tpl.Render(map[string]any{"x": "<script>"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "&lt;") {
		t.Fatalf("autoescape not applied: %q", out)
	}
}
