package environment

import (
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"

	gjerrors "github.com/jryberg/gojinja/pkg/errors"
	"github.com/jryberg/gojinja/pkg/escape"
	"github.com/jryberg/gojinja/pkg/runtime"
	"github.com/jryberg/gojinja/pkg/sandbox"
)

// strFormatMethod returns the bound `format` / `format_map` method of a
// str or Markup receiver, or nil for any other attribute. It mirrors
// Jinja2's SandboxedEnvironment.wrap_str_format: field lookups inside the
// format string go through the environment's (sandboxed) attribute and
// item access, and a Markup receiver escapes every substituted field.
func (e *Environment) strFormatMethod(recv any, attr string) any {
	if attr != "format" && attr != "format_map" {
		return nil
	}
	var s string
	markup := false
	switch x := recv.(type) {
	case string:
		s = x
	case escape.Markup:
		s, markup = string(x), true
	default:
		return nil
	}
	isMap := attr == "format_map"
	return KwargsCallable(func(args []any, kwargs map[string]any) (any, error) {
		f := &strFormatter{env: e, escape: markup, args: args, kwargs: kwargs}
		if isMap {
			if len(kwargs) != 0 {
				return nil, gjerrors.NewTemplateRuntimeError("format_map() takes no keyword arguments")
			}
			if len(args) != 1 {
				return nil, gjerrors.NewTemplateRuntimeError(fmt.Sprintf("format_map() takes exactly one argument (%d given)", len(args)))
			}
			f.args, f.kwargs, f.mapping = nil, nil, args[0]
		}
		out, err := f.vformat(s, 2)
		if err != nil {
			return nil, err
		}
		if markup {
			return escape.Markup(out), nil
		}
		return out, nil
	})
}

// strFormatter implements Python's string.Formatter.vformat as subclassed
// by Jinja2's SandboxedFormatter (and SandboxedEscapeFormatter when
// escape is set).
type strFormatter struct {
	env     *Environment
	escape  bool
	args    []any
	kwargs  map[string]any
	mapping any // format_map argument; replaces kwargs when set

	// autoIdx is the next automatic field number; manual records that an
	// explicit numeric field was used (string.Formatter's
	// auto_arg_index == False).
	autoIdx int
	manual  bool
}

func formatValueError(msg string) error { return gjerrors.NewTemplateRuntimeError(msg) }

// vformat expands format string s. depth bounds nested replacement
// fields inside format specs, like CPython's recursion_depth of 2.
func (f *strFormatter) vformat(s string, depth int) (string, error) {
	if depth < 0 {
		return "", formatValueError("Max string recursion exceeded")
	}
	var b strings.Builder
	i := 0
	for i < len(s) {
		c := s[i]
		if c != '{' && c != '}' {
			b.WriteByte(c)
			i++
			continue
		}
		if c == '}' {
			if i+1 < len(s) && s[i+1] == '}' {
				b.WriteByte('}')
				i += 2
				continue
			}
			return "", formatValueError("Single '}' encountered in format string")
		}
		if i+1 >= len(s) {
			return "", formatValueError("Single '{' encountered in format string")
		}
		if s[i+1] == '{' {
			b.WriteByte('{')
			i += 2
			continue
		}
		// Find the matching '}', counting nested braces in the spec.
		start := i + 1
		count := 1
		j := start
		for ; j < len(s); j++ {
			if s[j] == '{' {
				count++
			} else if s[j] == '}' {
				count--
				if count == 0 {
					break
				}
			}
		}
		if count != 0 {
			return "", formatValueError("expected '}' before end of string")
		}
		out, err := f.replaceField(s[start:j], depth)
		if err != nil {
			return "", err
		}
		b.WriteString(out)
		i = j + 1
	}
	return b.String(), nil
}

// replaceField renders one `{field!conv:spec}` body.
func (f *strFormatter) replaceField(field string, depth int) (string, error) {
	name, conv, spec, err := splitField(field)
	if err != nil {
		return "", err
	}
	switch {
	case name == "":
		if f.manual {
			return "", formatValueError("cannot switch from manual field specification to automatic field numbering")
		}
		name = strconv.Itoa(f.autoIdx)
		f.autoIdx++
	case isASCIIDigits(name):
		if f.autoIdx > 0 {
			return "", formatValueError("cannot switch from automatic field numbering to manual field specification")
		}
		f.manual = true
	}
	obj, err := f.getField(name)
	if err != nil {
		return "", err
	}
	obj, err = convertField(obj, conv)
	if err != nil {
		return "", err
	}
	spec, err = f.vformat(spec, depth-1)
	if err != nil {
		return "", err
	}
	return f.formatField(obj, spec)
}

