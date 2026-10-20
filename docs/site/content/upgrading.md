# Upgrading to v1.1

v1.1 brings many templates closer to Python Jinja2: Python's numbers, str and
bytes methods, codecs, lazy generator filters, and Python's error messages.
Templates that render identically in Python need no changes. This page lists
what changes for **Go code** that uses gojinja.

Most programs only call `gojinja.New`, `FromString`/`GetTemplate` and
`Render`, and are unaffected by the first section. Check the second section
if your Go code inspects the values templates produce, registers filters, or
matches on error types.

## Code that no longer compiles

All of these are in the advanced sub-packages, which the `gojinja` package
does not re-export.

### `runtime.LoopContext`

`loop` now pulls items from its iterable lazily, as Python's `LoopContext`
does, so the methods that may read ahead return an error:

| v1.0 | v1.1 |
|---|---|
| `Iterate() (any, bool)` | `Iterate() (any, bool, error)` |
| `Length() int`, `Revindex() int`, `Revindex0() int` | each returns `(int, error)` |
| `Last() bool` | `Last() (bool, error)` |
| `NextItem() any` | `NextItem() (any, error)` |
| `Cycle(args ...any) any` | `Cycle(args ...any) (any, error)` |
| `Changed(value any) bool` | `Changed(values ...any) bool` |
| `Call(items []any) (string, error)` | `Call(iterable any) (any, error)` |
| `Get(attr string) (any, bool)` | `GetAttr(attr string) (any, bool, error)` |

`NewLoopContext` takes the items as an iterator, the iterable's length (or
`nil` when it has none), and the function that makes `previtem`'s and
`nextitem`'s Undefined:

```go
items := []any{"a", "b", "c"}
lc := runtime.NewLoopContext(
	eval.SliceNext(items),
	func() int { return len(items) },
	0,   // depth
	nil, // not a recursive loop
	func(hint string) any { return runtime.NewBase(hint, "", runtime.Missing, nil) },
)
for {
	v, ok, err := lc.Iterate()
	if err != nil || !ok {
		break
	}
	n, _ := lc.Length()
	fmt.Println(v, lc.Index(), "of", n)
}
```

### `runtime.OrderedDict.AsMap`

Removed. Read the dict in its order instead:

```go
for _, k := range d.Keys() {
	v, _ := d.Get(k)
	fmt.Println(k, v)
}
```

### `runtime.CallerFunc`

Removed. `caller` is now a macro, like the macro it is passed to, and templates
call it as before; nothing outside the evaluator received the old function.

### `ext.EnvLister`

Gained `Pformat(v any, depth int, compact bool) string`, which
`{% debug %}` uses to print Python's `pprint` layout. `*environment.Environment`
implements it; add the method to your own implementation, if you have one.

## Behavior changes for Go callers

These compile unchanged but return or accept different values.

- **Generator filters.** `map`, `select`, `reject`, `selectattr`, `rejectattr`,
  `unique`, `batch`, `slice` and `items` return a `*runtime.Iterator`, and
  `reverse` returns a reverse iterator, as in Python. Code calling them
  through `Environment.CallFilter` gets the iterator; read it with
  `eval.ToIterable`. In templates, write `xs|select('odd')|list` for a list.
- **New value types.** Values a template hands to Go — filter arguments,
  `pkg/native` results, `JSONVars` output — can now be `*big.Int` (ints past
  int64), `complex128`, `*runtime.Range` (from `range()`), `runtime.Bytes` (from
  `str.encode`), `*runtime.Iterator`, and `*runtime.Callable` (methods and the
  `range`, `lipsum`, `cycler`, `joiner` and `namespace` globals).
- **Builtin filters get lists themselves.** A builtin filter now receives an
  in-template list as a `*runtime.PyList`, so one returning its input returns
  the same list. Filters you register still receive `[]any`.
- **Error types.**
  - An unknown filter is a `TemplateRuntimeError` ("No filter named 'x'."),
    where it was a `FilterArgumentError`.
  - `GetTemplate` with no loader returns a `TemplateRuntimeError` ("no loader
    for this environment specified"), where it was a `TemplateNotFound`.
    A missing template is still a `TemplateNotFound`.
  - Many messages now read as Python's do.
- **Sandbox.** An unsafe attribute such as `{{ x.__class__ }}` or `{{ x._y }}` no
  longer fails the render. As in Python's `SandboxedEnvironment` it becomes an
  Undefined that renders empty and raises `SecurityError` once used. The value
  is never exposed.
- **Type names.** `runtime.PyTypeName` reports Go functions as `function`, and
  names the new types (`generator`, `range`, `bytes`, …).

See [Documented divergences](divergences.md) for the few places gojinja
deliberately differs from Python Jinja2, two of them new in v1.1: the bound on
integer size, and correctly rounded float math.
