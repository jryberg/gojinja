// Package parser turns a [lexer.TokenStream] into an [ast.Template].
// The implementation mirrors jinja2.parser.Parser (12 statement tags +
// 13 precedence levels).
package parser

import (
	"fmt"
	"strconv"

	"github.com/jryberg/gojinja/pkg/ast"
	gjerrors "github.com/jryberg/gojinja/pkg/errors"
	"github.com/jryberg/gojinja/pkg/lexer"
)

// ExtensionFunc is the parse hook an extension supplies for each of its
// tags. The current token is the tag name; the func consumes the rest
// of the directive (excluding the closing block_end, which the caller
// expects).
type ExtensionFunc func(*Parser) (ast.Node, error)

// Parser walks a token stream and produces an AST.
type Parser struct {
	stream        *lexer.TokenStream
	name          string
	filename      string
	extensions    map[string]ExtensionFunc
	tagStack      []string
	endTokenStack [][]string
}

// New constructs a parser over stream. Extensions can be registered later
// via [Parser.RegisterExtension].
func New(stream *lexer.TokenStream) *Parser {
	return &Parser{
		stream:     stream,
		name:       stream.Name(),
		filename:   stream.Filename(),
		extensions: map[string]ExtensionFunc{},
	}
}

// RegisterExtension binds parse to be invoked when the tag is encountered.
func (p *Parser) RegisterExtension(tag string, parse ExtensionFunc) {
	p.extensions[tag] = parse
}

// Stream exposes the underlying [lexer.TokenStream] so extension parsers
// can consume tokens.
func (p *Parser) Stream() *lexer.TokenStream { return p.stream }

// ParseExpressionPublic is the public entry point for extension code that
// needs to parse a Jinja2 expression at the current stream position.
func (p *Parser) ParseExpressionPublic() (ast.Expr, error) { return p.parseExpression() }

// statementKeywords matches Python's _statement_keywords frozenset.
var statementKeywords = map[string]bool{
	"for": true, "if": true, "block": true, "extends": true,
	"print": true, "macro": true, "include": true, "from": true,
	"import": true, "set": true, "with": true, "autoescape": true,
}

// compareKinds maps a comparison-operator [lexer.TokenKind] to the
// Operand.Op string used in the AST.
var compareKinds = map[lexer.TokenKind]string{
	lexer.TokenEq:   "eq",
	lexer.TokenNe:   "ne",
	lexer.TokenLt:   "lt",
	lexer.TokenLtEq: "lteq",
	lexer.TokenGt:   "gt",
	lexer.TokenGtEq: "gteq",
}

// =============================================================== plumbing

func (p *Parser) fail(msg string, lineno int) error {
	if lineno == 0 {
		lineno = p.stream.Current().Lineno
	}
	return gjerrors.NewTemplateSyntaxError(msg, lineno, p.name, p.filename)
}

func (p *Parser) failAssertion(msg string, lineno int) error {
	if lineno == 0 {
		lineno = p.stream.Current().Lineno
	}
	return gjerrors.NewTemplateAssertionError(msg, lineno, p.name, p.filename)
}

func (p *Parser) failUnknownTag(name string, lineno int) error {
	return p.fail(fmt.Sprintf("Encountered unknown tag %q.", name), lineno)
}

func (p *Parser) failEOF(end []string, lineno int) error {
	expected := append([][]string{}, p.endTokenStack...)
	if end != nil {
		expected = append(expected, end)
	}
	if len(expected) == 0 {
		return p.fail("Unexpected end of template.", lineno)
	}
	return p.fail(fmt.Sprintf("Unexpected end of template. Looking for %v.", expected[len(expected)-1]), lineno)
}

func (p *Parser) currentLine() int { return p.stream.Current().Lineno }

func (p *Parser) skipColon() {
	p.stream.SkipIf("colon")
}

// =============================================================== entry

// Parse parses the whole stream into a *ast.Template.
func (p *Parser) Parse() (*ast.Template, error) {
	body, err := p.subparse(nil)
	if err != nil {
		return nil, err
	}
	tpl := &ast.Template{Body: body}
	tpl.SetLineno(1)
	return tpl, nil
}

// ParseExpression parses a single expression. Useful for tools and tests.
func (p *Parser) ParseExpression() (ast.Expr, error) {
	return p.parseCondExpr()
}

// =============================================================== subparse

// subparse parses statements until one of endTokens is the current token
// or stream EOF. End tokens are token expressions like "name:endif".
func (p *Parser) subparse(endTokens []string) ([]ast.Node, error) {
	var body []ast.Node
	var dataBuffer []ast.Expr

	flushData := func() {
		if len(dataBuffer) > 0 {
			out := &ast.Output{Nodes: dataBuffer}
			out.SetLineno(dataBuffer[0].Position().Lineno)
			body = append(body, out)
			dataBuffer = nil
		}
	}

	if endTokens != nil {
		p.endTokenStack = append(p.endTokenStack, endTokens)
		defer func() { p.endTokenStack = p.endTokenStack[:len(p.endTokenStack)-1] }()
	}

	for !p.stream.EOS() {
		tok := p.stream.Current()
		switch tok.Kind {
		case lexer.TokenData:
			if tok.Value != "" {
				td := &ast.TemplateData{Data: tok.Value}
				td.SetLineno(tok.Lineno)
				dataBuffer = append(dataBuffer, td)
			}
			p.stream.Next()
		case lexer.TokenVariableBegin:
			p.stream.Next()
			expr, err := p.parseTuple(false, true, nil, false, false)
			if err != nil {
				return nil, err
			}
			dataBuffer = append(dataBuffer, expr)
			if _, err := p.stream.Expect("variable_end"); err != nil {
				return nil, err
			}
		case lexer.TokenBlockBegin:
			flushData()
			p.stream.Next()
			if endTokens != nil && p.currentMatchesAny(endTokens) {
				return body, nil
			}
			rv, err := p.parseStatement()
			if err != nil {
				return nil, err
			}
			switch v := rv.(type) {
			case []ast.Node:
				body = append(body, v...)
			default:
				if v != nil {
					body = append(body, v.(ast.Node))
				}
			}
			if _, err := p.stream.Expect("block_end"); err != nil {
				return nil, err
			}
		default:
			return nil, p.fail("internal parsing error", tok.Lineno)
		}
	}
	flushData()
	return body, nil
}

