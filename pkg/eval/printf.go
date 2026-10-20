package eval

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jryberg/gojinja/pkg/escape"
	"github.com/jryberg/gojinja/pkg/runtime"
)

// PercentFormat implements Python's printf-style `format % args` for str
// (CPython's PyUnicode_Format). args is a tuple of positional values, a
// mapping used by %(key) conversions, or a single value. When markup is
// set the format is a Markup: like markupsafe's escape helper, %s, %r
// and %a escape their argument while numeric conversions see it as is.
func PercentFormat(format string, args any, markup bool) (string, error) {
	p := percentFormatter{markup: markup}
	switch a := args.(type) {
	case runtime.Tuple:
		p.args = a
	default:
		p.args = []any{args}
	}
	// CPython treats anything with __getitem__ except tuples and strings
	// as a possible mapping for %(key) conversions.
	switch args.(type) {
	case runtime.Tuple, string, escape.Markup:
	case map[string]any, map[any]any, *runtime.OrderedDict, []any, *runtime.PyList, runtime.Undefined:
		p.mapping = args
	}
	return p.format(format)
}

type percentFormatter struct {
	args    []any
	next    int
	mapping any
	markup  bool
}

func (p *percentFormatter) nextArg() (any, error) {
	if p.next >= len(p.args) {
		return nil, fmt.Errorf("not enough arguments for format string")
	}
	v := p.args[p.next]
	p.next++
	return v, nil
}

func (p *percentFormatter) format(f string) (string, error) {
	rs := []rune(f)
	var b strings.Builder
	for i := 0; i < len(rs); i++ {
		if rs[i] != '%' {
			b.WriteRune(rs[i])
			continue
		}
		start := i
		i++
		if i >= len(rs) {
			return "", fmt.Errorf("incomplete format")
		}
		var arg any
		haveArg := false
		if rs[i] == '(' {
			if p.mapping == nil {
				return "", fmt.Errorf("format requires a mapping")
			}
			depth, k := 1, i+1
			for ; k < len(rs) && depth > 0; k++ {
				switch rs[k] {
				case '(':
					depth++
				case ')':
					depth--
				}
			}
			if depth > 0 {
				return "", fmt.Errorf("incomplete format key")
			}
			key := string(rs[i+1 : k-1])
			v, err := percentKey(p.mapping, key)
			if err != nil {
				return "", err
			}
			arg, haveArg = v, true
			// CPython makes the looked-up value the only argument, so a
			// later unkeyed conversion has nothing left.
			p.next = len(p.args)
			i = k
		}
		spec := percentSpec{width: -1, prec: -1}
		for ; i < len(rs); i++ {
			switch rs[i] {
			case '-':
				spec.left = true
			case '+':
				spec.plus = true
			case ' ':
				spec.space = true
			case '#':
				spec.alt = true
			case '0':
				spec.zero = true
			default:
				goto width
			}
		}
	width:
		if n, end, err := p.number(rs, i); err != nil {
			return "", err
		} else if end > i {
			spec.width, i = n, end
			if spec.width < 0 {
				spec.left, spec.width = true, -spec.width
			}
		}
		if i < len(rs) && rs[i] == '.' {
			i++
			n, end, err := p.number(rs, i)
			if err != nil {
				return "", err
			}
			spec.prec, i = max(n, 0), end
		}
		for i < len(rs) && (rs[i] == 'h' || rs[i] == 'l' || rs[i] == 'L') {
			i++
		}
		if i >= len(rs) {
			return "", fmt.Errorf("incomplete format")
		}
		verb := rs[i]
		if verb == '%' {
			b.WriteByte('%')
			continue
		}
		if !haveArg {
			v, err := p.nextArg()
			if err != nil {
				return "", err
			}
			arg = v
		}
		s, err := p.convert(verb, arg, spec, i-start)
		if err != nil {
			return "", err
		}
		b.WriteString(s)
	}
	// CPython only complains about leftovers when args is not a mapping.
	if p.next < len(p.args) && p.mapping == nil {
		return "", fmt.Errorf("not all arguments converted during string formatting")
	}
	return b.String(), nil
}

