package idtracking

import (
	"reflect"
	"sort"
	"testing"

	"github.com/jryberg/gojinja/pkg/ast"
)

// Functional copies of the five tests in jinja2/tests/test_idtracking.py.

func TestBasics(t *testing.T) {
	forLoop := &ast.For{
		Target: &ast.Name{Name: "foo", Ctx: ast.CtxStore},
		Iter:   &ast.Name{Name: "seq", Ctx: ast.CtxLoad},
		Body: []ast.Node{
			&ast.Output{Nodes: []ast.Expr{&ast.Name{Name: "foo", Ctx: ast.CtxLoad}}},
		},
	}
	tmpl := &ast.Template{Body: []ast.Node{
		&ast.Assign{Target: &ast.Name{Name: "foo", Ctx: ast.CtxStore}, Node: &ast.Name{Name: "bar", Ctx: ast.CtxLoad}},
		forLoop,
	}}

	sym := SymbolsForNode(tmpl, nil)
	wantRefs := map[string]string{
		"foo": "l_0_foo",
		"bar": "l_0_bar",
		"seq": "l_0_seq",
	}
	if !reflect.DeepEqual(sym.Refs, wantRefs) {
		t.Errorf("template refs mismatch:\n got %v\nwant %v", sym.Refs, wantRefs)
	}
	wantLoads := map[string]Load{
		"l_0_foo": {Kind: LoadUndefined},
		"l_0_bar": {Kind: LoadResolve, Arg: "bar"},
		"l_0_seq": {Kind: LoadResolve, Arg: "seq"},
	}
	if !reflect.DeepEqual(sym.Loads, wantLoads) {
		t.Errorf("template loads mismatch:\n got %v\nwant %v", sym.Loads, wantLoads)
	}

	forSym := SymbolsForNode(forLoop, sym)
	if got, want := forSym.Refs, map[string]string{"foo": "l_1_foo"}; !reflect.DeepEqual(got, want) {
		t.Errorf("for refs mismatch:\n got %v\nwant %v", got, want)
	}
	if got, want := forSym.Loads, map[string]Load{"l_1_foo": {Kind: LoadParameter}}; !reflect.DeepEqual(got, want) {
		t.Errorf("for loads mismatch:\n got %v\nwant %v", got, want)
	}
}