func (p *Parser) currentMatchesAny(exprs []string) bool {
	cur := p.stream.Current()
	for _, e := range exprs {
		if cur.Test(e) {
			return true
		}
	}
	return false
}

// =============================================================== statements

func (p *Parser) parseStatement() (any, error) {
	tok := p.stream.Current()
	if tok.Kind != lexer.TokenName {
		return nil, p.fail("tag name expected", tok.Lineno)
	}
	p.tagStack = append(p.tagStack, tok.Value)
	defer func() { p.tagStack = p.tagStack[:len(p.tagStack)-1] }()

	if statementKeywords[tok.Value] {
		switch tok.Value {
		case "for":
			return p.parseFor()
		case "if":
			return p.parseIf()
		case "block":
			return p.parseBlock()
		case "extends":
			return p.parseExtends()
		case "print":
			return p.parsePrint()
		case "macro":
			return p.parseMacro()
		case "include":
			return p.parseInclude()
		case "from":
			return p.parseFrom()
		case "import":
			return p.parseImport()
		case "set":
			return p.parseSet()
		case "with":
			return p.parseWith()
		case "autoescape":
			return p.parseAutoescape()
		}
	}
	if tok.Value == "call" {
		return p.parseCallBlock()
	}
	if tok.Value == "filter" {
		return p.parseFilterBlock()
	}
	if tok.Value == "do" {
		return p.parseDo()
	}
	if tok.Value == "break" {
		return p.parseBreak()
	}
	if tok.Value == "continue" {
		return p.parseContinue()
	}
	if ext := p.extensions[tok.Value]; ext != nil {
		return ext(p)
	}
	p.tagStack = p.tagStack[:len(p.tagStack)-1]
	defer func() { p.tagStack = append(p.tagStack, tok.Value) }()
	return nil, p.failUnknownTag(tok.Value, tok.Lineno)
}

// parseStatements consumes the body of a block — allows a leading colon
// for Python compatibility, expects block_end, then subparses until one
// of endTokens.
func (p *Parser) parseStatements(endTokens []string, dropNeedle bool) ([]ast.Node, error) {
	p.skipColon()
	if _, err := p.stream.Expect("block_end"); err != nil {
		return nil, err
	}
	body, err := p.subparse(endTokens)
	if err != nil {
		return nil, err
	}
	if p.stream.Current().Kind == lexer.TokenEOF {
		return nil, p.failEOF(endTokens, p.currentLine())
	}
	if dropNeedle {
		p.stream.Next()
	}
	return body, nil
}

func (p *Parser) parseSet() (ast.Stmt, error) {
	lineno := p.stream.Next().Lineno
	target, err := p.parseAssignTarget(true, false, nil, true)
	if err != nil {
		return nil, err
	}
	if p.stream.SkipIf("assign") {
		expr, err := p.parseTuple(false, true, nil, false, false)
		if err != nil {
			return nil, err
		}
		n := &ast.Assign{Target: target, Node: expr}
		n.SetLineno(lineno)
		return n, nil
	}
	filterNode, err := p.parseFilterChain(nil, false)
	if err != nil {
		return nil, err
	}
	body, err := p.parseStatements([]string{"name:endset"}, true)
	if err != nil {
		return nil, err
	}
	var fn *ast.Filter
	if filterNode != nil {
		fn, _ = filterNode.(*ast.Filter)
	}
	n := &ast.AssignBlock{Target: target, Filter: fn, Body: body}
	n.SetLineno(lineno)
	return n, nil
}

func (p *Parser) parseFor() (*ast.For, error) {
	tok, err := p.stream.Expect("name:for")
	if err != nil {
		return nil, err
	}
	target, err := p.parseAssignTarget(true, false, []string{"name:in"}, false)
	if err != nil {
		return nil, err
	}
	if _, err := p.stream.Expect("name:in"); err != nil {
		return nil, err
	}
	iter, err := p.parseTuple(false, false, []string{"name:recursive"}, false, false)
	if err != nil {
		return nil, err
	}
	var test ast.Node
	if p.stream.SkipIf("name:if") {
		test, err = p.parseExpression()
		if err != nil {
			return nil, err
		}
	}
	recursive := p.stream.SkipIf("name:recursive")
	body, err := p.parseStatements([]string{"name:endfor", "name:else"}, false)
	if err != nil {
		return nil, err
	}
	var elseBody []ast.Node
	if p.stream.Next().Value == "else" {
		elseBody, err = p.parseStatements([]string{"name:endfor"}, true)
		if err != nil {
			return nil, err
		}
	}
	n := &ast.For{
		Target: target, Iter: iter, Body: body, Else: elseBody,
		Test: test, Recursive: recursive,
	}
	n.SetLineno(tok.Lineno)
	return n, nil
}

func (p *Parser) parseIf() (*ast.If, error) {
	tok, err := p.stream.Expect("name:if")
	if err != nil {
		return nil, err
	}
	result := &ast.If{}
	result.SetLineno(tok.Lineno)
	node := result
	for {
		test, err := p.parseTuple(false, false, nil, false, false)
		if err != nil {
			return nil, err
		}
		node.Test = test
		body, err := p.parseStatements([]string{"name:elif", "name:else", "name:endif"}, false)
		if err != nil {
			return nil, err
		}
		node.Body = body
		t := p.stream.Next()
		if t.Test("name:elif") {
			elif := &ast.If{}
			elif.SetLineno(p.currentLine())
			result.Elif = append(result.Elif, elif)
			node = elif
			continue
		}
		if t.Test("name:else") {
			elseBody, err := p.parseStatements([]string{"name:endif"}, true)
			if err != nil {
				return nil, err
			}
			result.Else = elseBody
		}
		break
	}
	return result, nil
}

