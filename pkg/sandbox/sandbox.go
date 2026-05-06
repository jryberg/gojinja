// Package sandbox holds the safety predicates the default Environment
// applies to attribute / item / call requests. The predicates live here
// so the Environment can stay thin and other components (e.g. format-
// string evaluation) can re-use the same checks.
//
// Mirrors jinja2.sandbox but with stricter defaults — gojinja sandboxes
// by default whereas Jinja2 makes you opt in.
package sandbox

import (
	"reflect"
	"strings"
)

// IsSafeAttribute reports whether `attr` is allowed on `obj`. Rules:
//
//   - Empty names are rejected.
//   - Names starting with `_` are rejected (private convention).
//   - Reflect-internal types (reflect.Value, reflect.Type) are blocked
//     entirely.
//   - Pointer-to-pointer / function-typed attributes are allowed but the
//     caller must use [IsSafeCallable] before invoking the result.
func IsSafeAttribute(obj any, attr string) bool {
	if attr == "" {
		return false
	}
	if attr[0] == '_' {
		return false
	}
	if obj == nil {
		return true
	}
	t := reflect.TypeOf(obj)
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	switch t.PkgPath() {
	case "reflect", "runtime":
		// Reflection metadata access would let templates introspect
		// engine internals.
		return false
	}
	return true
}

// IsSafeCallable reports whether the given value may be invoked from a
// template. Currently every Go function is considered safe unless it has
// been flagged via [MarkUnsafe]. Phase 22+ may add an "alters_data"
// equivalent.
func IsSafeCallable(callee any) bool {
	if callee == nil {
		return false
	}
	if u, ok := callee.(unsafeMarker); ok && u.GoJinjaUnsafe() {
		return false
	}
	return true
}

// unsafeMarker is the interface a callable opts into to be rejected by
// the default Environment. The marker method name is deliberately
// long-winded to avoid colliding with anything common.
type unsafeMarker interface {
	GoJinjaUnsafe() bool
}

// UnsafeFunc is a wrapper that marks a callable unsafe; the env's Call
// dispatcher will refuse to invoke it.
type UnsafeFunc struct {
	Fn any
}

// GoJinjaUnsafe satisfies [unsafeMarker].
func (UnsafeFunc) GoJinjaUnsafe() bool { return true }

// MutatingMethods enumerates method names that should be blocked under
// the immutable variant of the sandbox (similar to Jinja2's
// `ImmutableSandboxedEnvironment`).
var MutatingMethods = map[string]bool{
	// list/slice
	"append": true, "extend": true, "insert": true, "pop": true,
	"remove": true, "sort": true, "reverse": true,
	// map/dict
	"clear": true, "popitem": true, "setdefault": true, "update": true,
	// set
	"add": true, "discard": true, "difference_update": true,
	"symmetric_difference_update": true,
}

// IsMutatingMethod returns true if attr is a known mutating method name
// on common built-in types. Used by the immutable-sandbox option.
func IsMutatingMethod(attr string) bool {
	return MutatingMethods[strings.ToLower(attr)]
}
