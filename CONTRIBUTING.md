# Contributing to gojinja

Thanks for thinking about contributing! gojinja exists to give Go programs the
exact same template-rendering behaviour as Python's Jinja2, byte-for-byte.
**Every template valid in canonical Python Jinja2 must render identically in
gojinja** — when a parity test fails, the right answer is to fix gojinja, not
the test. The deliberate (and only) exceptions are listed in
[`docs/divergences.md`](docs/divergences.md); please skim it before opening a
PR that changes rendering behaviour.

For local setup, build, test, and parity instructions, see
[`DEVELOPMENT.md`](DEVELOPMENT.md). This document covers only the contribution
process itself.

## Reporting a security issue

**Do not open a public issue for security bugs.** Use GitHub's private
vulnerability reporting at
<https://github.com/jryberg/gojinja/security/advisories/new>. See
[`SECURITY.md`](SECURITY.md) for what to include.

## Branching

- `main` is the only long-lived branch. It is always green (CI passes, parity
  passes).
- Open a PR from a feature branch in your fork. Name it whatever you want —
  it's your fork.
- Merge style on `main` is **squash-merge**. The PR title becomes the commit
  on `main`, so the title must conform to the commit format below.

## Commit message format — Conventional Commits 1.0.0

The PR title (and the resulting squashed commit on `main`) MUST follow
[Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/):

    <type>(<optional scope>): <description>

    [optional body]

    [optional BREAKING CHANGE: <reason> footer]

`<type>` is one of:

| Type       | Meaning                                  | Triggers release? |
|------------|------------------------------------------|-------------------|
| `feat`     | new user-visible feature                 | minor bump        |
| `fix`      | bug fix                                  | patch bump        |
| `perf`     | performance improvement                  | patch bump        |
| `refactor` | internal restructure, no behaviour delta | no release        |
| `docs`     | docs-only change                         | no release        |
| `test`     | tests only                               | no release        |
| `build`    | build / Makefile / module changes        | no release        |
| `ci`       | CI configuration                         | no release        |
| `chore`    | misc maintenance                         | no release        |
| `revert`   | revert a previous commit                 | depends           |

Add `!` after the type (or include `BREAKING CHANGE:` in the footer) to mark
a breaking change — that triggers a major bump.

`<optional scope>` is the package name, e.g. `feat(parser): support
loop.cycle`. Don't invent scopes — use directory names under `pkg/` or the
literal `cli` / `parity` / `docs` / `ci`.

### Examples

    feat(filters): add wordwrap break_on_hyphens kwarg
    fix(lexer): handle CRLF inside raw blocks
    docs: clarify autoescape default in README
    chore(deps): bump actions/setup-go from v5.0.0 to v5.1.0
    feat(env)!: drop WithLegacyUndefined option

## Sign-off

Every commit must carry a `Signed-off-by` trailer. This is a Developer
Certificate of Origin attestation (<https://developercertificate.org>) — by
signing off, you state you have the right to contribute the change under the
project's BSD 3-Clause license.

The [Local hooks](#local-hooks) section below explains the easiest way to add
this automatically. Without hooks, use `git commit -s` manually. Commits
without sign-off are rejected by CI.

## Local hooks

The repo ships hooks under `.githooks/` that mirror the CI gates. Install
once per clone:

    make hooks-install

That sets `core.hooksPath=.githooks` for this checkout and wires up:

- `prepare-commit-msg` — auto-appends `Signed-off-by:` using
  `git config user.email`.
- `commit-msg` — rejects commits without a sign-off (same regex as the CI
  DCO check).
- `pre-commit` — fast lint subset: `gofmt -l` on staged files, `go vet`,
  `make audit`, `staticcheck`.
- `pre-push` — full `make ci` (build + vet + test + audit + parity) plus
  `staticcheck` and `govulncheck`.

Bypass with `git commit --no-verify` / `git push --no-verify` only when
genuinely necessary. Disable the hooks entirely with `make hooks-uninstall`.

### Tool prerequisites

The hooks skip-with-warning anything that isn't installed (CI catches the
gap), but to run the same suite CI runs you'll want all of these on your
`PATH`. After `go install`, make sure `$(go env GOPATH)/bin` is on your
`PATH`.

| Tool | Install | Used by |
|---|---|---|
| Go 1.22+ (`go`, `gofmt`, `go vet`, `go test`) | <https://go.dev/dl/> | every hook |
| `staticcheck` | `go install honnef.co/go/tools/cmd/staticcheck@2025.1.1` | pre-commit, pre-push |
| `govulncheck` | `go install golang.org/x/vuln/cmd/govulncheck@v1.1.4` | pre-push |
| Python 3 + Jinja2 (for the parity harness) | `python3 -m pip install jinja2` | pre-push (`make parity`) |

The `staticcheck` and `govulncheck` versions match the pins in
`.github/workflows/ci.yml` — bump them here when CI bumps.

## Documenting a new filter, test, or global

The published [docs site](https://jryberg.github.io/gojinja/) regenerates
its filter / test / global reference pages from this repo on every push.
Each registered name needs **either** a structured godoc comment on its
underlying Go function **or** a hand-authored Markdown override page. CI
fails (`make docs-check`) if a name has neither.

### Godoc style (the preferred path)

Backfill a structured godoc on the underlying private func. The format
docgen looks for:

```go
// filterX implements the `name` filter: <one-line summary>.
//
// Signature: name(positional, kw=default)
//
// <Optional Behavior: paragraph for non-obvious cases>
//
// Example:
//
//	{{ value | name }}  →  result
//
// <Optional Divergence from Python: ... pointing at docs/divergences.md>
```

Same shape for tests (`testX`) and globals (`globalX`). Keep examples
short — prefer 2-3 lines over a long block. The first sentence becomes
the page summary; everything else becomes the page body.

### Override pages (for popular filters that deserve richer prose)

For filters / tests / globals that warrant side-by-side Python comparison,
common-gotcha admonitions, or multiple worked examples, drop a Markdown
file at:

    docs/site/content/templates/{filters,tests,globals}/<slug>.md

`<slug>` is the canonical name (e.g. `default.md`, `tojson.md`). The
override completely replaces the godoc-derived page. See
`docs/site/content/templates/filters/default.md` for the convention —
front matter isn't needed; just `# `name`` plus body.

### Local preview

```bash
make docs-check    # fails fast if a registered name has no docs
make docs-serve    # mkdocs serve on http://127.0.0.1:8000
```

## Pull request checklist

Before requesting review:

- [ ] PR title follows Conventional Commits (the CI will reject it otherwise).
- [ ] Each commit has `Signed-off-by`.
- [ ] `make build`, `make vet`, `make test`, `make audit`, `make parity` all
      pass locally.
- [ ] If your change is template-visible, you added a corpus case under
      `tools/parity/corpus/` that locks in the new behaviour.
- [ ] If your change touches IO, attribute access, callable dispatch, or the
      sandbox, you walked the relevant section of
      `docs/security-audit-checklist.md` and recorded findings in your PR
      description.
- [ ] You did not edit `CHANGELOG.md` by hand. Release-please regenerates it
      on each release; manual edits will be overwritten.

## Reviews & merging

PRs need one maintainer approval. CI must be green. Maintainers squash-merge
with the PR title as the commit subject; the PR description is preserved in
the commit body if you want context to appear in `git log`.

## License

By contributing, you agree your contribution is licensed under the project's
[BSD 3-Clause license](LICENSE).
