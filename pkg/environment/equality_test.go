package environment

import (
	"context"
	"strings"
	"testing"
)

// Expected values come from rendering the same templates with the Jinja2
// pinned in tools/parity/requirements.txt.

// Port of TestTestsCase.test_sameas / test_no_paren_for_arg1 plus
// identity checks on values that are uncomparable in Go.
func TestSameAs(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	cases := []struct {
		src  string
		vars map[string]any
		want string
	}{
		{`{{ foo is sameas false }}|{{ 0 is sameas false }}`, map[string]any{"foo": false}, "True|False"},
		{`{{ foo is sameas none }}`, map[string]any{"foo": nil}, "True"},
		{`{{ q is sameas q }}|{{ [1] is sameas [1] }}|{% set l = [1] %}{{ l is sameas l }}|{% set d = {} %}{{ d is sameas d }}|{{ {} is sameas {} }}|{{ none is sameas none }}`,
			nil, "False|False|True|True|False|True"},
	}
	for _, c := range cases {
		if got := render(t, e, c.src, c.vars); got != c.want {
			t.Errorf("%s = %q, want %q", c.src, got, c.want)
		}
	}
}

// Port of TestTestsCase.test_equalto and test_in, plus Python `==`
// semantics across numeric types, sequences and mappings.
func TestEquality(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	cases := []struct {
		src  string
		vars map[string]any
		want string
	}{
		{`{{ foo is eq 12 }}|{{ foo is eq 0 }}|{{ foo is eq (3 * 4) }}|{{ bar is eq "baz" }}|{{ bar is eq "zab" }}|{{ bar is eq ("ba" + "z") }}|{{ bar is eq bar }}|{{ bar is eq foo }}`,
			map[string]any{"foo": 12, "bar": "baz"}, "True|False|True|True|False|True|True|False"},
		{`{{ "o" is in "foo" }}|{{ "foo" is in "foo" }}|{{ "b" is in "foo" }}|{{ 1 is in ((1, 2)) }}|{{ 3 is in ((1, 2)) }}|{{ 1 is in [1, 2] }}|{{ 3 is in [1, 2] }}|{{ "foo" is in {"foo": 1}}}|{{ "baz" is in {"bar": 1}}}`,
			nil, "True|True|False|True|False|True|False|True|False"},
		{`{{ true == 1 }}|{{ 1 is eq true }}|{{ (1, 2) == (1, 2.0) }}|{{ [1] == (1,) }}|{{ [1] == [1.0] }}|{{ {'a': 1} == {'a': 1.0} }}|{{ {'a': [1]} != {'a': [2]} }}|{{ [1] is in [[1]] }}|{{ [1] in [[1.0]] }}|{{ 'a' is in {'a': 1} }}|{{ none == 0 }}|{{ 1 is ne 1.0 }}`,
			nil, "True|True|True|False|True|True|True|True|True|True|False|False"},
		{`{{ [3, 1, 2]|sort(reverse=true) }}|{{ 2 is gt 1 }}|{{ 1 is gt 1 }}|{{ 1 is le 1 }}|{{ 2 is le 1 }}`,
			nil, "[3, 2, 1]|True|False|True|False"},
	}
	for _, c := range cases {
		if got := render(t, e, c.src, c.vars); got != c.want {
			t.Errorf("%s = %q, want %q", c.src, got, c.want)
		}
	}
}

// Deeply nested data must fail like CPython's RecursionError, not
// overflow the Go stack.
func TestEqualityDepthBound(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	nest := func() any {
		var v any = []any{}
		for i := 0; i < 1100; i++ {
			v = []any{v}
		}
		return v
	}
	tpl, err := e.FromString(`{{ a == b }}`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tpl.RenderContext(context.Background(), map[string]any{"a": nest(), "b": nest()})
	if err == nil || !strings.Contains(err.Error(), "maximum recursion depth exceeded in comparison") {
		t.Fatalf("want recursion error, got %v", err)
	}
}
