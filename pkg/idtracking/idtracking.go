// Package idtracking implements name resolution / symbol tracking over the
// AST. Mirrors Jinja2's idtracking.py — Symbols, FrameSymbolVisitor,
// RootVisitor, and the four load kinds (parameter, resolve, alias,
// undefined).
package idtracking

import (
	"fmt"
	"sort"

	"github.com/jryberg/gojinja/pkg/ast"
)

// Load kinds. Mirrors VAR_LOAD_* in Python.
const (
	LoadParameter = "param"
	LoadResolve   = "resolve"
	LoadAlias     = "alias"
	LoadUndefined = "undefined"
)

// Load describes how a target identifier is materialised at runtime.
//
// Kind:
//   - "param"     — supplied by the caller (declare_parameter); Arg is "".
//   - "resolve"   — looked up from the surrounding context by Arg (the
//     original template-side name).
//   - "alias"     — resolved by reference to an outer-scope identifier in
//     Arg (e.g. "l_0_x").
//   - "undefined" — has no value; reads return the configured Undefined.
type Load struct {
	Kind string
	Arg  string
}

// Symbols collects refs/loads/stores for one frame. Frames stack via Parent;
// inner frames inherit refs from outer ones via FindRef / FindLoad.
type Symbols struct {
	Level  int
	Parent *Symbols
	// Refs maps template-side name → compiler-internal identifier
	// (`l_<level>_<name>`).
	Refs map[string]string
	// Loads maps compiler-internal identifier → Load info.
	Loads map[string]Load
	// Stores is the set of names assigned in this frame.
	Stores map[string]struct{}
}

// New constructs a fresh Symbols frame. Pass nil for the top-level frame.
func New(parent *Symbols) *Symbols {
	level := 0
	if parent != nil {
		level = parent.Level + 1
	}
	return &Symbols{
		Level:  level,
		Parent: parent,
		Refs:   map[string]string{},
		Loads:  map[string]Load{},
		Stores: map[string]struct{}{},
	}
}

// FindSymbols runs FrameSymbolVisitor over a sequence of nodes and returns
// the resulting Symbols. Mirrors `find_symbols`.
func FindSymbols(nodes []ast.Node, parent *Symbols) *Symbols {
	s := New(parent)
	v := &FrameSymbolVisitor{Symbols: s}
	for _, n := range nodes {
		v.Visit(n, false)
	}
	return s
}

// SymbolsForNode runs the RootVisitor over a single node. Mirrors
// `symbols_for_node`.
func SymbolsForNode(node ast.Node, parent *Symbols) *Symbols {
	s := New(parent)
	s.AnalyzeNode(node)
	return s
}

// AnalyzeNode runs the RootVisitor on the supplied node, populating refs,
// loads, and stores in this frame. The `for_branch` argument (Python kwarg)
// is exposed via [WithForBranch].
func (s *Symbols) AnalyzeNode(node ast.Node, opts ...AnalyzeOption) {
	o := analyzeOptions{forBranch: "body"}
	for _, op := range opts {
		op(&o)
	}
	v := &RootVisitor{frame: &FrameSymbolVisitor{Symbols: s}, forBranch: o.forBranch}
	v.Visit(node)
}

type analyzeOptions struct {
	forBranch string
}

// AnalyzeOption configures [Symbols.AnalyzeNode].
type AnalyzeOption func(*analyzeOptions)

// WithForBranch selects which subtree of a [*ast.For] node the analyzer
// descends into: "body" (default), "else", or "test".
func WithForBranch(branch string) AnalyzeOption {
	return func(o *analyzeOptions) { o.forBranch = branch }
}

// Ref returns the identifier for `name`, panicking when no frame in the
// chain has seen it. Mirrors Python's `ref(name)` (which raises
// AssertionError).
func (s *Symbols) Ref(name string) string {
	r := s.FindRef(name)
	if r == "" {
		panic(fmt.Sprintf("idtracking: tried to resolve a name to a reference that was unknown to the frame (%q)", name))
	}
	return r
}

// FindRef looks up the identifier for `name` in this frame, then any
// ancestor. Returns "" when the name is unknown.
func (s *Symbols) FindRef(name string) string {
	if id, ok := s.Refs[name]; ok {
		return id
	}
	if s.Parent != nil {
		return s.Parent.FindRef(name)
	}
	return ""
}

