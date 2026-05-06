package environment

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode"

	gjerrors "github.com/jryberg/gojinja/pkg/errors"
	"github.com/jryberg/gojinja/pkg/escape"
	"github.com/jryberg/gojinja/pkg/runtime"
)

// registerBuiltins seeds an Environment with the standard filters,
// tests, and globals.
func registerBuiltins(e *Environment) {
	// ------------------------------------------------------- globals
	e.globals["range"] = globalRange(e)
	e.globals["dict"] = KwargsCallable(globalDict)
	e.globals["namespace"] = namespaceCtor
	e.globals["cycler"] = globalCycler
	e.globals["joiner"] = globalJoiner
	e.globals["lipsum"] = globalLipsum

	// ------------------------------------------------------- filters
	e.filters["upper"] = Filter{Func: filterUpper}
	e.filters["lower"] = Filter{Func: filterLower}
	e.filters["title"] = Filter{Func: filterTitle}
	e.filters["capitalize"] = Filter{Func: filterCapitalize}
	e.filters["trim"] = Filter{Func: filterTrim}
	e.filters["length"] = Filter{Func: filterLength}
	e.filters["count"] = Filter{Func: filterLength}
	e.filters["string"] = Filter{Func: filterString}
	e.filters["safe"] = Filter{Func: filterSafe}
	e.filters["e"] = Filter{Func: filterEscape}
	e.filters["escape"] = Filter{Func: filterEscape}
	e.filters["forceescape"] = Filter{Func: filterForceEscape}
	e.filters["default"] = Filter{Func: filterDefault}
	e.filters["d"] = Filter{Func: filterDefault}
	e.filters["join"] = Filter{Func: filterJoin}
	e.filters["replace"] = Filter{Func: filterReplace}
	e.filters["reverse"] = Filter{Func: filterReverse}
	e.filters["abs"] = Filter{Func: filterAbs}
	e.filters["int"] = Filter{Func: filterInt}
	e.filters["float"] = Filter{Func: filterFloat}
	e.filters["list"] = Filter{Func: filterList}
	e.filters["first"] = Filter{Func: filterFirst}
	e.filters["last"] = Filter{Func: filterLast}
	e.filters["sort"] = Filter{Func: filterSort}
	e.filters["sum"] = Filter{Func: filterSum}
	e.filters["min"] = Filter{Func: filterMin}
	e.filters["max"] = Filter{Func: filterMax}
	e.filters["items"] = Filter{Func: filterItems}
	e.filters["dictsort"] = Filter{Func: filterDictsort}
	e.filters["unique"] = Filter{Func: filterUnique}
	e.filters["batch"] = Filter{Func: filterBatch}
	e.filters["slice"] = Filter{Func: filterSlice}
	e.filters["truncate"] = Filter{Func: filterTruncate}
	e.filters["wordcount"] = Filter{Func: filterWordcount}
	e.filters["indent"] = Filter{Func: filterIndent}
	e.filters["striptags"] = Filter{Func: filterStripTags}
	e.filters["center"] = Filter{Func: filterCenter}
	e.filters["round"] = Filter{Func: filterRound}
	e.filters["filesizeformat"] = Filter{Func: filterFilesizeformat}
	e.filters["urlencode"] = Filter{Func: filterUrlencode}
	e.filters["tojson"] = Filter{Func: filterTojson}
	e.filters["attr"] = Filter{Func: filterAttr}
	e.filters["format"] = Filter{Func: filterFormat}
	e.filters["map"] = Filter{Pass: runtime.PassContext, Func: filterMap}
	e.filters["select"] = Filter{Pass: runtime.PassContext, Func: filterSelect}
	e.filters["reject"] = Filter{Pass: runtime.PassContext, Func: filterReject}
	e.filters["selectattr"] = Filter{Pass: runtime.PassContext, Func: filterSelectAttr}
	e.filters["rejectattr"] = Filter{Pass: runtime.PassContext, Func: filterRejectAttr}
	e.filters["groupby"] = Filter{Func: filterGroupby}
	e.filters["wordwrap"] = Filter{Func: filterWordwrap}
	e.filters["pprint"] = Filter{Func: filterPprint}
	e.filters["random"] = Filter{Func: filterRandom}
	e.filters["xmlattr"] = Filter{Func: filterXmlattr}
	e.filters["urlize"] = Filter{Func: filterUrlize}

	// ------------------------------------------------------- tests
	e.tests["defined"] = Test{Func: testDefined}
	e.tests["undefined"] = Test{Func: testUndefined}
	e.tests["none"] = Test{Func: testNone}
	e.tests["false"] = Test{Func: testFalse}
	e.tests["true"] = Test{Func: testTrue}
	e.tests["boolean"] = Test{Func: testBoolean}
	e.tests["string"] = Test{Func: testString}
	e.tests["number"] = Test{Func: testNumber}
	e.tests["integer"] = Test{Func: testInteger}
	e.tests["float"] = Test{Func: testFloat}
	e.tests["odd"] = Test{Func: testOdd}
	e.tests["even"] = Test{Func: testEven}
	e.tests["divisibleby"] = Test{Func: testDivisibleBy}
	e.tests["sequence"] = Test{Func: testSequence}
	e.tests["mapping"] = Test{Func: testMapping}
	e.tests["iterable"] = Test{Func: testIterable}
	e.tests["lower"] = Test{Func: testLowerStr}
	e.tests["upper"] = Test{Func: testUpperStr}
	e.tests["==" /* eq alias */] = Test{Func: testEq}
	e.tests["eq"] = Test{Func: testEq}
	e.tests["equalto"] = Test{Func: testEq}
	e.tests["!="] = Test{Func: testNe}
	e.tests["ne"] = Test{Func: testNe}
	e.tests["<"] = Test{Func: testLt}
	e.tests["lt"] = Test{Func: testLt}
	e.tests["lessthan"] = Test{Func: testLt}
	e.tests["<="] = Test{Func: testLtEq}
	e.tests["le"] = Test{Func: testLtEq}
	e.tests[">"] = Test{Func: testGt}
	e.tests["gt"] = Test{Func: testGt}
	e.tests["greaterthan"] = Test{Func: testGt}
	e.tests[">="] = Test{Func: testGtEq}
	e.tests["ge"] = Test{Func: testGtEq}
	e.tests["in"] = Test{Func: testIn}
	e.tests["sameas"] = Test{Func: testSameAs}
	e.tests["callable"] = Test{Func: testCallable}
	e.tests["escaped"] = Test{Func: testEscaped}
	e.tests["filter"] = Test{Func: testIsFilter}
	e.tests["test"] = Test{Func: testIsTest}
}

// namespaceCtor implements the `namespace` global: a mutable container
// you can write to from inside a `{% set %}` block.
//
// Signature: namespace() | namespace(mapping) | namespace(**kwargs)
//
// Returns a [runtime.Namespace]. The point of `namespace` is to escape
// the loop-local scoping of plain `{% set %}` so accumulator-style
// patterns work:
//
//	{% set ns = namespace(found=false) %}
//	{% for item in items %}
//	  {% if item.match %}{% set ns.found = true %}{% endif %}
//	{% endfor %}
//	{% if ns.found %}…{% endif %}
//
// Both a positional mapping and keyword arguments seed the resulting
// Namespace.
func namespaceCtor(args []any, kwargs map[string]any) (any, error) {
	seed := map[string]any{}
	for _, a := range args {
		switch x := a.(type) {
		case map[string]any:
			for k, v := range x {
				seed[k] = v
			}
		case map[any]any:
			for k, v := range x {
				if s, ok := k.(string); ok {
					seed[s] = v
				}
			}
		case *runtime.OrderedDict:
			for _, k := range x.Keys() {
				if s, ok := k.(string); ok {
					v, _ := x.Get(k)
					seed[s] = v
				}
			}
		default:
			return nil, gjerrors.NewTemplateRuntimeError(fmt.Sprintf("namespace argument must be a mapping, got %T", a))
		}
	}
	for k, v := range kwargs {
		seed[k] = v
	}
	return runtime.NewNamespace(seed), nil
}

// =========================================================== Filter funcs