// number reads a width or precision at rs[i]: digits, or `*` taken from
// the arguments. end == i when there is none.
func (p *percentFormatter) number(rs []rune, i int) (int, int, error) {
	if i < len(rs) && rs[i] == '*' {
		v, err := p.nextArg()
		if err != nil {
			return 0, i, err
		}
		n, ok := asInt64(boolToInt(v))
		if !ok {
			return 0, i, fmt.Errorf("* wants int")
		}
		return int(n), i + 1, nil
	}
	end := i
	for end < len(rs) && rs[end] >= '0' && rs[end] <= '9' {
		end++
	}
	if end == i {
		return 0, i, nil
	}
	n, _ := strconv.Atoi(string(rs[i:end]))
	return n, end, nil
}

func percentKey(mapping any, key string) (any, error) {
	switch m := mapping.(type) {
	case map[string]any:
		if v, ok := m[key]; ok {
			return v, nil
		}
	case map[any]any:
		if v, ok := m[key]; ok {
			return v, nil
		}
	case *runtime.OrderedDict:
		if v, ok := m.Get(key); ok {
			return v, nil
		}
	case runtime.Undefined:
		return m.GetItem(key)
	default:
		return nil, fmt.Errorf("list indices must be integers or slices, not str")
	}
	return nil, fmt.Errorf("KeyError: %s", escape.Repr(key))
}

type percentSpec struct {
	left, plus, space, alt, zero bool
	width, prec                  int
}

func (p *percentFormatter) convert(verb rune, arg any, spec percentSpec, index int) (string, error) {
	switch verb {
	case 's', 'r', 'a':
		if verb == 's' {
			if err := FailStrict(arg); err != nil {
				return "", err
			}
		}
		var s string
		switch {
		case verb == 's' && p.markup:
			s = string(escape.Escape(arg))
		case verb == 's':
			s = escape.SoftStr(arg)
		case verb == 'r':
			s = escape.Repr(arg)
		default:
			s = pyASCII(escape.Repr(arg))
		}
		if verb != 's' && p.markup {
			s = string(escape.Escape(s))
		}
		if spec.prec >= 0 && utf8.RuneCountInString(s) > spec.prec {
			s = string([]rune(s)[:spec.prec])
		}
		return pad(s, spec, false), nil
	case 'c', 'd', 'i', 'u', 'o', 'x', 'X', 'e', 'E', 'f', 'F', 'g', 'G':
		// Undefined's __int__ and __float__ raise.
		if err := FailUndefined(arg); err != nil {
			return "", err
		}
	}
	switch verb {
	case 'c':
		switch x := arg.(type) {
		case string:
			if utf8.RuneCountInString(x) != 1 {
				return "", fmt.Errorf("%%c requires an int or a unicode character, not a string of length %d", utf8.RuneCountInString(x))
			}
			return pad(x, spec, false), nil
		}
		b, ok := runtime.BigInt(boolToInt(arg))
		if !ok {
			return "", fmt.Errorf("%%c requires an int or a unicode character, not %s", runtime.PyTypeName(arg))
		}
		if !b.IsInt64() || b.Int64() < 0 || b.Int64() > 0x10ffff {
			return "", fmt.Errorf("%%c arg not in range(0x110000)")
		}
		return pad(string(rune(b.Int64())), spec, false), nil
	case 'd', 'i', 'u':
		b, ok := runtime.BigInt(boolToInt(arg))
		if !ok {
			f, isFloat := asFloat(arg)
			if !isFloat {
				return "", fmt.Errorf("%%%c format: a real number is required, not %s", verb, runtime.PyTypeName(arg))
			}
			if math.IsInf(f, 0) || math.IsNaN(f) {
				return "", fmt.Errorf("cannot convert float %s to integer", pyFloatWord(f))
			}
			b, _ = new(big.Float).SetFloat64(math.Trunc(f)).Int(nil)
		}
		return formatInt(b, 10, false, spec, "")
	case 'o', 'x', 'X':
		b, ok := runtime.BigInt(boolToInt(arg))
		if !ok {
			return "", fmt.Errorf("%%%c format: an integer is required, not %s", verb, runtime.PyTypeName(arg))
		}
		base, prefix := 16, "0x"
		if verb == 'o' {
			base, prefix = 8, "0o"
		} else if verb == 'X' {
			prefix = "0X"
		}
		if !spec.alt {
			prefix = ""
		}
		return formatInt(b, base, verb == 'X', spec, prefix)
	case 'e', 'E', 'f', 'F', 'g', 'G':
		if _, isComplex := arg.(complex128); isComplex || !runtime.IsNumber(arg) {
			return "", fmt.Errorf("must be real number, not %s", runtime.PyTypeName(arg))
		}
		f, isFloat := asFloat(arg)
		if !isFloat {
			var err error
			if f, err = runtime.IntToFloat(boolToInt(arg)); err != nil {
				return "", err
			}
		}
		return formatFloat(f, verb, spec), nil
	}
	return "", fmt.Errorf("unsupported format character '%c' (0x%x) at index %d", verb, verb, index)
}