func (p *Parser) parseWith() (*ast.With, error) {
	lineno := p.stream.Next().Lineno
	var targets, values []ast.Expr
	for p.stream.Current().Kind != lexer.TokenBlockEnd {
		if len(targets) > 0 {
			if _, err := p.stream.Expect("comma"); err != nil {
				return nil, err
			}
		}
		t, err := p.parseAssignTarget(true, false, nil, false)
		if err != nil {
			return nil, err
		}
		setCtx(t, ast.CtxParam)
		targets = append(targets, t)
		if _, err := p.stream.Expect("assign"); err != nil {
			return nil, err
		}
		v, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		values = append(values, v)
	}
	body, err := p.parseStatements([]string{"name:endwith"}, true)
	if err != nil {
		return nil, err
	}
	n := &ast.With{Targets: targets, Values: values, Body: body}
	n.SetLineno(lineno)
	return n, nil
}

func (p *Parser) parseAutoescape() (ast.Stmt, error) {
	lineno := p.stream.Next().Lineno
	val, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	body, err := p.parseStatements([]string{"name:endautoescape"}, true)
	if err != nil {
		return nil, err
	}
	mod := &ast.ScopedEvalContextModifier{
		Options: []*ast.Keyword{{Key: "autoescape", Value: val}},
		Body:    body,
	}
	mod.SetLineno(lineno)
	scope := &ast.Scope{Body: []ast.Node{mod}}
	scope.SetLineno(lineno)
	return scope, nil
}

func (p *Parser) parseBlock() (*ast.Block, error) {
	lineno := p.stream.Next().Lineno
	nameTok, err := p.stream.Expect("name")
	if err != nil {
		return nil, err
	}
	scoped := p.stream.SkipIf("name:scoped")
	required := p.stream.SkipIf("name:required")
	if p.stream.Current().Kind == lexer.TokenSub {
		return nil, p.fail("Block names in Jinja have to be valid Python identifiers and may not contain hyphens, use an underscore instead.", p.currentLine())
	}
	body, err := p.parseStatements([]string{"name:endblock"}, true)
	if err != nil {
		return nil, err
	}
	if required {
		for _, b := range body {
			out, ok := b.(*ast.Output)
			if !ok {
				return nil, p.fail("Required blocks can only contain comments or whitespace", lineno)
			}
			for _, n := range out.Nodes {
				td, ok := n.(*ast.TemplateData)
				if !ok || !isAllSpace(td.Data) {
					return nil, p.fail("Required blocks can only contain comments or whitespace", lineno)
				}
			}
		}
	}
	p.stream.SkipIf("name:" + nameTok.Value)
	bn := &ast.Block{Name: nameTok.Value, Body: body, Scoped: scoped, Required: required}
	bn.SetLineno(lineno)
	return bn, nil
}

func (p *Parser) parseExtends() (*ast.Extends, error) {
	lineno := p.stream.Next().Lineno
	tpl, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	n := &ast.Extends{Template: tpl}
	n.SetLineno(lineno)
	return n, nil
}

func (p *Parser) parseInclude() (*ast.Include, error) {
	lineno := p.stream.Next().Lineno
	tpl, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	ignoreMissing := false
	if p.stream.Current().Test("name:ignore") && p.stream.Look().Test("name:missing") {
		ignoreMissing = true
		p.stream.Skip(2)
	}
	withCtx := p.parseImportContext(true)
	n := &ast.Include{Template: tpl, IgnoreMissing: ignoreMissing, WithContext: withCtx}
	n.SetLineno(lineno)
	return n, nil
}

func (p *Parser) parseImport() (*ast.Import, error) {
	lineno := p.stream.Next().Lineno
	tpl, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	if _, err := p.stream.Expect("name:as"); err != nil {
		return nil, err
	}
	target, err := p.parseAssignTarget(false, true, nil, false)
	if err != nil {
		return nil, err
	}
	withCtx := p.parseImportContext(false)
	n := &ast.Import{Template: tpl, Target: target.(*ast.Name).Name, WithContext: withCtx}
	n.SetLineno(lineno)
	return n, nil
}

func (p *Parser) parseFrom() (*ast.FromImport, error) {
	lineno := p.stream.Next().Lineno
	tpl, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	if _, err := p.stream.Expect("name:import"); err != nil {
		return nil, err
	}
	withContext := false
	withContextSet := false
	var names []ast.ImportName
	parseContext := func() bool {
		cur := p.stream.Current()
		if (cur.Value == "with" || cur.Value == "without") && p.stream.Look().Test("name:context") {
			withContext = p.stream.Next().Value == "with"
			withContextSet = true
			p.stream.Skip(1)
			return true
		}
		return false
	}
	for {
		if len(names) > 0 {
			if _, err := p.stream.Expect("comma"); err != nil {
				return nil, err
			}
		}
		if p.stream.Current().Kind == lexer.TokenName {
			if parseContext() {
				break
			}
			tgt, err := p.parseAssignTarget(false, true, nil, false)
			if err != nil {
				return nil, err
			}
			tn := tgt.(*ast.Name)
			if len(tn.Name) > 0 && tn.Name[0] == '_' {
				return nil, p.failAssertion("names starting with an underline can not be imported", tn.Position().Lineno)
			}
			entry := ast.ImportName{Name: tn.Name}
			if p.stream.SkipIf("name:as") {
				al, err := p.parseAssignTarget(false, true, nil, false)
				if err != nil {
					return nil, err
				}
				entry.Alias = al.(*ast.Name).Name
			}
			names = append(names, entry)
			if parseContext() || p.stream.Current().Kind != lexer.TokenComma {
				break
			}
		} else {
			if _, err := p.stream.Expect("name"); err != nil {
				return nil, err
			}
		}
	}
	if !withContextSet {
		withContext = false
	}
	n := &ast.FromImport{Template: tpl, Names: names, WithContext: withContext}
	n.SetLineno(lineno)
	return n, nil
}

func (p *Parser) parseImportContext(def bool) bool {
	if (p.stream.Current().Test("name:with") || p.stream.Current().Test("name:without")) && p.stream.Look().Test("name:context") {
		v := p.stream.Next().Value == "with"
		p.stream.Skip(1)
		return v
	}
	return def
}

