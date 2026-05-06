// Package lexer turns template source into a stream of [Token]s. The
// behavior mirrors Jinja2's lexer.py.
package lexer

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	gjerrors "github.com/jryberg/gojinja/pkg/errors"
)

// stateName is the lexer's per-position state. Mirrors the keys of
// `Lexer.rules` in Python (`root`, `comment_begin`, `block_begin`,
// `variable_begin`, `raw_begin`, `linestatement_begin`,
// `linecomment_begin`).
type stateName int

const (
	stateRoot stateName = iota
	stateCommentBegin
	stateBlockBegin
	stateVariableBegin
	stateRawBegin
	stateLineStatementBegin
	stateLineCommentBegin
)

// Lexer turns templates into tokens for a given [Options]. Construct with
// [New] (which compiles the per-environment regex rules once); the
// resulting Lexer is safe for concurrent use.
type Lexer struct {
	opts Options

	// rules are the precompiled regexes for each lexer state.
	rootData       *regexp.Regexp // big alternation that matches data + tag-begin
	rootTrailing   *regexp.Regexp // ".+" fallback when no tag is left
	commentEnd     *regexp.Regexp
	blockEnd       *regexp.Regexp
	variableEnd    *regexp.Regexp
	rawEnd         *regexp.Regexp
	lineStmtEnd    *regexp.Regexp
	lineCommentEnd *regexp.Regexp

	// tag-rule regexes, shared across block/variable/linestatement bodies.
	tagWhitespace *regexp.Regexp
	tagFloat      *regexp.Regexp
	tagInteger    *regexp.Regexp
	tagName       *regexp.Regexp
	tagString     *regexp.Regexp
	tagOperator   *regexp.Regexp

	// indices into rootData's named groups, in tag-rules order.
	rootGroupKinds []TokenKind // group i+1 → kind (0 = the leading data text)
}