func TestComplex(t *testing.T) {
	titleBlock := &ast.Block{
		Name: "title",
		Body: []ast.Node{
			&ast.Output{Nodes: []ast.Expr{&ast.TemplateData{Data: "Page Title"}}},
		},
	}

	renderTitleMacro := &ast.Macro{
		Name: "render_title",
		Args: []*ast.Name{{Name: "title", Ctx: ast.CtxParam}},
		Body: []ast.Node{
			&ast.Output{Nodes: []ast.Expr{
				&ast.TemplateData{Data: "\n  <div class=\"title\">\n    <h1>"},
				&ast.Name{Name: "title", Ctx: ast.CtxLoad},
				&ast.TemplateData{Data: "</h1>\n    <p>"},
				&ast.Name{Name: "subtitle", Ctx: ast.CtxLoad},
				&ast.TemplateData{Data: "</p>\n    "},
			}},
			&ast.Assign{
				Target: &ast.Name{Name: "subtitle", Ctx: ast.CtxStore},
				Node:   &ast.Const{Value: "something else"},
			},
			&ast.Output{Nodes: []ast.Expr{
				&ast.TemplateData{Data: "\n    <p>"},
				&ast.Name{Name: "subtitle", Ctx: ast.CtxLoad},
				&ast.TemplateData{Data: "</p>\n  </div>\n"},
			}},
			// In Python the If lives inside the preceding Output's
			// nodes list (Python's dynamic typing allows it). Our Output
			// is strictly typed, so the If is a sibling here. Symbol
			// tracking is order-independent for the assertions below.
			ifExpr(
				&ast.Name{Name: "something", Ctx: ast.CtxLoad},
				[]ast.Node{
					&ast.Assign{
						Target: &ast.Name{Name: "title_upper", Ctx: ast.CtxStore},
						Node: &ast.Filter{
							Node: &ast.Name{Name: "title", Ctx: ast.CtxLoad},
							Name: "upper",
						},
					},
					&ast.Output{Nodes: []ast.Expr{
						&ast.Name{Name: "title_upper", Ctx: ast.CtxLoad},
						&ast.Call{
							Node: &ast.Name{Name: "render_title", Ctx: ast.CtxLoad},
							Args: []ast.Expr{&ast.Const{Value: "Aha"}},
						},
					}},
				},
				nil,
				nil,
			),
		},
	}

	forLoop := &ast.For{
		Target: &ast.Name{Name: "item", Ctx: ast.CtxStore},
		Iter:   &ast.Name{Name: "seq", Ctx: ast.CtxLoad},
		Body: []ast.Node{
			&ast.Output{Nodes: []ast.Expr{
				&ast.TemplateData{Data: "\n    <li>"},
				&ast.Name{Name: "item", Ctx: ast.CtxLoad},
				&ast.TemplateData{Data: "</li>\n    <span>"},
			}},
			&ast.Include{Template: &ast.Const{Value: "helper.html"}, WithContext: true},
			&ast.Output{Nodes: []ast.Expr{&ast.TemplateData{Data: "</span>\n  "}}},
		},
	}

	bodyBlock := &ast.Block{
		Name: "body",
		Body: []ast.Node{
			&ast.Output{Nodes: []ast.Expr{
				&ast.TemplateData{Data: "\n  "},
				&ast.Call{
					Node: &ast.Name{Name: "render_title", Ctx: ast.CtxLoad},
					Args: []ast.Expr{&ast.Name{Name: "item", Ctx: ast.CtxLoad}},
				},
				&ast.TemplateData{Data: "\n  <ul>\n  "},
			}},
			forLoop,
			&ast.Output{Nodes: []ast.Expr{&ast.TemplateData{Data: "\n  </ul>\n"}}},
		},
	}

	tmpl := &ast.Template{Body: []ast.Node{
		&ast.Extends{Template: &ast.Const{Value: "layout.html"}},
		titleBlock,
		renderTitleMacro,
		bodyBlock,
	}}

	tmplSym := SymbolsForNode(tmpl, nil)
	if got, want := tmplSym.Refs, map[string]string{"render_title": "l_0_render_title"}; !reflect.DeepEqual(got, want) {
		t.Errorf("tmpl refs mismatch:\n got %v\nwant %v", got, want)
	}
	if got, want := tmplSym.Loads, map[string]Load{"l_0_render_title": {Kind: LoadUndefined}}; !reflect.DeepEqual(got, want) {
		t.Errorf("tmpl loads mismatch:\n got %v\nwant %v", got, want)
	}
	if got, want := setKeys(tmplSym.Stores), []string{"render_title"}; !reflect.DeepEqual(got, want) {
		t.Errorf("tmpl stores mismatch:\n got %v\nwant %v", got, want)
	}
	if got, want := tmplSym.DumpStores(), map[string]string{"render_title": "l_0_render_title"}; !reflect.DeepEqual(got, want) {
		t.Errorf("tmpl dump_stores mismatch:\n got %v\nwant %v", got, want)
	}

	macroSym := SymbolsForNode(renderTitleMacro, tmplSym)
	wantMacroRefs := map[string]string{
		"subtitle":    "l_1_subtitle",
		"something":   "l_1_something",
		"title":       "l_1_title",
		"title_upper": "l_1_title_upper",
	}
	if !reflect.DeepEqual(macroSym.Refs, wantMacroRefs) {
		t.Errorf("macro refs mismatch:\n got %v\nwant %v", macroSym.Refs, wantMacroRefs)
	}
	wantMacroLoads := map[string]Load{
		"l_1_subtitle":    {Kind: LoadResolve, Arg: "subtitle"},
		"l_1_something":   {Kind: LoadResolve, Arg: "something"},
		"l_1_title":       {Kind: LoadParameter},
		"l_1_title_upper": {Kind: LoadResolve, Arg: "title_upper"},
	}
	if !reflect.DeepEqual(macroSym.Loads, wantMacroLoads) {
		t.Errorf("macro loads mismatch:\n got %v\nwant %v", macroSym.Loads, wantMacroLoads)
	}
	if got, want := setKeys(macroSym.Stores), []string{"subtitle", "title", "title_upper"}; !reflect.DeepEqual(got, want) {
		t.Errorf("macro stores mismatch:\n got %v\nwant %v", got, want)
	}
	if got, want := macroSym.FindRef("render_title"), "l_0_render_title"; got != want {
		t.Errorf("macro find_ref(render_title) = %q, want %q", got, want)
	}
	wantMacroDump := map[string]string{
		"title":        "l_1_title",
		"title_upper":  "l_1_title_upper",
		"subtitle":     "l_1_subtitle",
		"render_title": "l_0_render_title",
	}
	if got := macroSym.DumpStores(); !reflect.DeepEqual(got, wantMacroDump) {
		t.Errorf("macro dump_stores mismatch:\n got %v\nwant %v", got, wantMacroDump)
	}

	bodySym := SymbolsForNode(bodyBlock, nil)
	wantBodyRefs := map[string]string{
		"item":         "l_0_item",
		"seq":          "l_0_seq",
		"render_title": "l_0_render_title",
	}
	if !reflect.DeepEqual(bodySym.Refs, wantBodyRefs) {
		t.Errorf("body refs mismatch:\n got %v\nwant %v", bodySym.Refs, wantBodyRefs)
	}
	wantBodyLoads := map[string]Load{
		"l_0_item":         {Kind: LoadResolve, Arg: "item"},
		"l_0_seq":          {Kind: LoadResolve, Arg: "seq"},
		"l_0_render_title": {Kind: LoadResolve, Arg: "render_title"},
	}
	if !reflect.DeepEqual(bodySym.Loads, wantBodyLoads) {
		t.Errorf("body loads mismatch:\n got %v\nwant %v", bodySym.Loads, wantBodyLoads)
	}
	if got := setKeys(bodySym.Stores); len(got) != 0 {
		t.Errorf("body stores should be empty; got %v", got)
	}

	forSym := SymbolsForNode(forLoop, bodySym)
	if got, want := forSym.Refs, map[string]string{"item": "l_1_item"}; !reflect.DeepEqual(got, want) {
		t.Errorf("for refs mismatch:\n got %v\nwant %v", got, want)
	}
	if got, want := forSym.Loads, map[string]Load{"l_1_item": {Kind: LoadParameter}}; !reflect.DeepEqual(got, want) {
		t.Errorf("for loads mismatch:\n got %v\nwant %v", got, want)
	}
	if got, want := setKeys(forSym.Stores), []string{"item"}; !reflect.DeepEqual(got, want) {
		t.Errorf("for stores mismatch:\n got %v\nwant %v", got, want)
	}
	if got, want := forSym.DumpStores(), map[string]string{"item": "l_1_item"}; !reflect.DeepEqual(got, want) {
		t.Errorf("for dump_stores mismatch:\n got %v\nwant %v", got, want)
	}
}

