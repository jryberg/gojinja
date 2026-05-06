package lexer

// TokenKind is the type of a lexer Token. The set mirrors Jinja2's
// TOKEN_* constants (49 total).
type TokenKind int

// Token kinds. The order is irrelevant; iota stability is fine because the
// values are not persisted anywhere.
const (
	TokenAdd TokenKind = iota
	TokenSub
	TokenMul
	TokenDiv
	TokenFloorDiv
	TokenMod
	TokenPow
	TokenTilde
	TokenPipe
	TokenEq
	TokenNe
	TokenLt
	TokenLtEq
	TokenGt
	TokenGtEq
	TokenAssign
	TokenDot
	TokenColon
	TokenComma
	TokenSemicolon
	TokenLParen
	TokenRParen
	TokenLBracket
	TokenRBracket
	TokenLBrace
	TokenRBrace
	TokenName
	TokenString
	TokenInteger
	TokenFloat
	TokenOperator // generic operator before resolution
	TokenWhitespace
	TokenBlockBegin
	TokenBlockEnd
	TokenVariableBegin
	TokenVariableEnd
	TokenCommentBegin
	TokenCommentEnd
	TokenComment
	TokenRawBegin
	TokenRawEnd
	TokenLineStatementBegin
	TokenLineStatementEnd
	TokenLineCommentBegin
	TokenLineCommentEnd
	TokenLineComment
	TokenData
	TokenInitial
	TokenEOF
)

// kindNames maps TokenKind to its Jinja2 string name. Used by error
// messages and for parity with Python's describe_token output.
var kindNames = map[TokenKind]string{
	TokenAdd:                "add",
	TokenSub:                "sub",
	TokenMul:                "mul",
	TokenDiv:                "div",
	TokenFloorDiv:           "floordiv",
	TokenMod:                "mod",
	TokenPow:                "pow",
	TokenTilde:              "tilde",
	TokenPipe:               "pipe",
	TokenEq:                 "eq",
	TokenNe:                 "ne",
	TokenLt:                 "lt",
	TokenLtEq:               "lteq",
	TokenGt:                 "gt",
	TokenGtEq:               "gteq",
	TokenAssign:             "assign",
	TokenDot:                "dot",
	TokenColon:              "colon",
	TokenComma:              "comma",
	TokenSemicolon:          "semicolon",
	TokenLParen:             "lparen",
	TokenRParen:             "rparen",
	TokenLBracket:           "lbracket",
	TokenRBracket:           "rbracket",
	TokenLBrace:             "lbrace",
	TokenRBrace:             "rbrace",
	TokenName:               "name",
	TokenString:             "string",
	TokenInteger:            "integer",
	TokenFloat:              "float",
	TokenOperator:           "operator",
	TokenWhitespace:         "whitespace",
	TokenBlockBegin:         "block_begin",
	TokenBlockEnd:           "block_end",
	TokenVariableBegin:      "variable_begin",
	TokenVariableEnd:        "variable_end",
	TokenCommentBegin:       "comment_begin",
	TokenCommentEnd:         "comment_end",
	TokenComment:            "comment",
	TokenRawBegin:           "raw_begin",
	TokenRawEnd:             "raw_end",
	TokenLineStatementBegin: "linestatement_begin",
	TokenLineStatementEnd:   "linestatement_end",
	TokenLineCommentBegin:   "linecomment_begin",
	TokenLineCommentEnd:     "linecomment_end",
	TokenLineComment:        "linecomment",
	TokenData:               "data",
	TokenInitial:            "initial",
	TokenEOF:                "eof",
}

// String returns the Jinja2 token name (e.g. "block_begin").
func (k TokenKind) String() string {
	if s, ok := kindNames[k]; ok {
		return s
	}
	return "unknown"
}

// reverseOperators maps an operator TokenKind back to its source symbol.
// Mirrors lexer.py:reverse_operators.
var reverseOperators = map[TokenKind]string{
	TokenAdd:       "+",
	TokenSub:       "-",
	TokenMul:       "*",
	TokenDiv:       "/",
	TokenFloorDiv:  "//",
	TokenMod:       "%",
	TokenPow:       "**",
	TokenTilde:     "~",
	TokenPipe:      "|",
	TokenEq:        "==",
	TokenNe:        "!=",
	TokenLt:        "<",
	TokenLtEq:      "<=",
	TokenGt:        ">",
	TokenGtEq:      ">=",
	TokenAssign:    "=",
	TokenDot:       ".",
	TokenColon:     ":",
	TokenComma:     ",",
	TokenSemicolon: ";",
	TokenLParen:    "(",
	TokenRParen:    ")",
	TokenLBracket:  "[",
	TokenRBracket:  "]",
	TokenLBrace:    "{",
	TokenRBrace:    "}",
}

