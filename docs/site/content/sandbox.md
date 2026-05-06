# Sandbox

The default `Environment` is **sandboxed**. gojinja inverts Python Jinja2's
default ("unsandboxed unless you ask for `SandboxedEnvironment`") on the
principle of "secure code before beauty" — see [divergences](divergences.md)
for the full list of inverted defaults.

## What the sandbox blocks

The sandbox layer in `pkg/sandbox` provides four predicates:

- `IsSafeAttribute(obj, attr) bool` — gates attribute access. Rejects
  Python-style dunder attributes (`__class__`, `__mro__`, `__bases__`,
  `__subclasses__`, `__init__`, etc.) that classic Jinja2 sandbox-escape
  exploits use to climb the type hierarchy.
- `IsSafeCallable(fn) bool` — gates calls. Rejects bound methods that
  could mutate shared state.
- `IsMutatingMethod(name) bool` — names of methods that mutate their
  receiver (`pop`, `append`, `clear`, etc.). The sandbox refuses to call
  these even on objects the template otherwise has access to.
- `UnsafeFunc(fn)` — wrap a Go function you want to **mark as
  unsafe-to-call**, even though it's been registered as a global. Useful
  for callables you only want available in a `WithUnsafe()`-built
  environment.

## Opting out

Pass `gj.WithUnsafe()` at construction:

```go
env, _ := gj.New(gj.WithUnsafe())   // sandbox disabled
```

!!! warning
    Never enable `WithUnsafe()` for templates supplied by untrusted users.
    The sandbox is the project's primary defence against template-driven
    Go runtime introspection. The `WithUnsafe()` opt-out exists for
    server-operator-controlled template bundles only.

## See also

- [Documented divergences](divergences.md) — full list of safe-by-default
  inversions.
- [Security audit checklist](security.md) — what reviewers check on every
  change that touches the sandbox.
- The full reference on
  [pkg.go.dev/.../pkg/sandbox](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/sandbox).
