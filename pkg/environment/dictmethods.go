package environment

import (
	"fmt"
	"sort"

	"github.com/jryberg/gojinja/pkg/runtime"
)

// dictMethod returns a synthetic Go callable mirroring Python's dict
// methods (`items`, `keys`, `values`) so templates that work in canonical
// Jinja2 also work here. Parity is the bar — Python's dict has these as
// instance methods; gojinja exposes them through attribute lookup so
// `{{ d.items() }}` and `{% for k, v in d.items() %}` render identically.
//
// Ordering: Python dicts preserve insertion order; Go maps don't. We sort
// keys lexicographically so renders are deterministic. Templates that
// rely on insertion order should pipe through `|sort` (or `|dictsort`)
// — same recommendation as for any cross-language template.
//
// Returns nil when attr is not a recognised dict method, letting the
// caller fall back to other resolution paths.
func dictMethod(m any, attr string) any {
	switch attr {
	case "items":
		return func() []any {
			if od, ok := m.(*runtime.OrderedDict); ok {
				return od.Items()
			}
			keys := dictStringKeys(m)
			out := make([]any, 0, len(keys))
			for _, k := range keys {
				out = append(out, runtime.Tuple{k, dictGet(m, k)})
			}
			return out
		}
	case "keys":
		return func() []any {
			if od, ok := m.(*runtime.OrderedDict); ok {
				return od.Keys()
			}
			keys := dictStringKeys(m)
			out := make([]any, 0, len(keys))
			for _, k := range keys {
				out = append(out, k)
			}
			return out
		}
	case "values":
		return func() []any {
			if od, ok := m.(*runtime.OrderedDict); ok {
				return od.Values()
			}
			keys := dictStringKeys(m)
			out := make([]any, 0, len(keys))
			for _, k := range keys {
				out = append(out, dictGet(m, k))
			}
			return out
		}
	case "get":
		return func(args ...any) any {
			if len(args) < 1 {
				return nil
			}
			if od, ok := m.(*runtime.OrderedDict); ok {
				if v, ok := od.Get(args[0]); ok {
					return v
				}
				// Try string key lookup as fallback for callers that
				// pass a stringified form.
				if v, ok := od.Get(stringifyKey(args[0])); ok {
					return v
				}
				if len(args) >= 2 {
					return args[1]
				}
				return nil
			}
			key := stringifyKey(args[0])
			if v, ok := dictLookup(m, key); ok {
				return v
			}
			if len(args) >= 2 {
				return args[1]
			}
			return nil
		}
	}
	// Mutating methods are only exposed on *OrderedDict (in-template
	// dict literals). Caller-supplied map[string]any / map[any]any
	// stay immutable — host data is the sandbox boundary.
	if od, ok := m.(*runtime.OrderedDict); ok {
		switch attr {
		case "update":
			return func(args ...any) (any, error) {
				if len(args) == 0 {
					return nil, nil
				}
				if err := od.Update(args[0]); err != nil {
					return nil, err
				}
				return nil, nil
			}
		case "pop":
			return func(args ...any) (any, error) {
				if len(args) == 0 {
					return nil, fmt.Errorf("pop expected at least 1 argument, got 0")
				}
				if len(args) >= 2 {
					return od.Pop(args[0], args[1], true)
				}
				return od.Pop(args[0], nil, false)
			}
		case "setdefault":
			return func(args ...any) (any, error) {
				if len(args) == 0 {
					return nil, fmt.Errorf("setdefault expected at least 1 argument, got 0")
				}
				var def any
				if len(args) >= 2 {
					def = args[1]
				}
				return od.SetDefault(args[0], def), nil
			}
		case "clear":
			return func() any {
				od.Clear()
				return nil
			}
		}
	}
	return nil
}

// dictStringKeys returns the lexicographically sorted string keys of m.
// Non-string keys in a map[any]any are stringified via fmt.Sprint so
// iteration is total — Python's dict has no such limitation, but our
// map[any]any is only produced by dict-literals which always have
// string-coercible keys in practice.
func dictStringKeys(m any) []string {
	switch x := m.(type) {
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return keys
	case map[any]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, stringifyKey(k))
		}
		sort.Strings(keys)
		return keys
	}
	return nil
}

func dictGet(m any, key string) any {
	switch x := m.(type) {
	case map[string]any:
		return x[key]
	case map[any]any:
		// Try the literal string key first; fall back to scanning
		// (rare — only if a non-string key happens to stringify to the
		// same value).
		if v, ok := x[key]; ok {
			return v
		}
		for k, v := range x {
			if stringifyKey(k) == key {
				return v
			}
		}
	}
	return nil
}

func dictLookup(m any, key string) (any, bool) {
	switch x := m.(type) {
	case map[string]any:
		v, ok := x[key]
		return v, ok
	case map[any]any:
		if v, ok := x[key]; ok {
			return v, true
		}
		for k, v := range x {
			if stringifyKey(k) == key {
				return v, true
			}
		}
	}
	return nil, false
}

func stringifyKey(k any) string {
	if s, ok := k.(string); ok {
		return s
	}
	return fmt.Sprint(k)
}
