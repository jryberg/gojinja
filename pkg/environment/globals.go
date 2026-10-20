package environment

import (
	"fmt"

	gjerrors "github.com/jryberg/gojinja/pkg/errors"
	"github.com/jryberg/gojinja/pkg/runtime"
)

// globalRange implements the `range` global: bounded Python-style
// range(start, stop, step).
//
// Signature: range(stop) | range(start, stop) | range(start, stop, step)
//
// Returns a list of integers (not a lazy iterator). The result size is
// bounded by [Environment.maxRange] (default 100000); exceeding the
// limit raises a TemplateRuntimeError. The bound is a hardening
// divergence from Python — see [WithRangeLimit].
//
// Example:
//
//	{% for i in range(5) %}{{ i }} {% endfor %}      →  0 1 2 3 4
//	{% for i in range(2, 8, 2) %}{{ i }} {% endfor %}  →  2 4 6
func globalRange(e *Environment) func(...int) ([]any, error) {
	return func(args ...int) ([]any, error) {
		var start, stop, step int
		switch len(args) {
		case 0:
			return nil, gjerrors.NewTemplateRuntimeError("range() requires at least 1 argument")
		case 1:
			start, stop, step = 0, args[0], 1
		case 2:
			start, stop, step = args[0], args[1], 1
		case 3:
			start, stop, step = args[0], args[1], args[2]
		default:
			return nil, gjerrors.NewTemplateRuntimeError("range() takes 1-3 arguments")
		}
		if step == 0 {
			return nil, gjerrors.NewTemplateRuntimeError("range() step argument must not be zero")
		}
		size := 0
		if step > 0 && start < stop {
			size = (stop - start + step - 1) / step
		} else if step < 0 && start > stop {
			size = (start - stop + (-step) - 1) / (-step)
		}
		if size > e.maxRange {
			return nil, gjerrors.NewTemplateRuntimeError(fmt.Sprintf("range size %d exceeds limit %d", size, e.maxRange))
		}
		out := make([]any, 0, size)
		for i := start; (step > 0 && i < stop) || (step < 0 && i > stop); i += step {
			out = append(out, i)
		}
		return out, nil
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
//	{{ dict(a=1, b=2) | items }}  →  [('a', 1), ('b', 2)]
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
