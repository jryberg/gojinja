package environment

import (
	"context"
	"strings"
	"testing"

	"github.com/jryberg/gojinja/pkg/runtime"
)

// Expected values come from rendering the same templates with the Jinja2
// pinned in tools/parity/requirements.txt. d is an empty dict and n is
// None.

// DebugUndefined prints Python's placeholders, naming Python types and
// keeping non-string keys, and carries the hints Python attaches.
func TestDebugUndefinedOutput(t *testing.T) {
	cases := []struct{ src, want string }{
		{`{{ q }}`, "{{ q }}"},
		{`{{ d.x }}`, "{{ no such element: dict object['x'] }}"},
		{`{{ d['x'] }}`, "{{ no such element: dict object['x'] }}"},
		{`{{ [1][5] }}`, "{{ no such element: list object[5] }}"},
		{`{{ [1]['x'] }}`, "{{ no such element: list object['x'] }}"},
		{`{{ none.x }}`, "{{ no such element: None['x'] }}"},
		{`{{ n.x }}`, "{{ no such element: None['x'] }}"},
		{`{{ 'ab'.zz }}`, "{{ no such element: str object['zz'] }}"},
		{`{{ 5.x }}`, "{{ no such element: int object['x'] }}"},
		{`{{ d[1] }}`, "{{ no such element: dict object[1] }}"},
		{`{{ (1,)[3] }}`, "{{ no such element: tuple object[3] }}"},
		{`{{ [1, 2][true] }}`, "2"},
		{`{{ [1][1.0] }}`, "{{ no such element: list object[1.0] }}"},
		{`{{ [3, 1]['index'](1) }}`, "1"},
		{`{{ 'a' if false }}`, ""},
		{`{{ []|first }}`, "{{ undefined value printed: No first item, sequence was empty. }}"},
		{`{{ []|max }}`, "{{ undefined value printed: No aggregated item, sequence was empty. }}"},
		{`{% macro m(a) %}{{ a }}{% endmacro %}{{ m() }}`, "{{ undefined value printed: parameter 'a' was not provided }}"},
		{`{{ loop }}`, "{{ loop }}"},
	}
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}), WithUndefined(runtime.NewDebug))
	for _, c := range cases {
		if got := render(t, e, c.src, map[string]any{"d": map[string]any{}, "n": nil}); got != c.want {
			t.Errorf("%s = %q, want %q", c.src, got, c.want)
		}
	}
}

// Undefined errors use Python's wording.
func TestUndefinedErrorMessages(t *testing.T) {
	cases := []struct{ src, msg string }{
		{`{{ q + 1 }}`, "'q' is undefined"},
		{`{{ d.x + 1 }}`, "'dict object' has no attribute 'x'"},
		{`{{ d[1] + 1 }}`, "dict object has no element 1"},
		{`{{ [1][5] + 1 }}`, "list object has no element 5"},
		{`{{ n.x + 1 }}`, "'None' has no attribute 'x'"},
		{`{{ 'ab'.zz + 1 }}`, "'str object' has no attribute 'zz'"},
		{`{{ q.x }}`, "'q' is undefined"},
		{`{{ ('a' if false) + 1 }}`, "the inline if-expression on line 1 evaluated to false and no else section was defined."},
		{`{% macro m(a) %}{{ a + 1 }}{% endmacro %}{{ m() }}`, "parameter 'a' was not provided"},
	}
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	for _, c := range cases {
		tpl, err := e.FromString(c.src)
		if err != nil {
			t.Fatalf("FromString(%q): %v", c.src, err)
		}
		_, err = tpl.RenderContext(context.Background(), map[string]any{"d": map[string]any{}, "n": nil})
		if err == nil || !strings.HasSuffix(err.Error(), c.msg) {
			t.Errorf("%s: got error %v, want %q", c.src, err, c.msg)
		}
	}
}

