package environment

import (
	"context"
	"strings"
	"testing"

	"github.com/jryberg/gojinja/pkg/lexer"
	"github.com/jryberg/gojinja/pkg/loader"
	"github.com/jryberg/gojinja/pkg/varsutil"
)

// TestUnknownNames holds unknown filters and tests: a compile-time error, or a runtime one inside if and conditional expressions, checked against Jinja2. Expected values come from the Jinja2 pinned in
// tools/parity/requirements.txt; err is the expected error's message, or
// "*" when only an error is expected.
func TestUnknownNames(t *testing.T) {
	cases := []struct {
		name, src, get string
		tpls           map[string]string
		vars           string
		trim           bool
		want, err      string
	}{
		{name: "Names1", src: `{{ x|f }}`, get: "", tpls: nil, vars: `{"x": 1}`, trim: false, want: ``, err: `No filter named 'f'.`},
		{name: "Names2", src: `{% if false %}{{ x|f }}{% endif %}ok`, get: "", tpls: nil, vars: `{"x": 1}`, trim: false, want: `ok`, err: ``},
		{name: "Names3", src: `{{ 1 if false else x|f }}`, get: "", tpls: nil, vars: `{"x": 1}`, trim: false, want: ``, err: `No filter named 'f' found.`},
		{name: "Names4", src: `{% filter f %}a{% endfilter %}`, get: "", tpls: nil, vars: `{"x": 1}`, trim: false, want: ``, err: `No filter named 'f'.`},
		{name: "Names5", src: `{% if true %}{% filter f %}a{% endfilter %}{% endif %}`, get: "", tpls: nil, vars: `{"x": 1}`, trim: false, want: ``, err: `No filter named 'f'.`},
		{name: "Names6", src: `{% for i in [] %}{{ i|f }}{% endfor %}`, get: "", tpls: nil, vars: `{"x": 1}`, trim: false, want: ``, err: `No filter named 'f'.`},
		{name: "Names7", src: `{% macro m() %}{{ 1|f }}{% endmacro %}`, get: "", tpls: nil, vars: `{"x": 1}`, trim: false, want: ``, err: `No filter named 'f'.`},
		{name: "Names8", src: `{% if true %}{% for i in [1] %}{{ i|f }}{% endfor %}{% endif %}`, get: "", tpls: nil, vars: `{"x": 1}`, trim: false, want: ``, err: `No filter named 'f'.`},
		{name: "Names9", src: `{{ x is f }}`, get: "", tpls: nil, vars: `{"x": 1}`, trim: false, want: ``, err: `No test named 'f'.`},
		{name: "Names10", src: `{% if x is f %}{% endif %}`, get: "", tpls: nil, vars: `{"x": 1}`, trim: false, want: ``, err: `No test named 'f' found.`},
		{name: "Names11", src: `{% set y = x|f if false %}ok`, get: "", tpls: nil, vars: `{"x": 1}`, trim: false, want: `ok`, err: ``},
		{name: "Names12", src: `{% if true %}{% set y = 1|f %}{% endif %}`, get: "", tpls: nil, vars: `{"x": 1}`, trim: false, want: ``, err: `No filter named 'f' found.`},
		{name: "Names13", src: `{% if true %}{% set y | f %}a{% endset %}{% endif %}`, get: "", tpls: nil, vars: `{"x": 1}`, trim: false, want: ``, err: `No filter named 'f'.`},
		{name: "Names14", src: `{% if true %}{% with a = 1|f %}{% endwith %}{% endif %}`, get: "", tpls: nil, vars: `{"x": 1}`, trim: false, want: ``, err: `No filter named 'f' found.`},
		{name: "Names15", src: `{% if true %}{% with a = 1 %}{{ a|f }}{% endwith %}{% endif %}`, get: "", tpls: nil, vars: `{"x": 1}`, trim: false, want: ``, err: `No filter named 'f'.`},
		{name: "Names16", src: `{% if false %}{% with a = 1|f %}{% endwith %}{% endif %}ok`, get: "", tpls: nil, vars: `{"x": 1}`, trim: false, want: `ok`, err: ``},
		{name: "Names17", src: `{{ [1]|map('f')|list }}`, get: "", tpls: nil, vars: `{"x": 1}`, trim: false, want: ``, err: `No filter named 'f'.`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := mustEnv(t, WithAutoescape(AutoescapeNever{}), WithLoader(loader.Dict(c.tpls)), WithLexerOptions(lexer.Options{TrimBlocks: c.trim}))
			vars, err := varsutil.JSONVars([]byte(c.vars))
			if err != nil {
				t.Fatal(err)
			}
			var tpl *Template
			if c.get != "" {
				tpl, err = e.GetTemplate(c.get)
			} else {
				tpl, err = e.FromString(c.src)
			}
			out := ""
			if err == nil {
				out, err = tpl.RenderContext(context.Background(), vars)
			}
			switch {
			case c.err == "*" && err == nil:
				t.Errorf("got %q; want an error", out)
			case c.err != "" && c.err != "*" && (err == nil || !strings.Contains(err.Error(), c.err)):
				t.Errorf("got %q, %v; want error containing %q", out, err, c.err)
			case c.err == "" && err != nil:
				t.Errorf("unexpected error: %v", err)
			case c.err == "" && out != c.want:
				t.Errorf("got %q, want %q", out, c.want)
			}
		})
	}
}
