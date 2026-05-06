package optimizer

import (
	"testing"

	"github.com/jryberg/gojinja/pkg/ast"
	"github.com/jryberg/gojinja/pkg/escape"
)

func TestFoldsArithmetic(t *testing.T) {
	expr := &ast.Add{}
	expr.Op = "+"
	expr.Left = &ast.Const{Value: int64(1)}
	expr.Right = &ast.Const{Value: int64(2)}
	r := Optimize(expr, ast.EvalCtx{}).(ast.Expr)
	c, ok := r.(*ast.Const)
	if !ok {
		t.Fatalf("expected Const, got %T", r)
	}
	if c.Value != int64(3) {
		t.Fatalf("expected 3, got %v", c.Value)
	}
}

func TestFoldsNestedArithmetic(t *testing.T) {
	// (2 * 3) + (10 - 4) = 12
	left := &ast.Mul{}
	left.Op = "*"
	left.Left = &ast.Const{Value: int64(2)}
	left.Right = &ast.Const{Value: int64(3)}

	right := &ast.Sub{}
	right.Op = "-"
	right.Left = &ast.Const{Value: int64(10)}
	right.Right = &ast.Const{Value: int64(4)}

	root := &ast.Add{}
	root.Op = "+"
	root.Left = left
	root.Right = right

	r := Optimize(root, ast.EvalCtx{}).(*ast.Const)
	if r.Value != int64(12) {
		t.Fatalf("expected 12, got %v", r.Value)
	}
}

func TestDoesNotFoldDivByZero(t *testing.T) {
	expr := &ast.Div{}
	expr.Op = "/"
	expr.Left = &ast.Const{Value: int64(1)}
	expr.Right = &ast.Const{Value: int64(0)}
	r := Optimize(expr, ast.EvalCtx{})
	if _, ok := r.(*ast.Const); ok {
		t.Fatalf("division by zero should not fold")
	}
	if _, ok := r.(*ast.Div); !ok {
		t.Fatalf("expected Div untouched, got %T", r)
	}
}

func TestDoesNotFoldName(t *testing.T) {
	tmpl := &ast.Template{Body: []ast.Node{
		&ast.Output{Nodes: []ast.Expr{&ast.Name{Name: "x", Ctx: ast.CtxLoad}}},
	}}
	Optimize(tmpl, ast.EvalCtx{})
	out := tmpl.Body[0].(*ast.Output)
	if _, ok := out.Nodes[0].(*ast.Name); !ok {
		t.Fatalf("Name should remain, got %T", out.Nodes[0])
	}
}

func TestDoesNotFoldFilter(t *testing.T) {
	// `name | upper` references `name` so should not fold.
	f := &ast.Filter{
		Node: &ast.Name{Name: "x", Ctx: ast.CtxLoad},
		Name: "upper",
	}
	r := Optimize(f, ast.EvalCtx{})
	if _, ok := r.(*ast.Const); ok {
		t.Fatalf("filter on Name should not fold")
	}
}

func TestDoesNotFoldGetattr(t *testing.T) {
	g := &ast.Getattr{Node: &ast.Name{Name: "obj", Ctx: ast.CtxLoad}, Attr: "field"}
	r := Optimize(g, ast.EvalCtx{})
	if _, ok := r.(*ast.Const); ok {
		t.Fatalf("getattr should not fold")
	}
}

func TestFoldsConcat(t *testing.T) {
	c := &ast.Concat{Nodes: []ast.Expr{
		&ast.Const{Value: "ab"},
		&ast.Const{Value: int64(42)},
	}}
	r := Optimize(c, ast.EvalCtx{}).(*ast.Const)
	if r.Value != "ab42" {
		t.Fatalf("expected 'ab42', got %v", r.Value)
	}
}

func TestFoldsTemplateDataAutoescape(t *testing.T) {
	// Autoescape on → folds to Markup.
	td := &ast.TemplateData{Data: "<b>"}
	r := Optimize(td, ast.EvalCtx{Autoescape: true}).(*ast.Const)
	m, ok := r.Value.(escape.Markup)
	if !ok {
		t.Fatalf("expected Markup, got %T", r.Value)
	}
	if string(m) != "<b>" {
		t.Fatalf("expected Markup(<b>), got %q", m)
	}
}

func TestVolatileBlocksTemplateDataFold(t *testing.T) {
	td := &ast.TemplateData{Data: "<b>"}
	r := Optimize(td, ast.EvalCtx{Autoescape: true, Volatile: true})
	if _, ok := r.(*ast.Const); ok {
		t.Fatalf("volatile context must keep TemplateData unfolded")
	}
}

func TestFoldsCondExprPickingTrueBranch(t *testing.T) {
	c := &ast.CondExpr{
		Test:  &ast.Const{Value: true},
		Expr1: &ast.Const{Value: "yes"},
		Expr2: &ast.Const{Value: "no"},
	}
	r := Optimize(c, ast.EvalCtx{}).(*ast.Const)
	if r.Value != "yes" {
		t.Fatalf("expected 'yes', got %v", r.Value)
	}
}

func TestFoldsListLiteral(t *testing.T) {
	l := &ast.List{Items: []ast.Expr{
		mulConst(2, 3),
		&ast.Const{Value: int64(7)},
	}}
	r := Optimize(l, ast.EvalCtx{}).(*ast.Const)
	got := r.Value.([]any)
	if len(got) != 2 || got[0] != int64(6) || got[1] != int64(7) {
		t.Fatalf("unexpected list value: %v", got)
	}
}

func TestPreservesLineNumber(t *testing.T) {
	expr := &ast.Add{}
	expr.Op = "+"
	expr.SetLineno(42)
	expr.Left = &ast.Const{Value: int64(1)}
	expr.Right = &ast.Const{Value: int64(2)}
	r := Optimize(expr, ast.EvalCtx{}).(*ast.Const)
	if got := r.Position().Lineno; got != 42 {
		t.Fatalf("expected line 42, got %d", got)
	}
}

func mulConst(a, b int64) ast.Expr {
	m := &ast.Mul{}
	m.Op = "*"
	m.Left = &ast.Const{Value: a}
	m.Right = &ast.Const{Value: b}
	return m
}