// filterUpper implements the `upper` filter: convert a value to uppercase.
//
// Signature: upper(s)
//
// Stringifies via [escape.SoftStr] then applies [strings.ToUpper] (Unicode-aware).
//
// Example:
//
//	{{ "hello" | upper }}  →  HELLO
func filterUpper(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (any, error) {
	return strings.ToUpper(escape.SoftStr(value)), nil
}

// filterLower implements the `lower` filter: convert a value to lowercase.
//
// Signature: lower(s)
//
// Stringifies via [escape.SoftStr] then applies [strings.ToLower] (Unicode-aware).
//
// Example:
//
//	{{ "HELLO" | lower }}  →  hello
func filterLower(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (any, error) {
	return strings.ToLower(escape.SoftStr(value)), nil
}

// filterTitle implements the `title` filter: title-case a string.
//
// Signature: title(s)
//
// Splits on whitespace and `-`, `(`, `{`, `[`, `<`. Apostrophes do NOT
// split words (so `"foo's bar" | title` → `Foo's Bar`, matching Python's
// `do_title`, NOT Python's `str.title()`).
//
// Example:
//
//	{{ "hello world" | title }}  →  Hello World
func filterTitle(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (any, error) {
	return jinjaTitle(escape.SoftStr(value)), nil
}

// jinjaTitle implements `do_title` from jinja2/filters.py.
func jinjaTitle(s string) string {
	isWordSep := func(r rune) bool {
		switch r {
		case ' ', '\t', '\n', '\r', '\f', '\v',
			'-', '(', '{', '[', '<':
			return true
		}
		return false
	}
	var b strings.Builder
	atStart := true
	for _, r := range s {
		if isWordSep(r) {
			b.WriteRune(r)
			atStart = true
			continue
		}
		if atStart {
			b.WriteRune(unicode.ToUpper(r))
			atStart = false
		} else {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

// filterCapitalize implements the `capitalize` filter: uppercase the first
// character, lowercase the rest.
//
// Signature: capitalize(s)
//
// Operates on bytes, not runes — input is expected to be UTF-8 with an
// ASCII first character (matching Python's behaviour for the common case).
//
// Example:
//
//	{{ "hello WORLD" | capitalize }}  →  Hello world
func filterCapitalize(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (any, error) {
	s := escape.SoftStr(value)
	if s == "" {
		return s, nil
	}
	return strings.ToUpper(s[:1]) + strings.ToLower(s[1:]), nil
}

// filterTrim implements the `trim` filter: strip surrounding characters.
//
// Signature: trim(s, chars=None)
//
// With no argument, trims ASCII + Unicode whitespace (Go's
// [strings.TrimSpace]). With `chars`, trims any of the given runes from
// both ends (Go's [strings.Trim]).
//
// Example:
//
//	{{ "  hi  " | trim }}        →  hi
//	{{ "##hi##" | trim('#') }}   →  hi
func filterTrim(_ *Environment, _ *runtime.Context, value any, args []any, _ map[string]any) (any, error) {
	s := escape.SoftStr(value)
	if len(args) > 0 {
		if cut, ok := args[0].(string); ok {
			return strings.Trim(s, cut), nil
		}
	}
	return strings.TrimSpace(s), nil
}

// filterLength implements the `length` filter (alias `count`): return the
// number of items in a sequence or mapping, or runes in a string.
//
// Signature: length(value)
//
// Strings: byte length (matches Python's `len()` on `str` for ASCII;
// multi-byte UTF-8 characters count as multiple bytes — same as Python's
// `bytes` and gojinja's other byte-oriented operations).
// Sequences and mappings: number of elements / key-value pairs.
// Undefined: 0 for ModeBase / ModeChainable; ModeStrict raises through Iter.
//
// Example:
//
//	{{ [1, 2, 3] | length }}      →  3
//	{{ "hello"  | length }}       →  5
//	{{ {'a': 1} | length }}       →  1
func filterLength(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (any, error) {
	switch x := value.(type) {
	case string:
		return len(x), nil
	case []any:
		return len(x), nil
	case runtime.Tuple:
		return len(x), nil
	case map[string]any:
		return len(x), nil
	case map[any]any:
		return len(x), nil
	case *runtime.OrderedDict:
		return x.Len(), nil
	case runtime.Undefined:
		// Mirrors Python: Undefined.__len__ → 0 for ModeBase /
		// ModeChainable. ModeStrict raises through Iter; we honour
		// that by routing through Iter() so the error surfaces
		// identically.
		if _, err := x.Iter(); err != nil {
			return 0, err
		}
		return 0, nil
	}
	return 0, gjerrors.NewFilterArgumentError("length: object has no length")
}

// filterString implements the `string` filter: stringify a value the way
// Python's `str()` would.
//
// Signature: string(value)
//
// Critically, this means `None` → `"None"`, `True` → `"True"`, `False` →
// `"False"` (capitalised, matching Python — NOT Go's lowercase form).
// Implemented by [escape.SoftStr].
//
// Example:
//
//	{{ true  | string }}  →  True
//	{{ none  | string }}  →  None
func filterString(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (any, error) {
	return escape.SoftStr(value), nil
}

// filterSafe implements the `safe` filter: mark a value as already-escaped
// so autoescape won't double-escape it.
//
// Signature: safe(value)
//
// Returns a [escape.Markup] wrapping. Once a value is Markup, the engine
// emits it verbatim through `{{ ... }}`. Use with care for any value that
// originated outside trusted code.
//
// Example:
//
//	{{ "<b>x</b>" | safe }}  →  <b>x</b>      (rendered as HTML)
//	{{ "<b>x</b>" }}         →  &lt;b&gt;x&lt;/b&gt;  (autoescaped)
func filterSafe(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (any, error) {
	return escape.Markup(escape.SoftStr(value)), nil
}

// filterEscape implements the `escape` filter (alias `e`): HTML-escape a
// value's stringified form, returning [escape.Markup].
//
// Signature: escape(value)
//
// If the input is already Markup, returns it unchanged (no double-escape).
// To force re-escape, use [filterForceEscape].
//
// Example:
//
//	{{ "<b>" | escape }}  →  &lt;b&gt;
func filterEscape(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (any, error) {
	return escape.Escape(value), nil
}

// filterForceEscape implements the `forceescape` filter: HTML-escape a
// value even if it's already Markup.
//
// Signature: forceescape(value)
//
// Example:
//
//	{{ "<b>" | safe | forceescape }}  →  &lt;b&gt;
func filterForceEscape(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (any, error) {
	return escape.ForceEscape(value), nil
}

// filterDefault implements the `default` filter (alias `d`): substitute a
// fallback when the value is undefined (or, optionally, falsy).
//
// Signature: default(value, default_value="", boolean=False)
//
// With `boolean=True`, the fallback is also used when the value is
// truthy-False (`""`, `0`, empty list, empty map, `None`, `False`).
//
// Example:
//
//	{{ unset_var | default('fallback') }}                       →  fallback
//	{{ ""        | default('fallback', true) }}                 →  fallback
func filterDefault(_ *Environment, _ *runtime.Context, value any, args []any, _ map[string]any) (any, error) {
	def := any("")
	booleanMode := false
	if len(args) > 0 {
		def = args[0]
	}
	if len(args) > 1 {
		if b, ok := args[1].(bool); ok {
			booleanMode = b
		}
	}
	if u, ok := value.(runtime.Undefiner); ok && u.IsUndefined() {
		return def, nil
	}
	if booleanMode {
		if !truthy(value) {
			return def, nil
		}
	}
	return value, nil
}

// filterJoin implements the `join` filter: concatenate the items of an
// iterable, with an optional separator.
//
// Signature: join(value, d="", attribute=None)
//
// `attribute` (positional or keyword) extracts the named attribute from
// each item before stringifying. Strings are iterated as runes (matching
// Python's `sep.join(string)`).
//
// Example:
//
//	{{ [1, 2, 3]     | join('-') }}                  →  1-2-3
//	{{ users         | join(', ', attribute='name') }}  →  Alice, Bob
//	{{ "abc"         | join('.') }}                  →  a.b.c
func filterJoin(env *Environment, _ *runtime.Context, value any, args []any, kwargs map[string]any) (any, error) {
	sep := ""
	if len(args) > 0 {
		sep = escape.SoftStr(args[0])
	}
	// Optional `attribute` kwarg / second positional — extract that
	// attribute from each element before joining.
	var attr string
	if len(args) > 1 {
		attr, _ = args[1].(string)
	} else if v, ok := kwargs["attribute"].(string); ok {
		attr = v
	}
	stringify := func(it any) (string, error) {
		if attr == "" {
			return escape.SoftStr(it), nil
		}
		v, err := env.GetAttr(it, attr)
		if err != nil {
			return "", err
		}
		return escape.SoftStr(v), nil
	}
	switch x := value.(type) {
	case []any:
		parts := make([]string, len(x))
		for i, it := range x {
			s, err := stringify(it)
			if err != nil {
				return nil, err
			}
			parts[i] = s
		}
		return strings.Join(parts, sep), nil
	case []string:
		return strings.Join(x, sep), nil
	case runtime.Tuple:
		parts := make([]string, len(x))
		for i, it := range x {
			s, err := stringify(it)
			if err != nil {
				return nil, err
			}
			parts[i] = s
		}
		return strings.Join(parts, sep), nil
	case string:
		// Python: `sep.join(string)` joins each char with sep.
		var b strings.Builder
		first := true
		for _, r := range x {
			if !first {
				b.WriteString(sep)
			}
			b.WriteRune(r)
			first = false
		}
		return b.String(), nil
	}
	return nil, gjerrors.NewFilterArgumentError(fmt.Sprintf("join: cannot join %T", value))
}

// filterReplace implements the `replace` filter: replace occurrences of
// `old` with `new` in a string.
//
// Signature: replace(s, old, new, count=None)
//
// With `count` set, only the first N occurrences are replaced.
//
// Example:
//
//	{{ "Hello World"   | replace('World', 'Go') }}  →  Hello Go
//	{{ "aaaa"          | replace('a', 'b', 2) }}    →  bbaa
func filterReplace(_ *Environment, _ *runtime.Context, value any, args []any, _ map[string]any) (any, error) {
	if len(args) < 2 {
		return nil, gjerrors.NewFilterArgumentError("replace requires (old, new) args")
	}
	old := escape.SoftStr(args[0])
	new_ := escape.SoftStr(args[1])
	count := -1
	if len(args) >= 3 {
		if n, ok := asInt(args[2]); ok {
			count = n
		}
	}
	return strings.Replace(escape.SoftStr(value), old, new_, count), nil
}

// filterReverse implements the `reverse` filter: reverse a string or
// sequence.
//
// Signature: reverse(value)
//
// Strings are reversed by rune (multi-byte safe). Lists return a new
// reversed slice; the original is left unmodified.
//
// Example:
//
//	{{ "abc"      | reverse }}  →  cba
//	{{ [1, 2, 3]  | reverse }}  →  [3, 2, 1]
func filterReverse(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (any, error) {
	switch x := value.(type) {
	case string:
		runes := []rune(x)
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		return string(runes), nil
	case []any:
		out := make([]any, len(x))
		for i, it := range x {
			out[len(x)-1-i] = it
		}
		return out, nil
	}
	return nil, gjerrors.NewFilterArgumentError(fmt.Sprintf("reverse: cannot reverse %T", value))
}

// filterAbs implements the `abs` filter: absolute value of a number.
//
// Signature: abs(value)
//
// Accepts int, int64, and float64. Other types raise a filter argument
// error.
//
// Example:
//
//	{{ -7    | abs }}  →  7
//	{{ -1.5  | abs }}  →  1.5
func filterAbs(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (any, error) {
	switch x := value.(type) {
	case int:
		if x < 0 {
			return -x, nil
		}
		return x, nil
	case int64:
		if x < 0 {
			return -x, nil
		}
		return x, nil
	case float64:
		if x < 0 {
			return -x, nil
		}
		return x, nil
	}
	return nil, gjerrors.NewFilterArgumentError(fmt.Sprintf("abs: not numeric %T", value))
}

// filterInt implements the `int` filter: convert a value to an integer.
//
// Signature: int(value, default=0, base=10)
//
// Conversion falls back to `default` if parsing fails. Bases other than
// 10 accept the optional `0x` / `0o` / `0b` prefix. Base-10 input that
// looks like a float is truncated toward zero (`"32.32" | int → 32`,
// matching Python).
//
// Example:
//
//	{{ "42"     | int }}             →  42
//	{{ "ff"     | int(0, 16) }}      →  255
//	{{ "bad"    | int(-1) }}         →  -1
func filterInt(_ *Environment, _ *runtime.Context, value any, args []any, kwargs map[string]any) (any, error) {
	def := int64(0)
	if len(args) > 0 {
		if n, ok := asInt(args[0]); ok {
			def = int64(n)
		}
	} else if v, ok := kwargs["default"]; ok {
		if n, ok := asInt(v); ok {
			def = int64(n)
		}
	}
	base := 10
	if len(args) > 1 {
		if n, ok := asInt(args[1]); ok {
			base = n
		}
	} else if v, ok := kwargs["base"]; ok {
		if n, ok := asInt(v); ok {
			base = n
		}
	}
	switch x := value.(type) {
	case int:
		return int64(x), nil
	case int64:
		return x, nil
	case float64:
		return int64(x), nil
	case bool:
		if x {
			return int64(1), nil
		}
		return int64(0), nil
	case string:
		s := strings.TrimSpace(x)
		// Python's int() with base != 10 accepts optional 0x/0o/0b prefix.
		if base != 10 {
			s = strings.TrimPrefix(s, "+")
			negate := false
			if strings.HasPrefix(s, "-") {
				negate = true
				s = s[1:]
			}
			switch base {
			case 16:
				s = strings.TrimPrefix(s, "0x")
				s = strings.TrimPrefix(s, "0X")
			case 8:
				s = strings.TrimPrefix(s, "0o")
				s = strings.TrimPrefix(s, "0O")
			case 2:
				s = strings.TrimPrefix(s, "0b")
				s = strings.TrimPrefix(s, "0B")
			}
			n, err := strconv.ParseInt(s, base, 64)
			if err != nil {
				return def, nil
			}
			if negate {
				n = -n
			}
			return n, nil
		}
		// Base 10: try int first, fall back to float→int (Python's
		// behaviour for "32.32"|int → 32).
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			return n, nil
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return int64(f), nil
		}
		return def, nil
	}
	return def, nil
}

// filterFloat implements the `float` filter: convert a value to a
// float64.
//
// Signature: float(value, default=0.0)
//
// Falls back to `default` if parsing fails.
//
// Example:
//
//	{{ "3.14"  | float }}        →  3.14
//	{{ "bad"   | float(0.0) }}   →  0.0
func filterFloat(_ *Environment, _ *runtime.Context, value any, args []any, kwargs map[string]any) (any, error) {
	def := 0.0
	if len(args) > 0 {
		if f, ok := numAsFloat(args[0]); ok {
			def = f
		}
	} else if v, ok := kwargs["default"]; ok {
		if f, ok := numAsFloat(v); ok {
			def = f
		}
	}
	switch x := value.(type) {
	case int:
		return float64(x), nil
	case int64:
		return float64(x), nil
	case float64:
		return x, nil
	case bool:
		if x {
			return 1.0, nil
		}
		return 0.0, nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		if err != nil {
			return def, nil
		}
		return f, nil
	}
	return def, nil
}

// filterList implements the `list` filter: convert a value to a list.
//
// Signature: list(value)
//
// Lists pass through. Strings are split into single-rune strings (matching
// Python's `list("abc")` → `['a', 'b', 'c']`).
//
// Example:
//
//	{{ "abc"  | list }}  →  ['a', 'b', 'c']
func filterList(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (any, error) {
	switch x := value.(type) {
	case []any:
		return x, nil
	case string:
		out := make([]any, 0, len(x))
		for _, r := range x {
			out = append(out, string(r))
		}
		return out, nil
	}
	return nil, gjerrors.NewFilterArgumentError(fmt.Sprintf("list: cannot convert %T", value))
}

// filterFirst implements the `first` filter: first item of a sequence.
//
// Signature: first(seq)
//
// Empty sequences and non-sequences yield an Undefined value (matching
// Python's `first` returning `Undefined`, not raising).
//
// Example:
//
//	{{ [1, 2, 3]  | first }}  →  1
func filterFirst(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (any, error) {
	if x, ok := value.([]any); ok {
		if len(x) == 0 {
			return runtime.NewBase("", "first", value, nil), nil
		}
		return x[0], nil
	}
	return runtime.NewBase("", "first", value, nil), nil
}

// filterLast implements the `last` filter: last item of a sequence.
//
// Signature: last(seq)
//
// Empty sequences and non-sequences yield an Undefined value. NOTE: don't
// use this on generators or maps — only ordered sequences have a defined
// "last".
//
// Example:
//
//	{{ [1, 2, 3]  | last }}  →  3
func filterLast(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (any, error) {
	if x, ok := value.([]any); ok {
		if len(x) == 0 {
			return runtime.NewBase("", "last", value, nil), nil
		}
		return x[len(x)-1], nil
	}
	return runtime.NewBase("", "last", value, nil), nil
}

// filterSort implements the `sort` filter: sort a sequence.
//
// Signature: sort(value, reverse=False, case_sensitive=False, attribute=None)
//
// Sort is **stable**. With `attribute`, sorts by the named attribute of
// each element (dotted paths and integer indices both supported via
// [lookupDottedAttr]). String comparisons are case-insensitive by default.
//
// Example:
//
//	{{ ['B', 'a', 'C'] | sort }}                      →  ['a', 'B', 'C']
//	{{ users           | sort(attribute='age') }}     →  sorted by age
//	{{ users           | sort(reverse=true, attribute='name') }}
func filterSort(env *Environment, _ *runtime.Context, value any, args []any, kwargs map[string]any) (any, error) {
	x, ok := value.([]any)
	if !ok {
		return nil, gjerrors.NewFilterArgumentError(fmt.Sprintf("sort: cannot sort %T", value))
	}
	out := make([]any, len(x))
	copy(out, x)
	// Python's signature: sort(reverse=False, case_sensitive=False, attribute=None)
	reverse := false
	if len(args) > 0 {
		reverse, _ = args[0].(bool)
	} else if v, ok := kwargs["reverse"].(bool); ok {
		reverse = v
	}
	caseSensitive := false
	if len(args) > 1 {
		caseSensitive, _ = args[1].(bool)
	} else if v, ok := kwargs["case_sensitive"].(bool); ok {
		caseSensitive = v
	}
	var attr string
	if len(args) > 2 {
		attr, _ = args[2].(string)
	} else if v, ok := kwargs["attribute"].(string); ok {
		attr = v
	}
	keyFn := func(v any) any {
		if attr == "" {
			if s, ok := v.(string); ok && !caseSensitive {
				return strings.ToLower(s)
			}
			return v
		}
		k, err := env.GetAttr(v, attr)
		if err != nil {
			return nil
		}
		if s, ok := k.(string); ok && !caseSensitive {
			return strings.ToLower(s)
		}
		return k
	}
	sort.SliceStable(out, func(i, j int) bool {
		ki, kj := keyFn(out[i]), keyFn(out[j])
		less := compareLess(ki, kj)
		if reverse {
			return !less && !equalAny(ki, kj)
		}
		return less
	})
	return out, nil
}

// ----------------------------------------------------- additional filters

// filterSum implements the `sum` filter: sum the items of a sequence.
//
// Signature: sum(iterable, attribute=None, start=0)
//
// `attribute` (positional or keyword, dotted-path supported) extracts a
// numeric field from each item. The result is `int64` when all addends
// are integers, `float64` otherwise.
//
// Example:
//
//	{{ [1, 2, 3]     | sum }}                       →  6
//	{{ products      | sum(attribute='price') }}    →  total price
func filterSum(env *Environment, _ *runtime.Context, value any, args []any, kwargs map[string]any) (any, error) {
	items, err := iterableForSum(value)
	if err != nil {
		return nil, err
	}
	// Python signature: sum(attribute=None, start=0)
	var attr string
	if len(args) > 0 {
		if s, ok := args[0].(string); ok {
			attr = s
		}
	} else if v, ok := kwargs["attribute"].(string); ok {
		attr = v
	}
	var startInt int64
	var startFloat float64
	useFloat := false
	startVal := kwargs["start"]
	if startVal == nil && len(args) > 1 {
		startVal = args[1]
	}
	if startVal != nil {
		switch s := startVal.(type) {
		case int:
			startInt = int64(s)
		case int64:
			startInt = s
		case float64:
			startFloat = s
			useFloat = true
		}
	}
	sumInt := startInt
	sumFloat := startFloat
	for _, it := range items {
		v := it
		if attr != "" {
			rv, err := lookupDottedAttr(env, it, attr)
			if err != nil {
				return nil, err
			}
			v = rv
		}
		if f, ok := v.(float64); ok {
			useFloat = true
			sumFloat += f
		} else if n, ok := asInt(v); ok {
			sumInt += int64(n)
		}
	}
	if useFloat {
		return sumFloat + float64(sumInt), nil
	}
	return sumInt, nil
}

// iterableForSum normalises sum-able inputs to []any. Tuples are flat.
func iterableForSum(value any) ([]any, error) {
	switch x := value.(type) {
	case []any:
		return x, nil
	case runtime.Tuple:
		return []any(x), nil
	}
	return nil, gjerrors.NewFilterArgumentError(fmt.Sprintf("sum: cannot sum %T", value))
}

// lookupDottedAttr resolves an attribute path like "real.value" against
// obj, returning the final value. Numeric segments index into tuples /
// lists (matching Python's `sum` attribute notation).
func lookupDottedAttr(env *Environment, obj any, path string) (any, error) {
	parts := strings.Split(path, ".")
	cur := obj
	for _, p := range parts {
		if n, err := strconv.Atoi(p); err == nil {
			v, err := env.GetItem(cur, int64(n))
			if err != nil {
				return nil, err
			}
			cur = v
			continue
		}
		v, err := env.GetAttr(cur, p)
		if err != nil {
			return nil, err
		}
		cur = v
	}
	return cur, nil
}

// filterMin implements the `min` filter: smallest item in a sequence.
//
// Signature: min(value, case_sensitive=False, attribute=None)
//
// Empty sequences yield Undefined. With `attribute`, compares by the
// named attribute (dotted path supported). String comparisons are
// case-insensitive by default.
//
// Example:
//
//	{{ [3, 1, 2]   | min }}                       →  1
//	{{ users       | min(attribute='age') }}      →  youngest user
func filterMin(env *Environment, _ *runtime.Context, value any, args []any, kwargs map[string]any) (any, error) {
	x, ok := value.([]any)
	if !ok || len(x) == 0 {
		return runtime.NewBase("", "min", value, nil), nil
	}
	keyFn := minMaxKeyFn(env, args, kwargs)
	mi := x[0]
	miKey := keyFn(mi)
	for _, it := range x[1:] {
		k := keyFn(it)
		if compareLess(k, miKey) {
			mi = it
			miKey = k
		}
	}
	return mi, nil
}

// filterMax implements the `max` filter: largest item in a sequence.
//
// Signature: max(value, case_sensitive=False, attribute=None)
//
// Empty sequences yield Undefined. With `attribute`, compares by the
// named attribute (dotted path supported). String comparisons are
// case-insensitive by default.
//
// Example:
//
//	{{ [3, 1, 2]   | max }}                       →  3
//	{{ users       | max(attribute='score') }}    →  top-scoring user
func filterMax(env *Environment, _ *runtime.Context, value any, args []any, kwargs map[string]any) (any, error) {
	x, ok := value.([]any)
	if !ok || len(x) == 0 {
		return runtime.NewBase("", "max", value, nil), nil
	}
	keyFn := minMaxKeyFn(env, args, kwargs)
	mx := x[0]
	mxKey := keyFn(mx)
	for _, it := range x[1:] {
		k := keyFn(it)
		if compareLess(mxKey, k) {
			mx = it
			mxKey = k
		}
	}
	return mx, nil
}

// minMaxKeyFn builds the key extractor used by min / max. Python:
// min(value, case_sensitive=False, attribute=None).
func minMaxKeyFn(env *Environment, args []any, kwargs map[string]any) func(any) any {
	caseSensitive := false
	if len(args) > 0 {
		caseSensitive, _ = args[0].(bool)
	} else if v, ok := kwargs["case_sensitive"].(bool); ok {
		caseSensitive = v
	}
	var attr string
	if len(args) > 1 {
		attr, _ = args[1].(string)
	} else if v, ok := kwargs["attribute"].(string); ok {
		attr = v
	}
	return func(v any) any {
		key := v
		if attr != "" {
			rv, err := lookupDottedAttr(env, v, attr)
			if err == nil {
				key = rv
			}
		}
		if !caseSensitive {
			if s, ok := key.(string); ok {
				return strings.ToLower(s)
			}
		}
		return key
	}
}

// filterItems implements the `items` filter: yield (key, value) pairs for
// a mapping.
//
// Signature: items(d)
//
// Output is a list of two-element tuples. For Go map types, keys are
// sorted lexicographically (Go map iteration order is non-deterministic;
// sorting keeps output stable across renders). For [runtime.OrderedDict],
// insertion order is preserved (matching Python dicts since 3.7).
// Undefined input yields an empty list.
//
// Example:
//
//	{% for k, v in {'a': 1, 'b': 2} | items %}{{ k }}={{ v }} {% endfor %}
//	→  a=1 b=2
func filterItems(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (any, error) {
	switch x := value.(type) {
	case map[string]any:
		// Map iteration is non-deterministic; sort by key so output
		// stays stable across runs. Templates relying on insertion
		// order should use a runtime.OrderedDict (built by `{...}`
		// literals or the `dict()` global).
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := make([]any, 0, len(keys))
		for _, k := range keys {
			out = append(out, runtime.Tuple{k, x[k]})
		}
		return out, nil
	case map[any]any:
		stringKeys := make([]string, 0, len(x))
		valByKey := make(map[string]any, len(x))
		origKey := make(map[string]any, len(x))
		for k, v := range x {
			s := stringifyKey(k)
			stringKeys = append(stringKeys, s)
			valByKey[s] = v
			origKey[s] = k
		}
		sort.Strings(stringKeys)
		out := make([]any, 0, len(stringKeys))
		for _, k := range stringKeys {
			out = append(out, runtime.Tuple{origKey[k], valByKey[k]})
		}
		return out, nil
	case *runtime.OrderedDict:
		return x.Items(), nil
	}
	if u, ok := value.(runtime.Undefiner); ok && u.IsUndefined() {
		return []any{}, nil
	}
	return nil, gjerrors.NewFilterArgumentError(fmt.Sprintf("items: %T is not a mapping", value))
}

// filterDictsort implements the `dictsort` filter: sort a dict and emit
// (key, value) pairs.
//
// Signature: dictsort(value, case_sensitive=False, by="key", reverse=False)
//
// Use `by="value"` to sort by the dict values instead of keys.
//
// Example:
//
//	{% for k, v in {'b': 2, 'a': 1} | dictsort %}{{ k }}={{ v }} {% endfor %}
//	→  a=1 b=2
func filterDictsort(_ *Environment, _ *runtime.Context, value any, args []any, _ map[string]any) (any, error) {
	m, ok := value.(map[string]any)
	if !ok {
		return nil, gjerrors.NewFilterArgumentError("dictsort requires a mapping")
	}
	caseSensitive := false
	if len(args) > 0 {
		caseSensitive, _ = args[0].(bool)
	}
	by := "key"
	if len(args) > 1 {
		if s, ok := args[1].(string); ok {
			by = s
		}
	}
	reverse := false
	if len(args) > 2 {
		reverse, _ = args[2].(bool)
	}
	type entry struct{ K, V any }
	es := make([]entry, 0, len(m))
	for k, v := range m {
		es = append(es, entry{k, v})
	}
	sort.SliceStable(es, func(i, j int) bool {
		var a, b any
		if by == "value" {
			a, b = es[i].V, es[j].V
		} else {
			a, b = es[i].K, es[j].K
		}
		if !caseSensitive {
			if as, ok := a.(string); ok {
				a = strings.ToLower(as)
			}
			if bs, ok := b.(string); ok {
				b = strings.ToLower(bs)
			}
		}
		less := compareLess(a, b)
		if reverse {
			return !less && !equalAny(a, b)
		}
		return less
	})
	out := make([]any, len(es))
	for i, e := range es {
		out[i] = []any{e.K, e.V}
	}
	return out, nil
}

// filterUnique implements the `unique` filter: drop duplicate items
// preserving input order.
//
// Signature: unique(value, case_sensitive=False)
//
// Equality is value equality for primitives. String comparisons are
// case-insensitive by default — first-seen casing wins.
//
// Example:
//
//	{{ [1, 2, 1, 3]            | unique }}  →  [1, 2, 3]
//	{{ ['Foo', 'foo', 'BAR']   | unique }}  →  ['Foo', 'BAR']
func filterUnique(_ *Environment, _ *runtime.Context, value any, args []any, _ map[string]any) (any, error) {
	x, ok := value.([]any)
	if !ok {
		return nil, gjerrors.NewFilterArgumentError("unique requires a sequence")
	}
	caseSensitive := false
	if len(args) > 0 {
		caseSensitive, _ = args[0].(bool)
	}
	seen := map[any]bool{}
	out := []any{}
	for _, it := range x {
		key := it
		if !caseSensitive {
			if s, ok := it.(string); ok {
				key = strings.ToLower(s)
			}
		}
		if !seen[key] {
			seen[key] = true
			out = append(out, it)
		}
	}
	return out, nil
}

// filterBatch implements the `batch` filter: group items into rows of N.
//
// Signature: batch(value, linecount, fill_with=None)
//
// The last row is padded with `fill_with` to reach `linecount` if a
// fill value is provided; otherwise it's left short.
//
// Example:
//
//	{{ [1, 2, 3, 4, 5]      | batch(2) }}     →  [[1, 2], [3, 4], [5]]
//	{{ [1, 2, 3, 4, 5]      | batch(2, 0) }}  →  [[1, 2], [3, 4], [5, 0]]
func filterBatch(_ *Environment, _ *runtime.Context, value any, args []any, _ map[string]any) (any, error) {
	x, ok := value.([]any)
	if !ok {
		return nil, gjerrors.NewFilterArgumentError("batch requires a sequence")
	}
	if len(args) < 1 {
		return nil, gjerrors.NewFilterArgumentError("batch requires linecount")
	}
	n, ok := asInt(args[0])
	if !ok || n <= 0 {
		return nil, gjerrors.NewFilterArgumentError("batch linecount must be positive")
	}
	var fill any
	if len(args) > 1 {
		fill = args[1]
	}
	var out []any
	for i := 0; i < len(x); i += n {
		end := i + n
		if end > len(x) {
			end = len(x)
		}
		row := append([]any(nil), x[i:end]...)
		if fill != nil {
			for len(row) < n {
				row = append(row, fill)
			}
		}
		out = append(out, row)
	}
	return out, nil
}

// filterSlice implements the `slice` filter: distribute items across N
// columns of nearly-equal length.
//
// Signature: slice(value, slices, fill_with=None)
//
// Useful for newspaper-style layouts. Earlier slices get one extra item
// when the count doesn't divide evenly. With `fill_with`, short slices
// are padded so all have equal length.
//
// Example:
//
//	{{ [1, 2, 3, 4, 5]   | slice(3) }}  →  [[1, 2], [3, 4], [5]]
func filterSlice(_ *Environment, _ *runtime.Context, value any, args []any, _ map[string]any) (any, error) {
	x, ok := value.([]any)
	if !ok {
		return nil, gjerrors.NewFilterArgumentError("slice requires a sequence")
	}
	if len(args) < 1 {
		return nil, gjerrors.NewFilterArgumentError("slice requires count")
	}
	n, ok := asInt(args[0])
	if !ok || n <= 0 {
		return nil, gjerrors.NewFilterArgumentError("slice count must be positive")
	}
	var fill any
	if len(args) > 1 {
		fill = args[1]
	}
	total := len(x)
	per := total / n
	extra := total % n
	out := make([]any, 0, n)
	pos := 0
	for i := 0; i < n; i++ {
		size := per
		if i < extra {
			size++
		}
		row := append([]any(nil), x[pos:pos+size]...)
		pos += size
		if fill != nil && i >= extra && extra != 0 {
			row = append(row, fill)
		}
		out = append(out, row)
	}
	return out, nil
}

// filterTruncate implements the `truncate` filter: shorten a string and
// append an ellipsis.
//
// Signature: truncate(s, length=255, killwords=False, end='...', leeway=5)
//
// Strings shorter than `length + leeway` are returned unchanged.
// `killwords=False` (the default) breaks at the last whitespace before
// the cut so words aren't split mid-word.
//
// Example:
//
//	{{ "this is a long sentence" | truncate(12) }}  →  this is a...
func filterTruncate(_ *Environment, _ *runtime.Context, value any, args []any, kwargs map[string]any) (any, error) {
	s := escape.SoftStr(value)
	length := 255
	killWords := false
	end := "..."
	leeway := 5
	if len(args) > 0 {
		if n, ok := asInt(args[0]); ok {
			length = n
		}
	} else if v, ok := kwargs["length"]; ok {
		if n, ok := asInt(v); ok {
			length = n
		}
	}
	if len(args) > 1 {
		killWords, _ = args[1].(bool)
	} else if v, ok := kwargs["killwords"].(bool); ok {
		killWords = v
	}
	if len(args) > 2 {
		end = escape.SoftStr(args[2])
	} else if v, ok := kwargs["end"]; ok {
		end = escape.SoftStr(v)
	}
	if len(args) > 3 {
		if n, ok := asInt(args[3]); ok {
			leeway = n
		}
	} else if v, ok := kwargs["leeway"]; ok {
		if n, ok := asInt(v); ok {
			leeway = n
		}
	}
	if len(s) <= length+leeway {
		return s, nil
	}
	if killWords {
		return s[:length-len(end)] + end, nil
	}
	cut := length - len(end)
	if cut < 0 {
		cut = 0
	}
	idx := strings.LastIndex(s[:cut], " ")
	if idx > 0 {
		return s[:idx] + end, nil
	}
	return s[:cut] + end, nil
}

// filterWordcount implements the `wordcount` filter: count "words" in a
// string.
//
// Signature: wordcount(s)
//
// A word is a maximal run of `[A-Za-z0-9_]`. Anything else is a separator.
//
// Example:
//
//	{{ "hello world foo"  | wordcount }}  →  3
func filterWordcount(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (any, error) {
	s := escape.SoftStr(value)
	count := 0
	inWord := false
	for _, r := range s {
		isWord := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
		if isWord && !inWord {
			count++
		}
		inWord = isWord
	}
	return count, nil
}

// filterIndent implements the `indent` filter: indent each line by N
// spaces.
//
// Signature: indent(s, width=4, first=False, blank=False)
//
// `first=False` skips indenting the first line (so the filter chains with
// templates that already have content on the line). `blank=False` skips
// blank lines.
//
// Example:
//
//	{{ "a\nb\nc" | indent(2, true) }}  →  "  a\n  b\n  c"
func filterIndent(_ *Environment, _ *runtime.Context, value any, args []any, _ map[string]any) (any, error) {
	s := escape.SoftStr(value)
	width := 4
	first := false
	blank := false
	if len(args) > 0 {
		if n, ok := asInt(args[0]); ok {
			width = n
		}
	}
	if len(args) > 1 {
		first, _ = args[1].(bool)
	}
	if len(args) > 2 {
		blank, _ = args[2].(bool)
	}
	pad := strings.Repeat(" ", width)
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if i == 0 && !first {
			continue
		}
		if line == "" && !blank {
			continue
		}
		lines[i] = pad + line
	}
	return strings.Join(lines, "\n"), nil
}

// filterStripTags implements the `striptags` filter: drop HTML tags and
// HTML comments, then collapse whitespace.
//
// Signature: striptags(s)
//
// `<!-- ... -->` spans are removed entirely (including content). Whitespace
// runs are collapsed to a single space.
//
// Example:
//
//	{{ "<p>hi <b>there</b></p>" | striptags }}  →  hi there
func filterStripTags(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (any, error) {
	s := escape.SoftStr(value)
	// Strip HTML comments first — Python's striptags drops the entire
	// `<!-- ... -->` span including its contents, not just the angles.
	for {
		i := strings.Index(s, "<!--")
		if i < 0 {
			break
		}
		j := strings.Index(s[i:], "-->")
		if j < 0 {
			s = s[:i]
			break
		}
		s = s[:i] + s[i+j+3:]
	}
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	// Collapse whitespace runs.
	out := strings.Fields(b.String())
	return strings.Join(out, " "), nil
}

// filterCenter implements the `center` filter: pad a string with spaces
// to centre it within a given width.
//
// Signature: center(s, width=80)
//
// Strings that already exceed `width` are returned unchanged.
//
// Example:
//
//	{{ "hi" | center(6) }}  →  "  hi  "
func filterCenter(_ *Environment, _ *runtime.Context, value any, args []any, _ map[string]any) (any, error) {
	s := escape.SoftStr(value)
	width := 80
	if len(args) > 0 {
		if n, ok := asInt(args[0]); ok {
			width = n
		}
	}
	if len(s) >= width {
		return s, nil
	}
	pad := width - len(s)
	left := pad / 2
	right := pad - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right), nil
}

// filterRound implements the `round` filter: round a number to N decimal
// places.
//
// Signature: round(value, precision=0, method='common')
//
// Methods: 'common' (round-half-to-even), 'ceil', 'floor'. Negative
// precision rounds to powers of ten (e.g. precision=-2 rounds to the
// nearest 100).
//
// Example:
//
//	{{ 3.14159 | round(2) }}             →  3.14
//	{{ 3.5     | round(0, 'ceil') }}     →  4
func filterRound(_ *Environment, _ *runtime.Context, value any, args []any, kwargs map[string]any) (any, error) {
	f, ok := numAsFloat(value)
	if !ok {
		return nil, gjerrors.NewFilterArgumentError("round requires a number")
	}
	// Python signature: round(precision=0, method='common')
	prec := 0
	if len(args) > 0 {
		if n, ok := asInt(args[0]); ok {
			prec = n
		}
	} else if v, ok := kwargs["precision"]; ok {
		if n, ok := asInt(v); ok {
			prec = n
		}
	}
	method := "common"
	if len(args) > 1 {
		if s, ok := args[1].(string); ok {
			method = s
		}
	} else if v, ok := kwargs["method"].(string); ok {
		method = v
	}
	mult := 1.0
	for i := 0; i < prec; i++ {
		mult *= 10
	}
	for i := 0; i > prec; i-- {
		mult /= 10
	}
	switch method {
	case "ceil":
		return mathCeil(f*mult) / mult, nil
	case "floor":
		return mathFloor(f*mult) / mult, nil
	default:
		return mathRound(f*mult) / mult, nil
	}
}

// filterFilesizeformat implements the `filesizeformat` filter: render a
// byte count as a human-friendly size.
//
// Signature: filesizeformat(value, binary=False)
//
// Default uses decimal (kB, MB, GB) with a 1000-base; `binary=True` uses
// binary (KiB, MiB, GiB) with a 1024-base.
//
// Example:
//
//	{{ 1500       | filesizeformat }}            →  1.5 kB
//	{{ 1500       | filesizeformat(true) }}      →  1.5 KiB
func filterFilesizeformat(_ *Environment, _ *runtime.Context, value any, args []any, _ map[string]any) (any, error) {
	f, ok := numAsFloat(value)
	if !ok {
		return nil, gjerrors.NewFilterArgumentError("filesizeformat requires a number")
	}
	binary := false
	if len(args) > 0 {
		binary, _ = args[0].(bool)
	}
	base := 1000.0
	suffix := []string{"Bytes", "kB", "MB", "GB", "TB", "PB", "EB", "ZB", "YB"}
	if binary {
		base = 1024.0
		suffix = []string{"Bytes", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB", "ZiB", "YiB"}
	}
	if f < base {
		if f == 1 {
			return "1 Byte", nil
		}
		return fmt.Sprintf("%d Bytes", int(f)), nil
	}
	idx := 0
	for f >= base && idx < len(suffix)-1 {
		f /= base
		idx++
	}
	return fmt.Sprintf("%.1f %s", f, suffix[idx]), nil
}

// filterUrlencode implements the `urlencode` filter: percent-encode a
// string or build a query string from a mapping.
//
// Signature: urlencode(value)
//
// Strings: percent-encode for the path-segment context (preserves `/`).
// Mappings: produce `k1=v1&k2=v2&...` with both keys and values
// percent-encoded for the query-string context. Map iteration is sorted
// by key for stability; [runtime.OrderedDict] preserves insertion order.
//
// Example:
//
//	{{ "hello world"          | urlencode }}  →  hello%20world
//	{{ {'q': 'go', 'page': 2} | urlencode }}  →  page=2&q=go
func filterUrlencode(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (any, error) {
	switch x := value.(type) {
	case string:
		return urlEncodePath(x), nil
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var parts []string
		for _, k := range keys {
			parts = append(parts, urlEncodeQS(k)+"="+urlEncodeQS(escape.SoftStr(x[k])))
		}
		return strings.Join(parts, "&"), nil
	case *runtime.OrderedDict:
		// Preserve insertion order — Jinja2's urlencode does too.
		var parts []string
		for _, k := range x.Keys() {
			ks, ok := k.(string)
			if !ok {
				ks = stringifyKey(k)
			}
			v, _ := x.Get(k)
			parts = append(parts, urlEncodeQS(ks)+"="+urlEncodeQS(escape.SoftStr(v)))
		}
		return strings.Join(parts, "&"), nil
	}
	return urlEncodePath(escape.SoftStr(value)), nil
}

// filterTojson implements the `tojson` filter: serialise a value as JSON
// safe for embedding in `<script>` blocks.
//
// Signature: tojson(value, indent=None)
//
// `&`, `<`, `>`, `'` are unicode-escaped (`<` etc.) so the result is
// safe inside HTML script context. The result is returned as Markup.
//
// Example:
//
//	<script>const data = {{ payload | tojson }};</script>
func filterTojson(_ *Environment, _ *runtime.Context, value any, args []any, _ map[string]any) (any, error) {
	indent := 0
	if len(args) > 0 {
		if n, ok := asInt(args[0]); ok {
			indent = n
		}
	}
	b, err := jsonMarshal(value, indent)
	if err != nil {
		return nil, err
	}
	// HTML-safe escape for embedding in <script>: replace &, <, >, ' with unicode escapes.
	out := strings.NewReplacer(
		"<", `<`,
		">", `>`,
		"&", `&`,
		"'", `'`,
	).Replace(string(b))
	return escape.Markup(out), nil
}

// filterAttr implements the `attr` filter: look up an attribute by name.
//
// Signature: attr(value, name)
//
// Equivalent to `value.name` in template syntax, but returns the result
// of attribute access through the engine's sandbox-aware [Environment.GetAttr].
// Useful when the attribute name is dynamic.
//
// Example:
//
//	{{ obj | attr('field_name') }}     ≡  {{ obj.field_name }}
func filterAttr(env *Environment, _ *runtime.Context, value any, args []any, _ map[string]any) (any, error) {
	if len(args) < 1 {
		return nil, gjerrors.NewFilterArgumentError("attr requires a name")
	}
	name, ok := args[0].(string)
	if !ok {
		return nil, gjerrors.NewFilterArgumentError("attr name must be a string")
	}
	return env.GetAttr(value, name)
}

// filterFormat implements the `format` filter: Python printf-style
// formatting.
//
// Signature: format(s, *args)
//
// Supports `%s`, `%d`, `%f`, `%x`, `%o`, `%b`, `%r` (Python repr), `%%`,
// and width/precision flags (`%05d`, `%.2f`). `%s` always stringifies
// via [escape.SoftStr] to match Python's `str()` semantics.
//
// Example:
//
//	{{ '%s scored %d' | format('Alice', 95) }}   →  Alice scored 95
func filterFormat(_ *Environment, _ *runtime.Context, value any, args []any, _ map[string]any) (any, error) {
	s := escape.SoftStr(value)
	return pythonFormat(s, args), nil
}

// pythonFormat applies a Python-style printf format string to args. The
// supported conversions are the cross-language overlap with Go's fmt
// plus `%r` (Python's repr) and `%s` always stringifying via SoftStr to
// match Python's str(). Anything else passes through to fmt.Sprintf.
func pythonFormat(format string, args []any) string {
	var b strings.Builder
	argIdx := 0
	for i := 0; i < len(format); i++ {
		c := format[i]
		if c != '%' {
			b.WriteByte(c)
			continue
		}
		// Find the conversion: scan flags + width + precision + verb.
		end := i + 1
		for end < len(format) {
			v := format[end]
			if (v >= '0' && v <= '9') || v == '-' || v == '+' || v == ' ' || v == '#' || v == '.' {
				end++
				continue
			}
			break
		}
		if end >= len(format) {
			b.WriteString(format[i:])
			return b.String()
		}
		spec := format[i:end]
		verb := format[end]
		i = end
		switch verb {
		case '%':
			b.WriteByte('%')
		case 's':
			if argIdx >= len(args) {
				b.WriteString(spec + string(verb))
				continue
			}
			b.WriteString(fmt.Sprintf(spec+"s", escape.SoftStr(args[argIdx])))
			argIdx++
		case 'r':
			if argIdx >= len(args) {
				b.WriteString(spec + string(verb))
				continue
			}
			// Python's %r: repr() of the value. We have escape.pyRepr
			// only inside the escape package; rebuild a minimal repr
			// here for the common types.
			b.WriteString(pythonRepr(args[argIdx]))
			argIdx++
		default:
			if argIdx >= len(args) {
				b.WriteString(spec + string(verb))
				continue
			}
			b.WriteString(fmt.Sprintf(spec+string(verb), args[argIdx]))
			argIdx++
		}
	}
	return b.String()
}

// pythonRepr returns Python's repr() of v for the limited set of types
// templates produce. Strings are single-quoted; other primitives use
// their str() form (which matches Python repr for those types).
func pythonRepr(v any) string {
	switch x := v.(type) {
	case nil:
		return "None"
	case bool:
		if x {
			return "True"
		}
		return "False"
	case string:
		return pyStringRepr(x)
	}
	return escape.SoftStr(v)
}

// pyStringRepr is Python repr for a string: single-quoted, with `\'`
// escape only when the string contains no double quotes.
func pyStringRepr(s string) string {
	hasSingle := strings.ContainsRune(s, '\'')
	hasDouble := strings.ContainsRune(s, '"')
	quote := byte('\'')
	if hasSingle && !hasDouble {
		quote = '"'
	}
	var b strings.Builder
	b.WriteByte(quote)
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case rune(quote):
			b.WriteByte('\\')
			b.WriteRune(r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte(quote)
	return b.String()
}

// ------------------------------------------------------ higher-order filters

// filterMap implements the `map` filter: apply a filter to each item, or
// extract an attribute from each item.
//
// Signature: map(value, *args, **kwargs)
//
// Two forms:
//   - `map(filter_name, *args)` applies the named filter to each item.
//   - `map(attribute='name', default=...)` extracts the named attribute
//     (dotted path supported) from each item, falling back to `default`
//     when the attribute is undefined.
//
// `None` and Undefined inputs yield an empty list (matching Python).
//
// Example:
//
//	{{ ['a', 'b']  | map('upper')         | list }}  →  ['A', 'B']
//	{{ users       | map(attribute='name') | list }}  →  list of names
func filterMap(env *Environment, ctx *runtime.Context, value any, args []any, kwargs map[string]any) (any, error) {
	// Python: map(seq, ...) treats None / missing as empty.
	x, err := mapSequenceArg(value)
	if err != nil {
		return nil, err
	}
	if attr, ok := kwargs["attribute"].(string); ok {
		def, hasDefault := kwargs["default"]
		out := make([]any, 0, len(x))
		for _, it := range x {
			v, err := lookupDottedAttr(env, it, attr)
			if err != nil {
				return nil, err
			}
			if u, ok := v.(runtime.Undefiner); ok && u.IsUndefined() {
				if hasDefault {
					v = def
				}
			}
			out = append(out, v)
		}
		return out, nil
	}
	if len(args) >= 1 {
		fname, ok := args[0].(string)
		if !ok {
			return nil, gjerrors.NewFilterArgumentError("map: first arg must be a filter name")
		}
		rest := args[1:]
		out := make([]any, 0, len(x))
		for _, it := range x {
			v, err := env.CallFilter(fname, ctx, it, rest, nil)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, nil
	}
	return nil, gjerrors.NewFilterArgumentError("map requires a filter name or attribute=")
}

// mapSequenceArg accepts the lenient sequence types Jinja2's map filter
// allows: lists, tuples, None (yields empty), strings (iterated as
// chars), and Undefined (empty). Returns []any.
func mapSequenceArg(value any) ([]any, error) {
	switch x := value.(type) {
	case nil:
		return nil, nil
	case []any:
		return x, nil
	case runtime.Tuple:
		return []any(x), nil
	case []string:
		out := make([]any, len(x))
		for i, s := range x {
			out[i] = s
		}
		return out, nil
	case string:
		out := make([]any, 0, len(x))
		for _, r := range x {
			out = append(out, string(r))
		}
		return out, nil
	}
	if u, ok := value.(runtime.Undefiner); ok && u.IsUndefined() {
		return nil, nil
	}
	return nil, gjerrors.NewFilterArgumentError(fmt.Sprintf("map/select/reject: %T is not iterable", value))
}

// filterSelect implements the `select` filter: keep items where a test
// passes.
//
// Signature: select(value, test_name=None, *test_args)
//
// With no arguments, keeps items that are truthy. With a test name,
// applies the named test to each item.
//
// Example:
//
//	{{ [1, 2, 3, 4]  | select('odd') | list }}      →  [1, 3]
//	{{ [0, 1, 2, '', 'x'] | select | list }}         →  [1, 2, 'x']
func filterSelect(env *Environment, ctx *runtime.Context, value any, args []any, _ map[string]any) (any, error) {
	return selectReject(env, ctx, value, args, true, "")
}

// filterReject implements the `reject` filter: drop items where a test
// passes (the complement of [filterSelect]).
//
// Signature: reject(value, test_name=None, *test_args)
//
// Example:
//
//	{{ [1, 2, 3, 4]  | reject('odd') | list }}  →  [2, 4]
func filterReject(env *Environment, ctx *runtime.Context, value any, args []any, _ map[string]any) (any, error) {
	return selectReject(env, ctx, value, args, false, "")
}

// filterSelectAttr implements the `selectattr` filter: keep items whose
// named attribute passes a test.
//
// Signature: selectattr(value, attribute, test_name=None, *test_args)
//
// With no test, keeps items where the attribute is truthy.
//
// Example:
//
//	{{ users | selectattr('admin') | list }}                →  admins
//	{{ users | selectattr('age', 'gt', 18) | list }}        →  adults
func filterSelectAttr(env *Environment, ctx *runtime.Context, value any, args []any, _ map[string]any) (any, error) {
	if len(args) < 1 {
		return nil, gjerrors.NewFilterArgumentError("selectattr requires an attribute name")
	}
	attr, ok := args[0].(string)
	if !ok {
		return nil, gjerrors.NewFilterArgumentError("selectattr: attribute must be a string")
	}
	return selectReject(env, ctx, value, args[1:], true, attr)
}

// filterRejectAttr implements the `rejectattr` filter: drop items whose
// named attribute passes a test (complement of [filterSelectAttr]).
//
// Signature: rejectattr(value, attribute, test_name=None, *test_args)
//
// Example:
//
//	{{ users | rejectattr('banned') | list }}  →  non-banned users
func filterRejectAttr(env *Environment, ctx *runtime.Context, value any, args []any, _ map[string]any) (any, error) {
	if len(args) < 1 {
		return nil, gjerrors.NewFilterArgumentError("rejectattr requires an attribute name")
	}
	attr, ok := args[0].(string)
	if !ok {
		return nil, gjerrors.NewFilterArgumentError("rejectattr: attribute must be a string")
	}
	return selectReject(env, ctx, value, args[1:], false, attr)
}

// selectReject is the shared engine for select/reject/selectattr/rejectattr.
// keep=true means "include items where the test passes"; attr (if set)
// means "test the named attribute of each item, not the item itself".
func selectReject(env *Environment, ctx *runtime.Context, value any, args []any, keep bool, attr string) (any, error) {
	x, err := mapSequenceArg(value)
	if err != nil {
		return nil, err
	}
	out := make([]any, 0, len(x))
	for _, it := range x {
		probe := it
		if attr != "" {
			v, err := env.GetAttr(it, attr)
			if err != nil {
				return nil, err
			}
			probe = v
		}
		var pass bool
		if len(args) == 0 {
			pass = truthy(probe)
		} else {
			tname, ok := args[0].(string)
			if !ok {
				return nil, gjerrors.NewFilterArgumentError("select/reject: test name must be a string")
			}
			rest := args[1:]
			res, err := env.CallTest(tname, ctx, probe, rest, nil)
			if err != nil {
				return nil, err
			}
			pass, _ = res.(bool)
		}
		if pass == keep {
			out = append(out, it)
		}
	}
	return out, nil
}

// =========================================================== Test funcs

// testDefined implements the `defined` test: true if the value is not
// Undefined.
//
// Signature: x is defined
//
// Example:
//
//	{% if user is defined %}…{% endif %}
func testDefined(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	if u, ok := value.(runtime.Undefiner); ok && u.IsUndefined() {
		return false, nil
	}
	return true, nil
}

// testUndefined implements the `undefined` test: true if the value is
// Undefined (the complement of [testDefined]).
//
// Signature: x is undefined
func testUndefined(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	if u, ok := value.(runtime.Undefiner); ok && u.IsUndefined() {
		return true, nil
	}
	return false, nil
}

// testNone implements the `none` test: true if the value is `None`
// (Go: `nil`).
//
// Signature: x is none
func testNone(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	return value == nil, nil
}

// testFalse implements the `false` test: true iff the value is exactly
// `False` (the boolean, not just falsy).
//
// Signature: x is false
func testFalse(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	return value == false, nil
}

// testTrue implements the `true` test: true iff the value is exactly
// `True` (the boolean, not just truthy).
//
// Signature: x is true
func testTrue(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	return value == true, nil
}

// testBoolean implements the `boolean` test: true if the value is a
// `bool` (either True or False).
//
// Signature: x is boolean
func testBoolean(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	_, ok := value.(bool)
	return ok, nil
}

// testString implements the `string` test: true if the value is a string
// or [escape.Markup].
//
// Signature: x is string
func testString(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	_, ok := value.(string)
	if ok {
		return true, nil
	}
	_, ok = value.(escape.Markup)
	return ok, nil
}

// testNumber implements the `number` test: true if the value is any
// numeric type (int, float, …).
//
// Signature: x is number
func testNumber(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	switch value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return true, nil
	}
	return false, nil
}

// testInteger implements the `integer` test: true if the value is any
// integer type (no floats).
//
// Signature: x is integer
func testInteger(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	switch value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return true, nil
	}
	return false, nil
}

// testFloat implements the `float` test: true if the value is a float32
// or float64.
//
// Signature: x is float
func testFloat(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	switch value.(type) {
	case float32, float64:
		return true, nil
	}
	return false, nil
}

// testOdd implements the `odd` test: true if the value is an integer and
// not divisible by 2.
//
// Signature: x is odd
//
// Example:
//
//	{% for n in numbers if n is odd %}{{ n }} {% endfor %}
func testOdd(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	if n, ok := asInt(value); ok {
		return n%2 != 0, nil
	}
	return false, nil
}

// testEven implements the `even` test: true if the value is an integer
// and divisible by 2.
//
// Signature: x is even
func testEven(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	if n, ok := asInt(value); ok {
		return n%2 == 0, nil
	}
	return false, nil
}

// testDivisibleBy implements the `divisibleby` test: true if value is
// divisible by the given divisor.
//
// Signature: x is divisibleby(n)
//
// Example:
//
//	{% if n is divisibleby(3) %}fizz{% endif %}
func testDivisibleBy(_ *Environment, _ *runtime.Context, value any, args []any, _ map[string]any) (bool, error) {
	if len(args) < 1 {
		return false, gjerrors.NewTemplateRuntimeError("divisibleby requires divisor")
	}
	n, ok := asInt(value)
	if !ok {
		return false, nil
	}
	d, ok := asInt(args[0])
	if !ok || d == 0 {
		return false, nil
	}
	return n%d == 0, nil
}

// testSequence implements the `sequence` test: true if the value can be
// length-queried and index-accessed.
//
// Signature: x is sequence
//
// Includes lists, tuples, strings, and mappings (mirrors Python's "anything
// with `__len__` and `__getitem__`").
func testSequence(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	switch value.(type) {
	case []any, string, runtime.Tuple, map[string]any, map[any]any, *runtime.OrderedDict:
		return true, nil
	}
	return false, nil
}

// testMapping implements the `mapping` test: true if the value is a
// dict-like.
//
// Signature: x is mapping
func testMapping(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	switch value.(type) {
	case map[string]any, map[any]any, *runtime.OrderedDict:
		return true, nil
	}
	return false, nil
}

// testIterable implements the `iterable` test: true if a `for` loop can
// iterate over the value.
//
// Signature: x is iterable
//
// Strings count as iterable (over runes). Tuples are not in the
// iterable predicate set in Jinja2; we match that.
func testIterable(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	switch value.(type) {
	case []any, string, map[string]any, map[any]any, *runtime.OrderedDict:
		return true, nil
	}
	return false, nil
}

// testLowerStr implements the `lower` test: true if the value is a string
// containing at least one letter and equal to its lowercase form.
//
// Signature: x is lower
func testLowerStr(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	if s, ok := value.(string); ok {
		return s == strings.ToLower(s) && containsLetter(s), nil
	}
	return false, nil
}

// testUpperStr implements the `upper` test: true if the value is a string
// containing at least one letter and equal to its uppercase form.
//
// Signature: x is upper
func testUpperStr(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	if s, ok := value.(string); ok {
		return s == strings.ToUpper(s) && containsLetter(s), nil
	}
	return false, nil
}

func containsLetter(s string) bool {
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			return true
		}
	}
	return false
}

// testEq implements the `eq` / `equalto` / `==` test: equality comparison.
//
// Signature: x is eq(other)  ·  x is equalto(other)  ·  x is ==(other)
//
// Example:
//
//	{% if name is eq("Alice") %}…{% endif %}
func testEq(_ *Environment, _ *runtime.Context, value any, args []any, _ map[string]any) (bool, error) {
	if len(args) < 1 {
		return false, nil
	}
	return equalAny(value, args[0]), nil
}

// testNe implements the `ne` / `!=` test: inequality comparison.
//
// Signature: x is ne(other)  ·  x is !=(other)
func testNe(_ *Environment, _ *runtime.Context, value any, args []any, _ map[string]any) (bool, error) {
	if len(args) < 1 {
		return false, nil
	}
	return !equalAny(value, args[0]), nil
}

// testLt implements the `lt` / `lessthan` / `<` test: strict-less comparison.
//
// Signature: x is lt(other)
func testLt(_ *Environment, _ *runtime.Context, value any, args []any, _ map[string]any) (bool, error) {
	if len(args) < 1 {
		return false, nil
	}
	return compareLess(value, args[0]), nil
}

// testLtEq implements the `le` / `<=` test: less-or-equal comparison.
//
// Signature: x is le(other)
func testLtEq(_ *Environment, _ *runtime.Context, value any, args []any, _ map[string]any) (bool, error) {
	if len(args) < 1 {
		return false, nil
	}
	return compareLess(value, args[0]) || equalAny(value, args[0]), nil
}

// testGt implements the `gt` / `greaterthan` / `>` test: strict-greater
// comparison.
//
// Signature: x is gt(other)
func testGt(_ *Environment, _ *runtime.Context, value any, args []any, _ map[string]any) (bool, error) {
	if len(args) < 1 {
		return false, nil
	}
	return !compareLess(value, args[0]) && !equalAny(value, args[0]), nil
}

// testGtEq implements the `ge` / `>=` test: greater-or-equal comparison.
//
// Signature: x is ge(other)
func testGtEq(_ *Environment, _ *runtime.Context, value any, args []any, _ map[string]any) (bool, error) {
	if len(args) < 1 {
		return false, nil
	}
	return !compareLess(value, args[0]), nil
}

// testIn implements the `in` test: membership check.
//
// Signature: x is in(collection)
//
// For lists/tuples: element-wise equality. For strings: substring match.
// For dicts: key existence (value must be a string).
//
// Example:
//
//	{% if "admin" is in(roles) %}…{% endif %}
func testIn(_ *Environment, _ *runtime.Context, value any, args []any, _ map[string]any) (bool, error) {
	if len(args) < 1 {
		return false, nil
	}
	switch col := args[0].(type) {
	case []any:
		for _, it := range col {
			if equalAny(value, it) {
				return true, nil
			}
		}
		return false, nil
	case string:
		s, ok := value.(string)
		if !ok {
			return false, nil
		}
		return strings.Contains(col, s), nil
	case map[string]any:
		s, ok := value.(string)
		if !ok {
			return false, nil
		}
		_, ok = col[s]
		return ok, nil
	}
	return false, nil
}

// testSameAs implements the `sameas` test: identity comparison (`is`).
//
// Signature: x is sameas(other)
//
// Compares by Go's `==` on `any` — for pointers and interface-typed
// values this is identity; for primitives it's value equality.
func testSameAs(_ *Environment, _ *runtime.Context, value any, args []any, _ map[string]any) (bool, error) {
	if len(args) < 1 {
		return false, nil
	}
	return value == args[0], nil
}

// testCallable implements the `callable` test: true when the value is
// invokable (a Go function, a macro, etc.).
//
// Signature: x is callable
//
// Mirrors Python's `callable()` builtin. Uses reflection.
func testCallable(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	if value == nil {
		return false, nil
	}
	rv := reflect.ValueOf(value)
	return rv.Kind() == reflect.Func, nil
}

// testEscaped implements the `escaped` test: true when the value is
// already [escape.Markup] (so autoescape will pass it through verbatim).
//
// Signature: x is escaped
func testEscaped(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	_, ok := value.(escape.Markup)
	return ok, nil
}

// testIsFilter implements the `filter` test: true when the value is the
// name of a registered filter on the environment.
//
// Signature: name is filter
func testIsFilter(env *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	name, ok := value.(string)
	if !ok {
		return false, nil
	}
	_, exists := env.filters[name]
	return exists, nil
}

// testIsTest implements the `test` test: true when the value is the name
// of a registered test on the environment.
//
// Signature: name is test
func testIsTest(env *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	name, ok := value.(string)
	if !ok {
		return false, nil
	}
	_, exists := env.tests[name]
	return exists, nil
}

// ----------------- comparison helpers shared by filters and tests -----------------

func equalAny(a, b any) bool {
	if af, ok := numAsFloat(a); ok {
		if bf, ok := numAsFloat(b); ok {
			return af == bf
		}
	}
	return a == b
}

func compareLess(a, b any) bool {
	if af, ok := numAsFloat(a); ok {
		if bf, ok := numAsFloat(b); ok {
			return af < bf
		}
	}
	if as, ok := a.(string); ok {
		if bs, ok := b.(string); ok {
			return as < bs
		}
	}
	// Tuple / list comparison — Python's sort uses lexicographic order
	// element-by-element, the way `(key, value)` pairs from
	// `dict.items()` get sorted by key, then by value. We treat
	// `[]any` and `runtime.Tuple` interchangeably so a list of either
	// can be sorted.
	al, aOK := asAnySlice(a)
	bl, bOK := asAnySlice(b)
	if aOK && bOK {
		n := len(al)
		if len(bl) < n {
			n = len(bl)
		}
		for i := 0; i < n; i++ {
			if compareLess(al[i], bl[i]) {
				return true
			}
			if compareLess(bl[i], al[i]) {
				return false
			}
		}
		return len(al) < len(bl)
	}
	return false
}

// asAnySlice unwraps runtime.Tuple or []any to a flat []any view.
func asAnySlice(v any) ([]any, bool) {
	switch x := v.(type) {
	case []any:
		return x, true
	case runtime.Tuple:
		return []any(x), true
	}
	return nil, false
}

func numAsFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case float64:
		return x, true
	}
	return 0, false
}

func truthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case int:
		return x != 0
	case int64:
		return x != 0
	case float64:
		return x != 0
	case string:
		return len(x) != 0
	case []any:
		return len(x) != 0
	}
	if u, ok := v.(runtime.Undefiner); ok && u.IsUndefined() {
		return false
	}
	return true
}
