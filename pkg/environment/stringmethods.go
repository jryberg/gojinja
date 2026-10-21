package environment

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/jryberg/gojinja/internal/pyunicode"
	gjerrors "github.com/jryberg/gojinja/pkg/errors"
	"github.com/jryberg/gojinja/pkg/escape"
	"github.com/jryberg/gojinja/pkg/eval"
	"github.com/jryberg/gojinja/pkg/runtime"
)

// stringMethod returns Python's str method attr bound to s, or nil when
// str has no such method (format and format_map live in strformat.go).
// Methods take their arguments like CPython's: argument counts, keyword
// support, types and the resulting TypeError/ValueError messages follow
// the C implementations, and indices count characters.
func stringMethod(s, attr string) any {
	spec, ok := strMethodSpecs[attr]
	if !ok {
		return nil
	}
	return KwargsCallable(func(args []any, kwargs map[string]any) (any, error) {
		args, err := spec.bind("str", attr, args, kwargs)
		if err != nil {
			return nil, gjerrors.NewTemplateRuntimeError(err.Error())
		}
		v, err := spec.call(s, args)
		if err != nil {
			if _, ok := gjerrors.AsTemplateError(err); ok {
				return nil, err
			}
			return nil, gjerrors.NewTemplateRuntimeError(err.Error())
		}
		return v, nil
	})
}

// argStyle selects which of CPython's argument-error wordings a method
// uses; they differ by how the C method parses its arguments.
type argStyle int

const (
	styleNoArgs     argStyle = iota // str.upper() takes no arguments (1 given)
	styleExpected                   // center expected at least 1 argument, got 0
	styleTakes                      // find() takes at least 1 argument (0 given)
	styleExactlyOne                 // str.join() takes exactly one argument (0 given)
	styleKeywords                   // split() takes at most 2 arguments (3 given)
)

type strMethodSpec struct {
	style    argStyle
	min, max int
	keywords []string // parameter names, for methods that accept keywords
	posOnly  int      // leading keywords that must be passed positionally
	call     func(s string, args []any) (any, error)
}

// bind checks the call shape and returns the positional arguments, with
// keyword arguments moved into their positions (missing optional ones
// are nil).
// typ is the receiver's Python type ("str" or "bytes").
func (m strMethodSpec) bind(typ, name string, args []any, kwargs map[string]any) ([]any, error) {
	n := len(args)
	if len(kwargs) > 0 && m.style != styleKeywords {
		if m.style == styleTakes {
			return nil, fmt.Errorf("%s() takes no keyword arguments", name)
		}
		return nil, fmt.Errorf("%s.%s() takes no keyword arguments", typ, name)
	}
	switch m.style {
	case styleNoArgs:
		if n > 0 {
			return nil, fmt.Errorf("%s.%s() takes no arguments (%d given)", typ, name, n)
		}
	case styleExactlyOne:
		if n != 1 {
			return nil, fmt.Errorf("%s.%s() takes exactly one argument (%d given)", typ, name, n)
		}
	case styleExpected:
		if m.min == m.max && n != m.min {
			return nil, fmt.Errorf("%s expected %d argument%s, got %d", name, m.min, plural(m.min), n)
		}
		if n < m.min {
			return nil, fmt.Errorf("%s expected at least %d argument%s, got %d", name, m.min, plural(m.min), n)
		}
		if n > m.max {
			return nil, fmt.Errorf("%s expected at most %d argument%s, got %d", name, m.max, plural(m.max), n)
		}
	case styleTakes:
		if n < m.min {
			return nil, fmt.Errorf("%s() takes at least %d argument%s (%d given)", name, m.min, plural(m.min), n)
		}
		if n > m.max {
			return nil, fmt.Errorf("%s() takes at most %d argument%s (%d given)", name, m.max, plural(m.max), n)
		}
	case styleKeywords:
		if n < m.posOnly {
			return nil, fmt.Errorf("%s() takes at least %d positional argument%s (%d given)", name, m.posOnly, plural(m.posOnly), n)
		}
		if n+len(kwargs) > m.max {
			return nil, fmt.Errorf("%s() takes at most %d argument%s (%d given)", name, m.max, plural(m.max), n+len(kwargs))
		}
		bound := make([]any, len(m.keywords))
		copy(bound, args)
		for k, v := range kwargs {
			i := -1
			for j, kw := range m.keywords {
				if kw == k {
					i = j
				}
			}
			if i < m.posOnly {
				return nil, fmt.Errorf("'%s' is an invalid keyword argument for %s()", k, name)
			}
			if i < n {
				return nil, fmt.Errorf("argument for %s() given by name ('%s') and position (%d)", name, k, i+1)
			}
			bound[i] = v
		}
		return bound, nil
	}
	bound := make([]any, max(m.max, n))
	copy(bound, args)
	return bound, nil
}

