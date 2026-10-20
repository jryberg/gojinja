package environment

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/jryberg/gojinja/pkg/runtime"
)

// The stateful CJK codecs (hz and the iso2022 family) port CPython's
// _codecs_cn.c and _codecs_iso2022.c state machines. Their charset
// tables and per-character encodings are probed from Python like the
// stateless codecs' (tools/parity/cjktables.py); which charsets each
// codec designates, and how, is CPython's configuration below.

const (
	isoESC   = 0x1b
	isoSO    = 0x0e
	isoSI    = 0x0f
	isoLF    = 0x0a
	isoASCII = 'B'
	isoDBCS  = 0x80 // marks a double-byte charset's final byte
)

const (
	isoNoShift     = 1 << iota // SO and SI are plain control characters
	isoUseG2                   // ESC . F designates G2 and ESC N shifts to it once
	isoJISX0208Ext             // ESC & @ may prefix ESC $ B
)

// isoDesignation is a charset (its escape's final byte, isoDBCS for
// double-byte sets) and the G0-G2 plane it is designated to.
type isoDesignation struct {
	mark  byte
	plane int
}

type isoConfig struct {
	flags        int
	designations []isoDesignation
}

// isoConfigs is CPython's iso2022_*_config.
var isoConfigs = map[string]isoConfig{
	"iso2022_kr": {0, []isoDesignation{{'C' | isoDBCS, 1}}},
	"iso2022_jp": {isoNoShift | isoJISX0208Ext, []isoDesignation{
		{'B' | isoDBCS, 0}, {'J', 0}, {'@' | isoDBCS, 0}}},
	"iso2022_jp_1": {isoNoShift | isoJISX0208Ext, []isoDesignation{
		{'B' | isoDBCS, 0}, {'D' | isoDBCS, 0}, {'J', 0}, {'@' | isoDBCS, 0}}},
	"iso2022_jp_2": {isoNoShift | isoUseG2 | isoJISX0208Ext, []isoDesignation{
		{'B' | isoDBCS, 0}, {'D' | isoDBCS, 0}, {'C' | isoDBCS, 0}, {'A' | isoDBCS, 0},
		{'J', 0}, {'@' | isoDBCS, 0}, {'A', 2}, {'F', 2}}},
	"iso2022_jp_2004": {isoNoShift | isoJISX0208Ext, []isoDesignation{
		{'Q' | isoDBCS, 0}, {'B' | isoDBCS, 0}, {'P' | isoDBCS, 0}}},
	"iso2022_jp_3": {isoNoShift | isoJISX0208Ext, []isoDesignation{
		{'O' | isoDBCS, 0}, {'B' | isoDBCS, 0}, {'P' | isoDBCS, 0}}},
	"iso2022_jp_ext": {isoNoShift | isoJISX0208Ext, []isoDesignation{
		{'B' | isoDBCS, 0}, {'D' | isoDBCS, 0}, {'J', 0}, {'I', 0}, {'@' | isoDBCS, 0}}},
}

// charsetTable decodes one charset: codes of width bytes to text.
type charsetTable struct {
	width int
	codes map[uint16]string
}

// isoCode is a character's code in the charset with final byte mark.
type isoCode struct {
	mark byte
	code uint16
}

func (c isoCode) width() int {
	if c.mark&isoDBCS != 0 {
		return 2
	}
	return 1
}

type statefulCodec struct {
	name     string
	hz       bool
	cfg      isoConfig
	charsets map[byte]*charsetTable // G0/G1 decoders by final byte
	g2       map[byte]*charsetTable
	encode   map[rune]isoCode
	pairs    map[[2]rune]isoCode
	pairHead map[rune]bool
}

var errInternalCodec = errors.New("internal codec error")

func charsetMap(index int) *charsetTable {
	if t, ok := charsetMaps[index]; ok {
		return t
	}
	r := &cjkReader{b: cjkCharsets[index]}
	t := &charsetTable{width: r.uint(), codes: map[uint16]string{}}
	for k := r.uint(); k > 0; k-- {
		code := uint16(r.uint())
		t.codes[code] = string(r.str())
	}
	if r.err != nil || len(r.b) != 0 {
		panic("gojinja: corrupt CJK charset table")
	}
	charsetMaps[index] = t
	return t
}

