package environment

import (
	"sort"
	"strings"
	"unicode"
)

// Python str semantics on top of Go's unicode package and the generated
// tables in strtables_table.go (which must share Go's Unicode version).

type runeRange struct{ lo, hi rune }

func inRanges(rs []runeRange, r rune) bool {
	i := sort.Search(len(rs), func(i int) bool { return rs[i].hi >= r })
	return i < len(rs) && rs[i].lo <= r
}

// isPyLower and isPyUpper are the derived Lowercase/Uppercase properties
// CPython's islower/isupper use.
func isPyLower(r rune) bool { return unicode.IsLower(r) || unicode.Is(unicode.Other_Lowercase, r) }
func isPyUpper(r rune) bool { return unicode.IsUpper(r) || unicode.Is(unicode.Other_Uppercase, r) }
func isPyCased(r rune) bool { return isPyLower(r) || isPyUpper(r) || unicode.IsTitle(r) }

func isPyDecimal(r rune) bool { return unicode.Is(unicode.Nd, r) }
func isPyDigit(r rune) bool   { return isPyDecimal(r) || inRanges(digitExtra, r) }
func isPyNumeric(r rune) bool {
	if unicode.IsNumber(r) {
		return !inRanges(numericMissing, r)
	}
	return inRanges(numericExtra, r)
}
func isPyAlpha(r rune) bool { return unicode.IsLetter(r) }
func isPyAlnum(r rune) bool {
	return isPyAlpha(r) || isPyDecimal(r) || isPyDigit(r) || isPyNumeric(r)
}

// isPyPrintable is str.isprintable for one character.
func isPyPrintable(r rune) bool {
	return r == ' ' || unicode.In(r, unicode.L, unicode.M, unicode.N, unicode.P, unicode.S)
}

func upperRune(r rune) string {
	if s, ok := upperFull[r]; ok {
		return s
	}
	return string(unicode.ToUpper(r))
}

func titleRune(r rune) string {
	if s, ok := titleFull[r]; ok {
		return s
	}
	return string(unicode.ToTitle(r))
}

// lowerAt lowercases rs[i], applying the Final_Sigma rule to capital
// sigma like CPython's handle_capital_sigma.
func lowerAt(rs []rune, i int) string {
	r := rs[i]
	if r == 0x03a3 {
		return string(finalSigma(rs, i))
	}
	if s, ok := lowerFull[r]; ok {
		return s
	}
	return string(unicode.ToLower(r))
}

func finalSigma(rs []rune, i int) rune {
	j := i - 1
	for j >= 0 && inRanges(caseIgnorable, rs[j]) {
		j--
	}
	before := j >= 0 && isPyCased(rs[j])
	k := i + 1
	for k < len(rs) && inRanges(caseIgnorable, rs[k]) {
		k++
	}
	after := k < len(rs) && isPyCased(rs[k])
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
		switch {
		case casefoldFull[r] != "":
			b.WriteString(casefoldFull[r])
		case lowerFull[r] != "":
			b.WriteString(lowerFull[r])
		default:
			b.WriteRune(unicode.ToLower(r))
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
		prevCased = isPyCased(r)
	}
	return b.String()
}

func pySwapcase(s string) string {
	rs := []rune(s)
	var b strings.Builder
	for i, r := range rs {
		switch {
		case isPyUpper(r):
			b.WriteString(lowerAt(rs, i))
		case isPyLower(r):
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
		if isPyUpper(r) || unicode.IsTitle(r) {
			return false
		}
		cased = cased || isPyLower(r)
	}
	return cased
}

func pyIsUpper(s string) bool {
	cased := false
	for _, r := range s {
		if isPyLower(r) || unicode.IsTitle(r) {
			return false
		}
		cased = cased || isPyUpper(r)
	}
	return cased
}

func pyIsTitle(s string) bool {
	cased, prevCased := false, false
	for _, r := range s {
		switch {
		case isPyUpper(r) || unicode.IsTitle(r):
			if prevCased {
				return false
			}
			prevCased, cased = true, true
		case isPyLower(r):
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

func pyIsIdentifier(s string) bool {
	for i, r := range []rune(s) {
		if i == 0 && !inRanges(xidStart, r) || i > 0 && !inRanges(xidContinue, r) {
			return false
		}
	}
	return s != ""
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