func noArgs(f func(string) any) strMethodSpec {
	return strMethodSpec{style: styleNoArgs, call: func(s string, _ []any) (any, error) { return f(s), nil }}
}

func predicate(f func(string) bool) strMethodSpec {
	return noArgs(func(s string) any { return f(s) })
}

var strMethodSpecs map[string]strMethodSpec

func init() {
	strMethodSpecs = map[string]strMethodSpec{
		"capitalize": noArgs(func(s string) any { return pyCapitalize(s) }),
		"casefold":   noArgs(func(s string) any { return pyCasefold(s) }),
		"lower":      noArgs(func(s string) any { return pyLower(s) }),
		"upper":      noArgs(func(s string) any { return pyUpper(s) }),
		"swapcase":   noArgs(func(s string) any { return pySwapcase(s) }),
		"title":      noArgs(func(s string) any { return pyTitle(s) }),

		"isalnum":      predicate(func(s string) bool { return allRunes(s, pyunicode.IsAlnum) }),
		"isalpha":      predicate(func(s string) bool { return allRunes(s, pyunicode.IsAlpha) }),
		"isascii":      predicate(func(s string) bool { return s == "" || allRunes(s, func(r rune) bool { return r < 0x80 }) }),
		"isdecimal":    predicate(func(s string) bool { return allRunes(s, pyunicode.IsDecimal) }),
		"isdigit":      predicate(func(s string) bool { return allRunes(s, pyunicode.IsDigit) }),
		"isidentifier": predicate(pyunicode.IsIdentifier),
		"islower":      predicate(pyIsLower),
		"isnumeric":    predicate(func(s string) bool { return allRunes(s, pyunicode.IsNumeric) }),
		"isprintable":  predicate(func(s string) bool { return s == "" || allRunes(s, pyunicode.IsPrintable) }),
		"isspace":      predicate(func(s string) bool { return allRunes(s, pyunicode.IsSpace) }),
		"istitle":      predicate(pyIsTitle),
		"isupper":      predicate(pyIsUpper),

		"center": {style: styleExpected, min: 1, max: 2, call: padMethod("center")},
		"ljust":  {style: styleExpected, min: 1, max: 2, call: padMethod("ljust")},
		"rjust":  {style: styleExpected, min: 1, max: 2, call: padMethod("rjust")},
		"strip":  {style: styleExpected, max: 1, call: stripMethod("strip", true, true)},
		"lstrip": {style: styleExpected, max: 1, call: stripMethod("lstrip", true, false)},
		"rstrip": {style: styleExpected, max: 1, call: stripMethod("rstrip", false, true)},
		"replace": {style: styleExpected, min: 2, max: 3, call: func(s string, a []any) (any, error) {
			oldS, err := strArg("replace() argument 1", a[0])
			if err != nil {
				return nil, err
			}
			newS, err := strArg("replace() argument 2", a[1])
			if err != nil {
				return nil, err
			}
			count := -1
			if a[2] != nil {
				if count, err = strIndexArg(a[2]); err != nil {
					return nil, err
				}
			}
			return pyReplace(s, oldS, newS, count), nil
		}},
		"maketrans": {style: styleExpected, min: 1, max: 3, call: func(_ string, a []any) (any, error) {
			return pyMaketrans(a[0], a[1], a[2])
		}},

		"count":      {style: styleTakes, min: 1, max: 3, call: searchMethod("count")},
		"find":       {style: styleTakes, min: 1, max: 3, call: searchMethod("find")},
		"rfind":      {style: styleTakes, min: 1, max: 3, call: searchMethod("rfind")},
		"index":      {style: styleTakes, min: 1, max: 3, call: searchMethod("index")},
		"rindex":     {style: styleTakes, min: 1, max: 3, call: searchMethod("rindex")},
		"startswith": {style: styleTakes, min: 1, max: 3, call: affixMethod("startswith")},
		"endswith":   {style: styleTakes, min: 1, max: 3, call: affixMethod("endswith")},

		"join": {style: styleExactlyOne, call: func(s string, a []any) (any, error) {
			items, err := eval.ToIterable(a[0])
			if err != nil || a[0] == nil {
				return nil, fmt.Errorf("can only join an iterable")
			}
			parts := make([]string, len(items))
			for i, it := range items {
				str, ok := eqString(it)
				if !ok {
					return nil, fmt.Errorf("sequence item %d: expected str instance, %s found", i, runtime.PyTypeName(it))
				}
				parts[i] = str
			}
			return strings.Join(parts, s), nil
		}},
		"partition":  {style: styleExactlyOne, call: partitionMethod(false)},
		"rpartition": {style: styleExactlyOne, call: partitionMethod(true)},
		"removeprefix": {style: styleExactlyOne, call: func(s string, a []any) (any, error) {
			p, err := strArg("removeprefix() argument", a[0])
			if err != nil {
				return nil, err
			}
			return strings.TrimPrefix(s, p), nil
		}},
		"removesuffix": {style: styleExactlyOne, call: func(s string, a []any) (any, error) {
			p, err := strArg("removesuffix() argument", a[0])
			if err != nil {
				return nil, err
			}
			return strings.TrimSuffix(s, p), nil
		}},
		"translate": {style: styleExactlyOne, call: func(s string, a []any) (any, error) {
			return pyTranslate(s, a[0])
		}},
		"zfill": {style: styleExactlyOne, call: func(s string, a []any) (any, error) {
			width, err := strIndexArg(a[0])
			if err != nil {
				return nil, err
			}
			return pyZfill(s, width), nil
		}},

		"split":  {style: styleKeywords, max: 2, keywords: []string{"sep", "maxsplit"}, call: splitMethod(false)},
		"rsplit": {style: styleKeywords, max: 2, keywords: []string{"sep", "maxsplit"}, call: splitMethod(true)},
		"splitlines": {style: styleKeywords, max: 1, keywords: []string{"keepends"}, call: func(s string, a []any) (any, error) {
			lines := pySplitlines(s, a[0] != nil && eval.Truthy(a[0]))
			out := make([]any, len(lines))
			for i, l := range lines {
				out[i] = l
			}
			return out, nil
		}},
		"expandtabs": {style: styleKeywords, max: 1, keywords: []string{"tabsize"}, call: func(s string, a []any) (any, error) {
			tab := 8
			if a[0] != nil {
				var err error
				if tab, err = strIndexArg(a[0]); err != nil {
					return nil, err
				}
			}
			return pyExpandtabs(s, tab), nil
		}},
		"encode": {style: styleKeywords, max: 2, keywords: []string{"encoding", "errors"}, call: func(s string, a []any) (any, error) {
			encoding, errorsMode, err := codecArgs("encode", a)
			if err != nil {
				return nil, err
			}
			return pyEncode(s, encoding, errorsMode)
		}},
	}
}

