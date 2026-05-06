# Installation

gojinja is a single Go module with **zero external dependencies**.

## Library

```bash
go get github.com/jryberg/gojinja@latest
```

Requires Go 1.22 or newer.

```go
import gj "github.com/jryberg/gojinja"

env, _ := gj.New()
tpl, _ := env.FromString("Hello, {{ name | upper }}!")
out, _ := tpl.Render(map[string]any{"name": "World"})
```

The root package re-exports the types and constructors most consumers reach
for. Specialised use cases (custom AST inspection, parity tooling, the
lexer) can import the per-feature subpackages directly — see
[the API overview](api/index.md).

## Command-line tool

A small CLI ships under `cmd/gojinja`:

```bash
go install github.com/jryberg/gojinja/cmd/gojinja@latest

gojinja render --template page.j2 --vars vars.json
```

Pre-built binaries for Linux, macOS, and Windows are published with each
release on the [GitHub releases
page](https://github.com/jryberg/gojinja/releases).

See the [CLI guide](cli.md) for details.

## Verifying parity with Python Jinja2

The project's main correctness criterion is *byte-for-byte identical output*
to canonical Python Jinja2. If you maintain a parity-sensitive template, you
can run the parity harness yourself:

```bash
git clone https://github.com/jryberg/gojinja
cd gojinja
make parity   # requires python3 + jinja2 on $PATH
```
