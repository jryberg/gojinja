package environment

import (
	"context"
	"errors"
	"testing"

	gjerrors "github.com/jryberg/gojinja/pkg/errors"
	"github.com/jryberg/gojinja/pkg/runtime"
)

// errUndefined marks a case where Python raises UndefinedError and
// errAny one where it raises any other exception.
const (
	errUndefined = "!UndefinedError"
	errAny       = "!error"
)

// renderOrErr renders src and classifies a failure as errUndefined or
// errAny so cases can be compared against Python's outcome.
func renderOrErr(t *testing.T, e *Environment, src string) string {
	t.Helper()
	tpl, err := e.FromString(src)
	if err != nil {
		t.Fatalf("FromString(%q): %v", src, err)
	}
	out, err := tpl.RenderContext(context.Background(), nil)
	if err != nil {
		if _, ok := errors.AsType[*gjerrors.UndefinedError](err); ok {
			return errUndefined
		}
		return errAny
	}
	return out
}

// Expected values come from rendering the same templates with the Jinja2
// pinned in tools/parity/requirements.txt, once per
// Undefined class.
func TestUndefinedSemantics(t *testing.T) {
	modes := []runtime.UndefinedFactory{runtime.NewBase, runtime.NewChainable, runtime.NewDebug, runtime.NewStrict}
	names := []string{"Undefined", "ChainableUndefined", "DebugUndefined", "StrictUndefined"}
	cases := []struct {
		src  string
		want [4]string
	}{
		{`{{ -q }}`, [4]string{errUndefined, errUndefined, errUndefined, errUndefined}},
		{`{{ +q }}`, [4]string{errUndefined, errUndefined, errUndefined, errUndefined}},
		{`{{ q % 1 }}`, [4]string{errUndefined, errUndefined, errUndefined, errUndefined}},
		{`{{ q + 1 }}`, [4]string{errUndefined, errUndefined, errUndefined, errUndefined}},
		{`{{ 1 + q }}`, [4]string{errUndefined, errUndefined, errUndefined, errUndefined}},
		{`{{ q * 2 }}`, [4]string{errUndefined, errUndefined, errUndefined, errUndefined}},
		{`{{ q < 1 }}`, [4]string{errUndefined, errUndefined, errUndefined, errUndefined}},
		{`{{ q ~ 'a' }}`, [4]string{"a", "a", "{{ q }}a", errUndefined}},
		{`{{ q, }}`, [4]string{"(Undefined,)", "(Undefined,)", "(Undefined,)", "(Undefined,)"}},
		{`{{ [q] }}`, [4]string{"[Undefined]", "[Undefined]", "[Undefined]", "[Undefined]"}},
		{`{{ {'a': q} }}`, [4]string{"{'a': Undefined}", "{'a': Undefined}", "{'a': Undefined}", "{'a': Undefined}"}},
		{`{{ q == z }}`, [4]string{"True", "True", "True", errUndefined}},
		{`{{ q != z }}`, [4]string{"False", "False", "False", errUndefined}},
		{`{{ q == 1 }}`, [4]string{"False", "False", "False", errUndefined}},
		{`{{ q in [1] }}`, [4]string{"False", "False", "False", errUndefined}},
		{`{{ 1 in q }}`, [4]string{"False", "False", "False", errUndefined}},
		{`{{ q }}`, [4]string{"", "", "{{ q }}", errUndefined}},
		{`{% if q %}a{% else %}b{% endif %}`, [4]string{"b", "b", "b", errUndefined}},
		{`{{ not q }}`, [4]string{"True", "True", "True", errUndefined}},
		{`{{ 'a' if q else 'b' }}`, [4]string{"b", "b", "b", errUndefined}},
		{`{{ q or 'x' }}`, [4]string{"x", "x", "x", errUndefined}},
		{`{{ q|string }}`, [4]string{"", "", "{{ q }}", errUndefined}},
		{`{{ [q]|join }}`, [4]string{"", "", "{{ q }}", errUndefined}},
		{`{{ q|upper }}`, [4]string{"", "", "{{ Q }}", errUndefined}},
		{`{{ q|default('d') }}`, [4]string{"d", "d", "d", "d"}},
		{`{{ q|join(',') }}`, [4]string{"", "", "", errUndefined}},
		{`{{ q|sum }}`, [4]string{"0", "0", "0", errUndefined}},
		{`{{ q|int }}`, [4]string{errUndefined, errUndefined, errUndefined, errUndefined}},
		{`{{ q|float }}`, [4]string{errUndefined, errUndefined, errUndefined, errUndefined}},
		{`{{ q|round }}`, [4]string{errAny, errAny, errAny, errAny}},
		{`{{ q|round(0, 'ceil') }}`, [4]string{errUndefined, errUndefined, errUndefined, errUndefined}},
		{`{{ q is defined }}`, [4]string{"False", "False", "False", "False"}},
		{`{{ q is eq z }}`, [4]string{"True", "True", "True", errUndefined}},
		{`{{ q is sequence }}`, [4]string{"True", "True", "True", "False"}},
		{`{{ q is iterable }}`, [4]string{"True", "True", "True", errUndefined}},
		{`{{ q is callable }}`, [4]string{"True", "True", "True", "True"}},
		{`{{ q is escaped }}`, [4]string{"False", "True", "False", "False"}},
		{`{{ q is divisibleby 2 }}`, [4]string{errUndefined, errUndefined, errUndefined, errUndefined}},
		{`{{ q is even }}`, [4]string{errUndefined, errUndefined, errUndefined, errUndefined}},
		{`{{ q is lt 1 }}`, [4]string{errUndefined, errUndefined, errUndefined, errUndefined}},
		{`{{ q is ge 1 }}`, [4]string{errUndefined, errUndefined, errUndefined, errUndefined}},
		{`{{ q is lower }}`, [4]string{"False", "False", "True", errUndefined}},
		{`{{ q is filter }}`, [4]string{"False", "False", "False", errUndefined}},
		{`{{ q is in [1] }}`, [4]string{"False", "False", "False", errUndefined}},
		{`{{ q is number }}`, [4]string{"False", "False", "False", "False"}},
	}
	for i, f := range modes {
		e := mustEnv(t, WithAutoescape(AutoescapeNever{}), WithUndefined(f))
		for _, c := range cases {
			if got := renderOrErr(t, e, c.src); got != c.want[i] {
				t.Errorf("%s: %s = %q, want %q", names[i], c.src, got, c.want[i])
			}
		}
	}
}

