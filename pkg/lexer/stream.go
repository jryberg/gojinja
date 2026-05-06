package lexer

import (
	"fmt"

	gjerrors "github.com/jryberg/gojinja/pkg/errors"
)

// TokenStream is a forward iterator over [Token]s with a single-token
// peek buffer plus an unbounded "pushed" stack so the parser can speculate.
// Mirrors jinja2.lexer.TokenStream.
type TokenStream struct {
	tokens   []Token // remaining unread tokens (consumed front-to-back)
	pushed   []Token // pushed-back tokens, last item = next to read
	current  Token
	name     string
	filename string
	closed   bool
}

// NewTokenStream wraps a token slice (already passed through Wrap) into a
// streamable view. The slice is taken by reference but never mutated.
func NewTokenStream(tokens []Token, name, filename string) *TokenStream {
	s := &TokenStream{
		tokens:   tokens,
		current:  Token{Lineno: 1, Kind: TokenInitial},
		name:     name,
		filename: filename,
	}
	s.advance()
	return s
}

// Name returns the template name passed at construction.
func (s *TokenStream) Name() string { return s.name }

// Filename returns the template filename, if any.
func (s *TokenStream) Filename() string { return s.filename }

// Closed reports whether [Close] has been called.
func (s *TokenStream) Closed() bool { return s.closed }

// EOS reports whether the stream is exhausted (current is EOF and nothing
// is pushed back).
func (s *TokenStream) EOS() bool {
	return len(s.pushed) == 0 && s.current.Kind == TokenEOF
}

// Current returns the active token without consuming it.
func (s *TokenStream) Current() Token { return s.current }

// Push returns t to the front of the stream. The next call to [Next] will
// yield t.
func (s *TokenStream) Push(t Token) { s.pushed = append(s.pushed, t) }

// Look returns the token after the current one without permanently
// advancing. Internally it consumes one token, captures the new current,
// then pushes both back so the original ordering is preserved.
func (s *TokenStream) Look() Token {
	old := s.current
	s.Next()
	result := s.current
	s.Push(result)
	s.current = old
	return result
}

// Skip advances n tokens.
func (s *TokenStream) Skip(n int) {
	for i := 0; i < n; i++ {
		s.Next()
	}
}

// NextIf consumes and returns the current token if it matches expr;
// otherwise returns (Token{}, false).
func (s *TokenStream) NextIf(expr string) (Token, bool) {
	if s.current.Test(expr) {
		return s.Next(), true
	}
	return Token{}, false
}

// SkipIf is NextIf, discarding the token.
func (s *TokenStream) SkipIf(expr string) bool {
	_, ok := s.NextIf(expr)
	return ok
}

// Next consumes the current token, advances, and returns the consumed token.
func (s *TokenStream) Next() Token {
	rv := s.current
	s.advance()
	return rv
}

// Expect consumes the current token if it matches expr, or raises a
// TemplateSyntaxError.
func (s *TokenStream) Expect(expr string) (Token, error) {
	if !s.current.Test(expr) {
		desc := describeExpr(expr)
		if s.current.Kind == TokenEOF {
			return Token{}, gjerrors.NewTemplateSyntaxError(
				fmt.Sprintf("unexpected end of template, expected %q.", desc),
				s.current.Lineno, s.name, s.filename,
			)
		}
		return Token{}, gjerrors.NewTemplateSyntaxError(
			fmt.Sprintf("expected token %q, got %q", desc, s.current.String()),
			s.current.Lineno, s.name, s.filename,
		)
	}
	return s.Next(), nil
}

// Close marks the stream as drained.
func (s *TokenStream) Close() {
	s.current = Token{Lineno: s.current.Lineno, Kind: TokenEOF}
	s.tokens = nil
	s.pushed = nil
	s.closed = true
}

// advance moves current to the next token.
func (s *TokenStream) advance() {
	if len(s.pushed) > 0 {
		// pop from end (LIFO matches Python's deque-from-front + appendleft)
		s.current = s.pushed[len(s.pushed)-1]
		s.pushed = s.pushed[:len(s.pushed)-1]
		return
	}
	if s.current.Kind == TokenEOF {
		return
	}
	if len(s.tokens) == 0 {
		s.Close()
		return
	}
	s.current = s.tokens[0]
	s.tokens = s.tokens[1:]
}
