package gojinja_test

import (
	"context"
	"strings"
	"testing"

	gj "github.com/jryberg/gojinja"
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

