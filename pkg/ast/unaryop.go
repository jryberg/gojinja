package ast

import "fmt"

// unaryexprBase is embedded by every unary-op node.
type unaryexprBase struct {
	exprBase
	Op   string
	Node Expr
}

// asConstUnary folds Node and applies Op.
func (u unaryexprBase) asConstUnary(ctx EvalCtx) (any, error) {
	v, err := u.Node.AsConst(ctx)
	if err != nil {
		return nil, err
	}
	return applyUnaryop(u.Op, v)
}

// Not — `not x`.
type Not struct{ unaryexprBase }

// AsConst returns the boolean negation.
func (n *Not) AsConst(ctx EvalCtx) (any, error) {
	n.Op = "not"
	v, err := n.Node.AsConst(ctx)
	if err != nil {
		return nil, err
	}
	return !truthy(v), nil
}

// Neg — `-x`.
type Neg struct{ unaryexprBase }

// AsConst negates a numeric.
func (n *Neg) AsConst(ctx EvalCtx) (any, error) { n.Op = "-"; return n.asConstUnary(ctx) }

// UAdd — `+x` (unary plus). No-op for numerics. Named to avoid collision
// with [Pos] (the source-position struct).
type UAdd struct{ unaryexprBase }

// AsConst returns the value unchanged for numerics.
func (p *UAdd) AsConst(ctx EvalCtx) (any, error) { p.Op = "+"; return p.asConstUnary(ctx) }

// applyUnaryop applies a unary operator to a folded value.
func applyUnaryop(op string, v any) (any, error) {
	switch op {
	case "not":
		return !truthy(v), nil
	case "+":
		// Python `+x` returns x for numeric types.
		switch x := v.(type) {
		case int64, float64:
			return x, nil
		}
		return nil, ErrImpossible
	case "-":
		switch x := v.(type) {
		case int64:
			return -x, nil
		case float64:
			return -x, nil
		}
		return nil, ErrImpossible
	}
	return nil, fmt.Errorf("ast: unknown unary operator %q: %w", op, ErrImpossible)
}

// truthy mirrors Python's bool(x) for the value types we model.
func truthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case int64:
		return x != 0
	case float64:
		return x != 0
	case string:
		return len(x) != 0
	case []any:
		return len(x) != 0
	case map[any]any:
		return len(x) != 0
	}
	return true
}