// New compiles a Lexer for opts. Returns an error if any of the user-supplied
// markers can't be turned into a valid regex (extremely rare).
func New(opts Options) (*Lexer, error) {
	o := opts.WithDefaults()
	l := &Lexer{opts: o}

	blkS := regexp.QuoteMeta(o.BlockStart)
	blkE := regexp.QuoteMeta(o.BlockEnd)
	varS := regexp.QuoteMeta(o.VariableStart)
	varE := regexp.QuoteMeta(o.VariableEnd)
	cmtS := regexp.QuoteMeta(o.CommentStart)
	cmtE := regexp.QuoteMeta(o.CommentEnd)

	// Block suffix when trim_blocks is enabled — eat one optional newline.
	blockSuffix := ""
	if o.TrimBlocks {
		blockSuffix = `\n?`
	}

	// Build the root alternation. Order matters: comment_begin first
	// (longest delimiter wins ties), then block, then variable, then the
	// optional line-statement and line-comment. We also splice in the
	// `raw` directive recognition because raw_begin transitions out of
	// the normal flow.
	candidates := []rootRule{
		{"comment_begin", TokenCommentBegin, fmt.Sprintf("(?P<comment_begin>%s(\\-|\\+|))", cmtS)},
		{"block_begin", TokenBlockBegin, fmt.Sprintf("(?P<block_begin>%s(\\-|\\+|))", blkS)},
		{"variable_begin", TokenVariableBegin, fmt.Sprintf("(?P<variable_begin>%s(\\-|\\+|))", varS)},
	}
	if o.LineStatementPrefix != "" {
		lsp := regexp.QuoteMeta(o.LineStatementPrefix)
		candidates = append(candidates, rootRule{
			"linestatement_begin", TokenLineStatementBegin,
			fmt.Sprintf(`(?P<linestatement_begin>(?m:^)[ \t\v]*%s(\-|\+|))`, lsp),
		})
	}
	if o.LineCommentPrefix != "" {
		lcp := regexp.QuoteMeta(o.LineCommentPrefix)
		// Python uses `(?:^|(?<=\S))` (lookbehind). Go's regexp lacks
		// lookbehind; we emulate by matching `(?:^|[^\n\r])` and then
		// adjusting the start in code so we don't consume the leading
		// non-whitespace byte. We *do* match it but then back-off by
		// one byte at emit time — see emitRoot.
		candidates = append(candidates, rootRule{
			"linecomment_begin", TokenLineCommentBegin,
			fmt.Sprintf(`(?P<linecomment_begin>(?m:^|[^\S\r\n])[^\S\r\n]*%s(\-|\+|))`, lcp),
		})
	}

	// Also recognize {% raw %} as a special root rule that skips straight
	// to raw_begin state.
	rawBegin := fmt.Sprintf(`(?P<raw_begin>%s(\-|\+|)\s*raw\s*(?:\-%s\s*|%s))`, blkS, blkE, blkE)

	// Sort candidates by descending pattern length proxy: longer delimiters
	// must come first in the alternation. We ensure comment/block/variable
	// are ordered by their start-marker length.
	candidates = sortByDelimLen(candidates, o)

	parts := []string{rawBegin}
	for _, c := range candidates {
		parts = append(parts, c.pattern)
	}
	rootData := fmt.Sprintf(`(?s)(.*?)(?:%s)`, strings.Join(parts, "|"))
	re, err := regexp.Compile(rootData)
	if err != nil {
		return nil, fmt.Errorf("lexer: compiling root rule: %w", err)
	}
	l.rootData = re
	l.rootGroupKinds = []TokenKind{TokenRawBegin}
	for _, c := range candidates {
		l.rootGroupKinds = append(l.rootGroupKinds, c.kind)
	}

	l.rootTrailing = regexp.MustCompile(`(?s).+`)

	l.commentEnd = regexp.MustCompile(fmt.Sprintf(
		`(?s)(.*?)((?:\+%s|\-%s\s*|%s%s))`, cmtE, cmtE, cmtE, blockSuffix))
	l.blockEnd = regexp.MustCompile(fmt.Sprintf(
		`(?s)(?:\+%s|\-%s\s*|%s%s)`, blkE, blkE, blkE, blockSuffix))
	l.variableEnd = regexp.MustCompile(fmt.Sprintf(
		`(?s)\-%s\s*|%s`, varE, varE))
	l.rawEnd = regexp.MustCompile(fmt.Sprintf(
		`(?s)(.*?)((?:%s(\-|\+|))\s*endraw\s*(?:\+%s|\-%s\s*|%s%s))`,
		blkS, blkE, blkE, blkE, blockSuffix))
	l.lineStmtEnd = regexp.MustCompile(`(?s)\s*(\n|$)`)
	l.lineCommentEnd = regexp.MustCompile(`(?s)(.*?)()(?:\n|$)`)

	// Tag-body rules (used inside block_begin / variable_begin /
	// linestatement_begin states).
	l.tagWhitespace = regexp.MustCompile(`\s+`)
	// integer (decimal/binary/octal/hex with optional `_` separators)
	l.tagInteger = regexp.MustCompile(
		`(?i)(?:0b(?:_?[0-1])+|0o(?:_?[0-7])+|0x(?:_?[0-9a-f])+|[1-9](?:_?[0-9])*|0(?:_?0)*)`)
	// float — the `(?<!\.)` lookbehind from Python is enforced in code.
	l.tagFloat = regexp.MustCompile(
		`(?i)(?:[0-9]+_)*[0-9]+(?:(?:\.(?:[0-9]+_)*[0-9]+)?e[+\-]?(?:[0-9]+_)*[0-9]+|\.(?:[0-9]+_)*[0-9]+)`)
	// String literal — single OR double quoted, with backslash escapes.
	l.tagString = regexp.MustCompile(`(?s)(?:'(?:[^'\\]|\\.)*'|"(?:[^"\\]|\\.)*")`)
	// Name pattern — we accept ASCII identifiers in the regex and then
	// extend the match with a hand-rolled walk if the next bytes form
	// non-ASCII identifier continuation. This avoids having to ship
	// Python's giant character class.
	l.tagName = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)
	// Operator: longest-match-wins. We sort by descending length so `**`
	// beats `*`, etc.
	l.tagOperator = regexp.MustCompile(buildOperatorPattern())

	return l, nil
}

