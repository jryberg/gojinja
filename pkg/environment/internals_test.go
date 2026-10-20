package environment

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/jryberg/gojinja/pkg/lexer"
	"github.com/jryberg/gojinja/pkg/loader"
	"github.com/jryberg/gojinja/pkg/runtime"
	"github.com/jryberg/gojinja/pkg/varsutil"
)

// TestUnsandboxedInternals holds the Python internals unsandboxed templates read, __class__ and an Undefined's _undefined_name, checked against Jinja2's Environment. Expected values come from the Jinja2 pinned in
// tools/parity/requirements.txt; err is the expected error's message, or
// "*" when only an error is expected. Object addresses in reprs are
// masked.
func TestUnsandboxedInternals(t *testing.T) {
	cases := []struct {
		name, src, get string
		tpls           map[string]string
		vars           string
		trim           bool
		opts           []Option
		want, err      string
	}{
		{name: "InternalsDefault1", src: `{{ ''.__class__ }}|{{ ''.__class__.__name__ }}|{{ (''|safe).__class__ }}|{{ (''|safe).__class__.__name__ }}|{{ (''|safe).__class__.__module__ }}|{{ 1 .__class__.__name__ }}|{{ none.__class__ }}|{{ [].__class__.__qualname__ }}|{{ {}.__class__.__module__ }}|{{ cycler(1).__class__ }}|{{ cycler(1).__class__.__name__ }}|{{ cycler.__name__ }}|{{ cycler.__module__ }}|{{ u.__class__.__name__ }}|{{ (1, 2).__class__.__name__ }}|{{ 1.5.__class__ }}|{{ true.__class__.__name__ }}|{{ ([1]|select).__class__.__name__ }}|{{ range(2).__class__.__name__ }}|{{ ''.__class__.__class__ }}`, get: "", tpls: nil, vars: `{"x": {}}`, trim: false, opts: []Option{WithUnsafe()}, want: `<class 'str'>|str|<class 'markupsafe.Markup'>|Markup|markupsafe|int|<class 'NoneType'>|list|builtins|<class 'jinja2.utils.Cycler'>|Cycler|Cycler|jinja2.utils|Undefined|tuple|<class 'float'>|bool|generator|range|<class 'type'>`, err: ``},
		{name: "InternalsDefault2", src: `{{ foo._undefined_name }}|{{ foo.bar._undefined_name }}|{{ foo._undefined_hint }}|{{ x.y._undefined_name }}`, get: "", tpls: nil, vars: `{"x": {}}`, trim: false, opts: []Option{WithUnsafe()}, want: ``, err: `'foo' is undefined`},
		{name: "InternalsDefault3", src: `{{ foo.bar['baz']._undefined_name }}`, get: "", tpls: nil, vars: `{"x": {}}`, trim: false, opts: []Option{WithUnsafe()}, want: ``, err: `'foo' is undefined`},
		{name: "InternalsDefault4", src: `{{ range }}`, get: "", tpls: nil, vars: `{"x": {}}`, trim: false, opts: []Option{WithUnsafe()}, want: `<class 'range'>`, err: ``},
		{name: "InternalsChainable5", src: `{{ ''.__class__ }}|{{ ''.__class__.__name__ }}|{{ (''|safe).__class__ }}|{{ (''|safe).__class__.__name__ }}|{{ (''|safe).__class__.__module__ }}|{{ 1 .__class__.__name__ }}|{{ none.__class__ }}|{{ [].__class__.__qualname__ }}|{{ {}.__class__.__module__ }}|{{ cycler(1).__class__ }}|{{ cycler(1).__class__.__name__ }}|{{ cycler.__name__ }}|{{ cycler.__module__ }}|{{ u.__class__.__name__ }}|{{ (1, 2).__class__.__name__ }}|{{ 1.5.__class__ }}|{{ true.__class__.__name__ }}|{{ ([1]|select).__class__.__name__ }}|{{ range(2).__class__.__name__ }}|{{ ''.__class__.__class__ }}`, get: "", tpls: nil, vars: `{"x": {}}`, trim: false, opts: []Option{WithUnsafe(), WithUndefined(runtime.NewChainable)}, want: `<class 'str'>|str|<class 'markupsafe.Markup'>|Markup|markupsafe|int|<class 'NoneType'>|list|builtins|<class 'jinja2.utils.Cycler'>|Cycler|Cycler|jinja2.utils|ChainableUndefined|tuple|<class 'float'>|bool|generator|range|<class 'type'>`, err: ``},
		{name: "InternalsChainable6", src: `{{ foo._undefined_name }}|{{ foo.bar._undefined_name }}|{{ foo._undefined_hint }}|{{ x.y._undefined_name }}`, get: "", tpls: nil, vars: `{"x": {}}`, trim: false, opts: []Option{WithUnsafe(), WithUndefined(runtime.NewChainable)}, want: `foo|foo|None|y`, err: ``},
		{name: "InternalsChainable7", src: `{{ foo.bar['baz']._undefined_name }}`, get: "", tpls: nil, vars: `{"x": {}}`, trim: false, opts: []Option{WithUnsafe(), WithUndefined(runtime.NewChainable)}, want: `foo`, err: ``},
		{name: "InternalsChainable8", src: `{{ range }}`, get: "", tpls: nil, vars: `{"x": {}}`, trim: false, opts: []Option{WithUnsafe(), WithUndefined(runtime.NewChainable)}, want: `<class 'range'>`, err: ``},
	}
	addr := regexp.MustCompile(` at 0x[0-9a-f]+(>|&gt;)`)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := mustEnv(t, append([]Option{WithAutoescape(AutoescapeNever{}), WithLoader(loader.Dict(c.tpls)), WithLexerOptions(lexer.Options{TrimBlocks: c.trim})}, c.opts...)...)
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
			out = addr.ReplaceAllString(out, " at 0x…$1")
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
