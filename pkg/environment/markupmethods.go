package environment

import (
	"reflect"
	"strings"

	"github.com/jryberg/gojinja/pkg/escape"
	"github.com/jryberg/gojinja/pkg/eval"
	"github.com/jryberg/gojinja/pkg/runtime"
)

// markupWrapped are the str methods markupsafe's Markup overrides to
// return Markup (or a list/tuple of Markup) instead of str. Every other
// str method behaves exactly as on a plain str.
var markupWrapped = map[string]bool{
	"capitalize": true, "title": true, "lower": true, "upper": true,
	"replace": true, "ljust": true, "rjust": true, "lstrip": true,
	"rstrip": true, "center": true, "strip": true, "swapcase": true,
	"zfill": true, "split": true, "rsplit": true, "splitlines": true,
}

var markupType = reflect.TypeFor[escape.Markup]()

// markupMethod returns str method attr bound to a Markup receiver with
// markupsafe's semantics: wrapped methods return Markup, `replace`
// escapes the replacement, `ljust`/`rjust`/`center` escape the fill
// character and `join` escapes every item.
func markupMethod(m escape.Markup, attr string) any {
	if attr == "join" {
		return func(iter any) (escape.Markup, error) {
			items, err := eval.ToIterable(iter)
			if err != nil {
				return "", err
			}
			parts := make([]string, len(items))
			for i, it := range items {
				parts[i] = string(escape.Escape(it))
			}
			return escape.Markup(strings.Join(parts, string(m))), nil
		}
	}
	base := stringMethod(string(m), attr)
	if base == nil || !markupWrapped[attr] {
		return base
	}
	switch f := base.(type) {
	case func(any, any, ...any) string: // replace
		return func(old, repl any, args ...any) escape.Markup {
			return escape.Markup(f(old, string(escape.Escape(repl)), args...))
		}
	case func(...any) string:
		if attr == "ljust" || attr == "rjust" || attr == "center" {
			return func(args ...any) escape.Markup {
				if len(args) > 1 {
					args = append([]any(nil), args...)
					args[1] = string(escape.Escape(args[1]))
				}
				return escape.Markup(f(args...))
			}
		}
	}
	return markupResults(base)
}

// markupResults wraps fn so its string results become Markup and the
// strings inside a returned list or tuple become Markup too.
func markupResults(fn any) any {
	fv := reflect.ValueOf(fn)
	ft := fv.Type()
	in := make([]reflect.Type, ft.NumIn())
	for i := range in {
		in[i] = ft.In(i)
	}
	out := make([]reflect.Type, ft.NumOut())
	for i := range out {
		out[i] = ft.Out(i)
		if out[i].Kind() == reflect.String {
			out[i] = markupType
		}
	}
	wt := reflect.FuncOf(in, out, ft.IsVariadic())
	return reflect.MakeFunc(wt, func(args []reflect.Value) []reflect.Value {
		var res []reflect.Value
		if ft.IsVariadic() {
			res = fv.CallSlice(args)
		} else {
			res = fv.Call(args)
		}
		for i, r := range res {
			switch v := r.Interface().(type) {
			case string:
				res[i] = reflect.ValueOf(escape.Markup(v))
			case []any:
				res[i] = reflect.ValueOf(markupItems(v))
			case runtime.Tuple:
				res[i] = reflect.ValueOf(runtime.Tuple(markupItems(v)))
			}
		}
		return res
	}).Interface()
}

func markupItems(items []any) []any {
	out := make([]any, len(items))
	for i, it := range items {
		if s, ok := it.(string); ok {
			out[i] = escape.Markup(s)
		} else {
			out[i] = it
		}
	}
	return out
}
