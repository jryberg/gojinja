package filters

import (
	"context"
	"strings"
	"testing"

	"github.com/jryberg/gojinja/pkg/environment"
)

func renderB64(t *testing.T, src string, vars map[string]any) (string, error) {
	t.Helper()
	e, err := environment.New(
		environment.WithAutoescape(environment.AutoescapeNever{}),
		environment.WithFilter("base64decode", Base64Decode),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	tpl, err := e.FromString(src)
	if err != nil {
		t.Fatalf("FromString(%q): %v", src, err)
	}
	return tpl.RenderContext(context.Background(), vars)
}

// Expected values match Python's base64.b64decode(s).decode(encoding).
func TestBase64Decode(t *testing.T) {
	cases := []struct{ in, want string }{
		{"aGVsbG8=", "hello"},
		{"aGVs bG8=\n", "hello"},   // whitespace is skipped
		{"aGVs!bG8=", "hello"},     // non-alphabet bytes are skipped
		{"", ""},                   // empty input
		{"aGVsbG8==", "hello"},     // excess padding
		{"aGVsbG8=extra", "hello"}, // data after complete padding is ignored
		{"a=GVsbG8=", "hello"},     // '=' before the second data char is skipped
		{"w6XDpMO2", "åäö"},        // multi-byte utf-8
		{"c2V0ICQudGFnID0gIngiOw==", `set $.tag = "x";`},
	}
	for _, c := range cases {
		got, err := renderB64(t, "{{ v|base64decode }}", map[string]any{"v": c.in})
		if err != nil {
			t.Errorf("%q: unexpected error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("%q: got %q, want %q", c.in, got, c.want)
		}
	}
}

func TestBase64DecodeErrors(t *testing.T) {
	cases := []struct{ in, want string }{
		{"aGVsbG8", "incorrect padding"},
		{"YQ", "incorrect padding"},
		{"YQ=", "incorrect padding"},
		{"aGVsb", "number of data characters (5) cannot be 1 more than a multiple of 4"},
		{"a!b@c#d$e", "number of data characters (5) cannot be 1 more than a multiple of 4"},
		{"/w==", "not valid utf-8"},
		{"aGVsbG8=é", "only ASCII characters"},
	}
	for _, c := range cases {
		_, err := renderB64(t, "{{ v|base64decode }}", map[string]any{"v": c.in})
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%q: got error %v, want one containing %q", c.in, err, c.want)
		}
	}
}

func TestBase64DecodeEncoding(t *testing.T) {
	cases := []struct{ src, want string }{
		{"{{ 'w6XDpMO2'|base64decode('latin-1') }}", "Ã¥Ã¤Ã¶"},
		{"{{ 'w6XDpMO2'|base64decode(encoding='ISO-8859-1') }}", "Ã¥Ã¤Ã¶"},
		{"{{ 'aGVsbG8='|base64decode('ascii') }}", "hello"},
		{"{{ 'aGVsbG8='|base64decode('UTF8') }}", "hello"},
	}
	for _, c := range cases {
		got, err := renderB64(t, c.src, nil)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", c.src, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s: got %q, want %q", c.src, got, c.want)
		}
	}
	if _, err := renderB64(t, "{{ 'aGVsbG8='|base64decode('ebcdic') }}", nil); err == nil || !strings.Contains(err.Error(), "unknown encoding") {
		t.Errorf("got error %v, want unknown encoding", err)
	}
	if _, err := renderB64(t, "{{ 'w6XDpMO2'|base64decode('ascii') }}", nil); err == nil || !strings.Contains(err.Error(), "not valid ascii") {
		t.Errorf("got error %v, want invalid ascii", err)
	}
}

func TestBase64DecodeRejectsNonStrings(t *testing.T) {
	if _, err := renderB64(t, "{{ 42|base64decode }}", nil); err == nil {
		t.Error("expected error for an integer argument")
	}
	if _, err := renderB64(t, "{{ missing|base64decode }}", nil); err == nil {
		t.Error("expected error for an undefined argument")
	}
}

// TestBase64DecodeDefaultChain is the jinjaconf pattern: an unset variable
// defaults to the empty string, which decodes to the empty string.
func TestBase64DecodeDefaultChain(t *testing.T) {
	got, err := renderB64(t, "[{{ missing|default('')|base64decode }}]", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "[]" {
		t.Fatalf("got %q, want %q", got, "[]")
	}
}

func TestBase64DecodeNotRegisteredByDefault(t *testing.T) {
	e, err := environment.New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := e.FromString("{{ 'aGVsbG8='|base64decode }}"); err == nil {
		t.Fatal("expected base64decode to be unknown without WithFilter")
	}
}

func TestLookup(t *testing.T) {
	if _, ok := Lookup("base64decode"); !ok {
		t.Error("base64decode not found")
	}
	if _, ok := Lookup("nope"); ok {
		t.Error("unexpected filter nope")
	}
	if got := strings.Join(Names(), ","); got != "base64decode" {
		t.Errorf("Names() = %q", got)
	}
}
