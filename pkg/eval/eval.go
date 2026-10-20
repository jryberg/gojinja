// Package eval is gojinja's tree-walking evaluator. It walks an
// [ast.Template] against a [runtime.Context] and emits output through an
// [io.Writer]. There is no separate compile step — the parser's AST is
// evaluated directly. (Phase 7's optimizer pre-folds where possible; the
// cache stores the lowered AST.)
//
// The evaluator delegates several concerns through interfaces so the
// sandbox (Phase 19), filters (Phase 12), tests (Phase 13), and loaders
// (Phase 11) can plug in without circular imports:
//
//   - [Engine] supplies attr/item access, filter/test invocation,
//     undefined construction, template loading, and autoescape policy.
//
// The evaluator itself is stateless (apart from the io.Writer it writes
// to). Concurrent renders just need separate Contexts.
package eval

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/jryberg/gojinja/pkg/ast"
	gjerrors "github.com/jryberg/gojinja/pkg/errors"
	"github.com/jryberg/gojinja/pkg/escape"
	"github.com/jryberg/gojinja/pkg/runtime"
)

// Engine is the slice of Environment behaviour the evaluator needs. The
// real Environment in pkg/environment implements it; tests can supply a
// stub.
type Engine interface {
	// GetAttr fetches obj.attr, going through the sandbox.
	GetAttr(obj any, attr string) (any, error)
	// GetItem fetches obj[arg], going through the sandbox.
	GetItem(obj any, arg any) (any, error)
	// CallFilter invokes a registered filter by name.
	CallFilter(name string, ctx *runtime.Context, value any, args []any, kwargs map[string]any) (any, error)
	// CallTest invokes a registered test by name.
	CallTest(name string, ctx *runtime.Context, value any, args []any, kwargs map[string]any) (any, error)
	// Call invokes a callable (a Go func, a Macro, etc.) with positional
	// and keyword args. Sandbox enforcement applies for plain Go callables.
	Call(ctx *runtime.Context, callee any, args []any, kwargs map[string]any) (any, error)
	// UndefinedFactory returns the configured undefined factory.
	UndefinedFactory() runtime.UndefinedFactory
	// LoadTemplate loads and parses (cached) another template by name.
	LoadTemplate(name string) (*ast.Template, error)
	// AutoescapeFor returns the per-template autoescape decision.
	AutoescapeFor(name string) bool
}

// Render renders tpl into w against ctx. Returns the number of bytes
// written and any error.
func Render(ctx context.Context, w io.Writer, tpl *ast.Template, rctx *runtime.Context, eng Engine) (err error) {
	// repr and str() of too deeply nested values panic, having no error
	// result; the render fails with their error.
	defer func() {
		if r := recover(); r != nil {
			re, ok := r.(escape.RecursionError)
			if !ok {
				panic(r)
			}
			err = gjerrors.NewTemplateRuntimeError(re.Error())
		}
	}()
	ev := &evaluator{eng: eng, w: w, goCtx: ctx}
	return ev.evalTemplate(tpl.Body, rctx)
}

// maxInheritance bounds an extends chain, so circular inheritance fails
// instead of looping (Python runs out of recursion depth).
const maxInheritance = 256

// evalTemplate runs a template's top level and then, as Jinja2 does when
// a child template's root code finishes, the parent it extends.
func (e *evaluator) evalTemplate(body []ast.Node, ctx *runtime.Context) error {
	if err := e.evalBody(body, ctx); err != nil {
		return err
	}
	for depth := 0; e.parent != nil; depth++ {
		if depth == maxInheritance {
			return gjerrors.NewTemplateRuntimeError("circular template inheritance")
		}
		parent := e.parent
		e.parent, e.extendsFired = nil, false
		if err := e.evalBody(parent.Body, ctx); err != nil {
			return err
		}
	}
	return nil
}

// RenderString is a convenience over Render that returns the output as
// a string instead of writing to a Writer.
func RenderString(ctx context.Context, tpl *ast.Template, rctx *runtime.Context, eng Engine) (string, error) {
	var b strings.Builder
	if err := Render(ctx, &b, tpl, rctx, eng); err != nil {
		return "", err
	}
	return b.String(), nil
}

// evaluator is the per-render state.
type evaluator struct {
	eng   Engine
	w     io.Writer
	goCtx context.Context
	// extendsFired flips to true once `{% extends %}` runs. Output
	// (`{{ … }}`) statements that follow it in the child body are
	// discarded — Python skips them entirely (no evaluation, no side
	// effects). Non-output statements (`{% set %}`, `{% macro %}`,
	// `{% import %}`, …) still run, mirroring Jinja2's
	// `parent_template` gate.
	extendsFired bool
	// parent is the template the running one extends; evalTemplate
	// renders it once the child's top level is done.
	parent *ast.Template
}

// evalBody evaluates a slice of statements.
func (e *evaluator) evalBody(body []ast.Node, ctx *runtime.Context) error {
	for _, n := range body {
		if err := e.checkCancel(); err != nil {
			return err
		}
		if err := e.evalNode(n, ctx); err != nil {
			return err
		}
	}
	return nil
}

