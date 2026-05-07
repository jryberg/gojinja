package lexer

import (
	"strings"
	"testing"
)

// values returns (kind, value) pairs.
func values(t *testing.T, src string) []Token {
	t.Helper()
	l, err := New(DefaultOptions())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	stream, err := l.Tokenize(src, "test", "")
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}
	var out []Token
	for !stream.EOS() {
		out = append(out, stream.Current())
		stream.Next()
	}
	return out
}

func valuesOpts(t *testing.T, src string, o Options) ([]Token, error) {
	t.Helper()
	l, err := New(o)
	if err != nil {
		return nil, err
	}
	stream, err := l.Tokenize(src, "test", "")
	if err != nil {
		return nil, err
	}
	var out []Token
	for !stream.EOS() {
		out = append(out, stream.Current())
		stream.Next()
	}
	return out, nil
}

// ---------------------------------------------------------- TokenStream tests

// Mirrors TestTokenStream.test_simple in tests/test_lexnparse.py.
func TestTokenStreamSimple(t *testing.T) {
	toks := []Token{
		{Lineno: 1, Kind: TokenBlockBegin},
		{Lineno: 2, Kind: TokenBlockEnd},
		{Lineno: 2, Kind: TokenEOF},
	}
	ts := NewTokenStream(toks, "foo", "bar")
	if ts.Current().Kind != TokenBlockBegin {
		t.Fatalf("first current = %v", ts.Current().Kind)
	}
	if ts.EOS() {
		t.Fatal("EOS true at start")
	}
	ts.Next()
	if ts.Current().Kind != TokenBlockEnd {
		t.Fatalf("after first next = %v", ts.Current().Kind)
	}
	ts.Next()
	if ts.Current().Kind != TokenEOF {
		t.Fatalf("after second next = %v", ts.Current().Kind)
	}
	if !ts.EOS() {
		t.Fatal("expected EOS after EOF reached")
	}
}

// TestTokenStreamPushPeek covers Look (peek-one) and Push round-trips.
func TestTokenStreamPushPeek(t *testing.T) {
	toks := []Token{
		{Lineno: 1, Kind: TokenName, Value: "a"},
		{Lineno: 1, Kind: TokenName, Value: "b"},
		{Lineno: 1, Kind: TokenEOF},
	}
	ts := NewTokenStream(toks, "", "")
	peek := ts.Look()
	if peek.Value != "b" {
		t.Fatalf("Look() = %q, want b", peek.Value)
	}
	if cur := ts.Current(); cur.Value != "a" {
		t.Fatalf("Look mutated current to %q", cur.Value)
	}
	ts.Push(Token{Lineno: 1, Kind: TokenName, Value: "z"})
	if cur := ts.Current(); cur.Value != "a" {
		t.Fatalf("Push must not change Current; got %q", cur.Value)
	}
	ts.Next()
	if cur := ts.Current(); cur.Value != "z" {
		t.Fatalf("after Next, expected pushed token, got %q", cur.Value)
	}
}

// ---------------------------------------------------------- delimiter tests

// Mirrors test_balancing — custom delimiters {% %} ${ } work end-to-end.
func TestCustomDelimiters(t *testing.T) {
	o := DefaultOptions()
	o.VariableStart = "${"
	o.VariableEnd = "}"
	src := "${ x }"
	toks, err := valuesOpts(t, src, o)
	if err != nil {
		t.Fatal(err)
	}
	want := []TokenKind{TokenVariableBegin, TokenName, TokenVariableEnd}
	gotK := make([]TokenKind, 0, len(toks))
	for _, tk := range toks {
		gotK = append(gotK, tk.Kind)
	}
	if !equal(gotK, want) {
		t.Fatalf("kinds = %v, want %v", gotK, want)
	}
}

// Mirrors test_comments using `<!--` / `-->` markers.
func TestCustomCommentDelimiters(t *testing.T) {
	o := DefaultOptions()
	o.CommentStart = "<!--"
	o.CommentEnd = "-->"
	src := "before <!-- this is a comment --> after"
	toks, err := valuesOpts(t, src, o)
	if err != nil {
		t.Fatal(err)
	}
	if len(toks) != 2 {
		t.Fatalf("expected 2 data tokens (comment dropped), got %d: %#v", len(toks), toks)
	}
	if toks[0].Value != "before " || toks[1].Value != " after" {
		t.Fatalf("data tokens wrong: %#v", toks)
	}
}

// ------------------------------------------------------------- raw blocks

// Mirrors test_raw1.
func TestRawBlockPassthrough(t *testing.T) {
	src := `{% raw %}foo{% endraw %}|{%raw%}{{ bar }}|{% baz %}{%       endraw    %}`
	toks := values(t, src)
	// We expect one TokenData = "foo", one TokenData = "|", one TokenData = "{{ bar }}|{% baz %}"
	var datas []string
	for _, tk := range toks {
		if tk.Kind == TokenData {
			datas = append(datas, tk.Value)
		}
	}
	joined := strings.Join(datas, "")
	want := "foo|{{ bar }}|{% baz %}"
	if joined != want {
		t.Fatalf("raw passthrough = %q, want %q", joined, want)
	}
}

