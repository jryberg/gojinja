package runtime

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"unicode"

	"github.com/jryberg/gojinja/pkg/escape"
)

// IsValueError reports whether err is ParseInt's or ParseFloat's error
// for a string Python cannot convert: Python's ValueError.
func IsValueError(err error) bool {
	var ve escape.ValueError
	return errors.As(err, &ve)
}

func valueError(format string, a ...any) error {
	return escape.ValueError{Msg: fmt.Sprintf(format, a...)}
}

// asciiDigits strips Python's whitespace and turns Unicode decimal
// digits into ASCII ones, as int() and float() do before parsing.
func asciiDigits(s string) string {
	s = strings.TrimFunc(s, unicode.IsSpace)
	if isASCII(s) {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		if r > 0x7f && unicode.IsDigit(r) {
			// Decimal digits come in runs of ten starting at zero.
			start := r
			for unicode.IsDigit(start - 1) {
				start--
			}
			r = '0' + (r-start)%10
		}
		b.WriteRune(r)
	}
	return b.String()
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > 0x7f {
			return false
		}
	}
	return true
}

// digitsWithUnderscores reports whether s is digits valid in base, with
// single underscores allowed between them (or, after a prefix, before
// the first digit).
func digitsWithUnderscores(s string, base int, leadingUnderscore bool) bool {
	if s == "" {
		return false
	}
	prevUnderscore := !leadingUnderscore
	for i, r := range s {
		if r == '_' {
			if prevUnderscore {
				return false
			}
			prevUnderscore = true
			continue
		}
		if digitValue(r) >= base {
			return false
		}
		prevUnderscore = false
		_ = i
	}
	return !prevUnderscore
}

func digitValue(r rune) int {
	switch {
	case r >= '0' && r <= '9':
		return int(r - '0')
	case r >= 'a' && r <= 'z':
		return int(r-'a') + 10
	case r >= 'A' && r <= 'Z':
		return int(r-'A') + 10
	}
	return 99
}

// ParseInt is Python's int(s, base) for a string.
func ParseInt(s string, base int) (any, error) {
	if base != 0 && (base < 2 || base > 36) {
		return nil, valueError("int() base must be >= 2 and <= 36, or 0")
	}
	orig := s
	s = asciiDigits(s)
	neg := false
	if s != "" && (s[0] == '+' || s[0] == '-') {
		neg = s[0] == '-'
		s = s[1:]
	}
	prefixed := false
	if len(s) >= 2 && s[0] == '0' {
		p := unicode.ToLower(rune(s[1]))
		if pb := map[rune]int{'x': 16, 'o': 8, 'b': 2}[p]; pb != 0 && (base == 0 || base == pb) {
			base, s, prefixed = pb, s[2:], true
		}
	}
	if base == 0 {
		base = 10
		// A decimal literal may not have leading zeros (except zero).
		if strings.TrimLeft(strings.ReplaceAll(s, "_", ""), "0") != "" && strings.HasPrefix(s, "0") {
			return nil, invalidInt(orig, 0)
		}
	}
	if !digitsWithUnderscores(s, base, prefixed) {
		return nil, invalidInt(orig, base)
	}
	clean := strings.ReplaceAll(s, "_", "")
	if base&(base-1) != 0 {
		if err := escape.ParseIntDigits(len(clean)); err != nil {
			return nil, err
		}
	}
	n, ok := new(big.Int).SetString(clean, base)
	if !ok {
		return nil, invalidInt(orig, base)
	}
	if neg {
		n.Neg(n)
	}
	return NormInt(n)
}

func invalidInt(s string, base int) error {
	return valueError("invalid literal for int() with base %d: %s", base, escape.Repr(s))
}

// ParseFloat is Python's float(s) for a string.
func ParseFloat(s string) (float64, error) {
	t := asciiDigits(s)
	body := strings.TrimLeft(t, "+-")
	if len(t)-len(body) > 1 {
		return 0, invalidFloat(s)
	}
	switch strings.ToLower(body) {
	case "inf", "infinity":
		if strings.HasPrefix(t, "-") {
			return math.Inf(-1), nil
		}
		return math.Inf(1), nil
	case "nan":
		return math.NaN(), nil
	}
	mant, exp, hasExp := strings.Cut(strings.ToLower(body), "e")
	intPart, frac, hasDot := strings.Cut(mant, ".")
	ok := (intPart != "" || frac != "") &&
		(intPart == "" || digitsWithUnderscores(intPart, 10, false)) &&
		(!hasDot || frac == "" || digitsWithUnderscores(frac, 10, false))
	if hasExp {
		e := strings.TrimLeft(exp, "+-")
		ok = ok && len(exp)-len(e) <= 1 && digitsWithUnderscores(e, 10, false)
	}
	if !ok {
		return 0, invalidFloat(s)
	}
	f, err := strconv.ParseFloat(strings.ReplaceAll(t, "_", ""), 64)
	if err != nil && !errors.Is(err, strconv.ErrRange) {
		return 0, invalidFloat(s)
	}
	return f, nil
}

func invalidFloat(s string) error {
	return valueError("could not convert string to float: %s", escape.Repr(s))
}

// FloatToInt is Python's int(float): truncated exactly, a ValueError
// for NaN and an OverflowError for an infinity.
func FloatToInt(f float64) (any, error) {
	switch {
	case math.IsNaN(f):
		return nil, valueError("cannot convert float NaN to integer")
	case math.IsInf(f, 0):
		return nil, errors.New("cannot convert float infinity to integer")
	}
	if f > -1<<63 && f < 1<<63 {
		return int64(f), nil
	}
	i, _ := new(big.Float).SetFloat64(math.Trunc(f)).Int(nil)
	return NormInt(i)
}
