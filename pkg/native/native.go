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

// Render produces a typed Go value for tpl. Returns the original string
// if the rendered output doesn't parse as a literal.
func Render(ctx context.Context, tpl *environment.Template, vars map[string]any) (any, error) {
	out, err := tpl.RenderContext(ctx, vars)
	if err != nil {
		return nil, err
	}
	return Concat(out), nil
}

// Concat parses s as a Go literal. Tries int → float → bool → JSON
// (for objects/arrays/strings). Falls through to the original string
// when nothing matches.
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
