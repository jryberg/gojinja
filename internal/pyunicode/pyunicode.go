// Package pyunicode answers Unicode questions the way the CPython that
// runs the pinned Jinja2 does: str predicates, case mappings, the regex
// classes \d, \s and \w, identifiers and decimal digit values.
//
// Its tables are generated from that Python (tools/parity/pyunicode.py),
// so template output never depends on the Unicode version of the Go
// toolchain that builds gojinja. Code whose output a template can see
// uses this package instead of Go's unicode package.
package pyunicode

import (
	"fmt"
	"slices"
	"sort"
	"strings"
)

// Range is an inclusive range of code points.
type Range struct{ Lo, Hi rune }

// Set is a sorted list of disjoint ranges.
type Set []Range

// Contains reports whether r is in s.
func (s Set) Contains(r rune) bool {
	i := sort.Search(len(s), func(i int) bool { return s[i].Hi >= r })
	return i < len(s) && s[i].Lo <= r
}

// Class returns s as the body of a regexp character class, for use
// between brackets.
func (s Set) Class() string {
	var b strings.Builder
	for _, rg := range s {
		fmt.Fprintf(&b, `\x{%x}`, rg.Lo)
		if rg.Hi != rg.Lo {
			fmt.Fprintf(&b, `-\x{%x}`, rg.Hi)
		}
	}
	return b.String()
}

// Without returns s minus the runes rs.
func (s Set) Without(rs ...rune) Set {
	rs = slices.Sorted(slices.Values(rs))
	out := Set{}
	for _, rg := range s {
		lo := rg.Lo
		for _, x := range rs {
			if x < lo || x > rg.Hi {
				continue
			}
			if lo < x {
				out = append(out, Range{lo, x - 1})
			}
			lo = x + 1
		}
		if lo <= rg.Hi {
			out = append(out, Range{lo, rg.Hi})
		}
	}
	return out
}

// The regex classes Python's re module gives \d, \s and \w in str
// patterns. Go's regexp spells them as ASCII classes, so patterns that
// port a Python regex use these instead.
var (
	Decimal = decimalSet
	Space   = spaceSet
	Word    = wordSet
)

const (
	propLower = 1 << iota
	propUpper
	propTitle
	propCaseIgnorable
	propAlpha
	propAlnum
	propDecimal
	propDigit
	propNumeric
	propSpace
	propPrintable
	propWord
	propXIDStart
	propXIDContinue
	propJinjaName
)

var propSets = []struct {
	bit uint16
	set Set
}{
	{propLower, lowerSet}, {propUpper, upperSet}, {propTitle, titleSet},
	{propCaseIgnorable, caseIgnorableSet}, {propAlpha, alphaSet}, {propAlnum, alnumSet},
	{propDecimal, decimalSet}, {propDigit, digitSet}, {propNumeric, numericSet},
	{propSpace, spaceSet}, {propPrintable, printableSet}, {propWord, wordSet},
	{propXIDStart, xidStartSet}, {propXIDContinue, xidContinueSet},
	{propJinjaName, jinjaNameSet},
}

// asciiProps caches every property for ASCII, the common case.
var asciiProps = func() (out [128]uint16) {
	for _, p := range propSets {
		for r := range rune(128) {
			if p.set.Contains(r) {
				out[r] |= p.bit
			}
		}
	}
	return out
}()

func has(r rune, bit uint16, s Set) bool {
	if r >= 0 && r < 128 {
		return asciiProps[r]&bit != 0
	}
	return s.Contains(r)
}

// IsLower reports the Lowercase property, as str.islower does for one
// character.
func IsLower(r rune) bool { return has(r, propLower, lowerSet) }

// IsUpper reports the Uppercase property, as str.isupper does for one
// character.
func IsUpper(r rune) bool { return has(r, propUpper, upperSet) }

// IsTitle reports whether r is a titlecase letter (Lt).
func IsTitle(r rune) bool { return has(r, propTitle, titleSet) }

// IsCased reports whether r is lowercase, uppercase or titlecase.
func IsCased(r rune) bool { return IsLower(r) || IsUpper(r) || IsTitle(r) }

// IsCaseIgnorable reports whether str.lower's final-sigma rule skips r.
func IsCaseIgnorable(r rune) bool { return has(r, propCaseIgnorable, caseIgnorableSet) }

// IsAlpha is str.isalpha for one character.
func IsAlpha(r rune) bool { return has(r, propAlpha, alphaSet) }

