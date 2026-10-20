package runtime

import "errors"

// EvalContextEnvironment is the slice of the Environment that an EvalContext
// needs in order to compute its initial flags. Defining it as an interface
// breaks the runtime → environment package cycle.
type EvalContextEnvironment interface {
	// AutoescapeFor returns true if rendering a template with the given
	// (possibly empty) name should default to autoescape on. The argument
	// is the template's name; "" means "rendered from a string".
	AutoescapeFor(name string) bool
}

// EvalContext holds eval-time flags that can be flipped by AST nodes during
// rendering — most notably autoescape and its volatility. Mirror of
// jinja2.nodes.EvalContext.
//
// Volatile means "the autoescape state at this point in the AST is decided
// at runtime, not at compile time" — set by extensions or
// {% autoescape %} blocks whose argument is a non-constant expression.
type EvalContext struct {
	Env        EvalContextEnvironment
	Autoescape bool
	Volatile   bool
	Name       string
	// depth counts the nested template calls (macros, callers, blocks,
	// includes, imports, recursive loops) of the render; a pointer, so
	// Save/Revert copies share it.
	depth *int
	// modules holds the render's imported modules, by template name, so
	// importing a template again without context gives the same module.
	modules *map[string]*TemplateModule
}

// For is the eval context of the template name rendered within e's
// render, as an include or import is: its own autoescape setting, and
// e's call depth and imported modules.
func (e *EvalContext) For(name string) *EvalContext {
	if e == nil {
		return nil
	}
	c := *e
	c.Name, c.Volatile = name, false
	if c.Env != nil {
		c.Autoescape = c.Env.AutoescapeFor(name)
	}
	return &c
}

// Module returns the module of the template name imported earlier in
// the render without context.
func (e *EvalContext) Module(name string) (*TemplateModule, bool) {
	if e.modules == nil || *e.modules == nil {
		return nil, false
	}
	m, ok := (*e.modules)[name]
	return m, ok
}

// SetModule records the module of the template name.
func (e *EvalContext) SetModule(name string, m *TemplateModule) {
	if e.modules == nil {
		e.modules = new(map[string]*TemplateModule)
	}
	if *e.modules == nil {
		*e.modules = map[string]*TemplateModule{}
	}
	(*e.modules)[name] = m
}

var errRecursion = errors.New("maximum recursion depth exceeded")

// MaxCallDepth bounds nested template calls in one render. Python stops
// such recursion with RecursionError (an include chain gets about 990
// levels deep, macros about 200); unbounded, it would overflow the Go
// stack and crash the process.
const MaxCallDepth = 1000

// Enter records one more nested template call, failing like Python's
// RecursionError past [MaxCallDepth]. Every successful Enter must be
// paired with a [EvalContext.Leave].
func (e *EvalContext) Enter() error {
	if e.depth == nil {
		e.depth = new(int)
	}
	if *e.depth >= MaxCallDepth {
		return errRecursion
	}
	*e.depth++
	return nil
}

// Leave ends a call recorded by [EvalContext.Enter].
func (e *EvalContext) Leave() { *e.depth-- }

// NewEvalContext constructs an EvalContext for the given (env, template
// name) pair. env may be nil for tests; when nil, Autoescape defaults to
// false (gojinja's safer default is normally enforced at the Environment
// level, not at this struct).
func NewEvalContext(env EvalContextEnvironment, name string) *EvalContext {
	ec := &EvalContext{Env: env, Name: name, depth: new(int), modules: new(map[string]*TemplateModule)}
	if env != nil {
		ec.Autoescape = env.AutoescapeFor(name)
	}
	return ec
}

// Save returns a value-copy of e. Use with [EvalContext.Revert] to scope a
// modification to a sub-tree of the AST.
func (e *EvalContext) Save() EvalContext {
	return *e
}

// Revert restores e to a previously [Save]-d state.
func (e *EvalContext) Revert(saved EvalContext) {
	*e = saved
}