// sortByDelimLen orders the rootRule candidates by descending length of
// their starting marker. Comment/block/variable use o.{Comment,Block,Variable}Start;
// linestatement/linecomment use their prefixes. Stable for equal lengths.
func sortByDelimLen(in []rootRule, o Options) []rootRule {
	delim := func(r rootRule) string {
		switch r.kind {
		case TokenCommentBegin:
			return o.CommentStart
		case TokenBlockBegin:
			return o.BlockStart
		case TokenVariableBegin:
			return o.VariableStart
		case TokenLineStatementBegin:
			return o.LineStatementPrefix
		case TokenLineCommentBegin:
			return o.LineCommentPrefix
		}
		return ""
	}
	// Stable insertion sort.
	out := append([]rootRule(nil), in...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && len(delim(out[j-1])) < len(delim(out[j])); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// rootRule is a single tag-begin alternative within the root regex.
type rootRule struct {
	groupName string
	kind      TokenKind
	pattern   string
}

// buildOperatorPattern returns a regex alternation matching every entry in
// `operators`, longest first.
func buildOperatorPattern() string {
	keys := make([]string, 0, len(operators))
	for k := range operators {
		keys = append(keys, k)
	}
	// Sort by descending length, then lexically (deterministic).
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0; j-- {
			a, b := keys[j-1], keys[j]
			if len(a) > len(b) || (len(a) == len(b) && a <= b) {
				break
			}
			keys[j-1], keys[j] = b, a
		}
	}
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('|')
		}
		b.WriteString(regexp.QuoteMeta(k))
	}
	return b.String()
}

// Tokenize lexes source and returns a [TokenStream] ready for a parser.
// name and filename are passed through into errors and propagated to the
// stream.
func (l *Lexer) Tokenize(source, name, filename string) (*TokenStream, error) {
	raw, err := l.tokenize(source, name, filename)
	if err != nil {
		return nil, err
	}
	wrapped, err := l.wrap(raw, name, filename)
	if err != nil {
		return nil, err
	}
	return NewTokenStream(wrapped, name, filename), nil
}

// rawToken is the inter-stage representation produced by the state machine
// before [wrap] post-processes string/integer/float values.
type rawToken struct {
	Lineno int
	Kind   TokenKind
	Value  string
}

// normalizeNewlines replaces every \r\n / \r with \n. Mirrors Python
// `newline_re.split(source)[::2]` then `"\n".join(...)`.
func normalizeNewlines(source string) string {
	if !strings.ContainsAny(source, "\r") {
		return source
	}
	source = strings.ReplaceAll(source, "\r\n", "\n")
	source = strings.ReplaceAll(source, "\r", "\n")
	return source
}

