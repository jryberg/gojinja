// Package environment exposes the user-facing API: the [Environment]
// type with safe-by-default options and the [Template] type returned by
// loaders.
//
// Defaults intentionally diverge from Python Jinja2 — see
// docs/divergences.md. In short: sandbox on, autoescape on, host
// environment off, bounded range, bounded cache. Opting out is explicit.
package environment

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"

	"github.com/jryberg/gojinja/pkg/ast"
	"github.com/jryberg/gojinja/pkg/debug"
	gjerrors "github.com/jryberg/gojinja/pkg/errors"
	"github.com/jryberg/gojinja/pkg/escape"
	"github.com/jryberg/gojinja/pkg/eval"
	"github.com/jryberg/gojinja/pkg/ext"
	"github.com/jryberg/gojinja/pkg/lexer"
	"github.com/jryberg/gojinja/pkg/loader"
	"github.com/jryberg/gojinja/pkg/parser"
	"github.com/jryberg/gojinja/pkg/runtime"
	"github.com/jryberg/gojinja/pkg/sandbox"
)

// FilterFunc is a registered template filter. The pass-arg dispatch is
// driven by the registered [Filter.Pass] field.
type FilterFunc func(env *Environment, ctx *runtime.Context, value any, args []any, kwargs map[string]any) (any, error)


// Filter is a registered filter with its dispatch hint.
type Filter struct {
	Pass runtime.PassArg
	Func FilterFunc
}

// TestFunc is a registered template test (predicate).
type TestFunc func(env *Environment, ctx *runtime.Context, value any, args []any, kwargs map[string]any) (bool, error)

// Test is a registered test with its dispatch hint.
type Test struct {
	Pass runtime.PassArg
	Func TestFunc
}

// Loader is re-exported from [pkg/loader]. A loader resolves a template
// name to its source text. There is one Loader interface in the project;
// every concrete loader in [pkg/loader] satisfies it directly.
type Loader = loader.Loader

// Source is re-exported from [pkg/loader].
type Source = loader.Source

// Environment is the central object. It holds lexer/parser settings,
// filter and test registries, globals, the autoescape policy, and the
// loader.
type Environment struct {
	lexerOpts lexer.Options
	autoesc   AutoescapePolicy
	loader    Loader
	globals   map[string]any
	filters   map[string]Filter
	tests     map[string]Test

	// undefined factory used during render
	undefined runtime.UndefinedFactory

	// sandboxed gates GetAttr / GetItem / Call.
	sandboxed bool

	// Bounded resources — never -1.
	maxRange   int
	maxCache   int
	maxParseDepth int

	// Template AST cache, keyed by name. The in-memory map is the
	// primary cache (always present); externalCache, when non-nil, is
	// consulted on miss and updated on parse — typically a
	// cache.Filesystem so bytecode-cache-style persistence Just Works.
	mu            sync.RWMutex
	cache         map[string]*ast.Template
	externalCache externalASTCache

	// extensions registered via [WithExtension]; consulted by the parser
	// when it encounters an unknown statement keyword.
	extensions map[string]parser.ExtensionFunc
}

// externalASTCache mirrors pkg/cache.Cache to avoid an import cycle.
type externalASTCache interface {
	Get(key, checksum string) (*ast.Template, bool)
	Put(key, checksum string, tpl *ast.Template) error
}

// AutoescapePolicy decides whether to autoescape a given template.
type AutoescapePolicy interface {
	AutoescapeFor(name string) bool
}

// AutoescapeAlways always returns true.
type AutoescapeAlways struct{}

// AutoescapeFor satisfies [AutoescapePolicy].
func (AutoescapeAlways) AutoescapeFor(string) bool { return true }

// AutoescapeNever always returns false.
type AutoescapeNever struct{}

// AutoescapeFor satisfies [AutoescapePolicy].
func (AutoescapeNever) AutoescapeFor(string) bool { return false }

// AutoescapeByExtension is a [AutoescapePolicy] backed by a list of
// "auto-on" file extensions.
type AutoescapeByExtension struct {
	Enabled          []string
	Disabled         []string
	DefaultForString bool
	Default          bool
}

// AutoescapeFor satisfies [AutoescapePolicy].
func (a AutoescapeByExtension) AutoescapeFor(name string) bool {
	if name == "" {
		return a.DefaultForString
	}
	low := strings.ToLower(name)
	for _, ext := range a.Disabled {
		if strings.HasSuffix(low, "."+ext) {
			return false
		}
	}
	for _, ext := range a.Enabled {
		if strings.HasSuffix(low, "."+ext) {
			return true
		}
	}
	return a.Default
}

