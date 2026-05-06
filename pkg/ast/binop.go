package ast

import (
	"fmt"
	"math"
	"strings"
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
	la, lb, ok := numericPair(a, b)
	if !ok {
		return nil, ErrImpossible
	}
	switch op {
	case "+":
		return numAdd(la, lb), nil
	case "-":
		return numSub(la, lb), nil
	case "*":
		return numMul(la, lb), nil
	case "/":
		if isZero(lb) {
			return nil, ErrImpossible
		}
		return numDiv(la, lb), nil
	case "//":
		if isZero(lb) {
			return nil, ErrImpossible
		}
		return numFloorDiv(la, lb), nil
	case "%":
		if isZero(lb) {
			return nil, ErrImpossible
		}
		return numMod(la, lb), nil
	case "**":
		return numPow(la, lb), nil
	}
	return nil, fmt.Errorf("ast: unknown binary operator %q: %w", op, ErrImpossible)
}

// numericPair coerces both values to either two int64s or two float64s.
// The first return is the chosen widened pair; the second is true when
// the coercion succeeded.
func numericPair(a, b any) (any, any, bool) {
	if ai, ok := a.(int64); ok {
		if bi, ok := b.(int64); ok {
			return ai, bi, true
		}
		if bf, ok := b.(float64); ok {
			return float64(ai), bf, true
		}
	}
	if af, ok := a.(float64); ok {
		if bi, ok := b.(int64); ok {
			return af, float64(bi), true
		}
		if bf, ok := b.(float64); ok {
			return af, bf, true
		}
	}
	if ai, ok := asInt64(a); ok {
		if bi, ok := asInt64(b); ok {
			return ai, bi, true
		}
	}
	return nil, nil, false
}

// asInt64 attempts to widen a Go integer of any size into int64.
func asInt64(v any) (int64, bool) {
	switch x := v.(type) {
	case int:
		return int64(x), true
	case int8:
		return int64(x), true
	case int16:
		return int64(x), true
	case int32:
		return int64(x), true
	case int64:
		return x, true
	case uint:
		return int64(x), true
	case uint8:
		return int64(x), true
	case uint16:
		return int64(x), true
	case uint32:
		return int64(x), true
	case uint64:
		if x > math.MaxInt64 {
			return 0, false
		}
		return int64(x), true
	}
	return 0, false
}

func isZero(v any) bool {
	switch x := v.(type) {
	case int64:
		return x == 0
	case float64:
		return x == 0
	}
	return false
}

func numAdd(a, b any) any {
	if ai, ok := a.(int64); ok {
		if bi, ok := b.(int64); ok {
			return ai + bi
		}
	}
	return a.(float64) + b.(float64)
}

func numSub(a, b any) any {
	if ai, ok := a.(int64); ok {
		if bi, ok := b.(int64); ok {
			return ai - bi
		}
	}
	return a.(float64) - b.(float64)
}

func numMul(a, b any) any {
	if ai, ok := a.(int64); ok {
		if bi, ok := b.(int64); ok {
			return ai * bi
		}
	}
	return a.(float64) * b.(float64)
}

// numDiv mirrors Python's `/`: int / int returns float.
func numDiv(a, b any) any {
	af := toFloat(a)
	bf := toFloat(b)
	return af / bf
}

func numFloorDiv(a, b any) any {
	if ai, ok := a.(int64); ok {
		if bi, ok := b.(int64); ok {
			// Python floor-div rounds toward -inf.
			q := ai / bi
			if (ai%bi != 0) && ((ai < 0) != (bi < 0)) {
				q--
			}
			return q
		}
	}
	af := toFloat(a)
	bf := toFloat(b)
	return math.Floor(af / bf)
}

func numMod(a, b any) any {
	if ai, ok := a.(int64); ok {
		if bi, ok := b.(int64); ok {
			r := ai % bi
			if (r != 0) && ((r < 0) != (bi < 0)) {
				r += bi
			}
			return r
		}
	}
	af := toFloat(a)
	bf := toFloat(b)
	r := math.Mod(af, bf)
	if (r != 0) && ((r < 0) != (bf < 0)) {
		r += bf
	}
	return r
}

func numPow(a, b any) any {
	if ai, ok := a.(int64); ok {
		if bi, ok := b.(int64); ok && bi >= 0 {
			r := int64(1)
			x := ai
			for i := bi; i > 0; i >>= 1 {
				if i&1 == 1 {
					r *= x
				}
				x *= x
			}
			return r
		}
	}
	return math.Pow(toFloat(a), toFloat(b))
}

func toFloat(v any) float64 {
	switch x := v.(type) {
	case int64:
		return float64(x)
	case float64:
		return x
	}
	return math.NaN()
}
