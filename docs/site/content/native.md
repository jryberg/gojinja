# Native types

`pkg/native.Render` (re-exported as `gojinja.RenderNative`, when present)
returns a **typed Go value** instead of a string for templates whose entire
output parses as a JSON literal:

| Template renders to | `Render` returns |
|---|---|
| `42` | `float64(42)` |
| `true` | `bool(true)` |
| `null` | `nil` |
| `"hello"` | `string("hello")` |
| `[1, 2, 3]` | `[]any{1, 2, 3}` |
| `{"a": 1}` | `map[string]any{"a": 1}` |
| anything else | `string` (the rendered template, unchanged) |

This is the Go-idiomatic equivalent of Python Jinja2's
`NativeEnvironment` / `native_concat`. The detection rule is intentionally
**JSON-shaped** (not Python-literal-shaped) — gojinja doesn't parse Python
tuple syntax or `datetime` literals.

!!! note
    Full narrative coming in a follow-up. Reference on
    [pkg.go.dev/.../pkg/native](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/native).
