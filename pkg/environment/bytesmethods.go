package environment

import (
	"encoding/hex"
	"fmt"
	"strings"

	gjerrors "github.com/jryberg/gojinja/pkg/errors"
	"github.com/jryberg/gojinja/pkg/eval"
	"github.com/jryberg/gojinja/pkg/runtime"
)

// Python's bytes methods. They work byte by byte and their case, digit
// and whitespace rules are ASCII-only, unlike str's. Implementations
// receive the bytes as a Go string and return runtime.Bytes for byte
// results.

// bytesMethod returns Python's bytes method attr bound to b.
func bytesMethod(b runtime.Bytes, attr string) any {
	spec, ok := bytesMethodSpecs[attr]
	if !ok {
		return nil
	}
	return KwargsCallable(func(args []any, kwargs map[string]any) (any, error) {
		args, err := spec.bind("bytes", attr, args, kwargs)
		if err != nil {
			return nil, gjerrors.NewTemplateRuntimeError(err.Error())
		}
		v, err := spec.call(string(b), args)
		if err != nil {
			return nil, gjerrors.NewTemplateRuntimeError(err.Error())
		}
		return v, nil
	})
}

func isASCIISpace(c byte) bool {
	switch c {
	case ' ', '\t', '\n', '\r', '\v', '\f':
		return true
	}
	return false
}
func isASCIILower(c byte) bool { return c >= 'a' && c <= 'z' }
func isASCIIUpper(c byte) bool { return c >= 'A' && c <= 'Z' }
func isASCIIDigit(c byte) bool { return c >= '0' && c <= '9' }
func isASCIIAlpha(c byte) bool { return isASCIILower(c) || isASCIIUpper(c) }

func asciiUpper(c byte) byte {
	if isASCIILower(c) {
		return c - 32
	}
	return c
}

func asciiLower(c byte) byte {
	if isASCIIUpper(c) {
		return c + 32
	}
	return c
}

func mapBytes(s string, f func(int, byte) byte) runtime.Bytes {
	out := make(runtime.Bytes, len(s))
	for i := 0; i < len(s); i++ {
		out[i] = f(i, s[i])
	}
	return out
}

func allBytes(s string, pred func(byte) bool) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !pred(s[i]) {
			return false
		}
	}
	return true
}

// bytesArg requires a bytes argument.
func bytesArg(v any) (string, error) {
	if b, ok := v.(runtime.Bytes); ok {
		return string(b), nil
	}
	return "", fmt.Errorf("a bytes-like object is required, not '%s'", runtime.PyTypeName(v))
}

// bytesSubArg is the needle of find/count/index: bytes or a byte value.
func bytesSubArg(v any) (string, error) {
	if b, ok := v.(runtime.Bytes); ok {
		return string(b), nil
	}
	if _, isStr := eqString(v); !isStr {
		if n, err := indexArg(v); err == nil {
			if n < 0 || n > 255 {
				return "", fmt.Errorf("byte must be in range(0, 256)")
			}
			return string([]byte{byte(n)}), nil
		}
	}
	return "", fmt.Errorf("argument should be integer or bytes-like object, not '%s'", runtime.PyTypeName(v))
}

func bytesList(parts []string) []any {
	out := make([]any, len(parts))
	for i, p := range parts {
		out[i] = runtime.Bytes(p)
	}
	return out
}

func bytesNoArgs(f func(string) any) strMethodSpec {
	return strMethodSpec{style: styleNoArgs, call: func(s string, _ []any) (any, error) { return f(s), nil }}
}

var bytesMethodSpecs map[string]strMethodSpec