func (p *Parser) parseMacro() (*ast.Macro, error) {
	lineno := p.stream.Next().Lineno
	nameTgt, err := p.parseAssignTarget(false, true, nil, false)
	if err != nil {
		return nil, err
	}
	args, defaults, err := p.parseSignature()
	if err != nil {
		return nil, err
	}
	body, err := p.parseStatements([]string{"name:endmacro"}, true)
	if err != nil {
		return nil, err
	}
	n := &ast.Macro{Name: nameTgt.(*ast.Name).Name, Args: args, Defaults: defaults, Body: body}
	n.SetLineno(lineno)
	return n, nil
}

func (p *Parser) parseCallBlock() (*ast.CallBlock, error) {
	lineno := p.stream.Next().Lineno
	var args []*ast.Name
	var defaults []ast.Expr
	if p.stream.Current().Kind == lexer.TokenLParen {
		var err error
		args, defaults, err = p.parseSignature()
		if err != nil {
			return nil, err
		}
	}
	callExpr, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	call, ok := callExpr.(*ast.Call)
	if !ok {
		return nil, p.fail("expected call", lineno)
	}
	body, err := p.parseStatements([]string{"name:endcall"}, true)
	if err != nil {
		return nil, err
	}
	n := &ast.CallBlock{Call: call, Args: args, Defaults: defaults, Body: body}
	n.SetLineno(lineno)
	return n, nil
}

func (p *Parser) parseFilterBlock() (*ast.FilterBlock, error) {
	lineno := p.stream.Next().Lineno
	flt, err := p.parseFilterChain(nil, true)
	if err != nil {
		return nil, err
	}
	body, err := p.parseStatements([]string{"name:endfilter"}, true)
	if err != nil {
		return nil, err
	}
	n := &ast.FilterBlock{Body: body, Filter: flt.(*ast.Filter)}
	n.SetLineno(lineno)
	return n, nil
}

func (p *Parser) parsePrint() (*ast.Output, error) {
	lineno := p.stream.Next().Lineno
	var nodes []ast.Expr
	for p.stream.Current().Kind != lexer.TokenBlockEnd {
		if len(nodes) > 0 {
			if _, err := p.stream.Expect("comma"); err != nil {
				return nil, err
			}
		}
		e, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, e)
	}
	n := &ast.Output{Nodes: nodes}
	n.SetLineno(lineno)
	return n, nil
}

// parseDo handles `{% do expr %}`.
func (p *Parser) parseDo() (*ast.ExprStmt, error) {
	lineno := p.stream.Next().Lineno
	expr, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	n := &ast.ExprStmt{Node: expr}
	n.SetLineno(lineno)
	return n, nil
}

// parseBreak handles `{% break %}`.
func (p *Parser) parseBreak() (*ast.Break, error) {
	lineno := p.stream.Next().Lineno
	n := &ast.Break{}
	n.SetLineno(lineno)
	return n, nil
}

// parseContinue handles `{% continue %}`.
func (p *Parser) parseContinue() (*ast.Continue, error) {
	lineno := p.stream.Next().Lineno
	n := &ast.Continue{}
	n.SetLineno(lineno)
	return n, nil
}

// parseSignature parses a `(arg, arg=default, ...)` formal argument list.
func (p *Parser) parseSignature() ([]*ast.Name, []ast.Expr, error) {
	var args []*ast.Name
	var defaults []ast.Expr
	if _, err := p.stream.Expect("lparen"); err != nil {
		return nil, nil, err
	}
	for p.stream.Current().Kind != lexer.TokenRParen {
		if len(args) > 0 {
			if _, err := p.stream.Expect("comma"); err != nil {
				return nil, nil, err
			}
		}
		arg, err := p.parseAssignTarget(false, true, nil, false)
		if err != nil {
			return nil, nil, err
		}
		setCtx(arg, ast.CtxParam)
		if p.stream.SkipIf("assign") {
			d, err := p.parseExpression()
			if err != nil {
				return nil, nil, err
			}
			defaults = append(defaults, d)
		} else if len(defaults) > 0 {
			return nil, nil, p.fail("non-default argument follows default argument", p.currentLine())
		}
		args = append(args, arg.(*ast.Name))
	}
	if _, err := p.stream.Expect("rparen"); err != nil {
		return nil, nil, err
	}
	return args, defaults, nil
}

// =============================================================== assign target

func (p *Parser) parseAssignTarget(withTuple bool, nameOnly bool, extraEnd []string, withNamespace bool) (ast.Expr, error) {
	var target ast.Expr
	if nameOnly {
		tok, err := p.stream.Expect("name")
		if err != nil {
			return nil, err
		}
		n := &ast.Name{Name: tok.Value, Ctx: ast.CtxStore}
		n.SetLineno(tok.Lineno)
		target = n
	} else {
		var err error
		if withTuple {
			target, err = p.parseTuple(true, true, extraEnd, false, withNamespace)
		} else {
			target, err = p.parsePrimary(withNamespace)
		}
		if err != nil {
			return nil, err
		}
		setCtx(target, ast.CtxStore)
	}
	if !target.CanAssign() {
		return nil, p.fail(fmt.Sprintf("can't assign to %T", target), target.Position().Lineno)
	}
	return target, nil
}

// setCtx walks an expression tree (Name, NSRef, Tuple) and assigns ctx.
func setCtx(e ast.Expr, ctx ast.AssignContext) {
	switch x := e.(type) {
	case *ast.Name:
		x.Ctx = ctx
	case *ast.Tuple:
		x.Ctx = ctx
		for _, it := range x.Items {
			setCtx(it, ctx)
		}
	}
}

// =============================================================== expressions

func (p *Parser) parseExpression() (ast.Expr, error) {
	return p.parseCondExpr()
}

