package runtime

import (
	"sort"

	"github.com/jryberg/gojinja/pkg/escape"
)

// Namespace is a mutable container of named values, equivalent to Jinja2's
// jinja2.utils.Namespace. Templates create one with the `namespace()`
// global and use `{% set ns.attr = ... %}` to set fields. Lookup is
// case-sensitive and returns the gojinja Undefined for missing names.
type Namespace struct {
	attrs map[string]any
	order []string // attribute names in assignment order
}

// NewNamespace constructs a Namespace seeded from the given map, in
// lex-sorted key order (a Go map carries none). The map is copied —
// callers can safely mutate the original afterwards.
func NewNamespace(seed map[string]any) *Namespace {
	keys := make([]string, 0, len(seed))
	for k := range seed {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	ns := &Namespace{}
	for _, k := range keys {
		ns.Set(k, seed[k])
	}
	return ns
}

// Get returns the value for name and whether it was present.
func (n *Namespace) Get(name string) (any, bool) {
	if n == nil {
		return nil, false
	}
	v, ok := n.attrs[name]
	return v, ok
}

// Set assigns name to value, replacing any prior binding; a new name
// goes last, as in a Python dict.
func (n *Namespace) Set(name string, value any) {
	if n.attrs == nil {
		n.attrs = make(map[string]any)
	}
	if _, ok := n.attrs[name]; !ok {
		n.order = append(n.order, name)
	}
	n.attrs[name] = value
}

// Items returns the namespace's entries in assignment order, as
// jinja2.utils.Namespace's dict keeps them.
func (n *Namespace) Items() []NamespaceEntry {
	if n == nil {
		return nil
	}
	out := make([]NamespaceEntry, 0, len(n.order))
	for _, k := range n.order {
		out = append(out, NamespaceEntry{Name: k, Value: n.attrs[k]})
	}
	return out
}

// NamespaceEntry is a (name, value) pair returned by [Namespace.Items].
type NamespaceEntry struct {
	Name  string
	Value any
}

// ReprAttrs returns what Jinja2's `<Namespace {...}>` repr shows.
func (n *Namespace) ReprAttrs() (string, []string, []any) {
	items := n.Items()
	keys := make([]string, len(items))
	values := make([]any, len(items))
	for i, e := range items {
		keys[i], values[i] = e.Name, e.Value
	}
	return "Namespace", keys, values
}

// String returns Jinja2's repr, <Namespace {'k': 'v'}>.
func (n *Namespace) String() string {
	if n == nil {
		return "<Namespace nil>"
	}
	return escape.Repr(n)
}