func (e *evaluator) checkCancel() error {
	if e.goCtx == nil {
		return nil
	}
	select {
	case <-e.goCtx.Done():
		return e.goCtx.Err()
	default:
		return nil
	}
}

// evalNode dispatches a single statement.
func (e *evaluator) evalNode(n ast.Node, ctx *runtime.Context) error {
	switch x := n.(type) {
	case *ast.Output:
		return e.evalOutput(x, ctx)
	case *ast.For:
		return e.evalFor(x, ctx)
	case *ast.If:
		return e.evalIf(x, ctx)
	case *ast.Assign:
		return e.evalAssign(x, ctx)
	case *ast.AssignBlock:
		return e.evalAssignBlock(x, ctx)
	case *ast.With:
		return e.evalWith(x, ctx)
	case *ast.Block:
		return e.evalBlock(x, ctx)
	case *ast.Scope:
		return e.evalBody(x.Body, ctx.Child())
	case *ast.FilterBlock:
		return e.evalFilterBlock(x, ctx)
	case *ast.ScopedEvalContextModifier:
		return e.evalScopedECM(x, ctx)
	case *ast.EvalContextModifier:
		return e.evalECM(x, ctx)
	case *ast.ExprStmt:
		expr, ok := x.Node.(ast.Expr)
		if !ok {
			return fmt.Errorf("ExprStmt.Node is not an Expr: %T", x.Node)
		}
		_, err := e.evalExprAny(expr, ctx)
		return err
	case *ast.Continue:
		return errContinue
	case *ast.Break:
		return errBreak
	case *ast.Macro:
		return e.evalMacroDef(x, ctx)
	case *ast.CallBlock:
		return e.evalCallBlock(x, ctx)
	case *ast.Include:
		return e.evalInclude(x, ctx)
	case *ast.Extends:
		return e.evalExtends(x, ctx)
	case *ast.Import:
		return e.evalImport(x, ctx)
	case *ast.FromImport:
		return e.evalFromImport(x, ctx)
	}
	return fmt.Errorf("eval: unsupported statement %T", n)
}

// evalOutput emits each expression in turn.
func (e *evaluator) evalOutput(o *ast.Output, ctx *runtime.Context) error {
	if e.extendsFired {
		return nil
	}
	for _, n := range o.Nodes {
		v, err := e.evalExpr(n, ctx)
		if err != nil {
			return err
		}
		if err := e.write(v, ctx); err != nil {
			return err
		}
	}
	return nil
}

// write emits v with the autoescape rules of the active eval context.
func (e *evaluator) write(v any, ctx *runtime.Context) error {
	if err := FailStrict(v); err != nil {
		return err
	}
	var s string
	if ctx.Eval != nil && ctx.Eval.Autoescape {
		s = string(escape.Escape(v))
	} else {
		s = escape.SoftStr(v)
	}
	_, err := io.WriteString(e.w, s)
	return err
}

// =============================================================== if

func (e *evaluator) evalIf(n *ast.If, ctx *runtime.Context) error {
	v, err := e.evalExprAny(n.Test.(ast.Expr), ctx)
	if err != nil {
		return err
	}
	ok, err := truth(v)
	if err != nil {
		return err
	}
	if ok {
		return e.evalBody(n.Body, ctx)
	}
	for _, elif := range n.Elif {
		v, err := e.evalExprAny(elif.Test.(ast.Expr), ctx)
		if err != nil {
			return err
		}
		ok, err := truth(v)
		if err != nil {
			return err
		}
		if ok {
			return e.evalBody(elif.Body, ctx)
		}
	}
	return e.evalBody(n.Else, ctx)
}

// =============================================================== for

// errContinue / errBreak are sentinel errors used as control-flow signals
// inside evalFor. They never escape evalFor.
var (
	errContinue = fmt.Errorf("__continue__")
	errBreak    = fmt.Errorf("__break__")
)

func (e *evaluator) evalFor(n *ast.For, ctx *runtime.Context) error {
	iter, err := e.evalExprAny(n.Iter.(ast.Expr), ctx)
	if err != nil {
		return err
	}
	items, err := ToIterable(iter)
	if err != nil {
		return err
	}
	// Like Jinja2, every iteration (and the else body) runs in a fresh
	// scope of its own: the targets, `loop` and any `{% set %}` in the
	// body are gone when it ends. The loop's `if` filters the items of
	// recursive calls too.
	filter := func(items []any) ([]any, error) {
		if n.Test == nil {
			return items, nil
		}
		filtered := items[:0:0]
		for _, it := range items {
			scope := ctx.Child()
			if err := assignTarget(n.Target, it, scope); err != nil {
				return nil, err
			}
			tv, err := e.evalExprAny(n.Test.(ast.Expr), scope)
			if err != nil {
				return nil, err
			}
			ok, err := truth(tv)
			if err != nil {
				return nil, err
			}
			if ok {
				filtered = append(filtered, it)
			}
		}
		return filtered, nil
	}
	if items, err = filter(items); err != nil {
		return err
	}
	if len(items) == 0 {
		return e.evalBody(n.Else, ctx.Child())
	}
	var recurse func(any, int) (any, error)
	run := func(ev *evaluator, lc *runtime.LoopContext) error {
		lc.Equal = func(a, b any) bool { ok, err := ValuesEqual(a, b); return err == nil && ok }
		for {
			v, ok := lc.Iterate()
			if !ok {
				return nil
			}
			scope := ctx.Child()
			scope.Set("loop", lc)
			if err := assignTarget(n.Target, v, scope); err != nil {
				return err
			}
			if err := ev.evalBody(n.Body, scope); err != nil {
				if err == errContinue {
					continue
				}
				if err == errBreak {
					return nil
				}
				return err
			}
		}
	}
	if n.Recursive {
		recurse = func(iterable any, depth int) (any, error) {
			leave, err := enter(ctx)
			if err != nil {
				return nil, err
			}
			defer leave()
			its, err := ToIterable(iterable)
			if err != nil {
				return nil, err
			}
			if its, err = filter(its); err != nil {
				return nil, err
			}
			var b strings.Builder
			subEv := &evaluator{eng: e.eng, w: &b, goCtx: e.goCtx}
			if len(its) == 0 {
				if err := subEv.evalBody(n.Else, ctx.Child()); err != nil {
					return nil, err
				}
			} else if err := run(subEv, runtime.NewLoopContext(its, depth, recurse)); err != nil {
				return nil, err
			}
			if ctx.Eval != nil && ctx.Eval.Autoescape {
				return escape.Markup(b.String()), nil
			}
			return b.String(), nil
		}
	}
	return run(e, runtime.NewLoopContext(items, 0, recurse))
}

