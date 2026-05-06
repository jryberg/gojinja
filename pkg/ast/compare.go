package ast

import (
	"fmt"
	"reflect"
)

// compareConst applies a Python-style comparison op to two folded values.
// Returns ErrImpossible if we don't model the comparison.
func compareConst(op string, a, b any) (bool, error) {
	switch op {
	case "eq":
		return equalConst(a, b), nil
	case "ne":
		return !equalConst(a, b), nil
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

// equalConst is a Python-eq style equality check that treats numerics
// across int/float as equal when their numeric value is equal.
func equalConst(a, b any) bool {
	if af, aok := numAsFloat(a); aok {
		if bf, bok := numAsFloat(b); bok {
			return af == bf
		}
	}
	return reflect.DeepEqual(a, b)
}

// orderConst applies <, <=, >, >= to numerics or strings.
func orderConst(op string, a, b any) (bool, error) {
	if af, aok := numAsFloat(a); aok {
		if bf, bok := numAsFloat(b); bok {
			switch op {
			case "lt":
				return af < bf, nil
			case "lteq":
				return af <= bf, nil
			case "gt":
				return af > bf, nil
			case "gteq":
				return af >= bf, nil
			}
		}
	}
	if as, aok := a.(string); aok {
		if bs, bok := b.(string); bok {
			switch op {
			case "lt":
				return as < bs, nil
			case "lteq":
				return as <= bs, nil
			case "gt":
				return as > bs, nil
			case "gteq":
				return as >= bs, nil
			}
		}
	}
	return false, ErrImpossible
}

// inConst checks `a in b` for the b types we support.
func inConst(a, b any) (bool, error) {
	switch col := b.(type) {
	case string:
		s, ok := a.(string)
		if !ok {
			return false, ErrImpossible
		}
		return len(s) > 0 && len(col) >= len(s) && stringContains(col, s), nil
	case []any:
		for _, it := range col {
			if equalConst(a, it) {
				return true, nil
			}
		}
		return false, nil
	case map[any]any:
		_, ok := col[a]
		return ok, nil
	}
	return false, ErrImpossible
}

// stringContains is exposed so tests can mock it; tiny wrapper around
// strings.Contains kept here to avoid an import cycle.
func stringContains(haystack, needle string) bool {
	if needle == "" {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

// numAsFloat widens any modeled numeric to float64.
func numAsFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case int64:
		return float64(x), true
	case float64:
		return x, true
	}
	if i, ok := asInt64(v); ok {
		return float64(i), true
	}
	return 0, false
}
