# gojinja — security audit checklist

Run this checklist before every release tag. Record the result as `docs/security-audit-vX.Y.Z.md` with file/line citations (see `docs/security-audit-v0.1.0.md` for the format). Do not tag a release with unresolved items.

## 1. Dependency hygiene
- [ ] No new third-party Go module added without explicit user approval. If approval was granted, the rationale is logged in the commit message and the audit record.
- [ ] `go.sum` matches `go.mod`; no transitive surprises.

## 2. Untrusted input boundaries
- [ ] Every public function that accepts template source has a unit test for malformed input (truncated, oversized, embedded NUL bytes, deeply nested constructs).
- [ ] Every public function that accepts a template *name* validates it through `splitTemplatePath` (or its equivalent) before doing any I/O, lookup, or string concatenation against a filesystem path.
- [ ] No function trusts a string from a `Loader` to be ASCII / UTF-8 / safe — they validate.

## 3. File system safety
- [ ] No file open outside `pkg/loader/filesystem.go` and `pkg/cache/filesystem.go`.
- [ ] `FileSystemLoader` enforces an allowlisted root.
- [ ] Symlinks that resolve outside the allowlisted root are rejected.
- [ ] No use of `os.Chdir`, `os.Setenv`, `os.Chmod` (except for cache-file safe-perms write).

## 4. Sandbox correctness
- [ ] The default `Environment` is sandboxed.
- [ ] Reflection-based access of unexported fields is blocked.
- [ ] No reflection-based call of a function whose `unsafe_callable` flag is true.
- [ ] Format-string sandbox blocks attribute access through `%(name)s` lookup that would otherwise bypass `getattr`.

## 5. Resource bounds
- [ ] `range` is bounded.
- [ ] Template cache is bounded (no `-1` "unlimited" path).
- [ ] Parser depth is bounded; deeply nested input raises a syntax error rather than overflowing the Go stack.
- [ ] No unbounded recursion in evaluator (use heap-allocated frames or explicit depth counter).

## 6. Information disclosure
- [ ] Errors do not include host filesystem paths beyond the configured loader root.
- [ ] Errors do not include the raw value of secret-looking variables (no full context dumps in production error messages).
- [ ] `DebugUndefined` only enabled when explicitly opted in.

## 7. Concurrency safety
- [ ] No package-global mutable state without a mutex.
- [ ] `Environment` is safe for concurrent `Render` calls (per `Template.Render` doc).
- [ ] Cache writes are atomic (temp+rename) and protected against partial reads.

## 8. Disallowed Go features
- [ ] No `unsafe` package import outside packages explicitly approved by the user.
- [ ] No `os/exec` import.
- [ ] No `net/*` import unless the phase is the i18n or future networked-loader phase, with explicit rationale logged.
- [ ] No CGo.

## 9. Test integrity
- [ ] Every parity test that mirrors a Python test references the source test by name in a comment.
- [ ] No test was modified to make broken code pass.
- [ ] Failing tests are recorded as known-defects in the audit record, not silenced.

## 10. Tooling
- [ ] `go vet ./...` passes.
- [ ] `staticcheck ./...` passes (when installed).
- [ ] `gosec ./...` passes (when installed; its findings are reviewed and either fixed or annotated with rationale).

## How to record an audit pass

Create `docs/security-audit-vX.Y.Z.md` for the release. Walk each of the 10 sections, marking each item ✅ with a citation to the file/line that satisfies it (or ⚠️ with a follow-up note if deferred). The summary at the top should name the release tag, the date, and any deferred items. See `docs/security-audit-v0.1.0.md` as the canonical example.