func (p *Parser) parseCondExpr() (ast.Expr, error) {
	lineno := p.currentLine()
	expr1, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	for p.stream.SkipIf("name:if") {
		test, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		var expr3 ast.Expr
		if p.stream.SkipIf("name:else") {
			expr3, err = p.parseCondExpr()
			if err != nil {
				return nil, err
			}
		}
		ce := &ast.CondExpr{Test: test, Expr1: expr1, Expr2: expr3}
		ce.SetLineno(lineno)
		expr1 = ce
		lineno = p.currentLine()
	}
	return expr1, nil
}

func (p *Parser) parseOr() (ast.Expr, error) {
	lineno := p.currentLine()
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.stream.SkipIf("name:or") {
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		n := &ast.Or{}
		n.Op = "or"
		n.Left = left
		n.Right = right
		n.SetLineno(lineno)
		left = n
		lineno = p.currentLine()
	}
	return left, nil
}

func (p *Parser) parseAnd() (ast.Expr, error) {
	lineno := p.currentLine()
	left, err := p.parseNot()
	if err != nil {
		return nil, err
	}
	for p.stream.SkipIf("name:and") {
		right, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		n := &ast.And{}
		n.Op = "and"
		n.Left = left
		n.Right = right
		n.SetLineno(lineno)
		left = n
		lineno = p.currentLine()
	}
	return left, nil
}

func (p *Parser) parseNot() (ast.Expr, error) {
	if p.stream.Current().Test("name:not") {
		lineno := p.stream.Next().Lineno
		inner, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		n := &ast.Not{}
		n.Op = "not"
		n.Node = inner
		n.SetLineno(lineno)
		return n, nil
	}
	return p.parseCompare()
}

func (p *Parser) parseCompare() (ast.Expr, error) {
	lineno := p.currentLine()
	expr, err := p.parseMath1()
	if err != nil {
		return nil, err
	}
	var ops []*ast.Operand
	for {
		cur := p.stream.Current()
		if op, ok := compareKinds[cur.Kind]; ok {
			p.stream.Next()
			right, err := p.parseMath1()
			if err != nil {
				return nil, err
			}
			operand := &ast.Operand{Op: op, Expr: right}
			operand.SetLineno(lineno)
			ops = append(ops, operand)
		} else if p.stream.SkipIf("name:in") {
			right, err := p.parseMath1()
			if err != nil {
				return nil, err
			}
			operand := &ast.Operand{Op: "in", Expr: right}
			operand.SetLineno(lineno)
			ops = append(ops, operand)
		} else if p.stream.Current().Test("name:not") && p.stream.Look().Test("name:in") {
			p.stream.Skip(2)
			right, err := p.parseMath1()
			if err != nil {
				return nil, err
			}
			operand := &ast.Operand{Op: "notin", Expr: right}
			operand.SetLineno(lineno)
			ops = append(ops, operand)
		} else {
			break
		}
		lineno = p.currentLine()
	}
	if len(ops) == 0 {
		return expr, nil
	}
	c := &ast.Compare{Expr: expr, Ops: ops}
	c.SetLineno(lineno)
	return c, nil
}

func (p *Parser) parseMath1() (ast.Expr, error) {
	lineno := p.currentLine()
	left, err := p.parseConcat()
	if err != nil {
		return nil, err
	}
	for {
		k := p.stream.Current().Kind
		if k != lexer.TokenAdd && k != lexer.TokenSub {
			break
		}
		p.stream.Next()
		right, err := p.parseConcat()
		if err != nil {
			return nil, err
		}
		left = newBinop(k, left, right, lineno)
		lineno = p.currentLine()
	}
	return left, nil
}

func (p *Parser) parseConcat() (ast.Expr, error) {
	lineno := p.currentLine()
	first, err := p.parseMath2()
	if err != nil {
		return nil, err
	}
	args := []ast.Expr{first}
	for p.stream.Current().Kind == lexer.TokenTilde {
		p.stream.Next()
		next, err := p.parseMath2()
		if err != nil {
			return nil, err
		}
		args = append(args, next)
	}
	if len(args) == 1 {
		return args[0], nil
	}
	c := &ast.Concat{Nodes: args}
	c.SetLineno(lineno)
	return c, nil
}

func (p *Parser) parseMath2() (ast.Expr, error) {
	lineno := p.currentLine()
	left, err := p.parsePow()
	if err != nil {
		return nil, err
	}
	for {
		k := p.stream.Current().Kind
		if k != lexer.TokenMul && k != lexer.TokenDiv && k != lexer.TokenFloorDiv && k != lexer.TokenMod {
			break
		}
		p.stream.Next()
		right, err := p.parsePow()
		if err != nil {
			return nil, err
		}
		left = newBinop(k, left, right, lineno)
		lineno = p.currentLine()
	}
	return left, nil
}

func (p *Parser) parsePow() (ast.Expr, error) {
	lineno := p.currentLine()
	left, err := p.parseUnary(true)
	if err != nil {
		return nil, err
	}
	for p.stream.Current().Kind == lexer.TokenPow {
		p.stream.Next()
		right, err := p.parseUnary(true)
		if err != nil {
			return nil, err
		}
		n := &ast.Pow{}
		n.Op = "**"
		n.Left = left
		n.Right = right
		n.SetLineno(lineno)
		left = n
		lineno = p.currentLine()
	}
	return left, nil
}

func (p *Parser) parseUnary(withFilter bool) (ast.Expr, error) {
	tok := p.stream.Current()
	lineno := tok.Lineno
	var node ast.Expr
	switch tok.Kind {
	case lexer.TokenSub:
		p.stream.Next()
		inner, err := p.parseUnary(false)
		if err != nil {
			return nil, err
		}
		n := &ast.Neg{}
		n.Op = "-"
		n.Node = inner
		n.SetLineno(lineno)
		node = n
	case lexer.TokenAdd:
		p.stream.Next()
		inner, err := p.parseUnary(false)
		if err != nil {
			return nil, err
		}
		n := &ast.UAdd{}
		n.Op = "+"
		n.Node = inner
		n.SetLineno(lineno)
		node = n
	default:
		var err error
		node, err = p.parsePrimary(false)
		if err != nil {
			return nil, err
		}
	}
	var err error
	node, err = p.parsePostfix(node)
	if err != nil {
		return nil, err
	}
	if withFilter {
		node, err = p.parseFilterExpr(node)
		if err != nil {
			return nil, err
		}
	}
	return node, nil
}

