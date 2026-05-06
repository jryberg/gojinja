# `default`

*Aliases: `d`*

Substitute a fallback when the primary value is undefined (or, optionally,
falsy). The most common idiom for safely rendering optional template
variables.

**Signature:** `default(value, default_value="", boolean=False)`

## Behavior

- When `value` is `Undefined`, returns `default_value`.
- When `value` is defined but falsy (empty string, `0`, `None`, empty
  list, empty map, `False`) **and** `boolean=True`, returns `default_value`.
- Otherwise, returns `value` unchanged.

## Examples

=== "gojinja"

    ```jinja
    {{ unset_var | default('fallback') }}             {# → fallback #}
    {{ name      | default('Anonymous') }}            {# → name (if set) #}
    {{ ""        | default('Anonymous', true) }}      {# → Anonymous #}
    {{ 0         | default('—', true) }}              {# → — #}
    ```

=== "Python Jinja2"

    ```python
    >>> tpl = env.from_string("{{ x | default('fallback') }}")
    >>> tpl.render()
    'fallback'
    >>> tpl.render(x=None)        # None is defined; not Undefined
    'None'
    >>> tpl = env.from_string("{{ '' | default('fallback', True) }}")
    >>> tpl.render()
    'fallback'
    ```

## Common gotchas

!!! warning "`None` is not undefined"
    `None` (Go: `nil`) is a *defined* value. `{{ x | default('fallback') }}`
    where `x = None` returns `"None"`, not `"fallback"`. Pass
    `boolean=True` to also catch `None`/falsy values:
    `{{ x | default('fallback', true) }}`.

!!! tip "The `d` alias"
    `default` and `d` are interchangeable. `d` is shorter and very
    common in templates that thread a lot of optional values:

    ```jinja
    {{ user.email | d('—') }}
    ```

## See also

- [`defined` test](../tests/_generated/defined.md) for explicit
  `is defined` / `is undefined` checks.
- [Sandbox guide](../../sandbox.md) — undefined-mode selection.
- Reference on
  [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/environment).
