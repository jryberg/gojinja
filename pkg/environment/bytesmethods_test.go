package environment

import (
	"context"
	"strings"
	"testing"
)

// Expected values come from rendering the same templates with the Jinja2
// pinned in tools/parity/requirements.txt (that Python's bytes).

// Every bytes method, with Python's ASCII-only rules, int needles, slice
// arguments, keyword arguments and CPython's errors.
func TestBytesMethods(t *testing.T) {
	cases := []struct{ src, want, err string }{
		{`{% set b = 'Hello wORLD é'.encode() %}{{ b.upper() }}|{{ b.lower() }}|{{ b.swapcase() }}|{{ b.capitalize() }}|{{ b.title() }}|{{ 'ab cd-ef'.encode().title() }}`, "b'HELLO WORLD \\xc3\\xa9'|b'hello world \\xc3\\xa9'|b'hELLO World \\xc3\\xa9'|b'Hello world \\xc3\\xa9'|b'Hello World \\xc3\\xa9'|b'Ab Cd-Ef'", ""},
		{`{% set b = 'abc12'.encode() %}{{ b.isalnum() }}|{{ b.isalpha() }}|{{ '12'.encode().isdigit() }}|{{ 'é'.encode().isalpha() }}|{{ ''.encode().isascii() }}|{{ 'é'.encode().isascii() }}|{{ ' \x0b'.encode().isspace() }}|{{ '\x1c'.encode().isspace() }}`, "True|False|True|False|True|False|True|False", ""},
		{`{{ 'abc'.encode().islower() }}|{{ 'ABC1'.encode().isupper() }}|{{ 'Ab Cd'.encode().istitle() }}|{{ 'ab'.encode().istitle() }}|{{ '1'.encode().islower() }}`, "True|True|True|False|False", ""},
		{`{% set b = 'abcabc'.encode() %}{{ b.find('c'.encode()) }}|{{ b.find(99) }}|{{ b.rfind(98) }}|{{ b.count('b'.encode(), -3) }}|{{ b.index('c'.encode(), 3) }}|{{ b.find(''.encode(), 9) }}|{{ b.count(''.encode()) }}`, "2|2|4|1|5|-1|7", ""},
		{`{{ 'abc'.encode().find(300) }}`, "", "byte must be in range(0, 256)"},
		{`{{ 'abc'.encode().find('a') }}`, "", "argument should be integer or bytes-like object, not 'str'"},
		{`{{ 'abc'.encode().index('z'.encode()) }}`, "", "subsection not found"},
		{`{% set b = 'abc'.encode() %}{{ b.startswith('a'.encode()) }}|{{ b.endswith(('x'.encode(), 'bc'.encode())) }}|{{ b.startswith('b'.encode(), 1) }}|{{ b.endswith('b'.encode(), 0, 2) }}`, "True|True|True|True", ""},
		{`{{ 'abc'.encode().startswith('a') }}`, "", "startswith first arg must be bytes or a tuple of bytes, not str"},
		{`{{ 'ab'.encode().center(6) }}|{{ 'ab'.encode().center(5, '*'.encode()) }}|{{ 'ab'.encode().ljust(4, '.'.encode()) }}|{{ 'ab'.encode().rjust(4) }}|{{ '-4'.encode().zfill(5) }}`, "b'  ab  '|b'**ab*'|b'ab..'|b'  ab'|b'-0004'", ""},
		{`{{ 'ab'.encode().center(5, '*') }}`, "", "center() argument 2 must be a byte string of length 1, not str"},
		{`{{ '  ab \x1c'.encode().strip() }}|{{ 'xxabxx'.encode().lstrip('x'.encode()) }}|{{ 'xxabxx'.encode().rstrip('x'.encode()) }}`, "b'ab \\x1c'|b'abxx'|b'xxab'", ""},
		{`{{ 'ab'.encode().strip('a') }}`, "", "a bytes-like object is required, not 'str'"},
		{`{{ 'aaa'.encode().replace('a'.encode(), 'b'.encode(), 2) }}|{{ 'abc'.encode().replace(''.encode(), '-'.encode()) }}|{{ 'abc'.encode().replace(''.encode(), '-'.encode(), 2) }}`, "b'bba'|b'-a-b-c-'|b'-a-bc'", ""},
		{`{{ '-'.encode().join(['a'.encode(), 'b'.encode()]) }}|{{ ''.encode().join([]) }}`, "b'a-b'|b''", ""},
		{`{{ '-'.encode().join(['a']) }}`, "", "sequence item 0: expected a bytes-like object, str found"},
		{`{{ 'a,b,c'.encode().partition(','.encode()) }}|{{ 'a,b,c'.encode().rpartition(','.encode()) }}|{{ 'abc'.encode().partition('x'.encode()) }}|{{ 'abc'.encode().rpartition('x'.encode()) }}`, "(b'a', b',', b'b,c')|(b'a,b', b',', b'c')|(b'abc', b'', b'')|(b'', b'', b'abc')", ""},
		{`{{ 'TestX'.encode().removeprefix('Test'.encode()) }}|{{ 'XTests'.encode().removesuffix('Tests'.encode()) }}`, "b'X'|b'X'", ""},
		{`{{ 'a b  c'.encode().split() }}|{{ 'a b c'.encode().split(None, 1) }}|{{ '  a  b  '.encode().rsplit(None, 1) }}|{{ 'a,b,c'.encode().split(','.encode(), 1) }}|{{ 'a,b,c'.encode().rsplit(sep=','.encode(), maxsplit=1) }}|{{ 'a\x1cb'.encode().split() }}`, "[b'a', b'b', b'c']|[b'a', b'b c']|[b'  a', b'b']|[b'a', b'b,c']|[b'a,b', b'c']|[b'a\\x1cb']", ""},
		{`{{ 'a\nb\r\nc\x0bd'.encode().splitlines() }}|{{ 'a\nb'.encode().splitlines(keepends=true) }}`, "[b'a', b'b', b'c\\x0bd']|[b'a\\n', b'b']", ""},
		{`{{ 'a\tbc\td'.encode().expandtabs(4) }}`, "b'a   bc  d'", ""},
		{`{{ 'abc'.encode().hex() }}|{{ 'abcdef'.encode().hex(':') }}|{{ 'abcde'.encode().hex(':', 2) }}|{{ 'abcde'.encode().hex('-', -2) }}|{{ 'abc'.encode().hex(':'.encode()) }}|{{ ''.encode().hex(':') }}`, "616263|61:62:63:64:65:66|61:6263:6465|6162-6364-65|61:62:63|", ""},
		{`{{ 'abc'.encode().hex('::') }}`, "", "sep must be length 1."},
		{`{{ ''.encode().fromhex('41 42ff') }}|{{ ''.encode().fromhex('') }}`, "b'AB\\xff'|b''", ""},
		{`{{ ''.encode().fromhex('4g') }}`, "", "non-hexadecimal number found in fromhex() arg at position 1"},
		{`{{ ''.encode().fromhex('414') }}`, "", "non-hexadecimal number found in fromhex() arg at position 3"},
		{`{{ 'abc'.encode().translate(none, 'b'.encode()) }}|{{ 'abc'.encode().translate(''.encode().maketrans('ab'.encode(), 'xy'.encode())) }}|{{ 'abc'.encode().translate(''.encode().maketrans('a'.encode(), 'z'.encode()), delete='c'.encode()) }}`, "b'ac'|b'xyc'|b'zb'", ""},
		{`{{ 'abc'.encode().translate('x'.encode()) }}`, "", "translation table must be 256 characters long"},
		{`{{ 'abc'.encode().translate(table=none) }}`, "", "translate() takes at least 1 positional argument (0 given)"},
		{`{{ ''.encode().maketrans('a'.encode(), 'xy'.encode()) }}`, "", "maketrans arguments must have same length"},
		{`{{ 'a'.encode().upper(1) }}`, "", "bytes.upper() takes no arguments (1 given)"},
		{`{{ 'a'.encode().center() }}`, "", "center expected at least 1 argument, got 0"},
		{`{{ 'a'.encode().find() }}`, "", "find() takes at least 1 argument (0 given)"},
		{`{{ 'a'.encode().split(zz=1) }}`, "", "'zz' is an invalid keyword argument for split()"},
		{`{{ 'a'.encode().join() }}`, "", "bytes.join() takes exactly one argument (0 given)"},
		{`{{ 'a'.encode().maketrans() }}`, "", "maketrans expected 2 arguments, got 0"},
		{`{{ 'a'.encode().zfill('x') }}`, "", "'str' object cannot be interpreted as an integer"},
		{`{{ 'a'.encode().nosuch() }}`, "", "'bytes object' has no attribute 'nosuch'"},
	}
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	for _, c := range cases {
		tpl, err := e.FromString(c.src)
		if err != nil {
			t.Fatalf("FromString(%q): %v", c.src, err)
		}
		got, err := tpl.RenderContext(context.Background(), nil)
		switch {
		case c.err != "" && (err == nil || !strings.HasSuffix(err.Error(), c.err)):
			t.Errorf("%s: got %q, %v; want error %q", c.src, got, err, c.err)
		case c.err == "" && err != nil:
			t.Errorf("%s: unexpected error %v", c.src, err)
		case c.err == "" && got != c.want:
			t.Errorf("%s = %q, want %q", c.src, got, c.want)
		}
	}
}
