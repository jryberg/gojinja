package environment

import (
	"context"
	"strings"

	"github.com/jryberg/gojinja/pkg/ast"
	gjerrors "github.com/jryberg/gojinja/pkg/errors"
	"github.com/jryberg/gojinja/pkg/eval"
	"github.com/jryberg/gojinja/pkg/runtime"
)

// resolveInheritance walks an Extends chain at static-analysis time. It
// returns the chain ordered oldest-ancestor-first (so chain[0] is the
// root that should actually be rendered) and the per-name list of block
// override functions, also oldest-first.
//
// Extends with a non-constant template name (e.g. {% extends parent_name
// %}) is *not* statically resolved; in that case we return chain=[t] and
// let the evaluator's evalExtends drive the dynamic case at render time.
func (e *Environment) resolveInheritance(t *ast.Template) (chain []*ast.Template, err error) {
	chain = []*ast.Template{t}
	cur := t
	visited := map[string]bool{}
	for {
		var ext *ast.Extends
		for _, n := range cur.Body {
			if x, ok := n.(*ast.Extends); ok {
				ext = x
				break
			}
		}
		if ext == nil {
			break
		}
		// Only static names (Const string) are pre-resolved here.
		c, ok := ext.Template.(*ast.Const)
		if !ok {
			break
		}
		name, ok := c.Value.(string)
		if !ok {
			break
		}
		if visited[name] {
			return nil, gjerrors.NewTemplateRuntimeError("circular template inheritance: " + name)
		}
		visited[name] = true
		parent, err := e.LoadTemplate(name)
		if err != nil {
			return nil, err
		}
		chain = append([]*ast.Template{parent}, chain...)
		cur = parent
	}
	return chain, nil
}

// collectBlocks finds every {% block %} defined directly in body and
// returns them as render closures. Nested blocks (inside other blocks)
// are not collected — they're rendered as part of their containing block.
func collectBlocks(env *Environment, body []ast.Node) map[string]runtime.BlockRenderFunc {
	out := map[string]runtime.BlockRenderFunc{}
	walkBlocks(env, body, out)
	return out
}

func walkBlocks(env *Environment, body []ast.Node, out map[string]runtime.BlockRenderFunc) {
	for _, n := range body {
		if b, ok := n.(*ast.Block); ok {
			block := b
			out[b.Name] = func(rctx *runtime.Context) (string, error) {
				var sb strings.Builder
				if err := eval.Render(context.Background(), &sb, &ast.Template{Body: block.Body}, rctx, env); err != nil {
					return "", err
				}
				return sb.String(), nil
			}
			// Don't recurse into the block — its inner blocks render with the
			// outer block's body.
			continue
		}
		// Walk into wrapper Stmts that can contain other Blocks at the
		// top level (Scope, OverlayScope, ScopedEvalContextModifier, If).
		switch x := n.(type) {
		case *ast.Scope:
			walkBlocks(env, x.Body, out)
		case *ast.OverlayScope:
			walkBlocks(env, x.Body, out)
		case *ast.ScopedEvalContextModifier:
			walkBlocks(env, x.Body, out)
		case *ast.If:
			walkBlocks(env, x.Body, out)
			for _, e := range x.Elif {
				walkBlocks(env, e.Body, out)
			}
			walkBlocks(env, x.Else, out)
		}
	}
}