func parseStateful(name string, body []byte) *statefulCodec {
	r := &cjkReader{b: body}
	c := &statefulCodec{
		name: name, hz: name == "hz", cfg: isoConfigs[name],
		charsets: map[byte]*charsetTable{}, g2: map[byte]*charsetTable{},
		encode: map[rune]isoCode{}, pairs: map[[2]rune]isoCode{}, pairHead: map[rune]bool{},
	}
	for k := r.uint(); k > 0; k-- {
		kind, mark, index := r.uint(), byte(r.uint()), r.uint()
		if index >= len(cjkCharsets) {
			panic("gojinja: corrupt CJK codec table for " + name)
		}
		if kind == 2 {
			c.g2[mark] = charsetMap(index)
		} else {
			c.charsets[mark] = charsetMap(index)
		}
	}
	code := func() isoCode {
		mark := byte(r.uint())
		var v uint16
		for _, x := range r.str() {
			v = v<<8 | uint16(x)
		}
		return isoCode{mark, v}
	}
	for k := r.uint(); k > 0; k-- {
		cp := rune(r.uint())
		c.encode[cp] = code()
	}
	for k := r.uint(); k > 0; k-- {
		a, b := rune(r.uint()), rune(r.uint())
		c.pairs[[2]rune{a, b}] = code()
		c.pairHead[a] = true
	}
	if r.err != nil || len(r.b) != 0 {
		panic("gojinja: corrupt CJK codec table for " + name)
	}
	return c
}

// statefulLookup returns the stateful codec for a module name.
func statefulLookup(module string) (*statefulCodec, bool) {
	cjkOnce.Do(indexCJK)
	body, ok := isoBodies[module]
	if !ok {
		return nil, false
	}
	cjkMu.Lock()
	defer cjkMu.Unlock()
	c, ok := isoCodecs[module]
	if !ok {
		c = parseStateful(module, body)
		isoCodecs[module] = c
	}
	return c, true
}

// isoEncoder carries the encoder state across characters, error handler
// replacements included. size models CPython's output buffer, which hz
// output depends on (see put).
type isoEncoder struct {
	c       *statefulCodec
	out     runtime.Bytes
	g0, g1  byte
	shifted bool
	hzGB    bool
	size    int
}

func newISOEncoder(c *statefulCodec, n int) *isoEncoder {
	return &isoEncoder{c: c, out: runtime.Bytes{}, g0: isoASCII, g1: isoASCII, size: 2*n + 16}
}

// reserve grows the modelled buffer the way CPython does each time an
// encoder reports it full, until n more bytes fit.
func (e *isoEncoder) reserve(n int) {
	for e.size-len(e.out) < n {
		e.size += e.size>>1 | 1
	}
}

// put encodes rs[i] (with rs[i+1] when the two form a pair) and returns
// how many runes it consumed, or 0 when rs[i] cannot be encoded.
func (e *isoEncoder) put(rs []rune, i int) int {
	r := rs[i]
	if r < 0x80 {
		if e.c.hz {
			if e.hzGB {
				e.reserve(2)
				e.out = append(e.out, '~', '}')
				e.hzGB = false
			}
			e.reserve(1)
			e.out = append(e.out, byte(r))
			// CPython has consumed the '~' before writing its double, so
			// when the buffer is full at that point the second '~' is lost.
			if r == '~' {
				if e.size == len(e.out) {
					e.size += e.size>>1 | 1
				} else {
					e.out = append(e.out, '~')
				}
			}
			return 1
		}
		if e.g0 != isoASCII {
			e.out = append(e.out, isoESC, '(', isoASCII)
			e.g0 = isoASCII
		}
		if e.shifted {
			e.out = append(e.out, isoSI)
			e.shifted = false
		}
		e.out = append(e.out, byte(r))
		return 1
	}
	n := 1
	code, ok := isoCode{}, false
	if e.c.pairHead[r] && i+1 < len(rs) {
		// Matched on the follower's low 16 bits, as CPython does.
		if code, ok = e.c.pairs[[2]rune{r, rs[i+1] & 0xffff}]; ok {
			n = 2
		}
	}
	if !ok {
		if code, ok = e.c.encode[r]; !ok {
			return 0
		}
	}
	if e.c.hz {
		if !e.hzGB {
			e.reserve(4)
			e.out = append(e.out, '~', '{')
			e.hzGB = true
		} else {
			e.reserve(2)
		}
	} else if e.plane(code.mark) == 1 {
		if e.g1 != code.mark {
			if code.width() == 1 {
				e.out = append(e.out, isoESC, ')', code.mark)
			} else {
				e.out = append(e.out, isoESC, '$', ')', code.mark&^isoDBCS)
			}
			e.g1 = code.mark
		}
		if !e.shifted {
			e.out = append(e.out, isoSO)
			e.shifted = true
		}
	} else {
		if e.shifted {
			e.out = append(e.out, isoSI)
			e.shifted = false
		}
		if e.g0 != code.mark {
			switch {
			case code.width() == 1:
				e.out = append(e.out, isoESC, '(', code.mark)
			case code.mark == 'B'|isoDBCS:
				e.out = append(e.out, isoESC, '$', 'B')
			default:
				e.out = append(e.out, isoESC, '$', '(', code.mark&^isoDBCS)
			}
			e.g0 = code.mark
		}
	}
	if code.width() == 2 {
		e.out = append(e.out, byte(code.code>>8))
	}
	e.out = append(e.out, byte(code.code))
	return n
}

