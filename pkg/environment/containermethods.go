package environment

import (
	"fmt"
	"sort"
	"strconv"

	gjerrors "github.com/jryberg/gojinja/pkg/errors"
	"github.com/jryberg/gojinja/pkg/escape"
	"github.com/jryberg/gojinja/pkg/eval"
	"github.com/jryberg/gojinja/pkg/runtime"
)

// The methods of Python's list, tuple and dict, with CPython's argument
// checking and messages. They are KwargsCallables, so arguments reach
// them as the template's values: a list appended to another is the list
// itself, as in Python, not a copy.

// orderedKwargsFunc is a builtin that keeps the keyword arguments in
// call order, as Python does for dict(**kw), namespace(**kw) and
// dict.update(**kw).
type orderedKwargsFunc func(args []any, kwargs *runtime.OrderedDict) (any, error)

// dictClass is the `dict` global: callable like Python's dict, with the
// fromkeys class method.
type dictClass orderedKwargsFunc

// Get exposes dict.fromkeys.
func (d dictClass) Get(attr string) (any, bool) {
	if attr == "fromkeys" {
		return KwargsCallable(dictFromKeys), true
	}
	return nil, false
}

func (dictClass) String() string { return "<class 'dict'>" }

// orderedKwargs collects kwargs into a dict in the given order, or in
// sorted order when the order is unknown.
func orderedKwargs(kwargs map[string]any, order []string) *runtime.OrderedDict {
	out := runtime.NewOrderedDict()
	if order == nil {
		for k := range kwargs {
			order = append(order, k)
		}
		sort.Strings(order)
	}
	for _, k := range order {
		if v, ok := kwargs[k]; ok {
			out.Set(k, v)
		}
	}
	return out
}

// CallOrdered is [Environment.Call] with the keyword arguments' order,
// which the callables keeping it receive.
func (e *Environment) CallOrdered(ctx *runtime.Context, callee any, args []any, kwargs map[string]any, order []string) (any, error) {
	if c, ok := callee.(*runtime.Callable); ok {
		callee = c.Fn
	}
	switch fn := callee.(type) {
	case orderedKwargsFunc:
		return fn(args, orderedKwargs(kwargs, order))
	case dictClass:
		return fn(args, orderedKwargs(kwargs, order))
	}
	return e.Call(ctx, callee, args, kwargs)
}

func methodError(format string, a ...any) error {
	return gjerrors.NewTemplateRuntimeError(fmt.Sprintf(format, a...))
}

// noKwargs is CPython's check for methods without keyword parameters.
func noKwargs(qualname string, kwargs map[string]any) error {
	if len(kwargs) > 0 {
		return methodError("%s() takes no keyword arguments", qualname)
	}
	return nil
}

// oneArg checks a METH_O method's single argument.
func oneArg(qualname string, args []any, kwargs map[string]any) error {
	if err := noKwargs(qualname, kwargs); err != nil {
		return err
	}
	if len(args) != 1 {
		return methodError("%s() takes exactly one argument (%d given)", qualname, len(args))
	}
	return nil
}

// noArgsCheck checks a METH_NOARGS method.
func noArgsCheck(qualname string, args []any, kwargs map[string]any) error {
	if err := noKwargs(qualname, kwargs); err != nil {
		return err
	}
	if len(args) != 0 {
		return methodError("%s() takes no arguments (%d given)", qualname, len(args))
	}
	return nil
}

// argCount is CPython's _PyArg_CheckPositional, which names the method
// without its type.
func argCount(name string, args []any, min, max int) error {
	plural := func(n int) string {
		if n == 1 {
			return ""
		}
		return "s"
	}
	switch {
	case len(args) < min:
		qual := ""
		if min < max {
			qual = "at least "
		}
		return methodError("%s expected %s%d argument%s, got %d", name, qual, min, plural(min), len(args))
	case len(args) > max:
		qual := ""
		if min < max {
			qual = "at most "
		}
		return methodError("%s expected %s%d argument%s, got %d", name, qual, max, plural(max), len(args))
	}
	return nil
}

// sliceBound is a start/stop argument of index(): an integer, negative
// counting from the end, clamped to the sequence.
func sliceBound(v any, n int) (int, error) {
	i, err := indexArg(v)
	if err != nil {
		return 0, methodError("slice indices must be integers or have an __index__ method")
	}
	if i < 0 {
		i += n
		if i < 0 {
			i = 0
		}
	}
	return min(i, n), nil
}

