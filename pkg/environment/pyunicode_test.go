package environment

import (
	"context"
	"strings"
	"testing"

	"github.com/jryberg/gojinja/pkg/ext"
	"github.com/jryberg/gojinja/pkg/lexer"
)

// Expected values come from rendering the same templates with the Jinja2
// pinned in tools/parity/requirements.txt, under the pinned Python.

// TestPythonUnicodeData covers template behaviour that depends on Unicode
// data: it must follow the pinned Python (internal/pyunicode), never the
// Unicode version of the Go toolchain. Several characters here changed
// after Unicode 15.0, so a build whose Go has newer tables catches any
// code that consults Go's unicode package.
func TestPythonUnicodeData(t *testing.T) {
	cases := []struct{ env, src, want, err string }{
		{"", "{{ '\u019b'.upper() }}|{{ '\u019b'.title() }}|{{ '\u019bx'.capitalize() }}|{{ '\u019b'.swapcase() }}", "\u019b|\u019b|\u019bx|\u019b", ""},
		{"", "{{ '\u0295'.islower() }}|{{ '\u0295'.isalpha() }}|{{ '\u1c89'.lower() }}|{{ '\u1c89'.isupper() }}|{{ '\u1c8a'.upper() }}", "True|True|\u1c89|False|\u1c8a", ""},
		{"", "{{ '\\U0001e5d0'.isalpha() }}|{{ '\\U0001e5d0'.isprintable() }}|{{ '\\U0001e5d0x'.isidentifier() }}|{{ 'x\\U0001e5d0'.isidentifier() }}", "False|False|False|False", ""},
		{"", "{{ ['\u088f'] }}|{{ ['\\U0001e5d0'] }}|{{ '%r'|format('\u0897') }}", "['\\u088f']|['\\U0001e5d0']|'\\u0897'", ""},
		{"", "{{ '\\U00010d40'|int(-1) }}|{{ '\\U00010d40'.isdecimal() }}|{{ '\\U00010d41\\U00010d42'|float(-1) }}|{{ '\\U00010d40'.isnumeric() }}", "-1|False|-1|False", ""},
		{"", "{{ ('A\u019bB').lower() }}|{{ ('a\u019bb').title() }}|{{ ['b\u019b', 'A\u019b', '\u019ba']|sort|join(',') }}", "a\u019bb|A\u019bb|A\u019b,b\u019b,\u019ba", ""},
		{"", "{{ ('a' ~ '\u019b' ~ 'b c')|wordcount }}|{{ ('x\u019by z')|title }}|{{ ('\u019bxY')|capitalize }}|{{ 'a\u019b'|upper }}", "2|X\u019by Z|\u019bxy|A\u019b", ""},
		{"", "{{ 'www.a\u0295b.com x\u1c89y.org'|urlize }}", "<a href=\"https://www.a\u0295b.com\" rel=\"noopener\">www.a\u0295b.com</a> x\u1c89y.org", ""},
		{"", "{{ 'a\u03a3\u0345'.lower() }}|{{ '\u0345\u03a3'.lower() }}|{{ 'A\u0345\u03a3 b'.lower() }}", "a\u03c2\u0345|\u0345\u03c3|a\u0345\u03c2 b", ""},
		{"", "{{ '\\x1cab\\x1f'|trim }}|{{ '\\x1cab\\x1c'.strip() }}|{{ 'a\\x1cb'.split() }}|{{ ('a\\x1db c')|wordcount }}", "ab|ab|['a', 'b']|3", ""},
		{"", "{{ 'www.x.com'|urlize(rel='a\\x1cb') }}", "<a href=\"https://www.x.com\" rel=\"a b noopener\">www.x.com</a>", ""},
		{"", "{{ (1.0).fromhex(' 0x1p1\\t') }}|{{ '\u00a01.5\u3000'|float }}|{{ '\u0130nf'|float(-1) }}|{{ 'INF'|float }}", "2.0|1.5|-1|inf", ""},
		{"", "{{ (1.0).fromhex('\u00a00x1p1') }}", "", "invalid hexadecimal floating-point string"},
		{"", "{{ (1.0).fromhex('\u0130nf') }}", "", "invalid hexadecimal floating-point string"},
		{"", "{{ 'x'.encode('utf\u01308') }}|{{ 'x'.encode('UTF-8') }}|{{ 'x'.encode('Latin_1') }}", "b'x'|b'x'|b'x'", ""},
		{"", "{{ 'x'.encode('\u212aoi8-r') }}", "", "unknown encoding: \u212aoi8-r"},
		{"", "{{ 'x'.encode('utf\\x008') }}", "", "embedded null character"},
		{"", "{{ 'x'.encode('utf-8', 'str\\x00ict') }}", "", "embedded null character"},
		{"", "{{ ''.encode().decode('a\\x00') }}", "", "embedded null character"},
		{"", "{{ [[1, 2, 3, 4]]|map(attribute='\u0663')|list }}", "[4]", ""},
		{"", "{{ [{'a': 1}]|map(attribute='\u00b2', default='d')|list }}", "", "invalid literal for int() with base 10: '\u00b2'"},
		{"", "{{\u000b1\u000b}}|{{\u001c2\u001c}}|{{\u30003\u3000}}|{{\u00854\u0085}}", "1|2|3|4", ""},
		{"", "a\u3000{{- 1 }}|{{ 2 -}}\u3000\u001cb|{#\u3000c\u3000-#}\u3000d", "a1|2b|d", ""},
		{"", "{%\u3000raw\u3000%}{{ x }}{%\u3000endraw\u3000%}", "{{ x }}", ""},
		{"", "{{ 1\u0663 }}|{{ 1_\u0663 }}|{{ 0x\u0661f }}|{{ 9\u0669\u0669 * 2 }}|{{ 1\U0001d7d9 }}", "13|13|31|1998|11", ""},
		{"", "{{ 1\u0663.5 }}", "", "invalid character '\u0663' (U+0663) (<unknown>, line 1)"},
		{"", "{{ 1.\u0665 }}", "", "invalid character '\u0665' (U+0665) (<unknown>, line 1)"},
		{"", "{{ 1e\u0663 }}", "", "invalid decimal literal (<unknown>, line 1)"},
		{"", "{{ 1.5\U0001d7d9 }}", "", "invalid character '\U0001d7d9' (U+1D7D9) (<unknown>, line 1)"},
		{"", "{{ \u00b2 }}", "", "Invalid character in identifier"},
		{"", "{{ 0b1\u0661 }}", "", "Invalid character in identifier"},
		{"", "{{ \u037a }}", "", "Invalid character in identifier"},
		{"lstrip", "a\n\u3000\u001c{% if 1 %}b{% endif %}", "a\nb", ""},
		{"i18n", "{% trans trimmed %}  a  b\t\tc \n  d {% endtrans %}", "a  b\t\tc d", ""},
		{"i18n", "{% trans trimmed %}\u001ca\u001c\n\u001cb\u001c{% endtrans %}", "a b", ""},
	}
	lstrip := lexer.DefaultOptions()
	lstrip.LstripBlocks = true
	envs := map[string]*Environment{
		"":       mustEnv(t, WithAutoescape(AutoescapeNever{})),
		"i18n":   mustEnv(t, WithAutoescape(AutoescapeNever{}), WithI18NExtension(ext.NullTranslator{})),
		"lstrip": mustEnv(t, WithAutoescape(AutoescapeNever{}), WithLexerOptions(lstrip)),
	}
	for _, c := range cases {
		got, err := renderString(envs[c.env], c.src)
		switch {
		case c.err != "" && (err == nil || !strings.Contains(err.Error(), c.err)):
			t.Errorf("%+q: got %+q, %v; want error %+q", c.src, got, err, c.err)
		case c.err == "" && err != nil:
			t.Errorf("%+q: unexpected error %v", c.src, err)
		case c.err == "" && got != c.want:
			t.Errorf("%+q = %+q, want %+q", c.src, got, c.want)
		}
	}
}

// TestName is test_lexnparse.py's TestLexer.test_name: which names are
// valid identifiers, including the start characters \w lacks.
func TestName(t *testing.T) {
	cases := []struct {
		name  string
		valid bool
	}{
		{"foo", true},
		{"f\u00f6\u00f6", true},
		{"\u304d", true},
		{"_", true},
		{"1a", false},
		{"a-", false},
		{"\U0001f40da", false},
		{"a\U0001f40d\U0001f40d", false},
		{"\u1885", true},
		{"\u1886", true},
		{"\u2118", true},
		{"\u212e", true},
		{"\u00b7", false},
		{"a\u00b7", true},
	}
	e := mustEnv(t)
	for _, c := range cases {
		_, err := e.FromString("{{ " + c.name + " }}")
		if c.valid && err != nil {
			t.Errorf("%+q: unexpected error %v", c.name, err)
		}
		if !c.valid && err == nil {
			t.Errorf("%+q: want a TemplateSyntaxError", c.name)
		}
	}
}

func renderString(e *Environment, src string) (string, error) {
	tpl, err := e.FromString(src)
	if err != nil {
		return "", err
	}
	return tpl.RenderContext(context.Background(), nil)
}