func init() {
	bytesMethodSpecs = map[string]strMethodSpec{
		"lower": bytesNoArgs(func(s string) any { return mapBytes(s, func(_ int, c byte) byte { return asciiLower(c) }) }),
		"upper": bytesNoArgs(func(s string) any { return mapBytes(s, func(_ int, c byte) byte { return asciiUpper(c) }) }),
		"swapcase": bytesNoArgs(func(s string) any {
			return mapBytes(s, func(_ int, c byte) byte {
				if isASCIILower(c) {
					return asciiUpper(c)
				}
				return asciiLower(c)
			})
		}),
		"capitalize": bytesNoArgs(func(s string) any {
			return mapBytes(s, func(i int, c byte) byte {
				if i == 0 {
					return asciiUpper(c)
				}
				return asciiLower(c)
			})
		}),
		"title": bytesNoArgs(func(s string) any {
			prevCased := false
			return mapBytes(s, func(_ int, c byte) byte {
				out := asciiUpper(c)
				if prevCased {
					out = asciiLower(c)
				}
				prevCased = isASCIIAlpha(c)
				return out
			})
		}),
		"isalnum": bytesNoArgs(func(s string) any {
			return allBytes(s, func(c byte) bool { return isASCIIAlpha(c) || isASCIIDigit(c) })
		}),
		"isalpha": bytesNoArgs(func(s string) any { return allBytes(s, isASCIIAlpha) }),
		"isascii": bytesNoArgs(func(s string) any { return s == "" || allBytes(s, func(c byte) bool { return c < 0x80 }) }),
		"isdigit": bytesNoArgs(func(s string) any { return allBytes(s, isASCIIDigit) }),
		"isspace": bytesNoArgs(func(s string) any { return allBytes(s, isASCIISpace) }),
		"islower": bytesNoArgs(func(s string) any {
			return strings.IndexFunc(s, func(r rune) bool { return r < 0x80 && isASCIIUpper(byte(r)) }) < 0 &&
				strings.IndexFunc(s, func(r rune) bool { return r < 0x80 && isASCIILower(byte(r)) }) >= 0
		}),
		"isupper": bytesNoArgs(func(s string) any {
			return strings.IndexFunc(s, func(r rune) bool { return r < 0x80 && isASCIILower(byte(r)) }) < 0 &&
				strings.IndexFunc(s, func(r rune) bool { return r < 0x80 && isASCIIUpper(byte(r)) }) >= 0
		}),
		"istitle": bytesNoArgs(func(s string) any {
			cased, prevCased := false, false
			for i := 0; i < len(s); i++ {
				switch c := s[i]; {
				case isASCIIUpper(c):
					if prevCased {
						return false
					}
					prevCased, cased = true, true
				case isASCIILower(c):
					if !prevCased {
						return false
					}
					prevCased, cased = true, true
				default:
					prevCased = false
				}
			}
			return cased
		}),

		"center": {style: styleExpected, min: 1, max: 2, call: bytesPad("center")},
		"ljust":  {style: styleExpected, min: 1, max: 2, call: bytesPad("ljust")},
		"rjust":  {style: styleExpected, min: 1, max: 2, call: bytesPad("rjust")},
		"strip":  {style: styleExpected, max: 1, call: bytesStrip(true, true)},
		"lstrip": {style: styleExpected, max: 1, call: bytesStrip(true, false)},
		"rstrip": {style: styleExpected, max: 1, call: bytesStrip(false, true)},
		"replace": {style: styleExpected, min: 2, max: 3, call: func(s string, a []any) (any, error) {
			oldB, err := bytesArg(a[0])
			if err != nil {
				return nil, err
			}
			newB, err := bytesArg(a[1])
			if err != nil {
				return nil, err
			}
			count := -1
			if a[2] != nil {
				if count, err = strIndexArg(a[2]); err != nil {
					return nil, err
				}
			}
			if oldB != "" || count == 0 {
				return runtime.Bytes(strings.Replace(s, oldB, newB, count)), nil
			}
			var b strings.Builder
			for i := 0; i <= len(s); i++ {
				if count < 0 || i < count {
					b.WriteString(newB)
				}
				if i < len(s) {
					b.WriteByte(s[i])
				}
			}
			return runtime.Bytes(b.String()), nil
		}},
		"maketrans": {style: styleExpected, min: 2, max: 2, call: func(_ string, a []any) (any, error) {
			frm, err := bytesArg(a[0])
			if err != nil {
				return nil, err
			}
			to, err := bytesArg(a[1])
			if err != nil {
				return nil, err
			}
			if len(frm) != len(to) {
				return nil, fmt.Errorf("maketrans arguments must have same length")
			}
			table := make(runtime.Bytes, 256)
			for i := range table {
				table[i] = byte(i)
			}
			for i := 0; i < len(frm); i++ {
				table[frm[i]] = to[i]
			}
			return table, nil
		}},

		"count":      {style: styleTakes, min: 1, max: 3, call: bytesSearch("count")},
		"find":       {style: styleTakes, min: 1, max: 3, call: bytesSearch("find")},
		"rfind":      {style: styleTakes, min: 1, max: 3, call: bytesSearch("rfind")},
		"index":      {style: styleTakes, min: 1, max: 3, call: bytesSearch("index")},
		"rindex":     {style: styleTakes, min: 1, max: 3, call: bytesSearch("rindex")},
		"startswith": {style: styleTakes, min: 1, max: 3, call: bytesAffix("startswith")},
		"endswith":   {style: styleTakes, min: 1, max: 3, call: bytesAffix("endswith")},

		"join": {style: styleExactlyOne, call: func(s string, a []any) (any, error) {
			items, err := eval.ToIterable(a[0])
			if err != nil || a[0] == nil {
				return nil, fmt.Errorf("can only join an iterable")
			}
			parts := make([]string, len(items))
			for i, it := range items {
				b, ok := it.(runtime.Bytes)
				if !ok {
					return nil, fmt.Errorf("sequence item %d: expected a bytes-like object, %s found", i, runtime.PyTypeName(it))
				}
				parts[i] = string(b)
			}
			return runtime.Bytes(strings.Join(parts, s)), nil
		}},
		"partition":  {style: styleExactlyOne, call: bytesPartition(false)},
		"rpartition": {style: styleExactlyOne, call: bytesPartition(true)},
		"removeprefix": {style: styleExactlyOne, call: func(s string, a []any) (any, error) {
			p, err := bytesArg(a[0])
			if err != nil {
				return nil, err
			}
			return runtime.Bytes(strings.TrimPrefix(s, p)), nil
		}},
		"removesuffix": {style: styleExactlyOne, call: func(s string, a []any) (any, error) {
			p, err := bytesArg(a[0])
			if err != nil {
				return nil, err
			}
			return runtime.Bytes(strings.TrimSuffix(s, p)), nil
		}},
		"zfill": {style: styleExactlyOne, call: func(s string, a []any) (any, error) {
			width, err := strIndexArg(a[0])
			if err != nil {
				return nil, err
			}
			if len(s) >= width {
				return runtime.Bytes(s), nil
			}
			fill := width - len(s)
			out := []byte(strings.Repeat("0", fill) + s)
			if len(s) > 0 && (s[0] == '+' || s[0] == '-') {
				out[0], out[fill] = s[0], '0'
			}
			return runtime.Bytes(out), nil
		}},
		"fromhex": {style: styleExactlyOne, call: func(_ string, a []any) (any, error) {
			str, ok := a[0].(string)
			if !ok {
				return nil, fmt.Errorf("fromhex() argument must be str, not %s", runtime.PyTypeName(a[0]))
			}
			return bytesFromHex(str)
		}},

		"split":  {style: styleKeywords, max: 2, keywords: []string{"sep", "maxsplit"}, call: bytesSplit(false)},
		"rsplit": {style: styleKeywords, max: 2, keywords: []string{"sep", "maxsplit"}, call: bytesSplit(true)},
		"splitlines": {style: styleKeywords, max: 1, keywords: []string{"keepends"}, call: func(s string, a []any) (any, error) {
			keep := a[0] != nil && eval.Truthy(a[0])
			var parts []string
			start := 0
			for i := 0; i < len(s); i++ {
				if s[i] != '\n' && s[i] != '\r' {
					continue
				}
				end := i + 1
				if s[i] == '\r' && end < len(s) && s[end] == '\n' {
					end++
				}
				if keep {
					parts = append(parts, s[start:end])
				} else {
					parts = append(parts, s[start:i])
				}
				start = end
				i = end - 1
			}
			if start < len(s) {
				parts = append(parts, s[start:])
			}
			return bytesList(parts), nil
		}},
		"expandtabs": {style: styleKeywords, max: 1, keywords: []string{"tabsize"}, call: func(s string, a []any) (any, error) {
			tab := 8
			if a[0] != nil {
				var err error
				if tab, err = strIndexArg(a[0]); err != nil {
					return nil, err
				}
			}
			var b strings.Builder
			col := 0
			for i := 0; i < len(s); i++ {
				switch c := s[i]; c {
				case '\t':
					if tab > 0 {
						n := tab - col%tab
						b.WriteString(strings.Repeat(" ", n))
						col += n
					}
				case '\n', '\r':
					b.WriteByte(c)
					col = 0
				default:
					b.WriteByte(c)
					col++
				}
			}
			return runtime.Bytes(b.String()), nil
		}},
		"hex": {style: styleKeywords, max: 2, keywords: []string{"sep", "bytes_per_sep"}, call: bytesHex},
		"translate": {style: styleKeywords, max: 2, keywords: []string{"table", "delete"}, posOnly: 1, call: func(s string, a []any) (any, error) {
			var table string
			if a[0] != nil {
				var err error
				if table, err = bytesArg(a[0]); err != nil {
					return nil, err
				}
				if len(table) != 256 {
					return nil, fmt.Errorf("translation table must be 256 characters long")
				}
			}
			var del string
			if a[1] != nil {
				var err error
				if del, err = bytesArg(a[1]); err != nil {
					return nil, err
				}
			}
			var b strings.Builder
			for i := 0; i < len(s); i++ {
				if strings.IndexByte(del, s[i]) >= 0 {
					continue
				}
				if table != "" {
					b.WriteByte(table[s[i]])
				} else {
					b.WriteByte(s[i])
				}
			}
			return runtime.Bytes(b.String()), nil
		}},
		"decode": {style: styleKeywords, max: 2, keywords: []string{"encoding", "errors"}, call: func(s string, a []any) (any, error) {
			encoding, errorsMode := "utf-8", "strict"
			var err error
			if a[0] != nil {
				if encoding, err = strArg("decode() argument 'encoding'", a[0]); err != nil {
					return nil, err
				}
			}
			if a[1] != nil {
				if errorsMode, err = strArg("decode() argument 'errors'", a[1]); err != nil {
					return nil, err
				}
			}
			return pyDecode([]byte(s), encoding, errorsMode)
		}},
	}
}