// assignTarget writes value to target, supporting Name, NSRef, and Tuple
// destructuring. NSRef assignment requires the target to resolve to a
// runtime.Namespace.
func assignTarget(target ast.Node, value any, ctx *runtime.Context) error {
	switch t := target.(type) {
	case *ast.Name:
		ctx.Set(t.Name, value)
		return nil
	case *ast.NSRef:
		obj, ok := ctx.Resolve(t.Name)
		if !ok {
			return fmt.Errorf("namespace %q is not defined", t.Name)
		}
		ns, ok := obj.(*runtime.Namespace)
		if !ok {
			return fmt.Errorf("non-namespace object %T cannot be assigned to via attribute", obj)
		}
		ns.Set(t.Attr, value)
		return nil
	case *ast.Tuple:
		items, err := ToIterable(value)
		if err != nil {
			return err
		}
		if len(items) != len(t.Items) {
			return fmt.Errorf("cannot unpack %d values into %d targets", len(items), len(t.Items))
		}
		for i, it := range t.Items {
			if err := assignTarget(it, items[i], ctx); err != nil {
				return err
			}
		}
		return nil
	}
	return fmt.Errorf("eval: unsupported assignment target %T", target)
}

// =============================================================== set / with / etc.

func (e *evaluator) evalAssign(n *ast.Assign, ctx *runtime.Context) error {
	v, err := e.evalExprAny(n.Node.(ast.Expr), ctx)
	if err != nil {
		return err
	}
	return assignTarget(n.Target, v, ctx)
}

func (e *evaluator) evalAssignBlock(n *ast.AssignBlock, ctx *runtime.Context) error {
	var b strings.Builder
	sub := &evaluator{eng: e.eng, w: &b, goCtx: e.goCtx}
	if err := sub.evalBody(n.Body, ctx.Child()); err != nil {
		return err
	}
	v := any(b.String())
	if n.Filter != nil {
		fv, err := e.applyFilter(n.Filter, v, ctx)
		if err != nil {
			return err
		}
		v = fv
	}
	return assignTarget(n.Target, v, ctx)
}

// evalWith evaluates every value in the enclosing scope, then binds the
// targets in a scope of the body's own.
func (e *evaluator) evalWith(n *ast.With, ctx *runtime.Context) error {
	values := make([]any, len(n.Targets))
	for i := range n.Targets {
		v, err := e.evalExprAny(n.Values[i], ctx)
		if err != nil {
			return err
		}
		values[i] = v
	}
	scope := ctx.Child()
	for i, tgt := range n.Targets {
		if err := assignTarget(tgt, values[i], scope); err != nil {
			return err
		}
	}
	return e.evalBody(n.Body, scope)
}

func (e *evaluator) evalBlock(n *ast.Block, ctx *runtime.Context) error {
	// After `{% extends %}` has rendered the parent, the child body
	// keeps executing for side effects (`{% set %}`, `{% macro %}`, …)
	// but stops emitting output. `{% block %}` belongs to the output
	// surface, so it's also suppressed — Python's `parent_template`
	// gate covers it the same way.
	if e.extendsFired {
		return nil
	}
	// A block sees the template-level names; a scoped one also sees the
	// enclosing loop's or with's.
	scope := ctx.Root()
	if n.Scoped {
		scope = ctx
	}
	if n.Required && len(ctx.Blocks[n.Name]) == 0 {
		return requiredBlockError(n.Name)
	}
	if stack, ok := ctx.Blocks[n.Name]; ok && len(stack) > 0 {
		// Render the topmost (most-derived) block. super() walks toward 0.
		s, err := callBlockAtDepth(stack, len(stack)-1, scope, n.Name, e.eng.UndefinedFactory())
		if err != nil {
			return err
		}
		_, err = io.WriteString(e.w, s)
		return err
	}
	return e.evalBody(n.Body, scope.Child())
}