// strArg requires a str (or Markup) argument.
func strArg(what string, v any) (string, error) {
	if s, ok := eqString(v); ok {
		return s, nil
	}
	return "", fmt.Errorf("%s must be str, not %s", what, runtime.PyTypeName(v))
}

// codecArgs reads the encoding and errors arguments of str.encode and
// bytes.decode, which CPython converts to C strings and so rejects when
// they contain a NUL.
func codecArgs(method string, a []any) (encoding, errorsMode string, err error) {
	encoding, errorsMode = "utf-8", "strict"
	for i, name := range []string{"encoding", "errors"} {
		if a[i] == nil {
			continue
		}
		v, err := strArg(fmt.Sprintf("%s() argument '%s'", method, name), a[i])
		if err != nil {
			return "", "", err
		}
		if strings.ContainsRune(v, 0) {
			return "", "", escape.ValueError{Msg: "embedded null character"}
		}
		if i == 0 {
			encoding = v
		} else {
			errorsMode = v
		}
	}
	return encoding, errorsMode, nil
}

func strIndexArg(v any) (int, error) {
	n, err := indexArg(v)
	if err != nil {
		return 0, fmt.Errorf("'%s' object cannot be interpreted as an integer", runtime.PyTypeName(v))
	}
	return n, nil
}

