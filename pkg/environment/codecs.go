package environment

import (
	"encoding/binary"
	"fmt"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/jryberg/gojinja/internal/pyescape"
	"github.com/jryberg/gojinja/internal/unicodename"
	"github.com/jryberg/gojinja/pkg/runtime"
)

// Python's text codecs for str.encode and bytes.decode. Single-byte
// codecs and codec name aliases come from codectables_table.go, generated
// from the Python that runs the pinned Jinja2; the others are ports of
// CPython's implementations.

type codecKind int

const (
	kindUTF8 codecKind = iota
	kindUTF8Sig
	kindLimit // ascii, latin-1 and charmap: code points up to limit are one byte
	kindWide  // UTF-16 and UTF-32
	kindTable // single-byte charmap codecs
	kindUTF7
	kindUnicodeEscape
	kindRawUnicodeEscape
	kindPunycode
	kindUndefined
	kindCJK
)

type codec struct {
	kind  codecKind
	name  string // name used in error messages
	limit rune
	width int // 2 or 4 for kindWide
	order byteOrder
	bom   bool
	table *charmapTable
	cjk   *cjkCodec
	// encodeMap is the inverse of table.decode with its overrides applied.
	encodeMap map[rune]byte
}

// byteOrder reads and appends fixed-width units.
type byteOrder interface {
	binary.ByteOrder
	binary.AppendByteOrder
}

type charmapTable struct {
	decode [256]rune
	encode map[rune]int
}

const charmapUndefined = 0xfffe

var builtinCodecs = map[string]codec{
	"utf_8":     {kind: kindUTF8, name: "utf-8"},
	"utf_8_sig": {kind: kindUTF8Sig, name: "utf-8"},
	"ascii":     {kind: kindLimit, name: "ascii", limit: 0x7f},
	"latin_1":   {kind: kindLimit, name: "latin-1", limit: 0xff},
	// Without a mapping table the charmap codec is latin-1.
	"charmap":            {kind: kindLimit, name: "latin-1", limit: 0xff},
	"utf_16":             {kind: kindWide, name: "utf-16", width: 2, order: binary.LittleEndian, bom: true},
	"utf_16_le":          {kind: kindWide, name: "utf-16-le", width: 2, order: binary.LittleEndian},
	"utf_16_be":          {kind: kindWide, name: "utf-16-be", width: 2, order: binary.BigEndian},
	"utf_32":             {kind: kindWide, name: "utf-32", width: 4, order: binary.LittleEndian, bom: true},
	"utf_32_le":          {kind: kindWide, name: "utf-32-le", width: 4, order: binary.LittleEndian},
	"utf_32_be":          {kind: kindWide, name: "utf-32-be", width: 4, order: binary.BigEndian},
	"utf_7":              {kind: kindUTF7, name: "utf7"},
	"unicode_escape":     {kind: kindUnicodeEscape, name: "unicodeescape"},
	"raw_unicode_escape": {kind: kindRawUnicodeEscape, name: "rawunicodeescape"},
	"punycode":           {kind: kindPunycode, name: "punycode"},
	"undefined":          {kind: kindUndefined, name: "undefined"},
}

// errNotTextEncoding marks codecs that exist but are not text encodings;
// pyEncode and pyDecode word the error.
var errNotTextEncoding = fmt.Errorf("not a text encoding")