// callBlockAtDepth invokes the block at `depth` in `stack`, binding a
// `super` callable that recurses into depth-1 (or yields Undefined when
// no parent exists).
func callBlockAtDepth(stack []runtime.BlockRenderFunc, depth int, ctx *runtime.Context, name string, undef runtime.UndefinedFactory) (string, error) {
	leave, err := enter(ctx)
	if err != nil {
		return "", err
	}
	defer leave()
	scope := ctx.Child()
	if depth > 0 {
		scope.Set("super", func() (any, error) {
			return blockResult(callBlockAtDepth(stack, depth-1, ctx, name, undef))(ctx)
		})
	} else {
		scope.Set("super", undef("there is no parent block called "+escape.Repr(name)+".", "super", runtime.Missing, nil))
	}
	return stack[depth](scope)
}

// blockResult wraps a rendered block the way Jinja2's BlockReference
// returns it: Markup when autoescaping, so it is not escaped again.
func blockResult(s string, err error) func(*runtime.Context) (any, error) {
	return func(ctx *runtime.Context) (any, error) {
		if err != nil {
			return nil, err
		}
		if ctx.Eval != nil && ctx.Eval.Autoescape {
			return escape.Markup(s), nil
		}
		return s, nil
	}
}

// templateReference is Jinja2's `self`: attribute or item access renders
// the named block, most-derived definition first.
type templateReference struct {
	ctx   *runtime.Context
	undef runtime.UndefinedFactory
}

func (r *templateReference) String() string {
	if r.ctx.Name == "" {
		return "<TemplateReference None>"
	}
	return "<TemplateReference " + escape.Repr(r.ctx.Name) + ">"
}

func (r *templateReference) block(name string) any {
	stack := r.ctx.Blocks[name]
	if len(stack) == 0 {
		return r.undef("'jinja2.runtime.TemplateReference object' has no attribute "+escape.Repr(name), name, r, nil)
	}
	return func() (any, error) {
		return blockResult(callBlockAtDepth(stack, len(stack)-1, r.ctx, name, r.undef))(r.ctx)
	}
}

func (e *evaluator) evalFilterBlock(n *ast.FilterBlock, ctx *runtime.Context) error {
	var b strings.Builder
	sub := &evaluator{eng: e.eng, w: &b, goCtx: e.goCtx}
	if err := sub.evalBody(n.Body, ctx.Child()); err != nil {
		return err
	}
	v, err := e.applyFilter(n.Filter, b.String(), ctx)
	if err != nil {
		return err
	}
	return e.write(v, ctx)
}

// applyFilter runs a filter chain against a starting value.
func (e *evaluator) applyFilter(f *ast.Filter, value any, ctx *runtime.Context) (any, error) {
	args, kwargs, _, err := e.evalCallArgs(f.Args, f.Kwargs, f.DynArgs, f.DynKwargs, ctx)
	if err != nil {
		return nil, err
	}
	v, err := e.eng.CallFilter(f.Name, ctx, value, args, kwargs)
	v = runtime.ToPyList(v)
	if err != nil {
		return nil, err
	}
	if f.Node != nil {
		// Already-applied chain — should not happen for FilterBlock/AssignBlock.
		_ = f.Node
	}
	return v, nil
}

// =============================================================== eval ctx

func (e *evaluator) evalScopedECM(n *ast.ScopedEvalContextModifier, ctx *runtime.Context) error {
	saved := *ctx.Eval
	for _, opt := range n.Options {
		v, err := e.evalExprAny(opt.Value, ctx)
		if err != nil {
			return err
		}
		applyEvalOption(ctx.Eval, opt.Key, v)
	}
	defer func() { *ctx.Eval = saved }()
	return e.evalBody(n.Body, ctx.Child())
}

func (e *evaluator) evalECM(n *ast.EvalContextModifier, ctx *runtime.Context) error {
	for _, opt := range n.Options {
		v, err := e.evalExprAny(opt.Value, ctx)
		if err != nil {
			return err
		}
		applyEvalOption(ctx.Eval, opt.Key, v)
	}
	return nil
}

func applyEvalOption(ec *runtime.EvalContext, key string, v any) {
	if key == "autoescape" {
		ec.Autoescape = Truthy(v)
	}
}

// =============================================================== includes

func (e *evaluator) evalInclude(n *ast.Include, ctx *runtime.Context) error {
	v, err := e.evalExprAny(n.Template, ctx)
	if err != nil {
		return err
	}
	names := stringList(v)
	var loaded *ast.Template
	var lastErr error
	for _, name := range names {
		t, err := e.eng.LoadTemplate(name)
		if err == nil {
			loaded = t
			break
		}
		lastErr = err
	}
	if loaded == nil {
		if n.IgnoreMissing {
			return nil
		}
		if lastErr != nil {
			return lastErr
		}
		return gjerrors.NewTemplateNotFound(v, "")
	}
	// The included template gets a context of its own over the caller's
	// names, so its assignments stay inside it.
	subCtx := ctx.Derived(nil)
	if !n.WithContext {
		// Python: `without context` strips the caller's vars but
		// keeps env globals visible. We rebuild a fresh parent from
		// the engine's globals snapshot.
		parent := runtime.NewOrderedDict()
		if g, ok := e.eng.(globalsProvider); ok {
			parent.MergeMap(g.Globals())
		}
		subCtx = runtime.NewContext(names[0], ctx.Env, parent, ctx.Eval)
	}
	// Like a standalone render, the included template runs with its own
	// inheritance chain's blocks and its own extends state.
	if bp, ok := e.eng.(blocksProvider); ok {
		blocks, err := bp.TemplateBlocks(loaded)
		if err != nil {
			return err
		}
		subCtx.Blocks = blocks
	}
	leave, err := enter(ctx)
	if err != nil {
		return err
	}
	defer leave()
	sub := &evaluator{eng: e.eng, w: e.w, goCtx: e.goCtx}
	return sub.evalTemplate(loaded.Body, subCtx)
}