// splitField separates a replacement field into name, conversion and
// format spec, following CPython's parse_field.
func splitField(field string) (name string, conv byte, spec string, err error) {
	i := 0
	for i < len(field) {
		c := field[i]
		if c == '{' {
			return "", 0, "", formatValueError("unexpected '{' in field name")
		}
		if c == '[' {
			for i < len(field) && field[i] != ']' {
				i++
			}
			continue
		}
		if c == ':' || c == '!' {
			break
		}
		i++
	}
	name = field[:i]
	if i == len(field) {
		return name, 0, "", nil
	}
	rest := field[i+1:]
	if field[i] == '!' {
		if rest == "" {
			return "", 0, "", formatValueError("end of string while looking for conversion specifier")
		}
		conv = rest[0]
		rest = rest[1:]
		if rest != "" {
			if rest[0] != ':' {
				return "", 0, "", formatValueError("expected ':' after conversion specifier")
			}
			rest = rest[1:]
		}
	}
	return name, conv, rest, nil
}

// getField resolves `first(.attr|[key])*`, applying each step through the
// environment like SandboxedFormatter.get_field.
func (f *strFormatter) getField(name string) (any, error) {
	end := strings.IndexAny(name, ".[")
	if end < 0 {
		end = len(name)
	}
	obj, err := f.getValue(name[:end])
	if err != nil {
		return nil, err
	}
	rest := name[end:]
	unsafe := false
	for rest != "" {
		// Any lookup on an unsafe placeholder raises its SecurityError.
		if unsafe {
			return nil, obj.(runtime.Undefined).Fail()
		}
		switch rest[0] {
		case '.':
			rest = rest[1:]
			n := strings.IndexAny(rest, ".[")
			if n < 0 {
				n = len(rest)
			}
			attr := rest[:n]
			rest = rest[n:]
			if attr == "" {
				return nil, formatValueError("Empty attribute in format string")
			}
			if obj, unsafe, err = f.getAttr(obj, attr); err != nil {
				return nil, err
			}
		case '[':
			n := strings.IndexByte(rest, ']')
			if n < 0 {
				return nil, formatValueError("Missing ']' in format string")
			}
			key := rest[1:n]
			rest = rest[n+1:]
			if key == "" {
				return nil, formatValueError("Empty attribute in format string")
			}
			if rest != "" && rest[0] != '.' && rest[0] != '[' {
				return nil, formatValueError("Only '.' or '[' may follow ']' in format field specifier")
			}
			var arg any = key
			if isASCIIDigits(key) {
				idx, _ := strconv.Atoi(key)
				arg = idx
			}
			if obj, err = f.env.GetItem(obj, arg); err != nil {
				return nil, err
			}
		default:
			return nil, formatValueError("Only '.' or '[' may follow ']' in format field specifier")
		}
	}
	return obj, nil
}

// getAttr is the sandboxed attribute step. An unsafe attribute yields an
// Undefined that renders empty but raises SecurityError on further use,
// matching SandboxedEnvironment.unsafe_undefined; unsafe reports that.
func (f *strFormatter) getAttr(obj any, attr string) (v any, unsafe bool, err error) {
	if f.env.sandboxed && !sandbox.IsSafeAttribute(obj, attr) {
		msg := fmt.Sprintf("access to attribute %s of %s object is unsafe.", escape.Repr(attr), escape.Repr(pyTypeName(obj)))
		return f.env.undefined(msg, attr, obj, func(m string) error { return gjerrors.NewSecurityError(m) }), true, nil
	}
	v, err = f.env.GetAttr(obj, attr)
	return v, false, err
}

// getValue looks up the first part of a field: a positional index or a
// keyword / mapping key.
func (f *strFormatter) getValue(key string) (any, error) {
	if isASCIIDigits(key) {
		idx, err := strconv.Atoi(key)
		if err != nil || idx >= len(f.args) {
			return nil, gjerrors.NewTemplateRuntimeError("tuple index out of range")
		}
		return f.args[idx], nil
	}
	if f.mapping != nil {
		switch m := f.mapping.(type) {
		case *runtime.OrderedDict:
			if v, ok := m.Get(key); ok {
				return v, nil
			}
		case map[string]any:
			if v, ok := m[key]; ok {
				return v, nil
			}
		case map[any]any:
			if v, ok := m[key]; ok {
				return v, nil
			}
		default:
			return nil, gjerrors.NewTemplateRuntimeError(fmt.Sprintf("'%s' object is not subscriptable", pyTypeName(f.mapping)))
		}
	} else if v, ok := f.kwargs[key]; ok {
		return v, nil
	}
	return nil, gjerrors.NewTemplateRuntimeError(fmt.Sprintf("KeyError: %s", escape.Repr(key)))
}