func bytesPad(name string) func(string, []any) (any, error) {
	return func(s string, a []any) (any, error) {
		width, err := strIndexArg(a[0])
		if err != nil {
			return nil, err
		}
		fill := byte(' ')
		if a[1] != nil {
			f, ok := a[1].(runtime.Bytes)
			if !ok || len(f) != 1 {
				return nil, fmt.Errorf("%s() argument 2 must be a byte string of length 1, not %s", name, runtime.PyTypeName(a[1]))
			}
			fill = f[0]
		}
		if len(s) >= width {
			return runtime.Bytes(s), nil
		}
		marg := width - len(s)
		left := 0
		switch name {
		case "rjust":
			left = marg
		case "center":
			left = marg/2 + (marg & width & 1)
		}
		pad := func(n int) string { return strings.Repeat(string([]byte{fill}), n) }
		return runtime.Bytes(pad(left) + s + pad(marg-left)), nil
	}
}

func bytesStrip(left, right bool) func(string, []any) (any, error) {
	return func(s string, a []any) (any, error) {
		cut := func(c byte) bool { return isASCIISpace(c) }
		if a[0] != nil {
			chars, err := bytesArg(a[0])
			if err != nil {
				return nil, err
			}
			cut = func(c byte) bool { return strings.IndexByte(chars, c) >= 0 }
		}
		i, j := 0, len(s)
		for left && i < j && cut(s[i]) {
			i++
		}
		for right && j > i && cut(s[j-1]) {
			j--
		}
		return runtime.Bytes(s[i:j]), nil
	}
}

