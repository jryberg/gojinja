package runtime

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
	keys []any
	vals map[any]any
}

// NewOrderedDict returns an empty OrderedDict ready for Set.
func NewOrderedDict() *OrderedDict {
	return &OrderedDict{vals: map[any]any{}}
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
	if d == nil {
		return 0
	}
	return len(d.keys)
}

// Get looks up a value by key. Returns (nil, false) on miss.
func (d *OrderedDict) Get(key any) (any, bool) {
	if d == nil {
		return nil, false
	}
	v, ok := d.vals[key]
	return v, ok
}

// Set inserts or replaces an entry. New keys are appended to the
// iteration order; existing keys keep their position.
func (d *OrderedDict) Set(key, value any) {
	if d.vals == nil {
		d.vals = map[any]any{}
	}
	if _, ok := d.vals[key]; !ok {
		d.keys = append(d.keys, key)
	}
	d.vals[key] = value
}

// Keys returns a copy of the keys in insertion order.
func (d *OrderedDict) Keys() []any {
	if d == nil {
		return nil
	}
	out := make([]any, len(d.keys))
	copy(out, d.keys)
	return out
}

// Values returns the values in insertion order.
func (d *OrderedDict) Values() []any {
	if d == nil {
		return nil
	}
	out := make([]any, 0, len(d.keys))
	for _, k := range d.keys {
		out = append(out, d.vals[k])
	}
	return out
}

// Items returns (key, value) pairs in insertion order. Each pair is a
// runtime.Tuple so rendering matches Python's `(k, v)` repr; the eval
// layer's tuple-target unpacking sees them as iterables.
func (d *OrderedDict) Items() []any {
	if d == nil {
		return nil
	}
	out := make([]any, 0, len(d.keys))
	for _, k := range d.keys {
		out = append(out, Tuple{k, d.vals[k]})
	}
	return out
}

// AsMap returns a snapshot as map[any]any, losing order.
func (d *OrderedDict) AsMap() map[any]any {
	if d == nil {
		return nil
	}
	out := make(map[any]any, len(d.vals))
	for k, v := range d.vals {
		out[k] = v
	}
	return out
}