// convertField applies a `!s`, `!r` or `!a` conversion.
func convertField(v any, conv byte) (any, error) {
	switch conv {
	case 0:
		return v, nil
	case 's':
		return pyStr(v)
	case 'r':
		return escape.Repr(v), nil
	case 'a':
		return pyASCII(escape.Repr(v)), nil
	}
	return nil, formatValueError(fmt.Sprintf("Unknown conversion specifier %c", conv))
}

// formatField is format(value, spec), escaped for a Markup receiver the
// way markupsafe's EscapeFormatter does.
func (f *strFormatter) formatField(v any, spec string) (string, error) {
	if !f.escape {
		return pyFormat(v, spec)
	}
	switch x := v.(type) {
	case escape.Markup:
		if spec != "" {
			return "", formatValueError("Unsupported format specification for Markup.")
		}
		return string(x), nil
	case escape.HTMLer:
		if spec != "" {
			return "", formatValueError(fmt.Sprintf("Format specifier %s given, but %s does not define __html_format__. A class that defines __html__ must define __html_format__ to work with format specifiers.", spec, pyTypeName(v)))
		}
		return string(x.HTML()), nil
	}
	rv, err := pyFormat(v, spec)
	if err != nil {
		return "", err
	}
	return string(escape.Escape(rv)), nil
}

// pyStr is Python's str(), failing on a strict Undefined.
func pyStr(v any) (string, error) {
	if u, ok := v.(runtime.Undefined); ok && u.Mode == runtime.ModeStrict {
		return "", u.Fail()
	}
	return escape.SoftStr(v), nil
}

// pyASCII escapes the non-ASCII runes of a repr, like Python's ascii().
func pyASCII(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r < 0x80:
			b.WriteRune(r)
		case r <= 0xff:
			fmt.Fprintf(&b, `\x%02x`, r)
		case r <= 0xffff:
			fmt.Fprintf(&b, `\u%04x`, r)
		default:
			fmt.Fprintf(&b, `\U%08x`, r)
		}
	}
	return b.String()
}

func isASCIIDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// pyTypeName is the Python type name templates see for v.
func pyTypeName(v any) string {
	switch v.(type) {
	case nil:
		return "NoneType"
	case bool:
		return "bool"
	case string:
		return "str"
	case escape.Markup:
		return "Markup"
	case float32, float64:
		return "float"
	case []any, *runtime.PyList:
		return "list"
	case runtime.Tuple:
		return "tuple"
	case map[string]any, map[any]any, *runtime.OrderedDict:
		return "dict"
	case runtime.Undefined:
		return "Undefined"
	}
	switch reflect.ValueOf(v).Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "int"
	}
	return fmt.Sprintf("%T", v)
}

// ------------------------------------------------ format-spec mini-language

// fmtSpec is a parsed `[[fill]align][sign][z][#][0][width][,|_][.precision][type]`.
type fmtSpec struct {
	fill      rune
	align     rune
	sign      rune
	noNegZero bool
	alt       bool
	width     int
	grouping  byte
	precision int
	typ       byte
}

func isAlign(r rune) bool { return r == '<' || r == '>' || r == '=' || r == '^' }

