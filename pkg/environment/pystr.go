package environment

import (
	"strings"

	"github.com/jryberg/gojinja/internal/pyunicode"
)

// Python str semantics on top of the Unicode data in internal/pyunicode.

func upperRune(r rune) string {
	if s, ok := pyunicode.Upper(r); ok {
		return s
	}
	return string(r)
}

func titleRune(r rune) string {
	if s, ok := pyunicode.Title(r); ok {
		return s
	}
	return string(r)
}

// lowerAt lowercases rs[i], applying the Final_Sigma rule to capital
// sigma like CPython's handle_capital_sigma.
func lowerAt(rs []rune, i int) string {
	r := rs[i]
	if r == 0x03a3 {
		return string(finalSigma(rs, i))
	}
	if s, ok := pyunicode.Lower(r); ok {
		return s
	}
	return string(r)
}

func finalSigma(rs []rune, i int) rune {
	j := i - 1
	for j >= 0 && pyunicode.IsCaseIgnorable(rs[j]) {
		j--
	}
	before := j >= 0 && pyunicode.IsCased(rs[j])
	k := i + 1
	for k < len(rs) && pyunicode.IsCaseIgnorable(rs[k]) {
		k++
	}
	after := k < len(rs) && pyunicode.IsCased(rs[k])
	if before && !after {
		return 0x03c2
	}
	return 0x03c3
}

func pyUpper(s string) string {
	var b strings.Builder
	for _, r := range s {
		b.WriteString(upperRune(r))
	}
	return b.String()
}

func pyLower(s string) string {
	rs := []rune(s)
	var b strings.Builder
	for i := range rs {
		b.WriteString(lowerAt(rs, i))
	}
	return b.String()
}

// pyCasefold is str.casefold: full case folding, which unlike lower()
// has no final-sigma rule.
func pyCasefold(s string) string {
	var b strings.Builder
	for _, r := range s {
		if f, ok := pyunicode.Fold(r); ok {
			b.WriteString(f)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// pyCapitalize is str.capitalize: the first character title-cased, the
// rest lower-cased.
func pyCapitalize(s string) string {
	rs := []rune(s)
	var b strings.Builder
	for i := range rs {
		if i == 0 {
			b.WriteString(titleRune(rs[0]))
		} else {
			b.WriteString(lowerAt(rs, i))
		}
	}
	return b.String()
}

// pyTitle is str.title: a character after a cased one is lower-cased,
// any other is title-cased.
func pyTitle(s string) string {
	rs := []rune(s)
	var b strings.Builder
	prevCased := false
	for i, r := range rs {
		if prevCased {
			b.WriteString(lowerAt(rs, i))
		} else {
			b.WriteString(titleRune(r))
		}
		prevCased = pyunicode.IsCased(r)
	}
	return b.String()
}

func pySwapcase(s string) string {
	rs := []rune(s)
	var b strings.Builder
	for i, r := range rs {
		switch {
		case pyunicode.IsUpper(r):
			b.WriteString(lowerAt(rs, i))
		case pyunicode.IsLower(r):
			b.WriteString(upperRune(r))
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func pyIsLower(s string) bool {
	cased := false
	for _, r := range s {
		if pyunicode.IsUpper(r) || pyunicode.IsTitle(r) {
			return false
		}
		cased = cased || pyunicode.IsLower(r)
	}
	return cased
}

func pyIsUpper(s string) bool {
	cased := false
	for _, r := range s {
		if pyunicode.IsLower(r) || pyunicode.IsTitle(r) {
			return false
		}
		cased = cased || pyunicode.IsUpper(r)
	}
	return cased
}

func pyIsTitle(s string) bool {
	cased, prevCased := false, false
	for _, r := range s {
		switch {
		case pyunicode.IsUpper(r) || pyunicode.IsTitle(r):
			if prevCased {
				return false
			}
			prevCased, cased = true, true
		case pyunicode.IsLower(r):
			if !prevCased {
				return false
			}
			prevCased, cased = true, true
		default:
			prevCased = false
		}
	}
	return cased
}

// allRunes reports whether s is non-empty and every character satisfies
// pred, the shape of most str.is* methods.
func allRunes(s string, pred func(rune) bool) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !pred(r) {
			return false
		}
	}
	return true
}
