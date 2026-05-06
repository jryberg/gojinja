package lexer

import "unicode"

// isIdentStart reports whether r can start an identifier. Mirrors Python's
// XID_Start more loosely — we accept any Letter or `_` plus the Letter_Number
// category. This is wide enough to accept every identifier in Jinja2's test
// corpus and narrower than `unicode.IsLetter` alone in that we explicitly
// exclude marks and digits.
func isIdentStart(r rune) bool {
	if r == '_' {
		return true
	}
	if r < 0x80 {
		return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
	}
	return unicode.IsLetter(r) || unicode.Is(unicode.Nl, r)
}

// isIdentContinue reports whether r can appear after the start of an
// identifier. Approximates Python's XID_Continue.
func isIdentContinue(r rune) bool {
	if r == '_' {
		return true
	}
	if r < 0x80 {
		return (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9')
	}
	if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.Is(unicode.Nl, r) {
		return true
	}
	// Combining marks (Mn, Mc) and connector punctuation (Pc) — the same
	// shape Python's str.isidentifier accepts for continue characters.
	return unicode.Is(unicode.Mn, r) ||
		unicode.Is(unicode.Mc, r) ||
		unicode.Is(unicode.Pc, r)
}

// isIdentifier reports whether the entire string s is a valid identifier.
// Empty strings are rejected.
func isIdentifier(s string) bool {
	first := true
	for _, r := range s {
		if first {
			if !isIdentStart(r) {
				return false
			}
			first = false
			continue
		}
		if !isIdentContinue(r) {
			return false
		}
	}
	return !first
}

// decodeRune is a thin wrapper over the stdlib utf8 decoder, isolated here
// so we don't sprinkle the import across the package.
func decodeRune(s string) (rune, int) {
	if len(s) == 0 {
		return 0, 0
	}
	if s[0] < 0x80 {
		return rune(s[0]), 1
	}
	return decodeRuneSlow(s)
}