// lookupCodec resolves an encoding name like codecs.lookup: the name is
// lower-cased and normalised (runs of other characters than letters,
// digits and '.' become '_'), then resolved through Python's aliases.
func lookupCodec(encoding string) (codec, error) {
	var b strings.Builder
	pending := false
	for _, r := range strings.ToLower(encoding) {
		switch {
		case r == '.' || r < utf8.RuneSelf && (r >= 'a' && r <= 'z' || r >= '0' && r <= '9'):
			if pending && b.Len() > 0 {
				b.WriteByte('_')
			}
			pending = false
			b.WriteRune(r)
		case r >= utf8.RuneSelf && isPyAlnum(r):
			// Non-ASCII alphanumerics are dropped without separating.
			if pending && b.Len() > 0 {
				b.WriteByte('_')
			}
			pending = false
		default:
			pending = true
		}
	}
	// encodings.search_function: try the alias (also with '.' as '_'),
	// then the normalised name itself, as a module name without dots.
	norm := b.String()
	var candidates []string
	if alias, ok := codecAliases[norm]; ok {
		candidates = append(candidates, alias)
	} else if alias, ok := codecAliases[strings.ReplaceAll(norm, ".", "_")]; ok {
		candidates = append(candidates, alias)
	}
	candidates = append(candidates, norm)
	module, text := "", false
	for _, m := range candidates {
		if t, ok := codecModules[m]; ok && m != "" && !strings.Contains(m, ".") {
			module, text = m, t
			break
		}
	}
	if module == "" {
		return codec{}, fmt.Errorf("unknown encoding: %s", encoding)
	}
	if !text {
		return codec{}, errNotTextEncoding
	}
	if c, ok := builtinCodecs[module]; ok {
		return c, nil
	}
	if cj, ok := cjkLookup(module); ok {
		return codec{kind: kindCJK, name: module, cjk: cj}, nil
	}
	t := charmapTables[module]
	c := codec{kind: kindTable, name: "charmap", table: &t, encodeMap: make(map[rune]byte, 256)}
	for i := 255; i >= 0; i-- {
		if r := t.decode[i]; r != charmapUndefined {
			c.encodeMap[r] = byte(i)
		}
	}
	for r, v := range t.encode {
		if v < 0 {
			delete(c.encodeMap, r)
		} else {
			c.encodeMap[r] = byte(v)
		}
	}
	return c, nil
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
	if err == errNotTextEncoding {
		return nil, fmt.Errorf("'%s' is not a text encoding; use codecs.encode() to handle arbitrary codecs", encoding)
	}
	if err != nil {
		return nil, err
	}
	rs := []rune(s)
	switch c.kind {
	case kindUTF8:
		return runtime.Bytes(s), nil
	case kindUTF8Sig:
		return runtime.Bytes("\ufeff" + s), nil
	case kindWide:
		var out []byte
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
	case kindUTF7:
		return encodeUTF7(rs), nil
	case kindUnicodeEscape:
		return pyescape.Encode(rs, false), nil
	case kindRawUnicodeEscape:
		return pyescape.Encode(rs, true), nil
	case kindPunycode:
		return punycodeEncode(rs), nil
	case kindUndefined:
		return nil, fmt.Errorf("undefined encoding")
	case kindCJK:
		return c.cjk.encodeCJK(rs, errorsMode)
	}
	return encodeSingleByte(c, rs, errorsMode)
}

// encodeSingleByte encodes with ascii, latin-1, charmap or a charmap
// table, applying the error handler to unencodable characters.
func encodeSingleByte(c codec, rs []rune, errorsMode string) (runtime.Bytes, error) {
	encodeOne := func(r rune) (byte, bool) {
		if c.kind == kindTable {
			b, ok := c.encodeMap[r]
			return b, ok
		}
		return byte(r), r <= c.limit
	}
	reason := "character maps to <undefined>"
	if c.kind == kindLimit {
		reason = fmt.Sprintf("ordinal not in range(%d)", c.limit+1)
	}
	out := runtime.Bytes{}
	for i := 0; i < len(rs); {
		if b, ok := encodeOne(rs[i]); ok {
			out = append(out, b)
			i++
			continue
		}
		// Unencodable characters are handled a run at a time, as CPython's
		// error handlers are.
		end := i + 1
		for end < len(rs) {
			if _, ok := encodeOne(rs[end]); ok {
				break
			}
			end++
		}
		var repl []rune
		switch errorsMode {
		case "strict":
			what := "characters"
			if end-i == 1 {
				what = "character " + charRepr(rs[i])
			}
			return nil, fmt.Errorf("'%s' codec can't encode %s %s: %s", c.name, what, positionRange(i, end), reason)
		case "ignore":
		case "replace":
			repl = []rune(strings.Repeat("?", end-i))
		case "backslashreplace":
			for _, r := range rs[i:end] {
				repl = append(repl, []rune(strings.Trim(charRepr(r), "'"))...)
			}
		case "xmlcharrefreplace":
			for _, r := range rs[i:end] {
				repl = append(repl, []rune(fmt.Sprintf("&#%d;", r))...)
			}
		case "namereplace":
			for _, r := range rs[i:end] {
				repl = append(repl, []rune(nameReplace(r))...)
			}
		default:
			return nil, fmt.Errorf("unknown error handler name '%s'", errorsMode)
		}
		// The replacement must itself be encodable.
		for _, r := range repl {
			b, ok := encodeOne(r)
			if !ok {
				return nil, fmt.Errorf("'%s' codec can't encode %s %s: %s", c.name, "character "+charRepr(rs[i]), positionRange(i, end), reason)
			}
			out = append(out, b)
		}
		i = end
	}
	return out, nil
}

