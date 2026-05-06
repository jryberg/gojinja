# Template designer reference

This page is the language reference for templates rendered through
gojinja. It mirrors the structure of [Pallets' template designer
documentation][pallets-designer]. Behaviour is **byte-for-byte
identical** to canonical Python Jinja2 — every divergence is called out
inline as a `!!! warning` admonition pointing at the
[divergences page](../divergences.md).

[pallets-designer]: https://jinja.palletsprojects.com/en/stable/templates/

!!! info "Adapted from Jinja2"
    Sections of this page are adapted from the Pallets project's Jinja2
    documentation, which is licensed under
    [BSD-3-Clause](https://github.com/pallets/jinja/blob/main/LICENSE.txt).
    Where gojinja diverges, the differences are documented inline.

## Synopsis

A template is plain text with **expressions** delimited by `{{ ... }}`,
**statements** delimited by `{% ... %}`, and **comments** delimited by
`{# ... #}`. Anything outside those markers is emitted unchanged.

```jinja
<!DOCTYPE html>
<title>{{ title }}</title>
<ul>
{% for user in users %}
  <li><a href="{{ user.url }}">{{ user.name }}</a></li>
{% endfor %}
</ul>
{# A comment for template authors. Not emitted. #}
```

## Variables

Render a variable with `{{ name }}`. Attribute access uses dot syntax
(`{{ user.name }}`); item access uses square brackets
(`{{ users[0] }}`, `{{ env['HOME'] }}`).

Both forms are equivalent for Go map access; for struct-typed values,
the dot form invokes Go's reflection-based field lookup, while the
bracket form does not. Prefer the dot form unless the key is dynamic.

If an attribute or item doesn't exist, the result is an [Undefined
value](#undefined-values).

## Filters

Pipe a value through a **filter** with `|`:

```jinja
{{ name | upper }}
{{ list | join(", ") }}
{{ html_blob | safe }}            {# pre-escaped, won't be re-escaped #}
{{ users | sort(attribute='age') | first }}
```

Filters chain left-to-right. Arguments are passed in parentheses after
the filter name. See the [filter reference](filters/index.md) for the
full list — gojinja ships every Jinja2 builtin (51 unique filters plus
aliases).

## Tests

Apply a **test** with `is`:

```jinja
{% if name is defined %}…{% endif %}
{% if n is divisibleby(3) %}fizz{% endif %}
{% if user.role == 'admin' is sameas(true) %}…{% endif %}
```

The negated form is `is not`:

```jinja
{% if x is not none %}…{% endif %}
```

See the [test reference](tests/index.md) for the full list.

## Comments

```jinja
{# A line comment. #}
{# Comments may span
   multiple lines. #}
```

Comments are stripped at parse time and never appear in output. They
also affect [whitespace](#whitespace-control) like any other tag.

## Whitespace control

By default the engine emits **all** whitespace verbatim, including
newlines around `{% %}` tags. Add `-` to a tag's opening or closing
marker to strip whitespace on that side:

```jinja
<ul>
{%- for item in items %}
  <li>{{ item }}</li>
{%- endfor %}
</ul>
```

`{%-` strips whitespace before the tag (back to the previous block).
`-%}` strips whitespace after the tag (forward to the next block).
The default behaviour is configurable per-Environment with the lexer
options.

## Escaping

`{{ ... }}` HTML-escapes its output by default
([autoescape](#autoescape)). To emit raw HTML, mark a value as `safe`:

```jinja
{{ html_blob | safe }}
```

Or wrap server-side as `Markup`:

```go
env, _ := gj.New(gj.WithGlobal("greeting",
    gj.Markup("<b>Hello!</b>"),
))
```

To emit a literal `{{` or `{%` in template output, wrap in
`{% raw %}...{% endraw %}`:

```jinja
{% raw %}
  Use {{ name }} as a placeholder.
{% endraw %}
```

## Control structures

### `if` / `elif` / `else`

```jinja
{% if user.role == 'admin' %}
  Admin
{% elif user.role == 'editor' %}
  Editor
{% else %}
  Viewer
{% endif %}
```

### `for`

```jinja
{% for item in items %}
  {{ loop.index }}. {{ item }}
{% endfor %}
```

The loop body has access to a special `loop` variable:

| `loop.*` | Meaning |
|---|---|
| `index` | 1-indexed iteration counter |
| `index0` | 0-indexed iteration counter |
| `revindex` | reverse 1-indexed counter |
| `revindex0` | reverse 0-indexed counter |
| `first` | true on the first iteration |
| `last` | true on the last iteration |
| `length` | total number of items |
| `cycle('a', 'b')` | cycles through the given values |
| `previtem` | the previous item (Undefined on the first iteration) |
| `nextitem` | the next item (Undefined on the last iteration) |
| `changed(x)` | true when `x` differs from the previous iteration |
| `depth` | nesting depth (1 for the outer loop, 2 for nested, …) |

`{% else %}` after `{% for %}` runs when the iterable was empty:

```jinja
{% for user in users %}
  {{ user }}
{% else %}
  No users.
{% endfor %}
```

### `for` with unpacking

```jinja
{% for k, v in mapping | items %}
  {{ k }}: {{ v }}
{% endfor %}
```

### `break` / `continue`

```jinja
{% for n in numbers %}
  {% if n is divisibleby(7) %}{% break %}{% endif %}
  {{ n }}
{% endfor %}
```

!!! note "Built into the parser"
    `break` and `continue` are part of the gojinja parser — you don't need
    to enable an extension. (In Python Jinja2 they require the
    `loopcontrols` extension; in gojinja they're always available.)

### `do`

`do` evaluates an expression for its side effect and discards the result:

```jinja
{% do mylist.append(item) %}
```

Like `break` / `continue`, `do` is built into the parser.

## Template inheritance

Define a base template:

```jinja
{# base.html #}
<!DOCTYPE html>
<title>{% block title %}Default{% endblock %} — Site</title>
<body>
  <main>{% block content %}{% endblock %}</main>
</body>
```

Child templates `extend` it and override the named blocks:

```jinja
{# article.html #}
{% extends "base.html" %}

{% block title %}{{ super() }} — {{ article.title }}{% endblock %}

{% block content %}
  <article>{{ article.body | safe }}</article>
{% endblock %}
```

`super()` inside a block emits the parent block's content.

### Scoped blocks

By default, blocks have their own scope and don't see the parent
template's local variables. Add `scoped` to opt in:

```jinja
{% for item in items %}
  {% block item_view scoped %}
    {{ item.name }}
  {% endblock %}
{% endfor %}
```

### Required blocks

A child must override a block marked `required`:

```jinja
{% block hero required %}{% endblock %}
```

## Includes

```jinja
{% include "header.html" %}
{% include "header.html" with context %}
{% include "header.html" without context %}
{% include "header.html" ignore missing %}
{% include ["header_admin.html", "header.html"] %}    {# tries each in order #}
```

`with context` (the default) shares the parent template's variables;
`without context` walls them off. `ignore missing` silently no-ops when
no listed template can be found.

## Imports

```jinja
{% import "macros.html" as m %}
{{ m.button("Save", primary=true) }}
```

Or import specific names:

```jinja
{% from "macros.html" import button, link %}
{{ button("Save") }}
```

## Macros

A reusable parameterised block:

```jinja
{% macro button(label, primary=false) -%}
  <button class="btn{% if primary %} btn--primary{% endif %}">
    {{ label }}
  </button>
{%- endmacro %}

{{ button("Cancel") }}
{{ button("Save", primary=true) }}
```

A macro that accepts a `caller`-provided body (the `call` block):

```jinja
{% macro section(title) %}
  <section><h2>{{ title }}</h2>
    {{ caller() }}
  </section>
{% endmacro %}

{% call section("Latest news") %}
  {% for item in news %}<p>{{ item }}</p>{% endfor %}
{% endcall %}
```

## Assignments

`{% set %}` defines a variable in the current scope:

```jinja
{% set page_title = "Welcome" %}
{% set greeting, items = "Hi", [1, 2, 3] %}
```

`{% set ... %}...{% endset %}` captures a block of rendered text into a
variable:

```jinja
{% set greeting %}
Hello, {{ name }}!
{% endset %}
```

The captured value is `Markup` (autoescape-safe).

### `namespace` for accumulator patterns

`{% set %}` is **loop-local** — assignments inside `{% for %}` don't
leak out. To accumulate across loop iterations, use `namespace()`:

```jinja
{% set ns = namespace(found=false) %}
{% for item in items %}
  {% if item.match %}{% set ns.found = true %}{% endif %}
{% endfor %}
{% if ns.found %}…{% endif %}
```

## Expressions

The expression language closely matches Python's:

- Arithmetic: `+`, `-`, `*`, `/`, `//` (floor division), `%`, `**`.
- Comparisons: `==`, `!=`, `<`, `>`, `<=`, `>=`.
- Logical: `and`, `or`, `not`.
- Ternary: `value if test else other`.
- String concatenation: `"a" + "b"` or `"a" ~ "b"` (always-stringify form).
- Slicing: `seq[start:stop]`, `seq[::2]`.
- Membership: `x in collection`, `x not in collection`.

Literals: numbers (`1`, `1.5`), strings (`"foo"` / `'foo'`), tuples
(`(1, 2)`), lists (`[1, 2]`), dicts (`{'a': 1}`), `true` / `false` /
`none`. Inside templates these are case-insensitive aliases for the
Python forms `True` / `False` / `None`.

## Autoescape

`{{ ... }}` HTML-escapes by default (gojinja's safe-by-default
inversion). Override with the autoescape block:

```jinja
{% autoescape false %}
  {{ html_blob }}        {# rendered raw #}
{% endautoescape %}
```

Or per-value with the `safe` filter:

```jinja
{{ html_blob | safe }}
```

Or globally:

```go
env, _ := gj.New(gj.WithAutoescape(gj.AutoescapeNever{}))
```

!!! warning "Differs from Python Jinja2"
    Python Jinja2 defaults autoescape **off**. gojinja defaults it
    **on** for safety. See [divergences](../divergences.md).

## Undefined values

Reading a missing variable, attribute, or item yields an `Undefined`
sentinel. Its behaviour depends on the `Environment`'s undefined mode
([`WithUndefined`](https://pkg.go.dev/github.com/jryberg/gojinja#WithUndefined)):

| Mode | Print | Boolean | Attribute access | Arithmetic |
|---|---|---|---|---|
| `Base` (default) | `""` | `false` | error | error |
| `Chainable` | `""` | `false` | same Undefined | error |
| `Debug` | `"{{ name }}"` | `false` | error | error |
| `Strict` | error | error | error | error |

Use the `defined` test for explicit checks:

```jinja
{% if user is defined %}{{ user.name }}{% endif %}
{{ user.email | default('—') }}
```

## Custom delimiters

If `{{ ... }}` clashes with another templating layer (Vue, Mustache),
override the delimiters at construction:

```go
env, _ := gj.New(gj.WithLexerOptions(
    lexer.VariableStartString("[["),
    lexer.VariableEndString("]]"),
))
```

## See also

- [Filters](filters/index.md) — full reference for every filter.
- [Tests](tests/index.md) — full reference for every test.
- [Globals](globals/index.md) — `range`, `dict`, `lipsum`, `cycler`,
  `joiner`, `namespace`.
- [Documented divergences](../divergences.md) — every intentional
  difference from Python Jinja2.
- [Migrating from Python Jinja2](../migrating-from-jinja2.md).
