package ext

import (
	"fmt"
	"sort"

	"github.com/jryberg/gojinja/pkg/ast"
	"github.com/jryberg/gojinja/pkg/parser"
	"github.com/jryberg/gojinja/pkg/runtime"
)

// DebugGlobalName is the synthetic global the {% debug %} tag expands to.
// Callers wiring the debug extension by hand must register a function
// satisfying [DebugFunc] under this name in env globals; the [Debug]
// helper does that for the standard environment.
const DebugGlobalName = "__gojinja_debug__"

// DebugFunc is the signature of the helper invoked by `{% debug %}`. It
// receives the active runtime context and returns the dump.
type DebugFunc func(ctx *runtime.Context) (string, error)

// EnvLister is the slice of *environment.Environment behaviour the debug
// dump needs. Defining it here as an interface avoids an import cycle.
type EnvLister interface {
	Filters() []string
	Tests() []string
	// Pformat is Python's pprint.pformat with the given depth and compact.
	Pformat(v any, depth int, compact bool) string
}

// DebugTag is the parser hook for `{% debug %}`. It expands to an
// [ast.Output] that calls the helper registered under [DebugGlobalName]
// with a [ast.ContextReference] argument.
func DebugTag(p *parser.Parser) (ast.Node, error) {
	tok := p.Stream().Next()
	if tok.Value != "debug" {
		return nil, fmt.Errorf("ext.DebugTag: expected 'debug' tag, got %q", tok.Value)
	}
	call := &ast.Call{
		Node: &ast.Name{Name: DebugGlobalName, Ctx: ast.CtxLoad},
		Args: []ast.Expr{&ast.ContextReference{}},
	}
	call.SetLineno(tok.Lineno)
	out := &ast.Output{Nodes: []ast.Expr{call}}
	out.SetLineno(tok.Lineno)
	return out, nil
}

// DebugRender is the default helper bound by [DebugGlobalName], a port of
// Jinja2's DebugExtension._render: the context's names, and the sorted
// filter and test names, as pprint.pformat(result, depth=3, compact=True).
func DebugRender(ctx *runtime.Context) (string, error) {
	if ctx == nil {
		return "", fmt.Errorf("debug: nil context")
	}
	env, ok := ctx.Env.(EnvLister)
	if !ok {
		return "", fmt.Errorf("debug: the environment cannot list its filters and tests")
	}
	vars := runtime.NewOrderedDict()
	for k, v := range ctx.All() {
		if k != DebugGlobalName {
			vars.Set(k, v)
		}
	}
	names := func(list []string) []any {
		sort.Strings(list)
		out := make([]any, len(list))
		for i, n := range list {
			out[i] = n
		}
		return out
	}
	result := runtime.NewOrderedDict()
	result.Set("context", vars)
	result.Set("filters", names(env.Filters()))
	result.Set("tests", names(env.Tests()))
	return env.Pformat(result, 3, true), nil
}