// nameReplace is the namereplace handler's replacement for r: \N{NAME},
// or a backslash escape when r has no name.
func nameReplace(r rune) string {
	if name, ok := unicodename.Name(r); ok {
		return `\N{` + name + "}"
	}
	return strings.Trim(charRepr(r), "'")
}

func (c codec) appendUnit(out []byte, u uint32) []byte {
	if c.width == 2 {
		return c.order.AppendUint16(out, uint16(u))
	}
	return c.order.AppendUint32(out, u)
}

// decodeErrors applies a decode error handler to b[start:end].
type decodeErrors struct {
	codec string
	mode  string
	b     []byte
	sb    *strings.Builder
}

func (d decodeErrors) fail(start, end int, reason string) error {
	switch d.mode {
	case "strict":
		what := fmt.Sprintf("byte 0x%02x", d.b[start])
		if end-start > 1 {
			what = "bytes"
		}
		return fmt.Errorf("'%s' codec can't decode %s %s: %s", d.codec, what, positionRange(start, end), reason)
	case "ignore":
	case "replace":
		d.sb.WriteRune(utf8.RuneError)
	case "backslashreplace":
		for _, x := range d.b[start:end] {
			fmt.Fprintf(d.sb, `\x%02x`, x)
		}
	case "xmlcharrefreplace", "namereplace":
		return fmt.Errorf("don't know how to handle UnicodeDecodeError in error callback")
	default:
		return fmt.Errorf("unknown error handler name '%s'", d.mode)
	}
	return nil
}

