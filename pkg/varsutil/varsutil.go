// Package varsutil decodes JSON template variables into the Go value
// types gojinja templates render with Python-aligned output.
//
// Go's encoding/json collapses every JSON number to float64, while
// Python's json.loads preserves the int/float distinction (`1` → int,
// `1.0` → float). Jinja2 renders those two values differently — "1" vs
// "1.0" — so feeding gojinja a map decoded by vanilla json.Unmarshal
// silently breaks template parity with Python.
//
// [JSONVars] wraps the recommended decoder + normalisation pattern.
// [NormalizeJSONNumbers] is the lower-level walker for callers that
// already drive a json.Decoder themselves.
package varsutil

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// NormalizeJSONNumbers walks v and rewrites every json.Number into an
// int64 (when the literal had no decimal point or exponent — matching
// Python's int) or a float64 (matching Python's float). Maps and slices
// are walked recursively; values of any other type pass through
// unchanged.
//
// The input is normally the result of decoding JSON with a json.Decoder
// whose UseNumber has been set:
//
//	dec := json.NewDecoder(r)
//	dec.UseNumber()
//	var v map[string]any
//	_ = dec.Decode(&v)
//	v = varsutil.NormalizeJSONNumbers(v).(map[string]any)
//
// Without UseNumber, encoding/json has already converted numeric
// literals to float64 and the int/float distinction is gone — see
// [JSONVars] for the full recommended pattern.
func NormalizeJSONNumbers(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			out[k] = NormalizeJSONNumbers(val)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = NormalizeJSONNumbers(val)
		}
		return out
	case json.Number:
		if i, err := x.Int64(); err == nil {
			return i
		}
		if f, err := x.Float64(); err == nil {
			return f
		}
		return x.String()
	}
	return v
}

// JSONVars decodes JSON bytes into a map[string]any whose numeric leaves
// are int64 for integer literals and float64 for fractional / exponent
// literals — the same shape Python's json.loads would have produced for
// the same bytes.
//
// Prefer this to encoding/json.Unmarshal when feeding variables to a
// gojinja template. Vanilla json.Unmarshal makes every JSON number
// float64, which causes `{{ x }}` of an integer literal to render as
// "130000.0" instead of "130000" — a silent parity divergence from
// Python Jinja2 for templates shared between the two engines.
//
// The JSON document must be an object. Arrays and scalars return an
// error.
func JSONVars(b []byte) (map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("varsutil: JSONVars expected JSON object, got %T", v)
	}
	return NormalizeJSONNumbers(m).(map[string]any), nil
}
