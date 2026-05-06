#!/usr/bin/env python3
"""Batch parse / render driver for the external parity harness.

Reads a JSON job description from stdin:

    {
      "mode": "parse" | "render" | "render-loose",
      "items": [
        {"id": "...", "path": "/abs/template", "loader_root": "/abs/dir"},
        ...
      ]
    }

`loader_root` may be empty for parse-mode. For render-mode it's the
directory used as the FileSystem loader so {% extends %} / {% include %}
resolve. `render-loose` is the same as render but registers passthrough
stubs for any filter/test name not in stdlib Jinja2, switches to
ChainableUndefined, and stubs the loader so {% extends %}/{% include %}
of unknown names is silent — matching what the Go side does so the
comparison stays apples-to-apples.

Writes a JSON array of results to stdout:

    [
      {"id": "...", "ok": true, "out": "<rendered>"},
      {"id": "...", "ok": false, "err": "TemplateSyntaxError: ..."}
    ]

The driver is tolerant: it never aborts on a single failing item — every
input produces exactly one output entry. The Go side does the parity
comparison; this script just reports per-item facts.
"""

import json
import sys
import traceback

import jinja2


def parity_env(loader_root: str | None, *, loose: bool = False) -> jinja2.Environment:
    """Build a Jinja2 environment matching the gojinja-side defaults.

    autoescape=False, keep_trailing_newline=True, do/loopcontrols on.
    `loader_root` configures a FileSystemLoader so extends/include can
    resolve sibling templates. `loose=True` switches the env to
    ChainableUndefined and installs a fallback loader so missing
    sibling templates render as empty rather than aborting.
    """
    kwargs = dict(
        autoescape=False,
        keep_trailing_newline=True,
        extensions=["jinja2.ext.do", "jinja2.ext.loopcontrols"],
    )
    if loose:
        kwargs["undefined"] = jinja2.ChainableUndefined
    if loader_root:
        if loose:
            kwargs["loader"] = jinja2.ChoiceLoader([
                jinja2.FileSystemLoader(loader_root),
                _StubLoader(),
            ])
        else:
            kwargs["loader"] = jinja2.FileSystemLoader(loader_root)
    elif loose:
        kwargs["loader"] = _StubLoader()
    return jinja2.Environment(**kwargs)


class _StubLoader(jinja2.BaseLoader):
    """Fallback loader for loose mode: any name resolves to an empty
    template so {% extends "missing" %} / {% include "missing" %} don't
    abort the render. Matches the gojinja-side stub.
    """

    def get_source(self, environment, template):
        return "", template, lambda: True


# Filter names referenced by templates we sample from upstream projects
# but that don't ship in stdlib Jinja2. Loose mode binds these (and any
# other name discovered by the AST scan) to a passthrough so the
# template renders. Both engines see the same stub, so any divergence
# the harness then reports is real.
_PASSTHROUGH_FILTER = lambda value, *args, **kwargs: value
_PASSTHROUGH_TEST = lambda value, *args, **kwargs: True


def _scan_filter_test_names(env: jinja2.Environment, src: str) -> tuple[set[str], set[str]]:
    """Return (filter names, test names) referenced in src."""
    filters: set[str] = set()
    tests: set[str] = set()
    try:
        ast = env.parse(src)
    except Exception:
        return filters, tests
    for node in ast.find_all(jinja2.nodes.Filter):
        filters.add(node.name)
    for node in ast.find_all(jinja2.nodes.Test):
        tests.add(node.name)
    return filters, tests


def _install_loose_stubs(env: jinja2.Environment, src: str) -> None:
    filters, tests = _scan_filter_test_names(env, src)
    for name in filters:
        if name not in env.filters:
            env.filters[name] = _PASSTHROUGH_FILTER
    for name in tests:
        if name not in env.tests:
            env.tests[name] = _PASSTHROUGH_TEST


def run_parse(items: list[dict]) -> list[dict]:
    out = []
    for it in items:
        rec = {"id": it["id"], "ok": False}
        try:
            with open(it["path"], "r", encoding="utf-8", errors="replace") as f:
                src = f.read()
            env = parity_env(None)
            env.parse(src)
            rec["ok"] = True
        except Exception as e:
            rec["err"] = f"{type(e).__name__}: {e}"
        out.append(rec)
    return out


def run_render(items: list[dict], *, loose: bool = False) -> list[dict]:
    out = []
    for it in items:
        rec = {"id": it["id"], "ok": False}
        try:
            with open(it["path"], "r", encoding="utf-8", errors="replace") as f:
                src = f.read()
            env = parity_env(it.get("loader_root") or None, loose=loose)
            if loose:
                _install_loose_stubs(env, src)
            tmpl = env.from_string(src)
            rendered = tmpl.render({})
            rec["ok"] = True
            rec["out"] = rendered
        except Exception as e:
            rec["err"] = f"{type(e).__name__}: {e}"
        out.append(rec)
    return out


def main() -> int:
    job = json.load(sys.stdin)
    mode = job.get("mode")
    items = job.get("items", [])
    if mode == "parse":
        results = run_parse(items)
    elif mode == "render":
        results = run_render(items)
    elif mode == "render-loose":
        results = run_render(items, loose=True)
    else:
        print(f"unknown mode: {mode!r}", file=sys.stderr)
        return 2
    json.dump(results, sys.stdout)
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except Exception:
        traceback.print_exc()
        sys.exit(2)
