package optimizer

import (
	"github.com/jryberg/gojinja/pkg/ast"
)

// transformChildren walks each direct child of n, recursively optimises it,
// and writes the result back into the parent. Mirrors NodeTransformer.
func transformChildren(n ast.Node, ctx ast.EvalCtx) {
	switch x := n.(type) {
	case *ast.Template:
		transformStmtList(&x.Body, ctx)
	case *ast.Output:
		transformExprList(&x.Nodes, ctx)
	case *ast.Extends:
		x.Template = transformExpr(x.Template, ctx)
	case *ast.For:
		x.Target = transformAny(x.Target, ctx)
		x.Iter = transformAny(x.Iter, ctx)
		transformStmtList(&x.Body, ctx)
		transformStmtList(&x.Else, ctx)
		x.Test = transformAny(x.Test, ctx)
	case *ast.If:
		x.Test = transformAny(x.Test, ctx)
		transformStmtList(&x.Body, ctx)
		for _, e := range x.Elif {
			// Elif entries are themselves *If nodes; recurse via Optimize
			// so they receive the same treatment.
			transformChildren(e, ctx)
		}
		transformStmtList(&x.Else, ctx)
	case *ast.Macro:
		for i := range x.Args {
			// Names aren't foldable; only fold defaults.
			_ = i
		}
		transformExprList(&x.Defaults, ctx)
		transformStmtList(&x.Body, ctx)
	case *ast.CallBlock:
		if x.Call != nil {
			transformChildren(x.Call, ctx)
		}
		transformExprList(&x.Defaults, ctx)
		transformStmtList(&x.Body, ctx)
	case *ast.FilterBlock:
		transformStmtList(&x.Body, ctx)
		if x.Filter != nil {
			transformChildren(x.Filter, ctx)
		}
	case *ast.With:
		transformExprList(&x.Targets, ctx)
		transformExprList(&x.Values, ctx)
		transformStmtList(&x.Body, ctx)
	case *ast.Block:
		transformStmtList(&x.Body, ctx)
	case *ast.Include:
		x.Template = transformExpr(x.Template, ctx)
	case *ast.Import:
		x.Template = transformExpr(x.Template, ctx)
	case *ast.FromImport:
		x.Template = transformExpr(x.Template, ctx)
	case *ast.ExprStmt:
		x.Node = transformAny(x.Node, ctx)
	case *ast.Assign:
		x.Target = transformExpr(x.Target, ctx)
		x.Node = transformAny(x.Node, ctx)
	case *ast.AssignBlock:
		x.Target = transformExpr(x.Target, ctx)
		if x.Filter != nil {
			transformChildren(x.Filter, ctx)
		}
		transformStmtList(&x.Body, ctx)
	case *ast.Scope:
		transformStmtList(&x.Body, ctx)
	case *ast.OverlayScope:
		x.Context = transformExpr(x.Context, ctx)
		transformStmtList(&x.Body, ctx)
	case *ast.EvalContextModifier:
		for _, k := range x.Options {
			k.Value = transformExpr(k.Value, ctx)
		}
	case *ast.ScopedEvalContextModifier:
		for _, k := range x.Options {
			k.Value = transformExpr(k.Value, ctx)
		}
		transformStmtList(&x.Body, ctx)
	case *ast.Tuple:
		transformExprList(&x.Items, ctx)
	case *ast.List:
		transformExprList(&x.Items, ctx)
	case *ast.Dict:
		for _, p := range x.Items {
			p.Key = transformExpr(p.Key, ctx)
			p.Value = transformExpr(p.Value, ctx)
		}
	case *ast.CondExpr:
		x.Test = transformExpr(x.Test, ctx)
		x.Expr1 = transformExpr(x.Expr1, ctx)
		if x.Expr2 != nil {
			x.Expr2 = transformExpr(x.Expr2, ctx)
		}
	case *ast.Filter:
		if x.Node != nil {
			x.Node = transformExpr(x.Node, ctx)
		}
		transformExprList(&x.Args, ctx)
		for _, k := range x.Kwargs {
			k.Value = transformExpr(k.Value, ctx)
		}
		if x.DynArgs != nil {
			x.DynArgs = transformExpr(x.DynArgs, ctx)
		}
		if x.DynKwargs != nil {
			x.DynKwargs = transformExpr(x.DynKwargs, ctx)
		}
	case *ast.Test:
		x.Node = transformExpr(x.Node, ctx)
		transformExprList(&x.Args, ctx)
		for _, k := range x.Kwargs {
			k.Value = transformExpr(k.Value, ctx)
		}
		if x.DynArgs != nil {
			x.DynArgs = transformExpr(x.DynArgs, ctx)
		}
		if x.DynKwargs != nil {
			x.DynKwargs = transformExpr(x.DynKwargs, ctx)
		}
	case *ast.Call:
		x.Node = transformExpr(x.Node, ctx)
		transformExprList(&x.Args, ctx)
		for _, k := range x.Kwargs {
			k.Value = transformExpr(k.Value, ctx)
		}
		if x.DynArgs != nil {
			x.DynArgs = transformExpr(x.DynArgs, ctx)
		}
		if x.DynKwargs != nil {
			x.DynKwargs = transformExpr(x.DynKwargs, ctx)
		}
	case *ast.Getitem:
		x.Node = transformExpr(x.Node, ctx)
		x.Arg = transformExpr(x.Arg, ctx)
	case *ast.Getattr:
		x.Node = transformExpr(x.Node, ctx)
	case *ast.Slice:
		if x.Start != nil {
			x.Start = transformExpr(x.Start, ctx)
		}
		if x.Stop != nil {
			x.Stop = transformExpr(x.Stop, ctx)
		}
		if x.Step != nil {
			x.Step = transformExpr(x.Step, ctx)
		}
	case *ast.Concat:
		transformExprList(&x.Nodes, ctx)
	case *ast.Compare:
		x.Expr = transformExpr(x.Expr, ctx)
		for _, op := range x.Ops {
			op.Expr = transformExpr(op.Expr, ctx)
		}
	case *ast.MarkSafe:
		x.Expr = transformExpr(x.Expr, ctx)
	case *ast.MarkSafeIfAutoescape:
		x.Expr = transformExpr(x.Expr, ctx)
	case *ast.Add:
		x.Left = transformExpr(x.Left, ctx)
		x.Right = transformExpr(x.Right, ctx)
	case *ast.Sub:
		x.Left = transformExpr(x.Left, ctx)
		x.Right = transformExpr(x.Right, ctx)
	case *ast.Mul:
		x.Left = transformExpr(x.Left, ctx)
		x.Right = transformExpr(x.Right, ctx)
	case *ast.Div:
		x.Left = transformExpr(x.Left, ctx)
		x.Right = transformExpr(x.Right, ctx)
	case *ast.FloorDiv:
		x.Left = transformExpr(x.Left, ctx)
		x.Right = transformExpr(x.Right, ctx)
	case *ast.Mod:
		x.Left = transformExpr(x.Left, ctx)
		x.Right = transformExpr(x.Right, ctx)
	case *ast.Pow:
		x.Left = transformExpr(x.Left, ctx)
		x.Right = transformExpr(x.Right, ctx)
	case *ast.And:
		x.Left = transformExpr(x.Left, ctx)
		x.Right = transformExpr(x.Right, ctx)
	case *ast.Or:
		x.Left = transformExpr(x.Left, ctx)
		x.Right = transformExpr(x.Right, ctx)
	case *ast.Not:
		x.Node = transformExpr(x.Node, ctx)
	case *ast.Neg:
		x.Node = transformExpr(x.Node, ctx)
	case *ast.UAdd:
		x.Node = transformExpr(x.Node, ctx)
	}
	// Leaf nodes (Const, TemplateData, Name, ...) have no children.
}

// transformAny optimises any non-nil Node, returning the (possibly
// replaced) result.
func transformAny(n ast.Node, ctx ast.EvalCtx) ast.Node {
	if n == nil {
		return nil
	}
	return Optimize(n, ctx)
}

// transformExpr optimises an Expr slot, ensuring the result keeps the
// Expr static type (the optimizer never produces a non-Expr replacement
// for an Expr child).
func transformExpr(e ast.Expr, ctx ast.EvalCtx) ast.Expr {
	if e == nil {
		return nil
	}
	r := Optimize(e, ctx)
	if r == nil {
		return nil
	}
	return r.(ast.Expr)
}

func transformStmtList(list *[]ast.Node, ctx ast.EvalCtx) {
	for i, n := range *list {
		if n == nil {
			continue
		}
		(*list)[i] = Optimize(n, ctx)
	}
}

func transformExprList(list *[]ast.Expr, ctx ast.EvalCtx) {
	for i, e := range *list {
		if e == nil {
			continue
		}
		(*list)[i] = transformExpr(e, ctx)
	}
}
