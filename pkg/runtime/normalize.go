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
//   - *OrderedDict → values are replaced with their normalized form
//     in place; insertion order is preserved. The same pointer is
//     returned (no copy).
//   - []any and []map[string]any → fresh slice with element-wise
//     recursion.
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
		for _, k := range x.keys {
			x.vals[k] = NormalizeForTemplate(x.vals[k])
		}
		return x
	case map[string]any:
		return orderedFromStringMap(x)
	case map[any]any:
		return orderedFromAnyMap(x)
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = NormalizeForTemplate(e)
		}
		return out
	case []map[string]any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = orderedFromStringMap(e)
		}
		return out
	}
	return v
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
	out := &OrderedDict{
		keys: make([]any, 0, len(keys)),
		vals: make(map[any]any, len(keys)),
	}
	for _, k := range keys {
		out.keys = append(out.keys, k)
		out.vals[k] = NormalizeForTemplate(m[k])
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
	out := &OrderedDict{
		keys: make([]any, 0, len(ks)),
		vals: make(map[any]any, len(ks)),
	}
	for _, e := range ks {
		out.keys = append(out.keys, e.k)
		out.vals[e.k] = NormalizeForTemplate(m[e.k])
	}
	return out
}