// seqIndexOf is list.index / tuple.index.
func seqIndexOf(typ string, items []any, args []any, kwargs map[string]any) (any, error) {
	if err := noKwargs(typ+".index", kwargs); err != nil {
		return nil, err
	}
	if err := argCount("index", args, 1, 3); err != nil {
		return nil, err
	}
	start, stop := 0, len(items)
	var err error
	if len(args) > 1 {
		if start, err = sliceBound(args[1], len(items)); err != nil {
			return nil, err
		}
	}
	if len(args) > 2 {
		if stop, err = sliceBound(args[2], len(items)); err != nil {
			return nil, err
		}
	}
	for i := start; i < stop; i++ {
		eq, err := eval.SameOrEqual(items[i], args[0])
		if err != nil {
			return nil, err
		}
		if eq {
			return i, nil
		}
	}
	if typ == "tuple" {
		return nil, methodError("tuple.index(x): x not in tuple")
	}
	return nil, methodError("%s is not in list", escape.Repr(args[0]))
}

// seqCount is list.count / tuple.count.
func seqCount(typ string, items []any, args []any, kwargs map[string]any) (any, error) {
	if err := oneArg(typ+".count", args, kwargs); err != nil {
		return nil, err
	}
	n := 0
	for _, it := range items {
		eq, err := eval.SameOrEqual(it, args[0])
		if err != nil {
			return nil, err
		}
		if eq {
			n++
		}
	}
	return n, nil
}

// seqMethod returns the methods of a sequence that cannot change: a
// tuple's, or a Go slice's (count, index and copy of a list).
func seqMethod(typ string, items []any, attr string) any {
	switch attr {
	case "count":
		return KwargsCallable(func(args []any, kwargs map[string]any) (any, error) {
			return seqCount(typ, items, args, kwargs)
		})
	case "index":
		return KwargsCallable(func(args []any, kwargs map[string]any) (any, error) {
			return seqIndexOf(typ, items, args, kwargs)
		})
	case "copy":
		if typ == "list" {
			return KwargsCallable(func(args []any, kwargs map[string]any) (any, error) {
				if err := noArgsCheck("list.copy", args, kwargs); err != nil {
					return nil, err
				}
				return runtime.NewPyList(append([]any(nil), items...)), nil
			})
		}
	}
	return nil
}

// listMethod returns Python's list method attr bound to l.
func listMethod(env *Environment, ctx *runtime.Context, l *runtime.PyList, attr string) any {
	if l == nil {
		return nil
	}
	method := func(f func(args []any, kwargs map[string]any) (any, error)) any { return KwargsCallable(f) }
	switch attr {
	case "append":
		return method(func(args []any, kwargs map[string]any) (any, error) {
			if err := oneArg("list.append", args, kwargs); err != nil {
				return nil, err
			}
			l.Append(args[0])
			return nil, nil
		})
	case "extend":
		return method(func(args []any, kwargs map[string]any) (any, error) {
			if err := oneArg("list.extend", args, kwargs); err != nil {
				return nil, err
			}
			items, err := eval.ToIterable(args[0])
			if err != nil {
				return nil, methodError("%s", err)
			}
			l.Extend(append([]any(nil), items...))
			return nil, nil
		})
	case "insert":
		return method(func(args []any, kwargs map[string]any) (any, error) {
			if err := noKwargs("list.insert", kwargs); err != nil {
				return nil, err
			}
			if err := argCount("insert", args, 2, 2); err != nil {
				return nil, err
			}
			i, err := indexArg(args[0])
			if err != nil {
				return nil, err
			}
			l.Insert(i, args[1])
			return nil, nil
		})
	case "pop":
		return method(func(args []any, kwargs map[string]any) (any, error) {
			if err := noKwargs("list.pop", kwargs); err != nil {
				return nil, err
			}
			if err := argCount("pop", args, 0, 1); err != nil {
				return nil, err
			}
			if len(args) == 0 {
				return l.Pop(0, false)
			}
			i, err := indexArg(args[0])
			if err != nil {
				return nil, err
			}
			return l.Pop(i, true)
		})
	case "remove":
		return method(func(args []any, kwargs map[string]any) (any, error) {
			if err := oneArg("list.remove", args, kwargs); err != nil {
				return nil, err
			}
			for i, it := range l.Items() {
				eq, err := eval.SameOrEqual(it, args[0])
				if err != nil {
					return nil, err
				}
				if eq {
					l.RemoveAt(i)
					return nil, nil
				}
			}
			return nil, methodError("list.remove(x): x not in list")
		})
	case "clear", "reverse":
		return method(func(args []any, kwargs map[string]any) (any, error) {
			if err := noArgsCheck("list."+attr, args, kwargs); err != nil {
				return nil, err
			}
			if attr == "clear" {
				l.Clear()
			} else {
				l.Reverse()
			}
			return nil, nil
		})
	case "sort":
		return method(func(args []any, kwargs map[string]any) (any, error) {
			return nil, sortList(env, ctx, l, args, kwargs)
		})
	}
	return seqMethod("list", l.Items(), attr)
}

