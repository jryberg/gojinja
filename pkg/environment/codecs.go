package environment

import (
	"encoding/binary"
	"fmt"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/jryberg/gojinja/pkg/runtime"
)

// Python's codecs for str.encode and bytes.decode, limited to the ones the
// Go standard library can implement exactly: UTF-8, ASCII, Latin-1,
// UTF-16 and UTF-32 (with and without byte order).

// byteOrder reads and appends fixed-width units.
type byteOrder interface {
	binary.ByteOrder
	binary.AppendByteOrder
}

type codec struct {
	name  string // name used in error messages
	limit rune   // highest encodable code point for single-byte codecs
	width int    // 2 or 4 for UTF-16/32, 0 otherwise
	order byteOrder
	bom   bool
}

var codecAliases = map[string]string{
	"utf_8": "utf_8", "u8": "utf_8", "utf": "utf_8", "utf8": "utf_8", "utf8_ucs2": "utf_8", "utf8_ucs4": "utf_8", "cp65001": "utf_8",
	"ascii": "ascii", "646": "ascii", "ansi_x3.4_1968": "ascii", "ansi_x3_4_1968": "ascii", "ansi_x3.4_1986": "ascii", "cp367": "ascii",
	"csascii": "ascii", "ibm367": "ascii", "iso646_us": "ascii", "iso_646.irv_1991": "ascii", "iso_ir_6": "ascii", "us": "ascii", "us_ascii": "ascii",
	"latin_1": "latin_1", "8859": "latin_1", "cp819": "latin_1", "csisolatin1": "latin_1", "ibm819": "latin_1", "iso8859": "latin_1",
	"iso8859_1": "latin_1", "iso_8859_1": "latin_1", "iso_8859_1_1987": "latin_1", "iso_ir_100": "latin_1", "l1": "latin_1", "latin": "latin_1", "latin1": "latin_1",
	"utf_16": "utf_16", "u16": "utf_16", "utf16": "utf_16",
	"utf_16_le": "utf_16_le", "utf_16le": "utf_16_le", "unicodelittleunmarked": "utf_16_le",
	"utf_16_be": "utf_16_be", "utf_16be": "utf_16_be", "unicodebigunmarked": "utf_16_be",
	"utf_32": "utf_32", "u32": "utf_32", "utf32": "utf_32",
	"utf_32_le": "utf_32_le", "utf_32le": "utf_32_le",
	"utf_32_be": "utf_32_be", "utf_32be": "utf_32_be",
}

var codecs = map[string]codec{
	"utf_8":     {name: "utf-8"},
	"ascii":     {name: "ascii", limit: 0x7f},
	"latin_1":   {name: "latin-1", limit: 0xff},
	"utf_16":    {name: "utf-16", width: 2, order: binary.LittleEndian, bom: true},
	"utf_16_le": {name: "utf-16-le", width: 2, order: binary.LittleEndian},
	"utf_16_be": {name: "utf-16-be", width: 2, order: binary.BigEndian},
	"utf_32":    {name: "utf-32", width: 4, order: binary.LittleEndian, bom: true},
	"utf_32_le": {name: "utf-32-le", width: 4, order: binary.LittleEndian},
	"utf_32_be": {name: "utf-32-be", width: 4, order: binary.BigEndian},
}

// lookupCodec normalises an encoding name like encodings.normalize_encoding
// and resolves its alias.
func lookupCodec(encoding string) (codec, error) {
	var b strings.Builder
	pending := false
	for _, r := range strings.ToLower(encoding) {
		if r == '.' || r < utf8.RuneSelf && (r >= 'a' && r <= 'z' || r >= '0' && r <= '9') {
			if pending && b.Len() > 0 {
				b.WriteByte('_')
			}
			pending = false
			b.WriteRune(r)
		} else {
			pending = true
		}
	}
	if name, ok := codecAliases[b.String()]; ok {
		return codecs[name], nil
	}
	return codec{}, fmt.Errorf("unknown encoding: %s", encoding)
}

