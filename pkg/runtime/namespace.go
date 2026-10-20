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
}

// NewNamespace constructs a Namespace seeded from the given map. The map
// is copied — callers can safely mutate the original afterwards.
func NewNamespace(seed map[string]any) *Namespace {
	ns := &Namespace{attrs: make(map[string]any, len(seed))}
	for k, v := range seed {
		ns.attrs[k] = v
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

// Set assigns name to value, replacing any prior binding.
func (n *Namespace) Set(name string, value any) {
	if n.attrs == nil {
		n.attrs = make(map[string]any)
	}
	n.attrs[name] = value
}

// Items returns the namespace's entries sorted by name. Sorting matches
// jinja2.utils.Namespace.__repr__ (which uses dict order in Python 3.7+,
// insertion order; gojinja chooses sorted order for deterministic
// rendering).
func (n *Namespace) Items() []NamespaceEntry {
	if n == nil {
		return nil
	}
	out := make([]NamespaceEntry, 0, len(n.attrs))
	for k, v := range n.attrs {
		out = append(out, NamespaceEntry{Name: k, Value: v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
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
