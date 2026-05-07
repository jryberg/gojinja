package environment

import (
	"fmt"
	"reflect"

	"github.com/jryberg/gojinja/pkg/runtime"
)

// listMethod returns a synthetic Go callable mirroring Python's list
// instance methods on a CALLER-SUPPLIED bare slice (`[]any`).
//
// Mutating methods are intentionally absent here: caller-supplied data
// is the sandbox boundary — host code controls those slices, templates
// shouldn't be able to mutate them. Templates that need `append`,
// `extend`, etc. construct the list inside the template (`{% set xs =
// [] %}`); list literals produce a *runtime.PyList instead, and the
// mutating methods live on [pyListMethod].
//
// What we expose on `[]any`:
//   - count(item) → number of equal items
//   - index(item) → first index, error if absent
func listMethod(items []any, attr string) any {
	switch attr {
	case "count":
		return func(target any) int {
			n := 0
			for _, it := range items {
				if equalForCompare(it, target) {
					n++
				}
			}
			return n
		}
	case "index":
		return func(args ...any) (int, error) {
			if len(args) < 1 {
				return -1, fmt.Errorf("index() takes at least 1 argument")
			}
			target := args[0]
			start := argInt(args, 1, 0)
			end := argInt(args, 2, len(items))
			start, end = clampSlice(len(items), start, end)
			for i := start; i < end; i++ {
				if equalForCompare(items[i], target) {
					return i, nil
				}
			}
			return -1, fmt.Errorf("%v is not in list", target)
		}
	}
	return nil
}

// equalForCompare implements Python-ish equality for the limited set of
// types templates produce. Specifically: int/float widening,
// case-sensitive string comparison, recursive list/dict.
func equalForCompare(a, b any) bool {
	if a == nil || b == nil {
		return a == b
	}
	switch x := a.(type) {
	case int, int64:
		ai, _ := asInt64Value(a)
		if bi, ok := asInt64Value(b); ok {
			return ai == bi
		}
		if bf, ok := asFloat64Value(b); ok {
			return float64(ai) == bf
		}
		return false
	case float64, float32:
		af, _ := asFloat64Value(a)
		if bf, ok := asFloat64Value(b); ok {
			return af == bf
		}
		if bi, ok := asInt64Value(b); ok {
			return af == float64(bi)
		}
		return false
	case string:
		bs, ok := b.(string)
		return ok && x == bs
	case bool:
		bb, ok := b.(bool)
		return ok && x == bb
	}
	return reflect.DeepEqual(a, b)
}

func asInt64Value(v any) (int64, bool) {
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
	}
	return 0, false
}

func asFloat64Value(v any) (float64, bool) {
	switch x := v.(type) {
	case float32:
		return float64(x), true
	case float64:
		return x, true
	}
	return 0, false
}

// pyListMethod returns a synthetic callable for an in-template
// *runtime.PyList. It mirrors Python's list methods — append, extend,
// insert, pop, remove, clear, reverse, sort — plus the non-mutating
// count / index that listMethod also exposes.
//
// The closures capture *PyList by pointer, so callers re-binding the
// list through `{% set xs = ... %}` aren't required: a single binding
// to the same list is mutated in place, matching Python.
func pyListMethod(l *runtime.PyList, attr string) any {
	if l == nil {
		return nil
	}
	switch attr {
	case "append":
		return func(v any) any {
			l.Append(v)
			return nil
		}
	case "extend":
		return func(seq any) (any, error) {
			if err := l.Extend(seq); err != nil {
				return nil, err
			}
			return nil, nil
		}
	case "insert":
		return func(idx int, v any) any {
			l.Insert(idx, v)
			return nil
		}
	case "pop":
		return func(args ...any) (any, error) {
			if len(args) == 0 {
				return l.Pop(0, false)
			}
			idx, ok := argInt2(args[0])
			if !ok {
				return nil, fmt.Errorf("pop: integer argument required, got %T", args[0])
			}
			return l.Pop(idx, true)
		}
	case "remove":
		return func(v any) (any, error) {
			if err := l.Remove(v, equalForCompare); err != nil {
				return nil, err
			}
			return nil, nil
		}
	case "clear":
		return func() any {
			l.Clear()
			return nil
		}
	case "reverse":
		return func() any {
			l.Reverse()
			return nil
		}
	case "sort":
		return func(args []any, kwargs map[string]any) (any, error) {
			reverse := false
			if v, ok := kwargs["reverse"]; ok {
				if b, ok := v.(bool); ok {
					reverse = b
				}
			}
			l.Sort(pythonOrderLess, reverse)
			return nil, nil
		}
	case "count":
		// Reuse the bare-slice helper so behaviour stays identical.
		return listMethod(l.Items(), "count")
	case "index":
		return listMethod(l.Items(), "index")
	}
	return nil
}

// argInt2 is a more permissive int-arg coercion than argInt: it accepts
// int / int8..int64 / float64 (when the float is integral). Used by
// pop/insert where the user can write `xs.pop(0)` or `xs.pop(-1)`.
func argInt2(v any) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case int8:
		return int(x), true
	case int16:
		return int(x), true
	case int32:
		return int(x), true
	case int64:
		return int(x), true
	case float64:
		if x == float64(int(x)) {
			return int(x), true
		}
	}
	return 0, false
}

// pythonOrderLess is the comparator used by *PyList.Sort (and any other
// "python-like default ordering" needs). It mirrors Python's default
// ordering for the limited subset of types templates produce: numeric
// widening between int/float, lexicographic for strings, false<true
// for bools. Heterogeneous comparisons fall back to type-name ordering
// to keep sort total — Python 3 raises in that case, but raising here
// would surprise template authors more than producing a stable order.
func pythonOrderLess(a, b any) bool {
	if af, ok := asFloat64Value(a); ok {
		if bf, ok := asFloat64Value(b); ok {
			return af < bf
		}
	}
	if ai, ok := asInt64Value(a); ok {
		if bi, ok := asInt64Value(b); ok {
			return ai < bi
		}
		if bf, ok := asFloat64Value(b); ok {
			return float64(ai) < bf
		}
	}
	if as, ok := a.(string); ok {
		if bs, ok := b.(string); ok {
			return as < bs
		}
	}
	if ab, ok := a.(bool); ok {
		if bb, ok := b.(bool); ok {
			return !ab && bb
		}
	}
	// Fallback: stable cross-type ordering via type name.
	return fmt.Sprintf("%T", a) < fmt.Sprintf("%T", b)
}