// Port of TestFilter.test_int / test_float / test_round_positive /
// test_round_negative plus Python's untouched-default and half-even
// rounding semantics.
func TestNumericFilters(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	cases := []struct{ src, want string }{
		{`{{ 'x'|float(0) }}|{{ 'x'|float('a') }}|{{ 'x'|float }}|{{ '3.5'|float }}`, "0|a|0.0|3.5"},
		{`{{ 'x'|int(1.5) }}|{{ 'x'|int('a') }}|{{ 'inf'|int }}|{{ 'nan'|int(5) }}|{{ '32.32'|int }}`, "1.5|a|0|5|32"},
		{`{{ 0|round }}|{{ 3|round }}|{{ true|round }}|{{ 1250|round(-2) }}|{{ 5|round(-1) }}|{{ 15|round(-1) }}|{{ -15|round(-1) }}`,
			"0|3|1|1200|0|20|-20"},
		{`{{ 2.5|round }}|{{ 3.5|round }}|{{ 0.5|round }}|{{ -0.5|round }}|{{ -2.5|round }}|{{ 2.675|round(2) }}|{{ 0.125|round(2) }}|{{ 0.375|round(2) }}`,
			"2.0|4.0|0.0|-0.0|-2.0|2.67|0.12|0.38"},
		{`{{ 1250.0|round(-2) }}|{{ 12345.678|round(-2) }}|{{ 1e308|round(-308) }}|{{ 42.55|round }}|{{ 1.0000000000000002|round(15) }}`,
			"1200.0|12300.0|1e+308|43.0|1.0"},
		{`{{ 3|round(0, 'ceil') }}|{{ 3.14159|round(2, 'ceil') }}|{{ 42.55|round(1, 'floor') }}|{{ 1234|round(-2, 'ceil') }}|{{ 2.7|round(1, 'floor') }}`,
			"3.0|3.15|42.5|1300.0|2.7"},
	}
	for _, c := range cases {
		if got := render(t, e, c.src, nil); got != c.want {
			t.Errorf("%s = %q, want %q", c.src, got, c.want)
		}
	}
	if got := renderOrErr(t, e, `{{ 1.5|round(0, 'bogus') }}`); got != errAny {
		t.Errorf("round with unknown method = %q, want an error", got)
	}
}