func pyFloatWord(f float64) string {
	if math.IsNaN(f) {
		return "NaN"
	}
	return "infinity"
}

// formatInt renders an integer with printf flags; prec is the minimum
// number of digits.
func formatInt(n *big.Int, base int, upper bool, spec percentSpec, prefix string) (string, error) {
	neg := n.Sign() < 0
	abs := new(big.Int).Abs(n)
	var digits string
	if base == 10 {
		var err error
		if digits, err = escape.IntDecimal(abs); err != nil {
			return "", err
		}
	} else {
		digits = abs.Text(base)
	}
	if upper {
		digits = strings.ToUpper(digits)
	}
	if spec.prec > len(digits) {
		digits = strings.Repeat("0", spec.prec-len(digits)) + digits
	}
	return padNumber(signOf(neg, spec)+prefix, digits, spec), nil
}

func signOf(neg bool, spec percentSpec) string {
	switch {
	case neg:
		return "-"
	case spec.plus:
		return "+"
	case spec.space:
		return " "
	}
	return ""
}

// padNumber applies width: zero padding goes between sign/prefix and the
// digits, as in C's printf.
func padNumber(sign, digits string, spec percentSpec) string {
	n := utf8.RuneCountInString(sign) + utf8.RuneCountInString(digits)
	if spec.width > n && spec.zero && !spec.left {
		return sign + strings.Repeat("0", spec.width-n) + digits
	}
	return pad(sign+digits, spec, true)
}

func pad(s string, spec percentSpec, _ bool) string {
	n := utf8.RuneCountInString(s)
	if spec.width <= n {
		return s
	}
	fill := strings.Repeat(" ", spec.width-n)
	if spec.left {
		return s + fill
	}
	return fill + s
}

// formatFloat renders f like C's printf for e, f and g (CPython's
// %-formatting of floats).
func formatFloat(f float64, verb rune, spec percentSpec) string {
	neg := math.Signbit(f)
	if neg {
		f = -f
	}
	upper := verb == 'E' || verb == 'F' || verb == 'G'
	var digits string
	switch {
	case math.IsInf(f, 0):
		digits = "inf"
	case math.IsNaN(f):
		digits, neg = "nan", false
	default:
		prec := spec.prec
		if prec < 0 {
			prec = 6
		}
		switch verb {
		case 'e', 'E':
			digits = strconv.FormatFloat(f, 'e', prec, 64)
			if spec.alt && prec == 0 {
				digits = strings.Replace(digits, "e", ".e", 1)
			}
		case 'f', 'F':
			digits = strconv.FormatFloat(f, 'f', prec, 64)
			if spec.alt && prec == 0 {
				digits += "."
			}
		default:
			digits = formatG(f, prec, spec.alt)
		}
	}
	if upper {
		digits = strings.ToUpper(digits)
	}
	return padNumber(signOf(neg, spec), digits, spec)
}

// formatG is C's %g: the shorter of %e and %f for the precision, without
// trailing zeros unless alt is set.
func formatG(f float64, prec int, alt bool) string {
	if prec == 0 {
		prec = 1
	}
	exp := 0
	if f != 0 {
		e := strconv.FormatFloat(f, 'e', prec-1, 64)
		exp, _ = strconv.Atoi(e[strings.IndexByte(e, 'e')+1:])
	}
	var s string
	if exp >= -4 && exp < prec {
		s = strconv.FormatFloat(f, 'f', prec-1-exp, 64)
	} else {
		s = strconv.FormatFloat(f, 'e', prec-1, 64)
	}
	if alt {
		if !strings.Contains(s, ".") {
			if i := strings.IndexByte(s, 'e'); i >= 0 {
				s = s[:i] + "." + s[i:]
			} else {
				s += "."
			}
		}
		return s
	}
	mant, expPart := s, ""
	if i := strings.IndexByte(s, 'e'); i >= 0 {
		mant, expPart = s[:i], s[i:]
	}
	if strings.Contains(mant, ".") {
		mant = strings.TrimRight(strings.TrimRight(mant, "0"), ".")
	}
	return mant + expPart
}

// pyASCII is Python's ascii(): repr with non-ASCII characters escaped.
func pyASCII(repr string) string {
	var b strings.Builder
	for _, r := range repr {
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
