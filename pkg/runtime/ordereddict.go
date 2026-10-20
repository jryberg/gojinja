package runtime

import (
	"fmt"
	"sort"

	"github.com/jryberg/gojinja/pkg/escape"
)

// OrderedDict is an insertion-ordered map[any]any — the runtime
// representation of dicts built inside templates (`{a: 1, b: 2}`,
// `dict(a=1, b=2)`). Python 3.7+ guarantees dict insertion order; we
// match that for templates built in gojinja so iteration, rendering,
// and `.items()` / `.keys()` / `.values()` all see keys in the order
// they were added.
//
// Plain Go `map[string]any` / `map[any]any` values handed in via the
// render context retain Go's random iteration order — there's no
// "insertion" we can reconstruct after the fact. Callers who need
// stable order on Go-side input should pass an OrderedDict from Go.
type OrderedDict struct {
	keys []any          // the key objects, in insertion order
	vals map[string]any // values by HashKey of their key
	// proxy makes this Python's mappingproxy: a read-only view through
	// which every operation reads proxy.
	proxy *OrderedDict
}

// NewOrderedDict returns an empty OrderedDict ready for Set.
func NewOrderedDict() *OrderedDict {
	return &OrderedDict{vals: map[string]any{}}
}

// NewOrderedDictFromPairs builds an OrderedDict from a sequence of
// (key, value) pairs in argument order. Duplicate keys keep the
// position of the first occurrence (Python's behaviour).
func NewOrderedDictFromPairs(pairs ...[2]any) *OrderedDict {
	d := NewOrderedDict()
	for _, p := range pairs {
		d.Set(p[0], p[1])
	}
	return d
}

// Len returns the number of entries.
func (d *OrderedDict) Len() int {
	d = d.data()
	if d == nil {
		return 0
	}
	return len(d.keys)
}

// Get looks up a value by key. Returns (nil, false) on miss.
func (d *OrderedDict) Get(key any) (any, bool) {
	d = d.data()
	if d == nil {
		return nil, false
	}
	k, err := HashKey(key)
	if err != nil {
		return nil, false
	}
	v, ok := d.vals[k]
	return v, ok
}

// Put is Set for a key from a template: an unhashable key (a list or a
// dict) is Python's TypeError.
func (d *OrderedDict) Put(key, value any) error {
	d = d.data()
	if _, err := HashKey(key); err != nil {
		return err
	}
	d.Set(key, value)
	return nil
}

// Set inserts or replaces an entry. New keys are appended to the
// iteration order; existing keys keep their position.
func (d *OrderedDict) Set(key, value any) {
	d = d.data()
	if d.vals == nil {
		d.vals = map[string]any{}
	}
	// Like Python, a new value for an equal key keeps the first key
	// object and its position.
	k := hashKeyOf(key)
	if _, ok := d.vals[k]; !ok {
		d.keys = append(d.keys, key)
	}
	d.vals[k] = value
}

// Keys returns a copy of the keys in insertion order.
func (d *OrderedDict) Keys() []any {
	d = d.data()
	if d == nil {
		return nil
	}
	out := make([]any, len(d.keys))
	copy(out, d.keys)
	return out
}

// Values returns the values in insertion order.
func (d *OrderedDict) Values() []any {
	d = d.data()
	if d == nil {
		return nil
	}
	out := make([]any, 0, len(d.keys))
	for _, k := range d.keys {
		out = append(out, d.vals[hashKeyOf(k)])
	}
	return out
}

// Items returns (key, value) pairs in insertion order. Each pair is a
// runtime.Tuple so rendering matches Python's `(k, v)` repr; the eval
// layer's tuple-target unpacking sees them as iterables.
func (d *OrderedDict) Items() []any {
	d = d.data()
	if d == nil {
		return nil
	}
	out := make([]any, 0, len(d.keys))
	for _, k := range d.keys {
		out = append(out, Tuple{k, d.vals[hashKeyOf(k)]})
	}
	return out
}

// Delete removes key. Returns true if it existed.
func (d *OrderedDict) Delete(key any) bool {
	d = d.data()
	if d == nil {
		return false
	}
	hk := hashKeyOf(key)
	if _, ok := d.vals[hk]; !ok {
		return false
	}
	delete(d.vals, hk)
	for i, k := range d.keys {
		if hashKeyOf(k) == hk {
			d.keys = append(d.keys[:i], d.keys[i+1:]...)
			break
		}
	}
	return true
}

// Update mirrors Python's dict.update: merge another mapping into d,
// overwriting existing keys. Accepts *OrderedDict, map[string]any,
// map[any]any, or a sequence of (k, v) pairs.
func (d *OrderedDict) Update(other any) error {
	d = d.data()
	if d == nil {
		return fmt.Errorf("update on nil dict")
	}
	switch x := other.(type) {
	case nil:
		return nil
	case *OrderedDict:
		if x == nil {
			return nil
		}
		for _, k := range x.keys {
			d.Set(k, x.vals[hashKeyOf(k)])
		}
		return nil
	case map[string]any:
		// Go map iteration is randomised; sort for deterministic
		// insertion order so two updates of the same map produce the
		// same OrderedDict layout.
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			d.Set(k, x[k])
		}
		return nil
	case map[any]any:
		for k, v := range x {
			d.Set(k, v)
		}
		return nil
	case []any, Tuple:
		// Accept a sequence of 2-element pairs, mirroring
		// `dict.update([(k, v), ...])`.
		items, _ := x.([]any)
		if t, ok := x.(Tuple); ok {
			items = []any(t)
		}
		for _, pair := range items {
			pk, pv, ok := pairKV(pair)
			if !ok {
				return fmt.Errorf("dict update sequence element is not a 2-tuple")
			}
			d.Set(pk, pv)
		}
		return nil
	}
	if l, ok := other.(Lister); ok {
		for _, pair := range l.Items() {
			pk, pv, ok := pairKV(pair)
			if !ok {
				return fmt.Errorf("dict update sequence element is not a 2-tuple")
			}
			d.Set(pk, pv)
		}
		return nil
	}
	return fmt.Errorf("dict.update: unsupported source type %T", other)
}