// Option configures an [Environment] at construction.
type Option func(*Environment)

// WithLexerOptions overrides lexer settings (block markers, whitespace, etc.).
func WithLexerOptions(o lexer.Options) Option { return func(e *Environment) { e.lexerOpts = o } }

// WithLoader sets the template loader.
func WithLoader(l Loader) Option { return func(e *Environment) { e.loader = l } }

// WithAutoescape replaces the autoescape policy.
func WithAutoescape(p AutoescapePolicy) Option { return func(e *Environment) { e.autoesc = p } }

// WithGlobal registers (or replaces) a global by name.
func WithGlobal(name string, value any) Option {
	return func(e *Environment) { e.globals[name] = value }
}

// WithFilter registers a filter under name. Pass=PassNone means the
// filter takes only the value + args.
func WithFilter(name string, f Filter) Option {
	return func(e *Environment) { e.filters[name] = f }
}

// WithTest registers a test under name.
func WithTest(name string, t Test) Option {
	return func(e *Environment) { e.tests[name] = t }
}

// WithUndefined chooses the [Undefined] variant produced for missing
// names. Defaults to base.
func WithUndefined(f runtime.UndefinedFactory) Option {
	return func(e *Environment) { e.undefined = f }
}

// WithUnsafe disables the default sandbox. Reserved for explicit
// trusted-template scenarios; never enable for user-supplied templates.
func WithUnsafe() Option {
	return func(e *Environment) { e.sandboxed = false }
}

// WithHostEnv registers `env` as a global function that returns the
// host process's environment variables via os.Getenv. Missing variables
// return "". Off by default; opting in exposes every variable in the
// host environment to the template, so do not enable for templates
// supplied by untrusted users.
func WithHostEnv() Option {
	return func(e *Environment) {
		e.globals["env"] = func(name string) string { return os.Getenv(name) }
	}
}

// WithRangeLimit sets the maximum range size. Default 100000.
func WithRangeLimit(n int) Option { return func(e *Environment) { e.maxRange = n } }

// WithCacheSize sets the maximum compiled template cache. Default 400.
// Values <= 0 are clamped to 1 (unbounded caches are not permitted).
func WithCacheSize(n int) Option {
	return func(e *Environment) {
		if n <= 0 {
			n = 1
		}
		e.maxCache = n
	}
}

// WithExternalCache plugs a persistent AST cache (typically a
// pkg/cache.Filesystem) behind the in-memory cache. Compiled templates
// are persisted between processes so cold starts are cheaper.
func WithExternalCache(c interface {
	Get(key, checksum string) (*ast.Template, bool)
	Put(key, checksum string, tpl *ast.Template) error
}) Option {
	return func(e *Environment) { e.externalCache = c }
}

// WithExtension registers a parser extension. parse is invoked when the
// parser encounters `{% tag %}`. Extensions ship in pkg/ext.
func WithExtension(tag string, parse parser.ExtensionFunc) Option {
	return func(e *Environment) {
		if e.extensions == nil {
			e.extensions = map[string]parser.ExtensionFunc{}
		}
		e.extensions[tag] = parse
	}
}

// WithDebugExtension enables the `{% debug %}` tag, dumping the active
// context plus filter and test names. Equivalent to registering
// [ext.DebugTag] under "debug" and [ext.DebugRender] as the helper
// global.
func WithDebugExtension() Option {
	return func(e *Environment) {
		WithExtension("debug", ext.DebugTag)(e)
		e.globals[ext.DebugGlobalName] = ext.DebugRender
	}
}

// WithI18NExtension enables `{% trans %}` / `{% pluralize %}` /
// `{% endtrans %}` plus the `gettext` / `ngettext` / `pgettext` /
// `npgettext` and `_` globals backed by t. Pass [ext.NullTranslator]{}
// for a passthrough that returns messages unchanged.
func WithI18NExtension(t ext.Translator) Option {
	return func(e *Environment) {
		WithExtension("trans", ext.I18NTag)(e)
		for k, v := range ext.TranslatorGlobals(t) {
			e.globals[k] = v
		}
	}
}

