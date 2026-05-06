package idtracking

import (
	"fmt"

	"github.com/jryberg/gojinja/pkg/ast"
)

// RootVisitor seeds analysis at the root of a frame. It mirrors Python's
// RootVisitor: only a small number of node types start a frame; everything
// else is the body of the same frame and is reached via the
// FrameSymbolVisitor.
type RootVisitor struct {
	frame     *FrameSymbolVisitor
	forBranch string
}

// Visit dispatches `n` to the appropriate frame-starting handler.
func (r *RootVisitor) Visit(n ast.Node) {
	switch x := n.(type) {
	case *ast.Template, *ast.Block, *ast.Macro, *ast.FilterBlock,
		*ast.Scope, *ast.If, *ast.ScopedEvalContextModifier:
		for _, c := range ast.Children(n) {
			r.frame.Visit(c, false)
		}
	case *ast.AssignBlock:
		for _, c := range x.Body {
			r.frame.Visit(c, false)
		}
	case *ast.CallBlock:
		// All children except `Call` (the call expression).
		for _, a := range x.Args {
			r.frame.Visit(a, false)
		}
		for _, d := range x.Defaults {
			r.frame.Visit(d, false)
		}
		for _, b := range x.Body {
			r.frame.Visit(b, false)
		}
	case *ast.OverlayScope:
		for _, b := range x.Body {
			r.frame.Visit(b, false)
		}
	case *ast.For:
		r.visitFor(x)
	case *ast.With:
		for _, t := range x.Targets {
			r.frame.Visit(t, false)
		}
		for _, b := range x.Body {
			r.frame.Visit(b, false)
		}
	default:
		panic(fmt.Sprintf("idtracking: cannot find symbols for %T", n))
	}
}

func (r *RootVisitor) visitFor(node *ast.For) {
	switch r.forBranch {
	case "body":
		r.frame.Visit(node.Target, true)
		for _, b := range node.Body {
			r.frame.Visit(b, false)
		}
	case "else":
		for _, b := range node.Else {
			r.frame.Visit(b, false)
		}
	case "test":
		r.frame.Visit(node.Target, true)
		if node.Test != nil {
			r.frame.Visit(node.Test, false)
		}
	default:
		panic(fmt.Sprintf("idtracking: unknown for branch %q", r.forBranch))
	}
}

// FrameSymbolVisitor walks expression and statement bodies inside a single
// frame, recording loads/stores it discovers. It stops at nodes that begin
// new frames (Block, Scope, OverlayScope, For body, AssignBlock body, etc.).
type FrameSymbolVisitor struct {
	Symbols *Symbols
}

// Visit dispatches the node to the right handler. `storeAsParam` overrides
// any [*ast.Name] reached during this walk to be treated as a parameter
// declaration; it propagates through generic_visit and through a few
// container nodes like Tuple.
func (v *FrameSymbolVisitor) Visit(n ast.Node, storeAsParam bool) {
	if n == nil {
		return
	}
	switch x := n.(type) {
	case *ast.Name:
		v.visitName(x, storeAsParam)
	case *ast.NSRef:
		v.Symbols.Load(x.Name)
	case *ast.If:
		v.visitIf(x, storeAsParam)
	case *ast.Macro:
		v.Symbols.Store(x.Name)
	case *ast.Import:
		v.genericVisit(x, storeAsParam)
		v.Symbols.Store(x.Target)
	case *ast.FromImport:
		v.genericVisit(x, storeAsParam)
		for _, name := range x.Names {
			if name.Alias != "" {
				v.Symbols.Store(name.Alias)
			} else {
				v.Symbols.Store(name.Name)
			}
		}
	case *ast.Assign:
		v.Visit(x.Node, storeAsParam)
		v.Visit(x.Target, storeAsParam)
	case *ast.For:
		// The body is a separate frame; only the iterable is part of the
		// current scope.
		v.Visit(x.Iter, storeAsParam)
	case *ast.CallBlock:
		v.Visit(x.Call, storeAsParam)
	case *ast.FilterBlock:
		v.Visit(x.Filter, storeAsParam)
	case *ast.With:
		for _, value := range x.Values {
			v.Visit(value, storeAsParam)
		}
	case *ast.AssignBlock:
		v.Visit(x.Target, storeAsParam)
	case *ast.Scope:
		// Scopes start their own frame.
	case *ast.Block:
		// Blocks start their own frame.
	case *ast.OverlayScope:
		// Overlay scopes start their own frame.
	default:
		v.genericVisit(n, storeAsParam)
	}
}

func (v *FrameSymbolVisitor) visitName(n *ast.Name, storeAsParam bool) {
	switch {
	case storeAsParam || n.Ctx == ast.CtxParam:
		v.Symbols.DeclareParameter(n.Name)
	case n.Ctx == ast.CtxStore:
		v.Symbols.Store(n.Name)
	case n.Ctx == ast.CtxLoad:
		v.Symbols.Load(n.Name)
	}
}

func (v *FrameSymbolVisitor) visitIf(node *ast.If, storeAsParam bool) {
	v.Visit(node.Test, storeAsParam)
	original := v.Symbols

	innerVisit := func(stmts []ast.Node) *Symbols {
		v.Symbols = original.Copy()
		rv := v.Symbols
		for _, n := range stmts {
			v.Visit(n, storeAsParam)
		}
		v.Symbols = original
		return rv
	}

	bodySyms := innerVisit(node.Body)

	elifNodes := make([]ast.Node, 0, len(node.Elif))
	for _, e := range node.Elif {
		elifNodes = append(elifNodes, e)
	}
	elifSyms := innerVisit(elifNodes)

	elseSyms := innerVisit(node.Else)

	v.Symbols.BranchUpdate([]*Symbols{bodySyms, elifSyms, elseSyms})
}

func (v *FrameSymbolVisitor) genericVisit(n ast.Node, storeAsParam bool) {
	for _, c := range ast.Children(n) {
		v.Visit(c, storeAsParam)
	}
}
