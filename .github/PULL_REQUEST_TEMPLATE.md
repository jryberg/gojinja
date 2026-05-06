## What this changes

<!-- One or two sentences. The PR title is your conventional-commit subject;
this section is for the prose readers want when scanning git log. -->

## Why

<!-- Link the issue, the parity case, the upstream Jinja2 bug, or the
security finding. If this is a refactor, why now? -->

## How

<!-- Bullet list of the structural decisions a reviewer would otherwise have
to reverse-engineer from the diff. Skip the obvious. -->

## Verification

- [ ] `make build`
- [ ] `make vet`
- [ ] `make test`
- [ ] `make audit`
- [ ] `make parity`

If template-visible:
- [ ] Added a parity corpus case under `tools/parity/corpus/`.

If security-relevant (touches IO, attribute access, callable dispatch, or
the sandbox):
- [ ] Walked the relevant section(s) of `docs/security-audit-checklist.md`.
      Findings:

      <!-- summarise here -->

## Breaking changes

<!-- Mark "None" or describe. If breaking, the PR title must end in `!` so
release-please picks it up as a major bump. -->
