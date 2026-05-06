package ast

import (
	"github.com/jryberg/gojinja/pkg/escape"
)

// AssignContext is the role a [Name] / [Tuple] plays. The parser builds
// nodes with CtxLoad by default and rewrites to CtxStore / CtxParam when
// the node becomes an assignment target.
type AssignContext string

const (
	CtxLoad  AssignContext = "load"
	CtxStore AssignContext = "store"
	CtxParam AssignContext = "param"
)

// Name is a variable reference: `foo`, with a load/store/param context.
type Name struct {
	exprBase
	Name string
	Ctx  AssignContext
}

// CanAssign returns true unless the name is a reserved literal.
func (n *Name) CanAssign() bool {
	switch n.Name {
	case "true", "false", "none", "True", "False", "None":
		return false
	}
	return true
}

// NSRef is a namespace assignment target: `ns.attr` on the left of `=`.
type NSRef struct {
	exprBase
	Name string
	Attr string
}

// CanAssign — yes; the assignment runtime checks that Name resolves to a
// Namespace.
func (*NSRef) CanAssign() bool { return true }

// Const wraps a Go-side constant value: int, float64, string, bool, nil,
// list, map. Produced both by the parser and by the optimizer's folder.
type Const struct {
	literalBase
	Value any
}

// AsConst returns the wrapped value.
func (c *Const) AsConst(EvalCtx) (any, error) { return c.Value, nil }

// TemplateData is the literal text between tags. Auto-escape rules apply:
// when [EvalCtx.Autoescape] is on, the constant is a [escape.Markup].
type TemplateData struct {
	literalBase
	Data string
}

// AsConst returns either escape.Markup(Data) or Data depending on autoescape.
func (t *TemplateData) AsConst(ctx EvalCtx) (any, error) {
	if ctx.Volatile {
		return nil, ErrImpossible
	}
	if ctx.Autoescape {
		return escape.Markup(t.Data), nil
	}
	return t.Data, nil
}

// Tuple is a tuple literal: `(1, 2, 3)` or the implicit form `1, 2, 3`.
type Tuple struct {
	literalBase
	Items []Expr
	Ctx   AssignContext
}