// enter guards a nested template call (macro, caller, block, include,
// import, recursive loop) against runaway recursion; call the returned
// func when the call ends.
func enter(ctx *runtime.Context) (func(), error) {
	if ctx.Eval == nil {
		return func() {}, nil
	}
	if err := ctx.Eval.Enter(); err != nil {
		return nil, err
	}
	return ctx.Eval.Leave, nil
}

// requiredBlockError is what a required block that nothing overrides
// raises when it is rendered.
func requiredBlockError(name string) error {
	return gjerrors.NewTemplateRuntimeError("Required block " + escape.Repr(name) + " not found")
}

// RequiredBlock is the render function of a required block's own
// definition.
func RequiredBlock(name string) runtime.BlockRenderFunc {
	return func(*runtime.Context) (string, error) { return "", requiredBlockError(name) }
}

// orderedCaller is satisfied by the environment: Call with the order of
// the keyword arguments, for callables that keep it (dict(), namespace()).
type orderedCaller interface {
	CallOrdered(ctx *runtime.Context, callee any, args []any, kwargs map[string]any, order []string) (any, error)
}

// blocksProvider is satisfied by the environment: it returns the block
// stacks a template renders with, its inheritance chain resolved.
type blocksProvider interface {
	TemplateBlocks(t *ast.Template) (map[string][]runtime.BlockRenderFunc, error)
}

// globalsProvider is satisfied by the environment so the eval layer can
// fetch the globals snapshot without taking a hard dependency on the
// concrete *environment.Environment type.
type globalsProvider interface {
	Globals() map[string]any
}

// evalExtends records the parent template, which evalTemplate renders
// after the child's top level, and layers the parent's blocks beneath
// the ones already registered so the child's overrides win.
func (e *evaluator) evalExtends(n *ast.Extends, ctx *runtime.Context) error {
	if e.parent != nil {
		return gjerrors.NewTemplateRuntimeError("extended multiple times")
	}
	v, err := e.evalExprAny(n.Template, ctx)
	if err != nil {
		return err
	}
	names := stringList(v)
	if len(names) == 0 {
		return gjerrors.NewTemplateNotFound("", "")
	}
	parent, err := e.eng.LoadTemplate(names[0])
	if err != nil {
		return err
	}
	if bp, ok := e.eng.(blocksProvider); ok {
		blocks, err := bp.TemplateBlocks(parent)
		if err != nil {
			return err
		}
		for name, stack := range blocks {
			ctx.Blocks[name] = append(stack, ctx.Blocks[name]...)
		}
	}
	e.parent = parent
	e.extendsFired = true
	return nil
}

func (e *evaluator) evalImport(n *ast.Import, ctx *runtime.Context) error {
	v, err := e.evalExprAny(n.Template, ctx)
	if err != nil {
		return err
	}
	names := stringList(v)
	if len(names) == 0 {
		return gjerrors.NewTemplateNotFound("", "")
	}
	tpl, err := e.eng.LoadTemplate(names[0])
	if err != nil {
		return err
	}
	parent := ctx.Parent
	if !n.WithContext {
		parent = importParent(e.eng)
	}
	subCtx := runtime.NewContext(names[0], ctx.Env, parent, ctx.Eval)
	if n.WithContext {
		for k, v := range ctx.Locals() {
			subCtx.Set(k, v)
		}
	}
	if err := e.runImportedBody(tpl.Body, subCtx); err != nil {
		return err
	}
	exported := make(map[string]any, len(subCtx.Vars))
	for k, v := range subCtx.Vars {
		exported[k] = v
	}
	ctx.Set(n.Target, exported)
	return nil
}

// runImportedBody evaluates an imported template's body for its
// side-effects (macro definitions, `{% set %}` assignments, etc.) while
// discarding any inline output. Python's compiler routes imported
// templates' output to a discard buffer; without this, a stray trailing
// newline or stray `{{ … }}` in the imported file leaks into the
// caller's render output.
func (e *evaluator) runImportedBody(body []ast.Node, ctx *runtime.Context) error {
	leave, err := enter(ctx)
	if err != nil {
		return err
	}
	defer leave()
	sub := &evaluator{eng: e.eng, w: io.Discard, goCtx: e.goCtx}
	return sub.evalTemplate(body, ctx)
}

// importParent returns a globals-only parent OrderedDict — used for
// `import` / `from` without `with context`. Mirrors Python where
// imports are isolated from the caller's render vars by default.
func importParent(eng Engine) *runtime.OrderedDict {
	if g, ok := eng.(globalsProvider); ok {
		return runtime.OrderedDictFromMap(g.Globals())
	}
	return runtime.NewOrderedDict()
}

