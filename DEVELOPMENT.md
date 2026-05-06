# Developing gojinja

This document is for **contributors** to gojinja itself. If you're using the library in your application, see [README.md](README.md).

## Project mission

gojinja is a Go implementation of [Jinja2](https://github.com/pallets/jinja) (the template engine). The bar the project sets for itself:

> A template that renders in canonical Python Jinja2 should render byte-identically in gojinja.

This rule is non-negotiable. When a parity-corpus case fails, **fix gojinja, don't delete the case**. The deliberate-divergences table lives in [`docs/divergences.md`](docs/divergences.md).

## Table of contents

- [Setup](#setup)
- [Make targets](#make-targets)
- [Parity testing — the regression gate](#parity-testing--the-regression-gate)
  - [Corpus layout](#corpus-layout)
  - [Adding a corpus case](#adding-a-corpus-case)
  - [Driving from canonical Jinja2 tests](#driving-from-canonical-jinja2-tests)
- [Repository layout](#repository-layout)
- [Working principles](#working-principles)
- [Security audit checklist](#security-audit-checklist)
- [Adding new behaviour](#adding-new-behaviour)
  - [Adding a filter](#adding-a-filter)
  - [Adding a test (the Jinja2 kind)](#adding-a-test-the-jinja2-kind)
  - [Adding a parser tag / extension](#adding-a-parser-tag--extension)
- [Documentation files](#documentation-files)
- [Release process](#release-process)
- [Reference repo](#reference-repo)

---

## Setup

```bash
git clone https://github.com/jryberg/gojinja
cd gojinja
go build ./...
go test -race ./...
```

Requirements:
- Go 1.22+
- For the parity harness: `python3` with `jinja2` installed on `$PATH` (`python3 -m pip install jinja2`).
- Optional: `staticcheck` for `make lint`.

The reference Python implementation is expected at a sibling directory outside this repo (e.g. `../jinja-reference/`), checked out at the pinned upstream commit `5ef70112a1ff19c05324ff889dd30405b1002044` (see [`docs/divergences.md`](docs/divergences.md#reference)). It is **not a Go dependency** — gojinja never imports from it; the clone is used for research only.

## Make targets

```bash
make build   # go build ./...
make test    # go test -race ./...
make vet     # go vet ./...
make lint    # staticcheck ./... (skipped if not installed)
make audit   # smoke checks: no os/exec, no unsafe, no CGo, no third-party deps in pkg/cmd
make tidy    # go mod tidy
make parity  # run the Python-vs-gojinja parity harness
```

`make audit` is intentionally a thin grep-based check, not a substitute for the methodical walk in `docs/security-audit-v0.1.0.md`. The full per-section walk is run before each release tag and recorded as a fresh `docs/security-audit-vX.Y.Z.md`.

## Parity testing — the regression gate

```bash
make parity
```

Renders every curated template through both gojinja and canonical Python Jinja2 and diffs the output byte-by-byte. **Zero diffs** is the only passing state.

The corpus is **a regression gate, not a coverage menu**. New features ship with corpus cases that lock in their parity. When a case fails, the right response is to fix gojinja so the diff disappears — never to delete or weaken the case. Documented divergences (the small handful tabulated in [`docs/divergences.md`](docs/divergences.md)) are the only valid reason a Python template might *not* render identically in gojinja.

### Corpus layout

```
tools/parity/
├── main.go        # Go orchestrator: discovers cases, renders both sides, diffs
├── parity.py      # Python helper invoked per-case via os/exec
└── corpus/
    ├── 01_plain.j2
    ├── 01_plain.vars.json                  # flat case
    ├── …
    └── 60_inheritance_basic/               # multi-file case
        ├── _main.j2                        # entry template
        ├── _main.vars.json                 # render context
        ├── layout.j2                       # sibling template, loadable by name
        └── …
```

A case is either:
- **Flat:** `<name>.j2` + `<name>.vars.json`. The harness renders `<name>.j2` with the supplied vars and compares.
- **Directory:** `<name>/_main.j2` + `<name>/_main.vars.json` + zero or more sibling `*.j2` files. The harness wires the siblings into a `DictLoader` (gojinja side) and a `jinja2.DictLoader` (Python side) so `{% extends %}` / `{% include %}` / `{% import %}` resolve identically.

Both sides run with `autoescape=False, keep_trailing_newline=True` and the `do` + `loopcontrols` extensions enabled — these are the gojinja-default behaviours, applied to Python for parity.

The Go orchestrator decodes the JSON vars file via `json.Decoder.Token()` into a `*runtime.OrderedDict` so dict iteration order survives the harness boundary. Without this, `{% for k in d %}` over a JSON-loaded dict would diverge non-deterministically.

### Adding a corpus case

```bash
# 1. Write the template
cat > tools/parity/corpus/99_my_case.j2 <<'EOF'
{{ greeting|upper }}, {{ name }}!
EOF

# 2. Write the vars
cat > tools/parity/corpus/99_my_case.vars.json <<'EOF'
{"greeting": "hello", "name": "Alice"}
EOF

# 3. Run
make parity
```

If the new case passes, you've locked in parity for that template. If it fails, the diff output points at exactly where gojinja differs from Python — fix the engine, not the case.

For a multi-file scenario:

```bash
mkdir tools/parity/corpus/99_my_inheritance
cat > tools/parity/corpus/99_my_inheritance/_main.j2 <<'EOF'
{% extends "base" %}{% block body %}Hello, {{ name }}{% endblock %}
EOF
cat > tools/parity/corpus/99_my_inheritance/_main.vars.json <<'EOF'
{"name": "World"}
EOF
cat > tools/parity/corpus/99_my_inheritance/base.j2 <<'EOF'
[{% block body %}default{% endblock %}]
EOF
make parity
```

Filter cases:

```bash
go run ./tools/parity -filter 99_      # only run cases whose names contain "99_"
go run ./tools/parity -v               # verbose: log every passing case
```

### Driving from canonical Jinja2 tests

The richest source of corpus cases is the canonical Jinja2 test suite, under `tests/` in your sibling Jinja2 clone. The current corpus is drawn from:

| Jinja2 test file | gojinja corpus IDs |
|---|---|
| `test_filters.py` | `30_filters_a_strings` … `36_unique_minmax_groupby` |
| `test_tests.py` | `40_tests_basic` |
| `test_core_tags.py` | `50_for_loops` `51_macros` `52_set_with_namespace` `53_if_elif` |
| `test_inheritance.py` | `60_inheritance_basic` `61_inheritance_levels` |
| `test_imports.py` | `62_include_import` `63_import_context` |
| `test_lexnparse.py` | `70_lex_parse` `71_string_escapes` |
| `test_regression.py` | `80_regression_scoping` `81_regression_keyword_folding` `82_regression_misc` |
| `test_api.py` | `90_attribute_lookup` `91_undefined_modes` |

When porting a Python test to a corpus case, cite the source file in a leading `{# … #}` comment so future readers can cross-check.

## Repository layout

```
gojinja/
├── gojinja.go            # public façade — single-import surface for end users
├── aliases.go
├── cmd/gojinja/          # one-shot CLI (render --template …)
├── pkg/
│   ├── ast/              # 71 node types mirroring jinja2.nodes
│   ├── cache/            # Memory + Filesystem AST caches
│   ├── debug/            # Error wrapping with template + line + source excerpt
│   ├── environment/      # Environment, Template, GetAttr/GetItem, builtins
│   ├── errors/           # Typed error structs (TemplateSyntaxError, etc.)
│   ├── escape/           # Markup + HTMLer + Python-parity str/repr
│   ├── eval/             # Tree-walking evaluator
│   ├── ext/              # Debug / i18n extensions
│   ├── filters/          # (intentionally empty — filters live in pkg/environment)
│   ├── globals/          # (intentionally empty — globals live in pkg/environment)
│   ├── idtracking/       # Symbols / FrameSymbolVisitor / RootVisitor
│   ├── lexer/            # Tokens, options, state machine, fuzz target
│   ├── loader/           # Dict / FileSystem / Embed / Prefix / Choice / Func / Cached
│   ├── meta/             # FindUndeclaredVariables, FindReferencedTemplates
│   ├── native/           # JSON-typed render output (NativeEnvironment equivalent)
│   ├── optimizer/        # Constant-folding pass
│   ├── parser/           # Statement & expression parser
│   ├── runtime/          # Context, LoopContext, Namespace, OrderedDict, Tuple, …
│   ├── sandbox/          # IsSafeAttribute, IsSafeCallable, MutatingMethods
│   └── tests/            # (intentionally empty — Jinja2 tests live in pkg/environment)
├── tools/parity/         # parity harness (Go orchestrator + Python helper + corpus)
└── docs/
    ├── architecture.md
    ├── security-audit-checklist.md
    └── security-audit-v0.1.0.md
```

The empty `pkg/filters`, `pkg/globals`, and `pkg/tests` packages exist as future hooks for out-of-tree filter/global/test packs. The canonical lists live in `pkg/environment/builtins.go` and `pkg/environment/filters_extra.go`; the empty packages do not import anything and exist only to claim namespace.

## Working principles

These rules are durable — every contribution must respect them.

1. **Security-first.** Sandbox is the default. Host environment access, filesystem loaders, and any IO require explicit opt-in flags. No "do everything by default" behaviour — Python Jinja2's defaults are explicitly insecure for our purposes.
2. **No placeholder code.** Never write a function whose body says "TODO" or returns a stub. Plan deeply enough that every function shipped is fully implemented.
3. **Few external Go packages.** Stdlib only by default. Adding a Go module requires user approval first, with justification (active maintenance, wide adoption, or "we'd otherwise rewrite it").
4. **Tests are functional copies of Python tests** (when one exists). Tests verify intended behaviour — never edit a test or simplify a corpus case to make broken code pass.
5. **No legacy commentary.** Don't write `// formerly used X` or `// removed legacy Y`. Just write the code as it is now.
6. **Comments are short and useful.** Skip filler ("defence in depth", "for safety"). Only write a comment when *why* is non-obvious.
7. **Modular, no duplication.** Extract shared helpers when two pieces of code do the same thing.
8. **Full template-level Python parity is non-negotiable.** Every template valid in canonical Python Jinja2 must render identically in gojinja. Documented divergences live solely in [`docs/divergences.md`](docs/divergences.md) and require explicit approval to expand.

## Security audit checklist

`docs/security-audit-checklist.md` is the canonical 10-section checklist run before each release. The most recent walk is recorded in `docs/security-audit-v0.1.0.md` with file/line citations.

The hard rules the audit enforces:

- No `unsafe` import outside packages explicitly approved by the user.
- No `os/exec` in `pkg/` or `cmd/`. (The parity harness in `tools/parity/` may use `os/exec` because it is dev tooling, not part of the library.)
- No `net/*` imports.
- No CGo.
- File I/O is restricted to `pkg/loader` and `pkg/cache` (plus the CLI which opens user-supplied paths).
- `range`, parser depth, and template cache are all bounded.
- Errors don't leak filesystem paths beyond the configured loader root.
- `Environment` is safe for concurrent `Render` calls.

`make audit` runs the smoke-grep version of these checks. The full walk is manual.

## Adding new behaviour

### Adding a filter

1. Read the canonical Python implementation in `src/jinja2/filters.py` of your sibling Jinja2 clone. Cite the function name in a Go comment.
2. Add the implementation to `pkg/environment/builtins.go` (or `filters_extra.go` if it's a non-trivial filter that benefits from its own file). The signature is:
   ```go
   func filterFoo(env *Environment, ctx *runtime.Context, value any, args []any, kwargs map[string]any) (any, error)
   ```
3. Register it in `registerBuiltins`:
   ```go
   e.filters["foo"] = Filter{Func: filterFoo}
   ```
4. Add a corpus case under `tools/parity/corpus/` driving the new filter against Python's behaviour, with realistic edge cases (empty input, `Undefined`, kwargs).
5. Run `make parity` and `make test`.

### Adding a test (the Jinja2 kind)

Same shape, but in the `tests` namespace and the func returns `(bool, error)`:

```go
e.tests["startswith"] = Test{Func: testStartswith}

func testStartswith(env *Environment, ctx *runtime.Context, value any, args []any, kwargs map[string]any) (bool, error) {
    // ...
}
```

### Adding a parser tag / extension

For a tag that should always be available (like `do`), add it directly to `pkg/parser/parser.go` near the existing built-in handlers (`parseDo`, `parseBreak`, etc.).

For an opt-in extension, mirror the pattern of `pkg/ext/debug.go` and `pkg/ext/i18n.go`:

1. Define a `parser.ExtensionFunc` that consumes the tag and returns an `ast.Node`.
2. Register it via `environment.WithExtension(tagName, parseFunc)`.
3. If the extension needs a runtime helper, register that as a global with a `__gojinja_*__` prefix.
4. Add a convenience option (`environment.WithFooExtension()`) that wires both in one call.

## Documentation files

| File | Purpose |
|---|---|
| `README.md` | End-user documentation. Install, use cases, supported features, API. |
| `DEVELOPMENT.md` | This file — contributor documentation. |
| `CHANGELOG.md` | User-facing release notes. |
| `docs/divergences.md` | The hard parity rule and the deliberate-divergences table. The exhaustive list of cases where gojinja's output may differ from canonical Python Jinja2. |
| `docs/architecture.md` | ASCII data-flow diagram + per-package ownership table. |
| `docs/security-audit-checklist.md` | The 10-section audit run before each release. |
| `docs/security-audit-v0.1.0.md` | Most recent audit walk with file/line citations. |

When picking up the codebase in a fresh session, the recommended reading order is:

1. [`docs/divergences.md`](docs/divergences.md) — the parity rule and the deliberate divergences.
2. The Python source in your sibling clone of [Jinja2](https://github.com/pallets/jinja) (see [Reference repo](#reference-repo)) for whichever feature you're touching.

## Release process

Releases are automated via [release-please](https://github.com/googleapis/release-please)
and [GoReleaser](https://goreleaser.com).

The flow:

1. Contributors merge PRs into `main`. Each PR has a Conventional Commits
   title; release-please reads those titles to compute the next version and
   build the changelog block.
2. Release-please opens (or updates) a single open PR titled
   `chore(main): release X.Y.Z`. The PR body shows the next version and the
   generated changelog entry.
3. When the maintainer is ready to ship, they:
   1. Walk `docs/security-audit-checklist.md` end-to-end.
   2. Save the result as `docs/security-audit-vX.Y.Z.md` with file/line
      citations and commit it (one extra commit on `main`, which becomes the
      last commit before the release-please PR is merged).
   3. Run the full local suite: `make ci` (build + vet + test + audit +
      parity).
   4. Merge the release-please PR.
4. Merging the release-please PR causes release-please to:
   - Update `CHANGELOG.md` and `.release-please-manifest.json` on `main`.
   - Tag `vX.Y.Z`.
   - Create the GitHub Release with the generated notes.
5. The tag push triggers `.github/workflows/release.yml`, which runs
   GoReleaser to build the `gojinja` CLI for linux/darwin/windows ×
   amd64/arm64 and uploads the archives + checksums + SBOM to the release.

The maintainer never edits `CHANGELOG.md` by hand; it is fully derived from
the merged commit history.

### First release

The repo starts with no tags. The first commits that land on `main` after
the release automation is enabled (CI files, docs rewrites, etc.) are
collected by release-please into the first release PR. Merging that PR
cuts `v0.1.0` if any `feat:` commits are in the window (otherwise
`v0.0.1`), creates the `CHANGELOG.md`, and triggers the GoReleaser
workflow that builds and uploads the CLI binaries.

## Reference repo

The canonical Python Jinja2 source is expected at a sibling directory outside this repo (e.g. `../jinja-reference/`), checked out at the pinned commit recorded in [`docs/divergences.md`](docs/divergences.md#reference). It is **research-only**: gojinja never imports from it.

When implementing a feature:
- Read the Python source for the equivalent function before writing Go.
- Cite the file + class/function in a Go comment when the behaviour you're matching isn't obvious.
- For tests, name the original test function (e.g. `// Mirrors test_filters.py::TestFilter::test_default`) so future readers can cross-check.

If you need to bump the pinned reference commit, update the commit hash in [`docs/divergences.md`](docs/divergences.md#reference) (and the matching reference at line 48 above) with rationale, and re-run `make parity` before merging.
