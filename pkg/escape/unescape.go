package escape

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// Unescape is Python's html.unescape, which markupsafe's
// Markup.unescape and striptags use: every named and numeric character
// reference in s is replaced by the text it stands for, with HTML5's
// rules for references without a semicolon and for invalid numbers.
func Unescape(s string) string {
	if !strings.Contains(s, "&") {
		return s
	}
	var b strings.Builder
	for {
		i := strings.IndexByte(s, '&')
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		b.WriteString(s[:i])
		ref, n := charref(s[i+1:])
		if n == 0 {
			b.WriteByte('&')
			s = s[i+1:]
			continue
		}
		b.WriteString(ref)
		s = s[i+1+n:]
	}
}

// charref decodes the reference s starts with (after its '&'), matching
// html._charref: `#digits;?`, `#x hexdigits;?` or a name of up to 32
// characters. n is the length consumed, 0 when nothing matches.
func charref(s string) (string, int) {
	if strings.HasPrefix(s, "#") {
		base, start := 10, 1
		if len(s) > 1 && (s[1] == 'x' || s[1] == 'X') {
			base, start = 16, 2
		}
		end := start
		for end < len(s) && isDigit(s[end], base) {
			end++
		}
		if end == start {
			return "", 0
		}
		n := end
		if n < len(s) && s[n] == ';' {
			n++
		}
		return numericRef(s[start:end], base), n
	}
	// The name: 1 to 32 characters other than \t \n \f space < & # ;
	end, count := 0, 0
	for end < len(s) && count < 32 {
		r, size := utf8.DecodeRuneInString(s[end:])
		if strings.ContainsRune("\t\n\f <&#;", r) {
			break
		}
		end += size
		count++
	}
	if end == 0 {
		return "", 0
	}
	n := end
	if n < len(s) && s[n] == ';' {
		n++
	}
	name := s[:n]
	if text, ok := html5Entities[name]; ok {
		return text, n
	}
	// The longest known prefix of at least two characters.
	cut := []int{}
	for i := range name {
		cut = append(cut, i)
	}
	for k := len(cut) - 1; k >= 2; k-- {
		if text, ok := html5Entities[name[:cut[k]]]; ok {
			return text + name[cut[k]:], n
		}
	}
	return "&" + name, n
}

func isDigit(c byte, base int) bool {
	if base == 16 {
		return '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F'
	}
	return '0' <= c && c <= '9'
}

// numericRef is the text of a numeric character reference.
func numericRef(digits string, base int) string {
	num, err := strconv.ParseInt(digits, base, 64)
	if err != nil {
		// Too large for any code point.
		return "�"
	}
	if text, ok := invalidCharrefs[num]; ok {
		return text
	}
	if 0xD800 <= num && num <= 0xDFFF || num > 0x10FFFF {
		return "�"
	}
	if invalidCodepoints[num] {
		return ""
	}
	return string(rune(num))
}
