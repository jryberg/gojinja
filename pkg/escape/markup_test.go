package escape

import "testing"

// Mirror of markupsafe's escape table tests:
// markupsafe: Markup("<em>Hello</em>")        -> "<em>Hello</em>"   (already safe)
//             escape("<em>Hello</em>")        -> "&lt;em&gt;..."
//             escape(Markup("<em>"))          -> "<em>"             (idempotent)
//             escape(None)                    -> ""

func TestEscapePlainString(t *testing.T) {
	got := Escape(`<a href="x">Tom & Jerry</a>`)
	want := Markup(`&lt;a href=&#34;x&#34;&gt;Tom &amp; Jerry&lt;/a&gt;`)
	if got != want {
		t.Errorf("Escape(plain) = %q, want %q", got, want)
	}
}

func TestEscapeMarkupIsIdempotent(t *testing.T) {
	already := Markup(`<em>Hello</em>`)
	got := Escape(already)
	if got != already {
		t.Errorf("Escape(Markup) = %q, want it returned unchanged %q", got, already)
	}
}

func TestEscapeNil(t *testing.T) {
	// markupsafe.escape(None) == Markup('None') — match the Python repr
	// so autoescaped output stays parity-clean against canonical Jinja2.
	if got := Escape(nil); got != Markup("None") {
		t.Errorf("Escape(nil) = %q, want \"None\"", got)
	}
}

func TestEscapeNumber(t *testing.T) {
	if got := Escape(42); got != "42" {
		t.Errorf("Escape(42) = %q, want \"42\"", got)
	}
}

func TestEscapeQuoteChars(t *testing.T) {
	if got := Escape("'\""); got != Markup("&#39;&#34;") {
		t.Errorf("quote escape mismatch, got %q", got)
	}
}

func TestForceEscapeMarkup(t *testing.T) {
	already := Markup(`<em>Hi</em>`)
	got := ForceEscape(already)
	want := Markup(`&lt;em&gt;Hi&lt;/em&gt;`)
	if got != want {
		t.Errorf("ForceEscape(Markup) = %q, want %q", got, want)
	}
}

type customHTMLer struct{ s string }

func (c customHTMLer) HTML() Markup { return Markup(c.s) }

func TestEscapeHTMLerInterface(t *testing.T) {
	got := Escape(customHTMLer{s: "<b>safe</b>"})
	if got != Markup("<b>safe</b>") {
		t.Errorf("HTMLer not honored, got %q", got)
	}
}

func TestSoftStrMarkupPreservesText(t *testing.T) {
	if got := SoftStr(Markup("<em>")); got != "<em>" {
		t.Errorf("SoftStr(Markup) lost content, got %q", got)
	}
}

func TestConcatMixed(t *testing.T) {
	got := Concat("Hello, ", Markup("<b>"), "Tom & Jerry", Markup("</b>"))
	want := Markup("Hello, <b>Tom &amp; Jerry</b>")
	if got != want {
		t.Errorf("Concat mismatch, got %q, want %q", got, want)
	}
}

func TestPlainConcat(t *testing.T) {
	got := PlainConcat("a", Markup("<b>"), 1)
	if got != "a<b>1" {
		t.Errorf("PlainConcat = %q, want \"a<b>1\"", got)
	}
}

func TestNeedsEscapeFastPath(t *testing.T) {
	if !needsEscape("a&b") {
		t.Error("needsEscape didn't detect &")
	}
	if needsEscape("hello world") {
		t.Error("needsEscape false positive on safe text")
	}
}

func TestIsMarkupReturnsValue(t *testing.T) {
	if m, ok := IsMarkup(Markup("safe")); !ok || m != "safe" {
		t.Errorf("IsMarkup(Markup) = %q,%v", m, ok)
	}
	if _, ok := IsMarkup("not safe"); ok {
		t.Error("IsMarkup string should return false")
	}
}