// tokenize is the state-machine driver. Returns rawTokens (one per match
// emit). Mirrors lexer.py:tokeniter.
func (l *Lexer) tokenize(source, name, filename string) ([]rawToken, error) {
	source = normalizeNewlines(source)
	if !l.opts.KeepTrailingNewline && strings.HasSuffix(source, "\n") {
		source = source[:len(source)-1]
	}

	out := make([]rawToken, 0, 64)
	stack := []stateName{stateRoot}
	balancingStack := make([]byte, 0, 16) // bytes: ')', ']', '}'
	pos := 0
	lineno := 1
	newlinesStripped := 0
	lineStarting := true

	for pos < len(source) {
		st := stack[len(stack)-1]
		var (
			advanced int
			err      error
		)
		switch st {
		case stateRoot:
			advanced, err = l.lexRoot(source, &pos, &lineno, &out, &stack, &newlinesStripped, &lineStarting)
		case stateCommentBegin:
			advanced, err = l.lexCommentBody(source, pos, lineno, &out, &stack, name, filename)
			if err == nil {
				lineno += countNewlines(source[pos : pos+advanced])
				pos += advanced
				lineStarting = pos > 0 && source[pos-1] == '\n'
			}
		case stateBlockBegin, stateVariableBegin, stateLineStatementBegin:
			advanced, err = l.lexTagBody(st, source, pos, lineno, &out, &stack, &balancingStack, name, filename)
			if err == nil {
				lineno += countNewlines(source[pos : pos+advanced])
				pos += advanced
				lineStarting = advanced > 0 && source[pos-1] == '\n'
			}
		case stateRawBegin:
			advanced, err = l.lexRawBody(source, pos, lineno, &out, &stack, name, filename)
			if err == nil {
				lineno += countNewlines(source[pos : pos+advanced])
				pos += advanced
				lineStarting = advanced > 0 && source[pos-1] == '\n'
			}
		case stateLineCommentBegin:
			advanced, err = l.lexLineComment(source, pos, lineno, &out, &stack)
			if err == nil {
				lineno += countNewlines(source[pos : pos+advanced])
				pos += advanced
				lineStarting = advanced > 0 && source[pos-1] == '\n'
			}
		}
		if err != nil {
			return nil, err
		}
		if advanced == 0 {
			return nil, gjerrors.NewTemplateSyntaxError(
				fmt.Sprintf("unexpected char %q at %d", source[pos], pos),
				lineno, name, filename)
		}
	}
	// EOF reached. Anything still open is an error — except line
	// statement / line comment, which auto-close at end-of-input.
	for len(stack) > 1 {
		switch stack[len(stack)-1] {
		case stateLineStatementBegin:
			out = append(out, rawToken{Lineno: lineno, Kind: TokenLineStatementEnd, Value: ""})
			stack = stack[:len(stack)-1]
		case stateLineCommentBegin:
			out = append(out, rawToken{Lineno: lineno, Kind: TokenLineCommentEnd, Value: ""})
			stack = stack[:len(stack)-1]
		case stateBlockBegin:
			return nil, gjerrors.NewTemplateSyntaxError("unexpected end of template, expected end of statement block", lineno, name, filename)
		case stateVariableBegin:
			return nil, gjerrors.NewTemplateSyntaxError("unexpected end of template, expected end of print statement", lineno, name, filename)
		case stateCommentBegin:
			return nil, gjerrors.NewTemplateSyntaxError("Missing end of comment tag", lineno, name, filename)
		case stateRawBegin:
			return nil, gjerrors.NewTemplateSyntaxError("Missing end of raw directive", lineno, name, filename)
		default:
			return nil, gjerrors.NewTemplateSyntaxError("unexpected end of template", lineno, name, filename)
		}
	}
	return out, nil
}

// countNewlines counts '\n' bytes in s.
func countNewlines(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			n++
		}
	}
	return n
}