// parseSpec parses spec for an object whose default alignment is
// defaultAlign ('<' for str, '>' for numbers), as CPython's
// parse_internal_render_format_spec.
func parseSpec(spec, typeName string, defaultAlign rune) (fmtSpec, error) {
	sp := fmtSpec{fill: ' ', width: -1, precision: -1}
	rs := []rune(spec)
	pos := 0
	fillSet, alignSet := false, false
	if len(rs) >= 2 && isAlign(rs[1]) {
		sp.fill, sp.align = rs[0], rs[1]
		fillSet, alignSet = true, true
		pos = 2
	} else if len(rs) >= 1 && isAlign(rs[0]) {
		sp.align = rs[0]
		alignSet = true
		pos = 1
	}
	if pos < len(rs) && (rs[pos] == '+' || rs[pos] == '-' || rs[pos] == ' ') {
		sp.sign = rs[pos]
		pos++
	}
	if pos < len(rs) && rs[pos] == 'z' {
		sp.noNegZero = true
		pos++
	}
	if pos < len(rs) && rs[pos] == '#' {
		sp.alt = true
		pos++
	}
	if !fillSet && pos < len(rs) && rs[pos] == '0' {
		sp.fill = '0'
		if !alignSet && defaultAlign == '>' {
			sp.align = '='
		}
		pos++
	}
	n, pos := readInt(rs, pos)
	if n >= 0 {
		sp.width = n
	}
	if pos < len(rs) && rs[pos] == ',' {
		sp.grouping = ','
		pos++
	}
	if pos < len(rs) && rs[pos] == '_' {
		if sp.grouping != 0 {
			return sp, formatValueError("Cannot specify both ',' and '_'.")
		}
		sp.grouping = '_'
		pos++
	}
	if pos < len(rs) && rs[pos] == ',' && sp.grouping == '_' {
		return sp, formatValueError("Cannot specify both ',' and '_'.")
	}
	if pos < len(rs) && rs[pos] == '.' {
		n, pos = readInt(rs, pos+1)
		if n < 0 {
			return sp, formatValueError("Format specifier missing precision")
		}
		sp.precision = n
	}
	if len(rs)-pos > 1 {
		return sp, formatValueError(fmt.Sprintf("Invalid format specifier '%s' for object of type '%s'", spec, typeName))
	}
	if pos < len(rs) {
		if rs[pos] > 0x7f {
			return sp, formatValueError(fmt.Sprintf("Unknown format code '%c' for object of type '%s'", rs[pos], typeName))
		}
		sp.typ = byte(rs[pos])
	}
	if sp.grouping != 0 {
		switch sp.typ {
		case 'd', 'e', 'f', 'g', 'E', 'G', '%', 'F', 0:
		case 'b', 'o', 'x', 'X':
			if sp.grouping != '_' {
				return sp, formatValueError(fmt.Sprintf("Cannot specify ',' with '%c'.", sp.typ))
			}
		default:
			return sp, formatValueError(fmt.Sprintf("Cannot specify '%c' with '%c'.", sp.grouping, sp.typ))
		}
	}
	return sp, nil
}

// readInt reads decimal digits at pos; n is -1 when there are none.
func readInt(rs []rune, pos int) (int, int) {
	n := -1
	for pos < len(rs) && rs[pos] >= '0' && rs[pos] <= '9' {
		if n < 0 {
			n = 0
		}
		n = n*10 + int(rs[pos]-'0')
		pos++
	}
	return n, pos
}

// pyFormat is Python's format(value, spec) for the value types templates
// produce.
func pyFormat(v any, spec string) (string, error) {
	if spec == "" {
		return pyStr(v)
	}
	switch x := v.(type) {
	case string:
		return formatStr(x, spec)
	case escape.Markup:
		return formatStr(string(x), spec)
	case bool:
		var n uint64
		if x {
			n = 1
		}
		return formatInt(false, n, spec, "bool")
	case float64:
		return formatFloat(x, spec, "float")
	case float32:
		return formatFloat(float64(x), spec, "float")
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n := rv.Int()
		if n < 0 {
			return formatInt(true, uint64(-(n+1))+1, spec, "int")
		}
		return formatInt(false, uint64(n), spec, "int")
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return formatInt(false, rv.Uint(), spec, "int")
	}
	if u, ok := v.(runtime.Undefined); ok && u.Mode == runtime.ModeStrict {
		return "", u.Fail()
	}
	return "", gjerrors.NewTemplateRuntimeError(fmt.Sprintf("unsupported format string passed to %s.__format__", pyTypeName(v)))
}

func formatStr(s, spec string) (string, error) {
	sp, err := parseSpec(spec, "str", '<')
	if err != nil {
		return "", err
	}
	switch {
	case sp.sign != 0:
		return "", formatValueError("Sign not allowed in string format specifier")
	case sp.noNegZero:
		return "", formatValueError("Negative zero coercion (z) not allowed in format specifier")
	case sp.alt:
		return "", formatValueError("Alternate form (#) not allowed in string format specifier")
	case sp.align == '=':
		return "", formatValueError("'=' alignment not allowed in string format specifier")
	case sp.typ != 0 && sp.typ != 's':
		return "", formatValueError(fmt.Sprintf("Unknown format code '%c' for object of type 'str'", sp.typ))
	}
	if sp.precision >= 0 && utf8.RuneCountInString(s) > sp.precision {
		s = string([]rune(s)[:sp.precision])
	}
	return pad(sp, "", s, '<'), nil
}