// charRepr formats a code point the way UnicodeEncodeError messages do.
func charRepr(r rune) string {
	switch {
	case r < 0x100:
		return fmt.Sprintf(`'\x%02x'`, r)
	case r < 0x10000:
		return fmt.Sprintf(`'\u%04x'`, r)
	}
	return fmt.Sprintf(`'\U%08x'`, r)
}

func positionRange(start, end int) string {
	if end-start == 1 {
		return fmt.Sprintf("in position %d", start)
	}
	return fmt.Sprintf("in position %d-%d", start, end-1)
}

// pyEncode is str.encode(encoding, errors).
func pyEncode(s, encoding, errorsMode string) (runtime.Bytes, error) {
	c, err := lookupCodec(encoding)
	if err != nil {
		return nil, err
	}
	rs := []rune(s)
	var out []byte
	if c.width > 0 {
		if c.bom {
			out = c.appendUnit(out, 0xfeff)
		}
		for _, r := range rs {
			if c.width == 4 {
				out = c.appendUnit(out, uint32(r))
				continue
			}
			for _, u := range utf16.Encode([]rune{r}) {
				out = c.appendUnit(out, uint32(u))
			}
		}
		return out, nil
	}
	if c.limit == 0 {
		return runtime.Bytes(s), nil
	}
	for i := 0; i < len(rs); {
		if rs[i] <= c.limit {
			out = append(out, byte(rs[i]))
			i++
			continue
		}
		// Unencodable characters are handled a run at a time, as CPython's
		// error handlers are.
		end := i
		for end < len(rs) && rs[end] > c.limit {
			end++
		}
		switch errorsMode {
		case "strict":
			what := "characters"
			if end-i == 1 {
				what = "character " + charRepr(rs[i])
			}
			return nil, fmt.Errorf("'%s' codec can't encode %s %s: ordinal not in range(%d)", c.name, what, positionRange(i, end), c.limit+1)
		case "ignore":
		case "replace":
			out = append(out, strings.Repeat("?", end-i)...)
		case "backslashreplace":
			for _, r := range rs[i:end] {
				out = append(out, strings.Trim(charRepr(r), "'")...)
			}
		case "xmlcharrefreplace":
			for _, r := range rs[i:end] {
				out = append(out, fmt.Sprintf("&#%d;", r)...)
			}
		default:
			return nil, fmt.Errorf("unknown error handler name '%s'", errorsMode)
		}
		i = end
	}
	return out, nil
}

func (c codec) appendUnit(out []byte, u uint32) []byte {
	if c.width == 2 {
		return c.order.AppendUint16(out, uint16(u))
	}
	return c.order.AppendUint32(out, u)
}

// pyDecode is bytes.decode(encoding, errors).
func pyDecode(b []byte, encoding, errorsMode string) (string, error) {
	c, err := lookupCodec(encoding)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	// fail applies the error handler to the undecodable bytes b[start:end].
	fail := func(start, end int, reason string) error {
		switch errorsMode {
		case "strict":
			what := fmt.Sprintf("byte 0x%02x", b[start])
			if end-start > 1 {
				what = "bytes"
			}
			return fmt.Errorf("'%s' codec can't decode %s %s: %s", c.name, what, positionRange(start, end), reason)
		case "ignore":
		case "replace":
			sb.WriteRune(utf8.RuneError)
		case "backslashreplace":
			for _, x := range b[start:end] {
				fmt.Fprintf(&sb, `\x%02x`, x)
			}
		default:
			return fmt.Errorf("unknown error handler name '%s'", errorsMode)
		}
		return nil
	}
	switch {
	case c.width > 0:
		return decodeWide(c, b, &sb, fail)
	case c.limit == 0xff:
		for _, x := range b {
			sb.WriteRune(rune(x))
		}
	case c.limit == 0x7f:
		for i, x := range b {
			if x >= 0x80 {
				if err := fail(i, i+1, "ordinal not in range(128)"); err != nil {
					return "", err
				}
				continue
			}
			sb.WriteByte(x)
		}
	default:
		for i := 0; i < len(b); {
			n, reason := utf8Sequence(b[i:])
			if reason != "" {
				if err := fail(i, i+n, reason); err != nil {
					return "", err
				}
				i += n
				continue
			}
			sb.Write(b[i : i+n])
			i += n
		}
	}
	return sb.String(), nil
}

