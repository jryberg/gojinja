package ast

import (
	"strings"
	"testing"

	"github.com/jryberg/gojinja/pkg/escape"
)

// build a representative AST: `{{ "hi" + name }}`
func sampleTree() *Template {
	tpl := &Template{
		Body: []Node{
			&Output{
				Nodes: []Expr{
					&Add{
						binexprBase: binexprBase{
							exprBase: exprBase{posBase: posBase{Pos: Pos{Lineno: 1}}},
							Op:       "+",
							Left:     &Const{literalBase: literalBase{exprBase: exprBase{posBase: posBase{Pos: Pos{Lineno: 1}}}}, Value: "hi"},
							Right:    &Name{exprBase: exprBase{posBase: posBase{Pos: Pos{Lineno: 1}}}, Name: "name", Ctx: CtxLoad},
						},
					},
				},
			},
		},
	}
	tpl.Pos.Lineno = 1
	return tpl
}

// ------------------------------------------------------ Walk visits everything

func TestWalkVisitsEveryNode(t *testing.T) {
	tpl := sampleTree()
	var seen []string
	v := VisitorFunc(func(n Node) bool {
		seen = append(seen, kindOf(n))
		return true
	})
	Walk(tpl, v)
	want := []string{"*Template", "*Output", "*Add", "*Const", "*Name"}
	if !equalStrings(seen, want) {
		t.Fatalf("Walk order = %v, want %v", seen, want)
	}
}

func TestWalkRespectsDescendFalse(t *testing.T) {
	tpl := sampleTree()
	visits := 0
	v := VisitorFunc(func(n Node) bool {
		visits++
		_, isOutput := n.(*Output)
		return !isOutput // do not descend into Output
	})
	Walk(tpl, v)
	if visits != 2 { // Template + Output
		t.Fatalf("visited %d, want 2", visits)
	}
}

// ----------------------------------------------------- AsConst basic cases

func TestConstAsConst(t *testing.T) {
	c := &Const{Value: 42}
	v, err := c.AsConst(EvalCtx{})
	if err != nil || v.(int) != 42 {
		t.Fatalf("Const.AsConst = %v,%v", v, err)
	}
}

func TestTemplateDataAutoescape(t *testing.T) {
	td := &TemplateData{Data: "<b>"}
	v, err := td.AsConst(EvalCtx{Autoescape: false})
	if err != nil || v != "<b>" {
		t.Errorf("autoescape off: %v,%v", v, err)
	}
	v, err = td.AsConst(EvalCtx{Autoescape: true})
	if err != nil || v != escape.Markup("<b>") {
		t.Errorf("autoescape on: %v,%v", v, err)
	}
	_, err = td.AsConst(EvalCtx{Volatile: true})
	if !IsImpossible(err) {
		t.Errorf("volatile should fold to impossible, got %v", err)
	}
}

func TestBinopFolding(t *testing.T) {
	cases := []struct {
		name string
		op   string
		a    any
		b    any
		want any
	}{
		{"add_int", "+", int64(2), int64(3), int64(5)},
		{"sub_int", "-", int64(5), int64(2), int64(3)},
		{"mul_mixed", "*", int64(3), float64(1.5), float64(4.5)},
		{"div_int_to_float", "/", int64(7), int64(2), 3.5},
		{"floordiv_int", "//", int64(7), int64(2), int64(3)},
		{"mod_int", "%", int64(7), int64(3), int64(1)},
		{"pow_int", "**", int64(2), int64(10), int64(1024)},
		{"add_str", "+", "ab", "cd", "abcd"},
		{"mul_str", "*", "ab", int64(3), "ababab"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			got, err := applyBinop(c.op, c.a, c.b)
			if err != nil {
				t.Fatalf("err %v", err)
			}
			if got != c.want {
				t.Fatalf("op=%s: got %v(%T), want %v(%T)", c.op, got, got, c.want, c.want)
			}
		})
	}
}

func TestBinopDivByZero(t *testing.T) {
	_, err := applyBinop("/", int64(1), int64(0))
	if !IsImpossible(err) {
		t.Fatalf("div by zero should be Impossible, got %v", err)
	}
}

// And short-circuits without evaluating the right.
func TestAndShortCircuit(t *testing.T) {
	expr := &And{binexprBase: binexprBase{
		Op:    "and",
		Left:  &Const{Value: false},
		Right: &Name{Name: "wouldFail"}, // would return Impossible if evaluated
	}}
	v, err := expr.AsConst(EvalCtx{})
	if err != nil {
		t.Fatalf("expected no error (short-circuit), got %v", err)
	}
	if v != false {
		t.Fatalf("expected false, got %v", v)
	}
}

func TestOrShortCircuit(t *testing.T) {
	expr := &Or{binexprBase: binexprBase{
		Op:    "or",
		Left:  &Const{Value: 1},
		Right: &Name{Name: "wouldFail"},
	}}
	v, err := expr.AsConst(EvalCtx{})
	if err != nil || v != 1 {
		t.Fatalf("got %v,%v", v, err)
	}
}

// CondExpr folds via the test branch.
func TestCondExprFold(t *testing.T) {
	c := &CondExpr{
		Test:  &Const{Value: true},
		Expr1: &Const{Value: "a"},
		Expr2: &Const{Value: "b"},
	}
	v, err := c.AsConst(EvalCtx{})
	if err != nil || v != "a" {
		t.Fatalf("cond expr fold = %v,%v", v, err)
	}
}