func (p *Parser) parsePrimary(withNamespace bool) (ast.Expr, error) {
	tok := p.stream.Current()
	switch tok.Kind {
	case lexer.TokenName:
		p.stream.Next()
		switch tok.Value {
		case "true", "True":
			c := &ast.Const{Value: true}
			c.SetLineno(tok.Lineno)
			return c, nil
		case "false", "False":
			c := &ast.Const{Value: false}
			c.SetLineno(tok.Lineno)
			return c, nil
		case "none", "None":
			c := &ast.Const{Value: nil}
			c.SetLineno(tok.Lineno)
			return c, nil
		}
		if withNamespace && p.stream.Current().Kind == lexer.TokenDot {
			p.stream.Next()
			attr, err := p.stream.Expect("name")
			if err != nil {
				return nil, err
			}
			ns := &ast.NSRef{Name: tok.Value, Attr: attr.Value}
			ns.SetLineno(tok.Lineno)
			return ns, nil
		}
		n := &ast.Name{Name: tok.Value, Ctx: ast.CtxLoad}
		n.SetLineno(tok.Lineno)
		return n, nil
	case lexer.TokenString:
		p.stream.Next()
		buf := tok.Value
		for p.stream.Current().Kind == lexer.TokenString {
			buf += p.stream.Current().Value
			p.stream.Next()
		}
		c := &ast.Const{Value: buf}
		c.SetLineno(tok.Lineno)
		return c, nil
	case lexer.TokenInteger:
		p.stream.Next()
		n, err := strconv.ParseInt(tok.Value, 10, 64)
		if err != nil {
			return nil, p.fail("invalid integer literal", tok.Lineno)
		}
		c := &ast.Const{Value: n}
		c.SetLineno(tok.Lineno)
		return c, nil
	case lexer.TokenFloat:
		p.stream.Next()
		f, err := strconv.ParseFloat(tok.Value, 64)
		if err != nil {
			return nil, p.fail("invalid float literal", tok.Lineno)
		}
		c := &ast.Const{Value: f}
		c.SetLineno(tok.Lineno)
		return c, nil
	case lexer.TokenLParen:
		p.stream.Next()
		t, err := p.parseTuple(false, true, nil, true, false)
		if err != nil {
			return nil, err
		}
		if _, err := p.stream.Expect("rparen"); err != nil {
			return nil, err
		}
		return t, nil
	case lexer.TokenLBracket:
		return p.parseList()
	case lexer.TokenLBrace:
		return p.parseDict()
	default:
		return nil, p.fail(fmt.Sprintf("unexpected %q", tok.String()), tok.Lineno)
	}
}

func (p *Parser) parseTuple(simplified, withCondExpr bool, extraEnd []string, explicitParens, withNamespace bool) (ast.Expr, error) {
	lineno := p.currentLine()
	parse := func() (ast.Expr, error) {
		if simplified {
			return p.parsePrimary(withNamespace)
		}
		if withCondExpr {
			return p.parseExpression()
		}
		return p.parseOr()
	}
	var args []ast.Expr
	isTuple := false
	for {
		if len(args) > 0 {
			if _, err := p.stream.Expect("comma"); err != nil {
				return nil, err
			}
		}
		if p.isTupleEnd(extraEnd) {
			break
		}
		e, err := parse()
		if err != nil {
			return nil, err
		}
		args = append(args, e)
		if p.stream.Current().Kind == lexer.TokenComma {
			isTuple = true
		} else {
			break
		}
		lineno = p.currentLine()
	}
	if !isTuple {
		if len(args) > 0 {
			return args[0], nil
		}
		if !explicitParens {
			return nil, p.fail(fmt.Sprintf("Expected an expression, got %q", p.stream.Current().String()), p.currentLine())
		}
	}
	t := &ast.Tuple{Items: args, Ctx: ast.CtxLoad}
	t.SetLineno(lineno)
	return t, nil
}

func (p *Parser) isTupleEnd(extraEnd []string) bool {
	cur := p.stream.Current()
	if cur.Kind == lexer.TokenVariableEnd || cur.Kind == lexer.TokenBlockEnd || cur.Kind == lexer.TokenRParen {
		return true
	}
	for _, e := range extraEnd {
		if cur.Test(e) {
			return true
		}
	}
	return false
}

func (p *Parser) parseList() (ast.Expr, error) {
	tok, err := p.stream.Expect("lbracket")
	if err != nil {
		return nil, err
	}
	var items []ast.Expr
	for p.stream.Current().Kind != lexer.TokenRBracket {
		if len(items) > 0 {
			if _, err := p.stream.Expect("comma"); err != nil {
				return nil, err
			}
		}
		if p.stream.Current().Kind == lexer.TokenRBracket {
			break
		}
		e, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		items = append(items, e)
	}
	if _, err := p.stream.Expect("rbracket"); err != nil {
		return nil, err
	}
	l := &ast.List{Items: items}
	l.SetLineno(tok.Lineno)
	return l, nil
}

func (p *Parser) parseDict() (ast.Expr, error) {
	tok, err := p.stream.Expect("lbrace")
	if err != nil {
		return nil, err
	}
	var items []*ast.Pair
	for p.stream.Current().Kind != lexer.TokenRBrace {
		if len(items) > 0 {
			if _, err := p.stream.Expect("comma"); err != nil {
				return nil, err
			}
		}
		if p.stream.Current().Kind == lexer.TokenRBrace {
			break
		}
		key, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if _, err := p.stream.Expect("colon"); err != nil {
			return nil, err
		}
		value, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		pair := &ast.Pair{Key: key, Value: value}
		pair.SetLineno(key.Position().Lineno)
		items = append(items, pair)
	}
	if _, err := p.stream.Expect("rbrace"); err != nil {
		return nil, err
	}
	d := &ast.Dict{Items: items}
	d.SetLineno(tok.Lineno)
	return d, nil
}

