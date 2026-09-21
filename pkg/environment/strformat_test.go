package environment

import (
	"context"
	"errors"
	"strings"
	"testing"

	gjerrors "github.com/jryberg/gojinja/pkg/errors"
)

// Ported from jinja2 tests/test_security.py::TestStringFormat.
func TestStringFormatSandbox(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	cases := []struct{ src, want string }{
		{`{{ "a{0.__class__}b".format(42) }}`, "ab"},
		{`{{ "a{0.foo}b".format({"foo": 42}) }}`, "a42b"},
		{`{{ ("a{0.__class__}b{1}"|safe).format(42, "<foo>") }}`, "ab&lt;foo&gt;"},
		{`{{ ("a{0.foo}b{1}"|safe).format({"foo": 42}, "<foo>") }}`, "a42b&lt;foo&gt;"},
		{`{{ ("a{}b{}").format("foo", "42")}}`, "afoob42"},
		{`{{ ("a{}b{}"|safe).format(42, "<foo>") }}`, "a42b&lt;foo&gt;"},
	}
	for _, c := range cases {
		if got := render(t, e, c.src, nil); got != c.want {
			t.Errorf("%s = %q, want %q", c.src, got, c.want)
		}
	}
}

// Ported from jinja2 tests/test_security.py::TestStringFormatMap.
func TestStringFormatMapSandbox(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	cases := []struct{ src, want string }{
		{`{{ "a{x.__class__}b".format_map({"x":42}) }}`, "ab"},
		{`{{ "a{x.foo}b".format_map({"x":{"foo": 42}}) }}`, "a42b"},
		{`{{ ("a{x.foo}b{y}"|safe).format_map({"x":{"foo": 42}, "y":"<foo>"}) }}`, "a42b&lt;foo&gt;"},
	}
	for _, c := range cases {
		if got := render(t, e, c.src, nil); got != c.want {
			t.Errorf("%s = %q, want %q", c.src, got, c.want)
		}
	}

	tpl, err := e.FromString(`{{ "{0.__call__.__builtins__[__import__]}" | attr("format")(not_here) }}`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tpl.RenderContext(context.Background(), nil)
	var secErr *gjerrors.SecurityError
	if !errors.As(err, &secErr) {
		t.Fatalf("attr(\"format\") escape: want SecurityError, got %v", err)
	}
}

func TestStringFormatUnsafeChainRaises(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	tpl, err := e.FromString(`{{ "{0.__class__.__mro__}".format(42) }}`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tpl.RenderContext(context.Background(), nil)
	var secErr *gjerrors.SecurityError
	if !errors.As(err, &secErr) {
		t.Fatalf("want SecurityError, got %v", err)
	}
}

// Expected values come from rendering the same templates with Python
// Jinja2 3.1.6.
func TestStringFormatSpec(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	cases := []struct{ src, want string }{
		{`{{ "{} {}".format("a", "b") }}`, "a b"},
		{`{{ "{1}{0}{1}".format("a", "b") }}`, "bab"},
		{`{{ "{x}-{y}".format(x=1, y="z") }}`, "1-z"},
		{`{{ "{{{0}}}".format(7) }}`, "{7}"},
		{`{{ "{:>6}|{:<6}|{:^6}|{:*^7}|{:.2s}|{:05}".format("ab", "ab", "ab", "ab", "abc", "ab") }}`,
			"    ab|ab    |  ab  |**ab***|ab|ab000"},
		{`{{ "{:5d}|{:+d}|{: d}|{:,}|{:_}|{:09,}|{:08,}".format(42, 42, 42, 1234567, 1234567, 1234, 1234) }}`,
			"   42|+42| 42|1,234,567|1_234_567|0,001,234|0,001,234"},
		{`{{ "{:x}|{:#x}|{:X}|{:#o}|{:#b}|{:_b}|{:=+6}|{:c}".format(255, 255, 255, 8, 5, 255, 42, 65) }}`,
			"ff|0xff|FF|0o10|0b101|1111_1111|+   42|A"},
		{`{{ "{:.2f}|{:08.3f}|{:e}|{:.2E}|{:g}|{:.3g}|{:%}|{:.1%}|{:,.2f}|{:#.0f}".format(3.14159, 3.14159, 3.14159, 3.14159, 3.14159, 3.14159, 0.25, 0.25, 1234567.891, 3.0) }}`,
			"3.14|0003.142|3.141590e+00|3.14E+00|3.14159|3.14|25.000000%|25.0%|1,234,567.89|3."},
		{`{{ "{}|{:.3}|{:.2}|{:g}|{:g}|{:z.1f}|{:.1f}".format(1.5, 1.0, 10.0, 100000.0, 1000000.0, -0.04, 7) }}`,
			"1.5|1.0|1e+01|100000|1e+06|0.0|7.0"},
		{`{{ "{:>5}|{:d}|{}".format(true, false, none) }}`, "    1|0|None"},
		{`{{ "{!r}|{!s}|{!a}|{!r}".format("it's", 1.0, "é", [1, "a"]) }}`,
			`"it's"|1.0|'\xe9'|[1, 'a']`},
		{`{{ "{0:{1}}|{0:>{1}}".format("x", 4) }}`, "x   |   x"},
		{`{{ "{0[k]}|{1[1]}".format({"k": "v"}, [1, 2]) }}`, "v|2"},
		{`{{ "{k}".format_map({"k": 3}) }}`, "3"},
	}
	for _, c := range cases {
		if got := render(t, e, c.src, nil); got != c.want {
			t.Errorf("%s = %q, want %q", c.src, got, c.want)
		}
	}
}

func TestStringFormatErrors(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	cases := []struct{ src, want string }{
		{`{{ "{}{0}".format(1) }}`, "cannot switch from automatic field numbering to manual field specification"},
		{`{{ "{0}{}".format(1) }}`, "cannot switch from manual field specification to automatic field numbering"},
		{`{{ "a}b".format() }}`, "Single '}' encountered in format string"},
		{`{{ "a{".format() }}`, "Single '{' encountered in format string"},
		{`{{ "{0".format(1) }}`, "expected '}' before end of string"},
		{`{{ "{!x}".format(1) }}`, "Unknown conversion specifier x"},
		{`{{ "{:d}".format("a") }}`, "Unknown format code 'd' for object of type 'str'"},
		{`{{ "{:.2d}".format(1) }}`, "Precision not allowed in integer format specifier"},
		{`{{ "{:>5}".format(none) }}`, "unsupported format string passed to NoneType.__format__"},
		{`{{ "{1}".format(1) }}`, "tuple index out of range"},
		{`{{ "{k}".format_map({}) }}`, "KeyError: 'k'"},
		{`{{ "{}".format_map({}, x=1) }}`, "format_map() takes no keyword arguments"},
	}
	for _, c := range cases {
		tpl, err := e.FromString(c.src)
		if err != nil {
			t.Fatalf("FromString(%q): %v", c.src, err)
		}
		_, err = tpl.RenderContext(context.Background(), nil)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: want error containing %q, got %v", c.src, c.want, err)
		}
	}
}

func TestMarkupFormatEscapesFields(t *testing.T) {
	e := mustEnv(t)
	got := render(t, e, `{{ ("<b>{}</b>{}"|safe).format("<i>", "<u>"|safe) }}|{{ "<b>{}</b>".format("<i>") }}`, nil)
	want := "<b>&lt;i&gt;</b><u>|&lt;b&gt;&lt;i&gt;&lt;/b&gt;"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