// utf8Sequence returns the length of the UTF-8 sequence at the start of
// b, or the length of the invalid prefix and CPython's reason for it.
func utf8Sequence(b []byte) (int, string) {
	c := b[0]
	var need int
	lo, hi := byte(0x80), byte(0xbf)
	switch {
	case c < 0x80:
		return 1, ""
	case c >= 0xc2 && c <= 0xdf:
		need = 2
	case c >= 0xe0 && c <= 0xef:
		need = 3
		if c == 0xe0 {
			lo = 0xa0
		} else if c == 0xed {
			hi = 0x9f
		}
	case c >= 0xf0 && c <= 0xf4:
		need = 4
		if c == 0xf0 {
			lo = 0x90
		} else if c == 0xf4 {
			hi = 0x8f
		}
	default:
		return 1, "invalid start byte"
	}
	for k := 1; k < need; k++ {
		if k >= len(b) {
			return len(b), "unexpected end of data"
		}
		if b[k] < lo || b[k] > hi {
			return k, "invalid continuation byte"
		}
		lo, hi = 0x80, 0xbf
	}
	return need, ""
}

func decodeWide(c codec, b []byte, sb *strings.Builder, fail func(int, int, string) error) (string, error) {
	var order binary.ByteOrder = c.order
	i := 0
	if c.bom && len(b) >= c.width {
		switch {
		case c.width == 2 && b[0] == 0xff && b[1] == 0xfe, c.width == 4 && b[0] == 0xff && b[1] == 0xfe && b[2] == 0 && b[3] == 0:
			order, i = binary.LittleEndian, c.width
		case c.width == 2 && b[0] == 0xfe && b[1] == 0xff, c.width == 4 && b[0] == 0 && b[1] == 0 && b[2] == 0xfe && b[3] == 0xff:
			order, i = binary.BigEndian, c.width
		}
	}
	for i < len(b) {
		if len(b)-i < c.width {
			if err := fail(i, len(b), "truncated data"); err != nil {
				return "", err
			}
			break
		}
		if c.width == 4 {
			u := order.Uint32(b[i:])
			if u > 0x10ffff || u >= 0xd800 && u < 0xe000 {
				reason := "code point not in range(0x110000)"
				if u < 0x110000 {
					reason = "code point in surrogate code point range(0xd800, 0xe000)"
				}
				if err := fail(i, i+4, reason); err != nil {
					return "", err
				}
			} else {
				sb.WriteRune(rune(u))
			}
			i += 4
			continue
		}
		u := order.Uint16(b[i:])
		switch {
		case u < 0xd800 || u >= 0xe000:
			sb.WriteRune(rune(u))
			i += 2
		case u >= 0xdc00:
			if err := fail(i, i+2, "illegal encoding"); err != nil {
				return "", err
			}
			i += 2
		case len(b)-i < 4:
			if err := fail(i, len(b), "unexpected end of data"); err != nil {
				return "", err
			}
			i = len(b)
		default:
			lo := order.Uint16(b[i+2:])
			if lo < 0xdc00 || lo >= 0xe000 {
				if err := fail(i, i+2, "illegal UTF-16 surrogate"); err != nil {
					return "", err
				}
				i += 2
				continue
			}
			sb.WriteRune(utf16.DecodeRune(rune(u), rune(lo)))
			i += 4
		}
	}
	return sb.String(), nil
}
