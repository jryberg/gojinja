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

// registerBuiltins seeds an Environment with a minimal set of filters,
// tests, and globals so basic templates render. Full coverage is added
// in Phase 12 / 13 / 14.
func registerBuiltins(e *Environment) {
	// ------------------------------------------------------- globals
	e.globals["range"] = func(args ...int) ([]any, error) {
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
	// dict() accepts an optional mapping arg, alternating key/value
	// positional pairs, and keyword arguments — mirrors Python.
	e.globals["dict"] = KwargsCallable(func(args []any, kwargs map[string]any) (any, error) {
		out := runtime.NewOrderedDict()
		if len(args) == 1 {
			switch x := args[0].(type) {
			case *runtime.OrderedDict:
				for _, k := range x.Keys() {
					v, _ := x.Get(k)
					out.Set(k, v)
				}
			case map[string]any:
				for k, v := range x {
					out.Set(k, v)
				}
			case map[any]any:
				for k, v := range x {
					out.Set(k, v)
				}
			default:
				return nil, gjerrors.NewTemplateRuntimeError(fmt.Sprintf("dict() argument must be a mapping or alternating pairs, got %T", x))
			}
		} else {
			if len(args)%2 != 0 {
				return nil, gjerrors.NewTemplateRuntimeError("dict() requires alternating key/value")
			}
			for i := 0; i < len(args); i += 2 {
				k, ok := args[i].(string)
				if !ok {
					return nil, gjerrors.NewTemplateRuntimeError("dict() keys must be strings")
				}
				out.Set(k, args[i+1])
			}
		}
		// Iterate kwargs in deterministic key order. Go's map iteration is
		// randomized, so a naive `for k := range kwargs` produces flaky
		// `dict(a=1, b=2) | items` output. Python preserves the call-site
		// order; gojinja currently flattens kwargs into map[string]any at
		// the call boundary, so we settle for alphabetic — deterministic
		// and right whenever the call site already sorts that way.
		ks := make([]string, 0, len(kwargs))
		for k := range kwargs {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		for _, k := range ks {
			out.Set(k, kwargs[k])
		}
		return out, nil
	})
	// namespace() / namespace(a=1, b=2). Python supports either an
	// explicit dict-positional argument or keyword arguments. We accept
	// the dict-arg via Call's variadic-any binding and the kwargs via
	// the NamespaceFromKwargs adapter (see runtime.NewNamespace).
	e.globals["namespace"] = namespaceCtor
	e.globals["cycler"] = func(items ...any) *runtime.Cycler { return runtime.NewCycler(items...) }
	e.globals["joiner"] = func(seps ...string) *runtime.Joiner {
		s := ", "
		if len(seps) > 0 {
			s = seps[0]
		}
		return runtime.NewJoiner(s)
	}
	e.globals["lipsum"] = func(args ...int) any {
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

// namespaceCtor implements the `namespace()` global. Accepts an
// optional positional dict (Python: `namespace({'a': 1})`) and / or
// keyword arguments (`namespace(a=1, b=2)`). Both forms seed the
// resulting Namespace.
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

func filterUpper(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (any, error) {
	return strings.ToUpper(escape.SoftStr(value)), nil
}
func filterLower(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (any, error) {
	return strings.ToLower(escape.SoftStr(value)), nil
}
func filterTitle(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (any, error) {
	// Jinja2 splits on `-`, whitespace, `(`, `{`, `[`, `<` and lowercases
	// the rest of each word. Apostrophes do NOT split words, so
	// `"foo's bar".title()` becomes `"Foo's Bar"` (unlike Python's
	// builtin `str.title()`).
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
func filterCapitalize(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (any, error) {
	s := escape.SoftStr(value)
	if s == "" {
		return s, nil
	}
	return strings.ToUpper(s[:1]) + strings.ToLower(s[1:]), nil
}
func filterTrim(_ *Environment, _ *runtime.Context, value any, args []any, _ map[string]any) (any, error) {
	s := escape.SoftStr(value)
	if len(args) > 0 {
		if cut, ok := args[0].(string); ok {
			return strings.Trim(s, cut), nil
		}
	}
	return strings.TrimSpace(s), nil
}
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
func filterString(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (any, error) {
	return escape.SoftStr(value), nil
}
func filterSafe(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (any, error) {
	return escape.Markup(escape.SoftStr(value)), nil
}
func filterEscape(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (any, error) {
	return escape.Escape(value), nil
}
func filterForceEscape(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (any, error) {
	return escape.ForceEscape(value), nil
}
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
func filterInt(_ *Environment, _ *runtime.Context, value any, args []any, kwargs map[string]any) (any, error) {
	// Python signature: int(default=0, base=10)
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
func filterFirst(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (any, error) {
	if x, ok := value.([]any); ok {
		if len(x) == 0 {
			return runtime.NewBase("", "first", value, nil), nil
		}
		return x[0], nil
	}
	return runtime.NewBase("", "first", value, nil), nil
}
func filterLast(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (any, error) {
	if x, ok := value.([]any); ok {
		if len(x) == 0 {
			return runtime.NewBase("", "last", value, nil), nil
		}
		return x[len(x)-1], nil
	}
	return runtime.NewBase("", "last", value, nil), nil
}
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

func filterTruncate(_ *Environment, _ *runtime.Context, value any, args []any, kwargs map[string]any) (any, error) {
	// Python signature: truncate(s, length=255, killwords=False,
	//                             end='...', leeway=None).
	// `leeway` defaults to env.policies['truncate.leeway'] = 5 — short
	// inputs that exceed `length` by less than `leeway` are returned
	// unchanged.
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

// filterFormat implements `'%s %d' | format('a', 1)` style. Supports a
// subset of Python's printf-style: %s %d %f %x %o %b %% and %05d-style
// width/precision specifiers via fmt.Sprintf.
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

// filterMap supports two forms:
//   - {{ items | map('upper') }}            apply named filter to each
//   - {{ items | map(attribute='name') }}   extract attribute from each
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

func filterSelect(env *Environment, ctx *runtime.Context, value any, args []any, _ map[string]any) (any, error) {
	return selectReject(env, ctx, value, args, true, "")
}
func filterReject(env *Environment, ctx *runtime.Context, value any, args []any, _ map[string]any) (any, error) {
	return selectReject(env, ctx, value, args, false, "")
}
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

func testDefined(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	if u, ok := value.(runtime.Undefiner); ok && u.IsUndefined() {
		return false, nil
	}
	return true, nil
}
func testUndefined(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	if u, ok := value.(runtime.Undefiner); ok && u.IsUndefined() {
		return true, nil
	}
	return false, nil
}
func testNone(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	return value == nil, nil
}
func testFalse(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	return value == false, nil
}
func testTrue(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	return value == true, nil
}
func testBoolean(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	_, ok := value.(bool)
	return ok, nil
}
func testString(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	_, ok := value.(string)
	if ok {
		return true, nil
	}
	_, ok = value.(escape.Markup)
	return ok, nil
}
func testNumber(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	switch value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return true, nil
	}
	return false, nil
}
func testInteger(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	switch value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return true, nil
	}
	return false, nil
}
func testFloat(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	switch value.(type) {
	case float32, float64:
		return true, nil
	}
	return false, nil
}
func testOdd(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	if n, ok := asInt(value); ok {
		return n%2 != 0, nil
	}
	return false, nil
}
func testEven(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	if n, ok := asInt(value); ok {
		return n%2 == 0, nil
	}
	return false, nil
}
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
func testSequence(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	// Python's `is sequence`: anything with __len__ and __getitem__.
	// That includes dicts, lists, tuples, and strings.
	switch value.(type) {
	case []any, string, runtime.Tuple, map[string]any, map[any]any, *runtime.OrderedDict:
		return true, nil
	}
	return false, nil
}
func testMapping(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	switch value.(type) {
	case map[string]any, map[any]any, *runtime.OrderedDict:
		return true, nil
	}
	return false, nil
}
func testIterable(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	switch value.(type) {
	case []any, string, map[string]any, map[any]any, *runtime.OrderedDict:
		return true, nil
	}
	return false, nil
}
func testLowerStr(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	if s, ok := value.(string); ok {
		return s == strings.ToLower(s) && containsLetter(s), nil
	}
	return false, nil
}
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

func testEq(_ *Environment, _ *runtime.Context, value any, args []any, _ map[string]any) (bool, error) {
	if len(args) < 1 {
		return false, nil
	}
	return equalAny(value, args[0]), nil
}
func testNe(_ *Environment, _ *runtime.Context, value any, args []any, _ map[string]any) (bool, error) {
	if len(args) < 1 {
		return false, nil
	}
	return !equalAny(value, args[0]), nil
}
func testLt(_ *Environment, _ *runtime.Context, value any, args []any, _ map[string]any) (bool, error) {
	if len(args) < 1 {
		return false, nil
	}
	return compareLess(value, args[0]), nil
}
func testLtEq(_ *Environment, _ *runtime.Context, value any, args []any, _ map[string]any) (bool, error) {
	if len(args) < 1 {
		return false, nil
	}
	return compareLess(value, args[0]) || equalAny(value, args[0]), nil
}
func testGt(_ *Environment, _ *runtime.Context, value any, args []any, _ map[string]any) (bool, error) {
	if len(args) < 1 {
		return false, nil
	}
	return !compareLess(value, args[0]) && !equalAny(value, args[0]), nil
}
func testGtEq(_ *Environment, _ *runtime.Context, value any, args []any, _ map[string]any) (bool, error) {
	if len(args) < 1 {
		return false, nil
	}
	return !compareLess(value, args[0]), nil
}
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
func testSameAs(_ *Environment, _ *runtime.Context, value any, args []any, _ map[string]any) (bool, error) {
	if len(args) < 1 {
		return false, nil
	}
	return value == args[0], nil
}

// testCallable returns true when value is invokable (a Go function, a
// macro, an OrderedDict's bound method, etc.). Mirrors Python's
// `callable()` builtin.
func testCallable(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	if value == nil {
		return false, nil
	}
	rv := reflect.ValueOf(value)
	return rv.Kind() == reflect.Func, nil
}

// testEscaped returns true when value is already a Markup — i.e. the
// runtime treats it as safe HTML and won't double-escape on output.
func testEscaped(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	_, ok := value.(escape.Markup)
	return ok, nil
}

// testIsFilter returns true when name is a registered filter on the env.
func testIsFilter(env *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (bool, error) {
	name, ok := value.(string)
	if !ok {
		return false, nil
	}
	_, exists := env.filters[name]
	return exists, nil
}

// testIsTest returns true when name is a registered test on the env.
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