func bytesSearch(name string) func(string, []any) (any, error) {
	return func(s string, a []any) (any, error) {
		sub, err := bytesSubArg(a[0])
		if err != nil {
			return nil, err
		}
		start, end, err := sliceBounds(len(s), a[1], a[2])
		if err != nil {
			return nil, err
		}
		if name == "count" {
			switch {
			case end-start < len(sub):
				return int64(0), nil
			case sub == "":
				return int64(end - start + 1), nil
			}
			return int64(strings.Count(s[start:end], sub)), nil
		}
		idx := -1
		if end-start >= len(sub) {
			if name == "find" || name == "index" {
				idx = strings.Index(s[start:end], sub)
			} else {
				idx = strings.LastIndex(s[start:end], sub)
			}
			if idx >= 0 {
				idx += start
			}
		}
		if idx < 0 && (name == "index" || name == "rindex") {
			return nil, fmt.Errorf("subsection not found")
		}
		return int64(idx), nil
	}
}

func bytesAffix(name string) func(string, []any) (any, error) {
	return func(s string, a []any) (any, error) {
		affixes := []any{a[0]}
		if t, ok := a[0].(runtime.Tuple); ok {
			affixes = t
		} else if _, ok := a[0].(runtime.Bytes); !ok {
			return nil, fmt.Errorf("%s first arg must be bytes or a tuple of bytes, not %s", name, runtime.PyTypeName(a[0]))
		}
		start, end, err := sliceBounds(len(s), a[1], a[2])
		if err != nil {
			return nil, err
		}
		for _, it := range affixes {
			affix, err := bytesArg(it)
			if err != nil {
				return nil, err
			}
			n := len(affix)
			if end-n < start {
				continue
			}
			if name == "startswith" && s[start:start+n] == affix || name == "endswith" && s[end-n:end] == affix {
				return true, nil
			}
		}
		return false, nil
	}
}

