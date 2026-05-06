# External parity harness

A second parity runner that complements `tools/parity/` by stressing
gojinja against real-world Jinja2 templates from ten upstream open-source
projects. See `SOURCES.md` next to this file for the project list,
license, pinned commit, and rationale.

## Quick start

```bash
# 1. Fetch the templates (writes to ./cache/, gitignored).
go run ./tools/parity/external -fetch

# 2. Run parse-parity (default).
go run ./tools/parity/external

# 3. Run render-parity (more demanding).
go run ./tools/parity/external -render

# Restrict to one project.
go run ./tools/parity/external -only sphinx
```

## What the modes do

**Parse-parity (default).** Lex + parse each template through
`gj.Environment.FromString` and through Python's `jinja2.Environment.parse`.
Compare outcomes:

| gojinja | python | classification |
|---------|--------|----------------|
| OK      | OK     | both-ok        |
| err     | err    | both-fail (parity) |
| OK      | err    | go-only (gojinja accepted what Python rejected — bug) |
| err     | OK     | py-only (Python accepted what gojinja rejected — bug) |

A non-zero exit means at least one go-only or py-only divergence was
seen. `both-fail` is parity, not failure: real templates routinely
reference custom filters / unknown tags, and as long as both engines
fall over identically the engine surface is in agreement.

**Render-parity (`-render`).** Same comparison, but extends `parse` to
end-to-end render with empty context (`{}`) and a `FileSystemLoader`
rooted at each project's `LoaderRoot`. When both engines render
successfully, the outputs are diffed byte-for-byte.

## Authentication

Public repos work without auth, but GitHub's tarball endpoint applies
strict rate limits to anonymous traffic. The fetcher honors `GH_TOKEN`
or `GITHUB_TOKEN` from the environment if present.

## Layout per project

```
cache/<project>/
├── __LICENSE__/          # vendored upstream LICENSE / NOTICE
├── __MANIFEST__.json     # {project, repo, commit, file count, …}
└── <upstream relative path>/<template files>
```

Templates land under their original upstream path so `extends "../base.html"`
and friends still resolve when run through the FileSystem loader.

## When this finds a real divergence

Don't simplify the template. Don't narrow the manifest. The hard rule
applies: **fix gojinja, not the case.**