// operators is the symbol → kind table mirroring lexer.py:operators.
var operators = map[string]TokenKind{
	"+":  TokenAdd,
	"-":  TokenSub,
	"/":  TokenDiv,
	"//": TokenFloorDiv,
	"*":  TokenMul,
	"%":  TokenMod,
	"**": TokenPow,
	"~":  TokenTilde,
	"[":  TokenLBracket,
	"]":  TokenRBracket,
	"(":  TokenLParen,
	")":  TokenRParen,
	"{":  TokenLBrace,
	"}":  TokenRBrace,
	"==": TokenEq,
	"!=": TokenNe,
	">":  TokenGt,
	">=": TokenGtEq,
	"<":  TokenLt,
	"<=": TokenLtEq,
	"=":  TokenAssign,
	".":  TokenDot,
	":":  TokenColon,
	"|":  TokenPipe,
	",":  TokenComma,
	";":  TokenSemicolon,
}

// Token is the unit produced by the lexer. Lineno is 1-based.
type Token struct {
	Lineno int
	Kind   TokenKind
	// Value is the source text for literals/names/strings, or — for tokens
	// where the kind already determines the text (like operators) — the
	// raw matched text.
	Value string
}

// String returns a human-readable description of t, mirroring Jinja2's
// describe_token. For Name tokens, the identifier itself is returned.
func (t Token) String() string {
	if t.Kind == TokenName {
		return t.Value
	}
	return describeKind(t.Kind)
}

// Test reports whether t matches a token expression. Expressions are
// either a kind name ("name", "integer") or "kind:value" (e.g. "name:if").
// This mirrors Token.test in jinja2.lexer.
func (t Token) Test(expr string) bool {
	if expr == t.Kind.String() {
		return true
	}
	for i := 0; i < len(expr); i++ {
		if expr[i] == ':' {
			return expr[:i] == t.Kind.String() && expr[i+1:] == t.Value
		}
	}
	return false
}

// TestAny is a convenience for Test against multiple expressions.
func (t Token) TestAny(exprs ...string) bool {
	for _, e := range exprs {
		if t.Test(e) {
			return true
		}
	}
	return false
}

// describeKind mirrors lexer.py:_describe_token_type — friendlier names
// for tag delimiters and operators when surfaced in error messages.
func describeKind(k TokenKind) string {
	if op, ok := reverseOperators[k]; ok {
		return op
	}
	switch k {
	case TokenCommentBegin:
		return "begin of comment"
	case TokenCommentEnd:
		return "end of comment"
	case TokenComment, TokenLineComment:
		return "comment"
	case TokenBlockBegin:
		return "begin of statement block"
	case TokenBlockEnd:
		return "end of statement block"
	case TokenVariableBegin:
		return "begin of print statement"
	case TokenVariableEnd:
		return "end of print statement"
	case TokenLineStatementBegin:
		return "begin of line statement"
	case TokenLineStatementEnd:
		return "end of line statement"
	case TokenData:
		return "template data / text"
	case TokenEOF:
		return "end of template"
	}
	return k.String()
}

// describeExpr mirrors lexer.py:describe_token_expr.
func describeExpr(expr string) string {
	for i := 0; i < len(expr); i++ {
		if expr[i] == ':' {
			if expr[:i] == "name" {
				return expr[i+1:]
			}
			expr = expr[:i]
			break
		}
	}
	for k, name := range kindNames {
		if name == expr {
			return describeKind(k)
		}
	}
	return expr
}

// ignoredKinds are kinds dropped during the wrap pass (they never reach
// the parser). Mirrors lexer.py:ignored_tokens.
var ignoredKinds = map[TokenKind]bool{
	TokenCommentBegin:     true,
	TokenComment:          true,
	TokenCommentEnd:       true,
	TokenWhitespace:       true,
	TokenLineCommentBegin: true,
	TokenLineCommentEnd:   true,
	TokenLineComment:      true,
}

// ignoreIfEmpty are kinds that, when their match value is empty, are
// dropped. Mirrors lexer.py:ignore_if_empty.
var ignoreIfEmpty = map[TokenKind]bool{
	TokenWhitespace: true,
	TokenData:       true,
	TokenComment:    true,
	TokenLineComment: true,
}
