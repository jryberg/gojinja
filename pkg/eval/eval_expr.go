package eval

import (
	"fmt"

	"github.com/jryberg/gojinja/pkg/ast"
	"github.com/jryberg/gojinja/pkg/escape"
	"github.com/jryberg/gojinja/pkg/runtime"
)

// evalExpr dispatches expression evaluation. Returns an [ast.Expr]-typed
// value when the caller wants to keep flowing through expression nodes,
// but most callers want [evalExprAny] which returns the underlying Go
// value.
func (e *evaluator) evalExpr(n ast.Expr, ctx *runtime.Context) (any, error) {
	return e.evalExprAny(n, ctx)
}

// evalExprAny evaluates an expression and returns the resulting Go value.
func (e *evaluator) evalExprAny(n ast.Expr, ctx *runtime.Context) (any, error) {
	switch x := n.(type) {
	case *ast.Const:
		return x.Value, nil
	case *ast.TemplateData:
		if ctx.Eval != nil && ctx.Eval.Autoescape {
			return escape.Markup(x.Data), nil
		}
		return x.Data, nil
	case *ast.Name:
		return e.evalName(x, ctx), nil
	case *ast.Tuple:
		out := make([]any, 0, len(x.Items))
		for _, it := range x.Items {
			v, err := e.evalExprAny(it, ctx)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, nil
	case *ast.List:
		// In-template list literals produce a *runtime.PyList so they
		// expose Python-parity mutating methods (append, extend, etc.)
		// when the `do` extension is used. Caller-supplied []any from
		// the render context keeps its bare type and stays immutable —
		// the sandbox boundary is "values created inside the template
		// can be mutated; host data cannot".
		out := make([]any, 0, len(x.Items))
		for _, it := range x.Items {
			v, err := e.evalExprAny(it, ctx)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return runtime.NewPyList(out), nil
	case *ast.Dict:
		// Insertion-order parity with Python 3.7+: dict literals built
		// in templates iterate in declaration order. Plain map[any]any
		// loses that, so we use runtime.OrderedDict.
		out := runtime.NewOrderedDict()
		for _, p := range x.Items {
			k, err := e.evalExprAny(p.Key, ctx)
			if err != nil {
				return nil, err
			}
			v, err := e.evalExprAny(p.Value, ctx)
			if err != nil {
				return nil, err
			}
			out.Set(k, v)
		}
		return out, nil
	case *ast.Getattr:
		return e.evalGetattr(x, ctx)
	case *ast.Getitem:
		return e.evalGetitem(x, ctx)
	case *ast.Call:
		return e.evalCall(x, ctx)
	case *ast.Filter:
		return e.evalFilter(x, ctx)
	case *ast.Test:
		return e.evalTest(x, ctx)
	case *ast.CondExpr:
		return e.evalCondExpr(x, ctx)
	case *ast.Concat:
		return e.evalConcat(x, ctx)
	case *ast.Compare:
		return e.evalCompare(x, ctx)
	case *ast.Slice:
		return e.evalSlice(x, ctx)
	case *ast.MarkSafe:
		v, err := e.evalExprAny(x.Expr, ctx)
		if err != nil {
			return nil, err
		}
		return escape.Markup(escape.SoftStr(v)), nil
	case *ast.MarkSafeIfAutoescape:
		v, err := e.evalExprAny(x.Expr, ctx)
		if err != nil {
			return nil, err
		}
		if ctx.Eval != nil && ctx.Eval.Autoescape {
			return escape.Markup(escape.SoftStr(v)), nil
		}
		return v, nil
	case *ast.NSRef:
		return e.evalNSRef(x, ctx)
	case *ast.ContextReference:
		return ctx, nil
	case *ast.DerivedContextReference:
		return ctx, nil
	}
	// Binary/unary ops via fold helpers — these embed ast.binexprBase /
	// ast.unaryexprBase so we use the AST's AsConst on synthetic Const
	// inputs once we've folded operands.
	return e.evalBinUnaryOp(n, ctx)
}

// evalName resolves a name from the context, falling back to Undefined.
func (e *evaluator) evalName(n *ast.Name, ctx *runtime.Context) any {
	if v, ok := ctx.Resolve(n.Name); ok {
		return v
	}
	return ctx.ResolveOrUndefined(n.Name, e.eng.UndefinedFactory())
}

func (e *evaluator) evalNSRef(n *ast.NSRef, ctx *runtime.Context) (any, error) {
	v, ok := ctx.Resolve(n.Name)
	if !ok {
		return nil, fmt.Errorf("namespace %q is not defined", n.Name)
	}
	return e.eng.GetAttr(v, n.Attr)
}

func (e *evaluator) evalGetattr(n *ast.Getattr, ctx *runtime.Context) (any, error) {
	v, err := e.evalExprAny(n.Node, ctx)
	if err != nil {
		return nil, err
	}
	return e.eng.GetAttr(v, n.Attr)
}

func (e *evaluator) evalGetitem(n *ast.Getitem, ctx *runtime.Context) (any, error) {
	v, err := e.evalExprAny(n.Node, ctx)
	if err != nil {
		return nil, err
	}
	arg, err := e.evalExprAny(n.Arg, ctx)
	if err != nil {
		return nil, err
	}
	return e.eng.GetItem(v, arg)
}

func (e *evaluator) evalCall(n *ast.Call, ctx *runtime.Context) (any, error) {
	fn, err := e.evalExprAny(n.Node, ctx)
	if err != nil {
		return nil, err
	}
	args, kwargs, err := e.evalCallArgs(n.Args, n.Kwargs, ctx)
	if err != nil {
		return nil, err
	}
	if mv, ok := fn.(*macroValue); ok {
		return e.CallMacro(mv, ctx, args, kwargs, nil)
	}
	return e.eng.Call(ctx, fn, args, kwargs)
}

func (e *evaluator) evalCallArgs(astArgs []ast.Expr, astKwargs []*ast.Keyword, ctx *runtime.Context) ([]any, map[string]any, error) {
	args := make([]any, 0, len(astArgs))
	for _, a := range astArgs {
		v, err := e.evalExprAny(a, ctx)
		if err != nil {
			return nil, nil, err
		}
		args = append(args, v)
	}
	kwargs := make(map[string]any, len(astKwargs))
	for _, kw := range astKwargs {
		v, err := e.evalExprAny(kw.Value, ctx)
		if err != nil {
			return nil, nil, err
		}
		kwargs[kw.Key] = v
	}
	return args, kwargs, nil
}

func (e *evaluator) evalFilter(n *ast.Filter, ctx *runtime.Context) (any, error) {
	if n.Node == nil {
		return nil, fmt.Errorf("filter %q used outside filter block", n.Name)
	}
	value, err := e.evalExprAny(n.Node, ctx)
	if err != nil {
		return nil, err
	}
	args, kwargs, err := e.evalCallArgs(n.Args, n.Kwargs, ctx)
	if err != nil {
		return nil, err
	}
	return e.eng.CallFilter(n.Name, ctx, value, args, kwargs)
}

func (e *evaluator) evalTest(n *ast.Test, ctx *runtime.Context) (any, error) {
	value, err := e.evalExprAny(n.Node, ctx)
	if err != nil {
		return nil, err
	}
	args, kwargs, err := e.evalCallArgs(n.Args, n.Kwargs, ctx)
	if err != nil {
		return nil, err
	}
	return e.eng.CallTest(n.Name, ctx, value, args, kwargs)
}

func (e *evaluator) evalCondExpr(n *ast.CondExpr, ctx *runtime.Context) (any, error) {
	tv, err := e.evalExprAny(n.Test, ctx)
	if err != nil {
		return nil, err
	}
	if truthy(tv) {
		return e.evalExprAny(n.Expr1, ctx)
	}
	if n.Expr2 != nil {
		return e.evalExprAny(n.Expr2, ctx)
	}
	return e.eng.UndefinedFactory()("", "", nil, nil), nil
}

func (e *evaluator) evalConcat(n *ast.Concat, ctx *runtime.Context) (any, error) {
	var b []byte
	for _, x := range n.Nodes {
		v, err := e.evalExprAny(x, ctx)
		if err != nil {
			return nil, err
		}
		b = append(b, escape.SoftStr(v)...)
	}
	return string(b), nil
}

func (e *evaluator) evalCompare(n *ast.Compare, ctx *runtime.Context) (any, error) {
	left, err := e.evalExprAny(n.Expr, ctx)
	if err != nil {
		return nil, err
	}
	for _, op := range n.Ops {
		right, err := e.evalExprAny(op.Expr, ctx)
		if err != nil {
			return nil, err
		}
		ok, err := compareValues(op.Op, left, right)
		if err != nil {
			return nil, err
		}
		if !ok {
			return false, nil
		}
		left = right
	}
	return true, nil
}

func (e *evaluator) evalSlice(n *ast.Slice, ctx *runtime.Context) (any, error) {
	val := func(x ast.Expr) (any, error) {
		if x == nil {
			return nil, nil
		}
		return e.evalExprAny(x, ctx)
	}
	start, err := val(n.Start)
	if err != nil {
		return nil, err
	}
	stop, err := val(n.Stop)
	if err != nil {
		return nil, err
	}
	step, err := val(n.Step)
	if err != nil {
		return nil, err
	}
	return ast.SliceVal{Start: start, Stop: stop, Step: step}, nil
}

// evalBinUnaryOp delegates to the AST's AsConst by folding both operands
// to constants. Operands always fold first because we just evaluated them
// — so AsConst on a node whose children are wrapped in synthetic Consts
// will succeed where it can. For the not-foldable case (e.g. string concat
// rules), we replicate the logic here.
func (e *evaluator) evalBinUnaryOp(n ast.Expr, ctx *runtime.Context) (any, error) {
	switch x := n.(type) {
	case *ast.Add:
		return e.binop("+", x.Left, x.Right, ctx)
	case *ast.Sub:
		return e.binop("-", x.Left, x.Right, ctx)
	case *ast.Mul:
		return e.binop("*", x.Left, x.Right, ctx)
	case *ast.Div:
		return e.binop("/", x.Left, x.Right, ctx)
	case *ast.FloorDiv:
		return e.binop("//", x.Left, x.Right, ctx)
	case *ast.Mod:
		return e.binop("%", x.Left, x.Right, ctx)
	case *ast.Pow:
		return e.binop("**", x.Left, x.Right, ctx)
	case *ast.And:
		lv, err := e.evalExprAny(x.Left, ctx)
		if err != nil {
			return nil, err
		}
		if !truthy(lv) {
			return lv, nil
		}
		return e.evalExprAny(x.Right, ctx)
	case *ast.Or:
		lv, err := e.evalExprAny(x.Left, ctx)
		if err != nil {
			return nil, err
		}
		if truthy(lv) {
			return lv, nil
		}
		return e.evalExprAny(x.Right, ctx)
	case *ast.Not:
		v, err := e.evalExprAny(x.Node, ctx)
		if err != nil {
			return nil, err
		}
		return !truthy(v), nil
	case *ast.Neg:
		v, err := e.evalExprAny(x.Node, ctx)
		if err != nil {
			return nil, err
		}
		return negate(v)
	case *ast.UAdd:
		return e.evalExprAny(x.Node, ctx)
	}
	return nil, fmt.Errorf("eval: unsupported expression %T", n)
}

func (e *evaluator) binop(op string, left, right ast.Expr, ctx *runtime.Context) (any, error) {
	lv, err := e.evalExprAny(left, ctx)
	if err != nil {
		return nil, err
	}
	rv, err := e.evalExprAny(right, ctx)
	if err != nil {
		return nil, err
	}
	return applyBinop(op, lv, rv)
}
