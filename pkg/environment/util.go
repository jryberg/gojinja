package environment

import (
	"bytes"
	"encoding/json"
	"math"
	"strings"

	"github.com/jryberg/gojinja/pkg/runtime"
)

// Tiny math wrappers that match Python's rounding semantics for the
// `round` filter. Kept private to the environment package.

func mathRound(f float64) float64 { return math.Round(f) }
func mathCeil(f float64) float64  { return math.Ceil(f) }
func mathFloor(f float64) float64 { return math.Floor(f) }

// jsonMarshal serializes v as JSON. indent==0 → Python's default
// json.dumps (`{"a": 1, "b": 2}`); >0 → indented form. Recursively
// unwraps *runtime.OrderedDict into a key-ordered JSON object so Python
// 3.7+ dict-order semantics survive a tojson round-trip.
func jsonMarshal(v any, indent int) ([]byte, error) {
	prepared := jsonPrep(v)
	if indent == 0 {
		raw, err := json.Marshal(prepared)
		if err != nil {
			return nil, err
		}
		// Python's default uses ", " and ": " — Go's json.Marshal
		// emits ',' / ':'. Re-space the structural separators.
		return rewriteJSONSpacing(raw), nil
	}
	return marshalIndented(prepared, strings.Repeat(" ", indent))
}

// rewriteJSONSpacing rewrites raw compact JSON to Python's default
// spacing: `, ` after commas and `: ` after colons that aren't inside
// string literals.
func rewriteJSONSpacing(raw []byte) []byte {
	var b strings.Builder
	b.Grow(len(raw) + len(raw)/8)
	inString := false
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if inString {
			b.WriteByte(c)
			if c == '\\' && i+1 < len(raw) {
				b.WriteByte(raw[i+1])
				i++
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
			b.WriteByte(c)
		case ',':
			b.WriteByte(',')
			b.WriteByte(' ')
		case ':':
			b.WriteByte(':')
			b.WriteByte(' ')
		default:
			b.WriteByte(c)
		}
	}
	return []byte(b.String())
}

// jsonPrep walks v converting OrderedDict / map[any]any into
// map[string]any. Jinja2's default tojson policy passes
// `sort_keys=True` to json.dumps, so we don't preserve insertion
// order here — encoding/json sorts map keys alphabetically, matching
// Python's sorted output.
func jsonPrep(v any) any {
	switch x := v.(type) {
	case *runtime.OrderedDict:
		out := make(map[string]any, x.Len())
		for _, k := range x.Keys() {
			val, _ := x.Get(k)
			out[stringifyJSONKey(k)] = jsonPrep(val)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, it := range x {
			out[i] = jsonPrep(it)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			out[k] = jsonPrep(val)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			out[stringifyJSONKey(k)] = jsonPrep(val)
		}
		return out
	}
	return v
}

func stringifyJSONKey(k any) string {
	if s, ok := k.(string); ok {
		return s
	}
	b, _ := json.Marshal(k)
	return string(b)
}

func marshalIndented(v any, indent string) ([]byte, error) {
	// First marshal compact (preserving OrderedDict key order via
	// MarshalJSON), then re-indent.
	compact, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	if err := json.Indent(&b, compact, "", indent); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// urlEncodePath percent-encodes for the path portion of a URL — keeps
// `/`, encodes everything else outside the unreserved set.
func urlEncodePath(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case (c >= 'a' && c <= 'z') ||
			(c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') ||
			c == '-' || c == '_' || c == '.' || c == '~' || c == '/':
			b.WriteByte(c)
		default:
			b.WriteString(percent(c))
		}
	}
	return b.String()
}

// urlEncodeQS percent-encodes for query strings — replaces space with
// `+` and encodes `/` as well.
func urlEncodeQS(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == ' ':
			b.WriteByte('+')
		case (c >= 'a' && c <= 'z') ||
			(c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') ||
			c == '-' || c == '_' || c == '.' || c == '~':
			b.WriteByte(c)
		default:
			b.WriteString(percent(c))
		}
	}
	return b.String()
}

func percent(c byte) string {
	hex := "0123456789ABCDEF"
	return "%" + string(hex[c>>4]) + string(hex[c&0xF])
}
