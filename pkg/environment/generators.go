package environment

import (
	"fmt"
	"sort"

	gjerrors "github.com/jryberg/gojinja/pkg/errors"
	"github.com/jryberg/gojinja/pkg/escape"
	"github.com/jryberg/gojinja/pkg/eval"
	"github.com/jryberg/gojinja/pkg/runtime"
)

// The filters that are Python generator functions return a
// [runtime.Iterator]: lazy, true even when empty, without a length, and
// consumed once. As in Python, a generator's body — argument checks
// included — runs when its first item is requested.

func emptyNext() (any, bool, error) { return nil, false, nil }

// truthErr is bool(v), which raises for a StrictUndefined.
func truthErr(v any) (bool, error) {
	if err := eval.FailStrict(v); err != nil {
		return false, err
	}
	return eval.Truthy(v), nil
}

// filterMap implements the `map` filter: apply a filter to each item, or
// extract an attribute from each item.
//
// Signature: map(value, *args, **kwargs)
//
// Two forms:
//   - `map(filter_name, *args, **kwargs)` applies the named filter to
//     each item.
//   - `map(attribute='name', default=...)` extracts the named attribute
//     (dotted path supported) from each item, falling back to `default`
//     when the attribute is undefined.
//
// Returns a generator, as Python does: iterate it, or pass it to `list`.
// A false value (`None`, an empty list, Undefined) yields nothing.
//
// Example:
//
//	{{ ['a', 'b']  | map('upper')         | list }}  →  ['A', 'B']
//	{{ users       | map(attribute='name') | list }}  →  list of names
func filterMap(env *Environment, ctx *runtime.Context, value any, args []any, kwargs map[string]any) (any, error) {
	return runtime.NewGenerator("sync_do_map", func() (runtime.Next, error) {
		if empty, err := skipFalsy(value); empty || err != nil {
			return emptyNext, err
		}
		fn, err := prepareMap(env, ctx, args, kwargs)
		if err != nil {
			return nil, err
		}
		next, err := eval.Iterate(value)
		if err != nil {
			return nil, err
		}
		return func() (any, bool, error) {
			it, ok, err := next()
			if err != nil || !ok {
				return nil, false, err
			}
			v, err := fn(it)
			if err != nil {
				return nil, false, err
			}
			return v, true, nil
		}, nil
	}), nil
}

// prepareMap is prepare_map: the function map applies to each item.
func prepareMap(env *Environment, ctx *runtime.Context, args []any, kwargs map[string]any) (func(any) (any, error), error) {
	if attribute, ok := kwargs["attribute"]; ok && len(args) == 0 {
		var extra []string
		for k := range kwargs {
			if k != "attribute" && k != "default" {
				extra = append(extra, k)
			}
		}
		if len(extra) > 0 {
			sort.Strings(extra)
			return nil, gjerrors.NewFilterArgumentError("Unexpected keyword argument " + escape.Repr(extra[0]))
		}
		return makeAttrGetter(env, attribute, false, kwargs["default"]), nil
	}
	if len(args) == 0 {
		return nil, gjerrors.NewFilterArgumentError("map requires a filter argument")
	}
	name, rest := args[0], args[1:]
	return func(item any) (any, error) {
		fname, ok := name.(string)
		if !ok {
			return nil, gjerrors.NewTemplateRuntimeError(fmt.Sprintf("No filter named %s.", escape.Repr(name)))
		}
		return env.CallFilter(fname, ctx, item, rest, kwargs)
	}, nil
}

// filterSelect implements the `select` filter: keep items where a test
// passes.
//
// Signature: select(value, test_name=None, *test_args, **test_kwargs)
//
// With no arguments, keeps items that are truthy. With a test name,
// applies the named test to each item. Returns a generator.
//
// Example:
//
//	{{ [1, 2, 3, 4]  | select('odd') | list }}      →  [1, 3]
//	{{ [0, 1, 2, '', 'x'] | select | list }}         →  [1, 2, 'x']
func filterSelect(env *Environment, ctx *runtime.Context, value any, args []any, kwargs map[string]any) (any, error) {
	return selectReject(env, ctx, value, args, kwargs, true, false), nil
}

// filterReject implements the `reject` filter: drop items where a test
// passes (the complement of [filterSelect]).
//
// Signature: reject(value, test_name=None, *test_args, **test_kwargs)
//
// Returns a generator.
//
// Example:
//
//	{{ [1, 2, 3, 4]  | reject('odd') | list }}  →  [2, 4]
func filterReject(env *Environment, ctx *runtime.Context, value any, args []any, kwargs map[string]any) (any, error) {
	return selectReject(env, ctx, value, args, kwargs, false, false), nil
}

