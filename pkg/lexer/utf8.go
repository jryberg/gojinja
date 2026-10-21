package lexer

import "unicode/utf8"

// decodeRune decodes the first character of s, with a fast path for
// ASCII.
func decodeRune(s string) (rune, int) {
	if len(s) == 0 {
		return 0, 0
	}
	if s[0] < 0x80 {
		return rune(s[0]), 1
	}
	return utf8.DecodeRuneInString(s)
}
