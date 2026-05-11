# gojinja

A Go implementation of [Jinja2](https://github.com/pallets/jinja) — the template engine. The bar gojinja sets for itself: **a template that renders in canonical Python Jinja2 should render byte-identically in gojinja.**

📚 **Full documentation:** <https://jryberg.github.io/gojinja/>

[![CI](https://github.com/jryberg/gojinja/actions/workflows/ci.yml/badge.svg)](https://github.com/jryberg/gojinja/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/jryberg/gojinja.svg)](https://pkg.go.dev/github.com/jryberg/gojinja)
[![Go Report Card](https://goreportcard.com/badge/github.com/jryberg/gojinja)](https://goreportcard.com/report/github.com/jryberg/gojinja)
[![License: BSD 3-Clause](https://img.shields.io/badge/License-BSD%203--Clause-blue.svg)](LICENSE)
[![Conventional Commits](https://img.shields.io/badge/Conventional%20Commits-1.0.0-yellow.svg)](https://www.conventionalcommits.org/en/v1.0.0/)

```go
import gj "github.com/jryberg/gojinja"

env, _ := gj.New()
tpl, _ := env.FromString("Hello, {{ name|upper }}!")
out, _ := tpl.Render(map[string]any{"name": "World"})
// out == "Hello, WORLD!"
```

- **Single import** for most use cases (`github.com/jryberg/gojinja`).
- **Zero external Go dependencies.** Standard library only.
- **Safe by default.** Sandbox on, autoescape on, host-environment access off, bounded `range`, bounded cache, context-cancellable rendering.
- **Same templates as Python.** Use the exact `.j2` files you'd use with Python Jinja2.

> Looking to contribute? See [CONTRIBUTING.md](CONTRIBUTING.md) for the
> commit-message rules and PR process, and [DEVELOPMENT.md](DEVELOPMENT.md)
> for the local dev environment, parity tests, and architecture overview.

---

## Table of contents

- [Install](#install)
- [Quick start](#quick-start)
- [Common use cases](#common-use-cases)
  - [Render a string template](#render-a-string-template)
  - [Render with autoescape](#render-with-autoescape)
  - [Load JSON-shaped variables](#load-json-shaped-variables)
  - [Load templates from disk](#load-templates-from-disk)
  - [Embed templates in the binary](#embed-templates-in-the-binary)
  - [Read host environment variables](#read-host-environment-variables)
  - [Inheritance with `extends` / `block` / `super`](#inheritance-with-extends--block--super)
  - [Macros and `caller`](#macros-and-caller)
  - [Context cancellation](#context-cancellation)
  - [Custom filters and tests](#custom-filters-and-tests)
  - [i18n / translations](#i18n--translations)
  - [Persistent on-disk AST cache](#persistent-on-disk-ast-cache)
- [What's supported](#whats-supported)
  - [Language constructs](#language-constructs)
  - [Filters (54)](#filters-54)
  - [Tests (39)](#tests-39)
  - [Globals (6)](#globals-6)
  - [Loaders (7)](#loaders-7)
  - [Synthetic methods on built-in types](#synthetic-methods-on-built-in-types)
  - [Extensions](#extensions)
- [What's *not* supported](#whats-not-supported)
- [Security defaults vs. Python Jinja2](#security-defaults-vs-python-jinja2)
- [API reference](#api-reference)
  - [Options](#options)
  - [Autoescape policies](#autoescape-policies)
  - [Undefined variants](#undefined-variants)
  - [Custom filter / test signatures](#custom-filter--test-signatures)
- [CLI](#cli)
- [License](#license)

---

## Install

```bash
go get github.com/jryberg/gojinja@latest
```

Requires Go 1.22 or newer. No external Go dependencies.

## Quick start

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

    tpl, err := env.FromString("Hello, {{ name }}! You have {{ count }} message{{ 's' if count != 1 }}.")
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

---

## Common use cases

### Render a string template

```go
env, _ := gj.New()
tpl, _ := env.FromString("{{ greeting }}, {{ name }}!")
out, _ := tpl.Render(map[string]any{
    "greeting": "Hi",
    "name":     "Bob",
})
// out == "Hi, Bob!"
```

### Render with autoescape

Autoescape is **on by default** — the engine HTML-escapes any non-`Markup` value rendered with `{{ ... }}`. Use the `safe` filter to opt out for a specific value, or pass `gj.AutoescapeNever{}` to disable globally.

```go
env, _ := gj.New() // autoescape on (default)

tpl, _ := env.FromString(`<p>{{ user_input }}</p>`)
out, _ := tpl.Render(map[string]any{"user_input": "<script>alert(1)</script>"})
// out == "<p>&lt;script&gt;alert(1)&lt;/script&gt;</p>"

tpl2, _ := env.FromString(`<p>{{ html_blob|safe }}</p>`)
out2, _ := tpl2.Render(map[string]any{"html_blob": "<em>OK</em>"})
// out2 == "<p><em>OK</em></p>"
```

### Load JSON-shaped variables

If your template variables come from JSON, decode them with `gojinja.JSONVars` (or `gojinja.NormalizeJSONNumbers` if you already drive `json.Decoder` yourself). Vanilla `encoding/json.Unmarshal` collapses every JSON number to `float64`, while Python's `json.loads` preserves the int/float distinction — and `{{ x }}` renders the two differently (`"130000"` vs `"130000.0"`). Skipping this step silently breaks parity with Python Jinja2 for templates you share between the two engines.

```go
raw, _ := os.ReadFile("vars.json") // e.g. {"port": 130000, "rate": 1.5}

// Recommended:
vars, _ := gj.JSONVars(raw)

tpl, _ := env.FromString("{{ port }} {{ rate }}")
out, _ := tpl.Render(vars)
// → "130000 1.5"   (matches Python; vanilla json.Unmarshal would emit "130000.0 1.5")
```

If you already drive the decoder yourself:

```go
dec := json.NewDecoder(r)
dec.UseNumber()
var v map[string]any
_ = dec.Decode(&v)
vars := gj.NormalizeJSONNumbers(v).(map[string]any)
```

### Load templates from disk

`FileSystemLoader` is allowlisted: it requires explicit roots and rejects any name that resolves outside them (NUL bytes, `..`, drive letters, escaped symlinks). Symlinks pointing outside a root are rejected unless you pass `loader.WithFollowLinks(true)`.

```go
import "github.com/jryberg/gojinja/pkg/loader"

l, err := loader.NewFileSystem([]string{"./templates"})
if err != nil { log.Fatal(err) }

env, _ := gj.New(gj.WithLoader(l))

tpl, err := env.GetTemplate("page.html")
if err != nil { log.Fatal(err) }

out, _ := tpl.Render(map[string]any{"user": "Alice"})
```

### Embed templates in the binary

Bake templates into the binary with Go's `embed`:

```go
import (
    "embed"
    "github.com/jryberg/gojinja/pkg/loader"
)

//go:embed templates/*
var tmplFS embed.FS

func newEnv() *gj.Environment {
    env, _ := gj.New(gj.WithLoader(loader.NewEmbed(tmplFS, "templates")))
    return env
}
```

### Read host environment variables

Host env access is **off by default**. Opt in with `WithHostEnv()`, which registers `env` as a global function backed by `os.Getenv`:

```go
env, _ := gj.New(gj.WithHostEnv())
tpl, _ := env.FromString(`db: {{ env('DATABASE_URL')|default('sqlite://local.db', true) }}`)
out, _ := tpl.Render(nil)
```

Missing variables return `""` (matching `os.Getenv`). Pair with `default('...', true)` to substitute a fallback when the variable is unset *or* empty. The opt-in exposes every variable in the host process's environment to the template — don't enable it for templates supplied by untrusted users.

### Inheritance with `extends` / `block` / `super`

```go
env, _ := gj.New(
    gj.WithLoader(gj.DictLoader{
        "base.html":  `<title>{% block title %}default{% endblock %}</title><body>{% block body %}{% endblock %}</body>`,
        "child.html": `{% extends "base.html" %}{% block title %}{{ super() }} — Child{% endblock %}{% block body %}Hello.{% endblock %}`,
    }),
)
tpl, _ := env.GetTemplate("child.html")
out, _ := tpl.Render(nil)
// out == "<title>default — Child</title><body>Hello.</body>"
```

### Macros and `caller`

```go
src := `
{%- macro listing(items) -%}
<ul>
{% for x in items %}<li>{{ caller(x) }}</li>
{% endfor -%}
</ul>
{%- endmacro -%}

{% call(item) listing(['a', 'b', 'c']) %}<b>{{ item }}</b>{% endcall %}
`

env, _ := gj.New()
tpl, _ := env.FromString(src)
out, _ := tpl.Render(nil)
```

### Context cancellation

`Template.RenderContext(ctx, vars)` checks `ctx.Done()` at every statement boundary, so a long-running render (huge loop, expensive filter chain) terminates promptly when the caller cancels:

```go
import "time"

ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
defer cancel()

out, err := tpl.RenderContext(ctx, map[string]any{"items": hugeList})
// err == context.DeadlineExceeded if rendering exceeded the budget
```

### Custom filters and tests

```go
import (
    "strings"
    gj "github.com/jryberg/gojinja"
    "github.com/jryberg/gojinja/pkg/environment"
    "github.com/jryberg/gojinja/pkg/runtime"
)

func myEcho(env *environment.Environment, ctx *runtime.Context, value any, args []any, kwargs map[string]any) (any, error) {
    return strings.Repeat(value.(string), 3), nil
}

env, _ := gj.New(
    gj.WithFilter("echo3", environment.Filter{Func: myEcho}),
)
tpl, _ := env.FromString("{{ 'hi'|echo3 }}")
// → "hihihi"
```

Tests follow the same shape but return `(bool, error)`:

```go
gj.WithTest("startsWithA", environment.Test{Func: func(env *environment.Environment, ctx *runtime.Context, value any, args []any, kwargs map[string]any) (bool, error) {
    s, ok := value.(string)
    return ok && strings.HasPrefix(s, "A"), nil
}})
```

### i18n / translations

```go
import (
    "github.com/jryberg/gojinja/pkg/environment"
)

type frenchTranslator struct{}

func (frenchTranslator) Gettext(msg string) string {
    if msg == "Hello" { return "Bonjour" }
    return msg
}
func (frenchTranslator) NGettext(s, p string, n int) string { /* ... */ return s }
func (frenchTranslator) PGettext(_ /*ctx*/, msg string) string { return msg }
func (frenchTranslator) NPGettext(_, s, p string, n int) string { return s }

env, _ := gj.New(
    environment.WithI18NExtension(frenchTranslator{}),
)
tpl, _ := env.FromString(`{% trans %}Hello{% endtrans %}, {{ name }}!`)
out, _ := tpl.Render(map[string]any{"name": "Alice"})
// out == "Bonjour, Alice!"
```

`{% trans %}` / `{% pluralize %}` / `{% endtrans %}` with optional context (`{% trans 'title' %}`), variables (`{% trans name='Bob' %}`), and the `trimmed` modifier all work.

### Persistent on-disk AST cache

For a deployment that re-parses the same templates across processes, pipe a filesystem cache underneath:

```go
import "github.com/jryberg/gojinja/pkg/cache"

c, _ := cache.NewFilesystem("/var/cache/myapp/templates")
env, _ := gj.New(
    gj.WithLoader(myLoader),
    gj.WithExternalCache(c),
)
```

Cache files use atomic temp+rename writes and a magic+SHA-256 header, so a half-written cache file will never be read.

---

## What's supported

### Language constructs

| Construct | Notes |
|---|---|
| `{{ expr }}` | Variable / expression printing. Autoescape on by default. |
| `{% if %}` / `{% elif %}` / `{% else %}` / `{% endif %}` | Standard. |
| `{% for x in seq %}` … `{% else %}` … `{% endfor %}` | Includes the full `loop` object: `index`, `index0`, `revindex`, `revindex0`, `first`, `last`, `length`, `depth`, `depth0`, `previtem`, `nextitem`, `cycle()`, `changed()`. |
| `{% for x in seq recursive %}` | Recursive loops via `loop()`. |
| `{% set name = expr %}` / `{% set name %}…{% endset %}` | Including tuple unpacking and trailing filter. |
| `{% with x = 1, y = 2 %}…{% endwith %}` | Scoped bindings. |
| `{% block name %}…{% endblock %}` / `{% extends "..." %}` / `super()` | Multi-level inheritance, dynamic-name extends fall back to the runtime path. |
| `{% include %}` (with/without context, `ignore missing`) | List form picks the first found. |
| `{% import %}` / `{% from … import … %}` | Macros are closures over their definition site (Python parity). |
| `{% macro %}` … `{% endmacro %}` | Defaults, `varargs`/`kwargs`/`caller` catchers detected from body. |
| `{% call(args) %}…{% endcall %}` | Bidirectional caller closures. |
| `{% filter name %}…{% endfilter %}` | Filter blocks (chains too: `{% filter upper|escape %}`). |
| `{% autoescape on/off %}…{% endautoescape %}` | Scoped autoescape flip. |
| `{% raw %}…{% endraw %}` | Untokenized passthrough. |
| `{% do expr %}` | Always available (not an opt-in extension here). |
| `{% break %}` / `{% continue %}` | Always available. |
| `{# comment #}` | Single-line and multi-line. |
| String escapes | `\\`, `\'`, `\"`, `\a`, `\b`, `\f`, `\n`, `\r`, `\t`, `\v`, octal `\NNN`, `\xHH`, `\uHHHH`, `\UHHHHHHHH`. |
| Numeric literals | int (any size up to int64), float, scientific. |
| Operators | All Python operators Jinja2 supports — `+ - * / // % **`, `~` (concat), `and or not`, `<=` `<` `==` `!=` `>=` `>`, `in`, `not in`. |
| Comparison chains | `a < b < c` works. |
| Conditional expression | `x if cond else y`, `x if cond` (returns Undefined if cond is false). |
| Slicing | `s[1:4]`, `s[::-1]`, `xs[a:b:c]` for strings, lists, tuples. |
| Tuple / list / dict literals | Including unpacking targets in `for` and `set`. |
| Whitespace control | `{%-`, `-%}`, `{{-`, `-}}`, `{#-`, `-#}` plus the global `trim_blocks`, `lstrip_blocks`, `keep_trailing_newline`. |
| Line statements / line comments | Off by default; configurable via `WithLexerOptions`. |

### Filters (54)

Each renders byte-identically to canonical Jinja2 with the same kwargs and edge cases (empty input, `Undefined`, default values, attribute access, etc.).

`abs`, `attr`, `batch`, `capitalize`, `center`, `count`, `d` (alias of `default`), `default`, `dictsort`, `e` (alias of `escape`), `escape`, `filesizeformat`, `first`, `float`, `forceescape`, `format`, `groupby`, `indent`, `int`, `items`, `join`, `last`, `length`, `list`, `lower`, `map`, `max`, `min`, `pprint`, `random`, `reject`, `rejectattr`, `replace`, `reverse`, `round`, `safe`, `select`, `selectattr`, `slice`, `sort`, `string`, `striptags`, `sum`, `title`, `tojson`, `trim`, `truncate`, `unique`, `upper`, `urlencode`, `urlize`, `wordcount`, `wordwrap`, `xmlattr`.

Higher-order filters (`map`, `select`, `reject`, `selectattr`, `rejectattr`, `sort`, `groupby`, `min`, `max`, `sum`) accept Python kwargs (`attribute=`, `default=`, `case_sensitive=`, `reverse=`).

`groupby` and `sum` support **dotted attribute paths** (`'meta.k'`) and numeric tuple indexes (`groupby(0)`).

### Tests (39)

`!=`, `<`, `<=`, `==`, `>`, `>=`, `boolean`, `callable`, `defined`, `divisibleby`, `eq`, `equalto`, `escaped`, `even`, `false`, `filter`, `float`, `ge`, `greaterthan`, `gt`, `in`, `integer`, `iterable`, `le`, `lessthan`, `lower`, `lt`, `mapping`, `ne`, `none`, `number`, `odd`, `sameas`, `sequence`, `string`, `test`, `true`, `undefined`, `upper`.

### Globals (6)

| Global | Purpose |
|---|---|
| `range(start, stop, step)` | Bounded; capped by `WithRangeLimit` (default 100k). |
| `dict({mapping}, k=v, ...)` | Returns an insertion-ordered dict (Python 3.7+ parity). |
| `namespace(d, k=v, ...)` | Creates a `Namespace` for `{% set ns.x = ... %}`. |
| `cycler(*args)` | Cycles through values; methods `.next()`, `.current`, `.reset()`. |
| `joiner(sep)` | Returns sep on every call after the first — for comma-joined output. |
| `lipsum(n=5, html=True, min=20, max=100)` | Lorem-ipsum placeholder text. |

Opt-in globals (registered when their option is set):

| Global | Option | Purpose |
|---|---|---|
| `env(name)` | `WithHostEnv()` | Returns `os.Getenv(name)`. Empty string for unset. |

### Loaders (7)

| Type | Use |
|---|---|
| `DictLoader` | In-memory `map[string]string`. Tests, embedded fixtures. |
| `FileSystemLoader` | One or more allowlisted root directories. Symlinks rejected by default. |
| `EmbedLoader` | Backed by `embed.FS`. |
| `PrefixLoader` | Routes by name prefix to sub-loaders. |
| `ChoiceLoader` | Tries each loader in order. |
| `FuncLoader` | Wraps an arbitrary `Fetch(name)` function. |
| `CachedLoader` | Memoises another loader in process. |

### Synthetic methods on built-in types

Templates can call Python instance methods on Go values without any setup:

| Type | Methods exposed |
|---|---|
| `string` / `Markup` | `upper`, `lower`, `title`, `capitalize`, `swapcase`, `strip`, `lstrip`, `rstrip`, `split`, `rsplit`, `splitlines`, `join`, `startswith`, `endswith`, `find`, `rfind`, `index`, `rindex`, `count`, `replace`, `format`, `isdigit`, `isalpha`, `isalnum`, `isspace`, `isupper`, `islower`, `encode`, `zfill`, `ljust`, `rjust`, `center`. |
| `[]any` | `count`, `index`. |
| `map[string]any`, `map[any]any`, `*runtime.OrderedDict` | `items`, `keys`, `values`, `get`. |

Method lookup precedes key lookup on `.attr` access (Python parity); `[]` still goes straight to keys, so `d['items']` returns the `'items'` key while `d.items` returns the method.

### Extensions

| Extension | Status |
|---|---|
| `do` | Always available. |
| `loopcontrols` (`break`/`continue`) | Always available. |
| `debug` (`{% debug %}`) | Opt-in: `environment.WithDebugExtension()`. |
| `i18n` (`{% trans %}` / `{% pluralize %}`) | Opt-in: `environment.WithI18NExtension(translator)`. |
| Custom user extensions | `environment.WithExtension(tag, parseFunc)`. |

---

## What's *not* supported

These are deliberate divergences. Each was a conscious trade-off, not an oversight.

| Feature | Why it's missing | Workaround |
|---|---|---|
| `enable_async` / `async def` filters / `async for` | Different runtime model; Go has no `await`. | `Template.RenderContext(ctx, vars)` cancels mid-render through `context.Context`. |
| Python-runtime serialization (the `pickle` module) | Python-runtime specific. | Use `pkg/cache.Filesystem` for cross-process AST caching. |
| `\N{NAME}` Unicode-named escapes in string literals | Requires shipping a 32K-entry Unicode names table. | Other Python escapes (`\n`, `\xNN`, `\uNNNN`, `\UNNNNNNNN`, octal `\NNN`) all work. |
| Memcached bytecode cache | Avoid hard external deps. | Userland adapter satisfying the `Cache` interface. |
| `PackageLoader` (Python zipfile-aware) | Replaced by `EmbedLoader`. | Use `embed.FS`. |
| Python `__html__` / `__str__` dunder dispatch on arbitrary objects | Go has no dunder methods. | User Go types implement `escape.HTMLer` (for safe HTML) or `fmt.Stringer` (for custom string form). |

### Stricter defaults than Python

These aren't divergences in *capability* — gojinja can do everything Python does — but the defaults are stricter for security.

| Default | Python Jinja2 | gojinja | Opt-out |
|---|---|---|---|
| Sandbox | off | **on** | `gj.WithUnsafe()` |
| Autoescape | off | **on** (`AutoescapeAlways{}`) | `gj.WithAutoescape(gj.AutoescapeNever{})` |
| Host environment access | available via globals | **off** | `gj.WithHostEnv()` |
| `range` size limit | unbounded (sandbox: 100k) | **always bounded** (default 100k) | `gj.WithRangeLimit(n)` |
| Template cache size | 400 (with `-1` = unbounded) | **always bounded** (`-1` rejected) | `gj.WithCacheSize(n)` |
| Context cancellation | not exposed | **`RenderContext(ctx, vars)`** | n/a — always present |

---

## Security defaults vs. Python Jinja2

The library is **secure by default, opt out explicitly**. Defaults that require user opt-in (`os.Getenv` exposure, raw filesystem access, unbounded loops, unbounded caches) are *off*.

If you're rendering templates supplied by your application's *own* code, the default sandbox costs almost nothing. If you're rendering user-supplied templates, the sandbox keeps `__class__`/`__subclasses__`-style escapes off the table.

`WithUnsafe()` is the deliberate, audited opt-out. **Don't enable it for user-supplied templates.**

---

## API reference

Most consumers only need the top-level `github.com/jryberg/gojinja` package, which re-exports the user-facing types.

### Options

```go
gj.New(
    gj.WithLoader(loader),                    // template source
    gj.WithAutoescape(gj.AutoescapeAlways{}), // policy
    gj.WithGlobal("now", time.Now),           // expose a Go func/value as a global
    gj.WithFilter("echo3", filter),           // register a custom filter
    gj.WithTest("startsWithA", test),         // register a custom test
    gj.WithUndefined(runtime.NewStrict),      // strict undefined raises immediately
    gj.WithRangeLimit(50_000),                // tighten the range cap
    gj.WithCacheSize(1024),                   // larger in-memory cache
    gj.WithExternalCache(fsCache),            // persist parsed AST to disk
    gj.WithLexerOptions(lexerOpts),           // change block markers, line statements, etc.
    gj.WithUnsafe(),                          // turn off sandbox (audit before use!)
    gj.WithHostEnv(),                         // expose os.Getenv as the `env` global
)
```

### Autoescape policies

```go
gj.AutoescapeAlways{}                                        // always on (default)
gj.AutoescapeNever{}                                         // always off
gj.AutoescapeByExtension{Enabled: []string{"html", "htm"}}   // by file extension
```

### Undefined variants

```go
import "github.com/jryberg/gojinja/pkg/runtime"

runtime.NewBase       // default — silent, renders empty, errors on iteration
runtime.NewChainable  // attribute access on Undefined returns Undefined
runtime.NewDebug      // prints "{{ name }}"-style placeholder for debugging
runtime.NewStrict     // raises on any access
```

Wire via `gj.WithUndefined(runtime.NewStrict)`.

### Custom filter / test signatures

```go
type FilterFunc func(env *Environment, ctx *runtime.Context, value any, args []any, kwargs map[string]any) (any, error)

type TestFunc   func(env *Environment, ctx *runtime.Context, value any, args []any, kwargs map[string]any) (bool, error)
```

Wrap as `environment.Filter{Func: ...}` or `environment.Test{Func: ...}` and pass via `gj.WithFilter(name, ...)` / `gj.WithTest(name, ...)`.

For globals, register any Go function: gojinja dispatches via reflection, with optional support for `KwargsCallable` if you want to receive Python-style keyword arguments:

```go
type KwargsCallable func(args []any, kwargs map[string]any) (any, error)
```

---

## CLI

A small `gojinja` command lives in `cmd/gojinja` for one-shot template rendering:

```bash
go install github.com/jryberg/gojinja/cmd/gojinja@latest

gojinja render \
    --template path/to/template.html \
    --vars     path/to/vars.json \
    --out      result.html
```

Flags:

| Flag | Default | Effect |
|---|---|---|
| `--template` | (required) | Path to the entry template. |
| `--vars` | `""` | JSON file with the render context. |
| `--out` | `-` (stdout) | Output path. |
| `--root` | (template's dir) | Allow-listed loader root. Repeatable. |
| `--unsafe` | false | Turn off the sandbox. |
| `--no-autoescape` | false | Turn off autoescape. |
| `--host-env` | false | Register the `env()` global, backed by `os.Getenv`. |
| `--max-range` | 100000 | Override the range cap. |

Sandbox, autoescape, and host-env access default to safe; opt-out is always explicit.

---

## License

See [LICENSE](LICENSE).
