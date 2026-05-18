package runtime

import "sort"

// Context carries the per-render variable bindings and inheritance state.
// It mirrors jinja2.runtime.Context: a layered name lookup (Vars → Parent),
// plus an EvalContext, plus the block stack used for template inheritance.
//
// A Context is **per render** — never share one across goroutines or
// between concurrent renders. The Environment that produced it is held
// by Env, but Context never mutates the env.
type Context struct {
	// Parent is the immutable base scope (env globals + render args).
	// An OrderedDict so iteration matches Python's insertion-ordered
	// dict semantics — see NewOrderedDict / OrderedDict.
	Parent *OrderedDict
	// Vars is the mutable scope filled by `{% set %}` and `{% for %}`.
	Vars map[string]any
	// Eval carries the eval-time autoescape / volatile flags.
	Eval *EvalContext
	// Name is the template name being rendered ("" for from-string).
	Name string
	// Blocks maps a block name to its inheritance stack of render funcs,
	// oldest ancestor first. The active level is len(stack)-1; super()
	// walks toward 0.
	Blocks map[string][]BlockRenderFunc
	// Exported tracks variable names that should be visible after a
	// render — used by `{% from %}` imports.
	Exported map[string]bool
	// Env is opaque to runtime; the evaluator uses it to call filters and
	// do safe attr/item access.
	Env any
}

// BlockRenderFunc renders one level of a block in the inheritance chain.
type BlockRenderFunc func(*Context) (string, error)

// CallerFunc is the runtime contract for `caller()` produced by the
// `{% call %}` block. Lives here so both pkg/eval (which produces it)
// and pkg/environment (which dispatches the call) share the same named
// type — Go type switches only match named types when the names agree.
type CallerFunc func(args []any, kwargs map[string]any) (any, error)

// NewContext constructs a Context with the supplied parent globals and a
// freshly allocated empty Vars map. parent may be nil (treated as
// empty).
func NewContext(name string, env any, parent *OrderedDict, eval *EvalContext) *Context {
	if parent == nil {
		parent = NewOrderedDict()
	}
	return &Context{
		Parent:   parent,
		Vars:     map[string]any{},
		Eval:     eval,
		Name:     name,
		Blocks:   map[string][]BlockRenderFunc{},
		Exported: map[string]bool{},
		Env:      env,
	}
}

// Resolve looks up name in Vars, then Parent. Returns (value, true) on
// hit; (nil, false) on miss. Templates use [Context.ResolveOrUndefined]
// instead so that misses produce an Undefined sentinel.
func (c *Context) Resolve(name string) (any, bool) {
	if v, ok := c.Vars[name]; ok {
		return v, true
	}
	if v, ok := c.Parent.Get(name); ok {
		return v, true
	}
	return nil, false
}

// ResolveOrUndefined returns the value or an Undefined produced by the
// supplied factory when the name is missing. The factory is the engine's
// configured Undefined kind (Base/Chainable/Debug/Strict).
func (c *Context) ResolveOrUndefined(name string, factory UndefinedFactory) any {
	if v, ok := c.Resolve(name); ok {
		return v
	}
	if factory == nil {
		return NewBase("", name, nil, nil)
	}
	return factory("", name, nil, nil)
}

// Derived returns a child context that inherits parent + vars but allows
// independent mutation. Used by includes/macros. The merged Parent keeps
// insertion order: existing keys retain their slot, new keys append.
func (c *Context) Derived(locals map[string]any) *Context {
	merged := NewOrderedDict()
	if c.Parent != nil {
		for _, k := range c.Parent.Keys() {
			v, _ := c.Parent.Get(k)
			merged.Set(k, v)
		}
	}
	for k, v := range c.Vars {
		merged.Set(k, v)
	}
	for k, v := range locals {
		merged.Set(k, v)
	}
	return &Context{
		Parent:   merged,
		Vars:     map[string]any{},
		Eval:     c.Eval,
		Name:     c.Name,
		Blocks:   c.Blocks,
		Exported: map[string]bool{},
		Env:      c.Env,
	}
}

// Set stores name=value in the mutable scope.
func (c *Context) Set(name string, value any) {
	c.Vars[name] = value
}

// Export marks a name as exported.
func (c *Context) Export(name string) {
	c.Exported[name] = true
}

// All returns a merged snapshot of Parent ∪ Vars. Vars wins on collision.
// The result is a copy — callers can mutate safely. Order is not
// preserved (the return type is a Go map); use [Context.AllOrdered] if
// you need insertion-order iteration.
func (c *Context) All() map[string]any {
	parentLen := 0
	if c.Parent != nil {
		parentLen = c.Parent.Len()
	}
	out := make(map[string]any, parentLen+len(c.Vars))
	if c.Parent != nil {
		for _, k := range c.Parent.Keys() {
			ks, ok := k.(string)
			if !ok {
				continue
			}
			v, _ := c.Parent.Get(k)
			out[ks] = v
		}
	}
	for k, v := range c.Vars {
		out[k] = v
	}
	return out
}

// AllOrdered returns a merged snapshot of Parent ∪ Vars as an
// insertion-ordered dict: Parent keys first in their existing order,
// then Vars keys (sorted lexicographically because Go map iteration is
// randomized). Vars wins on collision and keeps the Parent slot.
func (c *Context) AllOrdered() *OrderedDict {
	out := NewOrderedDict()
	if c.Parent != nil {
		for _, k := range c.Parent.Keys() {
			v, _ := c.Parent.Get(k)
			out.Set(k, v)
		}
	}
	keys := make([]string, 0, len(c.Vars))
	for k := range c.Vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		out.Set(k, c.Vars[k])
	}
	return out
}
