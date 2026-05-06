package lexer

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// decodeStringLiteral interprets a Jinja2 string literal value (the text
// _between_ the surrounding quotes — already stripped by the caller) and
// returns the runtime value, applying Python's `unicode-escape` codec
// semantics.
//
// Supported escape sequences (mirrors Python):
//
//	\\           backslash
//	\'           single quote
//	\"           double quote
//	\a \b \f \n \r \t \v
//	\0           NUL (\NNN with no following digits)
//	\NNN         up to 3 octal digits
//	\xHH         exactly 2 hex digits
//	\uHHHH       exactly 4 hex digits
//	\UHHHHHHHH   exactly 8 hex digits (must be a valid code point)
//
// Unlike Python's codec, `\N{NAME}` is rejected — see docs/divergences.md.
// An unknown escape (e.g. `\q`) keeps the backslash literal, matching
// Python's default behavior for `unicode-escape`.
func decodeStringLiteral(value string) (string, error) {
	if !strings.ContainsRune(value, '\\') {
		return value, nil
	}
	var b strings.Builder
	b.Grow(len(value))
	i := 0
	for i < len(value) {
		c := value[i]
		if c != '\\' {
			b.WriteByte(c)
			i++
			continue
		}
		if i+1 >= len(value) {
			return "", fmt.Errorf("trailing backslash in string")
		}
		next := value[i+1]
		switch next {
		case '\\', '\'', '"':
			b.WriteByte(next)
			i += 2
		case 'a':
			b.WriteByte('\a')
			i += 2
		case 'b':
			b.WriteByte('\b')
			i += 2
		case 'f':
			b.WriteByte('\f')
			i += 2
		case 'n':
			b.WriteByte('\n')
			i += 2
		case 'r':
			b.WriteByte('\r')
			i += 2
		case 't':
			b.WriteByte('\t')
			i += 2
		case 'v':
			b.WriteByte('\v')
			i += 2
		case '\n':
			// Line-continuation: consume the newline.
			i += 2
		case 'x':
			if i+4 > len(value) {
				return "", fmt.Errorf("truncated \\xHH escape")
			}
			h := value[i+2 : i+4]
			n, err := strconv.ParseUint(h, 16, 8)
			if err != nil {
				return "", fmt.Errorf("invalid \\x escape: %s", h)
			}
			var buf [4]byte
			w := utf8.EncodeRune(buf[:], rune(n))
			b.Write(buf[:w])
			i += 4
		case 'u':
			if i+6 > len(value) {
				return "", fmt.Errorf("truncated \\uHHHH escape")
			}
			h := value[i+2 : i+6]
			n, err := strconv.ParseUint(h, 16, 16)
			if err != nil {
				return "", fmt.Errorf("invalid \\u escape: %s", h)
			}
			var buf [4]byte
			w := utf8.EncodeRune(buf[:], rune(n))
			b.Write(buf[:w])
			i += 6
		case 'U':
			if i+10 > len(value) {
				return "", fmt.Errorf("truncated \\UHHHHHHHH escape")
			}
			h := value[i+2 : i+10]
			n, err := strconv.ParseUint(h, 16, 21)
			if err != nil {
				return "", fmt.Errorf("invalid \\U escape: %s", h)
			}
			r := rune(n)
			if !utf8.ValidRune(r) {
				return "", fmt.Errorf("invalid Unicode code point U+%08X", n)
			}
			var buf [4]byte
			w := utf8.EncodeRune(buf[:], r)
			b.Write(buf[:w])
			i += 10
		case 'N':
			return "", fmt.Errorf(`\N{NAME} escapes are not supported by gojinja (deliberate divergence)`)
		default:
			if next >= '0' && next <= '7' {
				// Octal: up to 3 digits. Already 1 (next).
				j := i + 2
				count := 1
				for count < 3 && j < len(value) && value[j] >= '0' && value[j] <= '7' {
					j++
					count++
				}
				digits := value[i+1 : j]
				n, err := strconv.ParseUint(digits, 8, 9)
				if err != nil {
					return "", fmt.Errorf("invalid octal escape \\%s", digits)
				}
				var buf [4]byte
				w := utf8.EncodeRune(buf[:], rune(n))
				b.Write(buf[:w])
				i = j
				continue
			}
			// Unknown escape: keep the backslash literally (Python codec
			// behaviour for `unicode-escape`).
			b.WriteByte('\\')
			b.WriteByte(next)
			i += 2
		}
	}
	return b.String(), nil
}
