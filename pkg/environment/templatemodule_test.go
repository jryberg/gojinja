package environment

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/jryberg/gojinja/pkg/lexer"
	"github.com/jryberg/gojinja/pkg/loader"
	"github.com/jryberg/gojinja/pkg/varsutil"
)

// TestTemplateModule holds the module an import binds: exported names as attributes, printed as the template's output, cached per render, checked against Jinja2. Expected values come from the Jinja2 pinned in
// tools/parity/requirements.txt; err is the expected error's message, or
// "*" when only an error is expected. Object addresses in reprs are
// masked.
func TestTemplateModule(t *testing.T) {
	cases := []struct {
		name, src, get string
		tpls           map[string]string
		vars           string
		trim           bool
		want, err      string
	}{
		{name: "Module1", src: `{% import 'mod' as m %}{{ m }}|{{ [m] }}|{{ m.q }}|{{ m['q'] }}|{{ m.m() }}|{{ m.inner }}|{{ m.ns }}|{{ m.zz }}|{{ m is mapping }}|{{ m is defined }}|{{ m|string }}|{{ m is callable }}`, get: "", tpls: map[string]string{"mod": `Body<{{ 1 }}>{% set _p = 1 %}{% set q = 2 %}{% macro m() %}M{% endmacro %}{% macro _h() %}{% endmacro %}{% set ns = namespace(n=0) %}{% for i in [1] %}{% set inner = 3 %}{% endfor %}`}, vars: `{}`, trim: false, want: `Body<1>|[<TemplateModule 'mod'>]|2|2|M||<Namespace {'n': 0}>||False|True|Body<1>|False`, err: ``},
		{name: "Module2", src: `{% import 'mod' as m %}{{ m|length }}`, get: "", tpls: map[string]string{"mod": `Body<{{ 1 }}>{% set _p = 1 %}{% set q = 2 %}{% macro m() %}M{% endmacro %}{% macro _h() %}{% endmacro %}{% set ns = namespace(n=0) %}{% for i in [1] %}{% set inner = 3 %}{% endfor %}`}, vars: `{}`, trim: false, want: ``, err: `object of type 'TemplateModule' has no len()`},
		{name: "Module3", src: `{% import 'mod' as m %}{{ m|list }}`, get: "", tpls: map[string]string{"mod": `Body<{{ 1 }}>{% set _p = 1 %}{% set q = 2 %}{% macro m() %}M{% endmacro %}{% macro _h() %}{% endmacro %}{% set ns = namespace(n=0) %}{% for i in [1] %}{% set inner = 3 %}{% endfor %}`}, vars: `{}`, trim: false, want: ``, err: `'TemplateModule' object is not iterable`},
		{name: "Module4", src: `{% import 'mod' as m %}{{ m._p }}|{{ m._h }}|{{ m.__name__ }}|{{ m.items }}`, get: "", tpls: map[string]string{"mod": `Body<{{ 1 }}>{% set _p = 1 %}{% set q = 2 %}{% macro m() %}M{% endmacro %}{% macro _h() %}{% endmacro %}{% set ns = namespace(n=0) %}{% for i in [1] %}{% set inner = 3 %}{% endfor %}`}, vars: `{}`, trim: false, want: `|||`, err: ``},
		{name: "Module5", src: `{% import 'mod' as m %}{{ m._p.x }}`, get: "", tpls: map[string]string{"mod": `Body<{{ 1 }}>{% set _p = 1 %}{% set q = 2 %}{% macro m() %}M{% endmacro %}{% macro _h() %}{% endmacro %}{% set ns = namespace(n=0) %}{% for i in [1] %}{% set inner = 3 %}{% endfor %}`}, vars: `{}`, trim: false, want: ``, err: `'jinja2.environment.TemplateModule object' has no attribute '_p'`},
		{name: "Module6", src: `{% import 'mod' as m %}{{ m|tojson }}`, get: "", tpls: map[string]string{"mod": `Body<{{ 1 }}>{% set _p = 1 %}{% set q = 2 %}{% macro m() %}M{% endmacro %}{% macro _h() %}{% endmacro %}{% set ns = namespace(n=0) %}{% for i in [1] %}{% set inner = 3 %}{% endfor %}`}, vars: `{}`, trim: false, want: ``, err: `Object of type TemplateModule is not JSON serializable`},
		{name: "Module7", src: `{% import 'mod' as m %}{{ m == m }}|{{ m|pprint }}|{{ m|e }}`, get: "", tpls: map[string]string{"mod": `Body<{{ 1 }}>{% set _p = 1 %}{% set q = 2 %}{% macro m() %}M{% endmacro %}{% macro _h() %}{% endmacro %}{% set ns = namespace(n=0) %}{% for i in [1] %}{% set inner = 3 %}{% endfor %}`}, vars: `{}`, trim: false, want: `True|<TemplateModule 'mod'>|Body<1>`, err: ``},
		{name: "Module8", src: `{% import 'mod' as m with context %}{{ m }}`, get: "", tpls: map[string]string{"mod": `Body<{{ 1 }}>{% set _p = 1 %}{% set q = 2 %}{% macro m() %}M{% endmacro %}{% macro _h() %}{% endmacro %}{% set ns = namespace(n=0) %}{% for i in [1] %}{% set inner = 3 %}{% endfor %}`}, vars: `{}`, trim: false, want: `Body<1>`, err: ``},
		{name: "Module9", src: `{% import 'mod' as m %}{% import 'mod' as n %}{{ m is sameas n }}|{% set m.ns.n = 5 %}{{ n.ns.n }}`, get: "", tpls: map[string]string{"mod": `Body<{{ 1 }}>{% set _p = 1 %}{% set q = 2 %}{% macro m() %}M{% endmacro %}{% macro _h() %}{% endmacro %}{% set ns = namespace(n=0) %}{% for i in [1] %}{% set inner = 3 %}{% endfor %}`}, vars: `{}`, trim: false, want: ``, err: `expected token 'end of statement block', got '.'`},
		{name: "Module10", src: `{% import 'mod' as m with context %}{% import 'mod' as n with context %}{{ m is sameas n }}`, get: "", tpls: map[string]string{"mod": `Body<{{ 1 }}>{% set _p = 1 %}{% set q = 2 %}{% macro m() %}M{% endmacro %}{% macro _h() %}{% endmacro %}{% set ns = namespace(n=0) %}{% for i in [1] %}{% set inner = 3 %}{% endfor %}`}, vars: `{}`, trim: false, want: `False`, err: ``},
		{name: "Module11", src: `{% import 'mod' as m %}{{ {m: 1}|length }}|{{ m ~ '!' }}|{{ '%s' % m }}|{{ '{}'.format(m) }}`, get: "", tpls: map[string]string{"mod": `Body<{{ 1 }}>{% set _p = 1 %}{% set q = 2 %}{% macro m() %}M{% endmacro %}{% macro _h() %}{% endmacro %}{% set ns = namespace(n=0) %}{% for i in [1] %}{% set inner = 3 %}{% endfor %}`}, vars: `{}`, trim: false, want: `1|Body<1>!|Body<1>|Body<1>`, err: ``},
	}
	addr := regexp.MustCompile(` at 0x[0-9a-f]+>`)
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
			out = addr.ReplaceAllString(out, " at 0x…>")
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
