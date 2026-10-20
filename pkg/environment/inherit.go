package environment

import (
	"context"
	"strings"

	"github.com/jryberg/gojinja/pkg/ast"
	"github.com/jryberg/gojinja/pkg/eval"
	"github.com/jryberg/gojinja/pkg/runtime"
)

// TemplateBlocks returns the block stacks t starts rendering with: one
// entry per block t defines. An executed {% extends %} later adds the
// parent's blocks beneath them, as Jinja2 does.
func (e *Environment) TemplateBlocks(t *ast.Template) (map[string][]runtime.BlockRenderFunc, error) {
	blocks := map[string][]runtime.BlockRenderFunc{}
	for name, fn := range collectBlocks(e, t.Body) {
		blocks[name] = []runtime.BlockRenderFunc{fn}
	}
	return blocks, nil
}

// collectBlocks returns a render closure for every {% block %} in body,
// at any depth: Jinja2 registers blocks inside loops, conditionals and
// other blocks too, so a child template can override each of them.
func collectBlocks(env *Environment, body []ast.Node) map[string]runtime.BlockRenderFunc {
	out := map[string]runtime.BlockRenderFunc{}
	var walk func(n ast.Node)
	walk = func(n ast.Node) {
		if b, ok := n.(*ast.Block); ok && b.Required {
			out[b.Name] = eval.RequiredBlock(b.Name)
		} else if ok {
			out[b.Name] = func(rctx *runtime.Context) (string, error) {
				var sb strings.Builder
				if err := eval.Render(context.Background(), &sb, &ast.Template{Body: b.Body}, rctx, env); err != nil {
					return "", err
				}
				return sb.String(), nil
			}
		}
		for _, c := range ast.Children(n) {
			walk(c)
		}
	}
	for _, n := range body {
		walk(n)
	}
	return out
}
