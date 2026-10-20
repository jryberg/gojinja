package environment

import (
	"context"
	"strings"
	"testing"

	"github.com/jryberg/gojinja/pkg/loader"
)

// TestRecursionDepth checks that recursion as deep as Jinja2 allows
// renders, while runaway recursion fails like Python's RecursionError
// instead of overflowing the Go stack.
func TestRecursionDepth(t *testing.T) {
	e := mustEnv(t, WithLoader(loader.Dict{
		"selfinc": `{% if n > 0 %}{% set n = n - 1 %}{% include "selfinc" %}{% endif %}.`,
		"base":    "<p>{% block h %}x{% endblock %}</p>",
		"child":   "{% extends 'base' %}{% block h %}{{ self.h() }}{% endblock %}",
		"imp":     "{% import 'imp' as i %}",
	}))
	macro := "{% macro m(n) %}{% if n %}{{ m(n-1) }}{% endif %}{% endmacro %}{{ m(N) }}"
	loop := "{% for i in [N] recursive %}{% if i %}{{ loop([i-1]) }}{% endif %}{% endfor %}"
	caller := "{% macro w() %}{{ caller() }}{% endmacro %}{% macro m(n) %}{% if n %}{% call w() %}{{ m(n-1) }}{% endcall %}{% endif %}{% endmacro %}{{ m(N) }}"
	include := "{% set n = N %}{% include 'selfinc' %}"
	cases := []struct {
		src  string
		n    int
		fail bool
	}{
		// The deepest each form gets in Python before RecursionError.
		{include, 992, false},
		{macro, 198, false},
		{loop, 248, false},
		{caller, 66, false},
		{include, 100000, true},
		{macro, 100000, true},
		{loop, 100000, true},
		{caller, 100000, true},
		{"{% include 'child' %}", 0, true},
		{"{% import 'imp' as i %}", 0, true},
		{"{% block b %}{{ self.b() }}{% endblock %}", 0, true},
	}
	for _, c := range cases {
		tpl, err := e.FromString(c.src)
		if err != nil {
			t.Fatal(err)
		}
		_, err = tpl.RenderContext(context.Background(), map[string]any{"N": c.n})
		switch {
		case c.fail && (err == nil || !strings.Contains(err.Error(), "maximum recursion depth exceeded")):
			t.Errorf("%s (N=%d): got %v, want a recursion error", c.src, c.n, err)
		case !c.fail && err != nil:
			t.Errorf("%s (N=%d): %v", c.src, c.n, err)
		}
	}
}