// Port of jinja2.utils.urlize: punctuation, bracket balancing, schemes,
// IP hosts, Unicode whitespace and extra_schemes.
func TestUrlizePort(t *testing.T) {
	cases := []struct {
		src     string
		want    string
		wantErr bool
	}{
		{`{{ 'see (http://a.com/x).'|urlize }}`, "see (<a href=\"http://a.com/x\" rel=\"noopener\">http://a.com/x</a>).", false},
		{`{{ 'see <http://a.com>'|urlize }}`, "see &lt;<a href=\"http://a.com\" rel=\"noopener\">http://a.com</a>&gt;", false},
		{`{{ 'go www.a.com, now'|urlize }}`, "go <a href=\"https://www.a.com\" rel=\"noopener\">www.a.com</a>, now", false},
		{`{{ 'mail mailto:a@b.com and x@y.org.'|urlize }}`, "mail <a href=\"mailto:a@b.com\">a@b.com</a> and <a href=\"mailto:x@y.org\">x@y.org</a>.", false},
		{`{{ 'a.com b.co.uk c.dev d.local'|urlize }}`, "a.com b.co.uk c.dev d.local", false},
		{`{{ 'http://a.com/path?q=1&r=2#f'|urlize }}`, "<a href=\"http://a.com/path?q=1&amp;r=2#f\" rel=\"noopener\">http://a.com/path?q=1&amp;r=2#f</a>", false},
		{`{{ 'https://a.com/(x)'|urlize }}`, "<a href=\"https://a.com/(x)\" rel=\"noopener\">https://a.com/(x)</a>", false},
		{`{{ 'ftp://a.com'|urlize }}`, "ftp://a.com", false},
		{`{{ '1.2.3.4 http://1.2.3.4/x'|urlize }}`, "1.2.3.4 <a href=\"http://1.2.3.4/x\" rel=\"noopener\">http://1.2.3.4/x</a>", false},
		{`{{ 'a@b'|urlize }}`, "a@b", false},
		{`{{ 'x <script>alert(1)</script> http://a.com'|urlize }}`, "x &lt;script&gt;alert(1)&lt;/script&gt; <a href=\"http://a.com\" rel=\"noopener\">http://a.com</a>", false},
		{`{{ 'http://a.com/'|urlize(5) }}`, "<a href=\"http://a.com/\" rel=\"noopener\">http:...</a>", false},
		{`{{ 'foo example.org bar'|urlize }}`, "foo <a href=\"https://example.org\" rel=\"noopener\">example.org</a> bar", false},
		{`{{ 'tel:123 http://a.com'|urlize }}`, "tel:123 <a href=\"http://a.com\" rel=\"noopener\">http://a.com</a>", false},
		{`{{ 'trailing http://a.com/x),'|urlize }}`, "trailing <a href=\"http://a.com/x\" rel=\"noopener\">http://a.com/x</a>),", false},
		{`{{ 'café.com'|urlize }}`, "<a href=\"https://caf\u00e9.com\" rel=\"noopener\">caf\u00e9.com</a>", false},
		{`{{ 'ftp://a.com and tel:123 x'|urlize(extra_schemes=['ftp://', 'tel:']) }}`, "<a href=\"ftp://a.com\" rel=\"noopener\">ftp://a.com</a> and <a href=\"tel:123\" rel=\"noopener\">tel:123</a> x", false},
		{`{{ 'x'|urlize(extra_schemes=['bad scheme']) }}`, "", true},
		{`{{ 'go https://a.com/' ~ 'x' * 30|urlize(10) }}`, "", true},
		{`{{ 'http://[::1]:80/x https://10.0.0.1:8080 www.xn--bcher-kva.example'|urlize }}`, "<a href=\"http://[::1]:80/x\" rel=\"noopener\">http://[::1]:80/x</a> <a href=\"https://10.0.0.1:8080\" rel=\"noopener\">https://10.0.0.1:8080</a> <a href=\"https://www.xn--bcher-kva.example\" rel=\"noopener\">www.xn--bcher-kva.example</a>", false},
		{`{{ 'a  b\tc\nhttp://a.com\u00a0x'|urlize }}`, "a  b\tc\n<a href=\"http://a.com\" rel=\"noopener\">http://a.com</a>\u00a0x", false},
		{`{{ ('<b>' ~ 'http://a.com')|urlize }}`, "&lt;b&gt;http://a.com", false},
		{`{{ 'http://a.com'|e|urlize }}`, "<a href=\"http://a.com\" rel=\"noopener\">http://a.com</a>", false},
	}
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	for _, c := range cases {
		tpl, err := e.FromString(c.src)
		if err != nil {
			t.Fatalf("FromString(%q): %v", c.src, err)
		}
		got, err := tpl.RenderContext(context.Background(), nil)
		switch {
		case c.wantErr && err == nil:
			t.Errorf("%s = %q, want an error", c.src, got)
		case !c.wantErr && err != nil:
			t.Errorf("%s: unexpected error %v", c.src, err)
		case !c.wantErr && got != c.want:
			t.Errorf("%s = %q, want %q", c.src, got, c.want)
		}
	}
}

// random picks independently on every call, like random.choice.
func TestRandomPicksIndependently(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	seen := map[string]bool{}
	for range 200 {
		got := render(t, e, `{{ ['a', 'b', 'c']|random }}`, nil)
		if got != "a" && got != "b" && got != "c" {
			t.Fatalf("random picked %q", got)
		}
		seen[got] = true
	}
	if len(seen) < 2 {
		t.Errorf("200 picks from three items all returned %v", seen)
	}
	if got := render(t, e, `{{ []|random }}`, nil); got != "" {
		t.Errorf("random of an empty list = %q, want empty", got)
	}
}