// New constructs an Environment with safe defaults and the given options.
func New(opts ...Option) (*Environment, error) {
	e := &Environment{
		lexerOpts:     lexer.DefaultOptions(),
		autoesc:       AutoescapeAlways{},
		globals:       map[string]any{},
		filters:       map[string]Filter{},
		tests:         map[string]Test{},
		undefined:     runtime.NewBase,
		sandboxed:     true,
		maxRange:      100_000,
		maxCache:      400,
		maxParseDepth: 200,
		cache:         map[string]*ast.Template{},
		extensions:    map[string]parser.ExtensionFunc{},
	}
	registerBuiltins(e)
	for _, o := range opts {
		o(e)
	}
	return e, nil
}

// Filters returns a snapshot of the env's registered filter names.
func (e *Environment) Filters() []string {
	names := make([]string, 0, len(e.filters))
	for k := range e.filters {
		names = append(names, k)
	}
	return names
}

// Tests returns a snapshot of the env's registered test names.
func (e *Environment) Tests() []string {
	names := make([]string, 0, len(e.tests))
	for k := range e.tests {
		names = append(names, k)
	}
	return names
}

// AutoescapeFor returns the policy decision for the given template name.
// Implements [runtime.EvalContextEnvironment] and [eval.Engine].
func (e *Environment) AutoescapeFor(name string) bool {
	if e.autoesc == nil {
		return false
	}
	return e.autoesc.AutoescapeFor(name)
}

// Globals returns a snapshot of the env's globals map.
func (e *Environment) Globals() map[string]any {
	out := make(map[string]any, len(e.globals))
	for k, v := range e.globals {
		out[k] = v
	}
	return out
}

// Sandboxed reports whether sandbox checks are active.
func (e *Environment) Sandboxed() bool { return e.sandboxed }

// MaxRange returns the configured range cap.
func (e *Environment) MaxRange() int { return e.maxRange }

// FromString compiles a template from src, with no associated name.
func (e *Environment) FromString(src string) (*Template, error) {
	tpl, err := e.parse(src, "", "")
	if err != nil {
		return nil, err
	}
	return &Template{env: e, name: "", src: src, ast: tpl}, nil
}

// GetTemplate fetches the named template through the loader, parses it,
// and caches the AST. Lookup order: in-memory cache → externalCache (if
// configured, e.g. a *cache.Filesystem) → fresh parse.
func (e *Environment) GetTemplate(name string) (*Template, error) {
	e.mu.RLock()
	if t, ok := e.cache[name]; ok {
		e.mu.RUnlock()
		return &Template{env: e, name: name, ast: t}, nil
	}
	e.mu.RUnlock()
	if e.loader == nil {
		return nil, gjerrors.NewTemplateNotFound(name, "no loader configured")
	}
	src, err := e.loader.GetSource(name)
	if err != nil {
		return nil, err
	}
	// Try external cache (filesystem AST cache) before parsing.
	if e.externalCache != nil {
		key := makeCacheKey(name, src.Filename)
		if tpl, ok := e.externalCache.Get(key, sha256Hex(src.Code)); ok {
			e.storeAST(name, tpl)
			return &Template{env: e, name: name, ast: tpl}, nil
		}
	}
	tpl, err := e.parse(src.Code, name, src.Filename)
	if err != nil {
		return nil, err
	}
	if e.externalCache != nil {
		key := makeCacheKey(name, src.Filename)
		_ = e.externalCache.Put(key, sha256Hex(src.Code), tpl)
	}
	e.mu.Lock()
	if len(e.cache) >= e.maxCache {
		for k := range e.cache {
			delete(e.cache, k)
			break
		}
	}
	e.cache[name] = tpl
	e.mu.Unlock()
	return &Template{env: e, name: name, ast: tpl}, nil
}

// LoadTemplate satisfies [eval.Engine] — used by `{% include %}` and
// `{% extends %}`.
func (e *Environment) LoadTemplate(name string) (*ast.Template, error) {
	t, err := e.GetTemplate(name)
	if err != nil {
		return nil, err
	}
	return t.ast, nil
}

// UndefinedFactory satisfies [eval.Engine].
func (e *Environment) UndefinedFactory() runtime.UndefinedFactory {
	return e.undefined
}

// parse runs lexer + parser, returning the AST.
func (e *Environment) parse(src, name, filename string) (*ast.Template, error) {
	l, err := lexer.New(e.lexerOpts)
	if err != nil {
		return nil, err
	}
	stream, err := l.Tokenize(src, name, filename)
	if err != nil {
		return nil, err
	}
	p := parser.New(stream)
	for tag, fn := range e.extensions {
		p.RegisterExtension(tag, fn)
	}
	tpl, err := p.Parse()
	if err != nil {
		return nil, err
	}
	if err := e.validateNames(tpl, name, filename); err != nil {
		return nil, err
	}
	return tpl, nil
}

