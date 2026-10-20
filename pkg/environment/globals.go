package environment

import (
	"fmt"

	gjerrors "github.com/jryberg/gojinja/pkg/errors"
	"github.com/jryberg/gojinja/pkg/escape"
	"github.com/jryberg/gojinja/pkg/runtime"
)

// globalRange implements the `range` global: bounded Python-style
// range(start, stop, step).
//
// Signature: range(stop) | range(start, stop) | range(start, stop, step)
//
// Returns a [runtime.Range], Python's range object: it prints as
// range(0, 5), slices into another range and has start, stop, step,
// index() and count(). Like Jinja2's sandboxed safe_range, a range
// longer than [Environment.maxRange] (default 100000) is an error; the
// bound always applies here and is configurable — see [WithRangeLimit].
//
// Example:
//
//	{% for i in range(5) %}{{ i }} {% endfor %}      →  0 1 2 3 4
//	{% for i in range(2, 8, 2) %}{{ i }} {% endfor %}  →  2 4 6
func globalRange(e *Environment) KwargsCallable {
	return func(args []any, kwargs map[string]any) (any, error) {
		if len(kwargs) > 0 {
			for k := range kwargs {
				return nil, gjerrors.NewTemplateRuntimeError(fmt.Sprintf("safe_range() got an unexpected keyword argument %s", escape.Repr(k)))
			}
		}
		if err := argCount("range", args, 1, 3); err != nil {
			return nil, err
		}
		ints := make([]int, len(args))
		for i, a := range args {
			n, err := indexArg(a)
			if err != nil {
				return nil, err
			}
			ints[i] = n
		}
		r := &runtime.Range{Step: 1}
		switch len(ints) {
		case 1:
			r.Stop = ints[0]
		case 2:
			r.Start, r.Stop = ints[0], ints[1]
		case 3:
			r.Start, r.Stop, r.Step = ints[0], ints[1], ints[2]
		}
		if r.Step == 0 {
			return nil, gjerrors.NewTemplateRuntimeError("range() arg 3 must not be zero")
		}
		if r.Len() > e.maxRange {
			return nil, gjerrors.NewTemplateRuntimeError(fmt.Sprintf("Range too big. The sandbox blocks ranges larger than MAX_RANGE (%d).", e.maxRange))
		}
		return r, nil
	}
}

// globalDict implements the `dict` global: build a dict from positional
// pairs, keyword arguments, or another mapping.
//
// Signature: dict(**kwargs) | dict(mapping, **kwargs) | dict(pairs, **kwargs)
//
// Python's dict():
//   - `dict(a=1, b=2)`: keyword arguments become entries, in call order.
//   - `dict(mapping)`: shallow-copies an existing mapping.
//   - `dict(pairs)`: builds from a sequence of (key, value) pairs.
//
// Keyword arguments are added after the positional argument's entries.
// Returns a [runtime.OrderedDict]. `dict.fromkeys(keys, value=None)`
// is available too.
//
// Example:
//
//	{{ dict(a=1, b=2) | items | list }}  →  [('a', 1), ('b', 2)]
func globalDict(args []any, kwargs *runtime.OrderedDict) (any, error) {
	if len(args) > 1 {
		return nil, gjerrors.NewTemplateRuntimeError(fmt.Sprintf("dict expected at most 1 argument, got %d", len(args)))
	}
	out := runtime.NewOrderedDict()
	if len(args) == 1 {
		if err := updateDict(out, args[0]); err != nil {
			return nil, err
		}
	}
	for _, k := range kwargs.Keys() {
		v, _ := kwargs.Get(k)
		out.Set(k, v)
	}
	return out, nil
}

// globalCycler implements the `cycler` global: round-robin through a
// sequence of values.
//
// Signature: cycler(*items)
//
// Returns a [runtime.Cycler]. Use `.next()` to advance and read, `.current`
// to peek, `.reset()` to start over. Common use: zebra-striping rows.
//
// Example:
//
//	{% set row_class = cycler('odd', 'even') %}
//	{% for r in rows %}<tr class="{{ row_class.next() }}">…</tr>{% endfor %}
func globalCycler(items ...any) *runtime.Cycler {
	return runtime.NewCycler(items...)
}

// globalJoiner implements the `joiner` global: emit a separator on every
// call except the first.
//
// Signature: joiner(sep=", ")
//
// Useful inside loops when you need a separator between items but no
// trailing one. Calling the returned joiner returns "" the first time
// and `sep` thereafter.
//
// Example:
//
//	{% set sep = joiner(' | ') %}
//	{% for tag in tags %}{{ sep() }}{{ tag }}{% endfor %}
func globalJoiner(seps ...string) *runtime.Joiner {
	s := ", "
	if len(seps) > 0 {
		s = seps[0]
	}
	return runtime.NewJoiner(s)
}

// globalLipsum implements the `lipsum` global: generate Lorem-Ipsum
// placeholder text for layout work.
//
// Signature: lipsum(n=5, html=True, min=20, max=100)
//
// Generates `n` paragraphs of `min` to `max` words each. The result is
// HTML-wrapped in `<p>` tags by default.
//
// Divergence from Python: gojinja uses a smaller built-in word list
// (~150 words) than Python (380+). Output identity isn't a parity goal
// here — lipsum is a layout placeholder, not user-facing data.
//
// Example:
//
//	{{ lipsum(2) }}   →  two paragraphs of <p>…</p>
func globalLipsum(args ...int) any {
	n, min, max := 5, 20, 100
	if len(args) > 0 {
		n = args[0]
	}
	if len(args) > 1 {
		min = args[1]
	}
	if len(args) > 2 {
		max = args[2]
	}
	return generateLipsum(n, min, max, true)
}
