package errors

import (
	stderrors "errors"
	"strings"
	"testing"
)

func TestTemplateErrorMessage(t *testing.T) {
	err := NewTemplateError("boom")
	if err.Error() != "boom" {
		t.Fatalf("Error() = %q, want %q", err.Error(), "boom")
	}
}

func TestTemplateNotFoundDefaultMessage(t *testing.T) {
	err := NewTemplateNotFound("missing.html", "")
	if err.Error() != "missing.html" {
		t.Fatalf("Error() = %q, want %q", err.Error(), "missing.html")
	}
	if len(err.Templates) != 1 || err.Templates[0] != "missing.html" {
		t.Fatalf("Templates not seeded with name, got %v", err.Templates)
	}
}

func TestTemplatesNotFoundDefaultMessage(t *testing.T) {
	err := NewTemplatesNotFound([]any{"a.html", "b.html"}, "")
	got := err.Error()
	want := "none of the templates given were found: a.html, b.html"
	if got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
}

func TestTemplateSyntaxErrorFormat(t *testing.T) {
	err := NewTemplateSyntaxError("unexpected end of template", 3, "page.html", "")
	err.Source = "line 1\nline 2\n  unterminated\n"
	got := err.Error()
	if !strings.Contains(got, "unexpected end of template") {
		t.Errorf("missing message: %s", got)
	}
	if !strings.Contains(got, `File "page.html", line 3`) {
		t.Errorf("missing location: %s", got)
	}
	if !strings.Contains(got, "unterminated") {
		t.Errorf("missing source excerpt: %s", got)
	}
}

func TestTemplateSyntaxErrorTranslatedSkipsLocation(t *testing.T) {
	err := NewTemplateSyntaxError("plain message", 99, "x", "")
	err.Translated = true
	if err.Error() != "plain message" {
		t.Fatalf("Translated error should print message verbatim, got %q", err.Error())
	}
}

func TestErrorTypeAssertions(t *testing.T) {
	cases := []error{
		NewTemplateError("x"),
		NewTemplateNotFound("a", ""),
		NewTemplatesNotFound([]any{"a"}, ""),
		NewTemplateSyntaxError("x", 1, "", ""),
		NewTemplateAssertionError("x", 1, "", ""),
		NewTemplateRuntimeError("x"),
		NewUndefinedError("x"),
		NewSecurityError("x"),
		NewFilterArgumentError("x"),
	}
	for _, c := range cases {
		if _, ok := AsTemplateError(c); !ok {
			t.Errorf("AsTemplateError(%T) returned false", c)
		}
	}
}

func TestSpecificErrorAsTargets(t *testing.T) {
	err := error(NewSecurityError("blocked"))
	var sec *SecurityError
	if !stderrors.As(err, &sec) {
		t.Fatalf("errors.As did not match *SecurityError")
	}
	if sec.Message != "blocked" {
		t.Errorf("sec.Message = %q", sec.Message)
	}
}