// sortList is list.sort(*, key=None, reverse=False): a stable sort by
// `<`. It sorts a copy and keeps it only when every comparison worked.
func sortList(env *Environment, ctx *runtime.Context, l *runtime.PyList, args []any, kwargs map[string]any) error {
	if len(args) > 0 {
		return methodError("sort() takes no positional arguments")
	}
	var key any
	reverse := false
	for name, v := range kwargs {
		switch name {
		case "key":
			key = v
		case "reverse":
			n, err := indexArg(v)
			if err != nil {
				return err
			}
			reverse = n != 0
		default:
			return methodError("%s is an invalid keyword argument for sort()", escape.Repr(name))
		}
	}
	items := append([]any(nil), l.Items()...)
	keys := items
	if key != nil {
		keys = make([]any, len(items))
		for i, it := range items {
			k, err := env.Call(ctx, key, []any{it}, nil)
			if err != nil {
				return err
			}
			keys[i] = k
		}
	}
	idx := make([]int, len(items))
	for i := range idx {
		idx[i] = i
	}
	var cmpErr error
	sort.SliceStable(idx, func(i, j int) bool {
		a, b := keys[idx[i]], keys[idx[j]]
		if reverse {
			a, b = b, a
		}
		lt, err := eval.Compare("lt", a, b)
		if err != nil && cmpErr == nil {
			cmpErr = err
		}
		return lt
	})
	if cmpErr != nil {
		return cmpErr
	}
	sorted := make([]any, len(items))
	for i, j := range idx {
		sorted[i] = items[j]
	}
	l.Clear()
	l.Extend(sorted)
	return nil
}

// dictMethod returns Python's dict method attr bound to m. A Go map
// reached through a host value gets the methods of a snapshot of it.
// proxyMethods are the methods of a mappingproxy, which cannot change
// its dict.
var proxyMethods = map[string]bool{"get": true, "keys": true, "values": true, "items": true, "copy": true}

