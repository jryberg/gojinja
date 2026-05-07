package environment

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/jryberg/gojinja/pkg/ext"
)

func mustEnv(t *testing.T, opts ...Option) *Environment {
	t.Helper()
	e, err := New(opts...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return e
}

func render(t *testing.T, e *Environment, src string, vars map[string]any) string {
	t.Helper()
	tpl, err := e.FromString(src)
	if err != nil {
		t.Fatalf("FromString(%q): %v", src, err)
	}
	out, err := tpl.RenderContext(context.Background(), vars)
	if err != nil {
		t.Fatalf("Render(%q): %v", src, err)
	}
	return out
}

// =============================================================== smoke

func TestRenderPlainText(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	if got := render(t, e, "Hello, world!", nil); got != "Hello, world!" {
		t.Fatalf("got %q", got)
	}
}

func TestRenderVariable(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	if got := render(t, e, "Hello, {{ name }}!", map[string]any{"name": "Tom"}); got != "Hello, Tom!" {
		t.Fatalf("got %q", got)
	}
}

func TestAutoescapeOnByDefault(t *testing.T) {
	e := mustEnv(t)
	got := render(t, e, "{{ x }}", map[string]any{"x": "<script>"})
	if got != "&lt;script&gt;" {
		t.Fatalf("autoescape: got %q", got)
	}
}

func TestAutoescapeOptOut(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	got := render(t, e, "{{ x }}", map[string]any{"x": "<b>"})
	if got != "<b>" {
		t.Fatalf("opt-out: got %q", got)
	}
}

// =============================================================== if/for

func TestIfElse(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	tpl := "{% if x %}A{% else %}B{% endif %}"
	if got := render(t, e, tpl, map[string]any{"x": true}); got != "A" {
		t.Fatalf("true branch: %q", got)
	}
	if got := render(t, e, tpl, map[string]any{"x": false}); got != "B" {
		t.Fatalf("false branch: %q", got)
	}
}

func TestForLoop(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	got := render(t, e, "{% for x in xs %}{{ x }};{% endfor %}", map[string]any{"xs": []any{1, 2, 3}})
	if got != "1;2;3;" {
		t.Fatalf("got %q", got)
	}
}

func TestForLoopVariable(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	got := render(t, e, "{% for x in xs %}{{ loop.index }}={{ x }}{% if not loop.last %},{% endif %}{% endfor %}", map[string]any{"xs": []any{"a", "b", "c"}})
	if got != "1=a,2=b,3=c" {
		t.Fatalf("got %q", got)
	}
}

func TestForElseEmpty(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	got := render(t, e, "{% for x in xs %}A{% else %}empty{% endfor %}", map[string]any{"xs": []any{}})
	if got != "empty" {
		t.Fatalf("got %q", got)
	}
}

// =============================================================== filters

func TestFilterUpper(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	if got := render(t, e, "{{ 'abc' | upper }}", nil); got != "ABC" {
		t.Fatalf("got %q", got)
	}
}

func TestFilterChain(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	if got := render(t, e, "{{ '  hi  ' | trim | upper }}", nil); got != "HI" {
		t.Fatalf("got %q", got)
	}
}

func TestFilterDefault(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	if got := render(t, e, "{{ x | default('fallback') }}", map[string]any{}); got != "fallback" {
		t.Fatalf("got %q", got)
	}
	if got := render(t, e, "{{ x | default('fallback') }}", map[string]any{"x": "set"}); got != "set" {
		t.Fatalf("got %q", got)
	}
}

func TestFilterEscapeMakesStringSafe(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	if got := render(t, e, "{{ '<x>' | escape }}", nil); got != "&lt;x&gt;" {
		t.Fatalf("got %q", got)
	}
}

func TestSafeBypassesAutoescape(t *testing.T) {
	e := mustEnv(t)
	if got := render(t, e, "{{ x | safe }}", map[string]any{"x": "<b>safe</b>"}); got != "<b>safe</b>" {
		t.Fatalf("got %q", got)
	}
}

// =============================================================== tests

func TestIsDefined(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	if got := render(t, e, "{% if x is defined %}Y{% else %}N{% endif %}", map[string]any{"x": 1}); got != "Y" {
		t.Fatalf("got %q", got)
	}
	if got := render(t, e, "{% if x is defined %}Y{% else %}N{% endif %}", nil); got != "N" {
		t.Fatalf("got %q", got)
	}
}

func TestIsEven(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	if got := render(t, e, "{% if 4 is even %}Y{% endif %}", nil); got != "Y" {
		t.Fatalf("got %q", got)
	}
}

// =============================================================== set/with

func TestSet(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	got := render(t, e, "{% set x = 'hello' %}{{ x }}", nil)
	if got != "hello" {
		t.Fatalf("got %q", got)
	}
}

func TestWith(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	got := render(t, e, "{% with a = 1, b = 2 %}{{ a + b }}{% endwith %}", nil)
	if got != "3" {
		t.Fatalf("got %q", got)
	}
}

// =============================================================== comparisons

func TestComparisonOperators(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	cases := []struct {
		expr string
		want string
	}{
		{"{{ 1 < 2 }}", "True"},
		{"{{ 2 == 2 }}", "True"},
		{"{{ 'a' in ['a','b'] }}", "True"},
		{"{{ 'z' not in ['a','b'] }}", "True"},
	}
	for _, c := range cases {
		got := render(t, e, c.expr, nil)
		if got != c.want {
			t.Errorf("%s = %q, want %q", c.expr, got, c.want)
		}
	}
}

// =============================================================== ranges

func TestRangeBounded(t *testing.T) {
	e := mustEnv(t, WithRangeLimit(10), WithAutoescape(AutoescapeNever{}))
	got := render(t, e, "{% for i in range(3) %}{{ i }}{% endfor %}", nil)
	if got != "012" {
		t.Fatalf("got %q", got)
	}
	tpl, err := e.FromString("{% for i in range(1000) %}.{% endfor %}")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tpl.RenderContext(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "exceeds limit") {
		t.Fatalf("expected range-limit error, got %v", err)
	}
}

func TestCacheSizeRejectsUnbounded(t *testing.T) {
	// `WithCacheSize(-1)` (or 0) must clamp to a positive bound — never
	// "unlimited". An attacker who can name templates would otherwise
	// fill memory with parsed ASTs.
	e, err := New(WithCacheSize(-1))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if e.maxCache <= 0 {
		t.Fatalf("WithCacheSize(-1) produced maxCache=%d, want > 0", e.maxCache)
	}
	e2, err := New(WithCacheSize(0))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if e2.maxCache <= 0 {
		t.Fatalf("WithCacheSize(0) produced maxCache=%d, want > 0", e2.maxCache)
	}
}

// =============================================================== sandbox / safety

func TestUnderscoreAttributesBlocked(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	type s struct{ Field, _hidden string }
	tpl, err := e.FromString("{{ x._hidden }}")
	if err != nil {
		t.Fatal(err)
	}
	_, err = tpl.RenderContext(context.Background(), map[string]any{"x": s{Field: "ok", _hidden: "secret"}})
	if err == nil {
		t.Fatal("expected SecurityError on underscore attribute")
	}
	if !strings.Contains(err.Error(), "forbidden") {
		t.Fatalf("expected security message, got %v", err)
	}
}

// =============================================================== custom loader for include

type dictLoader map[string]string

func (d dictLoader) GetSource(name string) (Source, error) {
	src, ok := d[name]
	if !ok {
		return Source{}, &notFound{name: name}
	}
	return Source{Code: src, Filename: name}, nil
}

type notFound struct{ name string }

func (n *notFound) Error() string { return "not found: " + n.name }

func TestInclude(t *testing.T) {
	e := mustEnv(t,
		WithAutoescape(AutoescapeNever{}),
		WithLoader(dictLoader{"hello.html": "Hello, {{ name }}!"}),
	)
	got := render(t, e, "{% include 'hello.html' %}", map[string]any{"name": "T"})
	if got != "Hello, T!" {
		t.Fatalf("got %q", got)
	}
}

// =============================================================== more filters

func TestFiltersExpanded(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	cases := []struct {
		src  string
		vars map[string]any
		want string
	}{
		{"{{ [1,2,3] | sum }}", nil, "6"},
		{"{{ [3,1,2] | min }}", nil, "1"},
		{"{{ [3,1,2] | max }}", nil, "3"},
		{`{{ "Hello, World" | wordcount }}`, nil, "2"},
		{"{{ 1234567 | filesizeformat }}", nil, "1.2 MB"},
		{"{{ 1024 | filesizeformat(true) }}", nil, "1.0 KiB"},
		{`{{ [1,1,2,2,3] | unique | join(',') }}`, nil, "1,2,3"},
		{`{{ "<b>hi</b>" | striptags }}`, nil, "hi"},
		{"{{ 3.14159 | round(2) }}", nil, "3.14"},
		{`{{ "hello world" | urlencode }}`, nil, "hello%20world"},
	}
	for _, c := range cases {
		got := render(t, e, c.src, c.vars)
		if got != c.want {
			t.Errorf("%s = %q, want %q", c.src, got, c.want)
		}
	}
}

func TestDictsortFilter(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	got := render(t, e,
		`{% for k, v in d | dictsort %}{{ k }}={{ v }};{% endfor %}`,
		map[string]any{"d": map[string]any{"b": 2, "a": 1, "c": 3}})
	if got != "a=1;b=2;c=3;" {
		t.Fatalf("got %q", got)
	}
}

func TestItemsFilter(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	got := render(t, e,
		`{% for k, v in d | items %}{{ k }}:{{ v }};{% endfor %}`,
		map[string]any{"d": map[string]any{"x": 1, "y": 2}})
	if got != "x:1;y:2;" {
		t.Fatalf("got %q", got)
	}
}

// =============================================================== inheritance

func TestExtendsAndBlockOverride(t *testing.T) {
	e := mustEnv(t,
		WithAutoescape(AutoescapeNever{}),
		WithLoader(dictLoader{
			"base.html": "before [{% block content %}default{% endblock %}] after",
		}),
	)
	tpl, err := e.FromString(`{% extends 'base.html' %}{% block content %}OVERRIDE{% endblock %}`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := tpl.RenderContext(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != "before [OVERRIDE] after" {
		t.Fatalf("got %q", got)
	}
}

func TestExtendsBlockNotOverridden(t *testing.T) {
	e := mustEnv(t,
		WithAutoescape(AutoescapeNever{}),
		WithLoader(dictLoader{
			"base.html": "before [{% block content %}default{% endblock %}] after",
		}),
	)
	tpl, err := e.FromString(`{% extends 'base.html' %}`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := tpl.RenderContext(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != "before [default] after" {
		t.Fatalf("got %q", got)
	}
}

// =============================================================== more filters

func TestGroupbyFilter(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	got := render(t, e,
		`{% for grouper, items in xs | groupby('kind') %}{{ grouper }}={{ items | length }};{% endfor %}`,
		map[string]any{
			"xs": []any{
				map[string]any{"kind": "a", "n": 1},
				map[string]any{"kind": "b", "n": 2},
				map[string]any{"kind": "a", "n": 3},
			},
		})
	if got != "a=2;b=1;" {
		t.Fatalf("got %q", got)
	}
}

func TestWordwrapFilter(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	got := render(t, e, "{{ s | wordwrap(10) }}",
		map[string]any{"s": "the quick brown fox jumps"})
	want := "the quick\nbrown fox\njumps"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestUrlizeFilter(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	got := render(t, e,
		`{{ s | urlize }}`,
		map[string]any{"s": "visit https://example.com today"})
	if !strings.Contains(got, `<a href="https://example.com"`) {
		t.Fatalf("got %q", got)
	}
	if !strings.Contains(got, `rel="noopener"`) {
		t.Fatalf("missing rel=noopener: %q", got)
	}
}

func TestXmlattrFilter(t *testing.T) {
	e := mustEnv(t)
	got := render(t, e, "<a{{ {'href': '/x', 'class': 'btn'} | xmlattr }}>",
		nil)
	// Sorted: class first, then href.
	want := `<a class="btn" href="/x">`
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestXmlattrRejectsBadKeys(t *testing.T) {
	e := mustEnv(t)
	tpl, err := e.FromString(`{{ {'bad key': 1} | xmlattr }}`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tpl.RenderContext(context.Background(), nil); err == nil {
		t.Fatal("expected error for invalid attr name")
	}
}

// =============================================================== globals

func TestCyclerGlobal(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	got := render(t, e,
		`{% set c = cycler('odd','even') %}{% for i in range(4) %}{{ c.next() }};{% endfor %}`, nil)
	if got != "odd;even;odd;even;" {
		t.Fatalf("got %q", got)
	}
}

func TestJoinerGlobal(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	got := render(t, e,
		`{% set j = joiner('|') %}{% for x in xs %}{{ j() }}{{ x }}{% endfor %}`,
		map[string]any{"xs": []any{"a", "b", "c"}})
	if got != "a|b|c" {
		t.Fatalf("got %q", got)
	}
}

func TestNamespaceGlobal(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	got := render(t, e,
		`{% set ns = namespace() %}{% for i in range(3) %}{% set ns.last = i %}{% endfor %}{{ ns.last }}`, nil)
	if got != "2" {
		t.Fatalf("got %q", got)
	}
}

// =============================================================== break / continue / do

func TestBreak(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	got := render(t, e,
		`{% for i in range(10) %}{% if i == 3 %}{% break %}{% endif %}{{ i }}{% endfor %}`, nil)
	if got != "012" {
		t.Fatalf("got %q", got)
	}
}

func TestContinue(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	got := render(t, e,
		`{% for i in range(5) %}{% if i is even %}{% continue %}{% endif %}{{ i }};{% endfor %}`, nil)
	if got != "1;3;" {
		t.Fatalf("got %q", got)
	}
}

func TestDoStatement(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	// `do` evaluates and discards its expression. Useful for accumulating
	// state in a namespace inside a loop.
	got := render(t, e,
		`{% set ns = namespace() %}`+
			`{% set ns.acc = 0 %}`+
			`{% for i in range(4) %}{% do ns %}{% set ns.acc = ns.acc + i %}{% endfor %}`+
			`{{ ns.acc }}`, nil)
	if got != "6" {
		t.Fatalf("got %q", got)
	}
}

// =============================================================== call block / caller()

func TestCallBlockWithCaller(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	src := `{% macro wrap() %}<<{{ caller() }}>>{% endmacro %}` +
		`{% call wrap() %}inner{% endcall %}`
	got := render(t, e, src, nil)
	if got != "<<inner>>" {
		t.Fatalf("got %q", got)
	}
}

// =============================================================== i18n extension

// fakeTranslator returns deterministic translations for a tiny corpus —
// just enough to verify the extension wires the right gettext family.
type fakeTranslator struct{}

func (fakeTranslator) Gettext(msg string) string {
	switch msg {
	case "Hello %(name)s":
		return "Hej %(name)s"
	case "missing":
		return "fehlend"
	}
	return msg
}

func (fakeTranslator) NGettext(singular, plural string, n int) string {
	if n == 1 {
		switch singular {
		case "%(count)s item":
			return "%(count)s Eintrag"
		}
		return singular
	}
	switch plural {
	case "%(count)s items":
		return "%(count)s Einträge"
	}
	return plural
}

func (fakeTranslator) PGettext(ctxStr, msg string) string {
	if ctxStr == "title" && msg == "missing" {
		return "Fehlt"
	}
	return msg
}

func (fakeTranslator) NPGettext(_ /*ctxStr*/, singular, plural string, n int) string {
	if n == 1 {
		return singular
	}
	return plural
}

func TestI18NTransSimple(t *testing.T) {
	e := mustEnv(t,
		WithAutoescape(AutoescapeNever{}),
		WithI18NExtension(fakeTranslator{}),
	)
	got := render(t, e, `{% trans %}missing{% endtrans %}`, nil)
	if got != "fehlend" {
		t.Fatalf("trans render = %q, want %q", got, "fehlend")
	}
}

func TestI18NTransWithVar(t *testing.T) {
	e := mustEnv(t,
		WithAutoescape(AutoescapeNever{}),
		WithI18NExtension(fakeTranslator{}),
	)
	src := `{% trans %}Hello {{ name }}{% endtrans %}`
	got := render(t, e, src, map[string]any{"name": "Alice"})
	if got != "Hej Alice" {
		t.Fatalf("trans var render = %q, want %q", got, "Hej Alice")
	}
}

func TestI18NTransExplicitVarBinding(t *testing.T) {
	e := mustEnv(t,
		WithAutoescape(AutoescapeNever{}),
		WithI18NExtension(fakeTranslator{}),
	)
	src := `{% trans name='Bob' %}Hello {{ name }}{% endtrans %}`
	got := render(t, e, src, nil)
	if got != "Hej Bob" {
		t.Fatalf("trans explicit var render = %q, want %q", got, "Hej Bob")
	}
}

func TestI18NTransPluralize(t *testing.T) {
	e := mustEnv(t,
		WithAutoescape(AutoescapeNever{}),
		WithI18NExtension(fakeTranslator{}),
	)
	src := `{% trans count=count %}{{ count }} item{% pluralize %}{{ count }} items{% endtrans %}`
	got := render(t, e, src, map[string]any{"count": int64(1)})
	if got != "1 Eintrag" {
		t.Fatalf("singular pluralize render = %q, want %q", got, "1 Eintrag")
	}
	got = render(t, e, src, map[string]any{"count": int64(5)})
	if got != "5 Einträge" {
		t.Fatalf("plural pluralize render = %q, want %q", got, "5 Einträge")
	}
}

func TestI18NTransContext(t *testing.T) {
	e := mustEnv(t,
		WithAutoescape(AutoescapeNever{}),
		WithI18NExtension(fakeTranslator{}),
	)
	src := `{% trans 'title' %}missing{% endtrans %}`
	got := render(t, e, src, nil)
	if got != "Fehlt" {
		t.Fatalf("pgettext render = %q, want %q", got, "Fehlt")
	}
}

func TestI18NUnderscoreAlias(t *testing.T) {
	e := mustEnv(t,
		WithAutoescape(AutoescapeNever{}),
		WithI18NExtension(fakeTranslator{}),
	)
	got := render(t, e, `{{ _("missing") }}`, nil)
	if got != "fehlend" {
		t.Fatalf("_ alias render = %q, want %q", got, "fehlend")
	}
}

func TestI18NTrimmed(t *testing.T) {
	e := mustEnv(t,
		WithAutoescape(AutoescapeNever{}),
		WithI18NExtension(ext.NullTranslator{}),
	)
	src := "{% trans trimmed %}\n  Hello\n  world\n{% endtrans %}"
	got := render(t, e, src, nil)
	if got != "Hello world" {
		t.Fatalf("trimmed render = %q, want %q", got, "Hello world")
	}
}

func TestI18NSubstHelper(t *testing.T) {
	got := ext.I18NSubst("Hello %(name)s, %(count)s items", map[string]any{
		"name":  "Alice",
		"count": 7,
	})
	if got != "Hello Alice, 7 items" {
		t.Fatalf("subst = %q, want %q", got, "Hello Alice, 7 items")
	}
	// Avoid unused fmt import if needed (sanity check int formatting).
	if fmt.Sprintf("%v", 7) != "7" {
		t.Fatalf("fmt.Sprintf sanity")
	}
}

// =============================================================== debug extension

func TestDebugExtensionDumpsContextFiltersTests(t *testing.T) {
	e := mustEnv(t,
		WithAutoescape(AutoescapeNever{}),
		WithDebugExtension(),
	)
	out := render(t, e, "Hello\n{% debug %}\nGoodbye", map[string]any{"who": "world"})
	for _, want := range []string{
		"Hello", "Goodbye",
		"'context'", "'filters'", "'tests'",
		"'who'", `"world"`,
		"'abs'", // a sample filter
		"'in'",  // a sample test
	} {
		if !strings.Contains(out, want) {
			t.Errorf("debug output missing %q\nfull output: %s", want, out)
		}
	}
}

// =============================================================== macro varargs / kwargs

func TestMacroVarargs(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	src := `{% macro test() %}{{ varargs|join('|') }}{% endmacro %}` +
		`{{ test(1, 2, 3) }}`
	got := render(t, e, src, nil)
	if got != "1|2|3" {
		t.Fatalf("varargs render = %q, want %q", got, "1|2|3")
	}
}

func TestMacroKwargsCatcher(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	// Sort keys deterministically by accessing one specific entry. The
	// `kwargs` value is a map; render via attribute lookup.
	src := `{% macro test() %}{{ kwargs['foo'] }}-{{ kwargs['bar'] }}{% endmacro %}` +
		`{{ test(foo='hello', bar='world') }}`
	got := render(t, e, src, nil)
	if got != "hello-world" {
		t.Fatalf("kwargs catcher render = %q, want %q", got, "hello-world")
	}
}

func TestMacroExtraPositionalErrorsWhenNoVarargs(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	src := `{% macro test(a) %}{{ a }}{% endmacro %}{{ test(1, 2) }}`
	tpl, err := e.FromString(src)
	if err != nil {
		t.Fatalf("FromString: %v", err)
	}
	if _, err := tpl.RenderContext(context.Background(), nil); err == nil {
		t.Fatalf("expected error for extra positional, got nil")
	}
}

func TestMacroUnknownKwargErrors(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	src := `{% macro test(a) %}{{ a }}{% endmacro %}{{ test(1, b=2) }}`
	tpl, err := e.FromString(src)
	if err != nil {
		t.Fatalf("FromString: %v", err)
	}
	if _, err := tpl.RenderContext(context.Background(), nil); err == nil {
		t.Fatalf("expected error for unknown kwarg, got nil")
	}
}

func TestMacroDeclaredVarargsTakesPrecedence(t *testing.T) {
	// When `varargs` is a declared parameter, it does NOT act as the
	// catcher — it's just a regular arg.
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	src := `{% macro test(varargs) %}{{ varargs }}{% endmacro %}{{ test('plain') }}`
	got := render(t, e, src, nil)
	if got != "plain" {
		t.Fatalf("declared varargs got %q, want %q", got, "plain")
	}
}

// =============================================================== super()

func TestSuperInTemplate(t *testing.T) {
	e := mustEnv(t,
		WithAutoescape(AutoescapeNever{}),
		WithLoader(dictLoader{
			"base.html": "[{% block c %}base-content{% endblock %}]",
		}),
	)
	tpl, err := e.FromString(`{% extends 'base.html' %}{% block c %}{{ super() }}+child{% endblock %}`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := tpl.RenderContext(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != "[base-content+child]" {
		t.Fatalf("got %q", got)
	}
}

// =============================================================== context cancellation

func TestRenderRespectsContext(t *testing.T) {
	e := mustEnv(t, WithRangeLimit(1_000_000), WithAutoescape(AutoescapeNever{}))
	tpl, err := e.FromString("{% for i in range(1000) %}.{% endfor %}")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := tpl.RenderContext(ctx, nil); err == nil {
		t.Fatal("expected cancellation error")
	}
}

// =============================================================== parity divergences caught against real-world templates

// TestInUndefinedReturnsFalse mirrors Python Jinja2: iterating Undefined
// (default mode) produces an empty sequence, so "x in undef" is False.
// Exposed by mkdocs-material's content.html: `{% if "x" in features %}`
// against an empty render context.
func TestInUndefinedReturnsFalse(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	got := render(t, e, `{% if "x" in features %}A{% else %}B{% endif %}`, map[string]any{})
	if got != "B" {
		t.Fatalf(`'in' on undefined: got %q, want "B"`, got)
	}
}

// TestUnknownFilterAtCompileTime asserts unknown filters cause a
// TemplateAssertionError when the template is loaded — mirrors
// Python's `from_string` behaviour.
func TestUnknownFilterAtCompileTime(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	if _, err := e.FromString(`{{ x | nopesuchfilter }}`); err == nil {
		t.Fatal("expected error for unknown filter")
	}
}

// TestUnknownFilterDeferredInsideIf mirrors Jinja2 3.0+: filters and
// tests inside an If or CondExpr branch are checked at runtime instead
// of compile time. Exposed by nbconvert's celltags.j2 and mkdocs-material
// templates that wrap unknown filters in `{% if cond %}` guards. The
// macro never gets called, so the unknown filter must NOT trigger a
// compile-time TemplateAssertionError.
func TestUnknownFilterDeferredInsideIf(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	src := `{% macro m(c) %}{% if c %}{{ c | nopesuchfilter }}{% endif %}{% endmacro %}`
	if _, err := e.FromString(src); err != nil {
		t.Fatalf("filter inside If body should be deferred to runtime: %v", err)
	}
}

// TestExtendsKeepsChildBodyExecuting mirrors Python: a child template's
// top-level statements before {% extends %} run normally; statements
// after extends still execute for side-effects (set/macro/import) but
// their inline output is suppressed.
func TestExtendsKeepsChildBodyExecuting(t *testing.T) {
	e := mustEnv(t,
		WithAutoescape(AutoescapeNever{}),
		WithLoader(dictLoader{
			"base.html": "[{% block c %}default{% endblock %}]",
		}),
	)
	cases := []struct {
		name, src, want string
		wantErr         bool
	}{
		{"output_before_extends_emits",
			"pre {% extends 'base.html' %}", "pre [default]", false},
		{"output_after_extends_skipped",
			"{% extends 'base.html' %}{{ undef_func() }}",
			"[default]", false},
		{"set_before_extends_runs",
			"{% set x = 1 %}{% extends 'base.html' %}", "[default]", false},
		{"set_after_extends_runs",
			"{% extends 'base.html' %}{% set x = undef_func() %}", "", true},
		{"block_before_extends_renders_inline",
			"{% block c %}A{% endblock %}{% extends 'base.html' %}", "A[A]", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tpl, err := e.FromString(tc.src)
			if err != nil {
				t.Fatal(err)
			}
			got, err := tpl.RenderContext(context.Background(), nil)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestImportDiscardsBodyOutput regression-tests that {% import %} runs
// the imported body for side-effects only — stray text or trailing
// newlines in the imported template do NOT leak into the caller's
// output. Python isolates the import to a separate buffer.
func TestImportDiscardsBodyOutput(t *testing.T) {
	e := mustEnv(t,
		WithAutoescape(AutoescapeNever{}),
		WithLoader(dictLoader{
			"helpers.html": "{% macro greet(n) %}hi {{ n }}{% endmacro %}\nLEAK\n",
		}),
	)
	tpl, err := e.FromString(`<{% import "helpers.html" as h %}{{ h.greet("x") }}>`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := tpl.RenderContext(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != "<hi x>" {
		t.Fatalf("import leaked output: got %q", got)
	}
}

// TestFromImportMissingNameDeferred mirrors Python: a name not exported
// by the imported template binds to an Undefined that errors only when
// used, not at the import statement itself.
func TestFromImportMissingNameDeferred(t *testing.T) {
	e := mustEnv(t,
		WithAutoescape(AutoescapeNever{}),
		WithLoader(dictLoader{"empty.html": ""}),
	)
	// Importing a missing name should NOT raise.
	tpl, err := e.FromString(`{% from "empty.html" import nope %}done`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := tpl.RenderContext(context.Background(), nil)
	if err != nil {
		t.Fatalf("from-import of missing name should defer: %v", err)
	}
	if got != "done" {
		t.Fatalf("got %q", got)
	}
}

// TestLengthOnUndefinedReturnsZero mirrors Python: Undefined.__len__ is 0
// for ModeBase / ModeChainable, so `undef | length` is 0 (not an error).
func TestLengthOnUndefinedReturnsZero(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	got := render(t, e, `{{ x | length }}`, map[string]any{})
	if got != "0" {
		t.Fatalf("got %q, want \"0\"", got)
	}
}

// TestDictGlobalKwargOrderIsDeterministic regression-tests the flake
// caused by Go's randomized map iteration: dict(a=1, b=2) | items would
// occasionally produce the kwargs in reversed order. We sort kwargs
// alphabetically inside dict() so the items output is stable.
func TestDictGlobalKwargOrderIsDeterministic(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	for i := 0; i < 20; i++ {
		got := render(t, e, `{{ dict(a=1, b=2)|items|list }}`, nil)
		if got != "[('a', 1), ('b', 2)]" {
			t.Fatalf("iter %d: got %q", i, got)
		}
	}
}

// TestMacroBlockSetCompiles regression-tests the typed-nil *Filter walk
// crash in pkg/ast: a macro containing a block-form `{% set y -%}…{%-
// endset %}` (AssignBlock) used to panic during the macro-special-name
// detection walk because AssignBlock.Filter is a concrete-pointer field
// whose nil typed value evades the generic Node interface check.
func TestMacroBlockSetCompiles(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	src := `{% macro m() %}{% set y -%}hi{%- endset %}{% endmacro %}`
	tpl, err := e.FromString(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, err := tpl.RenderContext(context.Background(), nil); err != nil {
		t.Fatalf("render: %v", err)
	}
}

// TestHostEnvDisabledByDefault confirms the `env` global is not
// registered unless WithHostEnv() is supplied.
func TestHostEnvDisabledByDefault(t *testing.T) {
	t.Setenv("GOJINJA_TEST_VAR", "shouldNotBeRead")
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	tpl, err := e.FromString("{{ env('GOJINJA_TEST_VAR') }}")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, err := tpl.RenderContext(context.Background(), nil); err == nil {
		t.Fatal("expected error calling env() without WithHostEnv()")
	}
}

// TestHostEnvLookup confirms WithHostEnv() registers `env` and that it
// returns the value of a set variable.
func TestHostEnvLookup(t *testing.T) {
	t.Setenv("GOJINJA_TEST_VAR", "hello")
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}), WithHostEnv())
	if got := render(t, e, "{{ env('GOJINJA_TEST_VAR') }}", nil); got != "hello" {
		t.Fatalf("got %q, want %q", got, "hello")
	}
}

// TestHostEnvMissingReturnsEmpty mirrors os.Getenv: an unset variable
// renders as the empty string. Combined with the `default` filter in
// boolean mode, callers get a fallback for "unset or empty".
func TestHostEnvMissingReturnsEmpty(t *testing.T) {
	os.Unsetenv("GOJINJA_DEFINITELY_MISSING")
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}), WithHostEnv())
	if got := render(t, e, "[{{ env('GOJINJA_DEFINITELY_MISSING') }}]", nil); got != "[]" {
		t.Fatalf("got %q, want %q", got, "[]")
	}
	got := render(t, e, "[{{ env('GOJINJA_DEFINITELY_MISSING')|default('fallback', true) }}]", nil)
	if got != "[fallback]" {
		t.Fatalf("got %q, want %q", got, "[fallback]")
	}
}