// Pop removes and returns the value for key. With a default arg,
// returns the default when key is absent; without one, returns an
// error.
func (d *OrderedDict) Pop(key any, def any, hasDef bool) (any, error) {
	d = d.data()
	if d == nil || d.vals == nil {
		if hasDef {
			return def, nil
		}
		return nil, fmt.Errorf("pop from empty dict")
	}
	if v, ok := d.Get(key); ok {
		d.Delete(key)
		return v, nil
	}
	if hasDef {
		return def, nil
	}
	return nil, fmt.Errorf("%s", escape.Repr(key))
}

// SetDefault returns d[key] if present; otherwise inserts and returns
// def.
func (d *OrderedDict) SetDefault(key any, def any) any {
	d = d.data()
	if d == nil {
		return def
	}
	if v, ok := d.Get(key); ok {
		return v
	}
	d.Set(key, def)
	return def
}

// PopItem removes and returns the last-inserted entry as a Tuple of
// (key, value), matching Python 3.7+ LIFO semantics. Returns an error
// when d is empty.
func (d *OrderedDict) PopItem() (Tuple, error) {
	d = d.data()
	if d == nil || len(d.keys) == 0 {
		return nil, fmt.Errorf("popitem(): dictionary is empty")
	}
	last := len(d.keys) - 1
	k := d.keys[last]
	hk := hashKeyOf(k)
	v := d.vals[hk]
	d.keys = d.keys[:last]
	delete(d.vals, hk)
	return Tuple{k, v}, nil
}

// Copy returns a shallow copy of d preserving insertion order.
// Keys and values are not deep-copied; nested containers are shared.
func (d *OrderedDict) Copy() *OrderedDict {
	d = d.data()
	if d == nil {
		return NewOrderedDict()
	}
	out := &OrderedDict{
		keys: append([]any(nil), d.keys...),
		vals: make(map[string]any, len(d.vals)),
	}
	for k, v := range d.vals {
		out.vals[k] = v
	}
	return out
}

// Clear empties the dict in place.
func (d *OrderedDict) Clear() {
	d = d.data()
	if d == nil {
		return
	}
	d.keys = d.keys[:0]
	d.vals = map[string]any{}
}

// MergeMap appends entries from m into d in lex-sorted key order.
// Existing keys keep their slot; new keys append. Use when merging a Go
// map into an OrderedDict where deterministic iteration is required
// (Go's `range` over a map is randomised, so the sort substitutes for
// the insertion order Python preserves but Go doesn't carry).
func (d *OrderedDict) MergeMap(m map[string]any) {
	d = d.data()
	if d == nil || len(m) == 0 {
		return
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		d.Set(k, m[k])
	}
}

// MergeOrderedDict appends entries from src into d in src's key order.
// Existing keys keep their slot; new keys append.
func (d *OrderedDict) MergeOrderedDict(src *OrderedDict) {
	d = d.data()
	if d == nil || src == nil {
		return
	}
	for _, k := range src.Keys() {
		v, _ := src.Get(k)
		d.Set(k, v)
	}
}

// OrderedDictFromMap returns a new OrderedDict containing m's entries in
// lex-sorted key order. Equivalent to NewOrderedDict followed by
// MergeMap; provided as a convenience for the common one-shot
// conversion.
func OrderedDictFromMap(m map[string]any) *OrderedDict {
	out := NewOrderedDict()
	out.MergeMap(m)
	return out
}

// pairKV unpacks a (key, value) pair from a Tuple, []any, or
// *PyList of length 2. Used by OrderedDict.Update when the source is
// a sequence-of-pairs rather than another mapping.
func pairKV(pair any) (any, any, bool) {
	switch p := pair.(type) {
	case Tuple:
		if len(p) == 2 {
			return p[0], p[1], true
		}
	case []any:
		if len(p) == 2 {
			return p[0], p[1], true
		}
	case *PyList:
		if p != nil {
			items := p.Items()
			if len(items) == 2 {
				return items[0], items[1], true
			}
		}
	}
	return nil, nil, false
}

// NewMappingProxy is types.MappingProxyType(d), which dict views'
// mapping attribute gives: a read-only view of d that reads it live. It
// is an OrderedDict, so it reads, iterates and compares as d does; only
// its type name and repr, and the methods templates get, differ.
func NewMappingProxy(d *OrderedDict) *OrderedDict { return &OrderedDict{proxy: d} }

// IsProxy reports whether d is a mappingproxy.
func (d *OrderedDict) IsProxy() bool { return d != nil && d.proxy != nil }

// ProxyTarget returns the dict a mappingproxy views.
func (d *OrderedDict) ProxyTarget() (any, bool) {
	if !d.IsProxy() {
		return nil, false
	}
	return d.data(), true
}

// data is the dict d's operations act on.
func (d *OrderedDict) data() *OrderedDict {
	for d != nil && d.proxy != nil {
		d = d.proxy
	}
	return d
}
