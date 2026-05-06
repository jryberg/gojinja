# API reference — overview

This page is a **narrative tour** of the gojinja Go API. The
authoritative per-symbol reference (full signatures, source links, every
exported identifier) lives on
[pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja), with one
frozen permalink per release tag — see the [symbols index](symbols.md)
for a complete list with deep links.

## Single-import surface

The root `github.com/jryberg/gojinja` package re-exports everything most
consumers need:

```go
import gj "github.com/jryberg/gojinja"

env, _ := gj.New(
    gj.WithLoader(gj.DictLoader{"hello.j2": "Hi {{ name }}!"}),
    gj.WithAutoescape(gj.AutoescapeNever{}),
)
tpl, _ := env.GetTemplate("hello.j2")
out, _ := tpl.Render(map[string]any{"name": "World"})
```

## Building blocks

### Environment

The central object. Holds the loader, the filter/test/global registries,
the autoescape policy, the cache, the sandbox state, and the lexer
options. Construct with `gj.New(opts...)`. Every `Option` documents its
default — see the [symbols index](symbols.md).

### Loaders

Implementations of the `Loader` interface. Seven built-ins ship in
`pkg/loader` and are re-exported from the root. See [the loaders
guide](../loaders.md) for usage.

### Cache

A persistent AST cache contract. Two implementations ship — `MemoryCache`
and `FilesystemCache`. Plug in via `gj.WithExternalCache(...)`. See [the
caching guide](../caching.md).

### Runtime types

`pkg/runtime` exposes the `Undefined` variants, `Namespace`, `Cycler`,
`Tuple`, `OrderedDict`, plus `Context`/`LoopContext`/`EvalContext`. Most
consumers only see `Undefined` (chosen via `gj.WithUndefined`) and
`Namespace` (used by `{% set ns.attr %}`).

### Sandbox

`pkg/sandbox` exposes the predicates the engine uses to gate attribute
access and callable dispatch — `IsSafeAttribute`, `IsSafeCallable`,
`IsMutatingMethod`, `UnsafeFunc`. Importable directly so you can apply
the same rules to your own callables. See [the sandbox
guide](../sandbox.md).

### Escape

`pkg/escape` exposes `Markup`, `SoftStr`, `Escape`, `ForceEscape`. Use
`Markup` to mark pre-escaped HTML safe for output; the engine produces
it itself in normal use, but custom filters may need it.

## See also

- [Symbols index](symbols.md) — every exported identifier with a deep
  link to pkg.go.dev.
- The package indexes on pkg.go.dev:
  [root](https://pkg.go.dev/github.com/jryberg/gojinja),
  [environment](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/environment),
  [loader](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/loader),
  [cache](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/cache),
  [runtime](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/runtime),
  [sandbox](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/sandbox).