// validateNames mirrors Jinja2's compile-time check that every Filter
// and Test referenced in the AST has a registered implementation, with
// the explicit exception documented in Jinja2's compiler.py
// (`pull_dependencies`): "If the node is in an If or CondExpr node, the
// check is done at runtime instead." Jinja2 implements this by
// propagating a `soft_frame` flag inside If/CondExpr branches; that
// flag is reset whenever a For/Macro/Block/FilterBlock/CallBlock opens
// its own frame. We replicate the propagation so the parity surface is
// byte-for-byte identical: identical templates raise identical
// TemplateAssertionError messages and identical templates succeed in
// both engines.
func (e *Environment) validateNames(tpl *ast.Template, name, filename string) error {
	return validateNamesIn(tpl, e.filters, e.tests, name, filename, false)
}

func validateNamesIn(n ast.Node, filters map[string]Filter, tests map[string]Test, name, filename string, soft bool) error {
	switch x := n.(type) {
	case nil:
		return nil

	// Soft-frame creators. Jinja2's compiler visits the entire If
	// (including its test expression) and CondExpr (including the test
	// and both branches) under a `frame.soft()`, so EVERY filter/test
	// underneath is deferred to render time.
	case *ast.If:
		for _, c := range x.Body {
			if err := validateNamesIn(c, filters, tests, name, filename, true); err != nil {
				return err
			}
		}
		if err := validateNamesIn(x.Test, filters, tests, name, filename, true); err != nil {
			return err
		}
		for _, e := range x.Elif {
			if err := validateNamesIn(e, filters, tests, name, filename, true); err != nil {
				return err
			}
		}
		for _, c := range x.Else {
			if err := validateNamesIn(c, filters, tests, name, filename, true); err != nil {
				return err
			}
		}
		return nil
	case *ast.CondExpr:
		if err := validateNamesIn(x.Test, filters, tests, name, filename, true); err != nil {
			return err
		}
		if err := validateNamesIn(x.Expr1, filters, tests, name, filename, true); err != nil {
			return err
		}
		if x.Expr2 != nil {
			return validateNamesIn(x.Expr2, filters, tests, name, filename, true)
		}
		return nil

	// Hard-frame creators — their bodies open a fresh frame so any
	// inherited `soft` flag is dropped. (Mirrors `frame.inner()` in
	// Jinja2's compiler, which constructs a brand-new Frame.)
	case *ast.For:
		if err := validateNamesIn(x.Iter, filters, tests, name, filename, soft); err != nil {
			return err
		}
		if x.Test != nil {
			if err := validateNamesIn(x.Test, filters, tests, name, filename, false); err != nil {
				return err
			}
		}
		for _, c := range x.Body {
			if err := validateNamesIn(c, filters, tests, name, filename, false); err != nil {
				return err
			}
		}
		for _, c := range x.Else {
			if err := validateNamesIn(c, filters, tests, name, filename, false); err != nil {
				return err
			}
		}
		return nil
	case *ast.Macro:
		for _, d := range x.Defaults {
			if err := validateNamesIn(d, filters, tests, name, filename, soft); err != nil {
				return err
			}
		}
		for _, c := range x.Body {
			if err := validateNamesIn(c, filters, tests, name, filename, false); err != nil {
				return err
			}
		}
		return nil
	case *ast.CallBlock:
		if err := validateNamesIn(x.Call, filters, tests, name, filename, soft); err != nil {
			return err
		}
		for _, d := range x.Defaults {
			if err := validateNamesIn(d, filters, tests, name, filename, soft); err != nil {
				return err
			}
		}
		for _, c := range x.Body {
			if err := validateNamesIn(c, filters, tests, name, filename, false); err != nil {
				return err
			}
		}
		return nil
	case *ast.FilterBlock:
		// FilterBlock checks its trailing filter at compile time too.
		if x.Filter != nil {
			if err := validateNamesIn(x.Filter, filters, tests, name, filename, false); err != nil {
				return err
			}
		}
		for _, c := range x.Body {
			if err := validateNamesIn(c, filters, tests, name, filename, false); err != nil {
				return err
			}
		}
		return nil
	case *ast.Block:
		for _, c := range x.Body {
			if err := validateNamesIn(c, filters, tests, name, filename, false); err != nil {
				return err
			}
		}
		return nil

	case *ast.Filter:
		if !soft {
			if _, ok := filters[x.Name]; !ok {
				return gjerrors.NewTemplateAssertionError(
					fmt.Sprintf("No filter named %q.", x.Name),
					x.Position().Lineno, name, filename)
			}
		}
		// Filter sub-expressions inherit the parent's softness.
	case *ast.Test:
		if !soft {
			if _, ok := tests[x.Name]; !ok {
				return gjerrors.NewTemplateAssertionError(
					fmt.Sprintf("No test named %q.", x.Name),
					x.Position().Lineno, name, filename)
			}
		}
	}
	for _, c := range ast.Children(n) {
		if err := validateNamesIn(c, filters, tests, name, filename, soft); err != nil {
			return err
		}
	}
	return nil
}

