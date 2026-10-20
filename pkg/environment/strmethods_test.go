package environment

import (
	"context"
	"strings"
	"testing"
	"unicode"
)

// Expected values come from rendering the same templates with the Jinja2
// pinned in tools/parity/requirements.txt (that Python's str and bytes).

// Every Python str method, with Unicode text, slice arguments, keyword
// arguments and CPython's errors, plus str.encode and bytes.decode.
func TestStrMethods(t *testing.T) {
	cases := []struct{ src, want, err string }{
		{`{{ 'hello wORLD'.capitalize() }}|{{ 'éCOLE'.capitalize() }}|{{ 'ǆa'.capitalize() }}`, "Hello world|\u00c9cole|\u01c5a", ""},
		{`{{ 'Straße'.casefold() }}|{{ 'ÉCOLE'.casefold() }}`, "strasse|\u00e9cole", ""},
		{`{{ 'ab'.center(5) }}|{{ 'ab'.center(6, '*') }}|{{ 'é'.center(4, 'ü') }}|{{ 'ab'.center(1) }}`, "  ab |**ab**|\u00fc\u00e9\u00fc\u00fc|ab", ""},
		{`{{ 'aaaa'.count('aa') }}|{{ 'abcabc'.count('b', 2) }}|{{ 'abcabc'.count('b', -3, -1) }}|{{ 'abc'.count('') }}|{{ 'éé'.count('é') }}`, "2|1|1|4|2", ""},
		{`{{ 'abc'.encode() }}|{{ 'é'.encode() }}|{{ 'é'.encode('latin-1') }}|{{ 'a\n\''.encode() }}`, "b'abc'|b'\\xc3\\xa9'|b'\\xe9'|b\"a\\n'\"", ""},
		{`{{ 'abc'.endswith('c') }}|{{ 'abc'.endswith(('x', 'bc')) }}|{{ 'abc'.endswith('b', 0, 2) }}|{{ 'abc'.endswith('a', -5, 1) }}`, "True|True|True|True", ""},
		{`{{ 'a\tb\tc'.expandtabs() }}|{{ 'a\tbc\td'.expandtabs(4) }}|{{ 'ab\n\tc'.expandtabs(2) }}|{{ 'é\tx'.expandtabs(3) }}`, "a       b       c|a   bc  d|ab\n  c|\u00e9  x", ""},
		{`{{ 'abcabc'.find('c') }}|{{ 'abcabc'.find('c', 3) }}|{{ 'abcabc'.find('c', -2) }}|{{ 'abc'.find('z') }}|{{ 'éab'.find('a') }}|{{ 'abc'.find('', 5) }}|{{ 'abc'.find('', 3) }}`, "2|5|5|-1|1|-1|3", ""},
		{`{{ 'abcabc'.rfind('b') }}|{{ 'abcabc'.rfind('b', 0, 3) }}|{{ 'éaé'.rfind('é') }}|{{ 'abc'.rfind('') }}`, "4|1|2|3", ""},
		{`{{ 'abc'.index('b') }}|{{ 'éab'.index('b') }}|{{ 'abcb'.rindex('b') }}`, "1|2|3", ""},
		{`{{ 'abc'.index('z') }}`, "", "substring not found"},
		{`{{ 'ab1'.isalnum() }}|{{ 'ab 1'.isalnum() }}|{{ ''.isalnum() }}|{{ 'é'.isalpha() }}|{{ 'a1'.isalpha() }}|{{ 'abc'.isascii() }}|{{ 'é'.isascii() }}|{{ ''.isascii() }}`, "True|False|False|True|False|True|False|True", ""},
		{`{{ '123'.isdecimal() }}|{{ '²'.isdecimal() }}|{{ '²'.isdigit() }}|{{ '½'.isnumeric() }}|{{ '½'.isdigit() }}|{{ '٣'.isdecimal() }}`, "True|False|True|True|False|True", ""},
		{`{{ 'abc_1'.isidentifier() }}|{{ '1abc'.isidentifier() }}|{{ 'é'.isidentifier() }}|{{ ''.isidentifier() }}|{{ 'a-b'.isidentifier() }}`, "True|False|True|False|False", ""},
		{`{{ 'abc'.islower() }}|{{ 'aBc'.islower() }}|{{ '1'.islower() }}|{{ 'ABC1'.isupper() }}|{{ 'é'.islower() }}`, "True|False|False|True|True", ""},
		{`{{ 'ab c'.isprintable() }}|{{ 'a\n'.isprintable() }}|{{ ''.isprintable() }}|{{ 'a\u200b'.isprintable() }}`, "True|False|True|False", ""},
		{`{{ ' \t'.isspace() }}|{{ ''.isspace() }}|{{ '\x1c'.isspace() }}|{{ '\u00a0'.isspace() }}`, "True|False|True|True", ""},
		{`{{ 'Hello World'.istitle() }}|{{ 'Hello world'.istitle() }}|{{ 'HELLO'.istitle() }}|{{ ''.istitle() }}|{{ 'Ab Cd1 Ef'.istitle() }}|{{ "They'Re".istitle() }}`, "True|False|False|False|True|True", ""},
		{`{{ '-'.join(['a', 'b']) }}|{{ ''.join('abc') }}|{{ ', '.join(('x',)) }}`, "a-b|abc|x", ""},
		{`{{ '-'.join([1, 2]) }}`, "", "sequence item 0: expected str instance, int found"},
		{`{{ 'ab'.ljust(5) }}|{{ 'ab'.ljust(5, '*') }}|{{ 'é'.ljust(3, '.') }}|{{ 'ab'.rjust(5) }}|{{ 'é'.rjust(3, '0') }}`, "ab   |ab***|\u00e9..|   ab|00\u00e9", ""},
		{`{{ 'ab'.ljust(5, '**') }}`, "", "The fill character must be exactly one character long"},
		{`{{ 'HeLLo'.lower() }}|{{ 'İ'.lower()|length }}|{{ 'ÉCOLE'.lower() }}`, "hello|2|\u00e9cole", ""},
		{`{{ '  ab  '.lstrip() }}|{{ 'xxabxx'.lstrip('x') }}|{{ 'xxabxx'.rstrip('x') }}|{{ 'abcba'.strip('ab') }}|{{ '\u00a0ab\u2003'.strip() }}`, "ab  |abxx|xxab|c|ab", ""},
		{`{{ 'a,b,c'.partition(',') }}|{{ 'abc'.partition('x') }}|{{ 'a,b,c'.rpartition(',') }}|{{ 'abc'.rpartition('x') }}`, "('a', ',', 'b,c')|('abc', '', '')|('a,b', ',', 'c')|('', '', 'abc')", ""},
		{`{{ 'abc'.partition('') }}`, "", "empty separator"},
		{`{{ 'TestHook'.removeprefix('Test') }}|{{ 'TestHook'.removeprefix('X') }}|{{ 'MiscTests'.removesuffix('Tests') }}|{{ 'a'.removesuffix('') }}`, "Hook|TestHook|Misc|a", ""},
		{`{{ 'aaa'.replace('a', 'b') }}|{{ 'aaa'.replace('a', 'b', 2) }}|{{ 'abc'.replace('', '-') }}|{{ 'abc'.replace('', '-', 2) }}|{{ 'éé'.replace('é', 'e', 1) }}`, "bbb|bba|-a-b-c-|-a-bc|e\u00e9", ""},
		{`{{ 'a b  c'.split() }}|{{ 'a,b,,c'.split(',') }}|{{ 'a b c'.split(None, 1) }}|{{ '  a  b  '.split() }}|{{ 'a,b,c'.split(',', 1) }}|{{ ''.split() }}|{{ ''.split(',') }}`, "['a', 'b', 'c']|['a', 'b', '', 'c']|['a', 'b c']|['a', 'b']|['a', 'b,c']|[]|['']", ""},
		{`{{ 'a b c'.rsplit(None, 1) }}|{{ 'a,b,c'.rsplit(',', 1) }}|{{ '  a  b  '.rsplit(None, 1) }}|{{ 'a\u2003b'.split() }}`, "['a b', 'c']|['a,b', 'c']|['  a', 'b']|['a', 'b']", ""},
		{`{{ 'abc'.split('') }}`, "", "empty separator"},
		{`{{ 'abc'.startswith('a') }}|{{ 'abc'.startswith(('x', 'ab')) }}|{{ 'abc'.startswith('b', 1) }}|{{ 'abc'.startswith('', 3) }}|{{ 'abc'.startswith('', 4) }}`, "True|True|True|True|False", ""},
		{`{{ 'Hello World'.swapcase() }}|{{ 'ß'.swapcase() }}|{{ 'ǅ'.swapcase() }}`, "hELLO wORLD|SS|\u01c5", ""},
		{`{{ "they're bill's friends".title() }}|{{ 'hello world'.title() }}|{{ 'ab1cd e_f'.title() }}|{{ 'éCOLE ünd'.title() }}`, "They'Re Bill'S Friends|Hello World|Ab1Cd E_F|\u00c9cole \u00dcnd", ""},
		{`{{ 'hello'.upper() }}|{{ 'straße'.upper() }}|{{ 'ǆ'.upper() }}`, "HELLO|STRASSE|\u01c4", ""},
		{`{{ '42'.zfill(5) }}|{{ '-42'.zfill(5) }}|{{ '+4'.zfill(4) }}|{{ 'é'.zfill(3) }}|{{ 'abc'.zfill(2) }}`, "00042|-0042|+004|00\u00e9|abc", ""},
		{`{{ 'abc'.translate({97: 'X', 98: none}) }}|{{ 'abc'.translate({97: 120}) }}`, "Xc|xbc", ""},
		{`{{ 'abc'.maketrans('ab', 'xy') }}`, "{97: 120, 98: 121}", ""},
		{`{{ 'a{0}b'.format(1) }}|{{ 'x'.format_map({}) }}`, "a1b|x", ""},
		{`{{ 'abc'.splitlines() }}|{{ 'a\nb'.splitlines() }}`, "['abc']|['a', 'b']", ""},
		{`{{ 'abc'.isdigit() }}|{{ '12'.isdigit() }}`, "False|True", ""},
		{`{{ 'ǅ'.istitle() }}|{{ 'ǅ'.title() }}|{{ 'ǅ'.capitalize() }}|{{ 'ǅ'.lower() }}|{{ 'ǅ'.isupper() }}`, "True|\u01c5|\u01c5|\u01c6|False", ""},
		{`{{ 'ﬁ'.upper() }}|{{ 'ﬁ'.capitalize() }}|{{ 'ﬁ'.title() }}`, "FI|Fi|Fi", ""},
		{`{{ 'héllo'.encode() }}|{{ 'héllo'.encode()|length }}|{{ 'abc'.encode()[0] }}|{{ 'abc'.encode()[1:] }}|{{ 'abc'.encode()|list }}`, "b'h\\xc3\\xa9llo'|6|97|b'bc'|[97, 98, 99]", ""},
		{`{{ 'abc'.encode() == 'abc'.encode() }}|{{ 'abc'.encode() == 'abc' }}|{{ 'a'.encode() + 'b'.encode() }}|{{ 'ab'.encode() * 2 }}|{{ ''.encode() }}|{{ 'x' if ''.encode() else 'empty' }}`, "True|False|b'ab'|b'abab'|b''|empty", ""},
		{`{{ 'héllo'.encode().decode() }}|{{ 'é'.encode('latin-1').decode('latin-1') }}|{{ 'é'.encode('utf_16') }}|{{ 'é'.encode('utf-16-be') }}|{{ '😀'.encode('utf-16-le') }}|{{ 'é'.encode('UTF-32') }}`, "h\u00e9llo|\u00e9|b'\\xff\\xfe\\xe9\\x00'|b'\\x00\\xe9'|b'=\\xd8\\x00\\xde'|b'\\xff\\xfe\\x00\\x00\\xe9\\x00\\x00\\x00'", ""},
		{`{{ 'é€'.encode('ascii', 'replace') }}|{{ 'é€'.encode('ascii', 'ignore') }}|{{ 'é€'.encode('ascii', 'backslashreplace') }}|{{ 'é€'.encode('latin-1', 'xmlcharrefreplace') }}|{{ 'é'.encode(encoding='latin1') }}`, "b'??'|b''|b'\\\\xe9\\\\u20ac'|b'\\xe9&#8364;'|b'\\xe9'", ""},
		{`{{ 'aé'.encode('ascii') }}`, "", "'ascii' codec can't encode character '\\xe9' in position 1: ordinal not in range(128)"},
		{`{{ 'aé€'.encode('ascii') }}`, "", "'ascii' codec can't encode characters in position 1-2: ordinal not in range(128)"},
		{`{{ '€'.encode('latin-1') }}`, "", "'latin-1' codec can't encode character '\\u20ac' in position 0: ordinal not in range(256)"},
		{`{{ 'x'.encode('nope') }}`, "", "unknown encoding: nope"},
		{`{{ 'é'.encode('ascii', 'nope') }}`, "", "unknown error handler name 'nope'"},
		{`{{ 'x'.encode('ascii', 'nope') }}`, "b'x'", ""},
		{`{{ 'é'.encode('utf-8').decode('ascii') }}`, "", "'ascii' codec can't decode byte 0xc3 in position 0: ordinal not in range(128)"},
		{`{{ 'é'.encode('latin-1').decode() }}`, "", "'utf-8' codec can't decode byte 0xe9 in position 0: unexpected end of data"},
		{`{{ ('é'.encode() + 'a'.encode() )[0:1].decode() }}`, "", "'utf-8' codec can't decode byte 0xc3 in position 0: unexpected end of data"},
		{`{{ 'é'.encode('latin-1').decode('utf-8', 'replace') }}|{{ 'é'.encode('latin-1').decode('utf-8', 'ignore') }}|{{ 'é'.encode('latin-1').decode('utf-8', 'backslashreplace') }}`, "\ufffd||\\xe9", ""},
		{`{{ '"\''.encode() }}|{{ '\''.encode() }}|{{ '\t\x00\x7f'.encode() }}|{{ ['a'.encode()] }}`, "b'\"\\''|b\"'\"|b'\\t\\x00\\x7f'|[b'a']", ""},
		{`{{ 'a'.upper(1) }}`, "", "str.upper() takes no arguments (1 given)"},
		{`{{ 'a'.center() }}`, "", "center expected at least 1 argument, got 0"},
		{`{{ 'a'.center(1, 'x', 'y') }}`, "", "center expected at most 2 arguments, got 3"},
		{`{{ 'a'.center(width=3) }}`, "", "str.center() takes no keyword arguments"},
		{`{{ 'a'.find() }}`, "", "find() takes at least 1 argument (0 given)"},
		{`{{ 'a'.find('a', 1, 2, 3) }}`, "", "find() takes at most 3 arguments (4 given)"},
		{`{{ 'a'.find(sub='a') }}`, "", "find() takes no keyword arguments"},
		{`{{ 'a'.join() }}`, "", "str.join() takes exactly one argument (0 given)"},
		{`{{ 'a'.split(1, 2, 3) }}`, "", "split() takes at most 2 arguments (3 given)"},
		{`{{ 'a'.split(zz=1) }}`, "", "'zz' is an invalid keyword argument for split()"},
		{`{{ 'a b'.split(' ', sep=' ') }}`, "", "argument for split() given by name ('sep') and position (1)"},
		{`{{ 'a b c'.split(maxsplit=1) }}|{{ 'a,b,c'.rsplit(sep=',', maxsplit=1) }}|{{ 'a\nb'.splitlines(keepends=true) }}|{{ 'a\tb'.expandtabs(tabsize=2) }}`, "['a', 'b c']|['a,b', 'c']|['a\\n', 'b']|a b", ""},
		{`{{ 'a'.lstrip(1) }}`, "", "lstrip arg must be None or str"},
		{`{{ 'a'.find(1) }}`, "", "must be str, not int"},
		{`{{ 'a'.startswith(1) }}`, "", "startswith first arg must be str or a tuple of str, not int"},
		{`{{ 'a'.startswith(('a', 1)) }}`, "True", ""},
		{`{{ 'a'.replace(1, 'b') }}`, "", "replace() argument 1 must be str, not int"},
		{`{{ 'a'.center('x') }}`, "", "'str' object cannot be interpreted as an integer"},
		{`{{ 'a'.center(3, 1) }}`, "", "The fill character must be a unicode character, not int"},
		{`{{ 'a'.zfill('x') }}`, "", "'str' object cannot be interpreted as an integer"},
		{`{{ 'a'.find('a', 'x') }}`, "", "slice indices must be integers or None or have an __index__ method"},
		{`{{ 'a'.removeprefix(1) }}`, "", "removeprefix() argument must be str, not int"},
		{`{{ 'a'.partition(1) }}`, "", "must be str, not int"},
		{`{{ 'a'.join([1]) }}`, "", "sequence item 0: expected str instance, int found"},
		{`{{ 'a'.join(5) }}`, "", "can only join an iterable"},
		{`{{ 'abc'.translate(5) }}`, "", "'int' object is not subscriptable"},
		{`{{ 'abc'.translate({97: 1.5}) }}`, "", "character mapping must return integer, None or str"},
		{`{{ 'abc'.maketrans('a', 'xy') }}`, "", "the first two maketrans arguments must have equal length"},
		{`{{ 'abc'.maketrans(5) }}`, "", "if you give only one argument to maketrans it must be a dict"},
		{`{{ 'abc'.maketrans({'ab': 1}) }}`, "", "string keys in translate table must be of length 1"},
		{`{{ 'abc'.maketrans({97: 'x', 'b': none}) }}|{{ 'abc'.translate('abc'.maketrans('ab', 'xy', 'c')) }}`, "{97: 'x', 98: None}|xy", ""},
		{`{{ 'abc'.index('z') }}`, "", "substring not found"},
		{`{{ 'a'.encode(1) }}`, "", "encode() argument 'encoding' must be str, not int"},
		{`{{ 'x'.decode() }}`, "", "'str object' has no attribute 'decode'"},
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

// strtables_table.go only tabulates what Go's unicode package lacks, which
// is right only while both use the same Unicode version.
func TestStrTablesMatchGoUnicode(t *testing.T) {
	if unicode.Version != pyUnicodeVersion {
		t.Fatalf("Go has Unicode %s but strtables_table.go was generated from Unicode %s; run make strtables with a matching Python", unicode.Version, pyUnicodeVersion)
	}
}
