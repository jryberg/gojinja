package filters

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/jryberg/gojinja/pkg/environment"
	gjerrors "github.com/jryberg/gojinja/pkg/errors"
	"github.com/jryberg/gojinja/pkg/escape"
	"github.com/jryberg/gojinja/pkg/runtime"
)

// Base64Decode is the `base64decode` filter: decode a base64 string and
// return the text it encodes.
//
// Signature: base64decode(value, encoding="utf-8")
//
// Decoding follows Python's `base64.b64decode(value)` (non-strict): bytes
// outside the base64 alphabet are skipped, a complete padding sequence
// ends the input, and a truncated final quad is an error. The decoded
// bytes are then decoded strictly with `encoding` (utf-8, ascii or
// latin-1).
//
// Example:
//
//	{{ "aGVsbG8=" | base64decode }}  →  hello
var Base64Decode = environment.Filter{Func: filterBase64Decode}

func filterBase64Decode(_ *environment.Environment, _ *runtime.Context, value any, args []any, kwargs map[string]any) (any, error) {
	if u, ok := value.(runtime.Undefined); ok {
		return nil, u.Fail()
	}
	var src string
	switch v := value.(type) {
	case string:
		src = v
	case escape.Markup:
		src = string(v)
	case []byte:
		src = string(v)
	default:
		return nil, gjerrors.NewFilterArgumentError(fmt.Sprintf("base64decode: argument should be a string, not %T", value))
	}
	encoding := "utf-8"
	if len(args) > 0 {
		s, ok := args[0].(string)
		if !ok {
			return nil, gjerrors.NewFilterArgumentError("base64decode: encoding must be a string")
		}
		encoding = s
	}
	if v, ok := kwargs["encoding"]; ok {
		if len(args) > 0 {
			return nil, gjerrors.NewFilterArgumentError("base64decode: got multiple values for argument 'encoding'")
		}
		s, ok := v.(string)
		if !ok {
			return nil, gjerrors.NewFilterArgumentError("base64decode: encoding must be a string")
		}
		encoding = s
	}
	for i := 0; i < len(src); i++ {
		if src[i] >= utf8.RuneSelf {
			return nil, gjerrors.NewFilterArgumentError("base64decode: string argument should contain only ASCII characters")
		}
	}
	raw, err := decodeBase64Lenient(src)
	if err != nil {
		return nil, err
	}
	return decodeText(raw, encoding)
}

// decodeBase64Lenient mirrors CPython's binascii.a2b_base64 in non-strict
// mode.
func decodeBase64Lenient(s string) ([]byte, error) {
	out := make([]byte, 0, len(s)*3/4)
	var leftchar byte
	quadPos, pads, dataChars := 0, 0, 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '=' {
			if quadPos >= 2 {
				pads++
				if quadPos+pads >= 4 {
					return out, nil
				}
			}
			continue
		}
		v, ok := base64Value(c)
		if !ok {
			continue
		}
		pads = 0
		dataChars++
		switch quadPos {
		case 0:
			quadPos = 1
			leftchar = v
		case 1:
			quadPos = 2
			out = append(out, leftchar<<2|v>>4)
			leftchar = v & 0x0f
		case 2:
			quadPos = 3
			out = append(out, leftchar<<4|v>>2)
			leftchar = v & 0x03
		case 3:
			quadPos = 0
			out = append(out, leftchar<<6|v)
			leftchar = 0
		}
	}
	switch quadPos {
	case 0:
		return out, nil
	case 1:
		return nil, gjerrors.NewFilterArgumentError(fmt.Sprintf("base64decode: invalid base64-encoded string: number of data characters (%d) cannot be 1 more than a multiple of 4", dataChars))
	default:
		return nil, gjerrors.NewFilterArgumentError("base64decode: incorrect padding")
	}
}

func base64Value(c byte) (byte, bool) {
	switch {
	case c >= 'A' && c <= 'Z':
		return c - 'A', true
	case c >= 'a' && c <= 'z':
		return c - 'a' + 26, true
	case c >= '0' && c <= '9':
		return c - '0' + 52, true
	case c == '+':
		return 62, true
	case c == '/':
		return 63, true
	}
	return 0, false
}

// decodeText decodes b strictly, like Python's bytes.decode(encoding).
func decodeText(b []byte, encoding string) (string, error) {
	switch normalizeEncoding(encoding) {
	case "utf_8", "utf8", "u8":
		if !utf8.Valid(b) {
			return "", gjerrors.NewFilterArgumentError("base64decode: decoded bytes are not valid utf-8")
		}
		return string(b), nil
	case "ascii", "us_ascii":
		for _, c := range b {
			if c >= utf8.RuneSelf {
				return "", gjerrors.NewFilterArgumentError("base64decode: decoded bytes are not valid ascii")
			}
		}
		return string(b), nil
	case "latin_1", "latin1", "iso_8859_1", "iso8859_1", "l1":
		var sb strings.Builder
		sb.Grow(len(b))
		for _, c := range b {
			sb.WriteRune(rune(c))
		}
		return sb.String(), nil
	}
	return "", gjerrors.NewFilterArgumentError(fmt.Sprintf("base64decode: unknown encoding: %s", encoding))
}

// normalizeEncoding applies Python's codec-name normalisation (lowercase,
// hyphens and spaces become underscores).
func normalizeEncoding(name string) string {
	return strings.NewReplacer("-", "_", " ", "_").Replace(strings.ToLower(strings.TrimSpace(name)))
}
