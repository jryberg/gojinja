package environment

import (
	"context"
	"strings"
	"testing"

	"github.com/jryberg/gojinja/pkg/lexer"
	"github.com/jryberg/gojinja/pkg/loader"
	"github.com/jryberg/gojinja/pkg/varsutil"
)

// TestIncludeErrors holds include, extends and import with missing, undefined and odd template names, checked against Jinja2. Expected values come from the Jinja2 pinned in
// tools/parity/requirements.txt; err is the expected error's message, or
// "*" when only an error is expected.
func TestIncludeErrors(t *testing.T) {
	cases := []struct {
		name, src, get string
		tpls           map[string]string
		vars           string
		trim           bool
		want, err      string
	}{
		{name: "Include1", src: `{% include 'missing' ignore missing %}|`, get: "", tpls: map[string]string{"bad": `{% if %}`, "ok": `OK{{ x }}`, "undef": `{{ nope.attr }}`}, vars: `{"x": 1, "names": ["m"]}`, trim: false, want: `|`, err: ``},
		{name: "Include2", src: `{% include 'bad' ignore missing %}|`, get: "", tpls: map[string]string{"bad": `{% if %}`, "ok": `OK{{ x }}`, "undef": `{{ nope.attr }}`}, vars: `{"x": 1, "names": ["m"]}`, trim: false, want: ``, err: `Expected an expression, got 'end of statement block'`},
		{name: "Include3", src: `{% include ['bad', 'ok'] %}`, get: "", tpls: map[string]string{"bad": `{% if %}`, "ok": `OK{{ x }}`, "undef": `{{ nope.attr }}`}, vars: `{"x": 1, "names": ["m"]}`, trim: false, want: ``, err: `Expected an expression, got 'end of statement block'`},
		{name: "Include4", src: `{% include ['missing', 'ok'] %}`, get: "", tpls: map[string]string{"bad": `{% if %}`, "ok": `OK{{ x }}`, "undef": `{{ nope.attr }}`}, vars: `{"x": 1, "names": ["m"]}`, trim: false, want: `OK1`, err: ``},
		{name: "Include5", src: `{% include ['missing', 'bad'] ignore missing %}`, get: "", tpls: map[string]string{"bad": `{% if %}`, "ok": `OK{{ x }}`, "undef": `{{ nope.attr }}`}, vars: `{"x": 1, "names": ["m"]}`, trim: false, want: ``, err: `Expected an expression, got 'end of statement block'`},
		{name: "Include6", src: `{% include 'undef' ignore missing %}`, get: "", tpls: map[string]string{"bad": `{% if %}`, "ok": `OK{{ x }}`, "undef": `{{ nope.attr }}`}, vars: `{"x": 1, "names": ["m"]}`, trim: false, want: ``, err: `'nope' is undefined`},
		{name: "Include7", src: `{% include [none, 'ok'] %}`, get: "", tpls: map[string]string{"bad": `{% if %}`, "ok": `OK{{ x }}`, "undef": `{{ nope.attr }}`}, vars: `{"x": 1, "names": ["m"]}`, trim: false, want: `OK1`, err: ``},
		{name: "Include8", src: `{% include [] ignore missing %}x`, get: "", tpls: map[string]string{"bad": `{% if %}`, "ok": `OK{{ x }}`, "undef": `{{ nope.attr }}`}, vars: `{"x": 1, "names": ["m"]}`, trim: false, want: `x`, err: ``},
		{name: "Include9", src: `{% include [] %}`, get: "", tpls: map[string]string{"bad": `{% if %}`, "ok": `OK{{ x }}`, "undef": `{{ nope.attr }}`}, vars: `{"x": 1, "names": ["m"]}`, trim: false, want: ``, err: `Tried to select from an empty list of templates.`},
		{name: "Include10", src: `{% include 'missing' %}`, get: "", tpls: map[string]string{"bad": `{% if %}`, "ok": `OK{{ x }}`, "undef": `{{ nope.attr }}`}, vars: `{"x": 1, "names": ["m"]}`, trim: false, want: ``, err: `missing`},
		{name: "Include11", src: `{% include ['a', 'b'] %}`, get: "", tpls: map[string]string{"bad": `{% if %}`, "ok": `OK{{ x }}`, "undef": `{{ nope.attr }}`}, vars: `{"x": 1, "names": ["m"]}`, trim: false, want: ``, err: `none of the templates given were found: a, b`},
		{name: "Include12", src: `{% include u %}`, get: "", tpls: map[string]string{"bad": `{% if %}`, "ok": `OK{{ x }}`, "undef": `{{ nope.attr }}`}, vars: `{"x": 1, "names": ["m"]}`, trim: false, want: ``, err: `'u' is undefined`},
		{name: "Include13", src: `{% include u ignore missing %}x`, get: "", tpls: map[string]string{"bad": `{% if %}`, "ok": `OK{{ x }}`, "undef": `{{ nope.attr }}`}, vars: `{"x": 1, "names": ["m"]}`, trim: false, want: ``, err: `'u' is undefined`},
		{name: "Include14", src: `{% include [u, 'ok'] %}`, get: "", tpls: map[string]string{"bad": `{% if %}`, "ok": `OK{{ x }}`, "undef": `{{ nope.attr }}`}, vars: `{"x": 1, "names": ["m"]}`, trim: false, want: `OK1`, err: ``},
		{name: "Include15", src: `{% include [u] %}`, get: "", tpls: map[string]string{"bad": `{% if %}`, "ok": `OK{{ x }}`, "undef": `{{ nope.attr }}`}, vars: `{"x": 1, "names": ["m"]}`, trim: false, want: ``, err: `none of the templates given were found: 'u' is undefined`},
		{name: "Include16", src: `{% include [u] ignore missing %}x`, get: "", tpls: map[string]string{"bad": `{% if %}`, "ok": `OK{{ x }}`, "undef": `{{ nope.attr }}`}, vars: `{"x": 1, "names": ["m"]}`, trim: false, want: `x`, err: ``},
		{name: "Include17", src: `{% include 5 %}`, get: "", tpls: map[string]string{"bad": `{% if %}`, "ok": `OK{{ x }}`, "undef": `{{ nope.attr }}`}, vars: `{"x": 1, "names": ["m"]}`, trim: false, want: ``, err: `'int' object is not iterable`},
		{name: "Include18", src: `{% include [5, 'ok'] %}`, get: "", tpls: map[string]string{"bad": `{% if %}`, "ok": `OK{{ x }}`, "undef": `{{ nope.attr }}`}, vars: `{"x": 1, "names": ["m"]}`, trim: false, want: `OK1`, err: ``},
		{name: "Include19", src: `{% include ('m', 'n') %}`, get: "", tpls: map[string]string{"bad": `{% if %}`, "ok": `OK{{ x }}`, "undef": `{{ nope.attr }}`}, vars: `{"x": 1, "names": ["m"]}`, trim: false, want: ``, err: `none of the templates given were found: m, n`},
		{name: "Include20", src: `{% include ['m', none] %}`, get: "", tpls: map[string]string{"bad": `{% if %}`, "ok": `OK{{ x }}`, "undef": `{{ nope.attr }}`}, vars: `{"x": 1, "names": ["m"]}`, trim: false, want: ``, err: `none of the templates given were found: m, None`},
		{name: "Include21", src: `{% include names %}`, get: "", tpls: map[string]string{"bad": `{% if %}`, "ok": `OK{{ x }}`, "undef": `{{ nope.attr }}`}, vars: `{"x": 1, "names": ["m"]}`, trim: false, want: ``, err: `none of the templates given were found: m`},
		{name: "Include22", src: `{% include 'ok' ~ '' %}|{% include ('ok'|safe) %}`, get: "", tpls: map[string]string{"bad": `{% if %}`, "ok": `OK{{ x }}`, "undef": `{{ nope.attr }}`}, vars: `{"x": 1, "names": ["m"]}`, trim: false, want: `OK1|OK1`, err: ``},
		{name: "Include23", src: `{% import u as m %}`, get: "", tpls: map[string]string{"bad": `{% if %}`, "ok": `OK{{ x }}`, "undef": `{{ nope.attr }}`}, vars: `{"x": 1, "names": ["m"]}`, trim: false, want: ``, err: `'u' is undefined`},
		{name: "Include24", src: `{% extends u %}`, get: "", tpls: map[string]string{"bad": `{% if %}`, "ok": `OK{{ x }}`, "undef": `{{ nope.attr }}`}, vars: `{"x": 1, "names": ["m"]}`, trim: false, want: ``, err: `'u' is undefined`},
		{name: "Include25", src: `{% extends ['ok'] %}`, get: "", tpls: map[string]string{"bad": `{% if %}`, "ok": `OK{{ x }}`, "undef": `{{ nope.attr }}`}, vars: `{"x": 1, "names": ["m"]}`, trim: false, want: ``, err: `unhashable type: 'list'`},
		{name: "Include26", src: `{% extends 5 %}`, get: "", tpls: map[string]string{"bad": `{% if %}`, "ok": `OK{{ x }}`, "undef": `{{ nope.attr }}`}, vars: `{"x": 1, "names": ["m"]}`, trim: false, want: ``, err: `5`},
		{name: "Include27", src: `{% extends none %}`, get: "", tpls: map[string]string{"bad": `{% if %}`, "ok": `OK{{ x }}`, "undef": `{{ nope.attr }}`}, vars: `{"x": 1, "names": ["m"]}`, trim: false, want: ``, err: `None`},
		{name: "Include28", src: `{% import ['ok'] as m %}`, get: "", tpls: map[string]string{"bad": `{% if %}`, "ok": `OK{{ x }}`, "undef": `{{ nope.attr }}`}, vars: `{"x": 1, "names": ["m"]}`, trim: false, want: ``, err: `unhashable type: 'list'`},
		{name: "Include29", src: `{% from ('m', 'n') import a %}`, get: "", tpls: map[string]string{"bad": `{% if %}`, "ok": `OK{{ x }}`, "undef": `{{ nope.attr }}`}, vars: `{"x": 1, "names": ["m"]}`, trim: false, want: ``, err: `('m', 'n')`},
		{name: "Include30", src: `{% import 'ok' as m %}{{ m }}`, get: "", tpls: map[string]string{"bad": `{% if %}`, "ok": `OK{{ x }}`, "undef": `{{ nope.attr }}`}, vars: `{"x": 1, "names": ["m"]}`, trim: false, want: `OK`, err: ``},
		{name: "Include31", src: `{% from 'missing' import a %}`, get: "", tpls: map[string]string{"bad": `{% if %}`, "ok": `OK{{ x }}`, "undef": `{{ nope.attr }}`}, vars: `{"x": 1, "names": ["m"]}`, trim: false, want: ``, err: `missing`},
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