// filterSelectAttr implements the `selectattr` filter: keep items whose
// named attribute passes a test.
//
// Signature: selectattr(value, attribute, test_name=None, *test_args, **test_kwargs)
//
// With no test, keeps items where the attribute is truthy. Returns a
// generator.
//
// Example:
//
//	{{ users | selectattr('admin') | list }}                →  admins
//	{{ users | selectattr('age', 'gt', 18) | list }}        →  adults
func filterSelectAttr(env *Environment, ctx *runtime.Context, value any, args []any, kwargs map[string]any) (any, error) {
	return selectReject(env, ctx, value, args, kwargs, true, true), nil
}

// filterRejectAttr implements the `rejectattr` filter: drop items whose
// named attribute passes a test (complement of [filterSelectAttr]).
//
// Signature: rejectattr(value, attribute, test_name=None, *test_args, **test_kwargs)
//
// Returns a generator.
//
// Example:
//
//	{{ users | rejectattr('banned') | list }}  →  non-banned users
func filterRejectAttr(env *Environment, ctx *runtime.Context, value any, args []any, kwargs map[string]any) (any, error) {
	return selectReject(env, ctx, value, args, kwargs, false, true), nil
}

// selectReject is select_or_reject: the items whose test result is keep.
func selectReject(env *Environment, ctx *runtime.Context, value any, args []any, kwargs map[string]any, keep, lookupAttr bool) *runtime.Iterator {
	return runtime.NewGenerator("select_or_reject", func() (runtime.Next, error) {
		if empty, err := skipFalsy(value); empty || err != nil {
			return emptyNext, err
		}
		// prepare_select_or_reject: the attribute, then the test and its
		// arguments; without a test the value's truth decides.
		transform := func(v any) (any, error) { return v, nil }
		targs := args
		if lookupAttr {
			if len(targs) == 0 {
				return nil, gjerrors.NewFilterArgumentError("Missing parameter for attribute name")
			}
			transform = makeAttrGetter(env, targs[0], false, nil)
			targs = targs[1:]
		}
		test := truthErr
		if len(targs) > 0 {
			name, rest := targs[0], targs[1:]
			test = func(v any) (bool, error) {
				tname, ok := name.(string)
				if !ok {
					return false, gjerrors.NewTemplateRuntimeError(fmt.Sprintf("No test named %s.", escape.Repr(name)))
				}
				res, err := env.CallTest(tname, ctx, v, rest, kwargs)
				if err != nil {
					return false, err
				}
				return truthErr(res)
			}
		}
		next, err := eval.Iterate(value)
		if err != nil {
			return nil, err
		}
		return func() (any, bool, error) {
			for {
				it, ok, err := next()
				if err != nil || !ok {
					return nil, false, err
				}
				probe, err := transform(it)
				if err != nil {
					return nil, false, err
				}
				pass, err := test(probe)
				if err != nil {
					return nil, false, err
				}
				if pass == keep {
					return it, true, nil
				}
			}
		}, nil
	})
}

// filterUnique implements the `unique` filter: drop duplicate items
// preserving input order.
//
// Signature: unique(value, case_sensitive=False, attribute=None)
//
// Equality is value equality for primitives. String comparisons are
// case-insensitive by default — first-seen casing wins. With `attribute`,
// items are deduplicated by that (dotted) attribute. Returns a generator.
//
// Example:
//
//	{{ [1, 2, 1, 3]            | unique | list }}  →  [1, 2, 3]
//	{{ ['Foo', 'foo', 'BAR']   | unique | list }}  →  ['Foo', 'BAR']
func filterUnique(env *Environment, _ *runtime.Context, value any, args []any, kwargs map[string]any) (any, error) {
	caseSensitive := false
	if v, ok := paramArg(args, kwargs, 0, "case_sensitive"); ok {
		caseSensitive = eval.Truthy(v)
	}
	attribute, _ := paramArg(args, kwargs, 1, "attribute")
	return runtime.NewGenerator("sync_do_unique", func() (runtime.Next, error) {
		getter := makeAttrGetter(env, attribute, !caseSensitive, nil)
		next, err := eval.Iterate(value)
		if err != nil {
			return nil, err
		}
		seen := map[string]bool{}
		return func() (any, bool, error) {
			for {
				it, ok, err := next()
				if err != nil || !ok {
					return nil, false, err
				}
				key, err := getter(it)
				if err != nil {
					return nil, false, err
				}
				hk, err := runtime.HashKey(key)
				if err != nil {
					return nil, false, gjerrors.NewTemplateRuntimeError(err.Error())
				}
				if !seen[hk] {
					seen[hk] = true
					return it, true, nil
				}
			}
		}, nil
	}), nil
}

