// Package escape implements HTML escaping and the [Markup] type — the
// gojinja equivalent of Python's markupsafe. A Markup value is a string
// the engine has guaranteed safe for HTML output; concatenating Markup
// with plain strings escapes the plain strings on the way in.
//
// The escape table matches markupsafe exactly:
//
//	&  -> &amp;
//	<  -> &lt;
//	>  -> &gt;
//	'  -> &#39;
//	"  -> &#34;
package escape

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

// Markup is a string the engine has marked as already-safe for HTML
// output. Autoescape passes Markup values through verbatim; plain
// strings get HTML-escaped on the way out.
//
// **When to use Markup directly**: most code shouldn't. The engine
// produces Markup itself for filter outputs that are intentionally
// pre-escaped (`safe`, `escape`, `tojson`, `urlize`, `xmlattr`).
// Custom filters returning HTML should also wrap in Markup.
//
// **Caution**: marking a string as Markup turns off autoescape for that
// value. Never wrap untrusted input in Markup directly — the whole
// point of the autoescape default is to prevent that mistake.
//
// Example:
//
//	// In a custom filter that produces a snippet of HTML:
//	return escape.Markup(fmt.Sprintf("<b>%s</b>", escape.Escape(name))), nil
type Markup string

// String returns the underlying string, satisfying [fmt.Stringer].
func (m Markup) String() string { return string(m) }

// HTML returns m unchanged. Implementing [HTMLer] makes Markup self-marking.
func (m Markup) HTML() Markup { return m }

// HTMLer is the contract for values that already represent safe HTML. The
// engine treats values that satisfy HTMLer as if they were [Markup], i.e. it
// does not re-escape them.
type HTMLer interface {
	HTML() Markup
}

// IsMarkup reports whether v is a [Markup] value or implements [HTMLer], and
// returns the safe Markup form. Useful in branching code that needs to
// preserve safe values.
func IsMarkup(v any) (Markup, bool) {
	switch x := v.(type) {
	case Markup:
		return x, true
	case HTMLer:
		return x.HTML(), true
	default:
		return "", false
	}
}

// Escape returns v as a [Markup]. If v is already safe (Markup or
// satisfies [HTMLer]) it is returned unchanged; otherwise its string form
// is HTML-escaped.
//
// Idempotent: `Escape(Escape(x)) == Escape(x)`.
//
// Example:
//
//	escape.Escape("<b>")               →  Markup("&lt;b&gt;")
//	escape.Escape(escape.Markup("<b>")) →  Markup("<b>")  (unchanged)
func Escape(v any) Markup {
	if m, ok := IsMarkup(v); ok {
		return m
	}
	return Markup(escapeString(SoftStr(v)))
}

// ForceEscape always escapes — even values that are already [Markup].
// Used to implement the `forceescape` filter.
//
// Use when you have a Markup value you don't trust (e.g. came from a
// caller you don't want to grant autoescape-bypass).
//
// Example:
//
//	escape.ForceEscape(escape.Markup("<b>"))  →  Markup("&lt;b&gt;")
func ForceEscape(v any) Markup {
	return Markup(escapeString(SoftStr(v)))
}

// SoftStr returns the string form of v *without* escaping. It mirrors
// Python's str(): Markup values keep their underlying text; nil renders
// as "None"; bool renders as "True"/"False"; lists render as Python list
// reprs (`[1, 'a', True]`); maps render as Python dict reprs
// (`{'k': 'v'}`). Matching Python here is what gives identical output to
// canonical Jinja2 — `{{ x }}` of any value should be byte-equal to the
// Python rendering of the same value.
func SoftStr(v any) string {
	switch x := v.(type) {
	case nil:
		return "None"
	case bool:
		if x {
			return "True"
		}
		return "False"
	case string:
		return x
	case Markup:
		return string(x)
	case HTMLer:
		return string(x.HTML())
	case []byte:
		return string(x)
	case float32:
		return pyFloatStr(float64(x))
	case float64:
		return pyFloatStr(x)
	case []any:
		return pyListStr(x)
	case tupleRepr:
		return pyTupleStr(x.Iter())
	case map[string]any:
		return pyDictStr(stringMapToAny(x))
	case map[any]any:
		return pyDictStr(x)
	case orderedDictRepr:
		return pyOrderedDictStr(x)
	case fmt.Stringer:
		return x.String()
	default:
		return fmt.Sprint(v)
	}
}

// tupleRepr is the duck-typing interface for runtime.Tuple — kept
// without an import on pkg/runtime to avoid a cycle.
type tupleRepr interface {
	Iter() []any
}

// pyTupleStr renders a tuple Python-style: `(1,)` for a singleton (with
// trailing comma) and `(a, b, ...)` otherwise.
func pyTupleStr(items []any) string {
	var b strings.Builder
	b.WriteByte('(')
	for i, it := range items {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(pyRepr(it))
	}
	if len(items) == 1 {
		b.WriteByte(',')
	}
	b.WriteByte(')')
	return b.String()
}

// pyFloatStr renders a float Python-style: integral values get a `.0`
// suffix (`1.0`, not `1`), non-integral use Go's shortest-round-trip
// representation (matches Python's `repr` for the common cases).
func pyFloatStr(f float64) string {
	if math.IsNaN(f) {
		return "nan"
	}
	if math.IsInf(f, 1) {
		return "inf"
	}
	if math.IsInf(f, -1) {
		return "-inf"
	}
	// Use shortest round-trip: 'g' with precision -1.
	s := strconv.FormatFloat(f, 'g', -1, 64)
	// If there's no decimal point or exponent, append ".0".
	if !strings.ContainsAny(s, ".eE") {
		s += ".0"
	}
	return s
}

