package environment

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/jryberg/gojinja/internal/pyunicode"
	gjerrors "github.com/jryberg/gojinja/pkg/errors"
	"github.com/jryberg/gojinja/pkg/escape"
	"github.com/jryberg/gojinja/pkg/eval"
	"github.com/jryberg/gojinja/pkg/runtime"
)

// registerBuiltins seeds an Environment with the standard filters,
// tests, and globals.
func registerBuiltins(e *Environment) {
	// ------------------------------------------------------- globals
	// The globals print as Python's do: the sandbox's range is
	// safe_range (WithUnsafe makes it the range class), the others
	// Jinja2's classes and functions.
	e.globals["range"] = runtime.NewFunction("safe_range", globalRange(e))
	e.safeRange = e.globals["range"].(*runtime.Callable)
	e.globals["dict"] = dictClass(globalDict)
	e.globals["namespace"] = runtime.NewClass("jinja2.utils.Namespace", orderedKwargsFunc(namespaceCtor))
	e.globals["cycler"] = runtime.NewClass("jinja2.utils.Cycler", globalCycler)
	e.globals["joiner"] = runtime.NewClass("jinja2.utils.Joiner", globalJoiner)
	e.globals["lipsum"] = runtime.NewFunction("generate_lorem_ipsum", globalLipsum)

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

	for name, f := range e.filters {
		f.sig = builtinFilterSignatures[name]
		e.filters[name] = f
	}
	for name, t := range e.tests {
		t.sig = builtinTestSignatures[name]
		e.tests[name] = t
	}
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
func namespaceCtor(args []any, kwargs *runtime.OrderedDict) (any, error) {
	// Namespace(*args, **kwargs) seeds its attributes like dict().
	seed, err := globalDict(args, kwargs)
	if err != nil {
		return nil, err
	}
	ns := runtime.NewNamespace(nil)
	for _, it := range seed.(*runtime.OrderedDict).Items() {
		kv := it.(runtime.Tuple)
		name, ok := kv[0].(string)
		if !ok {
			return nil, gjerrors.NewTemplateRuntimeError("keywords must be strings")
		}
		ns.Set(name, kv[1])
	}
	return ns, nil
}

// =========================================================== Filter funcs

// filterUpper implements the `upper` filter: convert a value to uppercase.
//
// Signature: upper(s)
//
// Stringifies via [escape.SoftStr] then applies Python's str.upper (full
// Unicode case mapping, so "ß" becomes "SS").
//
// Example:
//
//	{{ "hello" | upper }}  →  HELLO
func filterUpper(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (any, error) {
	return keepMarkup(value, pyUpper(escape.SoftStr(value))), nil
}

// keepMarkup returns s as Markup when value was Markup. Python's string
// filters call soft_str(), which keeps a Markup a Markup, and Markup's
// str methods return Markup.
func keepMarkup(value any, s string) any {
	if _, ok := value.(escape.Markup); ok {
		return escape.Markup(s)
	}
	return s
}

// autoescapeOn reports whether the active eval context autoescapes.
func autoescapeOn(ctx *runtime.Context) bool {
	return ctx != nil && ctx.Eval != nil && ctx.Eval.Autoescape
}

// filterLower implements the `lower` filter: convert a value to lowercase.
//
// Signature: lower(s)
//
// Stringifies via [escape.SoftStr] then applies Python's str.lower (full
// Unicode case mapping, including the final-sigma rule).
//
// Example:
//
//	{{ "HELLO" | lower }}  →  hello
func filterLower(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (any, error) {
	return keepMarkup(value, pyLower(escape.SoftStr(value))), nil
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
	// do_title: re.split(r"([-\s({\[<]+)", s), then each piece becomes
	// piece[0].upper() + piece[1:].lower().
	isSep := func(r rune) bool { return pyunicode.IsSpace(r) || strings.ContainsRune("-({[<", r) }
	rs := []rune(s)
	var b strings.Builder
	for i := 0; i < len(rs); {
		j := i + 1
		for j < len(rs) && isSep(rs[j]) == isSep(rs[i]) {
			j++
		}
		b.WriteString(pyUpper(string(rs[i])))
		b.WriteString(pyLower(string(rs[i+1 : j])))
		i = j
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
	return keepMarkup(value, pyCapitalize(escape.SoftStr(value))), nil
}

// filterTrim implements the `trim` filter: strip surrounding characters.
//
// Signature: trim(s, chars=None)
//
// With no argument, trims whitespace as Python's str.strip does. With
// `chars`, trims any of the given characters from both ends; `chars`
// must be a string or None.
//
// Example:
//
//	{{ "  hi  " | trim }}        →  hi
//	{{ "##hi##" | trim('#') }}   →  hi
func filterTrim(_ *Environment, _ *runtime.Context, value any, args []any, kwargs map[string]any) (any, error) {
	s := escape.SoftStr(value)
	if v, ok := paramArg(args, kwargs, 0, "chars"); ok && v != nil {
		cut, isStr := eqString(v)
		if !isStr {
			return nil, gjerrors.NewFilterArgumentError("strip arg must be None or str")
		}
		return keepMarkup(value, strings.Trim(s, cut)), nil
	}
	return keepMarkup(value, strings.TrimFunc(s, pyunicode.IsSpace)), nil
}

// eqString returns the text of a str or Markup value.
func eqString(v any) (string, bool) {
	switch x := v.(type) {
	case string:
		return x, true
	case escape.Markup:
		return string(x), true
	}
	return "", false
}

// filterLength implements the `length` filter (alias `count`): return the
// number of items in a sequence or mapping, or runes in a string.
//
// Signature: length(value)
//
// Strings and Markup: number of characters, like Python's `len()` on a
// `str`.
// Sequences and mappings: number of elements / key-value pairs; `loop`:
// loop.length. Undefined: 0 for ModeBase / ModeChainable; ModeStrict
// raises. A generator or other iterator has no length and raises, as in
// Python: count it with `|list|length`.
//
// Example:
//
//	{{ [1, 2, 3] | length }}      →  3
//	{{ "hello"  | length }}       →  5
//	{{ {'a': 1} | length }}       →  1
func filterLength(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (any, error) {
	switch x := value.(type) {
	case string:
		return utf8.RuneCountInString(x), nil
	case escape.Markup:
		return utf8.RuneCountInString(string(x)), nil
	case runtime.Bytes:
		return len(x), nil
	case []any:
		return len(x), nil
	case runtime.Tuple:
		return len(x), nil
	case map[string]any:
		return len(x), nil
	case map[any]any:
		return len(x), nil
	case runtime.Undefined:
		// Mirrors Python: Undefined.__len__ → 0 for ModeBase /
		// ModeChainable. ModeStrict raises through Iter; we honour
		// that by routing through Iter() so the error surfaces
		// identically.
		if _, err := x.Iter(); err != nil {
			return 0, err
		}
		return 0, nil
	case interface{ Len() int }:
		// Lists, dicts and dict views.
		return x.Len(), nil
	case *runtime.LoopContext:
		return x.Length()
	}
	return 0, gjerrors.NewFilterArgumentError(fmt.Sprintf("object of type '%s' has no len()", runtime.PyTypeName(value)))
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
	if m, ok := value.(escape.Markup); ok {
		return m, nil
	}
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
		if !eval.Truthy(value) {
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
// Python's `sep.join(string)`). Under autoescape, a Markup separator or
// item makes the result Markup with every other part escaped.
//
// Example:
//
//	{{ [1, 2, 3]     | join('-') }}                  →  1-2-3
//	{{ users         | join(', ', attribute='name') }}  →  Alice, Bob
//	{{ "abc"         | join('.') }}                  →  a.b.c
func filterJoin(env *Environment, ctx *runtime.Context, value any, args []any, kwargs map[string]any) (any, error) {
	var sep any = ""
	if len(args) > 0 {
		sep = args[0]
	}
	// Optional `attribute` kwarg / second positional — extract that
	// attribute from each element before joining.
	attribute, _ := paramArg(args, kwargs, 1, "attribute")
	getter := makeAttrGetter(env, attribute, false, nil)
	items, err := iterArg("join", value)
	if err != nil {
		return nil, err
	}
	vals := make([]any, len(items))
	_, markup := sep.(escape.Markup)
	for i, it := range items {
		if attribute != nil {
			if it, err = getter(it); err != nil {
				return nil, err
			}
		}
		if err := eval.FailStrict(it); err != nil {
			return nil, err
		}
		if _, ok := it.(escape.Markup); ok {
			markup = true
		}
		vals[i] = it
	}
	// Under autoescape a Markup separator or item makes Python join as
	// Markup, escaping everything that is not already Markup.
	conv := escape.SoftStr
	if markup && autoescapeOn(ctx) {
		conv = func(v any) string { return string(escape.Escape(v)) }
	}
	parts := make([]string, len(vals))
	for i, v := range vals {
		parts[i] = conv(v)
	}
	out := strings.Join(parts, conv(sep))
	if markup && autoescapeOn(ctx) {
		return escape.Markup(out), nil
	}
	return out, nil
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
func filterReplace(_ *Environment, ctx *runtime.Context, value any, args []any, kwargs map[string]any) (any, error) {
	// The signature check guarantees old and new were passed.
	oldVal, _ := paramArg(args, kwargs, 0, "old")
	newVal, _ := paramArg(args, kwargs, 1, "new")
	old := escape.SoftStr(oldVal)
	count := -1
	if v, ok := paramArg(args, kwargs, 2, "count"); ok && v != nil {
		n, err := indexArg(v)
		if err != nil {
			return nil, err
		}
		count = n
	}
	if !autoescapeOn(ctx) {
		return strings.Replace(escape.SoftStr(value), old, escape.SoftStr(newVal), count), nil
	}
	// Under autoescape Python escapes the subject when old or new is
	// Markup, then Markup.replace escapes the replacement.
	_, oldMarkup := oldVal.(escape.Markup)
	_, newMarkup := newVal.(escape.Markup)
	subject := value
	if oldMarkup || newMarkup {
		subject = escape.Escape(value)
	}
	if m, ok := subject.(escape.Markup); ok {
		return escape.Markup(strings.Replace(string(m), old, string(escape.Escape(newVal)), count)), nil
	}
	return strings.Replace(escape.SoftStr(subject), old, escape.SoftStr(newVal), count), nil
}

func reverseRunes(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

// filterAbs implements the `abs` filter: absolute value of a number.
//
// Signature: abs(value)
//
// Python's abs(): exact for ints of any size (a bool gives 0 or 1), the
// magnitude for a complex number. Other types raise Python's "bad operand
// type for abs()".
//
// Example:
//
//	{{ -7    | abs }}  →  7
//	{{ -1.5  | abs }}  →  1.5
func filterAbs(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (any, error) {
	v, err := runtime.AbsNumber(value)
	if runtime.IsNotNumber(err) {
		return nil, gjerrors.NewFilterArgumentError(fmt.Sprintf("bad operand type for abs(): '%s'", runtime.PyTypeName(value)))
	}
	return v, err
}

// filterInt implements the `int` filter: convert a value to an integer.
//
// Signature: int(value, default=0, base=10)
//
// Conversion falls back to `default`, returned as given, if parsing
// fails; an undefined value raises. Bases other than
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
	if u, ok := value.(runtime.Undefined); ok {
		return nil, u.Fail()
	}
	// Python returns `default` untouched, whatever its type.
	var def any = int64(0)
	if len(args) > 0 {
		def = args[0]
	} else if v, ok := kwargs["default"]; ok {
		def = v
	}
	base := any(int64(10))
	if len(args) > 1 {
		base = args[1]
	} else if v, ok := kwargs["base"]; ok {
		base = v
	}
	// Jinja2's do_int: int(value, base) for a string, int(value)
	// otherwise; on a ValueError or TypeError, int(float(value)) (so
	// "42.23"|int is 42), and on any error of that (an OverflowError
	// included) the default.
	v, err := pyInt(value, base)
	if err == nil || !pyConversionError(err) {
		return v, err
	}
	f, err := pyFloat(value)
	if err != nil {
		return def, nil
	}
	if v, err = runtime.FloatToInt(f); err != nil {
		return def, nil
	}
	return v, nil
}

// nonIterable is Python's TypeError for iterating v.
func nonIterable(v any) string {
	return fmt.Sprintf("'%s' object is not iterable", runtime.PyTypeName(v))
}

// pyTypeError is a TypeError from int() or float().
type pyTypeError struct{ msg string }

func (e pyTypeError) Error() string { return e.msg }

func intTypeError(v any) error {
	return pyTypeError{fmt.Sprintf("int() argument must be a string, a bytes-like object or a real number, not '%s'", runtime.PyTypeName(v))}
}

func floatTypeError(v any) error {
	return pyTypeError{fmt.Sprintf("float() argument must be a string or a real number, not '%s'", runtime.PyTypeName(v))}
}

// pyConversionError reports whether err is the TypeError or ValueError
// do_int and do_float fall back to the default on.
func pyConversionError(err error) bool {
	var te pyTypeError
	return errors.As(err, &te) || runtime.IsValueError(err)
}

// pyInt is Python's int(value) — or int(value, base) for a string.
func pyInt(value, base any) (any, error) {
	switch x := value.(type) {
	case string:
		b, err := indexArg(base)
		if err != nil {
			return nil, intTypeError(base)
		}
		return runtime.ParseInt(x, b)
	case escape.Markup, runtime.Bytes:
		// The error shows the object itself: Markup('x'), b'x'.
		v, err := pyInt(escape.SoftStr(x), base)
		if runtime.IsValueError(err) {
			b, _ := indexArg(base)
			err = escape.ValueError{Msg: fmt.Sprintf("invalid literal for int() with base %d: %s", b, escape.Repr(x))}
		}
		return v, err
	case bool:
		if x {
			return int64(1), nil
		}
		return int64(0), nil
	case float64:
		return runtime.FloatToInt(x)
	}
	if b, ok := runtime.BigInt(value); ok {
		return runtime.NormInt(b)
	}
	return nil, intTypeError(value)
}

// pyFloat is Python's float(value).
func pyFloat(value any) (float64, error) {
	switch x := value.(type) {
	case float64:
		return x, nil
	case bool:
		if x {
			return 1, nil
		}
		return 0, nil
	case string:
		return runtime.ParseFloat(x)
	case escape.Markup, runtime.Bytes:
		f, err := runtime.ParseFloat(escape.SoftStr(x))
		if runtime.IsValueError(err) {
			err = escape.ValueError{Msg: "could not convert string to float: " + escape.Repr(x)}
		}
		return f, err
	case runtime.Undefined:
		// Undefined.__float__ raises its own error.
		return 0, x.Fail()
	}
	if runtime.IsInt(value) {
		return runtime.IntToFloat(value)
	}
	return 0, floatTypeError(value)
}

// filterFloat implements the `float` filter: convert a value to a
// float64.
//
// Signature: float(value, default=0.0)
//
// Falls back to `default`, returned as given, if parsing fails. An
// undefined value raises.
//
// Example:
//
//	{{ "3.14"  | float }}        →  3.14
//	{{ "bad"   | float(0.0) }}   →  0.0
//	{{ "bad"   | float(0) }}     →  0
func filterFloat(_ *Environment, _ *runtime.Context, value any, args []any, kwargs map[string]any) (any, error) {
	if u, ok := value.(runtime.Undefined); ok {
		return nil, u.Fail()
	}
	// Python returns `default` untouched, whatever its type.
	var def any = 0.0
	if len(args) > 0 {
		def = args[0]
	} else if v, ok := kwargs["default"]; ok {
		def = v
	}
	f, err := pyFloat(value)
	if err != nil {
		if pyConversionError(err) {
			return def, nil
		}
		return nil, err
	}
	return f, nil
}

// filterList implements the `list` filter: convert a value to a list.
//
// Signature: list(value)
//
// Python's `list()`: always a new list, even of a list. Strings split into
// single-character strings, dicts yield their keys, and a generator is
// read to the end.
//
// Example:
//
//	{{ "abc"  | list }}  →  ['a', 'b', 'c']
//	{{ {'a': 1, 'b': 2} | list }}  →  ['a', 'b']
func filterList(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (any, error) {
	x, err := eval.ToIterable(value)
	if err != nil {
		return nil, err
	}
	out := make([]any, len(x))
	copy(out, x)
	return out, nil
}

// filterFirst implements the `first` filter: first item of a sequence.
//
// Signature: first(seq)
//
// Works on any iterable: a string's first character, a dict's first key.
// An empty iterable yields an Undefined value; a non-iterable raises.
//
// Example:
//
//	{{ [1, 2, 3]  | first }}  →  1
func filterFirst(env *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (any, error) {
	if it, ok := value.(*runtime.Iterator); ok {
		// next(iter(seq)) reads just one item.
		v, ok, err := it.Next()
		if err != nil || ok {
			return v, err
		}
		return env.undefined("No first item, sequence was empty.", "", runtime.Missing, nil), nil
	}
	x, err := iterArg("first", value)
	if err != nil {
		return nil, err
	}
	if len(x) == 0 {
		return env.undefined("No first item, sequence was empty.", "", runtime.Missing, nil), nil
	}
	return x[0], nil
}

// filterLast implements the `last` filter: last item of a sequence.
//
// Signature: last(seq)
//
// Works on any iterable: a string's last character, a dict's last key.
// An empty iterable yields an Undefined value; a non-iterable raises.
//
// Example:
//
//	{{ [1, 2, 3]  | last }}  →  3
func filterLast(env *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (any, error) {
	if _, ok := value.(*runtime.Iterator); ok {
		return nil, gjerrors.NewTemplateRuntimeError(fmt.Sprintf("'%s' object is not reversible", runtime.PyTypeName(value)))
	}
	x, err := iterArg("last", value)
	if err != nil {
		if runtime.IsUndefined(value) {
			return nil, err
		}
		// next(iter(reversed(value))) fails at reversed().
		return nil, gjerrors.NewFilterArgumentError(fmt.Sprintf("'%s' object is not reversible", runtime.PyTypeName(value)))
	}
	if len(x) == 0 {
		return env.undefined("No last item, sequence was empty.", "", runtime.Missing, nil), nil
	}
	// reversed() indexes a Markup, which gives Markup characters.
	if _, ok := value.(escape.Markup); ok {
		return escape.Markup(x[len(x)-1].(string)), nil
	}
	return x[len(x)-1], nil
}

// filterSort implements the `sort` filter: sort a sequence.
//
// Signature: sort(value, reverse=False, case_sensitive=False, attribute=None)
//
// Accepts any iterable, like Python's `sorted()`; a dict sorts its keys.
// Sort is **stable**. With `attribute`, sorts by the named attribute of
// each element (dotted paths and integer indices both supported); a
// comma-separated list such as `'age,name'` sorts by each attribute in
// turn. String comparisons are case-insensitive by default.
//
// Example:
//
//	{{ ['B', 'a', 'C'] | sort }}                      →  ['a', 'B', 'C']
//	{{ users           | sort(attribute='age') }}     →  sorted by age
//	{{ users           | sort(attribute='age,name') }}
//	{{ users           | sort(reverse=true, attribute='name') }}
func filterSort(env *Environment, _ *runtime.Context, value any, args []any, kwargs map[string]any) (any, error) {
	x, err := eval.ToIterable(value)
	if err != nil || value == nil {
		return nil, gjerrors.NewFilterArgumentError(nonIterable(value))
	}
	// Python's signature: sort(reverse=False, case_sensitive=False, attribute=None)
	reverse, caseSensitive := false, false
	if v, ok := paramArg(args, kwargs, 0, "reverse"); ok {
		reverse = eval.Truthy(v)
	}
	if v, ok := paramArg(args, kwargs, 1, "case_sensitive"); ok {
		caseSensitive = eval.Truthy(v)
	}
	attribute, _ := paramArg(args, kwargs, 2, "attribute")
	out := make([]any, len(x))
	copy(out, x)
	if err := sortBy(out, makeMultiAttrGetter(env, attribute, !caseSensitive), reverse); err != nil {
		return nil, err
	}
	return out, nil
}

// sortBy sorts items like Python's sorted(items, key=key): every key is
// computed first, the sort is stable, keys are ordered with `<`, and
// unorderable keys fail with Python's TypeError.
func sortBy(items []any, key func(any) (any, error), reverse bool) error {
	type keyed struct{ key, item any }
	pairs := make([]keyed, len(items))
	for i, it := range items {
		k, err := key(it)
		if err != nil {
			return err
		}
		pairs[i] = keyed{k, it}
	}
	var cmpErr error
	sort.SliceStable(pairs, func(i, j int) bool {
		a, b := pairs[i].key, pairs[j].key
		if reverse {
			a, b = b, a
		}
		lt, err := eval.Compare("lt", a, b)
		if err != nil && cmpErr == nil {
			cmpErr = err
		}
		return lt
	})
	for i, p := range pairs {
		items[i] = p.item
	}
	return cmpErr
}

// attributeParts is _prepare_attribute_parts: a string splits at dots,
// all-digit parts becoming ints; any other attribute is a single part.
func attributeParts(attribute any) ([]any, error) {
	a, ok := attribute.(string)
	if !ok {
		if attribute == nil {
			return nil, nil
		}
		return []any{attribute}, nil
	}
	var parts []any
	for _, p := range strings.Split(a, ".") {
		if p != "" && strings.IndexFunc(p, func(r rune) bool { return !pyunicode.IsDigit(r) }) < 0 {
			n, err := runtime.ParseInt(p, 10)
			if err != nil {
				return nil, err
			}
			parts = append(parts, n)
			continue
		}
		parts = append(parts, p)
	}
	return parts, nil
}

// makeAttrGetter is make_attrgetter: it looks attribute up in an item
// with the environment's getitem, one part at a time. A non-nil def
// replaces an undefined step; lower applies ignore_case to the result.
func makeAttrGetter(env *Environment, attribute any, lower bool, def any) func(any) (any, error) {
	parts, perr := attributeParts(attribute)
	return func(item any) (any, error) {
		if perr != nil {
			return nil, perr
		}
		for _, p := range parts {
			v, err := env.GetItem(item, p)
			if err != nil {
				return nil, err
			}
			item = v
			if u, ok := item.(runtime.Undefiner); ok && def != nil && u.IsUndefined() {
				item = def
			}
		}
		if lower {
			item = ignoreCase(item)
		}
		return item, nil
	}
}

// makeMultiAttrGetter is make_multi_attrgetter: a string attribute is a
// comma-separated list of attributes, and the key is the list of their
// values.
func makeMultiAttrGetter(env *Environment, attribute any, lower bool) func(any) (any, error) {
	split := []any{attribute}
	if a, ok := attribute.(string); ok {
		split = split[:0]
		for _, p := range strings.Split(a, ",") {
			split = append(split, p)
		}
	}
	getters := make([]func(any) (any, error), len(split))
	for i, a := range split {
		getters[i] = makeAttrGetter(env, a, lower, nil)
	}
	return func(item any) (any, error) {
		key := make([]any, len(getters))
		for i, g := range getters {
			v, err := g(item)
			if err != nil {
				return nil, err
			}
			key[i] = v
		}
		return key, nil
	}
}

// ignoreCase is ignore_case: strings are lowercased, other values kept.
func ignoreCase(v any) any {
	switch x := v.(type) {
	case string:
		return pyLower(x)
	case escape.Markup:
		return escape.Markup(pyLower(string(x)))
	}
	return v
}

// ----------------------------------------------------- additional filters

// filterSum implements the `sum` filter: sum the items of a sequence.
//
// Signature: sum(iterable, attribute=None, start=0)
//
// `attribute` (positional or keyword, dotted-path supported) extracts a
// field from each item. Items are added to `start` with the `+` operator,
// so lists concatenate and mixing numbers with strings raises, as in
// Python; a string `start` raises.
//
// Example:
//
//	{{ [1, 2, 3]     | sum }}                       →  6
//	{{ products      | sum(attribute='price') }}    →  total price
func filterSum(env *Environment, _ *runtime.Context, value any, args []any, kwargs map[string]any) (any, error) {
	items, err := iterArg("sum", value)
	if err != nil {
		return nil, err
	}
	// Python signature: sum(attribute=None, start=0)
	attribute, _ := paramArg(args, kwargs, 0, "attribute")
	getter := makeAttrGetter(env, attribute, false, nil)
	var total any = int64(0)
	if v, ok := paramArg(args, kwargs, 1, "start"); ok {
		total = v
	}
	if _, ok := total.(string); ok {
		return nil, gjerrors.NewFilterArgumentError("sum() can't sum strings [use ''.join(seq) instead]")
	}
	for _, it := range items {
		if attribute != nil {
			if it, err = getter(it); err != nil {
				return nil, err
			}
		}
		if total, err = eval.BinaryOp("+", total, it); err != nil {
			return nil, err
		}
	}
	return total, nil
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
	x, err := iterArg("min", value)
	if err != nil {
		return nil, err
	}
	if len(x) == 0 {
		return env.undefined("No aggregated item, sequence was empty.", "", runtime.Missing, nil), nil
	}
	key := minMaxKey(env, args, kwargs)
	mi := x[0]
	miKey, err := key(mi)
	if err != nil {
		return nil, err
	}
	for _, it := range x[1:] {
		k, err := key(it)
		if err != nil {
			return nil, err
		}
		lt, err := eval.Compare("lt", k, miKey)
		if err != nil {
			return nil, err
		}
		if lt {
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
	x, err := iterArg("max", value)
	if err != nil {
		return nil, err
	}
	if len(x) == 0 {
		return env.undefined("No aggregated item, sequence was empty.", "", runtime.Missing, nil), nil
	}
	key := minMaxKey(env, args, kwargs)
	mx := x[0]
	mxKey, err := key(mx)
	if err != nil {
		return nil, err
	}
	for _, it := range x[1:] {
		// Python's max() compares with `>`.
		k, err := key(it)
		if err != nil {
			return nil, err
		}
		gt, err := eval.Compare("gt", k, mxKey)
		if err != nil {
			return nil, err
		}
		if gt {
			mx = it
			mxKey = k
		}
	}
	return mx, nil
}

// minMaxKey is _min_or_max's key for min(value, case_sensitive=False,
// attribute=None) and max.
func minMaxKey(env *Environment, args []any, kwargs map[string]any) func(any) (any, error) {
	caseSensitive := false
	if v, ok := paramArg(args, kwargs, 0, "case_sensitive"); ok {
		caseSensitive = eval.Truthy(v)
	}
	attribute, _ := paramArg(args, kwargs, 1, "attribute")
	return makeAttrGetter(env, attribute, !caseSensitive, nil)
}

// filterDictsort implements the `dictsort` filter: sort a dict and emit
// (key, value) pairs.
//
// Signature: dictsort(value, case_sensitive=False, by="key", reverse=False)
//
// Use `by="value"` to sort by the dict values instead of keys; any other
// `by` raises. Returns a list of (key, value) tuples.
//
// Example:
//
//	{% for k, v in {'b': 2, 'a': 1} | dictsort %}{{ k }}={{ v }} {% endfor %}
//	→  a=1 b=2
func filterDictsort(_ *Environment, _ *runtime.Context, value any, args []any, kwargs map[string]any) (any, error) {
	caseSensitive := false
	if v, ok := paramArg(args, kwargs, 0, "case_sensitive"); ok {
		caseSensitive = eval.Truthy(v)
	}
	pos := 0
	if v, ok := paramArg(args, kwargs, 1, "by"); ok {
		switch v {
		case "key":
		case "value":
			pos = 1
		default:
			return nil, gjerrors.NewFilterArgumentError(`You can only sort by either "key" or "value"`)
		}
	}
	reverse := false
	if v, ok := paramArg(args, kwargs, 2, "reverse"); ok {
		reverse = eval.Truthy(v)
	}
	var items []any
	switch m := value.(type) {
	case *runtime.OrderedDict:
		items = m.Items()
	case map[string]any:
		keys, _ := eval.ToIterable(m)
		for _, k := range keys {
			items = append(items, runtime.Tuple{k, m[k.(string)]})
		}
	case map[any]any:
		keys, _ := eval.ToIterable(m)
		for _, k := range keys {
			items = append(items, runtime.Tuple{k, m[k]})
		}
	default:
		if u, ok := value.(runtime.Undefined); ok {
			return nil, u.Fail()
		}
		return nil, gjerrors.NewFilterArgumentError(fmt.Sprintf("'%s' object has no attribute 'items'", runtime.PyTypeName(value)))
	}
	key := func(item any) (any, error) {
		v := item.(runtime.Tuple)[pos]
		if !caseSensitive {
			v = ignoreCase(v)
		}
		return v, nil
	}
	if err := sortBy(items, key, reverse); err != nil {
		return nil, err
	}
	return items, nil
}

// unhashable returns Python's TypeError for a value that cannot be a dict
// key (lists and dicts).
func unhashable(v any) error {
	switch v.(type) {
	case []any, *runtime.PyList, []string, map[string]any, map[any]any, *runtime.OrderedDict:
		return gjerrors.NewTemplateRuntimeError(fmt.Sprintf("unhashable type: '%s'", runtime.PyTypeName(v)))
	}
	return nil
}

// indexArg converts an argument Python requires to be an integer
// (anything with __index__): ints and bools pass, anything else raises
// Python's TypeError.
func indexArg(v any) (int, error) {
	switch x := v.(type) {
	case bool:
		if x {
			return 1, nil
		}
		return 0, nil
	case int:
		return x, nil
	case int64:
		return int(x), nil
	}
	switch rv := reflect.ValueOf(v); rv.Kind() {
	case reflect.Int8, reflect.Int16, reflect.Int32:
		return int(rv.Int()), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return int(rv.Uint()), nil
	}
	if runtime.IsInt(v) {
		return 0, gjerrors.NewFilterArgumentError("Python int too large to convert to C ssize_t")
	}
	return 0, gjerrors.NewFilterArgumentError(fmt.Sprintf("'%s' object cannot be interpreted as an integer", runtime.PyTypeName(v)))
}

// paramArg returns the argument bound to the Python parameter at
// position i (counting after the value) or passed by keyword as name.
func paramArg(args []any, kwargs map[string]any, i int, name string) (any, bool) {
	if i < len(args) {
		return args[i], true
	}
	v, ok := kwargs[name]
	return v, ok
}

// filterTruncate implements the `truncate` filter: shorten a string and
// append an ellipsis.
//
// Signature: truncate(s, length=255, killwords=False, end='...', leeway=5)
//
// A value no longer than `length + leeway` is returned unchanged, whatever
// its type (an Undefined or a short list too). `killwords=False` (the
// default) breaks at the last whitespace before the cut so words aren't
// split mid-word. Lengths count characters. A Markup input stays Markup
// and its `end` is escaped; `length` shorter than `end`, a negative
// `leeway`, a value without a length, or a long value that is not a
// string raises, as in Python.
//
// Example:
//
//	{{ "this is a long sentence" | truncate(12) }}  →  this is a...
func filterTruncate(_ *Environment, _ *runtime.Context, value any, args []any, kwargs map[string]any) (any, error) {
	s := escape.SoftStr(value)
	var lengthVal any = int64(255)
	if v, ok := paramArg(args, kwargs, 0, "length"); ok {
		lengthVal = v
	}
	killWords := false
	if v, ok := paramArg(args, kwargs, 1, "killwords"); ok {
		killWords = eval.Truthy(v)
	}
	var endVal any = "..."
	if v, ok := paramArg(args, kwargs, 2, "end"); ok {
		endVal = v
	}
	end := escape.SoftStr(endVal)
	// leeway=None means the truncate.leeway policy, 5.
	var leewayVal any = int64(5)
	if v, ok := paramArg(args, kwargs, 3, "leeway"); ok && v != nil {
		leewayVal = v
	}
	// do_truncate asserts length >= len(end) and leeway >= 0.
	endLen := int64(utf8.RuneCountInString(end))
	if ok, err := eval.Compare("gteq", lengthVal, endLen); err != nil {
		return nil, gjerrors.NewFilterArgumentError(err.Error())
	} else if !ok {
		return nil, gjerrors.NewFilterArgumentError(fmt.Sprintf("expected length >= %d, got %v", endLen, escape.SoftStr(lengthVal)))
	}
	if ok, err := eval.Compare("gteq", leewayVal, int64(0)); err != nil {
		return nil, gjerrors.NewFilterArgumentError(err.Error())
	} else if !ok {
		return nil, gjerrors.NewFilterArgumentError(fmt.Sprintf("expected leeway >= 0, got %v", escape.SoftStr(leewayVal)))
	}
	length, err := indexArg(lengthVal)
	if err != nil {
		return nil, err
	}
	leeway, err := indexArg(leewayVal)
	if err != nil {
		return nil, err
	}
	// len(s) of the value itself, which is returned as it is when short.
	n, err := filterLength(nil, nil, value, nil, nil)
	if err != nil {
		return nil, gjerrors.NewTemplateRuntimeError(err.Error())
	}
	if n.(int) <= length+leeway {
		return value, nil
	}
	cut := length - utf8.RuneCountInString(end)
	if err := truncateNonString(value, cut, killWords); err != nil {
		return nil, err
	}
	rs := []rune(s)
	head := string(rs[:cut])
	if !killWords {
		// s[:cut].rsplit(" ", 1)[0]
		if idx := strings.LastIndex(head, " "); idx >= 0 {
			head = head[:idx]
		}
	}
	// A Markup subject stays Markup, and Markup + str escapes the end.
	if _, ok := value.(escape.Markup); ok {
		return escape.Markup(head) + escape.Escape(endVal), nil
	}
	return head + end, nil
}

// truncateNonString is the error do_truncate meets slicing a value that
// is not a string, s[:cut], and adding the end or calling rsplit on it.
func truncateNonString(value any, cut int, killWords bool) error {
	pick := func(killed, kept string) error {
		if killWords {
			return methodError("%s", killed)
		}
		return methodError("%s", kept)
	}
	switch value.(type) {
	case string, escape.Markup:
		return nil
	case []any, *runtime.PyList:
		return pick(`can only concatenate list (not "str") to list`, "'list' object has no attribute 'rsplit'")
	case runtime.Tuple:
		return pick(`can only concatenate tuple (not "str") to tuple`, "'tuple' object has no attribute 'rsplit'")
	case runtime.Bytes:
		return pick("can't concat str to bytes", "a bytes-like object is required, not 'str'")
	case *runtime.Range:
		return pick("unsupported operand type(s) for +: 'range' and 'str'", "'range' object has no attribute 'rsplit'")
	case *runtime.OrderedDict, map[string]any, map[any]any:
		// A dict looks the slice up as a key.
		return methodError("slice(None, %d, None)", cut)
	}
	return methodError("'%s' object is not subscriptable", runtime.PyTypeName(value))
}

// filterWordcount implements the `wordcount` filter: count "words" in a
// string.
//
// Signature: wordcount(s)
//
// A word is a maximal run of Unicode letters, digits and underscores,
// Python's `\w`. Anything else is a separator.
//
// Example:
//
//	{{ "hello world foo"  | wordcount }}  →  3
func filterWordcount(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (any, error) {
	s := escape.SoftStr(value)
	count := 0
	inWord := false
	for _, r := range s {
		isWord := r == '_' || pyunicode.IsAlnum(r)
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
// `width` is a number of spaces or a string used as the indentation.
// `first=False` skips indenting the first line (so the filter chains with
// templates that already have content on the line). `blank=False` skips
// blank lines. As in Python, which adds a newline to the value and splits
// it, any value other than a string or Markup raises (an Undefined too).
//
// Example:
//
//	{{ "a\nb\nc" | indent(2, true) }}  →  "  a\n  b\n  c"
func filterIndent(_ *Environment, _ *runtime.Context, value any, args []any, kwargs map[string]any) (any, error) {
	// do_indent starts with s += "\n", then calls s.splitlines().
	switch x := value.(type) {
	case string, escape.Markup:
	case runtime.Undefined:
		return nil, x.Fail()
	case []any, *runtime.PyList:
		return nil, methodError("'list' object has no attribute 'splitlines'")
	case runtime.Tuple:
		return nil, methodError(`can only concatenate tuple (not "str") to tuple`)
	case runtime.Bytes:
		return nil, methodError("can't concat str to bytes")
	default:
		return nil, methodError("unsupported operand type(s) for +=: '%s' and 'str'", runtime.PyTypeName(value))
	}
	s := escape.SoftStr(value)
	// width is a string used as-is, or a count of spaces (" " * width).
	pad := "    "
	if v, ok := paramArg(args, kwargs, 0, "width"); ok {
		if w, isStr := v.(string); isStr {
			pad = w
		} else {
			p, err := eval.BinaryOp("*", " ", v)
			if err != nil {
				return nil, err
			}
			pad = escape.SoftStr(p)
		}
	}
	first := false
	if v, ok := paramArg(args, kwargs, 1, "first"); ok {
		first = eval.Truthy(v)
	}
	blank := false
	if v, ok := paramArg(args, kwargs, 2, "blank"); ok {
		blank = eval.Truthy(v)
	}
	// do_indent: s += "\n", then splitlines(), rejoined with "\n".
	lines := pySplitlines(s+"\n", false)
	var rv string
	if blank {
		rv = strings.Join(lines, "\n"+pad)
	} else {
		rv = lines[0]
		if len(lines) > 1 {
			rest := make([]string, len(lines)-1)
			for i, line := range lines[1:] {
				if line != "" {
					line = pad + line
				}
				rest[i] = line
			}
			rv += "\n" + strings.Join(rest, "\n")
		}
	}
	if first {
		rv = pad + rv
	}
	return keepMarkup(value, rv), nil
}

// filterStripTags implements the `striptags` filter: drop HTML tags and
// HTML comments, then collapse whitespace.
//
// Signature: striptags(s)
//
// markupsafe's Markup.striptags(): `<!-- ... -->` comments, then tags, are
// removed (an unclosed one is kept), whitespace runs collapse to a single
// space, and HTML entities are unescaped as Python's html.unescape does.
//
// Example:
//
//	{{ "<p>hi <b>there</b></p>" | striptags }}  →  hi there
func filterStripTags(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (any, error) {
	return stripTags(escape.SoftStr(value)), nil
}

// stripTags is markupsafe's Markup.striptags: comments, then tags, are
// removed (an unclosed one is kept), whitespace runs collapse to one
// space and entities are unescaped.
func stripTags(s string) string {
	for {
		start := strings.Index(s, "<!--")
		if start < 0 {
			break
		}
		end := strings.Index(s[start:], "-->")
		if end < 0 {
			break
		}
		s = s[:start] + s[start+end+3:]
	}
	for {
		start := strings.Index(s, "<")
		if start < 0 {
			break
		}
		end := strings.Index(s[start:], ">")
		if end < 0 {
			break
		}
		s = s[:start] + s[start+end+1:]
	}
	return escape.Unescape(strings.Join(splitWhitespace(s, -1, false), " "))
}

// filterCenter implements the `center` filter: pad a string with spaces
// to centre it within a given width.
//
// Signature: center(s, width=80)
//
// Strings that already exceed `width` are returned unchanged. An odd
// margin puts the extra space where Python's str.center does.
//
// Example:
//
//	{{ "hi" | center(6) }}  →  "  hi  "
//	{{ "hi" | center(5) }}  →  "  hi "
func filterCenter(_ *Environment, _ *runtime.Context, value any, args []any, kwargs map[string]any) (any, error) {
	s := escape.SoftStr(value)
	width := 80
	if v, ok := paramArg(args, kwargs, 0, "width"); ok {
		n, err := indexArg(v)
		if err != nil {
			return nil, err
		}
		width = n
	}
	return keepMarkup(value, pyCenter(s, width, " ")), nil
}

// filterRound implements the `round` filter: round a number to N decimal
// places.
//
// Signature: round(value, precision=0, method='common')
//
// Methods: 'common' (Python's round(): half-to-even on the exact value),
// 'ceil', 'floor'. Negative precision rounds to powers of ten (e.g.
// precision=-2 rounds to the nearest 100). 'common' keeps an integer
// input an integer; every other case returns a float.
//
// Example:
//
//	{{ 3.14159 | round(2) }}             →  3.14
//	{{ 2.5     | round }}                →  2.0
//	{{ 1250    | round(-2) }}            →  1200
//	{{ 3.5     | round(0, 'ceil') }}     →  4.0
func filterRound(_ *Environment, _ *runtime.Context, value any, args []any, kwargs map[string]any) (any, error) {
	// Python signature: round(precision=0, method='common')
	prec := 0
	if v, ok := paramArg(args, kwargs, 0, "precision"); ok {
		n, err := indexArg(v)
		if err != nil {
			return nil, err
		}
		prec = n
	}
	method := "common"
	if len(args) > 1 {
		if s, ok := args[1].(string); ok {
			method = s
		}
	} else if v, ok := kwargs["method"].(string); ok {
		method = v
	}
	if method != "common" && method != "ceil" && method != "floor" {
		return nil, gjerrors.NewFilterArgumentError("method must be common, ceil or floor")
	}
	// Python's round() rejects an Undefined with a TypeError, while
	// ceil/floor first multiply it, which raises the undefined error.
	if method != "common" {
		if err := eval.FailUndefined(value); err != nil {
			return nil, err
		}
	}
	if method == "common" {
		return pyRound(value, prec)
	}
	// func(value * 10**precision) / 10**precision, with Python's
	// arithmetic: exact for an int, a float for the result.
	p, err := runtime.NumBinop("**", int64(10), int64(prec))
	if err != nil {
		return nil, err
	}
	m, err := eval.BinaryOp("*", value, p)
	if err != nil {
		return nil, err
	}
	if f, ok := m.(float64); ok {
		if method == "ceil" {
			f = math.Ceil(f)
		} else {
			f = math.Floor(f)
		}
		if m, err = runtime.FloatToInt(f); err != nil {
			return nil, err
		}
	} else if !runtime.IsInt(m) {
		return nil, gjerrors.NewFilterArgumentError(fmt.Sprintf("must be real number, not %s", runtime.PyTypeName(m)))
	}
	return runtime.NumBinop("/", m, p)
}

// pyRound is Python's round(value, ndigits): an int stays an int, a
// float rounds half to even exactly.
func pyRound(value any, ndigits int) (any, error) {
	switch x := value.(type) {
	case float64:
		return pyRoundFloat(x, ndigits), nil
	case bool:
		if x {
			return int64(1), nil
		}
		return int64(0), nil
	}
	b, ok := runtime.BigInt(value)
	if !ok {
		return nil, gjerrors.NewFilterArgumentError(fmt.Sprintf("type %s doesn't define __round__ method", runtime.PyTypeName(value)))
	}
	if ndigits >= 0 {
		return runtime.NormInt(b)
	}
	r := roundRatHalfEven(new(big.Rat).SetInt(b), ndigits)
	return runtime.NormInt(r.Num())
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
	f, err := pyFloat(value)
	if err != nil {
		return nil, gjerrors.NewFilterArgumentError(err.Error())
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
	case escape.Markup:
		return urlEncodePath(string(x)), nil
	}
	// Any other iterable is a sequence of (key, value) pairs.
	items, err := eval.ToIterable(value)
	if err != nil || value == nil {
		return urlEncodePath(escape.SoftStr(value)), nil
	}
	parts := make([]string, 0, len(items))
	for _, it := range items {
		pair, err := eval.ToIterable(it)
		if err != nil {
			return nil, gjerrors.NewFilterArgumentError(fmt.Sprintf("cannot unpack non-iterable %s object", runtime.PyTypeName(it)))
		}
		switch {
		case len(pair) > 2:
			return nil, gjerrors.NewFilterArgumentError("too many values to unpack (expected 2)")
		case len(pair) < 2:
			return nil, gjerrors.NewFilterArgumentError(fmt.Sprintf("not enough values to unpack (expected 2, got %d)", len(pair)))
		}
		parts = append(parts, urlEncodeQS(escape.SoftStr(pair[0]))+"="+urlEncodeQS(escape.SoftStr(pair[1])))
	}
	return strings.Join(parts, "&"), nil
}

// filterTojson implements the `tojson` filter: serialise a value as JSON
// safe for embedding in `<script>` blocks.
//
// Signature: tojson(value, indent=None)
//
// `&`, `<`, `>` and `'` are unicode-escaped (`\u003c` etc.) so the
// result is safe in a `<script>` block or a single-quoted HTML attribute.
// Like json.dumps, non-ASCII characters and DEL are escaped as `\uXXXX`
// (surrogate pairs past the BMP). `indent` follows json.dumps: an int
// indents by that many spaces, a string indents with itself. The result
// is returned as Markup.
//
// Example:
//
//	<script>const data = {{ payload | tojson }};</script>
func filterTojson(_ *Environment, _ *runtime.Context, value any, args []any, kwargs map[string]any) (any, error) {
	// indent follows json.dumps: None is compact, an int indents by that
	// many spaces and a string indents with itself.
	pretty, indent := false, ""
	if v, ok := paramArg(args, kwargs, 0, "indent"); ok && v != nil {
		pretty = true
		if s, ok := v.(string); ok {
			indent = s
		} else {
			n, err := indexArg(v)
			if err != nil {
				return nil, err
			}
			indent = strings.Repeat(" ", max(n, 0))
		}
	}
	b, err := jsonMarshal(value, pretty, indent)
	if err != nil {
		return nil, err
	}
	// encoding/json already escapes <, > and & as \u003c, \u003e and
	// \u0026; htmlsafe_json_dumps also escapes the single quote so the
	// output is safe inside a single-quoted HTML attribute.
	return escape.Markup(strings.ReplaceAll(string(b), "'", `\u0027`)), nil
}

// filterAttr implements the `attr` filter: look up an attribute by name.
//
// Signature: attr(value, name)
//
// Python's getattr(): unlike `value.name` it never falls back to item
// lookup, so on a dict only its methods are attributes and a key yields
// Undefined. Other values go through the sandbox-aware
// [Environment.GetAttr]. Useful when the attribute name is dynamic.
//
// Example:
//
//	{{ obj | attr('field_name') }}     ≡  {{ obj.field_name }}
func filterAttr(env *Environment, _ *runtime.Context, value any, args []any, _ map[string]any) (any, error) {
	name, ok := eqString(args[0])
	if !ok {
		return nil, gjerrors.NewFilterArgumentError(fmt.Sprintf("attribute name must be string, not '%s'", runtime.PyTypeName(args[0])))
	}
	if u, ok := env.unsafeAttr(value, name); ok {
		return u, nil
	}
	// attr is getattr(): on a dict only its methods are attributes, never
	// its keys.
	switch m := value.(type) {
	case map[string]any, map[any]any, *runtime.OrderedDict:
		if method := dictMethod(m, name); method != nil {
			return bindMethod(value, name, method), nil
		}
		return env.undefined("", name, value, nil), nil
	}
	return env.GetAttr(value, name)
}

// filterFormat implements the `format` filter: Python printf-style
// formatting.
//
// Signature: format(value, *args, **kwargs)
//
// `value % args` (or `value % kwargs` when keywords are given) with
// CPython's printf-style rules, via [eval.PercentFormat]: every
// conversion type, flags, width and precision (including `*`), and
// `%(name)s` keys. A Markup format string escapes its %s/%r arguments
// and returns Markup.
//
// Example:
//
//	{{ '%s scored %d' | format('Alice', 95) }}   →  Alice scored 95
//	{{ '%(n)s: %(v).1f' | format(n='pi', v=3.14159) }}  →  pi: 3.1
func filterFormat(_ *Environment, _ *runtime.Context, value any, args []any, kwargs map[string]any) (any, error) {
	// soft_str(value) % (kwargs or args)
	var operand any = runtime.Tuple(args)
	if len(kwargs) > 0 {
		operand = kwargs
	}
	m, isMarkup := value.(escape.Markup)
	if !isMarkup {
		s, err := eval.PercentFormat(escape.SoftStr(value), operand, false)
		if err != nil {
			return nil, formatError(err)
		}
		return s, nil
	}
	s, err := eval.PercentFormat(string(m), operand, true)
	if err != nil {
		return nil, formatError(err)
	}
	return escape.Markup(s), nil
}

// formatError keeps template errors (an undefined argument) as they are
// and reports formatting failures as filter argument errors.
func formatError(err error) error {
	if _, ok := gjerrors.AsTemplateError(err); ok {
		return err
	}
	return gjerrors.NewFilterArgumentError(err.Error())
}

// ------------------------------------------------------ higher-order filters

// iterArg returns the items a filter iterates, as Python's iter() would:
// strings and Markup yield their characters, mappings their keys, and an
// Undefined nothing (a StrictUndefined raises).
func iterArg(filter string, value any) ([]any, error) {
	items, err := eval.ToIterable(value)
	if err != nil {
		if runtime.IsUndefined(value) {
			return nil, err
		}
		return nil, gjerrors.NewFilterArgumentError(nonIterable(value))
	}
	return items, nil
}

// skipFalsy reports whether a map/select-style filter yields nothing for
// value: Python's filters start with `if value:`, so a falsy value (None,
// 0, an empty sequence) gives an empty result before any argument is
// checked. A StrictUndefined raises, as bool() of it does.
func skipFalsy(value any) (bool, error) {
	if err := eval.FailStrict(value); err != nil {
		return true, err
	}
	return !eval.Truthy(value), nil
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
	return runtime.IsNumber(value), nil
}

// testInteger implements the `integer` test: true if the value is any
// integer type (no floats).
//
// Signature: x is integer
func testInteger(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	return runtime.IsInt(value), nil
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
	return modEquals(value, int64(2), int64(1))
}

// testEven implements the `even` test: true if the value is an integer
// and divisible by 2.
//
// Signature: x is even
func testEven(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	return modEquals(value, int64(2), int64(0))
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
		return false, nil
	}
	return modEquals(value, args[0], int64(0))
}

// modEquals is Python's `value % n == want`, raising like the `%`
// operator on undefined, non-numeric or zero operands.
func modEquals(value, n, want any) (bool, error) {
	r, err := eval.BinaryOp("%", value, n)
	if err != nil {
		return false, err
	}
	return eval.ValuesEqual(r, want)
}

// testSequence implements the `sequence` test: true if the value can be
// length-queried and index-accessed.
//
// Signature: x is sequence
//
// Includes lists, tuples, strings, and mappings (mirrors Python's "anything
// with `__len__` and `__getitem__`"). An Undefined is a sequence, except a
// StrictUndefined, whose `len()` fails.
func testSequence(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	switch x := value.(type) {
	case []any, string, escape.Markup, []string, runtime.Tuple, map[string]any, map[any]any, *runtime.OrderedDict,
		*runtime.PyList, *runtime.Range, runtime.Bytes:
		return true, nil
	case runtime.Undefined:
		return x.Mode != runtime.ModeStrict, nil
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
// Lists, tuples, strings (over runes) and mappings are iterable,
// matching Python's `iter(value)` succeeding. An Undefined iterates as
// empty; a StrictUndefined raises.
func testIterable(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	switch x := value.(type) {
	case []any, runtime.Tuple, string, escape.Markup, []string, map[string]any, map[any]any, *runtime.OrderedDict,
		*runtime.PyList, *runtime.Range, *runtime.DictView, runtime.Bytes, *runtime.Iterator, *runtime.LoopContext:
		return true, nil
	case runtime.Undefined:
		if _, err := x.Iter(); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

// testLowerStr implements the `lower` test: true if the value is a string
// containing at least one letter and equal to its lowercase form.
//
// Signature: x is lower
func testLowerStr(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	// Python tests str(value), so non-strings are checked by their text.
	s := escape.SoftStr(value)
	return pyIsLower(s), nil
}

// testUpperStr implements the `upper` test: true if the value is a string
// containing at least one letter and equal to its uppercase form.
//
// Signature: x is upper
func testUpperStr(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	// Python tests str(value), so non-strings are checked by their text.
	s := escape.SoftStr(value)
	return pyIsUpper(s), nil
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
	return eval.ValuesEqual(value, args[0])
}

// testNe implements the `ne` / `!=` test: inequality comparison.
//
// Signature: x is ne(other)  ·  x is !=(other)
func testNe(_ *Environment, _ *runtime.Context, value any, args []any, _ map[string]any) (bool, error) {
	if len(args) < 1 {
		return false, nil
	}
	eq, err := eval.ValuesEqual(value, args[0])
	return !eq, err
}

// testLt implements the `lt` / `lessthan` / `<` test: strict-less comparison.
//
// Signature: x is lt(other)
func testLt(_ *Environment, _ *runtime.Context, value any, args []any, _ map[string]any) (bool, error) {
	if len(args) < 1 {
		return false, nil
	}
	return eval.Compare("lt", value, args[0])
}

// testLtEq implements the `le` / `<=` test: less-or-equal comparison.
//
// Signature: x is le(other)
func testLtEq(_ *Environment, _ *runtime.Context, value any, args []any, _ map[string]any) (bool, error) {
	if len(args) < 1 {
		return false, nil
	}
	return eval.Compare("lteq", value, args[0])
}

// testGt implements the `gt` / `greaterthan` / `>` test: strict-greater
// comparison.
//
// Signature: x is gt(other)
func testGt(_ *Environment, _ *runtime.Context, value any, args []any, _ map[string]any) (bool, error) {
	if len(args) < 1 {
		return false, nil
	}
	return eval.Compare("gt", value, args[0])
}

// testGtEq implements the `ge` / `>=` test: greater-or-equal comparison.
//
// Signature: x is ge(other)
func testGtEq(_ *Environment, _ *runtime.Context, value any, args []any, _ map[string]any) (bool, error) {
	if len(args) < 1 {
		return false, nil
	}
	return eval.Compare("gteq", value, args[0])
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
	return eval.Contains(value, args[0])
}

// testSameAs implements the `sameas` test: identity comparison (`is`).
//
// Signature: x is sameas(other)
//
// Python identity has no direct Go equivalent: comparable values (None,
// bools, numbers, strings, pointers) compare with `==`; slices, maps and
// funcs compare by the address they reference. An Undefined is never
// the same as anything, because Python builds a fresh one per lookup.
func testSameAs(_ *Environment, _ *runtime.Context, value any, args []any, _ map[string]any) (bool, error) {
	if len(args) < 1 {
		return false, nil
	}
	return sameAs(value, args[0]), nil
}

func sameAs(a, b any) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if runtime.IsUndefined(a) || runtime.IsUndefined(b) {
		return false
	}
	ta, tb := reflect.TypeOf(a), reflect.TypeOf(b)
	if ta != tb {
		return false
	}
	va, vb := reflect.ValueOf(a), reflect.ValueOf(b)
	switch ta.Kind() {
	case reflect.Slice:
		return va.Pointer() == vb.Pointer() && va.Len() == vb.Len()
	case reflect.Map, reflect.Func:
		return va.Pointer() == vb.Pointer()
	}
	// Value.Comparable also rejects structs holding uncomparable
	// dynamic values, where `==` would panic.
	return va.Comparable() && va.Equal(vb)
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
	// Python's Undefined defines __call__ (which raises when invoked).
	if runtime.IsUndefined(value) {
		return true, nil
	}
	switch value.(type) {
	case *runtime.Callable, *runtime.Joiner, *runtime.LoopContext, *runtime.BlockReference:
		return true, nil
	}
	rv := reflect.ValueOf(value)
	return rv.Kind() == reflect.Func, nil
}

// testEscaped implements the `escaped` test: true when the value is
// already [escape.Markup] (so autoescape will pass it through verbatim).
//
// Signature: x is escaped
func testEscaped(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	// ChainableUndefined is the one Undefined that defines __html__.
	if u, ok := value.(runtime.Undefined); ok {
		return u.Mode == runtime.ModeChainable, nil
	}
	_, ok := value.(escape.Markup)
	return ok, nil
}

// testIsFilter implements the `filter` test: true when the value is the
// name of a registered filter on the environment.
//
// Signature: name is filter
func testIsFilter(env *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	if err := unhashable(value); err != nil {
		return false, err
	}
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
	if err := unhashable(value); err != nil {
		return false, err
	}
	name, ok := value.(string)
	if !ok {
		return false, nil
	}
	_, exists := env.tests[name]
	return exists, nil
}
