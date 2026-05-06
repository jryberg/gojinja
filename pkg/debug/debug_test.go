package debug

import (
	"errors"
	"strings"
	"testing"
)

type fakeKind struct{ msg string }

func (f *fakeKind) Error() string { return f.msg }

func TestWrapNilReturnsNil(t *testing.T) {
	if err := Wrap(nil, "x", 1, "src"); err != nil {
		t.Fatalf("Wrap(nil) = %v, want nil", err)
	}
}

func TestErrorFormatHasLocationAndSource(t *testing.T) {
	inner := errors.New("boom")
	te := Wrap(inner, "page.html", 2, "line one\nline two\nline three")
	out := te.Error()
	for _, want := range []string{"boom", `"page.html"`, "line 2", "line two"} {
		if !strings.Contains(out, want) {
			t.Errorf("formatted error missing %q in:\n%s", want, out)
		}
	}
}

func TestUnwrapReachesInner(t *testing.T) {
	target := &fakeKind{msg: "kaboom"}
	te := Wrap(target, "t", 1, "")
	var got *fakeKind
	if !errors.As(te, &got) {
		t.Fatal("errors.As did not reach inner")
	}
	if got.msg != "kaboom" {
		t.Fatalf("got %q", got.msg)
	}
}

func TestNoDoubleWrap(t *testing.T) {
	inner := errors.New("once")
	once := Wrap(inner, "t", 1, "")
	twice := Wrap(once, "other", 5, "")
	if once != twice {
		t.Fatal("Wrap of TraceError should be a no-op")
	}
}

func TestNoLocationStillFormatsMessage(t *testing.T) {
	te := Wrap(errors.New("plain"), "", 0, "")
	if te.Error() != "plain" {
		t.Fatalf("got %q", te.Error())
	}
}