func TestIfBranchingStores(t *testing.T) {
	tmpl := &ast.Template{Body: []ast.Node{
		ifExpr(
			&ast.Name{Name: "expression", Ctx: ast.CtxLoad},
			[]ast.Node{
				&ast.Assign{
					Target: &ast.Name{Name: "variable", Ctx: ast.CtxStore},
					Node:   &ast.Const{Value: int64(42)},
				},
			},
			nil,
			nil,
		),
	}}

	sym := SymbolsForNode(tmpl, nil)
	wantRefs := map[string]string{
		"variable":   "l_0_variable",
		"expression": "l_0_expression",
	}
	if !reflect.DeepEqual(sym.Refs, wantRefs) {
		t.Errorf("refs mismatch:\n got %v\nwant %v", sym.Refs, wantRefs)
	}
	if got, want := setKeys(sym.Stores), []string{"variable"}; !reflect.DeepEqual(got, want) {
		t.Errorf("stores mismatch:\n got %v\nwant %v", got, want)
	}
	wantLoads := map[string]Load{
		"l_0_variable":   {Kind: LoadResolve, Arg: "variable"},
		"l_0_expression": {Kind: LoadResolve, Arg: "expression"},
	}
	if !reflect.DeepEqual(sym.Loads, wantLoads) {
		t.Errorf("loads mismatch:\n got %v\nwant %v", sym.Loads, wantLoads)
	}
	if got, want := sym.DumpStores(), map[string]string{"variable": "l_0_variable"}; !reflect.DeepEqual(got, want) {
		t.Errorf("dump_stores mismatch:\n got %v\nwant %v", got, want)
	}
}