// lexRoot handles the root state: emit data + recognize a tag-begin.
//
// The match groups are: group 1 = the leading TEXT, then for each
// candidate alternative, a (text, sign) pair. We pick whichever named
// group matched.
func (l *Lexer) lexRoot(source string, pos, lineno *int, out *[]rawToken, stack *[]stateName, newlinesStripped *int, lineStarting *bool) (int, error) {
	loc := l.rootData.FindStringSubmatchIndex(source[*pos:])
	if loc == nil {
		// No more tag begins — emit the rest as data.
		text := source[*pos:]
		if text == "" {
			return 0, nil
		}
		*out = append(*out, rawToken{Lineno: *lineno, Kind: TokenData, Value: text})
		*lineno += countNewlines(text)
		consumed := len(text)
		*pos += consumed
		return consumed, nil
	}

	// loc[0..1] = full match span; loc[2..3] = group 1 (TEXT).
	text := source[*pos+loc[2] : *pos+loc[3]]

	// Find which named group fired (the tag-begin candidate).
	names := l.rootData.SubexpNames()
	var (
		matchedKind TokenKind = -1
		signGroup   int       // group of the (\-|\+|) suffix; raw_begin uses an inner sign group
	)
	for gi := 1; gi < len(names); gi++ {
		if names[gi] == "" {
			continue
		}
		s, e := loc[2*gi], loc[2*gi+1]
		if s < 0 {
			continue
		}
		_ = e
		switch names[gi] {
		case "raw_begin":
			matchedKind = TokenRawBegin
			signGroup = gi + 1
		case "comment_begin":
			matchedKind = TokenCommentBegin
			signGroup = gi + 1
		case "block_begin":
			matchedKind = TokenBlockBegin
			signGroup = gi + 1
		case "variable_begin":
			matchedKind = TokenVariableBegin
			signGroup = gi + 1
		case "linestatement_begin":
			matchedKind = TokenLineStatementBegin
			signGroup = gi + 1
		case "linecomment_begin":
			matchedKind = TokenLineCommentBegin
			signGroup = gi + 1
		}
		if matchedKind != -1 {
			break
		}
	}
	if matchedKind == -1 {
		return 0, fmt.Errorf("lexer: root regex matched but no named group fired")
	}

	// Pull the whitespace-control sign for this candidate.
	sign := ""
	if signGroup > 0 && 2*signGroup+1 <= len(loc) {
		s, e := loc[2*signGroup], loc[2*signGroup+1]
		if s >= 0 {
			sign = source[*pos+s : *pos+e]
		}
	}

	// Apply lstrip / sign rules to TEXT before emitting.
	text = l.applyWhitespaceControl(text, sign, matchedKind, newlinesStripped, *lineStarting)
	if text != "" || !ignoreIfEmpty[TokenData] {
		*out = append(*out, rawToken{Lineno: *lineno, Kind: TokenData, Value: text})
	}
	*lineno += countNewlines(text) + *newlinesStripped
	*newlinesStripped = 0

	// Emit the tag-begin token. The full delimiter span is loc[2*<signGroup-1>..]
	// but we don't need the body — only the kind matters. Consume up to loc[1].
	beginText := source[*pos+loc[3] : *pos+loc[1]]
	if matchedKind != TokenLineCommentBegin {
		*out = append(*out, rawToken{Lineno: *lineno, Kind: matchedKind, Value: beginText})
	} else {
		// Line comment begin: don't emit (it's in ignoredKinds anyway)
		*out = append(*out, rawToken{Lineno: *lineno, Kind: matchedKind, Value: beginText})
	}
	*lineno += countNewlines(beginText)

	// Push the new state.
	switch matchedKind {
	case TokenRawBegin:
		*stack = append(*stack, stateRawBegin)
	case TokenCommentBegin:
		*stack = append(*stack, stateCommentBegin)
	case TokenBlockBegin:
		*stack = append(*stack, stateBlockBegin)
	case TokenVariableBegin:
		*stack = append(*stack, stateVariableBegin)
	case TokenLineStatementBegin:
		*stack = append(*stack, stateLineStatementBegin)
	case TokenLineCommentBegin:
		*stack = append(*stack, stateLineCommentBegin)
	}

	consumed := loc[1]
	*pos += consumed
	*lineStarting = consumed > 0 && source[*pos-1] == '\n'
	return consumed, nil
}

// applyWhitespaceControl trims TEXT according to the sign modifier and
// (when sign is "") the lstrip_blocks setting. Mirrors lexer.py's
// OptionalLStrip handling.
func (l *Lexer) applyWhitespaceControl(text, sign string, kind TokenKind, newlinesStripped *int, lineStarting bool) string {
	switch sign {
	case "-":
		stripped := strings.TrimRightFunc(text, isASCIISpace)
		*newlinesStripped += countNewlines(text[len(stripped):])
		return stripped
	case "+":
		return text
	}
	if !l.opts.LstripBlocks || kind == TokenVariableBegin {
		return text
	}
	// lstrip_blocks active and no explicit sign. Strip the trailing
	// whitespace from text starting at the last newline if everything from
	// the newline onward is whitespace only.
	lpos := strings.LastIndexByte(text, '\n') + 1
	if lpos == 0 && !lineStarting {
		return text
	}
	tail := text[lpos:]
	for i := 0; i < len(tail); i++ {
		if !isASCIISpace(rune(tail[i])) {
			return text
		}
	}
	return text[:lpos]
}

// isASCIISpace mirrors Python's `\s` for ASCII characters.
func isASCIISpace(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\r', '\v', '\f':
		return true
	}
	return false
}

// lexCommentBody handles state `comment_begin`: consume up to comment_end,
// emit (TokenComment, TokenCommentEnd), pop state.
func (l *Lexer) lexCommentBody(source string, pos, lineno int, out *[]rawToken, stack *[]stateName, name, filename string) (int, error) {
	loc := l.commentEnd.FindStringSubmatchIndex(source[pos:])
	if loc == nil {
		return 0, gjerrors.NewTemplateSyntaxError("Missing end of comment tag", lineno, name, filename)
	}
	body := source[pos+loc[2] : pos+loc[3]]
	endText := source[pos+loc[4] : pos+loc[5]]
	if body != "" {
		*out = append(*out, rawToken{Lineno: lineno, Kind: TokenComment, Value: body})
	}
	*out = append(*out, rawToken{Lineno: lineno + countNewlines(body), Kind: TokenCommentEnd, Value: endText})
	*stack = (*stack)[:len(*stack)-1]
	return loc[1], nil
}