// IsAlnum is str.isalnum for one character.
func IsAlnum(r rune) bool { return has(r, propAlnum, alnumSet) }

// IsDecimal is str.isdecimal for one character, and the regex class \d.
func IsDecimal(r rune) bool { return has(r, propDecimal, decimalSet) }

// IsDigit is str.isdigit for one character.
func IsDigit(r rune) bool { return has(r, propDigit, digitSet) }

// IsNumeric is str.isnumeric for one character.
func IsNumeric(r rune) bool { return has(r, propNumeric, numericSet) }

// IsSpace is str.isspace for one character, and the regex class \s.
func IsSpace(r rune) bool { return has(r, propSpace, spaceSet) }

// IsPrintable is str.isprintable for one character; repr escapes the
// characters it rejects.
func IsPrintable(r rune) bool { return has(r, propPrintable, printableSet) }

// IsWord is the regex class \w: str.isalnum or '_'.
func IsWord(r rune) bool { return has(r, propWord, wordSet) }

// IsXIDStart reports whether str.isidentifier accepts r first.
func IsXIDStart(r rune) bool { return has(r, propXIDStart, xidStartSet) }

// IsXIDContinue reports whether str.isidentifier accepts r after the
// first character.
func IsXIDContinue(r rune) bool { return has(r, propXIDContinue, xidContinueSet) }

// IsJinjaName reports whether r can be part of a Jinja2 name token
// (jinja2._identifier: \w plus the identifier characters \w lacks).
func IsJinjaName(r rune) bool { return has(r, propJinjaName, jinjaNameSet) }

// IsIdentifier is str.isidentifier.
func IsIdentifier(s string) bool {
	first := true
	for _, r := range s {
		if first && !IsXIDStart(r) || !first && !IsXIDContinue(r) {
			return false
		}
		first = false
	}
	return !first
}

// DecimalValue returns the value of the decimal digit r, as int() reads
// it.
func DecimalValue(r rune) (int, bool) {
	if r >= '0' && r <= '9' {
		return int(r - '0'), true
	}
	i := sort.Search(len(decimalSet), func(i int) bool { return decimalSet[i].Hi >= r })
	if i < len(decimalSet) && decimalSet[i].Lo <= r {
		// Digits come in runs of ten, each starting at zero.
		return int(r-decimalSet[i].Lo) % 10, true
	}
	return 0, false
}

type mapping []struct {
	r rune
	s string
}

func (m mapping) get(r rune) (string, bool) {
	i := sort.Search(len(m), func(i int) bool { return m[i].r >= r })
	if i < len(m) && m[i].r == r {
		return m[i].s, true
	}
	return "", false
}

// Upper returns str.upper of r when it differs from r.
func Upper(r rune) (string, bool) { return upperMap.get(r) }

// Lower returns str.lower of r, without the final-sigma rule, when it
// differs from r.
func Lower(r rune) (string, bool) { return lowerMap.get(r) }

// Title returns str.title of r alone when it differs from r.
func Title(r rune) (string, bool) { return titleMap.get(r) }

// Fold returns str.casefold of r when it differs from r.
func Fold(r rune) (string, bool) { return foldMap.get(r) }

// LowerASCII lower-cases the ASCII letters of s only, like CPython's
// Py_TOLOWER, which it applies to encoding names and to float()'s inf
// and nan.
func LowerASCII(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r + 'a' - 'A'
		}
		return r
	}, s)
}

// IgnoreCaseClass returns the body of a regexp character class matching
// what the ASCII letters in letters match under Python's re.IGNORECASE:
// both cases plus Python's extra case folds (i matching U+0130 and
// U+0131, k matching the Kelvin sign, s matching long s). Go's (?i)
// knows only some of these, and also folds non-letters, so ported
// IGNORECASE patterns spell letters out with this instead.
func IgnoreCaseClass(letters string) string {
	var b strings.Builder
	for i := 0; i < len(letters); i++ {
		c := letters[i]
		if c < 'a' || c > 'z' {
			panic(fmt.Sprintf("pyunicode.IgnoreCaseClass: %q is not a lowercase ASCII letter", c))
		}
		b.WriteByte(c)
		b.WriteByte(c - 'a' + 'A')
		for _, r := range ignoreCaseFold[c-'a'] {
			fmt.Fprintf(&b, `\x{%x}`, r)
		}
	}
	return b.String()
}
