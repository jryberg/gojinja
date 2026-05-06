# Quick start

The shortest possible gojinja program:

```go
package main

import (
    "context"
    "fmt"
    "log"

    gj "github.com/jryberg/gojinja"
)

func main() {
    env, err := gj.New()
    if err != nil {
        log.Fatal(err)
    }

    tpl, err := env.FromString(
        "Hello, {{ name }}! You have {{ count }} message{{ 's' if count != 1 }}.")
    if err != nil {
        log.Fatal(err)
    }

    out, err := tpl.RenderContext(context.Background(), map[string]any{
        "name":  "Alice",
        "count": 3,
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(out)
    // → Hello, Alice! You have 3 messages.
}
```

## What just happened

- `gj.New()` constructs an `Environment` with **safe defaults**: sandbox on,
  HTML autoescape on, no host environment access, allowlisted file loader.
  See [the sandbox guide](sandbox.md) for the full list and how to opt out.
- `env.FromString` parses the template once and returns a reusable
  `*Template`.
- `tpl.RenderContext` evaluates the template against the given variables,
  honouring the supplied `context.Context` for cancellation. Use
  `tpl.Render` for the convenience version that wraps
  `context.Background()`.

## Loading templates from disk

For real applications you probably want a [loader](loaders.md) instead of
`FromString`:

```go
import "github.com/jryberg/gojinja/pkg/loader"

l, _ := loader.NewFileSystem([]string{"./templates"})
env, _ := gj.New(gj.WithLoader(l))

tpl, _ := env.GetTemplate("page.html")
out, _ := tpl.Render(map[string]any{"user": "Alice"})
```

`FileSystemLoader` is **allowlisted**: it rejects any template name that
escapes the supplied roots (NUL bytes, `..`, drive letters, symlinks
pointing outside the root). See [the loaders guide](loaders.md) for the
other built-in loaders (`Embed`, `Prefix`, `Choice`, `Func`, `Cached`).

## Autoescape

Autoescape is **on by default** — gojinja HTML-escapes anything rendered
through `{{ ... }}` unless it is already a `Markup` value. Use the `safe`
filter to opt out for one expression, or pass
`gj.WithAutoescape(gj.AutoescapeNever{})` to disable globally:

```go
env, _ := gj.New() // autoescape on (default)

tpl, _ := env.FromString(`<p>{{ user_input }}</p>`)
out, _ := tpl.Render(map[string]any{"user_input": "<script>alert(1)</script>"})
// → "<p>&lt;script&gt;alert(1)&lt;/script&gt;</p>"
```

## Custom filters and tests

```go
env, _ := gj.New(
    gj.WithFilter("shout", func(s string) string { return s + "!!!" }),
    gj.WithTest("blank", func(s string) bool { return s == "" }),
)
```

Filter and test signatures may use `(value)`, `(value, args...)`, or the
full
`(env *Environment, ctx *runtime.Context, value any, args []any, kwargs map[string]any) (any, error)`
form. The engine adapts the call site automatically.

## Where to next

- [Template designer reference](templates/designer.md) — variables, filters,
  tests, control structures, inheritance, expressions.
- [Filter reference](templates/filters/index.md) ·
  [Test reference](templates/tests/index.md) ·
  [Globals reference](templates/globals/index.md)
- [Migrating from Python Jinja2](migrating-from-jinja2.md)
- [Documented divergences](divergences.md)
