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
func Render(ctx context.Context, w io.Writer, tpl *ast.Template, rctx *runtime.Context, eng Engine) error {
	ev := &evaluator{eng: eng, w: w, goCtx: ctx}
	return ev.evalBody(tpl.Body, rctx)
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
	// extendsFired flips to true after `{% extends %}` runs to render
	// the parent. Output (`{{ … }}`) statements that follow it in the
	// child body are discarded — Python skips them entirely (no
	// evaluation, no side effects). Non-output statements
	// (`{% set %}`, `{% macro %}`, `{% import %}`, …) are NOT
	// suppressed, mirroring Jinja2's `parent_template`-gate model.
	extendsFired bool
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
		return e.evalBody(x.Body, ctx)
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
	if truthy(v) {
		return e.evalBody(n.Body, ctx)
	}
	for _, elif := range n.Elif {
		v, err := e.evalExprAny(elif.Test.(ast.Expr), ctx)
		if err != nil {
			return err
		}
		if truthy(v) {
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
	items, err := toIterable(iter)
	if err != nil {
		return err
	}
	// Save and later restore every variable the target mentions, so the
	// loop's iteration variables don't leak into the surrounding scope.
	// Python's for-loop scope rule: outside the loop body, outer-scope
	// names are restored. The else branch sees the outer scope, not the
	// (never-set) loop variable.
	restoreNames := collectTargetNames(n.Target)
	saved := make(map[string]any, len(restoreNames))
	hadKey := make(map[string]bool, len(restoreNames))
	for _, name := range restoreNames {
		if v, ok := ctx.Vars[name]; ok {
			saved[name] = v
			hadKey[name] = true
		}
	}
	defer func() {
		for _, name := range restoreNames {
			if hadKey[name] {
				ctx.Vars[name] = saved[name]
			} else {
				delete(ctx.Vars, name)
			}
		}
	}()

	if n.Test != nil {
		filtered := items[:0:0]
		for _, it := range items {
			if err := assignTarget(n.Target, it, ctx); err != nil {
				return err
			}
			tv, err := e.evalExprAny(n.Test.(ast.Expr), ctx)
			if err != nil {
				return err
			}
			if truthy(tv) {
				filtered = append(filtered, it)
			}
		}
		items = filtered
	}

	if len(items) == 0 {
		// Restore outer scope so the else block sees pre-loop bindings.
		for _, name := range restoreNames {
			if hadKey[name] {
				ctx.Vars[name] = saved[name]
			} else {
				delete(ctx.Vars, name)
			}
		}
		return e.evalBody(n.Else, ctx)
	}

	parentLoop, hadParent := ctx.Vars["loop"]
	defer func() {
		if hadParent {
			ctx.Vars["loop"] = parentLoop
		} else {
			delete(ctx.Vars, "loop")
		}
	}()

	var recurse func([]any, int) (string, error)
	if n.Recursive {
		recurse = func(its []any, depth int) (string, error) {
			var b strings.Builder
			subEv := &evaluator{eng: e.eng, w: &b, goCtx: e.goCtx}
			lc := runtime.NewLoopContext(its, depth, recurse)
			ctx.Vars["loop"] = lc
			for {
				v, ok := lc.Iterate()
				if !ok {
					break
				}
				if err := assignTarget(n.Target, v, ctx); err != nil {
					return "", err
				}
				if err := subEv.evalBody(n.Body, ctx); err != nil {
					if err == errContinue {
						continue
					}
					if err == errBreak {
						break
					}
					return "", err
				}
			}
			return b.String(), nil
		}
	}

	lc := runtime.NewLoopContext(items, 0, recurse)
	ctx.Vars["loop"] = lc
	for {
		v, ok := lc.Iterate()
		if !ok {
			break
		}
		if err := assignTarget(n.Target, v, ctx); err != nil {
			return err
		}
		if err := e.evalBody(n.Body, ctx); err != nil {
			if err == errContinue {
				continue
			}
			if err == errBreak {
				break
			}
			return err
		}
	}
	return nil
}

// collectTargetNames walks an assignment target and returns every plain
// Name's identifier — the for-loop's iteration variables. NSRef and
// Getitem/Getattr targets don't shadow outer scope names so they're
// skipped.
func collectTargetNames(n ast.Node) []string {
	var out []string
	var walk func(ast.Node)
	walk = func(x ast.Node) {
		switch t := x.(type) {
		case *ast.Name:
			out = append(out, t.Name)
		case *ast.Tuple:
			for _, it := range t.Items {
				walk(it)
			}
		}
	}
	walk(n)
	return out
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
		items, err := toIterable(value)
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
	if err := sub.evalBody(n.Body, ctx); err != nil {
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

func (e *evaluator) evalWith(n *ast.With, ctx *runtime.Context) error {
	saved := make(map[string]any, len(n.Targets))
	hadKey := make(map[string]bool, len(n.Targets))
	for i, tgt := range n.Targets {
		v, err := e.evalExprAny(n.Values[i], ctx)
		if err != nil {
			return err
		}
		name := tgt.(*ast.Name).Name
		if old, ok := ctx.Vars[name]; ok {
			saved[name] = old
			hadKey[name] = true
		}
		ctx.Set(name, v)
	}
	defer func() {
		for name := range hadKey {
			ctx.Vars[name] = saved[name]
		}
		for _, tgt := range n.Targets {
			if !hadKey[tgt.(*ast.Name).Name] {
				delete(ctx.Vars, tgt.(*ast.Name).Name)
			}
		}
	}()
	return e.evalBody(n.Body, ctx)
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
	if stack, ok := ctx.Blocks[n.Name]; ok && len(stack) > 0 {
		// Render the topmost (most-derived) block. super() walks toward 0.
		s, err := callBlockAtDepth(stack, len(stack)-1, ctx, n.Name)
		if err != nil {
			return err
		}
		_, err = io.WriteString(e.w, s)
		return err
	}
	return e.evalBody(n.Body, ctx)
}

// callBlockAtDepth invokes the block at `depth` in `stack`, binding a
// `super` callable that recurses into depth-1 (or yields Undefined when
// no parent exists).
func callBlockAtDepth(stack []runtime.BlockRenderFunc, depth int, ctx *runtime.Context, name string) (string, error) {
	prev, hadSuper := ctx.Vars["super"]
	if depth > 0 {
		ctx.Vars["super"] = func() (any, error) {
			return callBlockAtDepth(stack, depth-1, ctx, name)
		}
	} else {
		ctx.Vars["super"] = runtime.NewBase("there is no parent block called "+name, "super", nil, nil)
	}
	defer func() {
		if hadSuper {
			ctx.Vars["super"] = prev
		} else {
			delete(ctx.Vars, "super")
		}
	}()
	return stack[depth](ctx)
}

func (e *evaluator) evalFilterBlock(n *ast.FilterBlock, ctx *runtime.Context) error {
	var b strings.Builder
	sub := &evaluator{eng: e.eng, w: &b, goCtx: e.goCtx}
	if err := sub.evalBody(n.Body, ctx); err != nil {
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
	args, kwargs, err := e.evalCallArgs(f.Args, f.Kwargs, ctx)
	if err != nil {
		return nil, err
	}
	v, err := e.eng.CallFilter(f.Name, ctx, value, args, kwargs)
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
	return e.evalBody(n.Body, ctx)
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
		ec.Autoescape = truthy(v)
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
	subCtx := ctx
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
	return e.evalBody(loaded.Body, subCtx)
}

// globalsProvider is satisfied by the environment so the eval layer can
// fetch the globals snapshot without taking a hard dependency on the
// concrete *environment.Environment type.
type globalsProvider interface {
	Globals() map[string]any
}

// evalExtends loads the parent template and renders its body in the
// current context. Block overrides defined in the child are already
// layered onto ctx.Blocks before evaluation starts, so {% block %} in
// the parent dispatches up to the child's most-recent override. After
// the parent finishes, control returns to the child's body; the
// `extendsFired` gate then suppresses the child's remaining output
// statements but lets `{% set %}`, `{% macro %}`, `{% import %}`, etc.
// continue to execute (matching Python's `parent_template`-gate model).
func (e *evaluator) evalExtends(n *ast.Extends, ctx *runtime.Context) error {
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
	if err := e.evalBody(parent.Body, ctx); err != nil {
		return err
	}
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
		for k, v := range ctx.Vars {
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
	saved := e.w
	e.w = io.Discard
	defer func() { e.w = saved }()
	return e.evalBody(body, ctx)
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
		for k, v := range ctx.Vars {
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
	// Captured eval-context (autoescape flag at definition site).
	Autoescape bool
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
	autoescape := false
	if ctx.Eval != nil {
		autoescape = ctx.Eval.Autoescape
	}
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
		Autoescape:     autoescape,
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

// CallMacro invokes a macro value with positional args + kwargs and
// returns the rendered string (always a Markup if the macro was defined
// in autoescape mode). caller, if non-nil, is bound as `caller` inside
// the macro body so `{{ caller() }}` re-renders the {% call %} body. Any
// positional or keyword arguments not consumed by declared parameters are
// bound to `varargs` / `kwargs` when the macro body references those
// names; otherwise they raise an error.
func (e *evaluator) CallMacro(mv *macroValue, ctx *runtime.Context, args []any, kwargs map[string]any, caller any) (any, error) {
	// The macro frame's Parent merges the captured definition-site
	// scope (Parent + Vars). This is the macro's closure: imported
	// macros see the imported template's namespace; locally-defined
	// macros see (and can mutually call) their siblings.
	defCtx := mv.DefCtx
	if defCtx == nil {
		defCtx = ctx
	}
	parent := runtime.NewOrderedDict()
	parent.MergeOrderedDict(defCtx.Parent)
	parent.MergeMap(defCtx.Vars)
	frame := runtime.NewContext(ctx.Name, ctx.Env, parent, ctx.Eval)
	// kwargs is mutated as we bind named parameters; copy so callers
	// don't observe pops.
	remainingKw := make(map[string]any, len(kwargs))
	for k, v := range kwargs {
		remainingKw[k] = v
	}
	// Fill declared positional args.
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
			dv, err := e.evalExprAny(mv.Defaults[di], ctx)
			if err != nil {
				return nil, err
			}
			frame.Set(name.Name, dv)
			continue
		}
		frame.Set(name.Name, runtime.NewBase("", name.Name, nil, nil))
	}
	// caller binding — explicit value wins; otherwise, if the body
	// references caller, surface an Undefined so reads produce a clear
	// error rather than a NameError equivalent.
	if caller != nil {
		frame.Set("caller", caller)
		delete(remainingKw, "caller")
	} else if mv.AccessesCaller {
		if v, ok := remainingKw["caller"]; ok {
			frame.Set("caller", v)
			delete(remainingKw, "caller")
		} else {
			frame.Set("caller", runtime.NewBase("No caller defined", "caller", nil, nil))
		}
	}
	// kwargs catcher: collect leftover keyword arguments.
	if mv.CatchKwargs {
		frame.Set("kwargs", anyMap(remainingKw))
	} else if len(remainingKw) > 0 {
		if _, ok := remainingKw["caller"]; ok {
			return nil, fmt.Errorf("macro %q was invoked with two values for the special caller argument", mv.Name)
		}
		for k := range remainingKw {
			return nil, fmt.Errorf("macro %q takes no keyword argument %q", mv.Name, k)
		}
	}
	// varargs catcher: collect overflow positional arguments.
	if mv.CatchVarargs {
		extra := []any{}
		if len(args) > len(mv.Args) {
			extra = append(extra, args[len(mv.Args):]...)
		}
		frame.Set("varargs", extra)
	} else if len(args) > len(mv.Args) {
		return nil, fmt.Errorf("macro %q takes not more than %d argument(s)", mv.Name, len(mv.Args))
	}
	var b strings.Builder
	sub := &evaluator{eng: e.eng, w: &b, goCtx: e.goCtx}
	if err := sub.evalBody(mv.Body, frame); err != nil {
		return nil, err
	}
	if mv.Autoescape {
		return escape.Markup(b.String()), nil
	}
	return b.String(), nil
}

// anyMap reboxes a map[string]any as map[any]any so renderers and the
// `tojson` filter see a uniform shape — this matches what Jinja2 produces
// when binding `kwargs` (Python dict).
func anyMap(m map[string]any) map[any]any {
	out := make(map[any]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// evalCallBlock evaluates `{% call %}` — invokes the macro named in the
// inner Call, passing a caller closure that renders the call-block body.
func (e *evaluator) evalCallBlock(n *ast.CallBlock, ctx *runtime.Context) error {
	macro, err := e.evalExprAny(n.Call.Node, ctx)
	if err != nil {
		return err
	}
	mv, ok := macro.(*macroValue)
	if !ok {
		return fmt.Errorf("call block target is not a macro: %T", macro)
	}
	args, kwargs, err := e.evalCallArgs(n.Call.Args, n.Call.Kwargs, ctx)
	if err != nil {
		return err
	}
	// Build the caller. Its signature mirrors a Go-side macro: it takes
	// args + kwargs and returns the rendered call-body. If the call block
	// declares its own arg list, those bind for each invocation.
	caller := callerOnce{
		args:     n.Args,
		defaults: n.Defaults,
		body:     n.Body,
		ctx:      ctx,
		ev:       e,
	}
	v, err := e.CallMacro(mv, ctx, args, kwargs, any(caller.fn()))
	if err != nil {
		return err
	}
	return e.write(v, ctx)
}

// callerOnce binds a {% call %} body so `caller()` (and `caller(arg, ...)`)
// can re-render it from inside the macro.
type callerOnce struct {
	args     []*ast.Name
	defaults []ast.Expr
	body     []ast.Node
	ctx      *runtime.Context
	ev       *evaluator
}

func (c callerOnce) fn() runtime.CallerFunc {
	return func(args []any, kwargs map[string]any) (any, error) {
		frame := runtime.NewContext(c.ctx.Name, c.ctx.Env, c.ctx.Parent, c.ctx.Eval)
		for k, v := range c.ctx.Vars {
			frame.Set(k, v)
		}
		for i, name := range c.args {
			if i < len(args) {
				frame.Set(name.Name, args[i])
				continue
			}
			if v, ok := kwargs[name.Name]; ok {
				frame.Set(name.Name, v)
				continue
			}
			if di := i - (len(c.args) - len(c.defaults)); di >= 0 && di < len(c.defaults) {
				dv, err := c.ev.evalExprAny(c.defaults[di], c.ctx)
				if err != nil {
					return nil, err
				}
				frame.Set(name.Name, dv)
				continue
			}
			frame.Set(name.Name, runtime.NewBase("", name.Name, nil, nil))
		}
		var b strings.Builder
		sub := &evaluator{eng: c.ev.eng, w: &b, goCtx: c.ev.goCtx}
		if err := sub.evalBody(c.body, frame); err != nil {
			return nil, err
		}
		// Caller output respects the parent context's autoescape state.
		if c.ctx.Eval != nil && c.ctx.Eval.Autoescape {
			return escape.Markup(b.String()), nil
		}
		return b.String(), nil
	}
}

// =============================================================== helpers

// truthy mirrors ast.truthy without importing it (avoids cycle).
func truthy(v any) bool {
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
	case []any:
		return len(x) != 0
	case *runtime.PyList:
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

// toIterable coerces a value to []any for iteration.
func toIterable(v any) ([]any, error) {
	switch x := v.(type) {
	case nil:
		return nil, nil
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
		out := make([]any, 0, len(x))
		for _, r := range x {
			out = append(out, string(r))
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
	}
	if u, ok := v.(runtime.Undefiner); ok && u.IsUndefined() {
		return nil, nil
	}
	return nil, fmt.Errorf("eval: value of type %T is not iterable", v)
}

// stringList turns a single string or a list of strings into []string.
// Used by include / extends for selecting templates.
func stringList(v any) []string {
	switch x := v.(type) {
	case string:
		return []string{x}
	case []any:
		out := make([]string, 0, len(x))
		for _, it := range x {
			if s, ok := it.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}
