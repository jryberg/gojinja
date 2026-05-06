---
title: gojinja
hide:
  - navigation
  - toc
---

# gojinja

A **Jinja2-compatible template engine for Go**, built around a single hard
guarantee: every template that renders in [canonical Python
Jinja2](https://github.com/pallets/jinja) renders **byte-for-byte
identically** in gojinja.

```go
env, _ := gojinja.New(gojinja.WithAutoescape(gojinja.AutoescapeNever{}))
tpl, _ := env.FromString("Hello, {{ name | upper }}!")
out, _ := tpl.Render(map[string]any{"name": "World"})
// → "Hello, WORLD!"
```

<div class="grid cards" markdown>

-   :material-rocket-launch:{ .lg .middle } **Get started**

    ---

    Install gojinja and render your first template in under a minute.

    [:octicons-arrow-right-24: Quick start](quickstart.md)

-   :material-book-open-variant:{ .lg .middle } **Template language**

    ---

    Variables, filters, tests, control structures, inheritance, and the
    full template designer reference.

    [:octicons-arrow-right-24: Designer docs](templates/designer.md)

-   :material-shield-lock:{ .lg .middle } **Safe by default**

    ---

    Sandboxed environment, HTML autoescape on, hardened loaders, bounded
    `range`. Opt out only when you know you need to.

    [:octicons-arrow-right-24: Sandbox guide](sandbox.md)

-   :material-package-variant:{ .lg .middle } **API reference**

    ---

    Curated tour of `Environment`, `Loader`, `Cache`, and the runtime
    types — with deep links to the full reference on pkg.go.dev.

    [:octicons-arrow-right-24: API overview](api/index.md)

</div>

## Why gojinja

- **Template-level parity with Python Jinja2.** The same `.j2` files run on
  both sides without modification. Every release is gated by a parity
  harness that diffs gojinja's output byte-for-byte against canonical
  Jinja2.
- **Idiomatic Go API.** `context.Context` cancellation, variadic options,
  one stdlib-style `Environment` constructor. No `enable_async`, no
  Python-shaped two-arg constructors.
- **Stdlib only.** No third-party Go dependencies. The CLI ships as a
  single static binary.
- **Secure defaults.** The default `Environment` is sandboxed,
  autoescape is on, host environment access is off, `FileSystemLoader`
  enforces an allowlisted root, and `range` is bounded. Each default is
  documented and opt-out is explicit.

## Where to next

- [Quick start](quickstart.md) — render your first template.
- [Template designer](templates/designer.md) — the full template language.
- [Filters](templates/filters/index.md) · [Tests](templates/tests/index.md)
  · [Globals](templates/globals/index.md)
- [Migrating from Python Jinja2](migrating-from-jinja2.md) — what stays
  the same, what changes.
- [Documented divergences](divergences.md) — every intentional difference
  from Python Jinja2.
