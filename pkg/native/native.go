// Package native provides a render path that returns a Go value instead
// of a string. When the entire template's output parses as a Go literal
// (number, bool, list, dict, …), the parsed value is returned directly.
// Otherwise the joined string is returned.
//
// Mirrors jinja2.nativetypes.NativeEnvironment, but the parser is a Go
// JSON-style literal parser rather than Python's `ast.literal_eval`.
// Templates that expect Python-specific tokens (e.g. `True`/`False`/`None`
// in literal output) need to render them as JSON-compatible forms.
package native

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/jryberg/gojinja/pkg/environment"
)

// Render produces a typed Go value for tpl.
//
// Detection rule: the template is rendered to a string, then the string
// is parsed as a JSON literal. If parsing succeeds, the typed value is
// returned; otherwise, the original string is returned.
//
// Returned types: `int64`, `float64`, `bool`, `nil`, `string`, `[]any`,
// `map[string]any` — whatever JSON decodes to (with `json.Number`
// normalised to `int64` or `float64`).
//
// Example:
//
//	tpl, _ := env.FromString("{{ a + b }}")
//	v, _  := native.Render(ctx, tpl, map[string]any{"a": 2, "b": 3})
//	// v == int64(5)   (not the string "5")
//
//	tpl2, _ := env.FromString("hello {{ name }}")
//	v2, _   := native.Render(ctx, tpl2, map[string]any{"name": "world"})
//	// v2 == "hello world"   (no JSON literal, falls back to string)
//
// Divergence from Python's `NativeEnvironment`: detection uses **JSON**
// literal syntax, not Python's `ast.literal_eval`. Templates rendering
// `True` / `False` / `None` literally would not be picked up; render
// them as JSON-compatible forms (`true` / `false` / `null`) for typed
// output.
func Render(ctx context.Context, tpl *environment.Template, vars map[string]any) (any, error) {
	out, err := tpl.RenderContext(ctx, vars)
	if err != nil {
		return nil, err
	}
	return Concat(out), nil
}

// Concat parses s as a JSON literal. Returns a typed Go value when
// successful; otherwise returns s unchanged. Used internally by
// [Render]; exported for callers that already have a rendered string
// and want the same coerce-or-passthrough behaviour.
//
// Empty / whitespace-only strings pass through unchanged.
func Concat(s string) any {
	t := strings.TrimSpace(s)
	if t == "" {
		return s
	}
	// JSON handles numbers, bools, null, arrays, objects, and quoted strings.
	var v any
	dec := json.NewDecoder(strings.NewReader(t))
	dec.UseNumber()
	if err := dec.Decode(&v); err == nil {
		// json.Number → int64/float64 for ergonomics.
		return normalize(v)
	}
	return s
}

func normalize(v any) any {
	switch x := v.(type) {
	case json.Number:
		if i, err := x.Int64(); err == nil {
			return i
		}
		if f, err := x.Float64(); err == nil {
			return f
		}
		return x.String()
	case []any:
		out := make([]any, len(x))
		for i, it := range x {
			out[i] = normalize(it)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, vv := range x {
			out[k] = normalize(vv)
		}
		return out
	}
	return v
}
