package ext

import (
	"fmt"
	"strings"

	"github.com/jryberg/gojinja/pkg/ast"
	gjerrors "github.com/jryberg/gojinja/pkg/errors"
	"github.com/jryberg/gojinja/pkg/lexer"
	"github.com/jryberg/gojinja/pkg/parser"
)

// Translator is the gettext-style backend the i18n extension calls into.
// Implementations must be safe for concurrent use across renders.
//
// gojinja's idiomatic-Go alternative to Python Jinja2's `gettext`-based
// integration. Instead of depending on the C `gettext` library, you
// supply this small interface backed by whatever translation source
// suits you (JSON, YAML, a database, a `golang.org/x/text` catalog).
//
// Use [TranslatorGlobals] to wire an implementation into an
// environment.
//
// Example minimal implementation:
//
//	type myTr struct{ table map[string]string }
//	func (t myTr) Gettext(m string) string {
//	    if v, ok := t.table[m]; ok { return v }
//	    return m
//	}
//	func (t myTr) NGettext(s, p string, n int) string {
//	    if n == 1 { return t.Gettext(s) }
//	    return t.Gettext(p)
//	}
//	func (t myTr) PGettext(c, m string) string  { return t.Gettext(c+":"+m) }
//	func (t myTr) NPGettext(c, s, p string, n int) string {
//	    if n == 1 { return t.PGettext(c, s) }
//	    return t.PGettext(c, p)
//	}
//
//	globals := ext.TranslatorGlobals(myTr{table: ...})
//	env, _ := environment.New(environment.WithGlobals(globals))
type Translator interface {
	// Gettext returns the translation of message in the current locale,
	// or message unchanged if untranslated.
	Gettext(message string) string
	// NGettext returns singular when n == 1 and plural otherwise (or
	// whatever the locale's plural rule prescribes).
	NGettext(singular, plural string, n int) string
	// PGettext returns the translation of message in the given context,
	// allowing different translations for the same string when used with
	// different meanings.
	PGettext(context, message string) string
	// NPGettext is the plural-aware counterpart of PGettext.
	NPGettext(context, singular, plural string, n int) string
}

// NullTranslator is the passthrough [Translator]: returns messages
// unchanged, uses simple `n==1` plural selection, ignores context. The
// natural default and the right choice for tests.
type NullTranslator struct{}

// Gettext returns message unchanged.
func (NullTranslator) Gettext(message string) string { return message }

// NGettext picks singular for n==1, otherwise plural.
func (NullTranslator) NGettext(singular, plural string, n int) string {
	if n == 1 {
		return singular
	}
	return plural
}

// PGettext returns message unchanged.
func (NullTranslator) PGettext(_ /*context*/, message string) string { return message }

// NPGettext is NGettext, ignoring context.
func (NullTranslator) NPGettext(_ /*context*/, singular, plural string, n int) string {
	if n == 1 {
		return singular
	}
	return plural
}

// I18N global names. They're registered as plain Go funcs on the
// environment so the parser-emitted Call expressions resolve at render
// time.
const (
	I18NGettextName   = "gettext"
	I18NNGettextName  = "ngettext"
	I18NPGettextName  = "pgettext"
	I18NNPGettextName = "npgettext"
	// I18NSubstName is the placeholder-substitution helper.
	I18NSubstName = "__gojinja_i18n_subst__"
	// I18NUnderscoreAlias is the conventional `_(msg)` alias for gettext.
	I18NUnderscoreAlias = "_"
)

// TranslatorGlobals returns the gettext-family functions backed by t.
// Pass these to the environment via WithGlobal (or use the convenience
// option in pkg/environment).
func TranslatorGlobals(t Translator) map[string]any {
	if t == nil {
		t = NullTranslator{}
	}
	return map[string]any{
		I18NGettextName:     t.Gettext,
		I18NNGettextName:    t.NGettext,
		I18NPGettextName:    t.PGettext,
		I18NNPGettextName:   t.NPGettext,
		I18NUnderscoreAlias: t.Gettext,
		I18NSubstName:       I18NSubst,
	}
}

