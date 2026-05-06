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

Every commit must carry a `Signed-off-by` trailer. Use `git commit -s` to add
it automatically. This is a Developer Certificate of Origin attestation
(<https://developercertificate.org>) — by signing off, you state you have the
right to contribute the change under the project's BSD 3-Clause license.

Commits without sign-off will be rejected by CI.

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