// lexLineComment consumes from current pos to end-of-line, emitting
// (TokenLineComment, TokenLineCommentEnd), then pops state.
func (l *Lexer) lexLineComment(source string, pos, lineno int, out *[]rawToken, stack *[]stateName) (int, error) {
	rest := source[pos:]
	end := strings.IndexByte(rest, '\n')
	body := rest
	consumed := len(rest)
	if end >= 0 {
		body = rest[:end]
		consumed = end // do not consume the newline; root state will pick it up as data
	}
	if body != "" {
		*out = append(*out, rawToken{Lineno: lineno, Kind: TokenLineComment, Value: body})
	}
	*out = append(*out, rawToken{Lineno: lineno, Kind: TokenLineCommentEnd, Value: ""})
	*stack = (*stack)[:len(*stack)-1]
	return consumed, nil
}

// lexRawBody handles state `raw_begin`: consume up to {% endraw %}, emit
// data + raw_end, pop state. The leading dash on the closing tag (e.g.
// `{%- endraw %}`) strips trailing whitespace from the body — same
// OptionalLStrip semantics the root state applies before tag-begin tokens.
func (l *Lexer) lexRawBody(source string, pos, lineno int, out *[]rawToken, stack *[]stateName, name, filename string) (int, error) {
	loc := l.rawEnd.FindStringSubmatchIndex(source[pos:])
	if loc == nil {
		return 0, gjerrors.NewTemplateSyntaxError("Missing end of raw directive", lineno, name, filename)
	}
	text := source[pos+loc[2] : pos+loc[3]]
	endText := source[pos+loc[4] : pos+loc[5]]
	// Group 3 carries the `-`/`+`/empty modifier on the `{%- endraw` opener.
	sign := ""
	if 7 < len(loc) && loc[6] >= 0 {
		sign = source[pos+loc[6] : pos+loc[7]]
	}
	if sign == "-" {
		text = strings.TrimRightFunc(text, isASCIISpace)
	}
	if text != "" {
		*out = append(*out, rawToken{Lineno: lineno, Kind: TokenData, Value: text})
	}
	*out = append(*out, rawToken{Lineno: lineno + countNewlines(text), Kind: TokenRawEnd, Value: endText})
	*stack = (*stack)[:len(*stack)-1]
	return loc[1], nil
}

