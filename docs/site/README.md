# gojinja documentation site

Source for <https://jryberg.github.io/gojinja/>.

## Build locally

```bash
python -m venv .venv && source .venv/bin/activate
pip install -r docs/site/requirements.txt

make docs-check     # docgen check (registry coverage + godoc lint)
make docs           # generate filter/test/global pages + mkdocs build
make docs-serve     # mkdocs serve at http://127.0.0.1:8000
```

## Layout

- `mkdocs.yml` — site config (theme, plugins, nav).
- `requirements.txt` — Python deps for the site build.
- `overrides/` — Material theme overrides (only the bits we customise).
- `content/` — Markdown source.
  - `templates/{filters,tests,globals}/<name>.md` — hand-authored override
    pages (Jinja2-style: signature + summary + behavior + examples).
  - `templates/{filters,tests,globals}/_generated/<name>.md` — written by
    `tools/docgen` from the godoc on the underlying Go func when no override
    exists. Gitignored — regenerated each build.
  - Files like `architecture.md`, `divergences.md`, `security.md`, and
    `changelog.md` are thin wrappers that transclude the canonical sources
    in `docs/` (and `CHANGELOG.md`) via the `include-markdown` plugin.

## Versioning

Deploys go to the `gh-pages` branch via [mike](https://github.com/jimporter/mike).
Each release tag publishes one frozen folder (`/v1.4.2/`) and aliases:

- `stable` → latest non-prerelease tag
- `dev` → latest `main` build

The version selector in the header lets visitors switch between any
published version.

## Adding documentation for a new filter / test / global

1. Implement the feature and register it in `pkg/environment/builtins.go`
   (or `filters_extra.go`).
2. Either:
   - Write a structured godoc on the underlying private func (the project
     standard — used as the fallback content), or
   - Author a hand-written override at
     `docs/site/content/templates/filters/<name>.md` (preferred for the most
     popular filters; lets you write rich examples and Python comparisons).
3. Run `make docs-check`. CI will fail if any registered name has neither.
