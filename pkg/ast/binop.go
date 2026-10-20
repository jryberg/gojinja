package ast

import (
	"strings"

	"github.com/jryberg/gojinja/pkg/runtime"
)

// binexprBase is embedded by every binary-operator node. The Op field is
// also a const class attribute on the concrete node, but having it here
// lets folder code recover the operator with a single field read.
type binexprBase struct {
	exprBase
	Op    string
	Left  Expr
	Right Expr
}

// asConstBinop folds left & right and applies op.
func (b binexprBase) asConstBinop(ctx EvalCtx) (any, error) {
	lv, err := b.Left.AsConst(ctx)
	if err != nil {
		return nil, err
	}
	rv, err := b.Right.AsConst(ctx)
	if err != nil {
		return nil, err
	}
	return applyBinop(b.Op, lv, rv)
}

// Add — `a + b`.
type Add struct{ binexprBase }

// AsConst folds with `+`.
func (a *Add) AsConst(ctx EvalCtx) (any, error) { a.Op = "+"; return a.asConstBinop(ctx) }

// Sub — `a - b`.
type Sub struct{ binexprBase }

// AsConst folds with `-`.
func (s *Sub) AsConst(ctx EvalCtx) (any, error) { s.Op = "-"; return s.asConstBinop(ctx) }

// Mul — `a * b`.
type Mul struct{ binexprBase }

// AsConst folds with `*`.
func (m *Mul) AsConst(ctx EvalCtx) (any, error) { m.Op = "*"; return m.asConstBinop(ctx) }

// Div — `a / b` (Python true division: int/int = float).
type Div struct{ binexprBase }

// AsConst folds with `/`.
func (d *Div) AsConst(ctx EvalCtx) (any, error) { d.Op = "/"; return d.asConstBinop(ctx) }

// FloorDiv — `a // b`.
type FloorDiv struct{ binexprBase }

// AsConst folds with `//`.
func (f *FloorDiv) AsConst(ctx EvalCtx) (any, error) { f.Op = "//"; return f.asConstBinop(ctx) }

// Mod — `a % b`.
type Mod struct{ binexprBase }

// AsConst folds with `%`.
func (m *Mod) AsConst(ctx EvalCtx) (any, error) { m.Op = "%"; return m.asConstBinop(ctx) }

// Pow — `a ** b`.
type Pow struct{ binexprBase }

// AsConst folds with `**`.
func (p *Pow) AsConst(ctx EvalCtx) (any, error) { p.Op = "**"; return p.asConstBinop(ctx) }

// And — `a and b`. Short-circuits.
type And struct{ binexprBase }

// AsConst short-circuits on the left operand's truthiness.
func (a *And) AsConst(ctx EvalCtx) (any, error) {
	a.Op = "and"
	lv, err := a.Left.AsConst(ctx)
	if err != nil {
		return nil, err
	}
	if !truthy(lv) {
		return lv, nil
	}
	return a.Right.AsConst(ctx)
}

// Or — `a or b`. Short-circuits.
type Or struct{ binexprBase }

// AsConst short-circuits on the left operand's truthiness.
func (o *Or) AsConst(ctx EvalCtx) (any, error) {
	o.Op = "or"
	lv, err := o.Left.AsConst(ctx)
	if err != nil {
		return nil, err
	}
	if truthy(lv) {
		return lv, nil
	}
	return o.Right.AsConst(ctx)
}

// applyBinop performs a Python-compatible binary op on two folded values.
// Returns ErrImpossible for any combination we don't fully model.
func applyBinop(op string, a, b any) (any, error) {
	// String + string concatenation when both are strings.
	if op == "+" {
		if as, aok := a.(string); aok {
			if bs, bok := b.(string); bok {
				return as + bs, nil
			}
		}
	}
	// String * int repetition.
	if op == "*" {
		if s, ok := a.(string); ok {
			if n, ok := b.(int64); ok {
				if n < 0 {
					return "", nil
				}
				return strings.Repeat(s, int(n)), nil
			}
		}
		if n, ok := a.(int64); ok {
			if s, ok := b.(string); ok {
				if n < 0 {
					return "", nil
				}
				return strings.Repeat(s, int(n)), nil
			}
		}
	}
	// Numbers fold with the evaluator's arithmetic; anything it would
	// reject is left for render time to raise.
	v, err := runtime.NumBinop(op, a, b)
	if err != nil {
		return nil, ErrImpossible
	}
	return v, nil
}
