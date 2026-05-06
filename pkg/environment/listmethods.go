package environment

import (
	"fmt"
	"reflect"
)

// listMethod returns a synthetic Go callable mirroring Python's list
// instance methods. Mutating methods (`append`, `extend`, `insert`,
// `remove`, `pop`, `clear`, `sort`, `reverse`) are intentionally not
// exposed: the sandbox forbids mutating templates' inputs and Python
// templates that need those typically iterate non-mutatively anyway.
//
// What we expose:
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
