package meta

import (
	"testing"

	"github.com/jryberg/gojinja/pkg/ast"
	"github.com/jryberg/gojinja/pkg/lexer"
	"github.com/jryberg/gojinja/pkg/parser"
)

func parseTemplate(t *testing.T, src string) *ast.Template {
	t.Helper()
	l, err := lexer.New(lexer.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	stream, err := l.Tokenize(src, "test", "")
	if err != nil {
		t.Fatal(err)
	}
	tpl, err := parser.New(stream).Parse()
	if err != nil {
		t.Fatal(err)
	}
	return tpl
}

func TestFindUndeclaredSimple(t *testing.T) {
	tpl := parseTemplate(t, `{{ name }}`)
	got := FindUndeclaredVariables(tpl)
	if len(got) != 1 || got[0] != "name" {
		t.Fatalf("got %v", got)
	}
}

func TestFindUndeclaredIgnoresLocalSet(t *testing.T) {
	tpl := parseTemplate(t, `{% set x = 1 %}{{ x }}{{ y }}`)
	got := FindUndeclaredVariables(tpl)
	if len(got) != 1 || got[0] != "y" {
		t.Fatalf("got %v", got)
	}
}

func TestFindUndeclaredForLoopBindings(t *testing.T) {
	tpl := parseTemplate(t, `{% for x in xs %}{{ x }}{% endfor %}`)
	got := FindUndeclaredVariables(tpl)
	// x is bound by the loop; xs is referenced.
	if len(got) != 1 || got[0] != "xs" {
		t.Fatalf("got %v", got)
	}
}

func TestFindReferencedTemplatesStatic(t *testing.T) {
	tpl := parseTemplate(t, `{% extends 'base.html' %}{% include 'sub.html' %}`)
	got := FindReferencedTemplates(tpl)
	if len(got) != 2 {
		t.Fatalf("expected 2 names, got %d (%v)", len(got), got)
	}
	if got[0] == nil || *got[0] != "base.html" {
		t.Errorf("got[0] = %v", got[0])
	}
	if got[1] == nil || *got[1] != "sub.html" {
		t.Errorf("got[1] = %v", got[1])
	}
}

func TestFindReferencedTemplatesDynamic(t *testing.T) {
	tpl := parseTemplate(t, `{% include name %}`)
	got := FindReferencedTemplates(tpl)
	if len(got) != 1 || got[0] != nil {
		t.Fatalf("expected one nil entry for dynamic ref, got %v", got)
	}
}

func TestFindReferencedTemplatesList(t *testing.T) {
	tpl := parseTemplate(t, `{% include ['a.html', 'b.html'] %}`)
	got := FindReferencedTemplates(tpl)
	if len(got) != 2 {
		t.Fatalf("expected 2 names, got %d", len(got))
	}
	if got[0] == nil || *got[0] != "a.html" {
		t.Errorf("got[0] = %v", got[0])
	}
}
