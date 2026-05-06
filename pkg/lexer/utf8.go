package lexer

import "unicode/utf8"

// decodeRuneSlow handles non-ASCII paths. Split out so the hot path
// (ASCII) in [decodeRune] avoids the import-time overhead.
func decodeRuneSlow(s string) (rune, int) {
	return utf8.DecodeRuneInString(s)
}
