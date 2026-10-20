// Package unicodename maps between Unicode code points and character
// names the way Python's unicodedata does for \N{...} escapes and the
// namereplace error handler. The data in names_table.go is generated
// from the Python that runs the pinned Jinja2.
package unicodename

import (
	"maps"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type nameRange struct {
	prefix string
	lo, hi rune
	// computed marks ranges CPython names algorithmically; their lookup is
	// case-sensitive and accepts four or five hex digits.
	computed bool
}

// Hangul syllable naming, Unicode chapter 3.12.
const (
	hangulBase   = 0xac00
	hangulLCount = 19
	hangulVCount = 21
	hangulTCount = 28
	hangulCount  = hangulLCount * hangulVCount * hangulTCount
	hangulPrefix = "HANGUL SYLLABLE "
)

var (
	jamoL = []string{"G", "GG", "N", "D", "DD", "R", "M", "B", "BB", "S", "SS", "", "J", "JJ", "C", "K", "T", "P", "H"}
	jamoV = []string{"A", "AE", "YA", "YAE", "EO", "E", "YEO", "YE", "O", "WA", "WAE", "OE", "YO", "U", "WEO", "WE", "WI", "YU", "EU", "YI", "I"}
	jamoT = []string{"", "G", "GG", "GS", "N", "NJ", "NH", "D", "L", "LG", "LM", "LB", "LS", "LT", "LP", "LH", "M", "B", "BS", "S", "SS", "NG", "J", "C", "K", "T", "P", "H"}
)

var (
	loadOnce sync.Once
	names    []string        // parallel to namedRunes
	byName   map[string]rune // upper-cased stored names and aliases
)

func load() {
	names = strings.Split(nameList, "\n")
	byName = make(map[string]rune, len(names)+len(aliases))
	for i, n := range names {
		byName[n] = namedRunes[i]
	}
	maps.Copy(byName, aliases)
}

// Name returns the character name of r, like unicodedata.name.
func Name(r rune) (string, bool) {
	if r >= hangulBase && r < hangulBase+hangulCount {
		s := int(r - hangulBase)
		return hangulPrefix + jamoL[s/(hangulVCount*hangulTCount)] + jamoV[s%(hangulVCount*hangulTCount)/hangulTCount] + jamoT[s%hangulTCount], true
	}
	for _, nr := range nameRanges {
		if r >= nr.lo && r <= nr.hi {
			return nr.prefix + strings.ToUpper(strconv.FormatInt(int64(r), 16)), true
		}
	}
	loadOnce.Do(load)
	i := sort.Search(len(namedRunes), func(i int) bool { return namedRunes[i] >= r })
	if i < len(namedRunes) && namedRunes[i] == r {
		return names[i], true
	}
	return "", false
}

// Lookup returns the character named name, accepting formal aliases but
// not named sequences, like CPython's \N{...} handling.
func Lookup(name string) (rune, bool) {
	// CPython matches the Hangul prefix and jamo case-sensitively.
	if strings.HasPrefix(name, hangulPrefix) {
		rest := name[len(hangulPrefix):]
		l, n1 := longestJamo(rest, jamoL)
		v, n2 := longestJamo(rest[n1:], jamoV)
		t, n3 := longestJamo(rest[n1+n2:], jamoT)
		if l < 0 || v < 0 || t < 0 || n1+n2+n3 != len(rest) {
			return 0, false
		}
		return hangulBase + rune((l*hangulVCount+v)*hangulTCount+t), true
	}
	for _, nr := range nameRanges {
		if !nr.computed || !strings.HasPrefix(name, nr.prefix) {
			continue
		}
		digits := name[len(nr.prefix):]
		if len(digits) != 4 && len(digits) != 5 || strings.ToUpper(digits) != digits {
			return 0, false
		}
		v, err := strconv.ParseUint(digits, 16, 32)
		if err != nil {
			return 0, false
		}
		for _, cr := range nameRanges {
			if cr.computed && cr.prefix == nr.prefix && rune(v) >= cr.lo && rune(v) <= cr.hi {
				return rune(v), true
			}
		}
		return 0, false
	}
	upper := asciiUpper(name)
	for _, nr := range nameRanges {
		if nr.computed || !strings.HasPrefix(upper, nr.prefix) {
			continue
		}
		v, err := strconv.ParseUint(upper[len(nr.prefix):], 16, 32)
		if err == nil && rune(v) >= nr.lo && rune(v) <= nr.hi {
			if canonical, _ := Name(rune(v)); canonical == upper {
				return rune(v), true
			}
		}
	}
	loadOnce.Do(load)
	r, ok := byName[upper]
	return r, ok
}

// longestJamo is CPython's find_syllable: the longest jamo name that
// prefixes s (the empty name matches when nothing longer does).
func longestJamo(s string, jamo []string) (int, int) {
	best, bestLen := -1, -1
	for i, j := range jamo {
		if len(j) > bestLen && strings.HasPrefix(s, j) {
			best, bestLen = i, len(j)
		}
	}
	return best, max(bestLen, 0)
}

// asciiUpper upper-cases ASCII letters only, like CPython's Py_TOUPPER.
func asciiUpper(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'a' && c <= 'z' {
			b[i] = c - 32
		}
	}
	return string(b)
}