// Template is a compiled template ready to render.
type Template struct {
	env  *Environment
	name string
	src  string
	ast  *ast.Template
}

// Name returns the name passed to GetTemplate, or "" for FromString.
func (t *Template) Name() string { return t.name }

// Render renders the template with the given variables and returns the
// output as a string. Equivalent to RenderContext(context.Background(), vars).
func (t *Template) Render(vars map[string]any) (string, error) {
	return t.RenderContext(context.Background(), vars)
}

// RenderContext renders with cancellation support. Inheritance is
// resolved before evaluation: any `{% extends %}` chain whose parent
// names are constants is followed statically so block overrides are
// layered correctly.
func (t *Template) RenderContext(ctx context.Context, vars map[string]any) (string, error) {
	parent := make(map[string]any, len(t.env.globals)+len(vars))
	for k, v := range t.env.globals {
		parent[k] = v
	}
	for k, v := range vars {
		parent[k] = v
	}
	ec := runtime.NewEvalContext(t.env, t.name)
	rctx := runtime.NewContext(t.name, t.env, parent, ec)

	chain, err := t.env.resolveInheritance(t.ast)
	if err != nil {
		return "", err
	}
	// Layer block overrides oldest-first (chain[0] is the root). The
	// child wins because its blocks are appended last.
	for _, lvl := range chain {
		for name, fn := range collectBlocks(t.env, lvl.Body) {
			rctx.Blocks[name] = append(rctx.Blocks[name], fn)
		}
	}
	// Render the child's body. Its `{% extends %}` statement (if any)
	// will recursively render the parent at the appropriate point. This
	// matches Python's behaviour: top-level statements in the child
	// before `{% extends %}` execute and emit; statements after it
	// continue to execute (so `{% set %}`/`{% macro %}` still take
	// effect) but their `{{ … }}` output is suppressed.
	var b strings.Builder
	if err := eval.Render(ctx, &b, t.ast, rctx, t.env); err != nil {
		// Already-typed TemplateSyntaxError carries its own location;
		// other errors get wrapped with the template context.
		var se *gjerrors.TemplateSyntaxError
		if t.name != "" && !asTemplateSyntaxError(err, &se) {
			return "", debug.Wrap(err, t.name, 0, t.src)
		}
		return "", err
	}
	return b.String(), nil
}

// storeAST inserts tpl into the in-memory cache, evicting if needed.
func (e *Environment) storeAST(name string, tpl *ast.Template) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.cache) >= e.maxCache {
		for k := range e.cache {
			delete(e.cache, k)
			break
		}
	}
	e.cache[name] = tpl
}

// makeCacheKey is a tiny replacement for cache.CacheKey to avoid an
// import cycle (env imports cache would force cache to not exist
// independently — kept symmetric).
func makeCacheKey(name, filename string) string {
	return sha256Hex(name + "|" + filename)[:32]
}

// sha256Hex returns the hex-encoded SHA-256 of s.
func sha256Hex(s string) string {
	h := sha256New()
	h.Write([]byte(s))
	return hexEncode(h.Sum(nil))
}

// asTemplateSyntaxError is a tiny shim around errors.As; declared here
// to avoid pulling errors.As into the template-name comparison above.
func asTemplateSyntaxError(err error, target **gjerrors.TemplateSyntaxError) bool {
	for cur := err; cur != nil; {
		if t, ok := cur.(*gjerrors.TemplateSyntaxError); ok {
			*target = t
			return true
		}
		type unwrapper interface{ Unwrap() error }
		u, ok := cur.(unwrapper)
		if !ok {
			return false
		}
		cur = u.Unwrap()
	}
	return false
}

// =========================================================== Engine impl

