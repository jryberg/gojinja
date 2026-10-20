// Package pyescape implements Python's unicode_escape and
// raw_unicode_escape codecs, shared by template string literals (which
// Jinja2 decodes with unicode_escape) and str.encode/bytes.decode.
//
// Go strings cannot hold lone surrogates, so an escape that yields one
// (\ud800) decodes to U+FFFD; see the divergences table.
package pyescape

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/jryberg/gojinja/internal/unicodename"
)

// Handler applies a decode error handler to the input bytes
// [start, end), failing with the given reason or writing a replacement.
type Handler func(start, end int, reason string) error

// Strict is the strict error handler; its error is the bare reason, which
// is what Jinja2 reports for a bad string literal.
func Strict(_, _ int, reason string) error { return fmt.Errorf("%s", reason) }

// BackslashReplace is s.encode("ascii", "backslashreplace").
func BackslashReplace(s string) []byte {
	out := make([]byte, 0, len(s))
	for _, r := range s {
		switch {
		case r < 0x80:
			out = append(out, byte(r))
		case r < 0x100:
			out = fmt.Appendf(out, `\x%02x`, r)
		case r < 0x10000:
			out = fmt.Appendf(out, `\u%04x`, r)
		default:
			out = fmt.Appendf(out, `\U%08x`, r)
		}
	}
	return out
}

// Encode is the unicode_escape encoder, or with raw set the
// raw_unicode_escape encoder, which only escapes characters above 0xff.
func Encode(rs []rune, raw bool) []byte {
	out := []byte{}
	for _, r := range rs {
		switch {
		case r >= 0x10000:
			out = fmt.Appendf(out, `\U%08x`, r)
		case r >= 0x100:
			out = fmt.Appendf(out, `\u%04x`, r)
		case raw:
			out = append(out, byte(r))
		case r == '\\':
			out = append(out, `\\`...)
		case r == '\t':
			out = append(out, `\t`...)
		case r == '\n':
			out = append(out, `\n`...)
		case r == '\r':
			out = append(out, `\r`...)
		case r < 0x20 || r >= 0x7f:
			out = fmt.Appendf(out, `\x%02x`, r)
		default:
			out = append(out, byte(r))
		}
	}
	return out
}

func hexValue(c byte) (rune, bool) {
	switch {
	case c >= '0' && c <= '9':
		return rune(c - '0'), true
	case c >= 'a' && c <= 'f':
		return rune(c-'a') + 10, true
	case c >= 'A' && c <= 'F':
		return rune(c-'A') + 10, true
	}
	return 0, false
}

var simpleEscapes = map[byte]byte{
	'\\': '\\', '\'': '\'', '"': '"', 'b': '\b', 'f': '\f', 't': '\t', 'n': '\n', 'r': '\r', 'v': '\v', 'a': '\a',
}

// Decode is the unicode_escape decoder (bytes read as Latin-1, Python
// string-literal escapes interpreted), or with raw set the
// raw_unicode_escape decoder, which only knows \u and \U. Output goes to
// sb, where fail may also write replacements.
func Decode(b []byte, raw bool, sb *strings.Builder, fail Handler) (string, error) {
	for s := 0; s < len(b); {
		if b[s] != '\\' {
			sb.WriteRune(rune(b[s]))
			s++
			continue
		}
		start := s
		if raw {
			// \u escapes count only after an odd number of backslashes.
			j := s
			for j < len(b) && b[j] == '\\' {
				j++
			}
			n := j - s
			if n%2 == 0 || j >= len(b) || b[j] != 'u' && b[j] != 'U' {
				sb.WriteString(strings.Repeat(`\`, n))
				s = j
				continue
			}
			sb.WriteString(strings.Repeat(`\`, n-1))
			start = j - 1
			s = j
		} else {
			s++
			if s >= len(b) {
				if err := fail(start, s, `\ at end of string`); err != nil {
					return "", err
				}
				break
			}
		}
		c := b[s]
		s++
		if out, ok := simpleEscapes[c]; ok && !raw {
			sb.WriteByte(out)
			continue
		}
		digits := 0
		switch c {
		case '\n':
			continue
		case '0', '1', '2', '3', '4', '5', '6', '7':
			v := rune(c - '0')
			for k := 0; k < 2 && s < len(b) && b[s] >= '0' && b[s] <= '7'; k++ {
				v = v*8 + rune(b[s]-'0')
				s++
			}
			sb.WriteRune(v)
			continue
		case 'x':
			digits = 2
		case 'u':
			digits = 4
		case 'U':
			digits = 8
		case 'N':
			// \N{name}: CPython's error ranges end after 'N' when no
			// brace follows, at the '}' of an empty name, at the end of an
			// unclosed one, and after the '}' of an unknown name.
			reason := `malformed \N character escape`
			if s < len(b) && b[s] == '{' {
				end := strings.IndexByte(string(b[s+1:]), '}')
				switch {
				case end < 0:
					s = len(b)
				case end == 0:
					s++
				default:
					name := string(b[s+1 : s+1+end])
					s += end + 2
					if r, ok := unicodename.Lookup(name); ok {
						sb.WriteRune(r)
						continue
					}
					reason = "unknown Unicode character name"
				}
			}
			if err := fail(start, s, reason); err != nil {
				return "", err
			}
			continue
		default:
			// Unknown escapes are kept, the byte read as Latin-1.
			sb.WriteByte('\\')
			sb.WriteRune(rune(c))
			continue
		}
		var v rune
		k := 0
		for ; k < digits && s < len(b); k++ {
			h, ok := hexValue(b[s])
			if !ok {
				break
			}
			v = v<<4 | h
			s++
		}
		if k < digits {
			names := map[int]string{2: `\xXX`, 4: `\uXXXX`, 8: `\UXXXXXXXX`}
			if err := fail(start, s, "truncated "+names[digits]+" escape"); err != nil {
				return "", err
			}
			continue
		}
		if v > 0x10ffff {
			if err := fail(start, s, "illegal Unicode character"); err != nil {
				return "", err
			}
			continue
		}
		if v >= 0xd800 && v < 0xe000 {
			v = utf8.RuneError
		}
		sb.WriteRune(v)
	}
	return sb.String(), nil
}