func (e *isoEncoder) plane(mark byte) int {
	for _, d := range e.c.cfg.designations {
		if d.mark == mark {
			return d.plane
		}
	}
	return 0
}

func (c *statefulCodec) encodeStateful(rs []rune, errorsMode string) (runtime.Bytes, error) {
	e := newISOEncoder(c, len(rs))
	for i := 0; i < len(rs); {
		if n := e.put(rs, i); n > 0 {
			i += n
			continue
		}
		repl, err := cjkReplacement(c.name, rs[i], i, errorsMode)
		if err != nil {
			return nil, err
		}
		i++
		if errorsMode == "replace" {
			e.put([]rune{'?'}, 0)
			continue
		}
		// Other replacements are encoded strictly into a buffer of their
		// own, sharing the encoder state, and then copied over.
		rr := []rune(repl)
		inner := newISOEncoder(c, len(rr))
		inner.g0, inner.g1, inner.shifted, inner.hzGB = e.g0, e.g1, e.shifted, e.hzGB
		for k := 0; k < len(rr); {
			n := inner.put(rr, k)
			if n == 0 {
				return nil, fmt.Errorf("'%s' codec can't encode character %s in position %d: %s", c.name, charRepr(rr[k]), k, cjkIllegal)
			}
			k += n
		}
		e.g0, e.g1, e.shifted, e.hzGB = inner.g0, inner.g1, inner.shifted, inner.hzGB
		if n := len(inner.out); n > e.size-len(e.out) {
			if n < e.size>>1 {
				n = e.size>>1 | 1
			}
			e.size += n
		}
		e.out = append(e.out, inner.out...)
	}
	if e.hzGB {
		e.reserve(2)
		e.out = append(e.out, '~', '}')
	}
	if e.shifted {
		e.out = append(e.out, isoSI)
	}
	if e.g0 != isoASCII {
		e.out = append(e.out, isoESC, '(', isoASCII)
	}
	return e.out, nil
}

// errIncomplete marks a sequence cut off by the end of the input.
const errIncomplete = -1

func (c *statefulCodec) decodeStateful(b []byte, d decodeErrors) (string, error) {
	if c.hz {
		return c.decodeHZ(b, d)
	}
	return c.decodeISO(b, d)
}

// fail applies the error handler to an n-byte error at i (errIncomplete:
// the rest of the input) and returns the position after it.
func cjkFail(b []byte, i, n int, d decodeErrors) (int, error) {
	if n == errIncomplete {
		return len(b), d.fail(i, len(b), "incomplete multibyte sequence")
	}
	return i + n, d.fail(i, i+n, cjkIllegal)
}

func (c *statefulCodec) decodeHZ(b []byte, d decodeErrors) (string, error) {
	gb := false
	t := c.charsets['A'|isoDBCS]
	for i := 0; i < len(b); {
		ch, n := b[i], 0
		switch {
		case ch == '~':
			if i+1 >= len(b) {
				n = errIncomplete
				break
			}
			switch c2 := b[i+1]; {
			case c2 == '~' && !gb:
				d.sb.WriteByte('~')
			case c2 == '{' && !gb:
				gb = true
			case c2 == '\n' && !gb:
				// line continuation
			case c2 == '}' && gb:
				gb = false
			default:
				n = 1
			}
			if n == 0 {
				i += 2
			}
		case ch&0x80 != 0:
			n = 1
		case !gb:
			d.sb.WriteByte(ch)
			i++
		case i+1 >= len(b):
			n = errIncomplete
		default:
			if text, ok := t.codes[uint16(ch)<<8|uint16(b[i+1])]; ok {
				d.sb.WriteString(text)
				i += 2
			} else {
				n = 1
			}
		}
		if n != 0 {
			var err error
			if i, err = cjkFail(b, i, n, d); err != nil {
				return "", err
			}
		}
	}
	return d.sb.String(), nil
}

