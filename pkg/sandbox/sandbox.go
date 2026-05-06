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

// IsSafeAttribute reports whether `attr` is allowed on `obj`.
//
// Rules:
//   - Empty names are rejected.
//   - Names starting with `_` are rejected (private convention; blocks
//     Python-style dunders like `__class__`, `__mro__`, `__bases__`,
//     `__subclasses__`, `__init__` that classic Jinja2 sandbox-escape
//     exploits use to climb the type hierarchy).
//   - Reflection-internal types (`reflect.Value`, `reflect.Type`,
//     `runtime.*`) are blocked entirely — they would let templates
//     introspect engine internals.
//   - Function-typed attributes are allowed but the caller must use
//     [IsSafeCallable] before invoking the result.
//
// The default `environment.Environment` calls this on every attribute
// lookup. Disabled by `environment.WithUnsafe()`.
//
// Example:
//
//	sandbox.IsSafeAttribute(user, "name")        →  true
//	sandbox.IsSafeAttribute(user, "__class__")   →  false (private)
//	sandbox.IsSafeAttribute(reflect.ValueOf(0), "Type")  →  false
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
// template.
//
// Every Go function is considered safe unless its concrete type
// implements an `GoJinjaUnsafe() bool` method (which [UnsafeFunc] does).
// Use [UnsafeFunc] to wrap any callable you want exposed-but-rejected
// in the default sandbox.
//
// Example:
//
//	env, _ := environment.New(environment.WithGlobal("dangerous",
//	    sandbox.UnsafeFunc{Fn: deleteEverything},
//	))
//	// {{ dangerous() }} → SecurityError
//
//	envUnsafe, _ := environment.New(
//	    environment.WithUnsafe(),
//	    environment.WithGlobal("dangerous", deleteEverything),
//	)
//	// {{ dangerous() }} → invoked
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
// dispatcher refuses to invoke it from sandboxed templates.
//
// The wrapped function is still callable from outside the sandbox (e.g.
// in an `environment.WithUnsafe()` env), so this is the right way to
// "register a global, but only let the un-sandboxed config use it".
//
// Example:
//
//	dangerous := sandbox.UnsafeFunc{Fn: func() { /* destructive */ }}
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
// on common built-in types (`pop`, `append`, `clear`, `update`, etc.).
//
// Used by the immutable-sandbox variant of the engine: the sandbox
// refuses to call these even on objects the template otherwise has
// access to. Without this gate, `{{ ctx_list.append("evil") }}` would
// mutate context state across renders.
//
// Example:
//
//	sandbox.IsMutatingMethod("append")  →  true
//	sandbox.IsMutatingMethod("get")     →  false
func IsMutatingMethod(attr string) bool {
	return MutatingMethods[strings.ToLower(attr)]
}
