// Package varsutil decodes JSON template variables into the Go value
// types gojinja templates render with Python-aligned output.
//
// Two pieces of Python parity drive this package:
//
//   - Go's encoding/json collapses every JSON number to float64, while
//     Python's json.loads preserves the int/float distinction (`1` → int,
//     `1.0` → float). Jinja2 renders those two values differently — "1"
//     vs "1.0" — so feeding gojinja a map decoded by vanilla
//     json.Unmarshal silently breaks parity.
//   - Python dicts (and json.loads' output) preserve insertion order
//     since CPython 3.7. Go maps don't. A template iterating a dict
//     loaded by vanilla json.Unmarshal emits keys in random order per
//     process run — a visible divergence and a non-determinism bug.
//
// [JSONVars] handles both: it returns an [runtime.OrderedDict] whose
// keys appear in source order, with int64 / float64 leaves matching
// Python. [NormalizeJSONNumbers] remains exported for callers driving
// their own json.Decoder who only need the numeric fix.
package varsutil

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/jryberg/gojinja/pkg/runtime"
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
// literals to float64 and the int/float distinction is gone. This
// helper does not preserve dict insertion order — reach for [JSONVars]
// when feeding variables to a template.
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

// JSONVars decodes JSON bytes into a [runtime.OrderedDict] whose keys
// appear in source order at every nesting level, with Python-aligned
// numeric types — integer literals become int64, fractional / exponent
// literals become float64. Pass the result directly to
// [github.com/jryberg/gojinja.Template.Render] or
// [github.com/jryberg/gojinja.Template.RenderContext] to make
// `{% for k in d %}` iterate keys in the JSON source order, matching
// Python Jinja2.
//
// The JSON document must be an object. Arrays and scalars return an
// error.
func JSONVars(b []byte) (*runtime.OrderedDict, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	v, err := decodeValue(dec, tok)
	if err != nil {
		return nil, err
	}
	od, ok := v.(*runtime.OrderedDict)
	if !ok {
		return nil, fmt.Errorf("varsutil: JSONVars expected JSON object, got %T", v)
	}
	return od, nil
}

// decodeValue dispatches on the current JSON token, building an
// OrderedDict for objects and a []any for arrays. Mirrors the algorithm
// proven against the parity corpus.
func decodeValue(dec *json.Decoder, tok json.Token) (any, error) {
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			return decodeObject(dec)
		case '[':
			return decodeArray(dec)
		}
		return nil, fmt.Errorf("varsutil: unexpected delim %v", t)
	case string, bool, nil:
		return t, nil
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return i, nil
		}
		f, err := t.Float64()
		if err != nil {
			return nil, err
		}
		return f, nil
	}
	return nil, fmt.Errorf("varsutil: unexpected token %T", tok)
}

func decodeObject(dec *json.Decoder) (*runtime.OrderedDict, error) {
	out := runtime.NewOrderedDict()
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := keyTok.(string)
		if !ok {
			return nil, fmt.Errorf("varsutil: non-string object key %T", keyTok)
		}
		valTok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		val, err := decodeValue(dec, valTok)
		if err != nil {
			return nil, err
		}
		out.Set(key, val)
	}
	// Consume the closing '}'.
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	return out, nil
}

func decodeArray(dec *json.Decoder) ([]any, error) {
	out := []any{}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		v, err := decodeValue(dec, tok)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	// Consume the closing ']'.
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	return out, nil
}