func isEscEnd(c byte) bool { return c >= 'A' && c <= 'Z' || c == '@' }

func (c *statefulCodec) designates(mark byte) bool {
	for _, d := range c.cfg.designations {
		if d.mark == mark {
			return true
		}
	}
	return false
}

// processEsc handles the escape sequence at the start of b like CPython's
// iso2022processesc: it returns the sequence's length, or an error size.
func (c *statefulCodec) processEsc(b []byte, g *[3]byte) (consumed, errSize int) {
	ext := c.cfg.flags&isoJISX0208Ext != 0
	esclen := 0
	for i := 1; i < 16; i++ {
		if i >= len(b) {
			return 0, errIncomplete
		}
		if isEscEnd(b[i]) {
			esclen = i + 1
			break
		} else if ext && i+1 < len(b) && b[i] == '&' && b[i+1] == '@' {
			i += 2
		}
	}
	var charset byte
	var designation int
	switch esclen {
	case 0:
		return 0, 1
	case 3:
		if b[1] == '$' {
			charset = b[2] | isoDBCS
			break
		}
		charset = b[2]
		switch {
		case b[1] == '(':
		case b[1] == ')':
			designation = 1
		case c.cfg.flags&isoUseG2 != 0 && b[1] == '.':
			designation = 2
		default:
			return 0, 3
		}
	case 4:
		if b[1] != '$' {
			return 0, 4
		}
		charset = b[3] | isoDBCS
		switch b[2] {
		case '(':
		case ')':
			designation = 1
		default:
			return 0, 4
		}
	case 6:
		if !ext || b[3] != isoESC || b[4] != '$' || b[5] != 'B' {
			return 0, 6
		}
		charset = 'B' | isoDBCS
	default:
		return 0, esclen
	}
	if charset != isoASCII && !c.designates(charset) {
		return 0, esclen
	}
	g[designation] = charset
	return esclen, 0
}

func (c *statefulCodec) decodeISO(b []byte, d decodeErrors) (string, error) {
	g := [3]byte{isoASCII, isoASCII, isoASCII}
	shifted, escThroughout := false, false
	noShift := c.cfg.flags&isoNoShift != 0
	for i := 0; i < len(b); {
		ch, n := b[i], 0
		if escThroughout {
			// A non-ISO-2022 escape sequence passes through as Latin-1.
			d.sb.WriteRune(rune(ch))
			i++
			if isEscEnd(ch) {
				escThroughout = false
			}
			continue
		}
		switch {
		case ch == isoESC:
			switch {
			case i+1 >= len(b):
				n = errIncomplete
			case bytes.IndexByte([]byte("()$.&"), b[i+1]) >= 0:
				var used int
				if used, n = c.processEsc(b[i:], &g); n == 0 {
					i += used
				}
			case c.cfg.flags&isoUseG2 != 0 && b[i+1] == 'N':
				if i+2 >= len(b) {
					n = errIncomplete
					break
				}
				x := b[i+2]
				switch t := c.g2[g[2]]; {
				case t != nil:
					if text, ok := t.codes[uint16(x)]; ok {
						d.sb.WriteString(text)
					} else {
						n = 3
					}
				case g[2] == isoASCII:
					if x&0x80 != 0 {
						n = 3
					} else {
						d.sb.WriteByte(x)
					}
				default:
					return "", errInternalCodec
				}
				if n == 0 {
					i += 3
				}
			default:
				d.sb.WriteByte(isoESC)
				escThroughout = true
				i++
			}
		case (ch == isoSI || ch == isoSO) && !noShift:
			shifted = ch == isoSO
			i++
		case ch == isoLF:
			shifted = false
			d.sb.WriteByte(ch)
			i++
		case ch < 0x20:
			d.sb.WriteByte(ch)
			i++
		case ch >= 0x80:
			n = 1
		default:
			charset := g[0]
			if shifted {
				charset = g[1]
			}
			if charset == isoASCII {
				d.sb.WriteByte(ch)
				i++
				break
			}
			t := c.charsets[charset]
			if t == nil {
				return "", errInternalCodec
			}
			if i+t.width > len(b) {
				n = errIncomplete
				break
			}
			key := uint16(ch)
			if t.width == 2 {
				key = key<<8 | uint16(b[i+1])
			}
			if text, ok := t.codes[key]; ok {
				d.sb.WriteString(text)
				i += t.width
			} else {
				n = t.width
			}
		}
		if n != 0 {
			var err error
			if i, err = cjkFail(b, i, n, d); err != nil {
				return "", err
			}
		}
	}
	return d.sb.String(), nil
}