// filterBatch implements the `batch` filter: group items into rows of N.
//
// Signature: batch(value, linecount, fill_with=None)
//
// The last row is padded with `fill_with` to reach `linecount` if a
// fill value is provided; otherwise it's left short. A port of Python's
// do_batch, including its results for a zero or non-integer linecount.
// Returns a generator of lists.
//
// Example:
//
//	{{ [1, 2, 3, 4, 5]      | batch(2) | list }}     →  [[1, 2], [3, 4], [5]]
//	{{ [1, 2, 3, 4, 5]      | batch(2, 0) | list }}  →  [[1, 2], [3, 4], [5, 0]]
func filterBatch(_ *Environment, _ *runtime.Context, value any, args []any, kwargs map[string]any) (any, error) {
	// The signature check guarantees linecount was passed.
	linecount, _ := paramArg(args, kwargs, 0, "linecount")
	fill, _ := paramArg(args, kwargs, 1, "fill_with")
	return runtime.NewGenerator("do_batch", func() (runtime.Next, error) {
		next, err := eval.Iterate(value)
		if err != nil {
			return nil, err
		}
		var tmp []any
		done := false
		return func() (any, bool, error) {
			for !done {
				item, ok, err := next()
				if err != nil {
					return nil, false, err
				}
				if !ok {
					done = true
					break
				}
				full, err := eval.ValuesEqual(int64(len(tmp)), linecount)
				if err != nil {
					return nil, false, err
				}
				if full {
					row := tmp
					tmp = []any{item}
					return row, true, nil
				}
				tmp = append(tmp, item)
			}
			if len(tmp) == 0 {
				return nil, false, nil
			}
			row := tmp
			tmp = nil
			if fill != nil {
				short, err := eval.Compare("lt", int64(len(row)), linecount)
				if err != nil {
					return nil, false, err
				}
				if short {
					missing, err := eval.BinaryOp("-", linecount, int64(len(row)))
					if err != nil {
						return nil, false, err
					}
					n, err := indexArg(missing)
					if err != nil {
						return nil, false, err
					}
					for range n {
						row = append(row, fill)
					}
				}
			}
			return row, true, nil
		}, nil
	}), nil
}

// filterSlice implements the `slice` filter: distribute items across N
// columns of nearly-equal length.
//
// Signature: slice(value, slices, fill_with=None)
//
// Useful for newspaper-style layouts. Earlier slices get one extra item
// when the count doesn't divide evenly, and with `fill_with` every other
// slice gets it appended. Returns a generator of lists.
//
// Example:
//
//	{{ [1, 2, 3, 4, 5]   | slice(3) | list }}  →  [[1, 2], [3, 4], [5]]
func filterSlice(_ *Environment, _ *runtime.Context, value any, args []any, kwargs map[string]any) (any, error) {
	slices, _ := paramArg(args, kwargs, 0, "slices")
	fill, _ := paramArg(args, kwargs, 1, "fill_with")
	return runtime.NewGenerator("sync_do_slice", func() (runtime.Next, error) {
		items, err := eval.ToIterable(value)
		if err != nil {
			return nil, err
		}
		seq := append([]any(nil), items...)
		length := int64(len(seq))
		perV, err := eval.BinaryOp("//", length, slices)
		if err != nil {
			return nil, err
		}
		extraV, err := eval.BinaryOp("%", length, slices)
		if err != nil {
			return nil, err
		}
		n, err := indexArg(slices)
		if err != nil {
			return nil, err
		}
		per, err := indexArg(perV)
		if err != nil {
			return nil, err
		}
		extra, err := indexArg(extraV)
		if err != nil {
			return nil, err
		}
		i, offset := 0, 0
		return func() (any, bool, error) {
			if i >= n {
				return nil, false, nil
			}
			start := offset + i*per
			if i < extra {
				offset++
			}
			end := offset + (i+1)*per
			row := append([]any{}, seq[min(start, len(seq)):min(end, len(seq))]...)
			if fill != nil && i >= extra {
				row = append(row, fill)
			}
			i++
			return row, true, nil
		}, nil
	}), nil
}

