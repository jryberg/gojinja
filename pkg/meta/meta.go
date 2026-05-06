// Package meta provides static-analysis helpers over a parsed template:
// FindUndeclaredVariables (names looked up but not assigned anywhere) and
// FindReferencedTemplates (names of templates referenced via extends /
// include / import / from-import). Mirrors jinja2.meta.
package meta

import (
	"github.com/jryberg/gojinja/pkg/ast"
)

// FindUndeclaredVariables returns the set of names the template will try
// to resolve from its render context (excluding names that are bound by
// `{% set %}`, `{% for %}`, `{% with %}`, or macro/call args).
//
// The returned slice is sorted for determinism; callers can convert to a
// set if they want O(1) membership.
func FindUndeclaredVariables(t *ast.Template) []string {
	if t == nil {
		return nil
	}
	state := &declState{
		referenced: map[string]bool{},
		bound:      map[string]bool{},
	}
	for _, n := range t.Body {
		walkDecl(n, state)
	}
	out := make([]string, 0, len(state.referenced))
	for k := range state.referenced {
		if !state.bound[k] {
			out = append(out, k)
		}
	}
	sortStrings(out)
	return out
}

type declState struct {
	referenced map[string]bool
	bound      map[string]bool
}

func walkDecl(n ast.Node, s *declState) {
	switch x := n.(type) {
	case nil:
		return
	case *ast.Name:
		// A Name with ctx==load reads; ctx==store/param binds.
		switch x.Ctx {
		case ast.CtxStore, ast.CtxParam:
			s.bound[x.Name] = true
		default:
			s.referenced[x.Name] = true
		}
		return
	case *ast.NSRef:
		s.referenced[x.Name] = true
		return
	case *ast.Macro:
		s.bound[x.Name] = true
		for _, a := range x.Args {
			s.bound[a.Name] = true
		}
		for _, d := range x.Defaults {
			walkDecl(d, s)
		}
		for _, b := range x.Body {
			walkDecl(b, s)
		}
		return
	case *ast.For:
		walkDecl(x.Iter, s)
		// `target` introduces bindings.
		bindAssignTarget(x.Target, s)
		for _, b := range x.Body {
			walkDecl(b, s)
		}
		for _, e := range x.Else {
			walkDecl(e, s)
		}
		if x.Test != nil {
			walkDecl(x.Test, s)
		}
		return
	case *ast.With:
		for _, t := range x.Targets {
			bindAssignTarget(t, s)
		}
		for _, v := range x.Values {
			walkDecl(v, s)
		}
		for _, b := range x.Body {
			walkDecl(b, s)
		}
		return
	case *ast.Assign:
		walkDecl(x.Node, s)
		bindAssignTarget(x.Target, s)
		return
	case *ast.AssignBlock:
		bindAssignTarget(x.Target, s)
		for _, b := range x.Body {
			walkDecl(b, s)
		}
		return
	case *ast.FromImport:
		walkDecl(x.Template, s)
		for _, en := range x.Names {
			alias := en.Alias
			if alias == "" {
				alias = en.Name
			}
			s.bound[alias] = true
		}
		return
	case *ast.Import:
		walkDecl(x.Template, s)
		s.bound[x.Target] = true
		return
	}
	// Default: recurse into children.
	for _, c := range ast.Children(n) {
		walkDecl(c, s)
	}
}

// bindAssignTarget marks names introduced by an assignment target.
func bindAssignTarget(target ast.Node, s *declState) {
	switch t := target.(type) {
	case *ast.Name:
		s.bound[t.Name] = true
	case *ast.Tuple:
		for _, it := range t.Items {
			bindAssignTarget(it, s)
		}
	case *ast.NSRef:
		// Target's namespace name was bound elsewhere (by a regular `set`).
		s.referenced[t.Name] = true
	}
}

// FindReferencedTemplates returns the names of templates the template
// references via {% extends %}, {% include %}, {% import %}, {% from %}.
// A nil entry indicates a dynamic (non-constant) reference.
func FindReferencedTemplates(t *ast.Template) []*string {
	if t == nil {
		return nil
	}
	var out []*string
	collect := func(e ast.Expr) {
		if e == nil {
			out = append(out, nil)
			return
		}
		switch v := e.(type) {
		case *ast.Const:
			if s, ok := v.Value.(string); ok {
				out = append(out, &s)
				return
			}
			if list, ok := v.Value.([]any); ok {
				for _, it := range list {
					if s, ok := it.(string); ok {
						copy := s
						out = append(out, &copy)
					} else {
						out = append(out, nil)
					}
				}
				return
			}
			out = append(out, nil)
		case *ast.List:
			for _, it := range v.Items {
				if c, ok := it.(*ast.Const); ok {
					if s, ok := c.Value.(string); ok {
						copy := s
						out = append(out, &copy)
						continue
					}
				}
				out = append(out, nil)
			}
		default:
			out = append(out, nil)
		}
	}
	var walk func(n ast.Node)
	walk = func(n ast.Node) {
		switch x := n.(type) {
		case *ast.Extends:
			collect(x.Template)
		case *ast.Include:
			collect(x.Template)
		case *ast.Import:
			collect(x.Template)
		case *ast.FromImport:
			collect(x.Template)
		}
		for _, c := range ast.Children(n) {
			walk(c)
		}
	}
	for _, n := range t.Body {
		walk(n)
	}
	return out
}

// sortStrings is a tiny in-place insertion sort to keep deps light.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}
