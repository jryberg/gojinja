# Frequently asked questions

## Does gojinja support every Python Jinja2 template?

That's the project's hard rule. Every template that renders in canonical
Python Jinja2 must render byte-for-byte identically in gojinja. A parity
harness diffs gojinja's output against a Python reference on every commit
to verify this. If you find a divergence not on the [documented
divergences](divergences.md) page, please [file an
issue](https://github.com/jryberg/gojinja/issues).

## Can I share `.j2` files between Python and Go?

Yes. That's the point.

## Why is the sandbox on by default? Python Jinja2 has it off.

Because gojinja is a Go library, it's frequently used to render templates
on the server side from data that originated with end users (config
files, multi-tenant SaaS templates, etc.). Defaulting to "secure" is
strictly the safer call — see [divergences](divergences.md) for the full
list of safe-by-default inversions.

If you are sure your templates come only from trusted sources, opt out
with `gj.WithUnsafe()`.

## Why is autoescape on by default?

Same reason. The default workload for a server-side template engine is
HTML, and the cost of forgetting `|escape` is much higher than the cost
of remembering `|safe`.

## Does it support async?

Not in the Python `enable_async` / `async def` sense. Instead, every
render path accepts a `context.Context`, so cancellation, timeouts, and
deadlines compose with the rest of your Go code. See [migrating from
Jinja2](migrating-from-jinja2.md).

## Where's the API reference?

The narrative tour is at [API overview](api/index.md). The authoritative
per-symbol reference is on
[pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja), with one
permalinked version per release tag.

## How do I report a security issue?

See [SECURITY.md](https://github.com/jryberg/gojinja/blob/main/SECURITY.md)
in the repo. **Please don't file public GitHub issues for security
vulnerabilities.**

## How do I contribute a new filter?

1. Implement the filter func in `pkg/environment/builtins.go` (or
   `filters_extra.go` for the longer ones) and register it in the
   builtin map.
2. Write a structured godoc on the underlying `filterX` private func
   (summary, signature, behavior, example, divergences from Python).
3. Optionally, hand-author a richer `docs/site/content/templates/filters/<name>.md`
   override for popular filters.
4. Add a parity case under `tools/parity/corpus/` so the rendered output
   is checked against Python Jinja2 on every commit.
5. Run `make ci` and `make docs-check` — both must pass.