// pyDecode is bytes.decode(encoding, errors).
func pyDecode(b []byte, encoding, errorsMode string) (string, error) {
	// CPython returns '' for empty input before looking the codec up.
	if len(b) == 0 {
		return "", nil
	}
	c, err := lookupCodec(encoding)
	if err == errNotTextEncoding {
		return "", fmt.Errorf("'%s' is not a text encoding; use codecs.decode() to handle arbitrary codecs", encoding)
	}
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	d := decodeErrors{codec: c.name, mode: errorsMode, b: b, sb: &sb}
	switch c.kind {
	case kindUndefined:
		return "", fmt.Errorf("undefined encoding")
	case kindWide:
		return decodeWide(c, b, d)
	case kindUTF7:
		return decodeUTF7(b, d)
	case kindUnicodeEscape:
		return pyescape.Decode(b, false, &sb, d.fail)
	case kindRawUnicodeEscape:
		return pyescape.Decode(b, true, &sb, d.fail)
	case kindPunycode:
		return punycodeDecode(b, errorsMode)
	case kindCJK:
		return c.cjk.decode(b, d)
	case kindTable:
		for i, x := range b {
			if r := c.table.decode[x]; r != charmapUndefined {
				sb.WriteRune(r)
			} else if err := d.fail(i, i+1, "character maps to <undefined>"); err != nil {
				return "", err
			}
		}
	case kindLimit:
		for i, x := range b {
			if rune(x) > c.limit {
				if err := d.fail(i, i+1, fmt.Sprintf("ordinal not in range(%d)", c.limit+1)); err != nil {
					return "", err
				}
				continue
			}
			sb.WriteRune(rune(x))
		}
	default:
		if c.kind == kindUTF8Sig && len(b) >= 3 && string(b[:3]) == "\ufeff" {
			b = b[3:]
			d.b = b
		}
		for i := 0; i < len(b); {
			n, reason := utf8Sequence(b[i:])
			if reason != "" {
				if err := d.fail(i, i+n, reason); err != nil {
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

func decodeWide(c codec, b []byte, d decodeErrors) (string, error) {
	var order binary.ByteOrder = c.order
	sb := d.sb
	i := 0
	if c.bom {
		// Errors name the byte order in effect, native (little) unless a
		// BOM says otherwise.
		d.codec = c.name + "-le"
		if len(b) >= c.width {
			switch {
			case c.width == 2 && b[0] == 0xff && b[1] == 0xfe, c.width == 4 && b[0] == 0xff && b[1] == 0xfe && b[2] == 0 && b[3] == 0:
				order, i = binary.LittleEndian, c.width
			case c.width == 2 && b[0] == 0xfe && b[1] == 0xff, c.width == 4 && b[0] == 0 && b[1] == 0 && b[2] == 0xfe && b[3] == 0xff:
				order, i, d.codec = binary.BigEndian, c.width, c.name+"-be"
			}
		}
	}
	for i < len(b) {
		if len(b)-i < c.width {
			if err := d.fail(i, len(b), "truncated data"); err != nil {
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
				if err := d.fail(i, i+4, reason); err != nil {
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
			if err := d.fail(i, i+2, "illegal encoding"); err != nil {
				return "", err
			}
			i += 2
		case len(b)-i < 4:
			if err := d.fail(i, len(b), "unexpected end of data"); err != nil {
				return "", err
			}
			i = len(b)
		default:
			lo := order.Uint16(b[i+2:])
			if lo < 0xdc00 || lo >= 0xe000 {
				if err := d.fail(i, i+2, "illegal UTF-16 surrogate"); err != nil {
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

// utf7Category is CPython's utf7_category: 0 set D, 1 set O, 2
// whitespace, 3 special (always base64-encoded).
var utf7Category = [128]byte{
	3, 3, 3, 3, 3, 3, 3, 3, 3, 2, 2, 3, 3, 2, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	2, 1, 1, 1, 1, 1, 1, 0, 0, 0, 1, 3, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 0,
	1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 3, 1, 1, 1,
	1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 3, 3,
}

const base64Alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

func isBase64(c rune) bool { return c < 128 && strings.ContainsRune(base64Alphabet, c) }

// utf7Direct is ENCODE_DIRECT with set O and whitespace written directly.
func utf7Direct(c rune) bool { return c > 0 && c < 128 && utf7Category[c] != 3 }

// encodeUTF7 is CPython's _PyUnicode_EncodeUTF7 as str.encode('utf-7')
// calls it.
func encodeUTF7(rs []rune) runtime.Bytes {
	out := runtime.Bytes{}
	inShift := false
	bits, buf := 0, uint64(0)
	emit := func(u uint64) {
		bits += 16
		buf = buf<<16 | u
		for bits >= 6 {
			out = append(out, base64Alphabet[(buf>>(bits-6))&0x3f])
			bits -= 6
		}
		buf &= (1 << bits) - 1
	}
	for _, ch := range rs {
		if inShift && utf7Direct(ch) {
			if bits > 0 {
				out = append(out, base64Alphabet[(buf<<(6-bits))&0x3f])
				bits, buf = 0, 0
			}
			inShift = false
			if isBase64(ch) || ch == '-' {
				out = append(out, '-')
			}
			out = append(out, byte(ch))
			continue
		}
		if !inShift {
			switch {
			case ch == '+':
				out = append(out, '+', '-')
				continue
			case utf7Direct(ch):
				out = append(out, byte(ch))
				continue
			}
			out = append(out, '+')
			inShift = true
		}
		if ch >= 0x10000 {
			hi, lo := utf16.EncodeRune(ch)
			emit(uint64(hi))
			emit(uint64(lo))
		} else {
			emit(uint64(ch))
		}
	}
	if bits > 0 {
		out = append(out, base64Alphabet[(buf<<(6-bits))&0x3f])
	}
	if inShift {
		out = append(out, '-')
	}
	return out
}

// decodeUTF7 is CPython's PyUnicode_DecodeUTF7Stateful (final=True). A
// lone surrogate, which Go strings cannot hold, becomes U+FFFD.
func decodeUTF7(b []byte, d decodeErrors) (string, error) {
	sb := d.sb
	inShift := false
	bits, buf := 0, uint64(0)
	var surrogate rune
	start := 0
	for s := 0; s < len(b); {
		ch := rune(b[s])
		if inShift {
			if isBase64(ch) {
				buf = buf<<6 | uint64(strings.IndexRune(base64Alphabet, ch))
				bits += 6
				s++
				if bits >= 16 {
					out := rune(buf >> (bits - 16))
					bits -= 16
					buf &= (1 << bits) - 1
					if surrogate != 0 {
						if out >= 0xdc00 && out < 0xe000 {
							sb.WriteRune(utf16.DecodeRune(surrogate, out))
							surrogate = 0
							continue
						}
						sb.WriteRune(utf8.RuneError)
						surrogate = 0
					}
					switch {
					case out >= 0xd800 && out < 0xdc00:
						surrogate = out
					case out >= 0xdc00 && out < 0xe000:
						sb.WriteRune(utf8.RuneError)
					default:
						sb.WriteRune(out)
					}
				}
				continue
			}
			inShift = false
			var reason string
			switch {
			case bits >= 6:
				reason = "partial character in shift sequence"
			case buf != 0:
				reason = "non-zero padding bits in shift sequence"
			}
			if reason != "" {
				s++
				if err := d.fail(start, s, reason); err != nil {
					return "", err
				}
				continue
			}
			if surrogate != 0 && ch <= 127 && ch != '+' {
				sb.WriteRune(utf8.RuneError)
			}
			surrogate = 0
			if ch == '-' {
				s++
			}
			continue
		}
		switch {
		case ch == '+':
			start = s
			s++
			switch {
			case s < len(b) && b[s] == '-':
				s++
				sb.WriteByte('+')
			case s < len(b) && !isBase64(rune(b[s])):
				s++
				if err := d.fail(start, s, "ill-formed sequence"); err != nil {
					return "", err
				}
			default:
				inShift, surrogate, bits, buf = true, 0, 0, 0
			}
		case ch <= 127:
			s++
			sb.WriteRune(ch)
		default:
			start = s
			s++
			if err := d.fail(start, s, "unexpected special character"); err != nil {
				return "", err
			}
		}
	}
	if inShift && (surrogate != 0 || bits >= 6 || bits > 0 && buf != 0) {
		if err := d.fail(start, len(b), "unterminated shift sequence"); err != nil {
			return "", err
		}
	}
	return sb.String(), nil
}

// Punycode (RFC 3492), ported from Python's encodings/punycode.py.

const (
	punyBase        = 36
	punyTMin        = 1
	punyTMax        = 26
	punyInitialBias = 72
	punyInitialN    = 128
	punyDigits      = "abcdefghijklmnopqrstuvwxyz0123456789"
)

func punyAdapt(delta, numchars int, first bool) int {
	if first {
		delta /= 700
	} else {
		delta /= 2
	}
	delta += delta / numchars
	divisions := 0
	for delta > ((punyBase-punyTMin)*punyTMax)/2 {
		delta /= punyBase - punyTMin
		divisions += punyBase
	}
	return divisions + (punyBase-punyTMin+1)*delta/(delta+38)
}

func punyT(j, bias int) int {
	return max(punyTMin, min(punyBase*(j+1)-bias, punyTMax))
}

func punycodeEncode(rs []rune) runtime.Bytes {
	out := runtime.Bytes{}
	seen := map[rune]bool{}
	var extended []rune
	for _, r := range rs {
		if r < 128 {
			out = append(out, byte(r))
		} else if !seen[r] {
			seen[r] = true
			extended = append(extended, r)
		}
	}
	basicLen := len(out)
	if basicLen > 0 {
		out = append(out, '-')
	}
	for i := 1; i < len(extended); i++ {
		for j := i; j > 0 && extended[j] < extended[j-1]; j-- {
			extended[j], extended[j-1] = extended[j-1], extended[j]
		}
	}
	n, delta, bias, h := punyInitialN, 0, punyInitialBias, basicLen
	for _, m := range extended {
		delta += (int(m) - n) * (h + 1)
		n = int(m)
		for _, r := range rs {
			if int(r) < n {
				delta++
			}
			if int(r) == n {
				q := delta
				for j := 0; ; j++ {
					t := punyT(j, bias)
					if q < t {
						out = append(out, punyDigits[q])
						break
					}
					out = append(out, punyDigits[t+(q-t)%(punyBase-t)])
					q = (q - t) / (punyBase - t)
				}
				bias = punyAdapt(delta, h+1, h == basicLen)
				delta = 0
				h++
			}
		}
		delta++
		n++
	}
	return out
}

func punycodeDecode(b []byte, errorsMode string) (string, error) {
	if errorsMode != "strict" && errorsMode != "replace" && errorsMode != "ignore" {
		//lint:ignore ST1005 Python's exact message
		return "", fmt.Errorf("Unsupported error handling %s", errorsMode)
	}
	// Python's punycode_decode: the part before the last '-' is ASCII
	// decoded with the error handler, the rest strictly and upper-cased.
	asciiDecode := func(part []byte, mode string) (string, error) {
		var sb strings.Builder
		d := decodeErrors{codec: "ascii", mode: mode, b: part, sb: &sb}
		for k, c := range part {
			if c >= 0x80 {
				if err := d.fail(k, k+1, "ordinal not in range(128)"); err != nil {
					return "", err
				}
				continue
			}
			sb.WriteByte(c)
		}
		return sb.String(), nil
	}
	pos := strings.LastIndexByte(string(b), '-')
	var base, ext string
	var err error
	if pos < 0 {
		ext, err = asciiDecode(b, "strict")
	} else {
		if base, err = asciiDecode(b[:pos], errorsMode); err == nil {
			ext, err = asciiDecode(b[pos+1:], "strict")
		}
	}
	if err != nil {
		return "", err
	}
	ext = strings.ToUpper(ext)
	chars := []rune(base)
	char, at, bias := punyInitialN, -1, punyInitialBias
	for extpos := 0; extpos < len(ext); {
		// decode_generalized_number
		delta, w := 0, 1
		for j := 0; ; j++ {
			if extpos >= len(ext) {
				if errorsMode == "strict" {
					return "", fmt.Errorf("incomplete punicode string")
				}
				return string(chars), nil
			}
			c := ext[extpos]
			extpos++
			var digit int
			switch {
			case c >= 'A' && c <= 'Z':
				digit = int(c - 'A')
			case c >= '0' && c <= '9':
				digit = int(c) - 22
			default:
				if errorsMode == "strict" {
					//lint:ignore ST1005 Python's exact message
					return "", fmt.Errorf("Invalid extended code point '%c'", c)
				}
				return string(chars), nil
			}
			t := punyT(j, bias)
			delta += digit * w
			if digit < t {
				break
			}
			w *= punyBase - t
		}
		first := at == -1 && len(chars) == len([]rune(base))
		at += delta + 1
		char += at / (len(chars) + 1)
		if char > 0x10ffff {
			if errorsMode == "strict" {
				//lint:ignore ST1005 Python's exact message
				return "", fmt.Errorf("Invalid character U+%x", char)
			}
			char = '?'
		}
		at %= len(chars) + 1
		chars = append(chars[:at], append([]rune{rune(char)}, chars[at:]...)...)
		bias = punyAdapt(delta, len(chars), first)
	}
	return string(chars), nil
}
