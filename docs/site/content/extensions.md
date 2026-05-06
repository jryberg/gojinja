# Extensions

gojinja ships a small set of first-party Jinja2 extensions, plus a few that
are built into the parser itself rather than opt-in.

## Built into the parser (no opt-in needed)

- **`do`** — `{% do list.append(x) %}`, expression-as-statement.
- **`break` / `continue`** — loop control.
- **`autoescape` block** — `{% autoescape true %}...{% endautoescape %}`.
- **`with`** — `{% with foo = bar %}...{% endwith %}`.

## Opt-in extensions

These live in `pkg/ext` and are registered at `Environment` construction
time:

- **`i18n`** — `{% trans %}` / `{% pluralize %}` / `{{ _(...) }}` backed by
  a `Translator` interface you supply (`Gettext`, `NGettext`, `PGettext`,
  `NPGettext`). Replaces Python Jinja2's `gettext` integration with an
  idiomatic Go interface — no C `gettext` dependency.
- **`debug`** — `{% debug %}` dumps the active context for debugging.

!!! note
    Full reference for each extension and the `Translator` interface
    contract coming in a follow-up. See
    [pkg.go.dev/.../pkg/ext](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ext).
