package environment

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"strings"

	"github.com/jryberg/gojinja/pkg/escape"
	"github.com/jryberg/gojinja/pkg/runtime"
)

// pyRoundFloat is CPython's round(f, ndigits) for a float: the exact
// binary value is rounded half-to-even at the decimal position, then
// converted back to the nearest float. Non-finite input is returned
// unchanged and a zero result keeps the sign of f.
func pyRoundFloat(f float64, ndigits int) float64 {
	if math.IsNaN(f) || math.IsInf(f, 0) || ndigits > 323 {
		return f
	}
	if ndigits < -308 {
		return math.Copysign(0, f)
	}
	r := new(big.Rat).SetFloat64(f)
	out, _ := roundRatHalfEven(r, ndigits).Float64()
	if out == 0 {
		return math.Copysign(0, f)
	}
	return out
}

// roundRatHalfEven rounds r to ndigits decimal places (negative ndigits
// round to tens, hundreds, ...) with ties going to the even neighbour.
func roundRatHalfEven(r *big.Rat, ndigits int) *big.Rat {
	exp := ndigits
	if exp < 0 {
		exp = -exp
	}
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(exp)), nil)
	scaled := new(big.Rat).Set(r)
	if ndigits >= 0 {
		scaled.Mul(scaled, new(big.Rat).SetInt(scale))
	} else {
		scaled.Quo(scaled, new(big.Rat).SetInt(scale))
	}
	q, m := new(big.Int).QuoRem(scaled.Num(), scaled.Denom(), new(big.Int))
	// Compare 2*|remainder| with the denominator to classify the tie.
	cmp := new(big.Int).Abs(m)
	cmp.Lsh(cmp, 1)
	switch c := cmp.Cmp(scaled.Denom()); {
	case c > 0 || (c == 0 && q.Bit(0) == 1):
		if m.Sign() < 0 {
			q.Sub(q, big.NewInt(1))
		} else {
			q.Add(q, big.NewInt(1))
		}
	}
	out := new(big.Rat).SetInt(q)
	if ndigits >= 0 {
		return out.Quo(out, new(big.Rat).SetInt(scale))
	}
	return out.Mul(out, new(big.Rat).SetInt(scale))
}

// jsonMarshal serializes v as JSON. Without pretty it matches Python's
// default json.dumps (`{"a": 1, "b": 2}`); with pretty each level is
// indented by indent, as json.dumps(indent=...) does. Recursively
// unwraps *runtime.OrderedDict into a key-ordered JSON object so Python
// 3.7+ dict-order semantics survive a tojson round-trip.
func jsonMarshal(v any, pretty bool, indent string) ([]byte, error) {
	prepared, err := jsonPrep(v)
	if err != nil {
		return nil, err
	}
	if !pretty {
		raw, err := json.Marshal(prepared)
		if err != nil {
			return nil, err
		}
		// Python's default uses ", " and ": " — Go's json.Marshal
		// emits ',' / ':'. Re-space the structural separators.
		return rewriteJSONSpacing(raw), nil
	}
	return marshalIndented(prepared, indent)
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
// Python's sorted output. Like json.dumps it rejects a container that
// contains itself, and data nested past escape.MaxDepth.
func jsonPrep(v any) (any, error) {
	return (&jsonWalker{active: map[uintptr]bool{}}).prep(v)
}

type jsonWalker struct {
	depth  int
	active map[uintptr]bool
}

func (w *jsonWalker) prep(v any) (any, error) {
	var items []any
	var keys []any
	var get func(k any) any
	switch x := v.(type) {
	case *runtime.OrderedDict:
		if x == nil {
			return v, nil
		}
		keys = x.Keys()
		get = func(k any) any { val, _ := x.Get(k); return val }
	case map[string]any:
		for k := range x {
			keys = append(keys, k)
		}
		get = func(k any) any { return x[k.(string)] }
	case map[any]any:
		for k := range x {
			keys = append(keys, k)
		}
		get = func(k any) any { return x[k] }
	case []any:
		items = x
	case runtime.Tuple:
		items = x
	case *runtime.PyList:
		if x == nil {
			return []any{}, nil
		}
		items = x.Items()
	case *runtime.Range, *runtime.DictView, complex128:
		//lint:ignore ST1005 json.dumps's exact message
		return nil, fmt.Errorf("Object of type %s is not JSON serializable", runtime.PyTypeName(v))
	default:
		return v, nil
	}
	w.depth++
	defer func() { w.depth-- }()
	if w.depth > escape.MaxDepth {
		return nil, fmt.Errorf("maximum recursion depth exceeded while encoding a JSON object")
	}
	if id, ok := ppID(v); ok {
		if w.active[id] {
			//lint:ignore ST1005 json.dumps's exact message
			return nil, fmt.Errorf("Circular reference detected")
		}
		w.active[id] = true
		defer delete(w.active, id)
	}
	if get != nil {
		out := make(map[string]any, len(keys))
		for _, k := range keys {
			val, err := w.prep(get(k))
			if err != nil {
				return nil, err
			}
			out[stringifyJSONKey(k)] = val
		}
		return out, nil
	}
	out := make([]any, len(items))
	for i, it := range items {
		val, err := w.prep(it)
		if err != nil {
			return nil, err
		}
		out[i] = val
	}
	return out, nil
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
