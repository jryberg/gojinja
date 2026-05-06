#!/usr/bin/env python3
"""Renders a single template with the canonical Python Jinja2.

Usage:
  parity.py <template.j2> <vars.json>             # flat case
  parity.py <main.j2> <vars.json> <loader_dir>     # multi-file case

For multi-file cases, every sibling `*.j2` in `loader_dir` (other than
`_main.j2`) becomes a template loadable by name (without the `.j2`
suffix) — matching what the Go harness does with mapLoader.
"""

import glob
import json
import os
import sys

import jinja2


def main() -> int:
    if len(sys.argv) not in (3, 4):
        print(
            "usage: parity.py <template.j2> <vars.json> [loader_dir]",
            file=sys.stderr,
        )
        return 2
    tmpl_path, vars_path = sys.argv[1], sys.argv[2]
    loader_dir = sys.argv[3] if len(sys.argv) == 4 else None
    with open(tmpl_path, "r", encoding="utf-8") as f:
        src = f.read()
    with open(vars_path, "r", encoding="utf-8") as f:
        vars_ = json.load(f)
    kwargs = dict(
        autoescape=False,
        keep_trailing_newline=True,
        # gojinja makes `do` / `break` / `continue` always available, so
        # enable Jinja2's bundled extensions for parity. Documented as a
        # deliberate divergence in the project's INVENTORY.md §18.
        extensions=["jinja2.ext.do", "jinja2.ext.loopcontrols"],
    )
    if loader_dir:
        templates = {}
        for path in glob.glob(os.path.join(loader_dir, "*.j2")):
            name = os.path.splitext(os.path.basename(path))[0]
            if name == "_main":
                continue
            with open(path, "r", encoding="utf-8") as f:
                templates[name] = f.read()
        kwargs["loader"] = jinja2.DictLoader(templates)
    env = jinja2.Environment(**kwargs)
    tmpl = env.from_string(src)
    sys.stdout.write(tmpl.render(**vars_))
    return 0


if __name__ == "__main__":
    sys.exit(main())