// lexTagBody handles the body of block_begin, variable_begin, and
// linestatement_begin states. Each tries the closer first; if not closed
// (or closer is masked by an unbalanced bracket), iterates the tag rules.
func (l *Lexer) lexTagBody(st stateName, source string, pos, lineno int, out *[]rawToken, stack *[]stateName, balStack *[]byte, name, filename string) (int, error) {
	rest := source[pos:]
	// 1. Try the appropriate closer first (unless brackets are unbalanced).
	if len(*balStack) == 0 {
		switch st {
		case stateBlockBegin:
			if loc := l.blockEnd.FindStringIndex(rest); loc != nil && loc[0] == 0 {
				*out = append(*out, rawToken{Lineno: lineno, Kind: TokenBlockEnd, Value: rest[loc[0]:loc[1]]})
				*stack = (*stack)[:len(*stack)-1]
				return loc[1], nil
			}
		case stateVariableBegin:
			if loc := l.variableEnd.FindStringIndex(rest); loc != nil && loc[0] == 0 {
				*out = append(*out, rawToken{Lineno: lineno, Kind: TokenVariableEnd, Value: rest[loc[0]:loc[1]]})
				*stack = (*stack)[:len(*stack)-1]
				return loc[1], nil
			}
		case stateLineStatementBegin:
			if loc := l.lineStmtEnd.FindStringIndex(rest); loc != nil && loc[0] == 0 {
				*out = append(*out, rawToken{Lineno: lineno, Kind: TokenLineStatementEnd, Value: rest[loc[0]:loc[1]]})
				*stack = (*stack)[:len(*stack)-1]
				return loc[1], nil
			}
		}
	}
	// 2. Run tag rules in order.
	if loc := l.tagWhitespace.FindStringIndex(rest); loc != nil && loc[0] == 0 {
		val := rest[:loc[1]]
		if val != "" {
			*out = append(*out, rawToken{Lineno: lineno, Kind: TokenWhitespace, Value: val})
		}
		return loc[1], nil
	}
	// Float — must satisfy the (?<!\.) lookbehind by checking the byte before pos.
	if pos == 0 || source[pos-1] != '.' {
		if loc := l.tagFloat.FindStringIndex(rest); loc != nil && loc[0] == 0 {
			*out = append(*out, rawToken{Lineno: lineno, Kind: TokenFloat, Value: rest[:loc[1]]})
			return loc[1], nil
		}
	}
	if loc := l.tagInteger.FindStringIndex(rest); loc != nil && loc[0] == 0 {
		// Avoid eating the integer half of e.g. `1.5` — the float rule above
		// would have matched first.
		*out = append(*out, rawToken{Lineno: lineno, Kind: TokenInteger, Value: rest[:loc[1]]})
		return loc[1], nil
	}
	// Identifier — start with the ASCII regex, then extend across non-ASCII.
	if loc := l.tagName.FindStringIndex(rest); loc != nil && loc[0] == 0 {
		end := loc[1]
		for end < len(rest) {
			r, size := decodeRune(rest[end:])
			if !isIdentContinue(r) {
				break
			}
			end += size
		}
		*out = append(*out, rawToken{Lineno: lineno, Kind: TokenName, Value: rest[:end]})
		return end, nil
	}
	// Bare identifier-start that isn't ASCII (e.g. a unicode letter).
	if r, size := decodeRune(rest); size > 0 && isIdentStart(r) {
		end := size
		for end < len(rest) {
			r2, sz := decodeRune(rest[end:])
			if !isIdentContinue(r2) {
				break
			}
			end += sz
		}
		*out = append(*out, rawToken{Lineno: lineno, Kind: TokenName, Value: rest[:end]})
		return end, nil
	}
	if loc := l.tagString.FindStringIndex(rest); loc != nil && loc[0] == 0 {
		*out = append(*out, rawToken{Lineno: lineno, Kind: TokenString, Value: rest[:loc[1]]})
		return loc[1], nil
	}
	if loc := l.tagOperator.FindStringIndex(rest); loc != nil && loc[0] == 0 {
		val := rest[:loc[1]]
		// Maintain bracket balance.
		switch val {
		case "(":
			if len(*balStack) >= l.opts.MaxNestingDepth {
				return 0, gjerrors.NewTemplateSyntaxError("expression nested too deeply", lineno, name, filename)
			}
			*balStack = append(*balStack, ')')
		case "[":
			if len(*balStack) >= l.opts.MaxNestingDepth {
				return 0, gjerrors.NewTemplateSyntaxError("expression nested too deeply", lineno, name, filename)
			}
			*balStack = append(*balStack, ']')
		case "{":
			if len(*balStack) >= l.opts.MaxNestingDepth {
				return 0, gjerrors.NewTemplateSyntaxError("expression nested too deeply", lineno, name, filename)
			}
			*balStack = append(*balStack, '}')
		case ")", "]", "}":
			if len(*balStack) == 0 {
				return 0, gjerrors.NewTemplateSyntaxError(fmt.Sprintf("unexpected '%s'", val), lineno, name, filename)
			}
			expected := (*balStack)[len(*balStack)-1]
			*balStack = (*balStack)[:len(*balStack)-1]
			if string(expected) != val {
				return 0, gjerrors.NewTemplateSyntaxError(
					fmt.Sprintf("unexpected %q, expected %q", val, string(expected)), lineno, name, filename)
			}
		}
		*out = append(*out, rawToken{Lineno: lineno, Kind: TokenOperator, Value: val})
		return loc[1], nil
	}
	return 0, gjerrors.NewTemplateSyntaxError(
		fmt.Sprintf("unexpected char %q", rest[0]), lineno, name, filename)
}