func (e *evaluator) evalFromImport(n *ast.FromImport, ctx *runtime.Context) error {
	v, err := e.evalExprAny(n.Template, ctx)
	if err != nil {
		return err
	}
	names := stringList(v)
	if len(names) == 0 {
		return gjerrors.NewTemplateNotFound("", "")
	}
	tpl, err := e.eng.LoadTemplate(names[0])
	if err != nil {
		return err
	}
	parent := ctx.Parent
	if !n.WithContext {
		parent = importParent(e.eng)
	}
	subCtx := runtime.NewContext(names[0], ctx.Env, parent, ctx.Eval)
	if n.WithContext {
		for k, v := range ctx.Locals() {
			subCtx.Set(k, v)
		}
	}
	if err := e.runImportedBody(tpl.Body, subCtx); err != nil {
		return err
	}
	for _, entry := range n.Names {
		v, ok := subCtx.Vars[entry.Name]
		if !ok {
			// Mirrors Python: the missing name is bound to an
			// Undefined whose hint flags the export miss; the error
			// surfaces only when something actually tries to use it.
			factory := e.eng.UndefinedFactory()
			hint := fmt.Sprintf("the template %q (imported on line %d) does not export the requested name %q", names[0], n.Position().Lineno, entry.Name)
			v = factory(hint, entry.Name, nil, nil)
		}
		alias := entry.Alias
		if alias == "" {
			alias = entry.Name
		}
		ctx.Set(alias, v)
	}
	return nil
}

// =============================================================== macros

// macroValue is what we bind in the context for `{% macro X %}`.
type macroValue struct {
	Name     string
	Args     []*ast.Name
	Defaults []ast.Expr
	Body     []ast.Node
	// CatchVarargs is true when the macro body references the bare name
	// `varargs` without first storing it. Extra positional arguments at
	// call time are bound as a list to that name.
	CatchVarargs bool
	// CatchKwargs is true when the macro body references the bare name
	// `kwargs` without first storing it. Extra keyword arguments at call
	// time are bound as a dict to that name.
	CatchKwargs bool
	// AccessesCaller is true when the body references the bare name
	// `caller` without first storing it. Used to surface a clearer error
	// when the macro is called outside of a `{% call %}` block.
	AccessesCaller bool
	// DefCtx is a live reference to the runtime context the macro was
	// defined in. Macro bodies look up names against this scope, so an
	// imported macro reads the imported template's namespace (with
	// later macro definitions in the same module visible too —
	// recursion across imported macros works).
	DefCtx *runtime.Context
}

func (e *evaluator) evalMacroDef(n *ast.Macro, ctx *runtime.Context) error {
	declared := map[string]bool{}
	for _, a := range n.Args {
		declared[a.Name] = true
	}
	catchVar, catchKw, hasCaller := detectMacroSpecials(n.Body, declared)
	mv := &macroValue{
		Name:           n.Name,
		Args:           n.Args,
		Defaults:       n.Defaults,
		Body:           n.Body,
		CatchVarargs:   catchVar,
		CatchKwargs:    catchKw,
		AccessesCaller: hasCaller,
		DefCtx:         ctx,
	}
	ctx.Set(n.Name, mv)
	return nil
}

// detectMacroSpecials returns whether the macro body references the bare
// names `varargs`, `kwargs`, and `caller` without first having stored a
// local of the same name. Mirrors Jinja2's `find_undeclared(body,
// ("caller","kwargs","varargs"))` plus the `skip_special_params` logic
// (declared args take precedence over the catcher behaviour). The visit
// stops at nested Block nodes, matching UndeclaredNameVisitor.visit_Block.
func detectMacroSpecials(body []ast.Node, declared map[string]bool) (catchVar, catchKw, hasCaller bool) {
	watching := map[string]bool{}
	for _, name := range []string{"varargs", "kwargs", "caller"} {
		if !declared[name] {
			watching[name] = true
		}
	}
	undeclared := map[string]bool{}
	var walk func(n ast.Node)
	walk = func(n ast.Node) {
		if n == nil {
			return
		}
		if _, ok := n.(*ast.Block); ok {
			// Block bodies are their own frame — Jinja2's
			// UndeclaredNameVisitor stops here.
			return
		}
		if name, ok := n.(*ast.Name); ok {
			if name.Ctx == ast.CtxLoad {
				if watching[name.Name] {
					undeclared[name.Name] = true
				}
			} else {
				watching[name.Name] = false
			}
			return
		}
		for _, c := range ast.Children(n) {
			walk(c)
		}
	}
	for _, n := range body {
		walk(n)
	}
	return undeclared["varargs"], undeclared["kwargs"], undeclared["caller"]
}

// macroRepr is a macro's name the way Jinja2's messages show it.
func (mv *macroValue) macroRepr() string {
	if mv.Name == "" {
		return "None"
	}
	return escape.Repr(mv.Name)
}

