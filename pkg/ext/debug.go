package ext

import (
	"fmt"
	"sort"
	"strings"

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
// dump needs (filter and test names). Defining it here as an interface
// avoids an import cycle.
type EnvLister interface {
	Filters() []string
	Tests() []string
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

// DebugRender is the default helper bound by [DebugGlobalName]. It dumps
// context vars (sorted), filter names, and test names in a Python-pprint
// style — matching Jinja2's DebugExtension output close enough that
// scrapers looking for `'context'`, `'filters'`, `'tests'` and named
// entries find them.
func DebugRender(ctx *runtime.Context) (string, error) {
	if ctx == nil {
		return "", fmt.Errorf("debug: nil context")
	}
	var b strings.Builder
	vars := ctx.All()
	names := sortedKeys(vars)
	b.WriteString("{'context': {")
	for i, k := range names {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "'%s': %s", k, formatValue(vars[k]))
	}
	b.WriteString("},\n 'filters': [")
	if env, ok := ctx.Env.(EnvLister); ok {
		fs := env.Filters()
		sort.Strings(fs)
		for i, f := range fs {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "'%s'", f)
		}
	}
	b.WriteString("],\n 'tests': [")
	if env, ok := ctx.Env.(EnvLister); ok {
		ts := env.Tests()
		sort.Strings(ts)
		for i, tt := range ts {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "'%s'", tt)
		}
	}
	b.WriteString("]}")
	return b.String(), nil
}

func sortedKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func formatValue(v any) string {
	if v == nil {
		return "None"
	}
	switch x := v.(type) {
	case string:
		return fmt.Sprintf("%q", x)
	case bool:
		if x {
			return "True"
		}
		return "False"
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", x)
	case float32, float64:
		return fmt.Sprintf("%v", x)
	}
	return fmt.Sprintf("<%T>", v)
}
