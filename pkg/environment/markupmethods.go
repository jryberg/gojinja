package environment

import (
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
	"rstrip": true, "center": true, "strip": true, "translate": true,
	"expandtabs": true, "swapcase": true, "zfill": true, "casefold": true,
	"removeprefix": true, "removesuffix": true, "partition": true,
	"rpartition": true, "split": true, "rsplit": true, "splitlines": true,
}

// markupEscapedArg is the argument markupsafe escapes before calling the
// str method: replace's replacement and the padding methods' fill.
var markupEscapedArg = map[string]int{"replace": 1, "ljust": 1, "rjust": 1, "center": 1}

// markupMethod returns str method attr bound to a Markup receiver with
// markupsafe's semantics: wrapped methods return Markup, `replace`
// escapes the replacement, `ljust`/`rjust`/`center` escape the fill
// character and `join` escapes every item.
func markupMethod(m escape.Markup, attr string) any {
	switch attr {
	case "striptags", "unescape":
		return KwargsCallable(func(args []any, kwargs map[string]any) (any, error) {
			sig := &signature{fn: "Markup." + attr, params: []string{"self"}, required: 1}
			if msg := sig.check(len(args), kwargs); msg != "" {
				return nil, methodError("%s", msg)
			}
			if attr == "striptags" {
				return stripTags(string(m)), nil
			}
			return escape.Unescape(string(m)), nil
		})
	case "escape":
		// A classmethod: Markup.escape(s).
		return KwargsCallable(func(args []any, kwargs map[string]any) (any, error) {
			sig := &signature{fn: "Markup.escape", params: []string{"cls", "s"}, required: 2, posOnly: 2}
			if msg := sig.check(len(args), kwargs); msg != "" {
				return nil, methodError("%s", msg)
			}
			args, _ = sig.normalize(args, kwargs)
			return escape.Escape(args[0]), nil
		})
	}
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
	base, ok := stringMethod(string(m), attr).(KwargsCallable)
	if !ok || !markupWrapped[attr] {
		return stringMethod(string(m), attr)
	}
	return KwargsCallable(func(args []any, kwargs map[string]any) (any, error) {
		if i, ok := markupEscapedArg[attr]; ok && i < len(args) {
			args = append([]any(nil), args...)
			args[i] = string(escape.Escape(args[i]))
		}
		v, err := base(args, kwargs)
		if err != nil {
			return nil, err
		}
		switch x := v.(type) {
		case string:
			return escape.Markup(x), nil
		case []any:
			return markupItems(x), nil
		case runtime.Tuple:
			return runtime.Tuple(markupItems(x)), nil
		}
		return v, nil
	})
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
