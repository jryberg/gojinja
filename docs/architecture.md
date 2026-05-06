# gojinja — architecture

A single-page overview. Grows as phases land.

## Data flow

```
                              ┌──────────────────────────────────┐
                              │ Environment (pkg/environment)    │
                              │  options, policies, registries   │
                              └──────────────────────────────────┘
                                            │
                                            │ resolve template
                                            ▼
                              ┌──────────────────────────────────┐
                              │ Loader (pkg/loader)              │
                              │  Dict, FileSystem (root-locked), │
                              │  Embed, Prefix, Choice, Function │
                              └──────────────────────────────────┘
                                            │
                            source string + uptodate
                                            │
                                            ▼
   ┌──────────────┐    tokens   ┌──────────────┐    AST    ┌──────────────┐
   │  pkg/lexer   │────────────▶│  pkg/parser  │──────────▶│   pkg/ast    │
   └──────────────┘             └──────────────┘           └──────────────┘
                                                                 │
                                              symbols / fold     │
                                                                 ▼
                                                   ┌──────────────────────┐
                                                   │ pkg/idtracking +     │
                                                   │ pkg/optimizer        │
                                                   └──────────────────────┘
                                                                 │
                                                       cached IR │
                                            ┌────────────────────┘
                                            │
                              ┌─────────────────────────────┐
                              │ pkg/cache (Memory / FS)     │
                              │  bucket = magic+sum+IR      │
                              └─────────────────────────────┘
                                            │
                                            ▼
                              ┌──────────────────────────────────┐
                              │ pkg/eval (tree-walking interp.)  │
                              │  consults pkg/sandbox on every   │
                              │  attr/item/call; pkg/filters &   │
                              │  pkg/tests for | and is; calls   │
                              │  pkg/ext extension handlers      │
                              └──────────────────────────────────┘
                                            │
                                            ▼
                                  io.Writer (or NativeValue)
```

## Package ownership

| Layer | Package(s) | Responsibility |
|---|---|---|
| Lexical | `pkg/lexer` | source → tokens |
| Syntactic | `pkg/parser`, `pkg/ast` | tokens → AST |
| Static | `pkg/idtracking`, `pkg/optimizer` | symbol resolution + constant folding |
| Storage | `pkg/cache` | persistent IR |
| Runtime objects | `pkg/runtime`, `pkg/escape`, `pkg/errors` | Context, Markup, Undefined, error types |
| Evaluation | `pkg/eval` | walks AST, emits output |
| Safety | `pkg/sandbox` | consulted by eval on every reflective op |
| Plug-ins | `pkg/ext`, `pkg/filters`, `pkg/tests`, `pkg/globals` | extension framework + built-ins |
| Loaders | `pkg/loader` | source resolution |
| Native | `pkg/native` | NativeEnvironment, alternate concat |
| Façade | `pkg/environment`, root `gojinja` package | user-facing API |
| Tooling | `pkg/meta`, `pkg/debug`, `cmd/gojinja` | introspection, error rendering, CLI |

## Threading model

- `Environment` is read-mostly; mutation (extension registration, filter registration, policy edits) happens at construction time. After that it's safe for concurrent `Render` calls.
- `Template` is immutable post-compile.
- `Context` is per-render; never shared.
- Cache implementations carry their own internal locking; the public `Cache` interface promises concurrent safety.

## Diagram conventions

A solid arrow is a one-way data dependency. The cache is bidirectional because it both serves and stores. The sandbox is consulted by eval, not invoked by caller code directly.