func TestIfBranchingStoresUndefined(t *testing.T) {
	tmpl := &ast.Template{Body: []ast.Node{
		&ast.Assign{
			Target: &ast.Name{Name: "variable", Ctx: ast.CtxStore},
			Node:   &ast.Const{Value: int64(23)},
		},
		ifExpr(
			&ast.Name{Name: "expression", Ctx: ast.CtxLoad},
			[]ast.Node{
				&ast.Assign{
					Target: &ast.Name{Name: "variable", Ctx: ast.CtxStore},
					Node:   &ast.Const{Value: int64(42)},
				},
			},
			nil,
			nil,
		),
	}}

	sym := SymbolsForNode(tmpl, nil)
	wantRefs := map[string]string{
		"variable":   "l_0_variable",
		"expression": "l_0_expression",
	}
	if !reflect.DeepEqual(sym.Refs, wantRefs) {
		t.Errorf("refs mismatch:\n got %v\nwant %v", sym.Refs, wantRefs)
	}
	if got, want := setKeys(sym.Stores), []string{"variable"}; !reflect.DeepEqual(got, want) {
		t.Errorf("stores mismatch:\n got %v\nwant %v", got, want)
	}
	wantLoads := map[string]Load{
		"l_0_variable":   {Kind: LoadUndefined},
		"l_0_expression": {Kind: LoadResolve, Arg: "expression"},
	}
	if !reflect.DeepEqual(sym.Loads, wantLoads) {
		t.Errorf("loads mismatch:\n got %v\nwant %v", sym.Loads, wantLoads)
	}
	if got, want := sym.DumpStores(), map[string]string{"variable": "l_0_variable"}; !reflect.DeepEqual(got, want) {
		t.Errorf("dump_stores mismatch:\n got %v\nwant %v", got, want)
	}
}

func TestIfBranchingMultiScope(t *testing.T) {
	forLoop := &ast.For{
		Target: &ast.Name{Name: "item", Ctx: ast.CtxStore},
		Iter:   &ast.Name{Name: "seq", Ctx: ast.CtxLoad},
		Body: []ast.Node{
			ifExpr(
				&ast.Name{Name: "expression", Ctx: ast.CtxLoad},
				[]ast.Node{
					&ast.Assign{
						Target: &ast.Name{Name: "x", Ctx: ast.CtxStore},
						Node:   &ast.Const{Value: int64(42)},
					},
				},
				nil,
				nil,
			),
			&ast.Include{Template: &ast.Const{Value: "helper.html"}, WithContext: true},
		},
	}

	tmpl := &ast.Template{Body: []ast.Node{
		&ast.Assign{
			Target: &ast.Name{Name: "x", Ctx: ast.CtxStore},
			Node:   &ast.Const{Value: int64(23)},
		},
		forLoop,
	}}

	tmplSym := SymbolsForNode(tmpl, nil)
	forSym := SymbolsForNode(forLoop, tmplSym)
	if got, want := setKeys(forSym.Stores), []string{"item", "x"}; !reflect.DeepEqual(got, want) {
		t.Errorf("for stores mismatch:\n got %v\nwant %v", got, want)
	}
	wantLoads := map[string]Load{
		"l_1_x":          {Kind: LoadAlias, Arg: "l_0_x"},
		"l_1_item":       {Kind: LoadParameter},
		"l_1_expression": {Kind: LoadResolve, Arg: "expression"},
	}
	if !reflect.DeepEqual(forSym.Loads, wantLoads) {
		t.Errorf("for loads mismatch:\n got %v\nwant %v", forSym.Loads, wantLoads)
	}
}

// ifExpr is a small helper that mirrors `nodes.If(test, body, elif_, else_)`.
func ifExpr(test ast.Node, body []ast.Node, elif []*ast.If, els []ast.Node) *ast.If {
	return &ast.If{Test: test, Body: body, Elif: elif, Else: els}
}

func setKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