func formatInt(neg bool, mag uint64, spec, typeName string) (string, error) {
	sp, err := parseSpec(spec, typeName, '>')
	if err != nil {
		return "", err
	}
	switch sp.typ {
	case 'e', 'E', 'f', 'F', 'g', 'G', '%':
		f := float64(mag)
		if neg {
			f = -f
		}
		return formatFloat(f, spec, typeName)
	case 0, 'd', 'n', 'b', 'o', 'x', 'X', 'c':
	default:
		return "", formatValueError(fmt.Sprintf("Unknown format code '%c' for object of type '%s'", sp.typ, typeName))
	}
	if sp.precision >= 0 {
		return "", formatValueError("Precision not allowed in integer format specifier")
	}
	if sp.noNegZero {
		return "", formatValueError("Negative zero coercion (z) not allowed in integer format specifier")
	}
	if sp.typ == 'n' && sp.grouping != 0 {
		return "", formatValueError(fmt.Sprintf("Cannot specify '%c' with 'n'.", sp.grouping))
	}
	if sp.typ == 'c' {
		if sp.sign != 0 {
			return "", formatValueError("Sign not allowed with integer format specifier 'c'")
		}
		if sp.alt {
			return "", formatValueError("Alternate form (#) not allowed with integer format specifier 'c'")
		}
		if neg || mag > 0x10ffff {
			return "", gjerrors.NewTemplateRuntimeError("%c arg not in range(0x110000)")
		}
		return pad(sp, "", string(rune(mag)), '>'), nil
	}
	base, prefix, group := 10, "", 3
	switch sp.typ {
	case 'b':
		base, prefix, group = 2, "0b", 4
	case 'o':
		base, prefix, group = 8, "0o", 4
	case 'x':
		base, prefix, group = 16, "0x", 4
	case 'X':
		base, prefix, group = 16, "0X", 4
	}
	if !sp.alt {
		prefix = ""
	}
	digits := strconv.FormatUint(mag, base)
	if sp.typ == 'X' {
		digits = strings.ToUpper(digits)
	}
	return layoutNumber(sp, signStr(sp, neg), prefix, digits, "", group), nil
}

func formatFloat(x float64, spec, typeName string) (string, error) {
	sp, err := parseSpec(spec, typeName, '>')
	if err != nil {
		return "", err
	}
	if sp.typ == 'c' {
		return "", formatValueError(fmt.Sprintf("Unknown format code 'c' for object of type '%s'", typeName))
	}
	typ := sp.typ
	prec := sp.precision
	addDot0 := false
	switch typ {
	case 0:
		addDot0 = true
		if prec < 0 {
			typ = 'r'
		} else {
			typ = 'g'
		}
	case 'n':
		typ = 'g'
	case 'e', 'E', 'f', 'F', 'g', 'G', '%':
	default:
		return "", formatValueError(fmt.Sprintf("Unknown format code '%c' for object of type '%s'", typ, typeName))
	}
	if prec < 0 {
		prec = 6
	}
	suffix := ""
	if typ == '%' {
		x *= 100
		typ = 'f'
		suffix = "%"
	}
	neg := math.Signbit(x) && !math.IsNaN(x)
	a := math.Abs(x)
	var body string
	switch {
	case math.IsNaN(a):
		body = "nan"
	case math.IsInf(a, 0):
		body = "inf"
	default:
		body = floatBody(a, typ, prec, sp.alt, addDot0)
	}
	if typ == 'E' || typ == 'F' || typ == 'G' {
		body = strings.ToUpper(body)
	}
	if neg && sp.noNegZero && strings.Trim(strings.SplitN(body, "e", 2)[0], "0.") == "" {
		neg = false
	}
	intEnd := strings.IndexAny(body, ".eE%")
	if intEnd < 0 || math.IsNaN(a) || math.IsInf(a, 0) {
		intEnd = len(body)
	}
	return layoutNumber(sp, signStr(sp, neg), "", body[:intEnd], body[intEnd:]+suffix, 3), nil
}