// sliceBounds converts optional start/end arguments the way CPython's
// ADJUST_INDICES does for a string of n characters.
func sliceBounds(n int, startArg, endArg any) (int, int, error) {
	start, end := 0, n
	for i, v := range []any{startArg, endArg} {
		if v == nil {
			continue
		}
		x, err := indexArg(v)
		if err != nil {
			return 0, 0, fmt.Errorf("slice indices must be integers or None or have an __index__ method")
		}
		if i == 0 {
			start = x
		} else {
			end = x
		}
	}
	if end > n {
		end = n
	} else if end < 0 {
		end = max(end+n, 0)
	}
	if start < 0 {
		start = max(start+n, 0)
	}
	return start, end, nil
}

func searchMethod(name string) func(string, []any) (any, error) {
	return func(s string, a []any) (any, error) {
		sub, ok := eqString(a[0])
		if !ok {
			return nil, fmt.Errorf("must be str, not %s", runtime.PyTypeName(a[0]))
		}
		rs, subRs := []rune(s), []rune(sub)
		start, end, err := sliceBounds(len(rs), a[1], a[2])
		if err != nil {
			return nil, err
		}
		if name == "count" {
			if end-start < len(subRs) {
				return int64(0), nil
			}
			if len(subRs) == 0 {
				return int64(end - start + 1), nil
			}
			return int64(strings.Count(string(rs[start:end]), sub)), nil
		}
		idx := -1
		if end-start >= len(subRs) {
			window := string(rs[start:end])
			var b int
			if name == "find" || name == "index" {
				b = strings.Index(window, sub)
			} else {
				b = strings.LastIndex(window, sub)
			}
			if b >= 0 {
				idx = start + utf8.RuneCountInString(window[:b])
			}
		}
		if idx < 0 && (name == "index" || name == "rindex") {
			return nil, fmt.Errorf("substring not found")
		}
		return int64(idx), nil
	}
}

func affixMethod(name string) func(string, []any) (any, error) {
	return func(s string, a []any) (any, error) {
		affixes := []any{a[0]}
		tuple, isTuple := a[0].(runtime.Tuple)
		if isTuple {
			affixes = tuple
		} else if _, ok := eqString(a[0]); !ok {
			return nil, fmt.Errorf("%s first arg must be str or a tuple of str, not %s", name, runtime.PyTypeName(a[0]))
		}
		rs := []rune(s)
		start, end, err := sliceBounds(len(rs), a[1], a[2])
		if err != nil {
			return nil, err
		}
		// Like CPython, tuple items are checked one by one, stopping at the
		// first match.
		for _, it := range affixes {
			affix, ok := eqString(it)
			if !ok {
				return nil, fmt.Errorf("tuple for %s must only contain str, not %s", name, runtime.PyTypeName(it))
			}
			n := utf8.RuneCountInString(affix)
			if end-n < start {
				continue
			}
			if name == "startswith" && string(rs[start:start+n]) == affix ||
				name == "endswith" && string(rs[end-n:end]) == affix {
				return true, nil
			}
		}
		return false, nil
	}
}