// Mirrors test_raw2 — dash modifiers around the raw block.
func TestRawWithDashes(t *testing.T) {
	src := "1  {%- raw -%}   2   {%- endraw -%}   3"
	toks := values(t, src)
	var b strings.Builder
	for _, tk := range toks {
		if tk.Kind == TokenData {
			b.WriteString(tk.Value)
		}
	}
	if got := b.String(); got != "123" {
		t.Fatalf("raw dash-strip = %q, want %q", got, "123")
	}
}

// -------------------------------------------------------------- operators

// Mirrors test_operators — every entry in the operators table tokenizes
// to its mapped kind when wrapped in {{ ... }}.
func TestEveryOperatorMaps(t *testing.T) {
	for sym, want := range operators {
		// Skip brace/bracket/paren — they need balanced pairs to lex inside {{ }}.
		switch sym {
		case "(", ")", "[", "]", "{", "}":
			continue
		}
		toks := values(t, "{{ "+sym+" }}")
		// Find the operator-kind token (skip variable_begin and any padding).
		var found TokenKind
		for _, tk := range toks {
			if tk.Kind == want {
				found = tk.Kind
				break
			}
		}
		if found != want {
			t.Errorf("operator %q -> got %v, want %v", sym, found, want)
		}
	}
}

// ------------------------------------------------------------- newlines

// Mirrors test_normalizing across \n / \r\n / \r outputs.
func TestNewlineNormalizing(t *testing.T) {
	for _, seq := range []string{"\n", "\r\n", "\r"} {
		o := DefaultOptions()
		o.NewlineSequence = seq
		toks, err := valuesOpts(t, "1\n2\r\n3\n4\n", o)
		if err != nil {
			t.Fatal(err)
		}
		var b strings.Builder
		for _, tk := range toks {
			if tk.Kind == TokenData {
				b.WriteString(tk.Value)
			}
		}
		got := strings.ReplaceAll(b.String(), seq, "X")
		// keep_trailing_newline=false drops the final newline.
		if got != "1X2X3X4" {
			t.Errorf("seq=%q normalized = %q, want %q", seq, got, "1X2X3X4")
		}
	}
}

// Mirrors test_trailing_newline.
func TestTrailingNewline(t *testing.T) {
	cases := []struct {
		keep   bool
		src    string
		wanted string
	}{
		{true, "", ""},
		{false, "", ""},
		{true, "no\nnewline", "no\nnewline"},
		{false, "with\nnewline\n", "with\nnewline"},
		{true, "with\nnewline\n", "with\nnewline\n"},
		{false, "with\nseveral\n\n\n", "with\nseveral\n\n"},
		{true, "with\nseveral\n\n\n", "with\nseveral\n\n\n"},
	}
	for _, c := range cases {
		o := DefaultOptions()
		o.KeepTrailingNewline = c.keep
		toks, err := valuesOpts(t, c.src, o)
		if err != nil {
			t.Fatal(err)
		}
		var b strings.Builder
		for _, tk := range toks {
			if tk.Kind == TokenData {
				b.WriteString(tk.Value)
			}
		}
		if b.String() != c.wanted {
			t.Errorf("keep=%v src=%q -> %q, want %q", c.keep, c.src, b.String(), c.wanted)
		}
	}
}

// -------------------------------------------------------- numeric literals

func TestIntegerLiterals(t *testing.T) {
	cases := map[string]string{
		"0":         "0",
		"42":        "42",
		"1_000":     "1000",
		"0b1010":    "10",
		"0o17":      "15",
		"0xff":      "255",
		"0xFF":      "255",
		"0b1_0_1_0": "10",
	}
	for src, want := range cases {
		toks := values(t, "{{ "+src+" }}")
		found := false
		for _, tk := range toks {
			if tk.Kind == TokenInteger {
				if tk.Value != want {
					t.Errorf("integer %q -> %q, want %q", src, tk.Value, want)
				}
				found = true
				break
			}
		}
		if !found {
			t.Errorf("integer not tokenized for %q", src)
		}
	}
}

