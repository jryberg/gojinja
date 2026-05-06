package ast

// Visitor is the contract for tree walkers. Visit is called for every
// node encountered. The return value tells the walker whether to descend
// into the node's children.
type Visitor interface {
	Visit(n Node) (descend bool)
}

// VisitorFunc adapts a plain function to [Visitor].
type VisitorFunc func(Node) bool

// Visit forwards to the underlying function.
func (f VisitorFunc) Visit(n Node) bool { return f(n) }

// Walk visits every node in the subtree rooted at n in depth-first
// pre-order. The walker descends only when the visitor returns true.
func Walk(n Node, v Visitor) {
	if n == nil {
		return
	}
	if !v.Visit(n) {
		return
	}
	for _, c := range Children(n) {
		Walk(c, v)
	}
}

// Children returns every direct child node of n. The result is appended
// in source-order so [Walk] visits the AST in a sensible order.
func Children(n Node) []Node {
	switch x := n.(type) {
	case *Template:
		return appendList(nil, x.Body)
	case *Output:
		return appendExprs(nil, x.Nodes)
	case *Extends:
		return appendIfNotNil(nil, x.Template)
	case *For:
		out := []Node{x.Target, x.Iter}
		out = appendList(out, x.Body)
		out = appendList(out, x.Else)
		out = appendIfNotNil(out, x.Test)
		return out
	case *If:
		out := []Node{x.Test}
		out = appendList(out, x.Body)
		for _, e := range x.Elif {
			out = append(out, e)
		}
		out = appendList(out, x.Else)
		return out
	case *Macro:
		out := []Node{}
		for _, a := range x.Args {
			out = append(out, a)
		}
		for _, d := range x.Defaults {
			out = append(out, d)
		}
		out = appendList(out, x.Body)
		return out
	case *CallBlock:
		out := []Node{x.Call}
		for _, a := range x.Args {
			out = append(out, a)
		}
		for _, d := range x.Defaults {
			out = append(out, d)
		}
		out = appendList(out, x.Body)
		return out
	case *FilterBlock:
		out := appendList(nil, x.Body)
		if x.Filter != nil {
			out = append(out, x.Filter)
		}
		return out
	case *With:
		out := []Node{}
		for _, t := range x.Targets {
			out = append(out, t)
		}
		for _, v := range x.Values {
			out = append(out, v)
		}
		out = appendList(out, x.Body)
		return out
	case *Block:
		return appendList(nil, x.Body)
	case *Include:
		return appendIfNotNil(nil, x.Template)
	case *Import:
		return appendIfNotNil(nil, x.Template)
	case *FromImport:
		return appendIfNotNil(nil, x.Template)
	case *ExprStmt:
		return appendIfNotNil(nil, x.Node)
	case *Assign:
		return []Node{x.Target, x.Node}
	case *AssignBlock:
		out := []Node{x.Target}
		if x.Filter != nil {
			out = append(out, x.Filter)
		}
		out = appendList(out, x.Body)
		return out
	case *Scope:
		return appendList(nil, x.Body)
	case *OverlayScope:
		out := []Node{x.Context}
		return appendList(out, x.Body)
	case *EvalContextModifier:
		out := []Node{}
		for _, k := range x.Options {
			out = append(out, k)
		}
		return out
	case *ScopedEvalContextModifier:
		out := []Node{}
		for _, k := range x.Options {
			out = append(out, k)
		}
		out = appendList(out, x.Body)
		return out
	case *Tuple:
		return appendExprs(nil, x.Items)
	case *List:
		return appendExprs(nil, x.Items)
	case *Dict:
		out := []Node{}
		for _, p := range x.Items {
			out = append(out, p)
		}
		return out
	case *Pair:
		return []Node{x.Key, x.Value}
	case *Keyword:
		return []Node{x.Value}
	case *CondExpr:
		out := []Node{x.Test, x.Expr1}
		out = appendIfNotNil(out, x.Expr2)
		return out
	case *Filter:
		out := []Node{}
		if x.Node != nil {
			out = append(out, x.Node)
		}
		out = appendExprs(out, x.Args)
		for _, k := range x.Kwargs {
			out = append(out, k)
		}
		out = appendIfNotNil(out, x.DynArgs)
		out = appendIfNotNil(out, x.DynKwargs)
		return out
	case *Test:
		out := []Node{x.Node}
		out = appendExprs(out, x.Args)
		for _, k := range x.Kwargs {
			out = append(out, k)
		}
		out = appendIfNotNil(out, x.DynArgs)
		out = appendIfNotNil(out, x.DynKwargs)
		return out
	case *Call:
		out := []Node{x.Node}
		out = appendExprs(out, x.Args)
		for _, k := range x.Kwargs {
			out = append(out, k)
		}
		out = appendIfNotNil(out, x.DynArgs)
		out = appendIfNotNil(out, x.DynKwargs)
		return out
	case *Getitem:
		return []Node{x.Node, x.Arg}
	case *Getattr:
		return []Node{x.Node}
	case *Slice:
		out := []Node{}
		out = appendIfNotNil(out, x.Start)
		out = appendIfNotNil(out, x.Stop)
		out = appendIfNotNil(out, x.Step)
		return out
	case *Concat:
		return appendExprs(nil, x.Nodes)
	case *Compare:
		out := []Node{x.Expr}
		for _, op := range x.Ops {
			out = append(out, op)
		}
		return out
	case *Operand:
		return []Node{x.Expr}
	case *Add:
		return []Node{x.Left, x.Right}
	case *Sub:
		return []Node{x.Left, x.Right}
	case *Mul:
		return []Node{x.Left, x.Right}
	case *Div:
		return []Node{x.Left, x.Right}
	case *FloorDiv:
		return []Node{x.Left, x.Right}
	case *Mod:
		return []Node{x.Left, x.Right}
	case *Pow:
		return []Node{x.Left, x.Right}
	case *And:
		return []Node{x.Left, x.Right}
	case *Or:
		return []Node{x.Left, x.Right}
	case *Not:
		return []Node{x.Node}
	case *Neg:
		return []Node{x.Node}
	case *UAdd:
		return []Node{x.Node}
	case *MarkSafe:
		return []Node{x.Expr}
	case *MarkSafeIfAutoescape:
		return []Node{x.Expr}
	}
	// Leaf nodes have no children: Const, TemplateData, Name, NSRef,
	// EnvironmentAttribute, ExtensionAttribute, ImportedName, InternalName,
	// ContextReference, DerivedContextReference, Continue, Break.
	return nil
}

func appendList(dst []Node, src []Node) []Node {
	for _, n := range src {
		if n != nil {
			dst = append(dst, n)
		}
	}
	return dst
}

func appendExprs(dst []Node, src []Expr) []Node {
	for _, n := range src {
		if n != nil {
			dst = append(dst, n)
		}
	}
	return dst
}

func appendIfNotNil(dst []Node, n Node) []Node {
	if n != nil {
		return append(dst, n)
	}
	return dst
}