// Get exposes the public attributes of Jinja2's Macro.
func (mv *macroValue) Get(attr string) (any, bool) {
	switch attr {
	case "name":
		if mv.Name == "" {
			return nil, true
		}
		return mv.Name, true
	case "arguments":
		names := make(runtime.Tuple, len(mv.Args))
		for i, a := range mv.Args {
			names[i] = a.Name
		}
		return names, true
	case "catch_kwargs":
		return mv.CatchKwargs, true
	case "catch_varargs":
		return mv.CatchVarargs, true
	case "caller":
		return mv.AccessesCaller, true
	case "explicit_caller":
		for _, a := range mv.Args {
			if a.Name == "caller" {
				return true, true
			}
		}
		return false, true
	}
	return nil, false
}

// String is Jinja2's Macro repr.
func (mv *macroValue) String() string {
	if mv.Name == "" {
		return "<Macro anonymous>"
	}
	return "<Macro " + escape.Repr(mv.Name) + ">"
}

// CallMacro invokes a macro the way Jinja2's Macro.__call__ does:
// parameters are filled from the positional arguments, then from
// keywords, then from their defaults, which are evaluated in the macro's
// own frame so they can refer to earlier parameters. caller, when not
// nil, is bound as `caller`; extra arguments go to `varargs` / `kwargs`
// when the body uses them and are an error otherwise. The result is
// Markup when the calling context autoescapes.
func (e *evaluator) CallMacro(mv *macroValue, ctx *runtime.Context, args []any, kwargs map[string]any, kwOrder []string, caller any) (any, error) {
	leave, err := enter(ctx)
	if err != nil {
		return nil, err
	}
	defer leave()
	// The macro frame's Parent merges the captured definition-site
	// scope. This is the macro's closure: imported macros see the
	// imported template's namespace; locally-defined macros see (and can
	// mutually call) their siblings.
	defCtx := mv.DefCtx
	if defCtx == nil {
		defCtx = ctx
	}
	parent := runtime.NewOrderedDict()
	parent.MergeOrderedDict(defCtx.Parent)
	parent.MergeMap(defCtx.Locals())
	frame := runtime.NewContext(ctx.Name, ctx.Env, parent, ctx.Eval)
	remainingKw := make(map[string]any, len(kwargs))
	for k, v := range kwargs {
		remainingKw[k] = v
	}
	for i, name := range mv.Args {
		if i < len(args) {
			frame.Set(name.Name, args[i])
			continue
		}
		if v, ok := remainingKw[name.Name]; ok {
			frame.Set(name.Name, v)
			delete(remainingKw, name.Name)
			continue
		}
		if di := i - (len(mv.Args) - len(mv.Defaults)); di >= 0 && di < len(mv.Defaults) {
			dv, err := e.evalExprAny(mv.Defaults[di], frame)
			if err != nil {
				return nil, err
			}
			frame.Set(name.Name, dv)
			continue
		}
		frame.Set(name.Name, missingParam(e.eng, name.Name))
	}
	if caller != nil {
		frame.Set("caller", caller)
	} else if mv.AccessesCaller {
		if v, ok := remainingKw["caller"]; ok {
			frame.Set("caller", v)
			delete(remainingKw, "caller")
		} else {
			frame.Set("caller", e.eng.UndefinedFactory()("No caller defined", "caller", runtime.Missing, nil))
		}
	}
	if mv.CatchKwargs {
		// Python's kwargs dict keeps the call's keyword order.
		rest := runtime.NewOrderedDict()
		for _, k := range kwOrder {
			if v, ok := remainingKw[k]; ok {
				rest.Set(k, v)
			}
		}
		frame.Set("kwargs", rest)
	} else if len(remainingKw) > 0 {
		if _, ok := remainingKw["caller"]; ok {
			//lint:ignore ST1005 Jinja2's exact message
			return nil, fmt.Errorf("macro %s was invoked with two values for the special caller argument. This is most likely a bug.", mv.macroRepr())
		}
		names := make([]string, 0, len(remainingKw))
		for k := range remainingKw {
			names = append(names, k)
		}
		sort.Strings(names)
		return nil, fmt.Errorf("macro %s takes no keyword argument %s", mv.macroRepr(), escape.Repr(names[0]))
	}
	if mv.CatchVarargs {
		extra := []any{}
		if len(args) > len(mv.Args) {
			extra = append(extra, args[len(mv.Args):]...)
		}
		frame.Set("varargs", runtime.Tuple(extra))
	} else if len(args) > len(mv.Args) {
		return nil, fmt.Errorf("macro %s takes not more than %d argument(s)", mv.macroRepr(), len(mv.Args))
	}
	var b strings.Builder
	sub := &evaluator{eng: e.eng, w: &b, goCtx: e.goCtx}
	if err := sub.evalBody(mv.Body, frame); err != nil {
		return nil, err
	}
	if ctx.Eval != nil && ctx.Eval.Autoescape {
		return escape.Markup(b.String()), nil
	}
	return b.String(), nil
}