func TestFloatLiterals(t *testing.T) {
	cases := []string{"3.14", "1e5", "1.5e-2", "1_000.5"}
	for _, src := range cases {
		toks := values(t, "{{ "+src+" }}")
		found := false
		for _, tk := range toks {
			if tk.Kind == TokenFloat {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("float not tokenized for %q: %#v", src, toks)
		}
	}
}

// ------------------------------------------------------- string literals

func TestStringEscapeSequences(t *testing.T) {
	// Mirrors test_string_escapes minus the \N{NAME} case (deliberate divergence — see docs/divergences.md).
	cases := map[string]string{
		`'a\nb'`:       "a\nb",
		`'\t'`:         "\t",
		`'\x41'`:       "A",
		`'♨'`:          "♨",
		`'\U00002668'`: "♨",
		`'\\'`:         "\\",
		`"\""`:         `"`,
		`'\''`:         `'`,
	}
	for src, want := range cases {
		toks := values(t, "{{ "+src+" }}")
		found := false
		for _, tk := range toks {
			if tk.Kind == TokenString {
				if tk.Value != want {
					t.Errorf("string %s decoded to %q, want %q", src, tk.Value, want)
				}
				found = true
				break
			}
		}
		if !found {
			t.Errorf("no string token for %s", src)
		}
	}
}

func TestStringNamedEscapeRejected(t *testing.T) {
	// We deliberately do not support \N{NAME}.
	_, err := valuesOpts(t, `{{ "\N{HOT SPRINGS}" }}`, DefaultOptions())
	if err == nil {
		t.Fatal("expected error for \\N{NAME}, got nil")
	}
}

// ----------------------------------------------------------- identifiers

// Mirrors test_name parametrized cases. We restrict to the lex-level
// predicate: a name token is emitted iff the input begins with an
// identifier-start character. Cases like "1a" or "a-" are valid lex
// streams (integer + name, name + sub) and are rejected only at parse
// time, so they are not tested here.
func TestIdentifierValidity(t *testing.T) {
	valid := []string{"foo", "föö", "き", "_"}
	for _, name := range valid {
		toks := values(t, "{{ "+name+" }}")
		var got string
		for _, tk := range toks {
			if tk.Kind == TokenName {
				got = tk.Value
				break
			}
		}
		if got != name {
			t.Errorf("name %q -> token value %q", name, got)
		}
	}
	// isIdentifier directly: rejects empty and pure-non-letter starts.
	if isIdentifier("") {
		t.Error("isIdentifier(\"\") should be false")
	}
	if isIdentifier("1abc") {
		t.Error("isIdentifier(\"1abc\") should be false (digit start)")
	}
	if !isIdentifier("a_b") {
		t.Error("isIdentifier(\"a_b\") should be true")
	}
}

// -------------------------------------------------------------- balancing

func TestUnbalancedBracketError(t *testing.T) {
	_, err := valuesOpts(t, `{{ (1, 2) ] }}`, DefaultOptions())
	if err == nil {
		t.Fatal("expected error for unbalanced ]")
	}
}

func TestNestingDepthCap(t *testing.T) {
	o := DefaultOptions()
	o.MaxNestingDepth = 4
	src := "{{ ((((( 1 ))))) }}" // 5 opens
	_, err := valuesOpts(t, src, o)
	if err == nil {
		t.Fatal("expected nesting-depth error")
	}
}

// ------------------------------------------------------------- whitespace

// Mirrors test_lstrip — lstrip_blocks strips the leading spaces of a
// whitespace-only line that ends with a block tag. Empty data tokens are
// dropped on the wrap pass (ignoreIfEmpty), so the leading "    " becomes
// no token at all rather than an empty-string data token.
func TestLstripBlocks(t *testing.T) {
	o := DefaultOptions()
	o.LstripBlocks = true
	toks, err := valuesOpts(t, "    {% if x %}\nhi\n    {% endif %}", o)
	if err != nil {
		t.Fatal(err)
	}
	for _, tk := range toks {
		if tk.Kind == TokenData {
			if strings.HasPrefix(tk.Value, "    ") {
				t.Fatalf("data %q still has 4-space lstrip-eligible prefix", tk.Value)
			}
		}
	}
	// And: with lstrip_blocks OFF the prefix should be preserved.
	o2 := DefaultOptions()
	toks2, err := valuesOpts(t, "    {% if x %}", o2)
	if err != nil {
		t.Fatal(err)
	}
	if toks2[0].Kind != TokenData || toks2[0].Value != "    " {
		t.Fatalf("without lstrip, leading data should be %q, got %#v", "    ", toks2[0])
	}
}

// Mirrors trim_blocks — newline directly after a block tag is dropped.
func TestTrimBlocks(t *testing.T) {
	o := DefaultOptions()
	o.TrimBlocks = true
	toks, err := valuesOpts(t, "{% if x %}\nhi", o)
	if err != nil {
		t.Fatal(err)
	}
	// Find the data token after the if.
	var data string
	for _, tk := range toks {
		if tk.Kind == TokenData {
			data = tk.Value
		}
	}
	if data != "hi" {
		t.Fatalf("trim_blocks data = %q, want %q", data, "hi")
	}
}

// ---------------------------------------------------------------- helpers

func equal[T comparable](a, b []T) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
