# Security policy

## Reporting a vulnerability

**Please do not report security vulnerabilities through public GitHub
issues, discussions, or pull requests.**

Use GitHub's private vulnerability reporting:
<https://github.com/jryberg/gojinja/security/advisories/new>

If you cannot use that channel, email the maintainer at the address listed
on the GitHub profile of [@jryberg](https://github.com/jryberg) and prefix
the subject with `[gojinja security]`.

Please include:

- A description of the issue and its impact (information disclosure, sandbox
  escape, denial of service, …).
- A minimal reproducer template + render context, if applicable. The smaller
  the template, the faster we can verify and fix.
- The gojinja version (commit SHA or tag) you tested against.
- Whether you intend to publish a write-up; if so, a proposed disclosure
  date so we can coordinate.

## Scope

In scope:

- Sandbox escapes — any template that escalates from the default sandboxed
  environment to access disallowed attributes, callables, or globals.
- Resource exhaustion — DoS via templates that bypass the configured
  `range` / cache / parser-depth limits.
- Information leaks — error messages or render output that reveal absolute
  filesystem paths from a `FileSystemLoader` root the user did not
  explicitly expose.
- Path traversal in any loader.
- Issues in the AST gob-encoded cache format that would let a malicious
  cache file execute arbitrary callables on load.

Out of scope:

- Issues in `WithUnsafe()` mode. That mode explicitly disables sandbox
  enforcement and is documented as such.
- Issues in user-supplied callables registered via `WithGlobal` /
  `WithFilter` / `WithTest`. The user is responsible for the safety of
  callables they hand to the template.
- Vulnerabilities in `tools/parity/` — that is dev tooling, not part of the
  shipped library.

## Response timeline

We aim to acknowledge reports within 5 working days and ship a fix or
mitigation within 30 days. Critical issues (sandbox escapes that affect
default-config users) take priority.

## Disclosure

Once a fix is released, we publish a GitHub Security Advisory with a CVE if
applicable, credit you (with permission), and tag a patch release.