func padMethod(name string) func(string, []any) (any, error) {
	return func(s string, a []any) (any, error) {
		width, err := strIndexArg(a[0])
		if err != nil {
			return nil, err
		}
		fill := " "
		if a[1] != nil {
			f, ok := eqString(a[1])
			if !ok {
				//lint:ignore ST1005 CPython's exact message
				return nil, fmt.Errorf("The fill character must be a unicode character, not %s", runtime.PyTypeName(a[1]))
			}
			if utf8.RuneCountInString(f) != 1 {
				//lint:ignore ST1005 CPython's exact message
				return nil, fmt.Errorf("The fill character must be exactly one character long")
			}
			fill = f
		}
		n := utf8.RuneCountInString(s)
		if n >= width {
			return s, nil
		}
		switch name {
		case "ljust":
			return s + strings.Repeat(fill, width-n), nil
		case "rjust":
			return strings.Repeat(fill, width-n) + s, nil
		}
		return pyCenter(s, width, fill), nil
	}
}

// pyCenter is str.center: widths count characters, and an odd margin
// puts the extra fill on the left when width is odd (CPython's
// `left = marg/2 + (marg & width & 1)`).
func pyCenter(s string, width int, fill string) string {
	n := utf8.RuneCountInString(s)
	if n >= width {
		return s
	}
	marg := width - n
	left := marg/2 + (marg & width & 1)
	return strings.Repeat(fill, left) + s + strings.Repeat(fill, marg-left)
}

func stripMethod(name string, left, right bool) func(string, []any) (any, error) {
	return func(s string, a []any) (any, error) {
		cut := pyunicode.IsSpace
		if a[0] != nil {
			chars, ok := eqString(a[0])
			if !ok {
				return nil, fmt.Errorf("%s arg must be None or str", name)
			}
			cut = func(r rune) bool { return strings.ContainsRune(chars, r) }
		}
		if left {
			s = strings.TrimLeftFunc(s, cut)
		}
		if right {
			s = strings.TrimRightFunc(s, cut)
		}
		return s, nil
	}
}

func partitionMethod(fromRight bool) func(string, []any) (any, error) {
	return func(s string, a []any) (any, error) {
		sep, ok := eqString(a[0])
		if !ok {
			return nil, fmt.Errorf("must be str, not %s", runtime.PyTypeName(a[0]))
		}
		if sep == "" {
			return nil, fmt.Errorf("empty separator")
		}
		i := strings.Index(s, sep)
		if fromRight {
			i = strings.LastIndex(s, sep)
		}
		switch {
		case i >= 0:
			return runtime.Tuple{s[:i], sep, s[i+len(sep):]}, nil
		case fromRight:
			return runtime.Tuple{"", "", s}, nil
		}
		return runtime.Tuple{s, "", ""}, nil
	}
}

func splitMethod(fromRight bool) func(string, []any) (any, error) {
	return func(s string, a []any) (any, error) {
		limit := -1
		if a[1] != nil {
			var err error
			if limit, err = strIndexArg(a[1]); err != nil {
				return nil, err
			}
		}
		var parts []string
		if a[0] == nil {
			parts = splitWhitespace(s, limit, fromRight)
		} else {
			sep, ok := eqString(a[0])
			if !ok {
				return nil, fmt.Errorf("must be str or None, not %s", runtime.PyTypeName(a[0]))
			}
			if sep == "" {
				return nil, fmt.Errorf("empty separator")
			}
			parts = splitSep(s, sep, limit, fromRight)
		}
		out := make([]any, len(parts))
		for i, p := range parts {
			out[i] = p
		}
		return out, nil
	}
}