// FindLoad walks the frame chain looking for the load info attached to the
// given identifier.
func (s *Symbols) FindLoad(target string) (Load, bool) {
	if l, ok := s.Loads[target]; ok {
		return l, true
	}
	if s.Parent != nil {
		return s.Parent.FindLoad(target)
	}
	return Load{}, false
}

// Store records `name` as assigned in this frame and, if it has not been
// referenced yet, picks an alias / undefined load for it.
func (s *Symbols) Store(name string) {
	s.Stores[name] = struct{}{}
	if _, seen := s.Refs[name]; seen {
		return
	}
	if s.Parent != nil {
		if outer := s.Parent.FindRef(name); outer != "" {
			s.defineRef(name, &Load{Kind: LoadAlias, Arg: outer})
			return
		}
	}
	s.defineRef(name, &Load{Kind: LoadUndefined})
}

// DeclareParameter registers `name` as supplied by the caller. Returns the
// minted identifier.
func (s *Symbols) DeclareParameter(name string) string {
	s.Stores[name] = struct{}{}
	return s.defineRef(name, &Load{Kind: LoadParameter})
}

// Load registers `name` as referenced. If the name is already known
// (locally or in a parent frame), nothing changes; otherwise it gets a
// resolve load.
func (s *Symbols) Load(name string) {
	if s.FindRef(name) != "" {
		return
	}
	s.defineRef(name, &Load{Kind: LoadResolve, Arg: name})
}

// BranchUpdate folds the contributions of multiple sibling frames (the
// branches of an If / elif / else) back into this frame, choosing the right
// load kind for stores that only existed inside a branch.
func (s *Symbols) BranchUpdate(branches []*Symbols) {
	stores := map[string]struct{}{}
	for _, b := range branches {
		for k := range b.Stores {
			stores[k] = struct{}{}
		}
	}
	for k := range s.Stores {
		delete(stores, k)
	}
	for _, b := range branches {
		for k, v := range b.Refs {
			s.Refs[k] = v
		}
		for k, v := range b.Loads {
			s.Loads[k] = v
		}
		for k := range b.Stores {
			s.Stores[k] = struct{}{}
		}
	}
	for name := range stores {
		target := s.FindRef(name)
		if target == "" {
			panic(fmt.Sprintf("idtracking: branch_update could not find ref for %q", name))
		}
		if s.Parent != nil {
			if outer := s.Parent.FindRef(name); outer != "" {
				s.Loads[target] = Load{Kind: LoadAlias, Arg: outer}
				continue
			}
		}
		s.Loads[target] = Load{Kind: LoadResolve, Arg: name}
	}
}

// DumpStores returns the {name: identifier} mapping for every name stored
// in this frame or any ancestor (closest frame wins).
func (s *Symbols) DumpStores() map[string]string {
	rv := map[string]string{}
	for n := s; n != nil; n = n.Parent {
		names := make([]string, 0, len(n.Stores))
		for k := range n.Stores {
			names = append(names, k)
		}
		sort.Strings(names)
		for _, name := range names {
			if _, seen := rv[name]; !seen {
				if id := s.FindRef(name); id != "" {
					rv[name] = id
				}
			}
		}
	}
	return rv
}

// DumpParamTargets returns the set of identifiers loaded as parameters in
// this frame or any ancestor.
func (s *Symbols) DumpParamTargets() map[string]struct{} {
	rv := map[string]struct{}{}
	for n := s; n != nil; n = n.Parent {
		for target, load := range s.Loads {
			if load.Kind == LoadParameter {
				rv[target] = struct{}{}
			}
		}
		_ = n
	}
	return rv
}

// Copy returns a shallow copy of s with cloned refs/loads/stores maps.
func (s *Symbols) Copy() *Symbols {
	rv := &Symbols{
		Level:  s.Level,
		Parent: s.Parent,
		Refs:   make(map[string]string, len(s.Refs)),
		Loads:  make(map[string]Load, len(s.Loads)),
		Stores: make(map[string]struct{}, len(s.Stores)),
	}
	for k, v := range s.Refs {
		rv.Refs[k] = v
	}
	for k, v := range s.Loads {
		rv.Loads[k] = v
	}
	for k := range s.Stores {
		rv.Stores[k] = struct{}{}
	}
	return rv
}

func (s *Symbols) defineRef(name string, load *Load) string {
	id := fmt.Sprintf("l_%d_%s", s.Level, name)
	s.Refs[name] = id
	if load != nil {
		s.Loads[id] = *load
	}
	return id
}
