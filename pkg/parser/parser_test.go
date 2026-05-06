package parser

import (
	"strings"
	"testing"

	"github.com/jryberg/gojinja/pkg/ast"
	"github.com/jryberg/gojinja/pkg/lexer"
)

// parse is a test helper that lexes + parses src.
func parse(t *testing.T, src string) *ast.Template {
	t.Helper()
	l, err := lexer.New(lexer.DefaultOptions())
	if err != nil {
		t.Fatalf("lexer: %v", err)
	}
	stream, err := l.Tokenize(src, "test", "")
	if err != nil {
		t.Fatalf("tokenize: %v", err)
	}
	p := New(stream)
	tpl, err := p.Parse()
	if err != nil {
		t.Fatalf("parse(%q): %v", src, err)
	}
	return tpl
}

func parseExpect(t *testing.T, src, errSubstr string) {
	t.Helper()
	l, err := lexer.New(lexer.DefaultOptions())
	if err != nil {
		t.Fatalf("lexer: %v", err)
	}
	stream, err := l.Tokenize(src, "test", "")
	if err != nil {
		// Lex error is OK if the user expected one too.
		if errSubstr == "" || strings.Contains(err.Error(), errSubstr) {
			return
		}
		t.Fatalf("unexpected lex error: %v", err)
	}
	p := New(stream)
	_, err = p.Parse()
	if err == nil {
		t.Fatalf("parse(%q) expected error containing %q, got nil", src, errSubstr)
	}
	if errSubstr != "" && !strings.Contains(err.Error(), errSubstr) {
		t.Fatalf("parse(%q) error %q does not contain %q", src, err.Error(), errSubstr)
	}
}

// expectAST builds and dumps a template, asserting it contains every
// fragment in wantContains.
func expectAST(t *testing.T, src string, wantContains ...string) {
	t.Helper()
	tpl := parse(t, src)
	dump := ast.Dump(tpl)
	for _, w := range wantContains {
		if !strings.Contains(dump, w) {
			t.Fatalf("parse(%q) dump = %s, missing %q", src, dump, w)
		}
	}
}

// =============================================================== Expressions

func TestParseInteger(t *testing.T) {
	expectAST(t, "{{ 42 }}", "Const", "42")
}

func TestParseFloat(t *testing.T) {
	expectAST(t, "{{ 3.14 }}", "Const", "3.14")
}

func TestParseString(t *testing.T) {
	expectAST(t, `{{ "hi" }}`, "Const", `"hi"`)
}

func TestParseStringConcat(t *testing.T) {
	// Adjacent strings are concatenated by parsePrimary, mirroring Python.
	expectAST(t, `{{ "a" "b" }}`, "Const", `"ab"`)
}

func TestParseBoolNone(t *testing.T) {
	expectAST(t, "{{ true }}", "Const", "True")
	expectAST(t, "{{ False }}", "Const", "False")
	expectAST(t, "{{ None }}", "Const", "None")
}

func TestParseList(t *testing.T) {
	expectAST(t, "{{ [1, 2, 3] }}", "List")
}

func TestParseDict(t *testing.T) {
	expectAST(t, "{{ {'a': 1} }}", "Dict", "Pair")
}

func TestParseTuple(t *testing.T) {
	expectAST(t, "{{ (1, 2) }}", "Tuple")
}

func TestParseNameRef(t *testing.T) {
	expectAST(t, "{{ x }}", `Name(name="x"`)
}

func TestParseAttrAndItem(t *testing.T) {
	expectAST(t, "{{ a.b }}", "Getattr")
	expectAST(t, "{{ a[1] }}", "Getitem")
}

func TestParseSlice(t *testing.T) {
	expectAST(t, "{{ a[1:2:3] }}", "Slice")
}

func TestParseCall(t *testing.T) {
	expectAST(t, "{{ f(1, 2, x=3, *a, **k) }}", "Call")
}

// Operator precedence: 1 + 2 * 3 == 7, parser produces Add(1, Mul(2,3))
func TestPrecedenceAddMul(t *testing.T) {
	expectAST(t, "{{ 1 + 2 * 3 }}", "Add", "Mul")
}

// Chained comparison
func TestChainedCompare(t *testing.T) {
	expectAST(t, "{{ 1 < 2 < 3 }}", "Compare")
}

// in / not in
func TestInOperators(t *testing.T) {
	expectAST(t, `{{ 'a' in xs }}`, "Compare")
	expectAST(t, `{{ 'a' not in xs }}`, "Compare", "notin")
}