// CallFilter looks up name in the filter registry and invokes it.
func (e *Environment) CallFilter(name string, ctx *runtime.Context, value any, args []any, kwargs map[string]any) (any, error) {
	f, ok := e.filters[name]
	if !ok {
		return nil, gjerrors.NewFilterArgumentError(fmt.Sprintf("no filter named %q", name))
	}
	return f.Func(e, ctx, value, args, kwargs)
}

// CallTest looks up name in the test registry and invokes it.
func (e *Environment) CallTest(name string, ctx *runtime.Context, value any, args []any, kwargs map[string]any) (any, error) {
	t, ok := e.tests[name]
	if !ok {
		return nil, gjerrors.NewTemplateRuntimeError(fmt.Sprintf("no test named %q", name))
	}
	return t.Func(e, ctx, value, args, kwargs)
}

// KwargsCallable is the signature globals / filters / tests use when
// they need to receive Python keyword arguments. The eval layer
// recognises both this named type and the bare equivalent
// `func([]any, map[string]any) (any, error)` so callers can register
// either form.
type KwargsCallable func(args []any, kwargs map[string]any) (any, error)

// Call invokes a callable. The sandbox is consulted in Phase 19; for now,
// Go functions are dispatched through reflection.
func (e *Environment) Call(ctx *runtime.Context, callee any, args []any, kwargs map[string]any) (any, error) {
	if callee == nil {
		return nil, gjerrors.NewTemplateRuntimeError("cannot call nil")
	}
	// Sandbox-flagged callables refuse from the start.
	if e.sandboxed && !sandbox.IsSafeCallable(callee) {
		return nil, gjerrors.NewSecurityError("callable is not safe to invoke")
	}
	// Built-in callables.
	switch fn := callee.(type) {
	case *runtime.Joiner:
		return fn.Call(), nil
	case runtime.CallerFunc:
		return fn(args, kwargs)
	case func() (any, error):
		// `super()` binding — see eval.callBlockAtDepth.
		return fn()
	case KwargsCallable:
		return fn(args, kwargs)
	case func(args []any, kwargs map[string]any) (any, error):
		return fn(args, kwargs)
	}
	// Handle native Go funcs.
	v := reflect.ValueOf(callee)
	if v.Kind() != reflect.Func {
		return nil, gjerrors.NewTemplateRuntimeError(fmt.Sprintf("value of type %T is not callable", callee))
	}
	t := v.Type()
	if len(kwargs) != 0 {
		return nil, gjerrors.NewTemplateRuntimeError("keyword arguments are not supported on Go callables")
	}
	variadic := t.IsVariadic()
	fixed := t.NumIn()
	if variadic {
		fixed--
		if len(args) < fixed {
			return nil, gjerrors.NewTemplateRuntimeError(fmt.Sprintf("function expects at least %d args, got %d", fixed, len(args)))
		}
	} else if t.NumIn() != len(args) {
		return nil, gjerrors.NewTemplateRuntimeError(fmt.Sprintf("function expects %d args, got %d", t.NumIn(), len(args)))
	}
	in := make([]reflect.Value, 0, len(args))
	for i := 0; i < fixed; i++ {
		v, err := convertArg(args[i], t.In(i))
		if err != nil {
			return nil, err
		}
		in = append(in, v)
	}
	if variadic {
		elem := t.In(fixed).Elem()
		for i := fixed; i < len(args); i++ {
			v, err := convertArg(args[i], elem)
			if err != nil {
				return nil, err
			}
			in = append(in, v)
		}
	}
	out := v.Call(in)
	switch len(out) {
	case 0:
		return nil, nil
	case 1:
		return out[0].Interface(), nil
	case 2:
		// (value, error) convention.
		if errIfc, ok := out[1].Interface().(error); ok && errIfc != nil {
			return nil, errIfc
		}
		return out[0].Interface(), nil
	}
	return nil, gjerrors.NewTemplateRuntimeError("Go callable returned more than 2 values")
}

