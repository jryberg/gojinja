package eval

import (
	"fmt"
	"math"
	"reflect"
	"strings"

	"github.com/jryberg/gojinja/pkg/runtime"
)

// applyBinop is the runtime equivalent of ast.applyBinop. We can't import
// the ast package's helper because it's lower-cased. The two stay in sync
// by virtue of mirroring jinja2's _binop_to_func table.
func applyBinop(op string, a, b any) (any, error) {
	if op == "+" {
		if as, ok := a.(string); ok {
			if bs, ok := b.(string); ok {
				return as + bs, nil
			}
		}
		if al, ok := asAnyList(a); ok {
			if bl, ok := asAnyList(b); ok {
				out := make([]any, 0, len(al)+len(bl))
				out = append(out, al...)
				out = append(out, bl...)
				// If either operand was a *PyList, the result keeps
				// list-mutability so downstream `{% do x.append %}`
				// works on the concatenation.
				if _, ok := a.(*runtime.PyList); ok {
					return runtime.NewPyList(out), nil
				}
				if _, ok := b.(*runtime.PyList); ok {
					return runtime.NewPyList(out), nil
				}
				return out, nil
			}
		}
	}
	if op == "*" {
		if s, ok := a.(string); ok {
			if n, ok := asInt(b); ok {
				if n < 0 {
					return "", nil
				}
				return strings.Repeat(s, n), nil
			}
		}
		if n, ok := asInt(a); ok {
			if s, ok := b.(string); ok {
				if n < 0 {
					return "", nil
				}
				return strings.Repeat(s, n), nil
			}
		}
	}
	la, lb, ok := numericPair(a, b)
	if !ok {
		return nil, fmt.Errorf("unsupported operand types for %s: %T and %T", op, a, b)
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
			return nil, fmt.Errorf("division by zero")
		}
		return toFloat(la) / toFloat(lb), nil
	case "//":
		if isZero(lb) {
			return nil, fmt.Errorf("integer division by zero")
		}
		return numFloorDiv(la, lb), nil
	case "%":
		if isZero(lb) {
			return nil, fmt.Errorf("modulo by zero")
		}
		return numMod(la, lb), nil
	case "**":
		return numPow(la, lb), nil
	}
	return nil, fmt.Errorf("unknown binary operator %q", op)
}

func negate(v any) (any, error) {
	switch x := v.(type) {
	case int:
		return -x, nil
	case int64:
		return -x, nil
	case float64:
		return -x, nil
	}
	return nil, fmt.Errorf("cannot negate %T", v)
}

// compareValues evaluates a single comparison op on two values.
func compareValues(op string, a, b any) (bool, error) {
	switch op {
	case "eq":
		return valuesEqual(a, b), nil
	case "ne":
		return !valuesEqual(a, b), nil
	case "lt", "lteq", "gt", "gteq":
		return orderValues(op, a, b)
	case "in":
		return inValue(a, b)
	case "notin":
		ok, err := inValue(a, b)
		return !ok, err
	}
	return false, fmt.Errorf("unknown compare op %q", op)
}

func valuesEqual(a, b any) bool {
	if af, ok := numAsFloat(a); ok {
		if bf, ok := numAsFloat(b); ok {
			return af == bf
		}
	}
	return reflect.DeepEqual(a, b)
}

func orderValues(op string, a, b any) (bool, error) {
	if af, ok := numAsFloat(a); ok {
		if bf, ok := numAsFloat(b); ok {
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
	if as, ok := a.(string); ok {
		if bs, ok := b.(string); ok {
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
	return false, fmt.Errorf("unsupported comparison %T %s %T", a, op, b)
}

func inValue(needle, haystack any) (bool, error) {
	switch col := haystack.(type) {
	case string:
		s, ok := needle.(string)
		if !ok {
			return false, fmt.Errorf("'in' string requires string left-hand")
		}
		return strings.Contains(col, s), nil
	case []any:
		return containsAny(needle, col), nil
	case *runtime.PyList:
		if col == nil {
			return false, nil
		}
		return containsAny(needle, col.Items()), nil
	case runtime.Tuple:
		return containsAny(needle, []any(col)), nil
	case map[string]any:
		s, ok := needle.(string)
		if !ok {
			return false, nil
		}
		_, ok = col[s]
		return ok, nil
	case map[any]any:
		_, ok := col[needle]
		return ok, nil
	case *runtime.OrderedDict:
		if col == nil {
			return false, nil
		}
		_, ok := col.Get(needle)
		return ok, nil
	case runtime.Undefined:
		// Mirrors Python: iterating Undefined (default mode) yields the
		// empty sequence, so `x in undef` is False. ModeStrict errors at
		// the iteration boundary instead.
		if _, err := col.Iter(); err != nil {
			return false, err
		}
		return false, nil
	}
	return false, fmt.Errorf("'in' on unsupported type %T", haystack)
}

func containsAny(needle any, col []any) bool {
	for _, it := range col {
		if valuesEqual(needle, it) {
			return true
		}
	}
	return false
}

// asAnyList unwraps the underlying []any view of list-like values:
// plain slices, *runtime.PyList, runtime.Tuple. Returns (nil, false)
// for everything else. Centralising the unwrap keeps in-template
// PyLists transparent to op evaluators that only need a slice view.
func asAnyList(v any) ([]any, bool) {
	switch x := v.(type) {
	case []any:
		return x, true
	case *runtime.PyList:
		if x == nil {
			return nil, false
		}
		return x.Items(), true
	case runtime.Tuple:
		return []any(x), true
	}
	return nil, false
}

// ------------------------------------------------------------ numeric helpers

func numericPair(a, b any) (any, any, bool) {
	if ai, ok := asInt64(a); ok {
		if bi, ok := asInt64(b); ok {
			return ai, bi, true
		}
		if bf, ok := asFloat(b); ok {
			return float64(ai), bf, true
		}
	}
	if af, ok := asFloat(a); ok {
		if bf, ok := asFloat(b); ok {
			return af, bf, true
		}
		if bi, ok := asInt64(b); ok {
			return af, float64(bi), true
		}
	}
	return nil, nil, false
}

func asInt(v any) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case int64:
		return int(x), true
	case float64:
		if x == math.Floor(x) {
			return int(x), true
		}
	}
	return 0, false
}

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
	}
	return 0, false
}

func asFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float32:
		return float64(x), true
	case float64:
		return x, true
	}
	return 0, false
}

func numAsFloat(v any) (float64, bool) {
	if i, ok := asInt64(v); ok {
		return float64(i), true
	}
	if f, ok := asFloat(v); ok {
		return f, true
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
	return toFloat(a) + toFloat(b)
}
func numSub(a, b any) any {
	if ai, ok := a.(int64); ok {
		if bi, ok := b.(int64); ok {
			return ai - bi
		}
	}
	return toFloat(a) - toFloat(b)
}
func numMul(a, b any) any {
	if ai, ok := a.(int64); ok {
		if bi, ok := b.(int64); ok {
			return ai * bi
		}
	}
	return toFloat(a) * toFloat(b)
}

func numFloorDiv(a, b any) any {
	if ai, ok := a.(int64); ok {
		if bi, ok := b.(int64); ok {
			q := ai / bi
			if (ai%bi != 0) && ((ai < 0) != (bi < 0)) {
				q--
			}
			return q
		}
	}
	return math.Floor(toFloat(a) / toFloat(b))
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
	af, bf := toFloat(a), toFloat(b)
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
	if f, ok := numAsFloat(v); ok {
		return f
	}
	return math.NaN()
}