// Filter chain
func TestFilterChain(t *testing.T) {
	expectAST(t, "{{ x | upper | trim }}", "Filter")
}

// Test
func TestIsTest(t *testing.T) {
	expectAST(t, "{{ x is defined }}", "Test")
}

// Conditional expression
func TestCondExpr(t *testing.T) {
	expectAST(t, "{{ a if b else c }}", "CondExpr")
}

// Concat operator ~
func TestConcatOp(t *testing.T) {
	expectAST(t, `{{ "a" ~ x ~ "b" }}`, "Concat")
}

// Unary
func TestUnary(t *testing.T) {
	expectAST(t, "{{ -x }}", "Neg")
	expectAST(t, "{{ +x }}", "UAdd")
	expectAST(t, "{{ not x }}", "Not")
}

// Power
func TestPower(t *testing.T) {
	expectAST(t, "{{ 2 ** 10 }}", "Pow")
}

// =============================================================== Statements

func TestParseIf(t *testing.T) {
	expectAST(t, "{% if x %}yes{% endif %}", "If", "TemplateData")
}

func TestParseIfElifElse(t *testing.T) {
	expectAST(t, "{% if a %}A{% elif b %}B{% else %}C{% endif %}", "If")
}

func TestParseFor(t *testing.T) {
	expectAST(t, "{% for x in xs %}{{ x }}{% endfor %}", "For")
}

func TestParseForElse(t *testing.T) {
	expectAST(t, "{% for x in xs %}A{% else %}empty{% endfor %}", "For")
}

func TestParseForIfRecursive(t *testing.T) {
	expectAST(t, "{% for x in xs if x.shown recursive %}{{ x }}{% endfor %}", "For", "recursive=True")
}

func TestParseSet(t *testing.T) {
	expectAST(t, "{% set x = 1 %}", "Assign")
}

func TestParseSetBlock(t *testing.T) {
	expectAST(t, "{% set x %}body{% endset %}", "AssignBlock")
}

func TestParseSetTuple(t *testing.T) {
	expectAST(t, "{% set a, b = 1, 2 %}", "Assign", "Tuple")
}

func TestParseMacro(t *testing.T) {
	expectAST(t, "{% macro greet(name, suffix='!') %}hi {{ name }}{{ suffix }}{% endmacro %}", "Macro")
}

func TestParseCallBlock(t *testing.T) {
	expectAST(t, "{% call greet('hi') %}body{% endcall %}", "CallBlock")
}

func TestParseFilterBlock(t *testing.T) {
	expectAST(t, "{% filter upper %}hi{% endfilter %}", "FilterBlock")
}

func TestParseWith(t *testing.T) {
	expectAST(t, "{% with x=1, y=2 %}body{% endwith %}", "With")
}

func TestParseInclude(t *testing.T) {
	expectAST(t, `{% include 'foo.html' %}`, "Include")
}

func TestParseIncludeIgnoreMissingWithoutContext(t *testing.T) {
	expectAST(t,
		`{% include 'foo.html' ignore missing without context %}`,
		"Include", "ignoreMissing=True", "withContext=False",
	)
}

func TestParseExtends(t *testing.T) {
	expectAST(t, `{% extends 'base.html' %}`, "Extends")
}

func TestParseImport(t *testing.T) {
	expectAST(t, `{% import 'mod.html' as m %}`, "Import")
}

func TestParseFromImport(t *testing.T) {
	expectAST(t, `{% from 'mod.html' import a, b as c %}`, "FromImport")
}

func TestParseBlock(t *testing.T) {
	expectAST(t, "{% block content %}body{% endblock %}", "Block")
}

func TestParseAutoescape(t *testing.T) {
	expectAST(t, "{% autoescape true %}body{% endautoescape %}", "ScopedEvalContextModifier")
}

// =============================================================== Errors

func TestUnknownTagError(t *testing.T) {
	parseExpect(t, "{% xyz %}", "Encountered unknown tag")
}

func TestUnclosedIfError(t *testing.T) {
	parseExpect(t, "{% if x %}body", "")
}

func TestHyphenInBlockNameError(t *testing.T) {
	parseExpect(t, "{% block my-block %}body{% endblock %}", "hyphen")
}

func TestFromImportUnderscoreError(t *testing.T) {
	parseExpect(t, "{% from 'm' import _x %}", "underline")
}