// GetAttr fetches obj.attr through the sandbox-aware predicate.
func (e *Environment) GetAttr(obj any, attr string) (any, error) {
	// Built-in types with custom attribute resolution.
	switch x := obj.(type) {
	case *runtime.Cycler:
		if v, ok := x.Get(attr); ok {
			return v, nil
		}
	case *runtime.LoopContext:
		if v, ok := x.Get(attr); ok {
			return v, nil
		}
	case *runtime.Namespace:
		if v, ok := x.Get(attr); ok {
			return v, nil
		}
	case map[string]any:
		// Python parity: `d.foo` checks for an attribute (synthetic
		// dict method) FIRST, then falls back to a key. Use `d['foo']`
		// to skip the method and go straight to a key.
		if m := dictMethod(x, attr); m != nil {
			return m, nil
		}
		if v, ok := x[attr]; ok {
			return v, nil
		}
	case map[any]any:
		if m := dictMethod(x, attr); m != nil {
			return m, nil
		}
		if v, ok := x[attr]; ok {
			return v, nil
		}
	case *runtime.OrderedDict:
		if m := dictMethod(x, attr); m != nil {
			return m, nil
		}
		if v, ok := x.Get(attr); ok {
			return v, nil
		}
	case string:
		if m := stringMethod(x, attr); m != nil {
			return m, nil
		}
	case escape.Markup:
		if m := stringMethod(string(x), attr); m != nil {
			return m, nil
		}
	case []any:
		if m := listMethod(x, attr); m != nil {
			return m, nil
		}
	}
	if u, ok := obj.(runtime.Undefiner); ok && u.IsUndefined() {
		// Calling .x on Undefined: chainable returns self; others fail.
		if und, ok := obj.(runtime.Undefined); ok {
			return und.GetAttr(attr)
		}
	}
	if obj == nil {
		return e.undefined("", attr, nil, nil), nil
	}
	v := reflect.ValueOf(obj)
	for v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return e.undefined("", attr, nil, nil), nil
		}
		v = v.Elem()
	}
	if v.Kind() == reflect.Struct {
		if e.sandboxed && !sandbox.IsSafeAttribute(obj, attr) {
			return nil, gjerrors.NewSecurityError(fmt.Sprintf("access to attribute %q is forbidden", attr))
		}
		f := v.FieldByName(attr)
		if f.IsValid() && f.CanInterface() {
			return f.Interface(), nil
		}
		// Loop variable special-case.
		if lc, ok := obj.(*runtime.LoopContext); ok {
			if got, ok := lc.Get(attr); ok {
				return got, nil
			}
		}
	}
	if v.Kind() == reflect.Map {
		key := reflect.ValueOf(attr)
		if v.Type().Key() == key.Type() {
			it := v.MapIndex(key)
			if it.IsValid() {
				return it.Interface(), nil
			}
		}
	}
	if lc, ok := obj.(*runtime.LoopContext); ok {
		if got, ok := lc.Get(attr); ok {
			return got, nil
		}
	}
	return e.undefined("", attr, obj, nil), nil
}

// GetItem fetches obj[arg].
func (e *Environment) GetItem(obj any, arg any) (any, error) {
	if obj == nil {
		return e.undefined("", "", nil, nil), nil
	}
	if u, ok := obj.(runtime.Undefiner); ok && u.IsUndefined() {
		if und, ok := obj.(runtime.Undefined); ok {
			return und.GetItem(arg)
		}
	}
	switch x := obj.(type) {
	case map[string]any:
		if k, ok := arg.(string); ok {
			if v, ok := x[k]; ok {
				return v, nil
			}
		}
		return e.undefined("", fmt.Sprintf("%v", arg), obj, nil), nil
	case map[any]any:
		if v, ok := x[arg]; ok {
			return v, nil
		}
		return e.undefined("", fmt.Sprintf("%v", arg), obj, nil), nil
	case *runtime.OrderedDict:
		if v, ok := x.Get(arg); ok {
			return v, nil
		}
		return e.undefined("", fmt.Sprintf("%v", arg), obj, nil), nil
	case []any:
		if sv, ok := arg.(ast.SliceVal); ok {
			return sliceList(x, sv), nil
		}
		idx, ok := asInt(arg)
		if !ok {
			return nil, gjerrors.NewTemplateRuntimeError(fmt.Sprintf("list indices must be integers, not %T", arg))
		}
		if idx < 0 {
			idx += len(x)
		}
		if idx < 0 || idx >= len(x) {
			return e.undefined("", fmt.Sprintf("%v", arg), obj, nil), nil
		}
		return x[idx], nil
	case runtime.Tuple:
		if sv, ok := arg.(ast.SliceVal); ok {
			out := sliceList([]any(x), sv)
			return runtime.Tuple(out), nil
		}
		idx, ok := asInt(arg)
		if !ok {
			return nil, gjerrors.NewTemplateRuntimeError(fmt.Sprintf("tuple indices must be integers, not %T", arg))
		}
		if idx < 0 {
			idx += len(x)
		}
		if idx < 0 || idx >= len(x) {
			return e.undefined("", fmt.Sprintf("%v", arg), obj, nil), nil
		}
		return x[idx], nil
	case string:
		if sv, ok := arg.(ast.SliceVal); ok {
			return sliceString(x, sv), nil
		}
		idx, ok := asInt(arg)
		if !ok {
			// String indexed by string falls back to attribute access.
			if k, ok := arg.(string); ok {
				return e.GetAttr(obj, k)
			}
			return nil, gjerrors.NewTemplateRuntimeError(fmt.Sprintf("string indices must be integers, not %T", arg))
		}
		if idx < 0 {
			idx += len(x)
		}
		if idx < 0 || idx >= len(x) {
			return e.undefined("", fmt.Sprintf("%v", arg), obj, nil), nil
		}
		return string(x[idx]), nil
	}
	// Fallback: try attribute access.
	if k, ok := arg.(string); ok {
		return e.GetAttr(obj, k)
	}
	return e.undefined("", fmt.Sprintf("%v", arg), obj, nil), nil
}

