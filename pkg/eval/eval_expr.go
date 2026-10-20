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
		return runtime.Tuple(out), nil
	case *ast.List:
		// List literals are Python lists, with their mutating methods.
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
			if err := out.Put(k, v); err != nil {
				return nil, err
			}
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
		if err := FailStrict(v); err != nil {
			return nil, err
		}
		return escape.Markup(escape.SoftStr(v)), nil
	case *ast.MarkSafeIfAutoescape:
		v, err := e.evalExprAny(x.Expr, ctx)
		if err != nil {
			return nil, err
		}
		if ctx.Eval != nil && ctx.Eval.Autoescape {
			if err := FailStrict(v); err != nil {
				return nil, err
			}
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
	if n.Name == "self" {
		return &templateReference{ctx: ctx.Root(), undef: e.eng.UndefinedFactory()}
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
	if ref, ok := v.(*templateReference); ok {
		return ref.block(n.Attr), nil
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
	if ref, ok := v.(*templateReference); ok {
		if name, ok := arg.(string); ok {
			return ref.block(name), nil
		}
	}
	return e.eng.GetItem(v, arg)
}

func (e *evaluator) evalCall(n *ast.Call, ctx *runtime.Context) (any, error) {
	fn, err := e.evalExprAny(n.Node, ctx)
	if err != nil {
		return nil, err
	}
	args, kwargs, order, err := e.evalCallArgs(n.Args, n.Kwargs, n.DynArgs, n.DynKwargs, ctx)
	if err != nil {
		return nil, err
	}
	if mv, ok := fn.(*macroValue); ok {
		return e.CallMacro(mv, ctx, args, kwargs, order, nil)
	}
	if oc, ok := e.eng.(orderedCaller); ok {
		v, err := oc.CallOrdered(ctx, fn, args, kwargs, order)
		return runtime.ToPyList(v), err
	}
	v, err := e.eng.Call(ctx, fn, args, kwargs)
	return runtime.ToPyList(v), err
}

// evalCallArgs evaluates a call's arguments: the positional ones followed
// by the items of `*dynArgs`, the keywords merged with `**dynKwargs`. The
// last result is the keywords' order, which Python preserves.
func (e *evaluator) evalCallArgs(astArgs []ast.Expr, astKwargs []*ast.Keyword, dynArgs, dynKwargs ast.Expr, ctx *runtime.Context) ([]any, map[string]any, []string, error) {
	args := make([]any, 0, len(astArgs))
	for _, a := range astArgs {
		v, err := e.evalExprAny(a, ctx)
		if err != nil {
			return nil, nil, nil, err
		}
		args = append(args, v)
	}
	kwargs := make(map[string]any, len(astKwargs))
	order := make([]string, 0, len(astKwargs))
	for _, kw := range astKwargs {
		v, err := e.evalExprAny(kw.Value, ctx)
		if err != nil {
			return nil, nil, nil, err
		}
		kwargs[kw.Key] = v
		order = append(order, kw.Key)
	}
	if dynArgs != nil {
		v, err := e.evalExprAny(dynArgs, ctx)
		if err != nil {
			return nil, nil, nil, err
		}
		items, err := ToIterable(v)
		if err != nil {
			return nil, nil, nil, err
		}
		args = append(args, items...)
	}
	if dynKwargs != nil {
		v, err := e.evalExprAny(dynKwargs, ctx)
		if err != nil {
			return nil, nil, nil, err
		}
		m, ok := v.(*runtime.OrderedDict)
		if !ok {
			return nil, nil, nil, fmt.Errorf("argument after ** must be a mapping, not %s", runtime.PyTypeName(v))
		}
		for _, k := range m.Keys() {
			key, ok := k.(string)
			if !ok {
				return nil, nil, nil, fmt.Errorf("keywords must be strings")
			}
			if _, dup := kwargs[key]; dup {
				return nil, nil, nil, fmt.Errorf("got multiple values for keyword argument %s", escape.Repr(key))
			}
			kwargs[key], _ = m.Get(k)
			order = append(order, key)
		}
	}
	return args, kwargs, order, nil
}

func (e *evaluator) evalFilter(n *ast.Filter, ctx *runtime.Context) (any, error) {
	if n.Node == nil {
		return nil, fmt.Errorf("filter %q used outside filter block", n.Name)
	}
	value, err := e.evalExprAny(n.Node, ctx)
	if err != nil {
		return nil, err
	}
	args, kwargs, _, err := e.evalCallArgs(n.Args, n.Kwargs, n.DynArgs, n.DynKwargs, ctx)
	if err != nil {
		return nil, err
	}
	v, err := e.eng.CallFilter(n.Name, ctx, value, args, kwargs)
	return runtime.ToPyList(v), err
}

func (e *evaluator) evalTest(n *ast.Test, ctx *runtime.Context) (any, error) {
	value, err := e.evalExprAny(n.Node, ctx)
	if err != nil {
		return nil, err
	}
	args, kwargs, _, err := e.evalCallArgs(n.Args, n.Kwargs, n.DynArgs, n.DynKwargs, ctx)
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
	ok, err := truth(tv)
	if err != nil {
		return nil, err
	}
	if ok {
		return e.evalExprAny(n.Expr1, ctx)
	}
	if n.Expr2 != nil {
		return e.evalExprAny(n.Expr2, ctx)
	}
	// Python always uses the plain Undefined class for this implicit else.
	pos := fmt.Sprintf("line %d", n.Position().Lineno)
	if ctx.Name != "" {
		pos += " in " + escape.Repr(ctx.Name)
	}
	return runtime.NewBase("the inline if-expression on "+pos+" evaluated to false and no else section was defined.", "", runtime.Missing, nil), nil
}

// evalConcat implements `~`. Under autoescape it mirrors Python's
// markup_join: once any operand is Markup the result is Markup and every
// other operand is escaped.
func (e *evaluator) evalConcat(n *ast.Concat, ctx *runtime.Context) (any, error) {
	vals := make([]any, len(n.Nodes))
	markup := false
	for i, x := range n.Nodes {
		v, err := e.evalExprAny(x, ctx)
		if err != nil {
			return nil, err
		}
		if err := FailStrict(v); err != nil {
			return nil, err
		}
		if _, ok := v.(escape.Markup); ok {
			markup = true
		}
		vals[i] = v
	}
	markup = markup && ctx.Eval != nil && ctx.Eval.Autoescape
	var b []byte
	for _, v := range vals {
		if markup {
			b = append(b, escape.Escape(v)...)
		} else {
			b = append(b, escape.SoftStr(v)...)
		}
	}
	if markup {
		return escape.Markup(b), nil
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
		ok, err := truth(lv)
		if err != nil {
			return nil, err
		}
		if !ok {
			return lv, nil
		}
		return e.evalExprAny(x.Right, ctx)
	case *ast.Or:
		lv, err := e.evalExprAny(x.Left, ctx)
		if err != nil {
			return nil, err
		}
		ok, err := truth(lv)
		if err != nil {
			return nil, err
		}
		if ok {
			return lv, nil
		}
		return e.evalExprAny(x.Right, ctx)
	case *ast.Not:
		v, err := e.evalExprAny(x.Node, ctx)
		if err != nil {
			return nil, err
		}
		ok, err := truth(v)
		return !ok, err
	case *ast.Neg:
		v, err := e.evalExprAny(x.Node, ctx)
		if err != nil {
			return nil, err
		}
		return negate(v)
	case *ast.UAdd:
		v, err := e.evalExprAny(x.Node, ctx)
		if err != nil {
			return nil, err
		}
		return positive(v)
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
