// Package optimizer constant-folds AST expressions where evaluation is
// guaranteed pure. Equivalent to Jinja2's optimizer.py.
//
// The pass is post-order: children are folded first, then the parent is
// asked to fold via [ast.Expr.AsConst]. If folding succeeds and the
// produced value is safely representable as a [*ast.Const], the parent is
// rewritten in place. Volatile evaluation contexts opt the relevant nodes
// (TemplateData, MarkSafeIfAutoescape) out of folding so autoescape
// changes at runtime aren't bypassed.
package optimizer

import (
	"github.com/jryberg/gojinja/pkg/ast"
	"github.com/jryberg/gojinja/pkg/escape"
)

// Optimize returns a folded copy of the tree. The original is mutated in
// place: child slots that fold are replaced with [*ast.Const]. ctx
// supplies the evaluation context (autoescape / volatile flags) so
// [ast.TemplateData] and [ast.MarkSafeIfAutoescape] choose the right
// representation.
func Optimize(n ast.Node, ctx ast.EvalCtx) ast.Node {
	if n == nil {
		return nil
	}
	transformChildren(n, ctx)
	return tryFold(n, ctx)
}

// tryFold attempts to replace n with a Const when possible. Always returns
// a non-nil Node (n itself if the fold isn't possible).
func tryFold(n ast.Node, ctx ast.EvalCtx) ast.Node {
	expr, ok := n.(ast.Expr)
	if !ok {
		return n
	}
	// Already a Const — folding is a no-op.
	if _, ok := expr.(*ast.Const); ok {
		return expr
	}
	v, err := expr.AsConst(ctx)
	if err != nil {
		return n
	}
	if !hasSafeRepr(v) {
		return n
	}
	c := &ast.Const{Value: v}
	c.SetLineno(expr.Position().Lineno)
	return c
}

// hasSafeRepr reports whether v is a value the runtime would represent
// the same way after Const-wrapping. This rejects exotic Go types whose
// equality / rendering behaviour depends on type.
func hasSafeRepr(v any) bool {
	switch x := v.(type) {
	case nil, bool, string, int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64, float32, float64:
		return true
	case escape.Markup:
		return true
	case []any:
		for _, it := range x {
			if !hasSafeRepr(it) {
				return false
			}
		}
		return true
	case map[any]any:
		for k, val := range x {
			if !hasSafeRepr(k) || !hasSafeRepr(val) {
				return false
			}
		}
		return true
	case ast.SliceVal:
		return hasSafeRepr(x.Start) && hasSafeRepr(x.Stop) && hasSafeRepr(x.Step)
	}
	return false
}
