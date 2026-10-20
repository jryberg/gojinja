package eval

import (
	"fmt"
	"math"
	"strings"

	"github.com/jryberg/gojinja/pkg/escape"
	"github.com/jryberg/gojinja/pkg/runtime"
)

// applyBinop is the runtime equivalent of ast.applyBinop. We can't import
// the ast package's helper because it's lower-cased. The two stay in sync
// by virtue of mirroring jinja2's _binop_to_func table.
func applyBinop(op string, a, b any) (any, error) {
	// str % args is printf-style formatting; it accepts any right operand,
	// an Undefined included.
	if op == "%" {
		switch f := a.(type) {
		case string:
			return PercentFormat(f, b, false)
		case escape.Markup:
			s, err := PercentFormat(string(f), b, true)
			return escape.Markup(s), err
		}
	}
	if err := FailUndefined(a, b); err != nil {
		return nil, err
	}
	// Python's bool is an int subclass in arithmetic.
	a, b = boolToInt(a), boolToInt(b)
	if out, ok := markupBinop(op, a, b); ok {
		return out, nil
	}
	if ab, ok := a.(runtime.Bytes); ok {
		switch bb := b.(type) {
		case runtime.Bytes:
			if op == "+" {
				return append(append(runtime.Bytes{}, ab...), bb...), nil
			}
		default:
			if n, ok := asInt(b); ok && op == "*" {
				return runtime.Bytes(strings.Repeat(string(ab), max(n, 0))), nil
			}
		}
	}
	if op == "+" {
		if as, ok := a.(string); ok {
			if bs, ok := b.(string); ok {
				return as + bs, nil
			}
		}
		_, at := a.(runtime.Tuple)
		_, bt := b.(runtime.Tuple)
		if al, ok := asAnyList(a); ok {
			if bl, ok := asAnyList(b); ok {
				if at != bt {
					return nil, fmt.Errorf("can only concatenate %s (not %q) to %s", seqTypeName(a), seqTypeName(b), seqTypeName(a))
				}
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
				if at {
					return runtime.Tuple(out), nil
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
		if n, ok := asInt(b); ok {
			if out, ok := repeatSeq(a, n); ok {
				return out, nil
			}
		}
		if n, ok := asInt(a); ok {
			if out, ok := repeatSeq(b, n); ok {
				return out, nil
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

func boolToInt(v any) any {
	if b, ok := v.(bool); ok {
		if b {
			return int64(1)
		}
		return int64(0)
	}
	return v
}

// markupBinop applies markupsafe's Markup operators: `+` escapes the str
// operand and `*` repeats, both returning Markup.
func markupBinop(op string, a, b any) (any, bool) {
	am, aIsMarkup := a.(escape.Markup)
	bm, bIsMarkup := b.(escape.Markup)
	if !aIsMarkup && !bIsMarkup {
		return nil, false
	}
	switch op {
	case "+":
		if aIsMarkup {
			if _, ok := eqStr(b); ok {
				return am + escape.Escape(b), true
			}
		} else if _, ok := a.(string); ok {
			return escape.Escape(a) + bm, true
		}
	case "*":
		m := am
		n, ok := asInt(b)
		if !aIsMarkup {
			m = bm
			n, ok = asInt(a)
		}
		if ok {
			if n < 0 {
				n = 0
			}
			return escape.Markup(strings.Repeat(string(m), n)), true
		}
	}
	return nil, false
}

// repeatSeq implements Python's `seq * n` for lists and tuples, keeping
// the operand's type. Reports false when seq is not list-like.
func repeatSeq(seq any, n int) (any, bool) {
	items, ok := asAnyList(seq)
	if !ok {
		return nil, false
	}
	var out []any
	if n > 0 {
		out = make([]any, 0, len(items)*n)
		for i := 0; i < n; i++ {
			out = append(out, items...)
		}
	} else {
		out = []any{}
	}
	switch seq.(type) {
	case runtime.Tuple:
		return runtime.Tuple(out), true
	case *runtime.PyList:
		return runtime.NewPyList(out), true
	}
	return out, true
}

// seqTypeName is the Python type name of a list-like operand.
func seqTypeName(v any) string {
	if _, ok := v.(runtime.Tuple); ok {
		return "tuple"
	}
	return "list"
}

// FailUndefined returns the undefined error of the first Undefined in
// vals. Python's Undefined fails every arithmetic and ordering operator
// in all modes.
func FailUndefined(vals ...any) error {
	for _, v := range vals {
		if u, ok := v.(runtime.Undefined); ok {
			return u.Fail()
		}
	}
	return nil
}

// FailStrict returns the undefined error when v is a StrictUndefined,
// whose __str__, __bool__ and __iter__ all raise in Python.
func FailStrict(v any) error {
	if u, ok := v.(runtime.Undefined); ok && u.Mode == runtime.ModeStrict {
		return u.Fail()
	}
	return nil
}

// truth is [Truthy] for template conditions: a StrictUndefined raises.
func truth(v any) (bool, error) {
	if err := FailStrict(v); err != nil {
		return false, err
	}
	return Truthy(v), nil
}

func negate(v any) (any, error) {
	if err := FailUndefined(v); err != nil {
		return nil, err
	}
	switch x := v.(type) {
	case bool:
		if x {
			return int64(-1), nil
		}
		return int64(0), nil
	case int:
		return -x, nil
	case int64:
		return -x, nil
	case float64:
		return -x, nil
	}
	return nil, fmt.Errorf("cannot negate %T", v)
}

// positive implements unary `+`.
func positive(v any) (any, error) {
	if err := FailUndefined(v); err != nil {
		return nil, err
	}
	switch x := v.(type) {
	case bool:
		if x {
			return int64(1), nil
		}
		return int64(0), nil
	case int, int64, float64:
		return x, nil
	}
	return nil, fmt.Errorf("bad operand type for unary +: %T", v)
}

// compareValues evaluates a single comparison op on two values.
func compareValues(op string, a, b any) (bool, error) {
	switch op {
	case "eq":
		return ValuesEqual(a, b)
	case "ne":
		eq, err := ValuesEqual(a, b)
		return !eq, err
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

var orderSymbols = map[string]string{"lt": "<", "lteq": "<=", "gt": ">", "gteq": ">="}

// orderValues implements Python's ordering operators: numbers (bool
// included) compare by value, str and Markup by text, and lists with
// lists or tuples with tuples lexicographically. Anything else raises
// Python's TypeError.
func orderValues(op string, a, b any) (bool, error) {
	return orderDepth(op, a, b, 0)
}

func orderDepth(op string, a, b any, depth int) (bool, error) {
	if depth > maxEqualDepth {
		return false, fmt.Errorf("maximum recursion depth exceeded in comparison")
	}
	if err := FailUndefined(a, b); err != nil {
		return false, err
	}
	if ai, ok := eqInt(a); ok {
		if bi, ok := eqInt(b); ok {
			return cmpOrdered(op, ai, bi), nil
		}
	}
	if af, ok := eqFloat(a); ok {
		if bf, ok := eqFloat(b); ok {
			return cmpOrdered(op, af, bf), nil
		}
	}
	if as, ok := eqStr(a); ok {
		if bs, ok := eqStr(b); ok {
			return cmpOrdered(op, as, bs), nil
		}
	}
	if al, aTuple, ok := eqSeq(a); ok {
		if bl, bTuple, ok := eqSeq(b); ok && aTuple == bTuple {
			for i := 0; i < len(al) && i < len(bl); i++ {
				eq, err := elemEqual(al[i], bl[i], depth)
				if err != nil {
					return false, err
				}
				if !eq {
					return orderDepth(op, al[i], bl[i], depth+1)
				}
			}
			return cmpOrdered(op, len(al), len(bl)), nil
		}
	}
	return false, fmt.Errorf("'%s' not supported between instances of '%s' and '%s'", orderSymbols[op], runtime.PyTypeName(a), runtime.PyTypeName(b))
}

func cmpOrdered[T int | int64 | float64 | string](op string, a, b T) bool {
	switch op {
	case "lt":
		return a < b
	case "lteq":
		return a <= b
	case "gt":
		return a > b
	}
	return a >= b
}

// Compare evaluates the comparison operator op ("eq", "ne", "lt",
// "lteq", "gt", "gteq", "in", "notin") with Python semantics, so the
// builtin tests and filters share the operators' behaviour.
func Compare(op string, a, b any) (bool, error) {
	return compareValues(op, a, b)
}

// BinaryOp applies the arithmetic operator op ("+", "-", "*", "/",
// "//", "%", "**") with the template operators' semantics.
func BinaryOp(op string, a, b any) (any, error) {
	return applyBinop(op, a, b)
}

func inValue(needle, haystack any) (bool, error) {
	switch col := haystack.(type) {
	case string, escape.Markup:
		hay, _ := eqStr(col)
		s, ok := eqStr(needle)
		if !ok {
			return false, fmt.Errorf("'in <string>' requires string as left operand, not %T", needle)
		}
		return strings.Contains(hay, s), nil
	case []any:
		return containsAny(needle, col)
	case *runtime.PyList:
		if col == nil {
			return false, nil
		}
		return containsAny(needle, col.Items())
	case runtime.Tuple:
		return containsAny(needle, []any(col))
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
		if _, err := runtime.HashKey(needle); err != nil {
			return false, err
		}
		_, ok := col.Get(needle)
		return ok, nil
	case *runtime.DictView:
		return col.Contains(needle, SameOrEqual)
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

func containsAny(needle any, col []any) (bool, error) {
	for _, it := range col {
		// `in` checks identity before ==, like CPython.
		eq, err := elemEqual(needle, it, 0)
		if err != nil || eq {
			return eq, err
		}
	}
	return false, nil
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