func bytesPartition(fromRight bool) func(string, []any) (any, error) {
	return func(s string, a []any) (any, error) {
		sep, err := bytesArg(a[0])
		if err != nil {
			return nil, err
		}
		if sep == "" {
			return nil, fmt.Errorf("empty separator")
		}
		i := strings.Index(s, sep)
		if fromRight {
			i = strings.LastIndex(s, sep)
		}
		switch {
		case i >= 0:
			return runtime.Tuple{runtime.Bytes(s[:i]), runtime.Bytes(sep), runtime.Bytes(s[i+len(sep):])}, nil
		case fromRight:
			return runtime.Tuple{runtime.Bytes{}, runtime.Bytes{}, runtime.Bytes(s)}, nil
		}
		return runtime.Tuple{runtime.Bytes(s), runtime.Bytes{}, runtime.Bytes{}}, nil
	}
}

func bytesSplit(fromRight bool) func(string, []any) (any, error) {
	return func(s string, a []any) (any, error) {
		limit := -1
		if a[1] != nil {
			var err error
			if limit, err = strIndexArg(a[1]); err != nil {
				return nil, err
			}
		}
		if a[0] != nil {
			sep, err := bytesArg(a[0])
			if err != nil {
				return nil, err
			}
			if sep == "" {
				return nil, fmt.Errorf("empty separator")
			}
			return bytesList(splitSep(s, sep, limit, fromRight)), nil
		}
		// Whitespace splitting over ASCII whitespace, as split_whitespace.
		var parts []string
		if !fromRight {
			i := 0
			for limit != 0 {
				for i < len(s) && isASCIISpace(s[i]) {
					i++
				}
				if i == len(s) {
					break
				}
				j := i
				for i < len(s) && !isASCIISpace(s[i]) {
					i++
				}
				parts = append(parts, s[j:i])
				limit--
			}
			for i < len(s) && isASCIISpace(s[i]) {
				i++
			}
			if i < len(s) {
				parts = append(parts, s[i:])
			}
			return bytesList(parts), nil
		}
		i := len(s) - 1
		for limit != 0 {
			for i >= 0 && isASCIISpace(s[i]) {
				i--
			}
			if i < 0 {
				break
			}
			j := i
			for i >= 0 && !isASCIISpace(s[i]) {
				i--
			}
			parts = append([]string{s[i+1 : j+1]}, parts...)
			limit--
		}
		for i >= 0 && isASCIISpace(s[i]) {
			i--
		}
		if i >= 0 {
			parts = append([]string{s[:i+1]}, parts...)
		}
		return bytesList(parts), nil
	}
}