func (p *Parser) parsePostfix(node ast.Expr) (ast.Expr, error) {
	for {
		k := p.stream.Current().Kind
		if k == lexer.TokenDot || k == lexer.TokenLBracket {
			n, err := p.parseSubscript(node)
			if err != nil {
				return nil, err
			}
			node = n
		} else if k == lexer.TokenLParen {
			n, err := p.parseCall(node)
			if err != nil {
				return nil, err
			}
			node = n
		} else {
			break
		}
	}
	return node, nil
}

func (p *Parser) parseFilterExpr(node ast.Expr) (ast.Expr, error) {
	for {
		cur := p.stream.Current()
		if cur.Kind == lexer.TokenPipe {
			n, err := p.parseFilterChain(node, false)
			if err != nil {
				return nil, err
			}
			node = n
		} else if cur.Kind == lexer.TokenName && cur.Value == "is" {
			n, err := p.parseTest(node)
			if err != nil {
				return nil, err
			}
			node = n
		} else if cur.Kind == lexer.TokenLParen {
			n, err := p.parseCall(node)
			if err != nil {
				return nil, err
			}
			node = n
		} else {
			break
		}
	}
	return node, nil
}

func (p *Parser) parseSubscript(node ast.Expr) (ast.Expr, error) {
	tok := p.stream.Next()
	if tok.Kind == lexer.TokenDot {
		attr := p.stream.Current()
		p.stream.Next()
		if attr.Kind == lexer.TokenName {
			g := &ast.Getattr{Node: node, Attr: attr.Value, Ctx: ast.CtxLoad}
			g.SetLineno(tok.Lineno)
			return g, nil
		}
		if attr.Kind != lexer.TokenInteger {
			return nil, p.fail("expected name or number", attr.Lineno)
		}
		n, err := strconv.ParseInt(attr.Value, 10, 64)
		if err != nil {
			return nil, p.fail("invalid integer", attr.Lineno)
		}
		idx := &ast.Const{Value: n}
		idx.SetLineno(attr.Lineno)
		gi := &ast.Getitem{Node: node, Arg: idx, Ctx: ast.CtxLoad}
		gi.SetLineno(tok.Lineno)
		return gi, nil
	}
	if tok.Kind == lexer.TokenLBracket {
		var args []ast.Expr
		for p.stream.Current().Kind != lexer.TokenRBracket {
			if len(args) > 0 {
				if _, err := p.stream.Expect("comma"); err != nil {
					return nil, err
				}
			}
			s, err := p.parseSubscribed()
			if err != nil {
				return nil, err
			}
			args = append(args, s)
		}
		if _, err := p.stream.Expect("rbracket"); err != nil {
			return nil, err
		}
		var arg ast.Expr
		if len(args) == 1 {
			arg = args[0]
		} else {
			t := &ast.Tuple{Items: args, Ctx: ast.CtxLoad}
			t.SetLineno(tok.Lineno)
			arg = t
		}
		gi := &ast.Getitem{Node: node, Arg: arg, Ctx: ast.CtxLoad}
		gi.SetLineno(tok.Lineno)
		return gi, nil
	}
	return nil, p.fail("expected subscript expression", tok.Lineno)
}

func (p *Parser) parseSubscribed() (ast.Expr, error) {
	lineno := p.currentLine()
	var args [3]ast.Expr
	if p.stream.Current().Kind == lexer.TokenColon {
		p.stream.Next()
	} else {
		first, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if p.stream.Current().Kind != lexer.TokenColon {
			return first, nil
		}
		p.stream.Next()
		args[0] = first
	}
	// Slice: at least one ':' has been consumed.
	if p.stream.Current().Kind != lexer.TokenColon &&
		p.stream.Current().Kind != lexer.TokenRBracket &&
		p.stream.Current().Kind != lexer.TokenComma {
		stop, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		args[1] = stop
	}
	if p.stream.Current().Kind == lexer.TokenColon {
		p.stream.Next()
		if p.stream.Current().Kind != lexer.TokenRBracket && p.stream.Current().Kind != lexer.TokenComma {
			step, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			args[2] = step
		}
	}
	s := &ast.Slice{Start: args[0], Stop: args[1], Step: args[2]}
	s.SetLineno(lineno)
	return s, nil
}

func (p *Parser) parseCall(node ast.Expr) (ast.Expr, error) {
	tok := p.stream.Current()
	args, kwargs, dynA, dynK, err := p.parseCallArgs()
	if err != nil {
		return nil, err
	}
	c := &ast.Call{Node: node, Args: args, Kwargs: kwargs, DynArgs: dynA, DynKwargs: dynK}
	c.SetLineno(tok.Lineno)
	return c, nil
}

func (p *Parser) parseCallArgs() ([]ast.Expr, []*ast.Keyword, ast.Expr, ast.Expr, error) {
	tok, err := p.stream.Expect("lparen")
	if err != nil {
		return nil, nil, nil, nil, err
	}
	var args []ast.Expr
	var kwargs []*ast.Keyword
	var dynArgs, dynKwargs ast.Expr
	requireComma := false
	ensure := func(ok bool) error {
		if !ok {
			return p.fail("invalid syntax for function call expression", tok.Lineno)
		}
		return nil
	}
	for p.stream.Current().Kind != lexer.TokenRParen {
		if requireComma {
			if _, err := p.stream.Expect("comma"); err != nil {
				return nil, nil, nil, nil, err
			}
			if p.stream.Current().Kind == lexer.TokenRParen {
				break
			}
		}
		switch p.stream.Current().Kind {
		case lexer.TokenMul:
			if err := ensure(dynArgs == nil && dynKwargs == nil); err != nil {
				return nil, nil, nil, nil, err
			}
			p.stream.Next()
			e, err := p.parseExpression()
			if err != nil {
				return nil, nil, nil, nil, err
			}
			dynArgs = e
		case lexer.TokenPow:
			if err := ensure(dynKwargs == nil); err != nil {
				return nil, nil, nil, nil, err
			}
			p.stream.Next()
			e, err := p.parseExpression()
			if err != nil {
				return nil, nil, nil, nil, err
			}
			dynKwargs = e
		default:
			if p.stream.Current().Kind == lexer.TokenName && p.stream.Look().Kind == lexer.TokenAssign {
				if err := ensure(dynKwargs == nil); err != nil {
					return nil, nil, nil, nil, err
				}
				key := p.stream.Current().Value
				p.stream.Skip(2)
				v, err := p.parseExpression()
				if err != nil {
					return nil, nil, nil, nil, err
				}
				kw := &ast.Keyword{Key: key, Value: v}
				kw.SetLineno(v.Position().Lineno)
				kwargs = append(kwargs, kw)
			} else {
				if err := ensure(dynArgs == nil && dynKwargs == nil && len(kwargs) == 0); err != nil {
					return nil, nil, nil, nil, err
				}
				e, err := p.parseExpression()
				if err != nil {
					return nil, nil, nil, nil, err
				}
				args = append(args, e)
			}
		}
		requireComma = true
	}
	if _, err := p.stream.Expect("rparen"); err != nil {
		return nil, nil, nil, nil, err
	}
	return args, kwargs, dynArgs, dynKwargs, nil
}

