package ast

import (
	"fmt"
	"strings"

	"github.com/jryberg/gojinja/pkg/runtime"
)

// compareConst applies a Python-style comparison op to two folded values.
// Returns ErrImpossible if we don't model the comparison.
func compareConst(op string, a, b any) (bool, error) {
	switch op {
	case "eq":
		return equalConst(a, b)
	case "ne":
		eq, err := equalConst(a, b)
		return !eq, err
	case "lt", "lteq", "gt", "gteq":
		return orderConst(op, a, b)
	case "in":
		return inConst(a, b)
	case "notin":
		ok, err := inConst(a, b)
		return !ok, err
	}
	return false, fmt.Errorf("ast: unknown compare op %q: %w", op, ErrImpossible)
}

// equalConst folds == for numbers (compared exactly, as Python does)
// and strings; anything else is left to the evaluator.
func equalConst(a, b any) (bool, error) {
	if c, nan, ok := runtime.CompareNumbers(a, b); ok {
		return !nan && c == 0, nil
	}
	if as, ok := a.(string); ok {
		if bs, ok := b.(string); ok {
			return as == bs, nil
		}
	}
	return false, ErrImpossible
}

// orderConst applies <, <=, >, >= to numbers or strings.
func orderConst(op string, a, b any) (bool, error) {
	c, ok := 0, false
	if cmp, nan, isNum := runtime.CompareNumbers(a, b); isNum {
		if nan {
			return false, nil
		}
		c, ok = cmp, true
	} else if as, aok := a.(string); aok {
		if bs, bok := b.(string); bok {
			c, ok = strings.Compare(as, bs), true
		}
	}
	if !ok {
		return false, ErrImpossible
	}
	switch op {
	case "lt":
		return c < 0, nil
	case "lteq":
		return c <= 0, nil
	case "gt":
		return c > 0, nil
	}
	return c >= 0, nil
}

// inConst folds `a in b` for strings; other containers are left to the
// evaluator.
func inConst(a, b any) (bool, error) {
	s, ok := a.(string)
	col, colOK := b.(string)
	if !ok || !colOK {
		return false, ErrImpossible
	}
	return strings.Contains(col, s), nil
}
