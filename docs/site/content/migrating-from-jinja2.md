# Migrating from Python Jinja2

The project's hard rule is **byte-for-byte template parity** with Python
Jinja2. If your `.j2` files render correctly in Python, they will render
identically in gojinja.

What changes is the **host-side API** (you're now writing Go, not Python)
and a small set of **safe-by-default inversions**.

## What stays the same

- All template syntax: `{{ ... }}`, `{% ... %}`, `{# ... #}`, whitespace
  control (`{%- -%}`), expressions, control structures, inheritance,
  macros, imports, includes.
- All filters and tests — same names, same arguments, same semantics.
  Aliases (`d` → `default`, `e`/`escape`, `count` → `length`) work too.
- All builtin globals — `range`, `dict`, `lipsum`, `cycler`, `joiner`,
  `namespace`.
- Autoescape semantics, including `{% autoescape %}` blocks and the
  `safe`/`escape`/`forceescape` filters.
- Template inheritance (`extends`, `block`, `super`, scoped blocks).
- Sandbox attribute-access rules.

## What changes (the safe-by-default inversions)

The full list lives in [Documented divergences](divergences.md). The
short version:

| Topic | Python Jinja2 default | gojinja default |
|---|---|---|
| Sandbox | off (opt-in to `SandboxedEnvironment`) | **on** (opt-out via `WithUnsafe()`) |
| Autoescape | off | **on** |
| Host env access | none (you wire `os.environ`) | none (you wire `WithHostEnv()`) |
| `range` cap | unbounded (sandbox: 100k) | **always bounded** (configurable) |
| Cache `-1` | unbounded | **rejected** |
| `FileSystemLoader` | lenient | **allowlisted root, symlink-rejecting** |

## What changes (Go vs Python idioms)

- Your `Environment` is constructed with **variadic options**, not
  keyword arguments: `gj.New(gj.WithLoader(l), gj.WithAutoescape(...))`.
- Async support is provided through **`context.Context` cancellation**
  throughout the API, not Python-style `enable_async` / `async def`
  filters.
- The Python `PackageLoader` (which can read from zipfiles) is replaced
  with `EmbedLoader`, backed by Go's `embed.FS`. Same purpose,
  Go-idiomatic shape.
- The bytecode cache stores **gojinja's own AST format** (gob + magic +
  SHA-256) — not Python `marshal` bytecode. Same on-disk shape (atomic
  writes, integrity checks), different payload.
- `i18n` translations use a `Translator` Go interface instead of GNU
  `gettext`.
- Errors carry `(template name, line, col, source excerpt)` plus a Go
  stack — no synthetic Python frames.

## Practical migration steps

1. Drop your existing `.j2` files into the new `templates/` directory.
2. Wire a `FileSystemLoader` (or `EmbedLoader` if you want the templates
   in the binary) to point at it.
3. If your templates rendered with autoescape **off**, pass
   `gj.WithAutoescape(gj.AutoescapeNever{})` — or, better, audit each
   template for places that needed `safe` and remove them.
4. If you used `os.environ` from inside templates, wire
   `gj.WithHostEnv()`. Resist enabling it for templates supplied by
   untrusted users.
5. Replace `register_filter(...)` Python calls with
   `gj.WithFilter("name", goFunc)` options at construction.
6. If you used `range(0, 1_000_000)` anywhere, set
   `gj.WithRangeLimit(n)` to a value high enough for your real use.

## Things that don't have a gojinja equivalent

- `enable_async` / `async def` filters — use
  [`context.Context`](https://pkg.go.dev/context) cancellation instead.
- `\N{LATIN SMALL LETTER A}`-style Unicode named escapes inside string
  literals (everything else — `\n`, `\t`, `\xNN`, `\uNNNN`, `\UNNNNNNNN`,
  octal — is supported). See [divergences](divergences.md) for the
  rationale.
- Memcached bytecode cache as a built-in — implement the small `Cache`
  interface and you have it.