// parseFilterChain handles `| name(args) | name2(args)` chains. When
// startInline is true, the leading `|` is not required (used by filter
// blocks).
func (p *Parser) parseFilterChain(node ast.Expr, startInline bool) (ast.Expr, error) {
	for p.stream.Current().Kind == lexer.TokenPipe || startInline {
		if !startInline {
			p.stream.Next()
		}
		startInline = false
		tok, err := p.stream.Expect("name")
		if err != nil {
			return nil, err
		}
		name := tok.Value
		for p.stream.Current().Kind == lexer.TokenDot {
			p.stream.Next()
			seg, err := p.stream.Expect("name")
			if err != nil {
				return nil, err
			}
			name += "." + seg.Value
		}
		var args []ast.Expr
		var kwargs []*ast.Keyword
		var dynA, dynK ast.Expr
		if p.stream.Current().Kind == lexer.TokenLParen {
			args, kwargs, dynA, dynK, err = p.parseCallArgs()
			if err != nil {
				return nil, err
			}
		}
		f := &ast.Filter{Node: node, Name: name, Args: args, Kwargs: kwargs, DynArgs: dynA, DynKwargs: dynK}
		f.SetLineno(tok.Lineno)
		node = f
	}
	return node, nil
}

func (p *Parser) parseTest(node ast.Expr) (ast.Expr, error) {
	tok := p.stream.Next()
	negated := false
	if p.stream.Current().Test("name:not") {
		p.stream.Next()
		negated = true
	}
	nameTok, err := p.stream.Expect("name")
	if err != nil {
		return nil, err
	}
	name := nameTok.Value
	for p.stream.Current().Kind == lexer.TokenDot {
		p.stream.Next()
		seg, err := p.stream.Expect("name")
		if err != nil {
			return nil, err
		}
		name += "." + seg.Value
	}
	var args []ast.Expr
	var kwargs []*ast.Keyword
	var dynA, dynK ast.Expr
	cur := p.stream.Current()
	if cur.Kind == lexer.TokenLParen {
		args, kwargs, dynA, dynK, err = p.parseCallArgs()
		if err != nil {
			return nil, err
		}
	} else if testStartCanFollow(cur) {
		if cur.Test("name:is") {
			return nil, p.fail("You cannot chain multiple tests with is", cur.Lineno)
		}
		argNode, err := p.parsePrimary(false)
		if err != nil {
			return nil, err
		}
		argNode, err = p.parsePostfix(argNode)
		if err != nil {
			return nil, err
		}
		args = []ast.Expr{argNode}
	}
	t := &ast.Test{Node: node, Name: name, Args: args, Kwargs: kwargs, DynArgs: dynA, DynKwargs: dynK}
	t.SetLineno(tok.Lineno)
	if negated {
		n := &ast.Not{}
		n.Op = "not"
		n.Node = t
		n.SetLineno(tok.Lineno)
		return n, nil
	}
	return t, nil
}

// testStartCanFollow returns true if cur can start a test argument
// (mirroring Python's set check on `name`/`string`/`integer`/`float`/
// `lparen`/`lbracket`/`lbrace`, excluding else/or/and).
func testStartCanFollow(cur lexer.Token) bool {
	switch cur.Kind {
	case lexer.TokenName:
		switch cur.Value {
		case "else", "or", "and":
			return false
		}
		return true
	case lexer.TokenString, lexer.TokenInteger, lexer.TokenFloat,
		lexer.TokenLParen, lexer.TokenLBracket, lexer.TokenLBrace:
		return true
	}
	return false
}

// =============================================================== helpers

func newBinop(k lexer.TokenKind, left, right ast.Expr, lineno int) ast.Expr {
	switch k {
	case lexer.TokenAdd:
		n := &ast.Add{}
		n.Op = "+"
		n.Left = left
		n.Right = right
		n.SetLineno(lineno)
		return n
	case lexer.TokenSub:
		n := &ast.Sub{}
		n.Op = "-"
		n.Left = left
		n.Right = right
		n.SetLineno(lineno)
		return n
	case lexer.TokenMul:
		n := &ast.Mul{}
		n.Op = "*"
		n.Left = left
		n.Right = right
		n.SetLineno(lineno)
		return n
	case lexer.TokenDiv:
		n := &ast.Div{}
		n.Op = "/"
		n.Left = left
		n.Right = right
		n.SetLineno(lineno)
		return n
	case lexer.TokenFloorDiv:
		n := &ast.FloorDiv{}
		n.Op = "//"
		n.Left = left
		n.Right = right
		n.SetLineno(lineno)
		return n
	case lexer.TokenMod:
		n := &ast.Mod{}
		n.Op = "%"
		n.Left = left
		n.Right = right
		n.SetLineno(lineno)
		return n
	}
	return nil
}

func isAllSpace(s string) bool {
	for _, r := range s {
		switch r {
		case ' ', '\t', '\n', '\r', '\v', '\f':
		default:
			return false
		}
	}
	return true
}