// evalCallBlock evaluates `{% call %}`: the body becomes an anonymous
// macro, Jinja2's `caller`, passed to the called macro.
func (e *evaluator) evalCallBlock(n *ast.CallBlock, ctx *runtime.Context) error {
	macro, err := e.evalExprAny(n.Call.Node, ctx)
	if err != nil {
		return err
	}
	mv, ok := macro.(*macroValue)
	if !ok {
		return fmt.Errorf("call block target is not a macro: %T", macro)
	}
	args, kwargs, order, err := e.evalCallArgs(n.Call.Args, n.Call.Kwargs, n.Call.DynArgs, n.Call.DynKwargs, ctx)
	if err != nil {
		return err
	}
	declared := map[string]bool{}
	for _, a := range n.Args {
		declared[a.Name] = true
	}
	catchVar, catchKw, hasCaller := detectMacroSpecials(n.Body, declared)
	caller := &macroValue{
		Args:           n.Args,
		Defaults:       n.Defaults,
		Body:           n.Body,
		CatchVarargs:   catchVar,
		CatchKwargs:    catchKw,
		AccessesCaller: hasCaller,
		DefCtx:         ctx,
	}
	v, err := e.CallMacro(mv, ctx, args, kwargs, order, caller)
	if err != nil {
		return err
	}
	return e.write(v, ctx)
}

// =============================================================== helpers

// Truthy reports Python truthiness: empty strings, sequences and
// mappings, zero numbers, None and Undefined are false.
func Truthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case int:
		return x != 0
	case int64:
		return x != 0
	case float64:
		return x != 0
	case string:
		return len(x) != 0
	case escape.Markup:
		return len(x) != 0
	case runtime.Bytes:
		return len(x) != 0
	case []any:
		return len(x) != 0
	case runtime.Tuple:
		return len(x) != 0
	case *runtime.PyList:
		return x.Len() != 0
	case *runtime.OrderedDict:
		return x.Len() != 0
	case *runtime.DictView:
		return x.Len() != 0
	case map[any]any:
		return len(x) != 0
	case map[string]any:
		return len(x) != 0
	case runtime.Undefined:
		return false
	}
	if u, ok := v.(runtime.Undefiner); ok && u.IsUndefined() {
		return false
	}
	return true
}

// ToIterable coerces a value to []any the way `for` iterates it: dicts
// yield their keys, strings their characters, Undefined nothing.
func ToIterable(v any) ([]any, error) {
	switch x := v.(type) {
	case nil:
		return nil, fmt.Errorf("'NoneType' object is not iterable")
	case runtime.Tuple:
		return []any(x), nil
	case []any:
		return x, nil
	case *runtime.PyList:
		// In-template lists carry mutating methods, but iteration just
		// needs the underlying slice view.
		return x.Items(), nil
	case []string:
		out := make([]any, len(x))
		for i, s := range x {
			out[i] = s
		}
		return out, nil
	case string:
		return runeStrings(x), nil
	case escape.Markup:
		// Python iterates a Markup like the str it subclasses: plain str
		// characters of the escaped text.
		return runeStrings(string(x)), nil
	case runtime.Bytes:
		out := make([]any, len(x))
		for i, c := range x {
			out[i] = int64(c)
		}
		return out, nil
	case map[string]any:
		// Go map iteration is randomised. Lex-sort for deterministic
		// output — matches the policy used by `.items()/.keys()/.values()`
		// in pkg/environment/dictmethods.go. Templates iterating a
		// JSON-loaded dict get insertion order via *runtime.OrderedDict
		// (see varsutil.JSONVars); raw map[string]any from Go callers
		// can't carry insertion order, so this is the deterministic
		// fallback.
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := make([]any, len(keys))
		for i, k := range keys {
			out[i] = k
		}
		return out, nil
	case map[any]any:
		// Same rationale as map[string]any. Sort by the key's stringified
		// form (mirrors dictmethods.dictStringKeys) while keeping the
		// original any-typed key in the output slice.
		type ks struct {
			k any
			s string
		}
		ksv := make([]ks, 0, len(x))
		for k := range x {
			ksv = append(ksv, ks{k: k, s: fmt.Sprint(k)})
		}
		sort.Slice(ksv, func(i, j int) bool { return ksv[i].s < ksv[j].s })
		out := make([]any, len(ksv))
		for i, kv := range ksv {
			out[i] = kv.k
		}
		return out, nil
	case *runtime.OrderedDict:
		// Iterating a dict yields its keys in insertion order, matching
		// Python's `for k in d` semantics.
		return x.Keys(), nil
	case *runtime.DictView:
		return x.Items(), nil
	}
	if u, ok := v.(runtime.Undefined); ok {
		return u.Iter()
	}
	if u, ok := v.(runtime.Undefiner); ok && u.IsUndefined() {
		return nil, nil
	}
	return nil, fmt.Errorf("'%s' object is not iterable", runtime.PyTypeName(v))
}

func runeStrings(s string) []any {
	out := make([]any, 0, len(s))
	for _, r := range s {
		out = append(out, string(r))
	}
	return out
}

// missingParam is the value of a macro parameter the call did not pass.
func missingParam(eng Engine, name string) runtime.Undefined {
	return eng.UndefinedFactory()("parameter "+escape.Repr(name)+" was not provided", name, runtime.Missing, nil)
}

// stringList turns a single string or a list of strings into []string.
// Used by include / extends for selecting templates.
func stringList(v any) []string {
	if s, ok := v.(string); ok {
		return []string{s}
	}
	items, err := ToIterable(v)
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, it := range items {
		if s, ok := it.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