// I18NSubst replaces `%(name)s` placeholders in s with the stringified
// values from vars. Accepts either map[string]any or map[any]any (the
// shape ast.Dict produces). Missing keys are left untouched — over-
// translating is harmless and avoids cascading errors during locale
// rollout.
func I18NSubst(s string, vars any) string {
	if !strings.Contains(s, "%(") {
		return s
	}
	lookup := normaliseVarMap(vars)
	var b strings.Builder
	i := 0
	for i < len(s) {
		if i+2 < len(s) && s[i] == '%' && s[i+1] == '(' {
			closing := strings.IndexByte(s[i+2:], ')')
			if closing >= 0 {
				name := s[i+2 : i+2+closing]
				typeIdx := i + 2 + closing + 1
				// Skip the conversion type byte (e.g. 's', 'd').
				if typeIdx < len(s) {
					if v, ok := lookup[name]; ok {
						b.WriteString(stringify(v))
						i = typeIdx + 1
						continue
					}
				}
			}
		}
		// Treat `%%` as a literal `%` (matches Python's `%` formatting).
		if i+1 < len(s) && s[i] == '%' && s[i+1] == '%' {
			b.WriteByte('%')
			i += 2
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// orderedDictReader is the duck-typing interface implemented by
// *runtime.OrderedDict — kept in pkg/ext to avoid importing the runtime
// package just for one type assertion.
type orderedDictReader interface {
	Keys() []any
	Get(any) (any, bool)
}

func normaliseVarMap(vars any) map[string]any {
	switch m := vars.(type) {
	case map[string]any:
		return m
	case map[any]any:
		out := make(map[string]any, len(m))
		for k, v := range m {
			if s, ok := k.(string); ok {
				out[s] = v
			}
		}
		return out
	case orderedDictReader:
		keys := m.Keys()
		out := make(map[string]any, len(keys))
		for _, k := range keys {
			if s, ok := k.(string); ok {
				v, _ := m.Get(k)
				out[s] = v
			}
		}
		return out
	}
	return nil
}

func stringify(v any) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case []byte:
		return string(x)
	}
	return fmt.Sprint(v)
}

// I18NTag is the parser hook for `{% trans %}`. It supports:
//   - context: `{% trans 'ctx' %}` → pgettext / npgettext.
//   - vars: `{% trans var=expr, count=expr %}` (vars become %(name)s).
//   - trimmed: `{% trans trimmed %}` strips and collapses whitespace.
//   - plural: `{% trans count=c %} sing {% pluralize %} plur {% endtrans %}`.
//
// The body's `{{ name }}` interpolations are rewritten as %(name)s
// placeholders that the gettext lookup preserves and the runtime subst
// helper fills in.
func I18NTag(p *parser.Parser) (ast.Node, error) {
	tagTok := p.Stream().Next()
	if tagTok.Value != "trans" {
		return nil, fmt.Errorf("ext.I18NTag: expected 'trans' tag, got %q", tagTok.Value)
	}
	lineno := tagTok.Lineno
	stream := p.Stream()

	var contextStr string
	hasContext := false
	if stream.Current().Kind == lexer.TokenString {
		hasContext = true
		contextStr = stream.Next().Value
	}

	variables := map[string]ast.Expr{}
	var firstVarName string
	trimmed := -1 // -1 unset, 0 false, 1 true
	for stream.Current().Kind != lexer.TokenBlockEnd {
		if len(variables) > 0 {
			if _, err := stream.Expect("comma"); err != nil {
				return nil, err
			}
		}
		if stream.SkipIf("colon") {
			break
		}
		nameTok, err := stream.Expect("name")
		if err != nil {
			return nil, err
		}
		// `trimmed`/`notrimmed` are modifiers, not variables.
		if trimmed == -1 && (nameTok.Value == "trimmed" || nameTok.Value == "notrimmed") {
			if nameTok.Value == "trimmed" {
				trimmed = 1
			} else {
				trimmed = 0
			}
			continue
		}
		if _, dup := variables[nameTok.Value]; dup {
			return nil, gjerrors.NewTemplateSyntaxError(
				fmt.Sprintf("translatable variable %q defined twice", nameTok.Value),
				nameTok.Lineno, stream.Name(), stream.Filename(),
			)
		}
		var expr ast.Expr
		if stream.Current().Kind == lexer.TokenAssign {
			stream.Next()
			expr, err = p.ParseExpressionPublic()
			if err != nil {
				return nil, err
			}
		} else {
			n := &ast.Name{Name: nameTok.Value, Ctx: ast.CtxLoad}
			n.SetLineno(nameTok.Lineno)
			expr = n
		}
		variables[nameTok.Value] = expr
		if firstVarName == "" {
			firstVarName = nameTok.Value
		}
	}
	if _, err := stream.Expect("block_end"); err != nil {
		return nil, err
	}

	// Parse singular body until `{% pluralize %}` or `{% endtrans %}`.
	singularRefs, singular, hitPluralize, err := parseTransBlock(p, true)
	if err != nil {
		return nil, err
	}
	for _, name := range singularRefs {
		if _, ok := variables[name]; !ok {
			n := &ast.Name{Name: name, Ctx: ast.CtxLoad}
			variables[name] = n
		}
	}

	var plural string
	var pluralExprName string
	hasPlural := false
	if hitPluralize {
		// Consume `pluralize` token.
		if _, err := stream.Expect("name"); err != nil {
			return nil, err
		}
		// Optional explicit plural-count variable name.
		if stream.Current().Kind != lexer.TokenBlockEnd {
			tok, err := stream.Expect("name")
			if err != nil {
				return nil, err
			}
			if _, ok := variables[tok.Value]; !ok {
				return nil, gjerrors.NewTemplateSyntaxError(
					fmt.Sprintf("unknown variable %q for pluralization", tok.Value),
					tok.Lineno, stream.Name(), stream.Filename(),
				)
			}
			pluralExprName = tok.Value
		}
		if _, err := stream.Expect("block_end"); err != nil {
			return nil, err
		}
		var pluralRefs []string
		pluralRefs, plural, _, err = parseTransBlock(p, false)
		if err != nil {
			return nil, err
		}
		for _, name := range pluralRefs {
			if _, ok := variables[name]; !ok {
				n := &ast.Name{Name: name, Ctx: ast.CtxLoad}
				variables[name] = n
			}
		}
		// Consume `endtrans`.
		if _, err := stream.Expect("name"); err != nil {
			return nil, err
		}
		hasPlural = true
	} else {
		// Consume `endtrans`.
		if _, err := stream.Expect("name"); err != nil {
			return nil, err
		}
	}

	if trimmed == 1 {
		singular = trimWhitespace(singular)
		if plural != "" {
			plural = trimWhitespace(plural)
		}
	}

	// Pick the gettext function based on context + plural.
	funcName := I18NGettextName
	if hasContext {
		funcName = I18NPGettextName
	}
	if hasPlural {
		funcName = "n" + funcName
	}

	// Build args: [context?, singular, plural?, n?]
	args := []ast.Expr{}
	if hasContext {
		c := &ast.Const{Value: contextStr}
		c.SetLineno(lineno)
		args = append(args, c)
	}
	cs := &ast.Const{Value: singular}
	cs.SetLineno(lineno)
	args = append(args, cs)
	if hasPlural {
		cp := &ast.Const{Value: plural}
		cp.SetLineno(lineno)
		args = append(args, cp)
		// Pick the plural-count expression: explicit name from
		// `{% pluralize %}` if present, else first declared variable.
		var nExpr ast.Expr
		if pluralExprName != "" {
			nExpr = variables[pluralExprName]
		} else if firstVarName != "" {
			nExpr = variables[firstVarName]
		}
		if nExpr == nil {
			return nil, gjerrors.NewTemplateSyntaxError(
				"pluralize without variables", lineno, stream.Name(), stream.Filename(),
			)
		}
		args = append(args, nExpr)
	}

	gettextCall := &ast.Call{
		Node: &ast.Name{Name: funcName, Ctx: ast.CtxLoad},
		Args: args,
	}
	gettextCall.SetLineno(lineno)

	// Wrap with substitution if there are variables to expand.
	var resultExpr ast.Expr
	if len(variables) > 0 {
		// Build the var dict.
		dictItems := make([]*ast.Pair, 0, len(variables))
		for name, expr := range variables {
			dictItems = append(dictItems, &ast.Pair{
				Key:   &ast.Const{Value: name},
				Value: expr,
			})
		}
		dict := &ast.Dict{Items: dictItems}
		dict.SetLineno(lineno)
		subst := &ast.Call{
			Node: &ast.Name{Name: I18NSubstName, Ctx: ast.CtxLoad},
			Args: []ast.Expr{gettextCall, dict},
		}
		subst.SetLineno(lineno)
		// Mark safe-if-autoescape so translated strings respect the
		// caller's autoescape policy without double-escaping the
		// substituted values.
		safe := &ast.MarkSafeIfAutoescape{Expr: subst}
		safe.SetLineno(lineno)
		resultExpr = safe
	} else {
		safe := &ast.MarkSafeIfAutoescape{Expr: gettextCall}
		safe.SetLineno(lineno)
		resultExpr = safe
	}

	out := &ast.Output{Nodes: []ast.Expr{resultExpr}}
	out.SetLineno(lineno)
	return out, nil
}

// parseTransBlock collects literal data and `{{ name }}` substitutions
// from the body of a trans/pluralize block. Returns the list of
// referenced names, the assembled body (with `%(name)s` placeholders),
// and whether the terminator was `{% pluralize %}` (vs `{% endtrans %}`).
func parseTransBlock(p *parser.Parser, allowPluralize bool) (referenced []string, body string, hitPluralize bool, err error) {
	stream := p.Stream()
	var b strings.Builder
	for {
		cur := stream.Current()
		switch cur.Kind {
		case lexer.TokenData:
			b.WriteString(strings.ReplaceAll(cur.Value, "%", "%%"))
			stream.Next()
		case lexer.TokenVariableBegin:
			stream.Next()
			nameTok, e := stream.Expect("name")
			if e != nil {
				err = e
				return
			}
			referenced = append(referenced, nameTok.Value)
			fmt.Fprintf(&b, "%%(%s)s", nameTok.Value)
			if _, e := stream.Expect("variable_end"); e != nil {
				err = e
				return
			}
		case lexer.TokenBlockBegin:
			stream.Next()
			if stream.Current().Kind != lexer.TokenName {
				err = gjerrors.NewTemplateSyntaxError(
					"control structures in translatable sections are not allowed",
					cur.Lineno, stream.Name(), stream.Filename(),
				)
				return
			}
			tagName := stream.Current().Value
			switch tagName {
			case "endtrans":
				body = b.String()
				return
			case "pluralize":
				if !allowPluralize {
					err = gjerrors.NewTemplateSyntaxError(
						"a translatable section can have only one pluralize section",
						cur.Lineno, stream.Name(), stream.Filename(),
					)
					return
				}
				body = b.String()
				hitPluralize = true
				return
			case "trans":
				err = gjerrors.NewTemplateSyntaxError(
					"trans blocks can't be nested; did you mean `endtrans`?",
					cur.Lineno, stream.Name(), stream.Filename(),
				)
				return
			default:
				err = gjerrors.NewTemplateSyntaxError(
					fmt.Sprintf("control structures in translatable sections are not allowed; saw %q", tagName),
					cur.Lineno, stream.Name(), stream.Filename(),
				)
				return
			}
		case lexer.TokenEOF:
			err = gjerrors.NewTemplateSyntaxError(
				"unclosed translation block", cur.Lineno, stream.Name(), stream.Filename(),
			)
			return
		default:
			err = gjerrors.NewTemplateSyntaxError(
				fmt.Sprintf("unexpected token %q in translation block", cur.String()),
				cur.Lineno, stream.Name(), stream.Filename(),
			)
			return
		}
	}
}

// trimWhitespace collapses runs of whitespace around newlines into a
// single space, matching Jinja2's _ws_re behaviour.
func trimWhitespace(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	inWS := false
	for _, r := range s {
		switch r {
		case ' ', '\t', '\n', '\r':
			if !inWS {
				b.WriteByte(' ')
				inWS = true
			}
		default:
			b.WriteRune(r)
			inWS = false
		}
	}
	return b.String()
}
