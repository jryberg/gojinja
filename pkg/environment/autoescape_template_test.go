package environment

import (
	"context"
	"strings"
	"testing"

	"github.com/jryberg/gojinja/pkg/lexer"
	"github.com/jryberg/gojinja/pkg/loader"
	"github.com/jryberg/gojinja/pkg/varsutil"
)

// TestAutoescapePerTemplate holds each included or imported template's own autoescape setting, and macro results wrapped by the caller's, checked against Jinja2 with select_autoescape(['html']). Expected values come from the Jinja2 pinned in
// tools/parity/requirements.txt; err is the expected error's message, or
// "*" when only an error is expected.
func TestAutoescapePerTemplate(t *testing.T) {
	cases := []struct {
		name, src, get string
		tpls           map[string]string
		vars           string
		trim           bool
		aeExt          bool
		want, err      string
	}{
		{name: "AutoescapePerTemplate1", src: ``, get: "page.html", tpls: map[string]string{"page.html": `[{% include "part.txt" %}]({% import "m.txt" as mm %}{{ mm.mac() }})({{ mm }}){% from "m.txt" import mac %}{{ mac() }}|{% include "part.html" %}`, "part.txt": `{{ "<x>" }}`, "part.html": `{{ "<h>" }}`, "m.txt": `{% macro mac() %}{{ "<m>" }}{% endmacro %}T<{{ "<b>" }}>`, "page.txt": `[{% include "part.html" %}]{% import "m.html" as mh %}{{ mh.mac() }}|{% from "m.html" import mac %}{{ mac() ~ "<" }}`, "m.html": `{% macro mac() %}{{ "<m>" }}{% endmacro %}`, "caller.html": `{% macro w() %}<{{ caller() }}>{% endmacro %}{% call w() %}{{ "<c>" }}{% endcall %}|{% include "x.txt" with context %}|{% include "x.txt" without context %}`, "x.txt": `{{ "<i>" }}{% autoescape true %}{{ "<a>" }}{% endautoescape %}`}, vars: `{}`, trim: false, aeExt: true, want: `[<x>](<m>)(T<<b>>)<m>|&lt;h&gt;`, err: ``},
		{name: "AutoescapePerTemplate2", src: ``, get: "page.txt", tpls: map[string]string{"page.html": `[{% include "part.txt" %}]({% import "m.txt" as mm %}{{ mm.mac() }})({{ mm }}){% from "m.txt" import mac %}{{ mac() }}|{% include "part.html" %}`, "part.txt": `{{ "<x>" }}`, "part.html": `{{ "<h>" }}`, "m.txt": `{% macro mac() %}{{ "<m>" }}{% endmacro %}T<{{ "<b>" }}>`, "page.txt": `[{% include "part.html" %}]{% import "m.html" as mh %}{{ mh.mac() }}|{% from "m.html" import mac %}{{ mac() ~ "<" }}`, "m.html": `{% macro mac() %}{{ "<m>" }}{% endmacro %}`, "caller.html": `{% macro w() %}<{{ caller() }}>{% endmacro %}{% call w() %}{{ "<c>" }}{% endcall %}|{% include "x.txt" with context %}|{% include "x.txt" without context %}`, "x.txt": `{{ "<i>" }}{% autoescape true %}{{ "<a>" }}{% endautoescape %}`}, vars: `{}`, trim: false, aeExt: true, want: `[&lt;h&gt;]&lt;m&gt;|&lt;m&gt;<`, err: ``},
		{name: "AutoescapePerTemplate3", src: ``, get: "caller.html", tpls: map[string]string{"page.html": `[{% include "part.txt" %}]({% import "m.txt" as mm %}{{ mm.mac() }})({{ mm }}){% from "m.txt" import mac %}{{ mac() }}|{% include "part.html" %}`, "part.txt": `{{ "<x>" }}`, "part.html": `{{ "<h>" }}`, "m.txt": `{% macro mac() %}{{ "<m>" }}{% endmacro %}T<{{ "<b>" }}>`, "page.txt": `[{% include "part.html" %}]{% import "m.html" as mh %}{{ mh.mac() }}|{% from "m.html" import mac %}{{ mac() ~ "<" }}`, "m.html": `{% macro mac() %}{{ "<m>" }}{% endmacro %}`, "caller.html": `{% macro w() %}<{{ caller() }}>{% endmacro %}{% call w() %}{{ "<c>" }}{% endcall %}|{% include "x.txt" with context %}|{% include "x.txt" without context %}`, "x.txt": `{{ "<i>" }}{% autoescape true %}{{ "<a>" }}{% endautoescape %}`}, vars: `{}`, trim: false, aeExt: true, want: `<&lt;c&gt;>|<i>&lt;a&gt;|<i>&lt;a&gt;`, err: ``},
		{name: "AutoescapeFromString", src: `{% include "part.txt" %}|{% include "part.html" %}|{{ "<s>" }}`, get: "", tpls: map[string]string{"page.html": `[{% include "part.txt" %}]({% import "m.txt" as mm %}{{ mm.mac() }})({{ mm }}){% from "m.txt" import mac %}{{ mac() }}|{% include "part.html" %}`, "part.txt": `{{ "<x>" }}`, "part.html": `{{ "<h>" }}`, "m.txt": `{% macro mac() %}{{ "<m>" }}{% endmacro %}T<{{ "<b>" }}>`, "page.txt": `[{% include "part.html" %}]{% import "m.html" as mh %}{{ mh.mac() }}|{% from "m.html" import mac %}{{ mac() ~ "<" }}`, "m.html": `{% macro mac() %}{{ "<m>" }}{% endmacro %}`, "caller.html": `{% macro w() %}<{{ caller() }}>{% endmacro %}{% call w() %}{{ "<c>" }}{% endcall %}|{% include "x.txt" with context %}|{% include "x.txt" without context %}`, "x.txt": `{{ "<i>" }}{% autoescape true %}{{ "<a>" }}{% endautoescape %}`}, vars: `{}`, trim: false, aeExt: true, want: `<x>|&lt;h&gt;|&lt;s&gt;`, err: ``},
	}
	// select_autoescape(['html']) for the cases that ask for it.
	autoescapeFor := func(ext bool) AutoescapePolicy {
		if ext {
			return AutoescapeByExtension{Enabled: []string{"html"}, DefaultForString: true}
		}
		return AutoescapeNever{}
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := mustEnv(t, WithAutoescape(autoescapeFor(c.aeExt)), WithLoader(loader.Dict(c.tpls)), WithLexerOptions(lexer.Options{TrimBlocks: c.trim}))
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