// bytesHex is bytes.hex(sep=None, bytes_per_sep=1): sep goes between
// groups of bytes_per_sep bytes, counted from the right when positive
// and from the left when negative.
func bytesHex(s string, a []any) (any, error) {
	digits := hex.EncodeToString([]byte(s))
	if a[0] == nil {
		return digits, nil
	}
	var sep string
	switch x := a[0].(type) {
	case string:
		sep = x
	case runtime.Bytes:
		sep = string(x)
	default:
		//lint:ignore ST1005 CPython's exact message
		return nil, fmt.Errorf("sep must be str or bytes.")
	}
	if len([]rune(sep)) != 1 {
		//lint:ignore ST1005 CPython's exact message
		return nil, fmt.Errorf("sep must be length 1.")
	}
	group := 1
	if a[1] != nil {
		var err error
		if group, err = strIndexArg(a[1]); err != nil {
			return nil, err
		}
	}
	if group == 0 || len(s) == 0 {
		return digits, nil
	}
	fromLeft := group < 0
	if fromLeft {
		group = -group
	}
	var chunks []string
	n := len(s)
	if fromLeft {
		for i := 0; i < n; i += group {
			chunks = append(chunks, digits[2*i:2*min(i+group, n)])
		}
	} else {
		for i := n; i > 0; i -= group {
			chunks = append([]string{digits[2*max(i-group, 0) : 2*i]}, chunks...)
		}
	}
	return strings.Join(chunks, sep), nil
}

// bytesFromHex is bytes.fromhex: pairs of hex digits, with ASCII
// whitespace allowed between pairs.
func bytesFromHex(s string) (runtime.Bytes, error) {
	var out runtime.Bytes
	for i := 0; i < len(s); {
		if isASCIISpace(s[i]) {
			i++
			continue
		}
		if i+1 >= len(s) || !isHexDigit(s[i]) || !isHexDigit(s[i+1]) {
			// CPython reports the first bad digit, which for a valid
			// first digit is the (possibly missing) second one.
			pos := i
			if isHexDigit(s[i]) {
				pos = i + 1
			}
			return nil, fmt.Errorf("non-hexadecimal number found in fromhex() arg at position %d", pos)
		}
		b, _ := hex.DecodeString(s[i : i+2])
		out = append(out, b[0])
		i += 2
	}
	if out == nil {
		out = runtime.Bytes{}
	}
	return out, nil
}

func isHexDigit(c byte) bool {
	return isASCIIDigit(c) || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}