// orderedDictRepr lets escape recognise insertion-ordered dicts without
// importing pkg/runtime. Any type implementing this interface is
// rendered in the order Keys() returns. (`*runtime.OrderedDict`
// satisfies this in the runtime package.)
type orderedDictRepr interface {
	Keys() []any
	Get(any) (any, bool)
}

// pyRepr returns Python's repr() of v — used inside container reprs.
// Strings get single-quoted with backslash-escaped specials; numbers
// and bools use their str() form; nil → None; nested containers
// recurse.
func pyRepr(v any) string {
	switch x := v.(type) {
	case nil:
		return "None"
	case bool:
		if x {
			return "True"
		}
		return "False"
	case string:
		return pyStringRepr(x)
	case Markup:
		return "Markup(" + pyStringRepr(string(x)) + ")"
	case []byte:
		return pyStringRepr(string(x))
	case float32:
		return pyFloatStr(float64(x))
	case float64:
		return pyFloatStr(x)
	case []any:
		return pyListStr(x)
	case tupleRepr:
		return pyTupleStr(x.Iter())
	case map[string]any:
		return pyDictStr(stringMapToAny(x))
	case map[any]any:
		return pyDictStr(x)
	case orderedDictRepr:
		return pyOrderedDictStr(x)
	case fmt.Stringer:
		return x.String()
	}
	return fmt.Sprint(v)
}

// pyListStr renders Python's `[item, item, ...]` form by repr-ing each
// element.
func pyListStr(items []any) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, it := range items {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(pyRepr(it))
	}
	b.WriteByte(']')
	return b.String()
}

// pyOrderedDictStr renders an OrderedDict in insertion order, matching
// Python 3.7+ dict iteration semantics.
func pyOrderedDictStr(d orderedDictRepr) string {
	var b strings.Builder
	b.WriteByte('{')
	keys := d.Keys()
	for i, k := range keys {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(pyRepr(k))
		b.WriteString(": ")
		v, _ := d.Get(k)
		b.WriteString(pyRepr(v))
	}
	b.WriteByte('}')
	return b.String()
}

// pyDictStr renders Python's `{key: value, ...}` form. Keys are sorted
// (Go maps have no insertion order) so output is deterministic — for
// strict insertion-order parity, callers should pipe through |dictsort.
func pyDictStr(m map[any]any) string {
	keys := make([]any, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sortAnySlice(keys)
	var b strings.Builder
	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(pyRepr(k))
		b.WriteString(": ")
		b.WriteString(pyRepr(m[k]))
	}
	b.WriteByte('}')
	return b.String()
}

// pyStringRepr returns Python's repr() of a string: single-quoted, with
// `'` itself escaped only when the string contains no `"` (Python uses
// double quotes if the value contains a single quote and no double).
// For our parity surface — short keys / values — single quotes always
// match unless the value contains both kinds, which we render with
// `\'`.
func pyStringRepr(s string) string {
	hasSingle := strings.ContainsRune(s, '\'')
	hasDouble := strings.ContainsRune(s, '"')
	quote := byte('\'')
	if hasSingle && !hasDouble {
		quote = '"'
	}
	var b strings.Builder
	b.WriteByte(quote)
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case rune(quote):
			b.WriteByte('\\')
			b.WriteRune(r)
		default:
			if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&b, `\x%02x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte(quote)
	return b.String()
}

func stringMapToAny(m map[string]any) map[any]any {
	out := make(map[any]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// sortAnySlice sorts keys in Python-friendly order: strings
// lexicographically, numbers by value, others by their repr.
func sortAnySlice(keys []any) {
	sort.Slice(keys, func(i, j int) bool {
		return anyLess(keys[i], keys[j])
	})
}

func anyLess(a, b any) bool {
	as, aok := a.(string)
	bs, bok := b.(string)
	if aok && bok {
		return as < bs
	}
	return pyRepr(a) < pyRepr(b)
}

// Concat joins values into a single Markup. Plain values are escaped; Markup
// values are concatenated as-is. This is the join used in autoescape mode.
func Concat(values ...any) Markup {
	var b strings.Builder
	for _, v := range values {
		if m, ok := IsMarkup(v); ok {
			b.WriteString(string(m))
			continue
		}
		b.WriteString(escapeString(SoftStr(v)))
	}
	return Markup(b.String())
}

// PlainConcat joins values into a plain string, calling [SoftStr] on each.
// Used when autoescape is off.
func PlainConcat(values ...any) string {
	var b strings.Builder
	for _, v := range values {
		b.WriteString(SoftStr(v))
	}
	return b.String()
}

// escapeString applies the markupsafe escape table to s.
func escapeString(s string) string {
	if !needsEscape(s) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 8)
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '\'':
			b.WriteString("&#39;")
		case '"':
			b.WriteString("&#34;")
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// needsEscape returns true if s contains any character that would be escaped.
// Lets us skip allocation for the common all-safe input case.
func needsEscape(s string) bool {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '&', '<', '>', '\'', '"':
			return true
		}
	}
	return false
}
