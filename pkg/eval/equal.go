package eval

import (
	"reflect"

	gjerrors "github.com/jryberg/gojinja/pkg/errors"
	"github.com/jryberg/gojinja/pkg/escape"
	"github.com/jryberg/gojinja/pkg/runtime"
)

// maxEqualDepth bounds container recursion in [ValuesEqual] and the
// ordering comparisons.
const maxEqualDepth = escape.MaxDepth

// ValuesEqual reports Python's `a == b` for template values.
//
// Numbers compare by value across int/float, and bool counts as the int
// 0/1 the way Python's bool subclasses int. str and Markup compare by
// text. An Undefined equals another Undefined of the same kind, and a
// StrictUndefined fails. Lists compare element-wise with lists, tuples with tuples, and
// mappings key-by-key regardless of their Go representation. Values of
// any other type fall back to [reflect.DeepEqual], which never panics on
// uncomparable types.
func ValuesEqual(a, b any) (bool, error) {
	return valuesEqual(a, b, 0)
}

func valuesEqual(a, b any, depth int) (bool, error) {
	if depth > maxEqualDepth {
		return false, gjerrors.NewTemplateRuntimeError("maximum recursion depth exceeded in comparison")
	}
	if u, ok := a.(runtime.Undefined); ok {
		return u.Equal(b)
	}
	if u, ok := b.(runtime.Undefined); ok {
		return u.Equal(a)
	}
	if ai, ok := eqInt(a); ok {
		if bi, ok := eqInt(b); ok {
			return ai == bi, nil
		}
	}
	if af, ok := eqFloat(a); ok {
		if bf, ok := eqFloat(b); ok {
			return af == bf, nil
		}
		return false, nil
	}
	if as, ok := eqStr(a); ok {
		bs, ok := eqStr(b)
		return ok && as == bs, nil
	}
	if ab, ok := a.(runtime.Bytes); ok {
		bb, ok := b.(runtime.Bytes)
		return ok && string(ab) == string(bb), nil
	}
	if ar, ok := a.(*runtime.Range); ok {
		br, ok := b.(*runtime.Range)
		return ok && ar.Equal(br), nil
	}
	if _, ok := b.(*runtime.Range); ok {
		return false, nil
	}
	if av, ok := a.(*runtime.DictView); ok {
		return viewEqual(av, b, depth)
	}
	if bv, ok := b.(*runtime.DictView); ok {
		return viewEqual(bv, a, depth)
	}
	if al, aTuple, ok := eqSeq(a); ok {
		bl, bTuple, ok := eqSeq(b)
		if !ok || aTuple != bTuple || len(al) != len(bl) {
			return false, nil
		}
		for i := range al {
			eq, err := elemEqual(al[i], bl[i], depth)
			if err != nil || !eq {
				return false, err
			}
		}
		return true, nil
	}
	if am, ok := eqMapping(a); ok {
		bm, ok := eqMapping(b)
		if !ok || len(am.keys) != len(bm.keys) {
			return false, nil
		}
		for _, k := range am.keys {
			bv, found := bm.get(k)
			if !found {
				return false, nil
			}
			av, _ := am.get(k)
			eq, err := elemEqual(av, bv, depth)
			if err != nil || !eq {
				return false, err
			}
		}
		return true, nil
	}
	return reflect.DeepEqual(a, b), nil
}

// SameOrEqual is CPython's identity-then-== comparison, which `in`,
// list.index/count/remove and container equality use for members.
func SameOrEqual(a, b any) (bool, error) { return elemEqual(a, b, 0) }

// elemEqual compares container members the way CPython does: identity
// first, then `==`.
func elemEqual(a, b any, depth int) (bool, error) {
	switch x := a.(type) {
	case *runtime.PyList:
		if y, ok := b.(*runtime.PyList); ok && x == y {
			return true, nil
		}
	case *runtime.OrderedDict:
		if y, ok := b.(*runtime.OrderedDict); ok && x == y {
			return true, nil
		}
	}
	return valuesEqual(a, b, depth+1)
}

// eqInt returns v as an int64 when it is a Go integer or a bool.
func eqInt(v any) (int64, bool) {
	if b, ok := v.(bool); ok {
		if b {
			return 1, true
		}
		return 0, true
	}
	return asInt64(v)
}

// eqFloat returns v as a float64 when it is any Python number.
func eqFloat(v any) (float64, bool) {
	if i, ok := eqInt(v); ok {
		return float64(i), true
	}
	return asFloat(v)
}

func eqStr(v any) (string, bool) {
	switch x := v.(type) {
	case string:
		return x, true
	case escape.Markup:
		return string(x), true
	}
	return "", false
}

// eqSeq returns the items of a list or tuple and whether it is a tuple.
func eqSeq(v any) ([]any, bool, bool) {
	if t, ok := v.(runtime.Tuple); ok {
		return []any(t), true, true
	}
	if l, ok := asAnyList(v); ok {
		return l, false, true
	}
	if ss, ok := v.([]string); ok {
		out := make([]any, len(ss))
		for i, s := range ss {
			out[i] = s
		}
		return out, false, true
	}
	return nil, false, false
}

// mappingView is a read-only view over the dict representations a
// template can hold.
type mappingView struct {
	keys []any
	get  func(key any) (any, bool)
}

func eqMapping(v any) (mappingView, bool) {
	switch m := v.(type) {
	case *runtime.OrderedDict:
		if m == nil {
			return mappingView{}, false
		}
		return mappingView{keys: m.Keys(), get: m.Get}, true
	case map[string]any:
		keys := make([]any, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		return mappingView{keys: keys, get: func(key any) (any, bool) {
			s, ok := key.(string)
			if !ok {
				return nil, false
			}
			val, ok := m[s]
			return val, ok
		}}, true
	case map[any]any:
		keys := make([]any, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		// Keys always come from another mapping, so they are hashable.
		return mappingView{keys: keys, get: func(key any) (any, bool) {
			val, ok := m[key]
			return val, ok
		}}, true
	}
	return mappingView{}, false
}

// Contains reports Python's `needle in haystack`. Exported so the `in`
// test shares the operator's semantics.
func Contains(needle, haystack any) (bool, error) {
	return inValue(needle, haystack)
}

// viewEqual is == for a dict view: keys and items views compare as sets
// with another view of the same kind, a values view only with itself.
func viewEqual(a *runtime.DictView, b any, depth int) (bool, error) {
	bv, ok := b.(*runtime.DictView)
	if !ok || a.Kind != bv.Kind {
		return false, nil
	}
	if a.Kind == "values" {
		return a == bv, nil
	}
	if a.Len() != bv.Len() {
		return false, nil
	}
	for _, it := range a.Items() {
		found, err := bv.Contains(it, func(x, y any) (bool, error) { return elemEqual(x, y, depth) })
		if err != nil || !found {
			return false, err
		}
	}
	return true, nil
}
