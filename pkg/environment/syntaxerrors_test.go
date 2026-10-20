package environment

import (
	"context"
	"strings"
	"testing"

	"github.com/jryberg/gojinja/pkg/lexer"
	"github.com/jryberg/gojinja/pkg/loader"
	"github.com/jryberg/gojinja/pkg/varsutil"
)

// TestSyntaxErrorMessages holds syntax error messages for unknown tags, misnested blocks and unexpected ends, checked against Jinja2. Expected values come from the Jinja2 pinned in
// tools/parity/requirements.txt; err is the expected error's message, or
// "*" when only an error is expected.
func TestSyntaxErrorMessages(t *testing.T) {
	cases := []struct {
		name, src, get string
		tpls           map[string]string
		vars           string
		trim           bool
		want, err      string
	}{
		{name: "Syntax1", src: `{% for i in x %}{% endif %}`, get: "", tpls: nil, vars: `{"x": [1]}`, trim: false, want: ``, err: `Encountered unknown tag 'endif'. Jinja was looking for the following tags: 'endfor' or 'else'. The innermost block that needs to be closed is 'for'.`},
		{name: "Syntax2", src: `{% for i in x %}{% if true %}{% endfor %}`, get: "", tpls: nil, vars: `{"x": [1]}`, trim: false, want: ``, err: `Encountered unknown tag 'endfor'. You probably made a nesting mistake. Jinja is expecting this tag, but currently looking for 'elif' or 'else' or 'endif'. The innermost block that needs to be closed is 'if'.`},
		{name: "Syntax3", src: `{% if true %}{% endfor %}`, get: "", tpls: nil, vars: `{"x": [1]}`, trim: false, want: ``, err: `Encountered unknown tag 'endfor'. Jinja was looking for the following tags: 'elif' or 'else' or 'endif'. The innermost block that needs to be closed is 'if'.`},
		{name: "Syntax4", src: `{% foo %}`, get: "", tpls: nil, vars: `{"x": [1]}`, trim: false, want: ``, err: `Encountered unknown tag 'foo'.`},
		{name: "Syntax5", src: `{% for i in x %}{% foo %}{% endfor %}`, get: "", tpls: nil, vars: `{"x": [1]}`, trim: false, want: ``, err: `Encountered unknown tag 'foo'. Jinja was looking for the following tags: 'endfor' or 'else'. The innermost block that needs to be closed is 'for'.`},
		{name: "Syntax6", src: `{% for i in x %}`, get: "", tpls: nil, vars: `{"x": [1]}`, trim: false, want: ``, err: `Unexpected end of template. Jinja was looking for the following tags: 'endfor' or 'else'. The innermost block that needs to be closed is 'for'.`},
		{name: "Syntax7", src: `{% if true %}{% for i in x %}`, get: "", tpls: nil, vars: `{"x": [1]}`, trim: false, want: ``, err: `Unexpected end of template. Jinja was looking for the following tags: 'endfor' or 'else'. The innermost block that needs to be closed is 'for'.`},
		{name: "Syntax8", src: `{% block b %}{% if true %}`, get: "", tpls: nil, vars: `{"x": [1]}`, trim: false, want: ``, err: `Unexpected end of template. Jinja was looking for the following tags: 'elif' or 'else' or 'endif'. The innermost block that needs to be closed is 'if'.`},
		{name: "Syntax9", src: `{% macro m() %}{% endif %}{% endmacro %}`, get: "", tpls: nil, vars: `{"x": [1]}`, trim: false, want: ``, err: `Encountered unknown tag 'endif'. Jinja was looking for the following tags: 'endmacro'. The innermost block that needs to be closed is 'macro'.`},
		{name: "Syntax10", src: `{% if true %}{% elif false %}{% else %}{% elif true %}{% endif %}`, get: "", tpls: nil, vars: `{"x": [1]}`, trim: false, want: ``, err: `Encountered unknown tag 'elif'. Jinja was looking for the following tags: 'endif'. The innermost block that needs to be closed is 'if'.`},
		{name: "Syntax11", src: `{% with a = 1 %}{% endfor %}{% endwith %}`, get: "", tpls: nil, vars: `{"x": [1]}`, trim: false, want: ``, err: `Encountered unknown tag 'endfor'. Jinja was looking for the following tags: 'endwith'. The innermost block that needs to be closed is 'with'.`},
		{name: "Syntax12", src: `{% raw %}x`, get: "", tpls: nil, vars: `{"x": [1]}`, trim: false, want: ``, err: `Missing end of raw directive`},
		{name: "Syntax13", src: `{% if %}{% endif %}`, get: "", tpls: nil, vars: `{"x": [1]}`, trim: false, want: ``, err: `Expected an expression, got 'end of statement block'`},
		{name: "Syntax14", src: `{% endfor %}`, get: "", tpls: nil, vars: `{"x": [1]}`, trim: false, want: ``, err: `Encountered unknown tag 'endfor'.`},
		{name: "Syntax15", src: `{% set x %}{% endfor %}`, get: "", tpls: nil, vars: `{"x": [1]}`, trim: false, want: ``, err: `Encountered unknown tag 'endfor'. Jinja was looking for the following tags: 'endset'. The innermost block that needs to be closed is 'set'.`},
		{name: "Syntax16", src: `{% call foo() %}{% endmacro %}`, get: "", tpls: nil, vars: `{"x": [1]}`, trim: false, want: ``, err: `Encountered unknown tag 'endmacro'. Jinja was looking for the following tags: 'endcall'. The innermost block that needs to be closed is 'call'.`},
		{name: "Syntax17", src: `{% filter upper %}{% endif %}{% endfilter %}`, get: "", tpls: nil, vars: `{"x": [1]}`, trim: false, want: ``, err: `Encountered unknown tag 'endif'. Jinja was looking for the following tags: 'endfilter'. The innermost block that needs to be closed is 'filter'.`},
		{name: "Syntax18", src: `{% autoescape true %}{% endwith %}`, get: "", tpls: nil, vars: `{"x": [1]}`, trim: false, want: ``, err: `Encountered unknown tag 'endwith'. Jinja was looking for the following tags: 'endautoescape'. The innermost block that needs to be closed is 'autoescape'.`},
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