// Compare chain.
func TestCompareChain(t *testing.T) {
	c := &Compare{
		Expr: &Const{Value: int64(1)},
		Ops: []*Operand{
			{Op: "lt", Expr: &Const{Value: int64(2)}},
			{Op: "lt", Expr: &Const{Value: int64(3)}},
		},
	}
	v, err := c.AsConst(EvalCtx{})
	if err != nil || v != true {
		t.Fatalf("compare chain = %v,%v", v, err)
	}
	// false case: 1 < 2 < 0 -> false
	c.Ops[1].Expr = &Const{Value: int64(0)}
	v, err = c.AsConst(EvalCtx{})
	if err != nil || v != false {
		t.Fatalf("false compare chain = %v,%v", v, err)
	}
}

// MarkSafeIfAutoescape switches between escaped and raw.
func TestMarkSafeIfAutoescape(t *testing.T) {
	m := &MarkSafeIfAutoescape{Expr: &Const{Value: "x"}}
	v, _ := m.AsConst(EvalCtx{Autoescape: true})
	if _, ok := v.(escape.Markup); !ok {
		t.Errorf("autoescape on: expected Markup, got %T", v)
	}
	v, _ = m.AsConst(EvalCtx{Autoescape: false})
	if _, ok := v.(escape.Markup); ok {
		t.Errorf("autoescape off: expected non-Markup, got Markup")
	}
}

// Name.CanAssign rejects reserved literals.
func TestNameCanAssign(t *testing.T) {
	for _, n := range []string{"foo", "bar"} {
		if !(&Name{Name: n}).CanAssign() {
			t.Errorf("Name(%q).CanAssign() = false, want true", n)
		}
	}
	for _, n := range []string{"true", "false", "none", "True", "False", "None"} {
		if (&Name{Name: n}).CanAssign() {
			t.Errorf("Name(%q).CanAssign() = true, want false", n)
		}
	}
}

// Slice fold.
func TestSliceFold(t *testing.T) {
	s := &Slice{
		Start: &Const{Value: int64(1)},
		Stop:  &Const{Value: int64(5)},
		Step:  nil,
	}
	v, err := s.AsConst(EvalCtx{})
	if err != nil {
		t.Fatal(err)
	}
	sv, ok := v.(SliceVal)
	if !ok {
		t.Fatalf("expected SliceVal, got %T", v)
	}
	if sv.Start.(int64) != 1 || sv.Stop.(int64) != 5 || sv.Step != nil {
		t.Fatalf("SliceVal = %+v", sv)
	}
}

// Concat fold.
func TestConcatFold(t *testing.T) {
	c := &Concat{Nodes: []Expr{
		&Const{Value: "a"},
		&Const{Value: int64(42)},
		&Const{Value: "b"},
	}}
	v, err := c.AsConst(EvalCtx{})
	if err != nil {
		t.Fatal(err)
	}
	if v != "a42b" {
		t.Fatalf("Concat fold = %q", v)
	}
}

// Truthy table covers the types we model. Uses a slice (not a map) so
// slice-typed values can appear as keys.
func TestTruthy(t *testing.T) {
	cases := []struct {
		v    any
		want bool
	}{
		{nil, false},
		{false, false},
		{true, true},
		{int64(0), false},
		{int64(1), true},
		{float64(0), false},
		{float64(0.5), true},
		{"", false},
		{"x", true},
		{[]any(nil), false},
		{[]any{1}, true},
	}
	for _, c := range cases {
		if got := truthy(c.v); got != c.want {
			t.Errorf("truthy(%v) = %v, want %v", c.v, got, c.want)
		}
	}
}

// ---------------------------------------------------------------- Dump

func TestDumpRoundTripShape(t *testing.T) {
	tpl := sampleTree()
	got := Dump(tpl)
	// Just a sanity check of the shape; not full byte-for-byte parity.
	for _, want := range []string{"Template", "Output", "Add", "Const", "Name"} {
		if !strings.Contains(got, want) {
			t.Errorf("Dump missing %q in: %s", want, got)
		}
	}
}

// TestChildrenSkipsTypedNilFilter exercises the regression where
// AssignBlock and FilterBlock store an optional *Filter; an unset filter
// is a typed-nil pointer wrapped in the Node interface, so the generic
// `n != nil` check inside appendIfNotNil would let it through and the
// walker would later panic on x.Node deref.
func TestChildrenSkipsTypedNilFilter(t *testing.T) {
	cases := []struct {
		name string
		in   Node
	}{
		{"AssignBlock_no_filter", &AssignBlock{Target: &Name{Name: "y"}, Filter: nil, Body: nil}},
		{"FilterBlock_no_filter", &FilterBlock{Body: nil, Filter: nil}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panic during walk: %v", r)
				}
			}()
			Walk(tc.in, VisitorFunc(func(Node) bool { return true }))
		})
	}
}

// ---------------------------------------------------------- helpers

func kindOf(n Node) string {
	if n == nil {
		return "nil"
	}
	t := reflectKindString(n)
	return t
}

// reflectKindString avoids a reflect import in the helper above by using
// fmt-like type formatting. The tests don't need exact reflect output.
func reflectKindString(n Node) string {
	if n == nil {
		return "nil"
	}
	switch n.(type) {
	case *Template:
		return "*Template"
	case *Output:
		return "*Output"
	case *Add:
		return "*Add"
	case *Const:
		return "*Const"
	case *Name:
		return "*Name"
	}
	return "?"
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