// floatBody formats a non-negative finite float the way CPython's
// PyOS_double_to_string does for the given presentation type.
func floatBody(a float64, typ byte, prec int, alt, addDot0 bool) string {
	switch typ {
	case 'r':
		return escape.SoftStr(a)
	case 'e', 'E':
		s := strconv.FormatFloat(a, 'e', prec, 64)
		if alt && prec == 0 {
			s = strings.Replace(s, "e", ".e", 1)
		}
		return s
	case 'f', 'F':
		s := strconv.FormatFloat(a, 'f', prec, 64)
		if alt && prec == 0 {
			s += "."
		}
		return s
	}
	// 'g' / 'G' / no-type with precision.
	if prec == 0 {
		prec = 1
	}
	es := strconv.FormatFloat(a, 'e', prec-1, 64)
	exp, _ := strconv.Atoi(es[strings.IndexByte(es, 'e')+1:])
	// With addDot0 (no presentation type) CPython switches to exponent
	// notation one digit earlier: 10.0 formats as '1e+01' under '.2'.
	limit := prec
	if addDot0 {
		limit--
	}
	var s string
	if exp >= -4 && exp < limit {
		s = strconv.FormatFloat(a, 'f', prec-1-exp, 64)
	} else {
		s = es
	}
	if !alt {
		mant, expPart := s, ""
		if i := strings.IndexByte(s, 'e'); i >= 0 {
			mant, expPart = s[:i], s[i:]
		}
		if strings.Contains(mant, ".") {
			mant = strings.TrimRight(mant, "0")
			mant = strings.TrimSuffix(mant, ".")
		}
		s = mant + expPart
		if addDot0 && !strings.ContainsAny(s, ".e") {
			s += ".0"
		}
	} else if !strings.Contains(s, ".") {
		if i := strings.IndexByte(s, 'e'); i >= 0 {
			s = s[:i] + "." + s[i:]
		} else {
			s += "."
		}
	}
	return s
}

func signStr(sp fmtSpec, neg bool) string {
	switch {
	case neg:
		return "-"
	case sp.sign == '+':
		return "+"
	case sp.sign == ' ':
		return " "
	}
	return ""
}

// layoutNumber assembles sign, prefix, (grouped) integer digits and the
// remainder, then pads to the requested width. Sign-aware zero padding
// (fill '0' with '=' alignment) pads the digits themselves so thousands
// separators run through the padding, as CPython does.
func layoutNumber(sp fmtSpec, sign, prefix, digits, rest string, group int) string {
	minWidth := 0
	if sp.fill == '0' && sp.align == '=' && sp.width > 0 {
		minWidth = sp.width - len(sign) - len(prefix) - utf8.RuneCountInString(rest)
	}
	digits = groupDigits(digits, sp.grouping, group, minWidth)
	return pad(sp, sign+prefix, digits+rest, '>')
}

// groupDigits inserts sep every size digits from the right and zero-pads
// to minWidth, mirroring _PyUnicode_InsertThousandsGrouping.
func groupDigits(digits string, sep byte, size, minWidth int) string {
	if sep == 0 {
		if n := minWidth - len(digits); n > 0 {
			return strings.Repeat("0", n) + digits
		}
		return digits
	}
	var chunks []string
	remaining := len(digits)
	for {
		l := min(size, max(remaining, minWidth, 1))
		zeros := max(0, l-remaining)
		chars := max(0, min(remaining, l))
		chunks = append(chunks, strings.Repeat("0", zeros)+digits[remaining-chars:remaining])
		remaining -= chars
		minWidth -= l
		if remaining <= 0 && minWidth <= 0 {
			break
		}
		minWidth--
	}
	var b strings.Builder
	for i := len(chunks) - 1; i >= 0; i-- {
		b.WriteString(chunks[i])
		if i > 0 {
			b.WriteByte(sep)
		}
	}
	return b.String()
}

// pad applies fill/align/width. lead is the sign+prefix that '='
// alignment keeps in front of the padding.
func pad(sp fmtSpec, lead, body string, defaultAlign rune) string {
	n := utf8.RuneCountInString(lead) + utf8.RuneCountInString(body)
	if sp.width <= n {
		return lead + body
	}
	fill := strings.Repeat(string(sp.fill), sp.width-n)
	align := sp.align
	if align == 0 {
		align = defaultAlign
	}
	switch align {
	case '<':
		return lead + body + fill
	case '^':
		left := (sp.width - n) / 2
		return strings.Repeat(string(sp.fill), left) + lead + body + strings.Repeat(string(sp.fill), sp.width-n-left)
	case '=':
		return lead + fill + body
	}
	return fill + lead + body
}
