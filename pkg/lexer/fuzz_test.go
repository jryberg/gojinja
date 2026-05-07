package lexer

import (
	"strings"
	"testing"
)

// FuzzLexerNoPanic asserts the lexer never panics on arbitrary input and
// always either returns tokens (terminated by EOF) or a typed error.
//
// Run with `go test -run=^$ -fuzz=FuzzLexerNoPanic -fuzztime=30s ./pkg/lexer`.
func FuzzLexerNoPanic(f *testing.F) {
	seeds := []string{
		"",
		"plain text",
		"{{ name }}",
		"{% if x %}{% endif %}",
		"{# comment #}",
		"{% raw %}{{ not_a_var }}{% endraw %}",
		"{{ 1 + 2 * 3 }}",
		"{{ 'hello\\nworld' }}",
		"{{ obj.attr.nested }}",
		"{{ a, b, c }}",
		"{{",       // unterminated
		"{% raw %", // unterminated
		"{# unfinished",
		"{{ '\\u00ff' }}",
		"{{ 0xff_ff }}",
		"{{ 1e-10 }}",
		"{{- x -}}",
		"{%- if x +%}{% endif %}",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	l, err := New(DefaultOptions())
	if err != nil {
		f.Fatalf("constructing lexer: %v", err)
	}

	f.Fuzz(func(t *testing.T, src string) {
		// Cap input length so individual fuzz cases stay quick.
		if len(src) > 16*1024 {
			return
		}
		stream, err := l.Tokenize(src, "fuzz", "")
		if err != nil {
			return // typed error — fine
		}
		// Drain — the only requirement is termination + no panic.
		seen := 0
		for !stream.EOS() {
			tk := stream.Next()
			if tk.Lineno < 1 {
				t.Fatalf("non-positive lineno: %#v in src=%q", tk, src)
			}
			seen++
			if seen > 1<<20 {
				t.Fatalf("token storm; lexer not advancing? src=%q", src)
			}
		}
	})
}

// TestLexerCornerCases exercises a few edge cases that are likely to break
// the state machine: empty input, only whitespace, only a comment, mixed
// directives, and the float-lookbehind workaround.
func TestLexerCornerCases(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"empty", ""},
		{"whitespace_only", "   \n   "},
		{"comment_only", "{# just a comment #}"},
		{"adjacent_directives", "{% if x %}{{ y }}{% endif %}"},
		{"float_after_dot_method", "{{ x.5 }}"}, // tricky: .5 is not a number here
		{"nested_brackets", "{{ a[b[c[d]]] }}"},
		{"escape_in_dict_key", "{{ {'a\\nb': 1} }}"},
		{"keep_trailing_newline_default", "x\n"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			l, err := New(DefaultOptions())
			if err != nil {
				t.Fatal(err)
			}
			stream, err := l.Tokenize(c.src, "corner", "")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			for !stream.EOS() {
				stream.Next()
			}
		})
	}
}

// TestUnterminatedConstructs verifies that we produce a syntax error
// (rather than panic or silently truncate) for malformed input.
func TestUnterminatedConstructs(t *testing.T) {
	cases := map[string]string{
		"unterminated_block":   "{% if x ",
		"unterminated_var":     "{{ name ",
		"unterminated_comment": "{# never closes",
		"unterminated_raw":     "{% raw %} body without end",
		"unterminated_string":  "{{ 'oops",
	}
	for name, src := range cases {
		name, src := name, src
		t.Run(name, func(t *testing.T) {
			l, err := New(DefaultOptions())
			if err != nil {
				t.Fatal(err)
			}
			_, err = l.Tokenize(src, name, "")
			if err == nil {
				t.Fatalf("expected error for %q, got nil", src)
			}
			if !strings.Contains(strings.ToLower(err.Error()), "unexpected") &&
				!strings.Contains(strings.ToLower(err.Error()), "missing") &&
				!strings.Contains(strings.ToLower(err.Error()), "end") {
				t.Fatalf("error doesn't read like a termination problem: %v", err)
			}
		})
	}
}

// TestLineStatement verifies that a configured line_statement_prefix is
// rewritten to a block tag.
func TestLineStatement(t *testing.T) {
	o := DefaultOptions()
	o.LineStatementPrefix = "##"
	src := "## if x\nhi\n## endif"
	toks, err := valuesOpts(t, src, o)
	if err != nil {
		t.Fatal(err)
	}
	// Line statements rewrite to BlockBegin/BlockEnd in wrap.
	got := 0
	for _, tk := range toks {
		if tk.Kind == TokenBlockBegin || tk.Kind == TokenBlockEnd {
			got++
		}
	}
	if got != 4 {
		t.Fatalf("expected 4 block delimiters from line statements, got %d: %#v", got, toks)
	}
}

// TestLineComment verifies that a line_comment_prefix consumes to EOL and
// does not produce parser-visible tokens.
func TestLineComment(t *testing.T) {
	o := DefaultOptions()
	o.LineCommentPrefix = "##"
	src := "before ## a comment\nafter"
	toks, err := valuesOpts(t, src, o)
	if err != nil {
		t.Fatal(err)
	}
	// No parser-visible token should carry the comment text.
	for _, tk := range toks {
		if strings.Contains(tk.Value, "a comment") {
			t.Fatalf("line-comment text leaked into token: %#v", tk)
		}
	}
}
