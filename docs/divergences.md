# Documented divergences from canonical Python Jinja2

gojinja exists to give Go programs the exact same template-rendering behaviour
as Python's [Jinja2](https://github.com/pallets/jinja), byte-for-byte. **Every
template that renders in canonical Python Jinja2 must render identically in
gojinja.** When a parity-corpus case fails, the right response is to fix
gojinja, not to delete or weaken the case.

The table below is the **only** approved set of divergences. Expanding it
requires explicit maintainer approval.

| Topic | Jinja2 default / behavior | gojinja behavior | Reason |
|---|---|---|---|
| Sandbox | `Environment` is unsandboxed; `SandboxedEnvironment` is opt-in. | The default `Environment` is sandboxed. An explicit `WithUnsafe()` option enables un-sandboxed access. | "Secure code before beauty." |
| Autoescape | `autoescape=False` default. | `autoescape=true` default. Opt out via option. | Default safe HTML. |
| Host environment access | None by default; users add it via globals. | Disallowed by default. An explicit option (e.g. `WithHostEnv()`) registers `os.Getenv` as a global. | Security-first. |
| Loaders | `FileSystemLoader(followlinks=False)` default but otherwise lenient. | `FileSystemLoader` validates against an allowlisted root; `followlinks` defaults False; symlink crossing the root is rejected. | Hardening. |
| `range`/loop DoS | sandbox uses `safe_range(MAX_RANGE=100000)`; non-sandbox unbounded. | Always bounded. Limit is configurable. | Hardening. |
| Cache size | `cache_size=400` default; `-1` allows unbounded. | Hard upper bound enforced; `-1` rejected. | Hardening. |
| Async | Python async/await via `enable_async`. | Not mirrored. We provide `context.Context` cancellation throughout the API instead. Tests in `test_async*.py` are **not** ported; equivalent cancellation tests are added. | Different runtime model. |
| Python-object serialization | Upstream Python-only object-marshalling test suite. | Not implemented — Python-runtime specific. | N/A in Go. |
| `PackageLoader` | Loads from arbitrary Python packages incl. zipfiles. | Replaced with `EmbedLoader` backed by `embed.FS`. | Idiomatic Go; safer. |
| `MemcachedBytecodeCache` | Built-in. | Not built in. We expose a `Cache` interface and ship `MemoryCache` and `FilesystemCache`. Memcached is a userland adapter. | Fewer dependencies. |
| `i18n` translations | Python `gettext` based. | We accept a `Translator` interface implementing `Gettext`/`NGettext`/`PGettext`/`NPGettext`. | Idiomatic Go; no `gettext` C dep. |
| Bytecode | Python bytecode via `marshal`. | We cache parsed AST + lowered IR with a magic + checksum. Same on-disk format design (magic + checksum + payload), but payload is our IR. | We don't have Python bytecode. |
| Tracebacks | Synthetic Python frames pointing to template line. | Errors carry `(template name, line, col, source excerpt)` and a Go stack; we don't fake Go stack frames. | Idiomatic Go errors. |
| `\N{NAME}` Unicode named escapes in string literals | Supported via Python's `unicode-escape` codec. | **Not supported.** Strings using `\N{LATIN SMALL LETTER A}` raise a `TemplateSyntaxError`. All other Python string escapes (`\n`, `\t`, `\xNN`, `\uNNNN`, `\UNNNNNNNN`, `\NNN` octal) are supported. | Requires shipping a 32K-entry Unicode names table for marginal template-engine value. Can revisit. |

## Reference

- **Upstream source:** <https://github.com/pallets/jinja>
- **Pinned upstream commit:** `5ef70112a1ff19c05324ff889dd30405b1002044` (Jinja2 `main`, 2025-06-14)
- **Verified upstream counts at the pin:** 49 lexer tokens · 71 AST classes · 54 filter entries (52 unique + 2 aliases) · 39 test entries (22 unique + 17 aliases) · 8 i18n gettext functions · 679 Python tests across 22 test files.

To bump the pin, update the commit hash above and the matching reference in
`DEVELOPMENT.md`, with a note on the rationale, and re-run `make parity`
before merging.
