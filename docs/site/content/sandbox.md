# Sandbox

The default `Environment` is **sandboxed**. gojinja inverts Python Jinja2's
default ("unsandboxed unless you ask for `SandboxedEnvironment`") on the
principle of "secure code before beauty" — see [divergences](divergences.md)
for the full list of inverted defaults.

## What the sandbox blocks

The sandbox layer in `pkg/sandbox` provides four predicates:

- `IsSafeAttribute(obj, attr) bool` — gates attribute access. Rejects
  every name starting with an underscore — Python-style dunders such as
  `__class__`, `__mro__`, `__subclasses__` and `__init__` that classic
  Jinja2 sandbox-escape exploits use to climb the type hierarchy among
  them — and every attribute of Go's `reflect` and `runtime` types.
- `IsSafeCallable(fn) bool` — gates calls. Rejects callables marked with
  `UnsafeFunc`.
- `IsMutatingMethod(name) bool` — names of methods that mutate their
  receiver (`pop`, `append`, `clear`, etc.), for your own checks. The
  default sandbox does not apply it: like Jinja2's `SandboxedEnvironment`,
  it lets templates call `{% do items.append(x) %}`.
- `UnsafeFunc(fn)` — wrap a Go function you want to **mark as
  unsafe-to-call**, even though it's been registered as a global. Useful
  for callables you only want available in a `WithUnsafe()`-built
  environment.

## What a blocked attribute does

As in Python's `SandboxedEnvironment`, an unsafe attribute that exists
(`x.__class__`, a namespace's `ns._secret`, a generator's `gi_frame`)
gives an Undefined, never the value. It prints as nothing and fails
`is defined`, and using it — calling it, reading its attributes, or
printing it under `StrictUndefined` — raises `SecurityError`
("access to attribute '__class__' of 'list' object is unsafe."). An
underscore name the object lacks is looked up as an item instead, so a
dict key such as `doc._id` works as in Jinja2.

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
