package environment

import (
	"context"
	"testing"
)

// Expected values come from rendering the same templates with the Jinja2
// pinned in tools/parity/requirements.txt (and that Python's textwrap).

// Port of TestFilter.test_wordwrap plus textwrap behaviour: hyphen and
// em-dash splitting, long words, kept line breaks, Unicode text, and the
// str.splitlines boundaries and repr escapes the filter relies on.
func TestWordwrapPort(t *testing.T) {
	cases := []struct {
		src     string
		want    string
		wantErr bool
	}{
		{`{{ string|wordwrap(20) }}`, "Hello!\nThis is Jinja saying\nsomething.", false},
		{`{{ 'a b c'|wordwrap(width=3, wrapstring='|') }}`, "a b|c", false},
		{`{{ 'aBc d'|wordwrap(2) }}`, "aB\nc\nd", false},
		{`{{ 'the quick brown fox jumps over the lazy dog'|wordwrap(10) }}`, "the quick\nbrown fox\njumps over\nthe lazy\ndog", false},
		{`{{ 'Hello there -- you goof-ball, use the -b option!'|wordwrap(10) }}`, "Hello\nthere --\nyou goof-\nball, use\nthe -b\noption!", false},
		{`{{ 'Hello there -- you goof-ball, use the -b option!'|wordwrap(10, break_on_hyphens=false) }}`, "Hello\nthere --\nyou\ngoof-ball,\nuse the -b\noption!", false},
		{`{{ 'supercalifragilisticexpialidocious word'|wordwrap(8) }}`, "supercal\nifragili\nsticexpi\nalidocio\nus word", false},
		{`{{ 'supercalifragilisticexpialidocious word'|wordwrap(8, false) }}`, "supercalifragilisticexpialidocious\nword", false},
		{`{{ 'well-known self-evident e-mail co-op a-b-c --x x-- 1-2 ab-12'|wordwrap(6) }}`, "well-\nknown\nself-e\nvident\ne-mail\nco-op\na-b-c\n--x\nx--\n1-2\nab-12", false},
		{`{{ 'line one is long\n\nline two\r\nthree\x0bfour'|wordwrap(5) }}`, "line\none\nis\nlong\n\nline\ntwo\nthree\nfour", false},
		{`{{ '   leading and trailing   '|wordwrap(7) }}`, "leading\nand tra\niling", false},
		{`{{ 'tab\there  and   spaces'|wordwrap(6) }}`, "tab\nhere\nand\nspaces", false},
		{`{{ 'über straße naïve façade'|wordwrap(6) }}`, "\u00fcber\nstra\u00dfe\nna\u00efve\nfa\u00e7ade", false},
		{`{{ 'x'|wordwrap(0) }}`, "", true},
		{`{{ ''|wordwrap(0) }}`, "", false},
		{`{{ 'a-b-c-d-e-f-g-h'|wordwrap(4) }}`, "a-b-\nc-d-\ne-f-\ng-h", false},
		{`{{ 'aaaa-bbbb-cccc'|wordwrap(6) }}`, "aaaa-\nbbbb-\ncccc", false},
		{`{{ '----abc'|wordwrap(3) }}`, "---\n-ab\nc", false},
		{`{{ 'foo--bar baz---qux'|wordwrap(5) }}`, "foo--\nbar\nbaz\n---\nqux", false},
		{`{{ 'x\u00a0y z'|wordwrap(3) }}`, "x\u00a0y\nz", false},
		{`{{ 'abc'|wordwrap(1) }}`, "a\nb\nc", false},
		{`{{ 'a\x1cb'.strip() }}|{{ '\x1c a \x1f'.strip() }}`, "a\u001cb|a", false},
		{`{{ 'a\x0bb\x0cc\x1cd\x85e\u2028f\r\ng\rh'.splitlines() }}`, "['a', 'b', 'c', 'd', 'e', 'f', 'g', 'h']", false},
		{`{{ 'a\nb\r\n'.splitlines(true) }}`, "['a\\n', 'b\\r\\n']", false},
		{`{{ 'a\x85b' }}|{{ ['a\x85b', '\u2028', '\x7f', 'é\u200b'] }}`, "a\u0085b|['a\\x85b', '\\u2028', '\\x7f', '\u00e9\\u200b']", false},
	}
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	vars := map[string]any{"string": "Hello!\nThis is Jinja saying something."}
	for _, c := range cases {
		tpl, err := e.FromString(c.src)
		if err != nil {
			t.Fatalf("FromString(%q): %v", c.src, err)
		}
		got, err := tpl.RenderContext(context.Background(), vars)
		switch {
		case c.wantErr && err == nil:
			t.Errorf("%s = %q, want an error", c.src, got)
		case !c.wantErr && err != nil:
			t.Errorf("%s: unexpected error %v", c.src, err)
		case !c.wantErr && got != c.want:
			t.Errorf("%s = %q, want %q", c.src, got, c.want)
		}
	}
}