func dictMethod(m any, attr string) any {
	d, ok := m.(*runtime.OrderedDict)
	if !ok {
		d, ok = runtime.NormalizeForTemplate(m).(*runtime.OrderedDict)
		if !ok || d == nil {
			return nil
		}
	}
	method := func(f func(args []any, kwargs map[string]any) (any, error)) any { return KwargsCallable(f) }
	if d.IsProxy() && !proxyMethods[attr] {
		return nil
	}
	switch attr {
	case "keys", "values", "items":
		return method(func(args []any, kwargs map[string]any) (any, error) {
			if err := noArgsCheck("dict."+attr, args, kwargs); err != nil {
				return nil, err
			}
			return &runtime.DictView{Dict: d, Kind: attr}, nil
		})
	case "get":
		return method(func(args []any, kwargs map[string]any) (any, error) {
			if err := noKwargs("dict.get", kwargs); err != nil {
				return nil, err
			}
			if err := argCount("get", args, 1, 2); err != nil {
				return nil, err
			}
			if _, err := runtime.HashKey(args[0]); err != nil {
				return nil, methodError("%s", err)
			}
			if v, ok := d.Get(args[0]); ok {
				return v, nil
			}
			if len(args) == 2 {
				return args[1], nil
			}
			return nil, nil
		})
	case "copy", "clear":
		return method(func(args []any, kwargs map[string]any) (any, error) {
			if err := noArgsCheck("dict."+attr, args, kwargs); err != nil {
				return nil, err
			}
			if attr == "copy" {
				return d.Copy(), nil
			}
			d.Clear()
			return nil, nil
		})
	case "pop":
		return method(func(args []any, kwargs map[string]any) (any, error) {
			if err := noKwargs("dict.pop", kwargs); err != nil {
				return nil, err
			}
			if err := argCount("pop", args, 1, 2); err != nil {
				return nil, err
			}
			if _, err := runtime.HashKey(args[0]); err != nil {
				return nil, methodError("%s", err)
			}
			if len(args) == 2 {
				return d.Pop(args[0], args[1], true)
			}
			v, err := d.Pop(args[0], nil, false)
			if err != nil {
				return nil, methodError("%s", err)
			}
			return v, nil
		})
	case "popitem":
		return method(func(args []any, kwargs map[string]any) (any, error) {
			if err := noArgsCheck("dict.popitem", args, kwargs); err != nil {
				return nil, err
			}
			item, err := d.PopItem()
			if err != nil {
				// str() of Python's KeyError quotes its message.
				return nil, methodError("%s", escape.Repr(err.Error()))
			}
			return item, nil
		})
	case "setdefault":
		return method(func(args []any, kwargs map[string]any) (any, error) {
			if err := noKwargs("dict.setdefault", kwargs); err != nil {
				return nil, err
			}
			if err := argCount("setdefault", args, 1, 2); err != nil {
				return nil, err
			}
			if _, err := runtime.HashKey(args[0]); err != nil {
				return nil, methodError("%s", err)
			}
			var def any
			if len(args) == 2 {
				def = args[1]
			}
			return d.SetDefault(args[0], def), nil
		})
	case "update":
		return orderedKwargsFunc(func(args []any, kwargs *runtime.OrderedDict) (any, error) {
			if err := argCount("update", args, 0, 1); err != nil {
				return nil, err
			}
			if len(args) == 1 {
				if err := updateDict(d, args[0]); err != nil {
					return nil, err
				}
			}
			for _, k := range kwargs.Keys() {
				v, _ := kwargs.Get(k)
				d.Set(k, v)
			}
			return nil, nil
		})
	case "fromkeys":
		return KwargsCallable(dictFromKeys)
	}
	return nil
}

// updateDict merges src into d as dict.update(src) and dict(src) do: a
// mapping's items, or an iterable of key/value pairs.
func updateDict(d *runtime.OrderedDict, src any) error {
	if m, ok := src.(*runtime.OrderedDict); ok {
		for _, k := range m.Keys() {
			v, _ := m.Get(k)
			d.Set(k, v)
		}
		return nil
	}
	if m, ok := runtime.NormalizeForTemplate(src).(*runtime.OrderedDict); ok && m != nil {
		return updateDict(d, m)
	}
	items, err := eval.ToIterable(src)
	if err != nil {
		return methodError("%s", err)
	}
	for i, it := range items {
		pair, err := eval.ToIterable(it)
		if err != nil {
			return methodError("cannot convert dictionary update sequence element #%d to a sequence", i)
		}
		if len(pair) != 2 {
			return methodError("dictionary update sequence element #%d has length %d; 2 is required", i, len(pair))
		}
		if err := d.Put(pair[0], pair[1]); err != nil {
			return methodError("%s", err)
		}
	}
	return nil
}

// dictFromKeys is dict.fromkeys(iterable, value=None).
func dictFromKeys(args []any, kwargs map[string]any) (any, error) {
	if err := noKwargs("dict.fromkeys", kwargs); err != nil {
		return nil, err
	}
	if err := argCount("fromkeys", args, 1, 2); err != nil {
		return nil, err
	}
	keys, err := eval.ToIterable(args[0])
	if err != nil {
		return nil, methodError("%s", err)
	}
	var value any
	if len(args) == 2 {
		value = args[1]
	}
	out := runtime.NewOrderedDict()
	for _, k := range keys {
		if err := out.Put(k, value); err != nil {
			return nil, methodError("%s", err)
		}
	}
	return out, nil
}

func stringifyKey(k any) string {
	if s, ok := k.(string); ok {
		return s
	}
	if i, ok := k.(int64); ok {
		return strconv.FormatInt(i, 10)
	}
	return fmt.Sprint(k)
}