// splitWhitespace is CPython's split_whitespace / rsplit_whitespace: runs
// of whitespace separate fields, and after limit splits the remainder is
// kept with only its leading (or, from the right, trailing) whitespace
// removed.
func splitWhitespace(s string, limit int, fromRight bool) []string {
	rs := []rune(s)
	var parts []string
	if !fromRight {
		i := 0
		for limit != 0 {
			for i < len(rs) && pyunicode.IsSpace(rs[i]) {
				i++
			}
			if i == len(rs) {
				break
			}
			j := i
			for i < len(rs) && !pyunicode.IsSpace(rs[i]) {
				i++
			}
			parts = append(parts, string(rs[j:i]))
			limit--
		}
		for i < len(rs) && pyunicode.IsSpace(rs[i]) {
			i++
		}
		if i < len(rs) {
			parts = append(parts, string(rs[i:]))
		}
		return parts
	}
	i := len(rs) - 1
	for limit != 0 {
		for i >= 0 && pyunicode.IsSpace(rs[i]) {
			i--
		}
		if i < 0 {
			break
		}
		j := i
		for i >= 0 && !pyunicode.IsSpace(rs[i]) {
			i--
		}
		parts = append(parts, string(rs[i+1:j+1]))
		limit--
	}
	for i >= 0 && pyunicode.IsSpace(rs[i]) {
		i--
	}
	if i >= 0 {
		parts = append(parts, string(rs[:i+1]))
	}
	for l, r := 0, len(parts)-1; l < r; l, r = l+1, r-1 {
		parts[l], parts[r] = parts[r], parts[l]
	}
	return parts
}

func splitSep(s, sep string, limit int, fromRight bool) []string {
	if limit < 0 {
		return strings.Split(s, sep)
	}
	if !fromRight {
		return strings.SplitN(s, sep, limit+1)
	}
	var parts []string
	for ; limit > 0; limit-- {
		i := strings.LastIndex(s, sep)
		if i < 0 {
			break
		}
		parts = append([]string{s[i+len(sep):]}, parts...)
		s = s[:i]
	}
	return append([]string{s}, parts...)
}

// pyReplace is str.replace; an empty old inserts new around every
// character.
func pyReplace(s, oldS, newS string, count int) string {
	if oldS != "" {
		return strings.Replace(s, oldS, newS, count)
	}
	rs := []rune(s)
	var b strings.Builder
	n := 0
	for i := 0; i <= len(rs); i++ {
		if count < 0 || n < count {
			b.WriteString(newS)
			n++
		}
		if i < len(rs) {
			b.WriteRune(rs[i])
		}
	}
	return b.String()
}

// pyZfill pads with zeros after any leading sign.
func pyZfill(s string, width int) string {
	rs := []rune(s)
	if len(rs) >= width {
		return s
	}
	fill := width - len(rs)
	out := []rune(strings.Repeat("0", fill) + s)
	if len(rs) > 0 && (rs[0] == '+' || rs[0] == '-') {
		out[0], out[fill] = rs[0], '0'
	}
	return string(out)
}

// pyExpandtabs replaces tabs with spaces up to the next multiple of
// tabsize, restarting the column at each newline or carriage return.
func pyExpandtabs(s string, tabsize int) string {
	var b strings.Builder
	col := 0
	for _, r := range s {
		switch r {
		case '\t':
			if tabsize > 0 {
				n := tabsize - col%tabsize
				b.WriteString(strings.Repeat(" ", n))
				col += n
			}
		case '\n', '\r':
			b.WriteRune(r)
			col = 0
		default:
			b.WriteRune(r)
			col++
		}
	}
	return b.String()
}

// pyTranslate is str.translate: each character's code point is looked up
// in table; None deletes it, an int or str replaces it, and a missing key
// keeps it.
func pyTranslate(s string, table any) (string, error) {
	var b strings.Builder
	for _, r := range s {
		v, found, err := translateLookup(table, int64(r))
		if err != nil {
			return "", err
		}
		if !found {
			b.WriteRune(r)
			continue
		}
		switch x := v.(type) {
		case nil:
		case string:
			b.WriteString(x)
		case escape.Markup:
			b.WriteString(string(x))
		default:
			n, err := indexArg(v)
			if err != nil {
				return "", fmt.Errorf("character mapping must return integer, None or str")
			}
			if n < 0 || n > 0x10ffff {
				return "", fmt.Errorf("character mapping must be in range(0x110000)")
			}
			b.WriteRune(rune(n))
		}
	}
	return b.String(), nil
}

