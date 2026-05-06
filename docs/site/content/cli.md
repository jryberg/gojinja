# Command-line tool

gojinja ships a small CLI under `cmd/gojinja` for one-off rendering and
pipeline use.

## Install

```bash
go install github.com/jryberg/gojinja/cmd/gojinja@latest
```

Or download a pre-built binary from the [GitHub releases
page](https://github.com/jryberg/gojinja/releases).

## Render a template

```bash
gojinja render --template page.j2 --vars vars.json
```

`--vars` accepts a JSON file (object at the top level). The rendered
template goes to stdout.

!!! note "Coming in a follow-up"
    Full flag reference (sandbox controls, autoescape policy, custom
    delimiters, output file, exit codes) lands on this page next.
