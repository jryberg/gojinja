package runtime

import (
	"fmt"
	"reflect"
	"sort"
)

// PyList is the runtime representation of a list literal built inside a
// template (`{% set xs = [] %}`, `[1, 2, 3]`). It carries pointer-receiver
// mutating methods that mirror Python's list — `append`, `extend`,
// `insert`, `pop`, `remove`, `clear`, `reverse`, `sort` — so templates
// that work in canonical Jinja2 with the `do` extension also work here.
//
// Caller-supplied `[]any` (data injected via the render context) keeps
// its bare slice type and stays immutable. The split honours the sandbox
// principle: only values created inside the template gain mutation.
//
// PyList implements the [Lister] interface so generic consumers can
// recover the underlying slice without binding to this concrete type.
type PyList struct {
	items []any
}

// NewPyList wraps an existing []any. The slice is adopted, not copied —
// callers should hand over ownership.
func NewPyList(items []any) *PyList {
	if items == nil {
		items = []any{}
	}
	return &PyList{items: items}
}

// Items returns the underlying slice. Mutations through the returned
// slice are visible to subsequent calls; in template-eval flows this is
// fine because rendering is single-goroutine.
func (l *PyList) Items() []any {
	if l == nil {
		return nil
	}
	return l.items
}

// Len returns the element count.
func (l *PyList) Len() int {
	if l == nil {
		return 0
	}
	return len(l.items)
}

// Append mirrors Python's list.append.
func (l *PyList) Append(v any) { l.items = append(l.items, v) }

// Extend mirrors Python's list.extend: walk the iterable, append each
// element. Accepts []any, *PyList, Tuple, or any value implementing
// Lister.
func (l *PyList) Extend(seq any) error {
	switch x := seq.(type) {
	case nil:
		return nil
	case []any:
		l.items = append(l.items, x...)
		return nil
	case *PyList:
		if x != nil {
			l.items = append(l.items, x.items...)
		}
		return nil
	case Tuple:
		l.items = append(l.items, []any(x)...)
		return nil
	}
	if other, ok := seq.(Lister); ok {
		l.items = append(l.items, other.Items()...)
		return nil
	}
	// Fall back to reflect for arbitrary slices.
	v := reflect.ValueOf(seq)
	if v.Kind() == reflect.Slice || v.Kind() == reflect.Array {
		for i := 0; i < v.Len(); i++ {
			l.items = append(l.items, v.Index(i).Interface())
		}
		return nil
	}
	return fmt.Errorf("list.extend: argument of type %T is not iterable", seq)
}

// Insert mirrors Python's list.insert with negative-index handling.
func (l *PyList) Insert(idx int, v any) {
	n := len(l.items)
	if idx < 0 {
		idx += n
		if idx < 0 {
			idx = 0
		}
	}
	if idx > n {
		idx = n
	}
	l.items = append(l.items, nil)
	copy(l.items[idx+1:], l.items[idx:])
	l.items[idx] = v
}

// Pop mirrors Python's list.pop. With no args, pops the last element.
// With one int arg (possibly negative), pops at that index. Returns an
// error if the list is empty or the index is out of range.
func (l *PyList) Pop(idx int, hasIdx bool) (any, error) {
	n := len(l.items)
	if n == 0 {
		return nil, fmt.Errorf("pop from empty list")
	}
	pos := n - 1
	if hasIdx {
		pos = idx
		if pos < 0 {
			pos += n
		}
		if pos < 0 || pos >= n {
			return nil, fmt.Errorf("pop index out of range")
		}
	}
	v := l.items[pos]
	l.items = append(l.items[:pos], l.items[pos+1:]...)
	return v, nil
}

// Remove deletes the first element equal to v, or returns an error.
// Equality uses reflect.DeepEqual with numeric widening, mirroring the
// helper used by [].count / [].index.
func (l *PyList) Remove(v any, eq func(a, b any) bool) error {
	for i, it := range l.items {
		if eq(it, v) {
			l.items = append(l.items[:i], l.items[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("list.remove(x): x not in list")
}

// Clear empties the list in place.
func (l *PyList) Clear() { l.items = l.items[:0] }

// Reverse reverses in place.
func (l *PyList) Reverse() {
	for i, j := 0, len(l.items)-1; i < j; i, j = i+1, j-1 {
		l.items[i], l.items[j] = l.items[j], l.items[i]
	}
}

// Sort sorts in place. The less callback decides ordering; if reverse
// is true, the comparison is inverted. Pass a nil less to fall back to
// the caller's default (the environment package wires this with its
// Python-ish ordering).
func (l *PyList) Sort(less func(a, b any) bool, reverse bool) {
	if less == nil {
		// Should never happen in production — environment passes a
		// comparator. Be safe rather than panic.
		return
	}
	cmp := less
	if reverse {
		cmp = func(a, b any) bool { return less(b, a) }
	}
	sort.SliceStable(l.items, func(i, j int) bool {
		return cmp(l.items[i], l.items[j])
	})
}

// Lister is satisfied by any list-like value whose elements can be
// surfaced as a []any view. Used by toIterable / truthy / filter
// dispatch to handle PyList (and any future variants) without binding
// to a concrete type.
type Lister interface {
	Items() []any
}