// translateLookup is table[key]: a missing key or index means "keep the
// character"; a table that cannot be subscripted raises.
func translateLookup(table any, key int64) (any, bool, error) {
	switch t := table.(type) {
	case *runtime.OrderedDict:
		v, ok := t.Get(key)
		return v, ok, nil
	case map[any]any:
		v, ok := t[key]
		return v, ok, nil
	case map[string]any:
		return nil, false, nil
	case []any:
		if key < int64(len(t)) {
			return t[key], true, nil
		}
		return nil, false, nil
	case *runtime.PyList:
		return translateLookup(t.Items(), key)
	case runtime.Tuple:
		return translateLookup([]any(t), key)
	case string:
		if rs := []rune(t); key < int64(len(rs)) {
			return string(rs[key]), true, nil
		}
		return nil, false, nil
	}
	return nil, false, fmt.Errorf("'%s' object is not subscriptable", runtime.PyTypeName(table))
}

// pyMaketrans is str.maketrans: one dict argument (keys may be single
// characters), or two equal-length strings plus optional characters to
// delete. The result maps code points.
func pyMaketrans(x, y, z any) (any, error) {
	out := runtime.NewOrderedDict()
	if y == nil {
		var items []any
		switch m := x.(type) {
		case *runtime.OrderedDict:
			items = m.Items()
		case map[string]any, map[any]any:
			items = mapItemsSorted(m)
		default:
			return nil, fmt.Errorf("if you give only one argument to maketrans it must be a dict")
		}
		for _, it := range items {
			kv := it.(runtime.Tuple)
			switch k := kv[0].(type) {
			case string:
				if utf8.RuneCountInString(k) != 1 {
					return nil, fmt.Errorf("string keys in translate table must be of length 1")
				}
				r, _ := utf8.DecodeRuneInString(k)
				out.Set(int64(r), kv[1])
			default:
				n, err := indexArg(k)
				if err != nil {
					return nil, fmt.Errorf("keys in translate table must be strings or integers")
				}
				out.Set(int64(n), kv[1])
			}
		}
		return out, nil
	}
	xs, ok1 := eqString(x)
	ys, ok2 := eqString(y)
	if !ok1 || !ok2 {
		return nil, fmt.Errorf("maketrans() argument must be str, not %s", runtime.PyTypeName(x))
	}
	xr, yr := []rune(xs), []rune(ys)
	if len(xr) != len(yr) {
		return nil, fmt.Errorf("the first two maketrans arguments must have equal length")
	}
	for i := range xr {
		out.Set(int64(xr[i]), int64(yr[i]))
	}
	if z != nil {
		zs, ok := eqString(z)
		if !ok {
			return nil, fmt.Errorf("maketrans() argument 3 must be str, not %s", runtime.PyTypeName(z))
		}
		for _, r := range zs {
			out.Set(int64(r), nil)
		}
	}
	return out, nil
}

func mapItemsSorted(m any) []any {
	keys, _ := eval.ToIterable(m)
	items := make([]any, len(keys))
	for i, k := range keys {
		var v any
		switch mm := m.(type) {
		case map[string]any:
			v = mm[k.(string)]
		case map[any]any:
			v = mm[k]
		}
		items[i] = runtime.Tuple{k, v}
	}
	return items
}

// pySplitlines is str.splitlines: it breaks on \n, \r, \r\n, \v, \f,
// \x1c-\x1e, \x85, \u2028 and \u2029, keeping the breaks when keep is
// set.
func pySplitlines(s string, keep bool) []string {
	var out []string
	rs := []rune(s)
	start := 0
	for i := 0; i < len(rs); i++ {
		switch rs[i] {
		case '\n', '\r', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
		default:
			continue
		}
		end := i + 1
		if rs[i] == '\r' && end < len(rs) && rs[end] == '\n' {
			end++
		}
		if keep {
			out = append(out, string(rs[start:end]))
		} else {
			out = append(out, string(rs[start:i]))
		}
		start = end
		i = end - 1
	}
	if start < len(rs) {
		out = append(out, string(rs[start:]))
	}
	return out
}
