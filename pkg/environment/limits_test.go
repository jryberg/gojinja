package environment

import (
	"context"
	"math"
	"strings"
	"testing"
	"time"
)

// TestLexLinear guards against quadratic lexing: a tag-body rule that
// fails to match must not scan the rest of the source. It compares two
// input sizes instead of bounding one, so a slow runner or -race can't
// fail it: four times the tags take about 4x as long when lexing is
// linear and about 16x when it is quadratic.
func TestLexLinear(t *testing.T) {
	e := mustEnv(t)
	parse := func(n int) time.Duration {
		src := strings.Repeat("{{ x }}{% if 1 %}a{% endif %}", n)
		best := time.Duration(math.MaxInt64)
		for range 3 {
			start := time.Now()
			if _, err := e.FromString(src); err != nil {
				t.Fatal(err)
			}
			best = min(best, time.Since(start))
		}
		return best
	}
	small, large := parse(2500), parse(10000)
	if ratio := float64(large) / float64(small); ratio > 8 {
		t.Fatalf("4x the tags took %.1fx as long (%v vs %v)", ratio, large, small)
	}
}

// TestNestingDepth checks that every nesting depth Jinja2 accepts parses,
// while absurdly deep input fails like Python's RecursionError instead of
// overflowing the Go stack.
func TestNestingDepth(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	forms := map[string]func(int) string{
		"neg":    func(n int) string { return "{{ " + strings.Repeat("-", n) + "1 }}" },
		"not":    func(n int) string { return "{{ " + strings.Repeat("not ", n) + "1 }}" },
		"plus":   func(n int) string { return "{{ 1" + strings.Repeat(" + 1", n) + " }}" },
		"and":    func(n int) string { return "{{ 1" + strings.Repeat(" and 1", n) + " }}" },
		"cond":   func(n int) string { return "{{ 1" + strings.Repeat(" if 1 else 1", n) + " }}" },
		"filter": func(n int) string { return "{{ 1" + strings.Repeat("|abs", n) + " }}" },
		"attr":   func(n int) string { return "{{ x" + strings.Repeat(".a", n) + " }}" },
		"if":     func(n int) string { return strings.Repeat("{% if 1 %}", n) + strings.Repeat("{% endif %}", n) },
		"with":   func(n int) string { return strings.Repeat("{% with %}", n) + strings.Repeat("{% endwith %}", n) },
		"macro":  func(n int) string { return strings.Repeat("{% macro m() %}", n) + strings.Repeat("{% endmacro %}", n) },
	}
	// The deepest each form gets in Python.
	pyMax := map[string]int{"neg": 491, "not": 491, "plus": 491, "and": 491, "cond": 491, "filter": 328, "attr": 197, "if": 98, "with": 247, "macro": 98}
	for name, mk := range forms {
		tpl, err := e.FromString(mk(pyMax[name]))
		if err == nil {
			_, err = tpl.RenderContext(context.Background(), map[string]any{"x": nil})
		}
		if err != nil && !strings.Contains(err.Error(), "has no attribute") {
			t.Errorf("%s at depth %d: %v", name, pyMax[name], err)
		}
		if _, err := e.FromString(mk(50000)); err == nil || !strings.Contains(err.Error(), "maximum recursion depth exceeded") {
			t.Errorf("%s at depth 50000: got %v, want a recursion error", name, err)
		}
	}
	// Flat chains do not nest, in Python or here.
	for _, src := range []string{
		"{{ 'a'" + strings.Repeat(" ~ 'a'", 100000) + " }}",
		"{{ 1" + strings.Repeat(" < 2", 100000) + " }}",
		"{{ [" + strings.Repeat("1, ", 100000) + "]|length }}",
	} {
		if _, err := e.FromString(src); err != nil {
			t.Errorf("%.20s...: %v", src, err)
		}
	}
}

// TestRecursiveValues checks Python's output for containers that contain
// themselves, and that values nested too deeply fail instead of
// overflowing the Go stack.
func TestRecursiveValues(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	cases := []struct{ src, want, err string }{
		{`{% set d = {'a': 1} %}{% do d.update({'self': d}) %}{{ d }}`, "{'a': 1, 'self': {...}}", ""},
		{`{% set d = {'a': 1} %}{% do d.update({'l': [d]}) %}{{ d }}`, "{'a': 1, 'l': [{...}]}", ""},
		{`{% set d = {'a': 1} %}{% do d.update({'self': d}) %}{{ d == d }}|{{ d in [d] }}`, "True|True", ""},
		{`{% set d = {'a': 1} %}{% do d.update({'self': d}) %}{{ d|tojson }}`, "", "Circular reference detected"},
		{`{% set ns = namespace(a=1) %}{% set ns.self = ns %}{{ ns }}`, "<Namespace {'a': 1, 'self': <Namespace {...}>}>", ""},
		{`{{ namespace(a=[1, 'x'], b={'k': none}) }}`, "<Namespace {'a': [1, 'x'], 'b': {'k': None}}>", ""},
	}
	deep := `{% set ns = namespace(l=[]) %}{% for i in range(300) %}{% for j in range(100) %}{% set ns.l = [ns.l] %}{% endfor %}{% endfor %}`
	for _, out := range []string{"{{ ns.l }}", "{{ ns.l|tojson }}", "{{ ns.l|pprint }}", "{{ '%s' % (ns.l,) }}", "{{ [ns.l] < [[ns.l]] }}"} {
		cases = append(cases, struct{ src, want, err string }{deep + out, "", "maximum recursion depth exceeded"})
	}
	for _, c := range cases {
		tpl, err := e.FromString(c.src)
		if err != nil {
			t.Fatal(err)
		}
		out, err := tpl.RenderContext(context.Background(), nil)
		switch {
		case c.err != "" && (err == nil || !strings.Contains(err.Error(), c.err)):
			t.Errorf("%s: got %q, %v; want error %q", c.src, out, err, c.err)
		case c.err == "" && (err != nil || out != c.want):
			t.Errorf("%s: got %q, %v; want %q", c.src, out, err, c.want)
		}
	}
}