// AsConst folds every item; ErrImpossible if any item can't be folded.
func (t *Tuple) AsConst(ctx EvalCtx) (any, error) {
	out := make([]any, 0, len(t.Items))
	for _, it := range t.Items {
		v, err := it.AsConst(ctx)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// CanAssign — only if every item is itself assignable.
func (t *Tuple) CanAssign() bool {
	for _, it := range t.Items {
		if !it.CanAssign() {
			return false
		}
	}
	return true
}

// List is a list literal: `[1, 2, 3]`.
type List struct {
	literalBase
	Items []Expr
}

// AsConst folds each element.
func (l *List) AsConst(ctx EvalCtx) (any, error) {
	out := make([]any, 0, len(l.Items))
	for _, it := range l.Items {
		v, err := it.AsConst(ctx)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// Dict is a dict literal: `{a: 1, b: 2}`. Items are [Pair] nodes.
type Dict struct {
	literalBase
	Items []*Pair
}

// AsConst returns a map[any]any if every key+value folds.
func (d *Dict) AsConst(ctx EvalCtx) (any, error) {
	out := make(map[any]any, len(d.Items))
	for _, p := range d.Items {
		k, err := p.Key.AsConst(ctx)
		if err != nil {
			return nil, err
		}
		v, err := p.Value.AsConst(ctx)
		if err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, nil
}

// Pair is a (key, value) entry inside a [Dict].
type Pair struct {
	helperBase
	Key   Expr
	Value Expr
}

// Keyword is a (name, value) entry passed as a keyword argument to a Call/Filter/Test.
type Keyword struct {
	helperBase
	Key   string
	Value Expr
}

// CondExpr is the inline-if expression: `a if test else b`.
type CondExpr struct {
	exprBase
	Test  Expr
	Expr1 Expr
	Expr2 Expr // may be nil
}

// AsConst evaluates the test; folds to whichever branch wins.
func (c *CondExpr) AsConst(ctx EvalCtx) (any, error) {
	tv, err := c.Test.AsConst(ctx)
	if err != nil {
		return nil, err
	}
	if truthy(tv) {
		return c.Expr1.AsConst(ctx)
	}
	if c.Expr2 == nil {
		return nil, ErrImpossible
	}
	return c.Expr2.AsConst(ctx)
}

// Filter applies `expr | name(args)`. When Node is nil the filter is the
// `{% filter %}` block form.
type Filter struct {
	exprBase
	Node      Expr // may be nil
	Name      string
	Args      []Expr
	Kwargs    []*Keyword
	DynArgs   Expr
	DynKwargs Expr
}

// Test applies `expr is name(args)`. Same shape as Filter.
type Test struct {
	exprBase
	Node      Expr
	Name      string
	Args      []Expr
	Kwargs    []*Keyword
	DynArgs   Expr
	DynKwargs Expr
}

// Call is a callable invocation: `obj(arg, kw=v, *a, **k)`.
type Call struct {
	exprBase
	Node      Expr
	Args      []Expr
	Kwargs    []*Keyword
	DynArgs   Expr
	DynKwargs Expr
}

// Getitem is subscript access: `obj[arg]`.
type Getitem struct {
	exprBase
	Node Expr
	Arg  Expr
	Ctx  AssignContext
}

// Getattr is attribute access: `obj.attr`.
type Getattr struct {
	exprBase
	Node Expr
	Attr string
	Ctx  AssignContext
}

// Slice is the `start:stop:step` argument inside a subscript.
type Slice struct {
	exprBase
	Start Expr
	Stop  Expr
	Step  Expr
}

// AsConst folds each part; returns a SliceVal struct so the caller can
// distinguish slice from regular index.
func (s *Slice) AsConst(ctx EvalCtx) (any, error) {
	c := func(e Expr) (any, error) {
		if e == nil {
			return nil, nil
		}
		return e.AsConst(ctx)
	}
	start, err := c(s.Start)
	if err != nil {
		return nil, err
	}
	stop, err := c(s.Stop)
	if err != nil {
		return nil, err
	}
	step, err := c(s.Step)
	if err != nil {
		return nil, err
	}
	return SliceVal{Start: start, Stop: stop, Step: step}, nil
}

// SliceVal is the folded form of a [Slice]. Each field is nil if absent.
type SliceVal struct {
	Start any
	Stop  any
	Step  any
}

// Concat is the result of `a ~ b ~ c` (string-concat operator).
type Concat struct {
	exprBase
	Nodes []Expr
}

// AsConst stringifies each operand and concatenates.
func (c *Concat) AsConst(ctx EvalCtx) (any, error) {
	var out string
	for _, n := range c.Nodes {
		v, err := n.AsConst(ctx)
		if err != nil {
			return nil, err
		}
		out += escape.SoftStr(v)
	}
	return out, nil
}

// Operand pairs an operator with its right-hand expression inside a
// chained [Compare].
type Operand struct {
	helperBase
	Op   string // "eq", "ne", "lt", "lteq", "gt", "gteq", "in", "notin"
	Expr Expr
}

// Compare implements chained comparisons: `a < b == c`.
type Compare struct {
	exprBase
	Expr Expr
	Ops  []*Operand
}

// AsConst walks the chain, short-circuiting on the first false.
func (c *Compare) AsConst(ctx EvalCtx) (any, error) {
	left, err := c.Expr.AsConst(ctx)
	if err != nil {
		return nil, err
	}
	for _, op := range c.Ops {
		right, err := op.Expr.AsConst(ctx)
		if err != nil {
			return nil, err
		}
		ok, err := compareConst(op.Op, left, right)
		if err != nil {
			return nil, err
		}
		if !ok {
			return false, nil
		}
		left = right
	}
	return true, nil
}

// EnvironmentAttribute reads a named attribute from the Environment.
type EnvironmentAttribute struct {
	exprBase
	Name string
}

// ExtensionAttribute reads a constant from a registered extension.
type ExtensionAttribute struct {
	exprBase
	Identifier string
	Name       string
}

// ImportedName is a placeholder referencing an imported Go function.
// gojinja doesn't reference Python module paths; the runtime gets the
// callable by Name.
type ImportedName struct {
	exprBase
	ImportName string
}

// InternalName is a parser-generated identifier hidden from templates.
// Use [NewInternalName] from the parser package; it is not constructible
// outside of that path.
type InternalName struct {
	exprBase
	Name string
}

// MarkSafe wraps an expression as Markup unconditionally.
type MarkSafe struct {
	exprBase
	Expr Expr
}

// AsConst folds the wrapped expression and wraps the result in Markup.
func (m *MarkSafe) AsConst(ctx EvalCtx) (any, error) {
	v, err := m.Expr.AsConst(ctx)
	if err != nil {
		return nil, err
	}
	return escape.Markup(escape.SoftStr(v)), nil
}

// MarkSafeIfAutoescape wraps as Markup only when autoescape is active.
type MarkSafeIfAutoescape struct {
	exprBase
	Expr Expr
}

// AsConst returns Markup(v) if autoescape, else v.
func (m *MarkSafeIfAutoescape) AsConst(ctx EvalCtx) (any, error) {
	if ctx.Volatile {
		return nil, ErrImpossible
	}
	v, err := m.Expr.AsConst(ctx)
	if err != nil {
		return nil, err
	}
	if ctx.Autoescape {
		return escape.Markup(escape.SoftStr(v)), nil
	}
	return v, nil
}

// ContextReference yields the active runtime context.
type ContextReference struct{ exprBase }

// DerivedContextReference yields the active context including local vars.
type DerivedContextReference struct{ exprBase }
