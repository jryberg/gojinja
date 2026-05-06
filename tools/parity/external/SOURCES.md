# External parity corpus — sources

The `tools/parity/external/` harness stresses gojinja against real-world Jinja2
templates from ten widely-used open-source projects. The fetched files
themselves are **not** committed to this repository — `tools/parity/external/cache/`
is `.gitignore`d. This file is the manifest: it pins each upstream commit,
records the source license, and lists the subdirectories we sample from.

## Selection criteria

1. **Permissive license only.** BSD-2/3-Clause, MIT, or Apache-2.0. (We
   considered Pelican but its AGPL-3.0 license makes vendoring incompatible
   with this project's MIT license.)
2. **Real template usage.** Each project ships at least one directory of
   `*.j2`/`*.html`/`*.sql` files that pass through Jinja2 in production.
3. **Self-contained where possible.** We prefer paths whose templates render
   with stock Jinja2 features (or fail in the same way under both engines).
   Templates that hard-require project-specific filters/globals are still
   useful: the harness diffs *parse outcomes*, not just *render outputs*, so
   a template that fails to parse should fail in both engines for the same
   reason.

## The ten projects

Each row's commit is the upstream `HEAD` at the time this manifest was
written (2026-05-06).

| # | Project | License | Pinned commit | Sampled paths |
|---|---------|---------|---------------|---------------|
| 1 | [pallets/jinja](https://github.com/pallets/jinja) — Jinja2 itself, used as a self-test | BSD-3-Clause | `5ef70112a1ff19c05324ff889dd30405b1002044` | `tests/res/templates/` |
| 2 | [pallets/flask](https://github.com/pallets/flask) — reference web framework | BSD-3-Clause | `7374c85ddefc3f4b177a698ab9f0cbb6a5c0b392` | `examples/tutorial/flaskr/templates/` |
| 3 | [sphinx-doc/sphinx](https://github.com/sphinx-doc/sphinx) — documentation generator | BSD-2-Clause | `cc7c6f435ad37bb12264f8118c8461b230e6830c` | `sphinx/themes/**/*.html` |
| 4 | [mkdocs/mkdocs](https://github.com/mkdocs/mkdocs) — static site generator | BSD-2-Clause | `2862536793b3c67d9d83c33e0dd6d50a791928f8` | `mkdocs/themes/mkdocs/`, `mkdocs/themes/readthedocs/` |
| 5 | [squidfunk/mkdocs-material](https://github.com/squidfunk/mkdocs-material) — Material theme for MkDocs | MIT | `8d01326cd2e8d8030d39e6d69790bd01fa3b7e46` | `material/templates/**/*.html` |
| 6 | [jupyter/nbconvert](https://github.com/jupyter/nbconvert) — Jupyter notebook conversion | BSD-3-Clause | `78ed30837a607deab7cf0a12dca072bf3f63417a` | `share/templates/**/*.j2` |
| 7 | [cookiecutter/cookiecutter](https://github.com/cookiecutter/cookiecutter) — project scaffolding | BSD-3-Clause | `c88fbe921c97c58b65f1883ba90a0ab53cc91b34` | `tests/fake-repo-*/` (renamed dirs containing `{{cookiecutter.*}}` placeholders) |
| 8 | [apache/airflow](https://github.com/apache/airflow) — workflow orchestration | Apache-2.0 | `4d8eae3f28d981bc7afca0143e3d8d24e71d89b7` | `**/templates/**/*.html` (FAB and pagefind subsets) |
| 9 | [dbt-labs/dbt-adapters](https://github.com/dbt-labs/dbt-adapters) — Jinja2 SQL macros that ship inside dbt-core | Apache-2.0 | `0f260a278fa9a3e6e2750d6e608ebcb2bcb0cde6` | `dbt-adapters/src/dbt/include/global_project/macros/**/*.sql` |
| 10 | [netbox-community/netbox](https://github.com/netbox-community/netbox) — network source-of-truth | Apache-2.0 | `30f9d3ed604e2a227a0835ce200bd5958707b1c6` | `netbox/templates/**/*.html` (capped at the first 100 by path) |

> dbt's macros moved out of `dbt-core` into the `dbt-adapters` repo during
> the multi-repo split. We sample the new home so the macros remain current.

## Licensing & redistribution

The harness fetches these files into a *local cache* (`cache/<project>/`)
that is excluded from this repository via `.gitignore`. Nothing under
`cache/` is checked into gojinja, redistributed, or relicensed.

Each project's full `LICENSE` (and `NOTICE` where applicable) is downloaded
into `cache/<project>/__LICENSE__` next to its templates so an auditor can
trace any file back to the originating license.

If you want to vendor a *subset* of these templates inside another
project, you must:

1. Read the upstream license of that specific project.
2. Preserve copyright headers and `LICENSE`/`NOTICE` content as required.
3. Note the upstream commit SHA (this file).

## Fetching

Run from the repo root:

```
go run ./tools/parity/external -fetch
```

This will:

1. Read this manifest (well, the equivalent Go-side `manifest.go`).
2. For each project, shallow-clone at the pinned commit into
   `tools/parity/external/cache/<project>/`.
3. Drop unrelated paths so the cache stays small.
4. Record each project's `LICENSE` next to the templates.

Running without `-fetch` skips the network and validates whatever is
already in the cache.

## Validating

```
go run ./tools/parity/external          # parse-parity (default)
go run ./tools/parity/external -render  # additionally attempt render-parity
```

The parse-parity mode lexes and parses each template through both
gojinja and Python Jinja2 and compares the *outcome*: success vs. error,
and (when both succeed) AST node-count + leaf-token shape. The render-parity
mode renders templates that don't reference loaders/extensions/custom
context and diffs the bytes.

See `README.md` next to this file for details.