// filterItems implements the `items` filter: yield (key, value) pairs for
// a mapping.
//
// Signature: items(d)
//
// Returns a generator of two-element tuples, in the dict's order. For Go
// map types, keys are sorted lexicographically (Go map iteration order
// is non-deterministic; sorting keeps output stable across renders).
// Undefined input yields nothing; anything else that is not a mapping
// raises once the generator is read.
//
// Example:
//
//	{% for k, v in {'a': 1, 'b': 2} | items %}{{ k }}={{ v }} {% endfor %}
//	→  a=1 b=2
func filterItems(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (any, error) {
	return runtime.NewDelegatingGenerator("do_items", "dict_itemiterator", func() (runtime.Next, error) {
		if runtime.IsUndefined(value) {
			return emptyNext, nil
		}
		switch x := value.(type) {
		case *runtime.OrderedDict:
			keys, err := eval.Iterate(x)
			if err != nil {
				return nil, err
			}
			return func() (any, bool, error) {
				k, ok, err := keys()
				if err != nil || !ok {
					return nil, false, err
				}
				v, _ := x.Get(k)
				return runtime.Tuple{k, v}, true, nil
			}, nil
		case map[string]any, map[any]any:
			keys, err := eval.ToIterable(x)
			if err != nil {
				return nil, err
			}
			items := make([]any, len(keys))
			for i, k := range keys {
				if m, ok := x.(map[string]any); ok {
					items[i] = runtime.Tuple{k, m[k.(string)]}
				} else {
					items[i] = runtime.Tuple{k, x.(map[any]any)[k]}
				}
			}
			return eval.SliceNext(items), nil
		}
		return nil, gjerrors.NewTemplateRuntimeError("Can only get item pairs from a mapping.")
	}), nil
}

// filterReverse implements the `reverse` filter: reverse a string or
// sequence.
//
// Signature: reverse(value)
//
// Strings are reversed by character, and a Markup stays Markup. Other
// iterables return Python's reverse iterator for their type (iterate it,
// or pass it to `list`); a generator, which cannot be reversed, is read
// into a new reversed list.
//
// Example:
//
//	{{ "abc"      | reverse }}         →  cba
//	{{ [1, 2, 3]  | reverse | list }}  →  [3, 2, 1]
func filterReverse(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (any, error) {
	switch x := value.(type) {
	case string:
		return reverseRunes(x), nil
	case escape.Markup:
		// value[::-1] on a Markup is a Markup.
		return escape.Markup(reverseRunes(string(x))), nil
	case *runtime.PyList:
		// Like list_reverseiterator, it reads the list as it is then.
		i := x.Len() - 1
		return runtime.NewIterator("list_reverseiterator", func() (any, bool, error) {
			if i < 0 || i >= x.Len() {
				i = -1
				return nil, false, nil
			}
			i--
			return x.Items()[i+1], true, nil
		}), nil
	case []any, []string:
		items, _ := eval.ToIterable(x)
		return runtime.NewIterator("list_reverseiterator", reversedNext(items)), nil
	case runtime.Tuple, runtime.Bytes:
		items, _ := eval.ToIterable(x)
		return runtime.NewIterator("reversed", reversedNext(items)), nil
	case *runtime.Range:
		return runtime.NewIterator("range_iterator", reversedNext(x.List())), nil
	case *runtime.OrderedDict:
		return runtime.NewIterator("dict_reversekeyiterator", eval.SizeChecked(reversedItems(x.Keys()), x.Len)), nil
	case *runtime.DictView:
		kind := map[string]string{"keys": "key", "values": "value", "items": "item"}[x.Kind]
		return runtime.NewIterator("dict_reverse"+kind+"iterator", eval.SizeChecked(reversedItems(x.Items()), x.Len)), nil
	case map[string]any, map[any]any:
		keys, _ := eval.ToIterable(x)
		return runtime.NewIterator("dict_reversekeyiterator", reversedNext(keys)), nil
	case runtime.Undefined:
		// reversed() takes an Undefined's length, which a strict one
		// refuses.
		if _, err := x.Iter(); err != nil {
			return nil, err
		}
		return runtime.NewIterator("reversed", emptyNext), nil
	}
	// reversed() refused it: Python reverses list(value) instead.
	items, err := eval.ToIterable(value)
	if err != nil || value == nil {
		if runtime.IsUndefined(value) {
			return nil, err
		}
		return nil, gjerrors.NewFilterArgumentError("argument must be iterable")
	}
	return reversedItems(items), nil
}

func reversedItems(items []any) []any {
	out := make([]any, len(items))
	for i, it := range items {
		out[len(items)-1-i] = it
	}
	return out
}

func reversedNext(items []any) runtime.Next {
	return eval.SliceNext(reversedItems(items))
}