// convertArg coerces a template-supplied argument to the destination
// reflect type. Used by the Go-callable bridge.
func convertArg(a any, dst reflect.Type) (reflect.Value, error) {
	if a == nil {
		return reflect.Zero(dst), nil
	}
	av := reflect.ValueOf(a)
	if av.Type().AssignableTo(dst) {
		return av, nil
	}
	if av.Type().ConvertibleTo(dst) {
		return av.Convert(dst), nil
	}
	return reflect.Value{}, gjerrors.NewTemplateRuntimeError(fmt.Sprintf("cannot pass %T as %s", a, dst))
}

// asInt is a tiny duplicate of the helper in pkg/eval (kept here to
// avoid a cycle).
func asInt(v any) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case int64:
		return int(x), true
	case float64:
		if x == float64(int(x)) {
			return int(x), true
		}
	}
	return 0, false
}

// sliceList implements Python's `list[start:stop:step]` semantics.
// Negative indices are interpreted from the end; nil ends mean
// "to/from the boundary"; negative step iterates backwards.
func sliceList(items []any, sv ast.SliceVal) []any {
	start, stop, step := normaliseSlice(len(items), sv)
	if step == 0 {
		return nil
	}
	out := []any{}
	if step > 0 {
		for i := start; i < stop; i += step {
			out = append(out, items[i])
		}
	} else {
		for i := start; i > stop; i += step {
			out = append(out, items[i])
		}
	}
	return out
}

// sliceString slices a string Python-style. The unit is the byte
// (matching Python slicing on str: it's by code-point — for our parity
// purposes the corpus uses ASCII).
func sliceString(s string, sv ast.SliceVal) string {
	rs := []rune(s)
	start, stop, step := normaliseSlice(len(rs), sv)
	if step == 0 {
		return ""
	}
	var b strings.Builder
	if step > 0 {
		for i := start; i < stop; i += step {
			b.WriteRune(rs[i])
		}
	} else {
		for i := start; i > stop; i += step {
			b.WriteRune(rs[i])
		}
	}
	return b.String()
}

// normaliseSlice resolves a SliceVal into concrete (start, stop, step)
// indices for a sequence of length n, applying Python's edge rules.
func normaliseSlice(n int, sv ast.SliceVal) (int, int, int) {
	step := 1
	if sv.Step != nil {
		if v, ok := asInt(sv.Step); ok {
			step = v
		}
	}
	if step == 0 {
		return 0, 0, 0
	}
	var start, stop int
	if step > 0 {
		start = 0
		stop = n
	} else {
		start = n - 1
		stop = -1
	}
	if sv.Start != nil {
		if v, ok := asInt(sv.Start); ok {
			start = v
			if start < 0 {
				start += n
			}
			if step > 0 {
				if start < 0 {
					start = 0
				}
				if start > n {
					start = n
				}
			} else {
				if start < -1 {
					start = -1
				}
				if start >= n {
					start = n - 1
				}
			}
		}
	}
	if sv.Stop != nil {
		if v, ok := asInt(sv.Stop); ok {
			stop = v
			if stop < 0 {
				stop += n
			}
			if step > 0 {
				if stop < 0 {
					stop = 0
				}
				if stop > n {
					stop = n
				}
			} else {
				if stop < -1 {
					stop = -1
				}
				if stop >= n {
					stop = n - 1
				}
			}
		}
	}
	return start, stop, step
}