// wrap is the post-processing pass that converts rawTokens into Tokens
// suitable for the parser. Mirrors lexer.py:Lexer.wrap. It:
//   - drops ignored kinds (whitespace, comments)
//   - rewrites linestatement_begin/_end to block_begin/_end
//   - drops raw_begin/_end (parser doesn't see them)
//   - resolves operator tokens to their typed kind
//   - decodes string-literal escapes and validates identifiers
//   - normalizes newlines in TokenData values (Python uses
//     newline_re.sub, but we already normalized to \n; if the env wants
//     `\r\n` output, we re-encode here)
func (l *Lexer) wrap(raw []rawToken, name, filename string) ([]Token, error) {
	out := make([]Token, 0, len(raw))
	for _, t := range raw {
		k := t.Kind
		v := t.Value
		if ignoredKinds[k] {
			continue
		}
		switch k {
		case TokenLineStatementBegin:
			k = TokenBlockBegin
		case TokenLineStatementEnd:
			k = TokenBlockEnd
		case TokenRawBegin, TokenRawEnd:
			continue
		case TokenData:
			if l.opts.NewlineSequence != "\n" {
				v = strings.ReplaceAll(v, "\n", l.opts.NewlineSequence)
			}
		case TokenName:
			if !isIdentifier(v) {
				return nil, gjerrors.NewTemplateSyntaxError(
					"Invalid character in identifier", t.Lineno, name, filename)
			}
		case TokenString:
			// Strip surrounding quotes, normalize newlines, decode escapes.
			if len(v) < 2 {
				return nil, gjerrors.NewTemplateSyntaxError(
					"unterminated string literal", t.Lineno, name, filename)
			}
			body := v[1 : len(v)-1]
			if l.opts.NewlineSequence != "\n" {
				body = strings.ReplaceAll(body, "\n", l.opts.NewlineSequence)
			}
			decoded, err := decodeStringLiteral(body)
			if err != nil {
				return nil, gjerrors.NewTemplateSyntaxError(err.Error(), t.Lineno, name, filename)
			}
			v = decoded
		case TokenInteger:
			n, err := parseIntLiteral(v)
			if err != nil {
				return nil, gjerrors.NewTemplateSyntaxError(err.Error(), t.Lineno, name, filename)
			}
			v = strconv.FormatInt(n, 10)
		case TokenFloat:
			f, err := parseFloatLiteral(v)
			if err != nil {
				return nil, gjerrors.NewTemplateSyntaxError(err.Error(), t.Lineno, name, filename)
			}
			v = strconv.FormatFloat(f, 'g', -1, 64)
		case TokenOperator:
			opKind, ok := operators[v]
			if !ok {
				return nil, gjerrors.NewTemplateSyntaxError(
					fmt.Sprintf("unknown operator %q", v), t.Lineno, name, filename)
			}
			k = opKind
		}
		out = append(out, Token{Lineno: t.Lineno, Kind: k, Value: v})
	}
	out = append(out, Token{Lineno: lastLineno(out), Kind: TokenEOF})
	return out, nil
}

// lastLineno returns the line of the last token, or 1 if empty.
func lastLineno(toks []Token) int {
	if len(toks) == 0 {
		return 1
	}
	return toks[len(toks)-1].Lineno
}

// parseIntLiteral parses a Jinja2 integer literal, supporting all Python
// bases and `_` separators.
func parseIntLiteral(s string) (int64, error) {
	clean := strings.ReplaceAll(s, "_", "")
	switch {
	case strings.HasPrefix(clean, "0b") || strings.HasPrefix(clean, "0B"):
		return strconv.ParseInt(clean[2:], 2, 64)
	case strings.HasPrefix(clean, "0o") || strings.HasPrefix(clean, "0O"):
		return strconv.ParseInt(clean[2:], 8, 64)
	case strings.HasPrefix(clean, "0x") || strings.HasPrefix(clean, "0X"):
		return strconv.ParseInt(clean[2:], 16, 64)
	}
	return strconv.ParseInt(clean, 10, 64)
}

// parseFloatLiteral parses a Jinja2 float literal, allowing `_` separators.
func parseFloatLiteral(s string) (float64, error) {
	return strconv.ParseFloat(strings.ReplaceAll(s, "_", ""), 64)
}
