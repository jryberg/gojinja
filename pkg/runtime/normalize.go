package runtime

import (
	"fmt"
	"sort"
)

// NormalizeForTemplate walks v and rewrites every Go map it encounters
// as an [*OrderedDict] so the template engine sees one uniform dict
// representation regardless of how the caller built the input.
//
// The rules:
//
//   - map[string]any → *OrderedDict with keys lex-sorted. Go's map
//     iteration is randomised, so the lex sort is a deterministic
//     substitute for the insertion order Python preserves but Go cannot
//     carry. Each value is recursively normalized.
//   - map[any]any → *OrderedDict with keys ordered by their stringified
//     form. Mirrors the eval-side fallback for map[any]any iteration.
//   - *OrderedDict → a copy with normalized values; insertion order is
//     preserved. Like every container, it is copied so that a template
//     mutating it never changes the caller's data or a later render.
//   - []any, []string and []map[string]any → a fresh [*PyList] (Python's
//     mutable list) with element-wise recursion, so templates can
//     mutate it without touching the caller's data.
//   - Anything else → returned unchanged. Structs and other Go types
//     are reached through the engine's attribute lookup, which uses
//     reflection; they do not need normalization here.
//
// Use this at the boundary where user data enters the template engine
// (typically [Template.RenderContext]). After normalization, every dict
// in the value graph is an *OrderedDict, so iteration in `{% for k, v in
// d.items() %}` is order-stable at every nesting depth. For data loaded
// via JSONVars or constructed with NewOrderedDict the order is the
// caller's source order; for Go maps it is lex-sorted.
func NormalizeForTemplate(v any) any {
	switch x := v.(type) {
	case nil:
		return nil
	case *OrderedDict:
		if x == nil {
			return x
		}
		out := NewOrderedDict()
		for _, k := range x.keys {
			out.Set(k, NormalizeForTemplate(x.vals[hashKeyOf(k)]))
		}
		return out
	case map[string]any:
		return orderedFromStringMap(x)
	case map[any]any:
		return orderedFromAnyMap(x)
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = NormalizeForTemplate(e)
		}
		return NewPyList(out)
	case []string:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = e
		}
		return NewPyList(out)
	case []map[string]any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = orderedFromStringMap(e)
		}
		return NewPyList(out)
	}
	return v
}

// ToPyList returns v with every []any and []string, at any depth inside
// lists and tuples, turned into a fresh [*PyList]: the lists filters and
// Go callables return become Python lists a template can mutate.
func ToPyList(v any) any {
	switch x := v.(type) {
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = ToPyList(e)
		}
		return NewPyList(out)
	case []string:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = e
		}
		return NewPyList(out)
	case Tuple:
		var out Tuple
		for i, e := range x {
			if c := ToPyList(e); !sameValue(c, e) {
				if out == nil {
					out = append(Tuple(nil), x...)
				}
				out[i] = c
			}
		}
		if out != nil {
			return out
		}
	}
	return v
}

// sameValue reports whether ToPyList left e unchanged (only lists are
// replaced, so a type check suffices).
func sameValue(a, b any) bool {
	switch a.(type) {
	case *PyList:
		_, wasList := b.(*PyList)
		return wasList
	}
	return true
}

// orderedFromStringMap converts a map[string]any into a fresh
// *OrderedDict with lex-sorted keys, recursively normalizing values.
// A nil input returns a typed nil *OrderedDict so the value still passes
// the *OrderedDict type assertion downstream.
func orderedFromStringMap(m map[string]any) *OrderedDict {
	if m == nil {
		return (*OrderedDict)(nil)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := NewOrderedDict()
	for _, k := range keys {
		out.Set(k, NormalizeForTemplate(m[k]))
	}
	return out
}

// orderedFromAnyMap converts a map[any]any into a fresh *OrderedDict
// with keys ordered by their stringified form. Matches the lex-sort
// strategy used in pkg/eval's toIterable for non-string keys.
func orderedFromAnyMap(m map[any]any) *OrderedDict {
	if m == nil {
		return (*OrderedDict)(nil)
	}
	type kv struct {
		k any
		s string
	}
	ks := make([]kv, 0, len(m))
	for k := range m {
		ks = append(ks, kv{k, fmt.Sprint(k)})
	}
	sort.Slice(ks, func(i, j int) bool { return ks[i].s < ks[j].s })
	out := NewOrderedDict()
	for _, e := range ks {
		out.Set(e.k, NormalizeForTemplate(m[e.k]))
	}
	return out
}
