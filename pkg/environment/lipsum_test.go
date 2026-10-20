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

// TestLipsum holds the lipsum global's arguments, errors and output shape, checked against Jinja2 (the text itself is random). Expected values come from the Jinja2 pinned in
// tools/parity/requirements.txt; err is the expected error's message, or
// "*" when only an error is expected. Object addresses in reprs are
// masked.
func TestLipsum(t *testing.T) {
	cases := []struct {
		name, src, get string
		tpls           map[string]string
		vars           string
		trim           bool
		want, err      string
	}{
		{name: "Lipsum1", src: `{{ lipsum(1, min=5, max=5) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `empty range in randrange(5, 5)`},
		{name: "Lipsum2", src: `{{ lipsum(1, min=9, max=5) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `empty range in randrange(9, 5)`},
		{name: "Lipsum3", src: `{{ lipsum("a") }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `'str' object cannot be interpreted as an integer`},
		{name: "Lipsum4", src: `{{ lipsum(1.5) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `'float' object cannot be interpreted as an integer`},
		{name: "Lipsum5", src: `{{ lipsum(1, min=1.5) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `'float' object cannot be interpreted as an integer`},
		{name: "Lipsum6", src: `{{ lipsum(1, min="a") }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `'str' object cannot be interpreted as an integer`},
		{name: "Lipsum7", src: `{{ lipsum(1, 2, 3, 4, 5) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `generate_lorem_ipsum() takes from 0 to 4 positional arguments but 5 were given`},
		{name: "Lipsum8", src: `{{ lipsum(x=1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `generate_lorem_ipsum() got an unexpected keyword argument 'x'`},
		{name: "Lipsum9", src: `{{ lipsum(1, n=2) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `generate_lorem_ipsum() got multiple values for argument 'n'`},
		{name: "Lipsum10", src: `{{ lipsum(none) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `'NoneType' object cannot be interpreted as an integer`},
		{name: "Lipsum11", src: `{{ lipsum(0) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: ``},
		{name: "Lipsum12", src: `{{ lipsum(0) is escaped }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `True`, err: ``},
		{name: "Lipsum13", src: `{{ lipsum(0, false) is escaped }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `False`, err: ``},
		{name: "Lipsum14", src: `{{ lipsum(-1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: ``},
		{name: "Lipsum15", src: `{{ lipsum(0, min='a') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: ``},
		{name: "Lipsum16", src: `{{ lipsum(1, min=-5, max=-3) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<p>.</p>`, err: ``},
		{name: "Lipsum17", src: `{{ lipsum(1, html=false, min=1, max=2)|wordcount }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1`, err: ``},
		{name: "Lipsum18", src: `{{ lipsum(2, html=false).count("\n\n") }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1`, err: ``},
		{name: "Lipsum19", src: `{{ lipsum(2).count("<p>") }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `2`, err: ``},
		{name: "Lipsum20", src: `{{ lipsum(2).count("\n") }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1`, err: ``},
		{name: "Lipsum21", src: `{{ lipsum(1, false)[-1] }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `.`, err: ``},
		{name: "Lipsum22", src: `{{ lipsum(1, false)[0] is upper }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `True`, err: ``},
		{name: "Lipsum23", src: `{{ lipsum() is escaped }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `True`, err: ``},
		{name: "Lipsum24", src: `{{ lipsum(3, html=false) is string }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `True`, err: ``},
		{name: "Lipsum25", src: `{{ lipsum(1, 0, 3, 4)|wordcount }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `3`, err: ``},
		{name: "Lipsum26", src: `{{ lipsum|string }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<function generate_lorem_ipsum at 0x…>`, err: ``},
		{name: "Lipsum27", src: `{% set ns = namespace(ok=true) %}{% set words = 'a ac accumsan ad adipiscing aenean aliquam aliquet amet ante aptent arcu at auctor augue bibendum blandit class commodo condimentum congue consectetur consequat conubia convallis cras cubilia cum curabitur curae cursus dapibus diam dictum dictumst dignissim dis dolor donec dui duis egestas eget eleifend elementum elit enim erat eros est et etiam eu euismod facilisi facilisis fames faucibus felis fermentum feugiat fringilla fusce gravida habitant habitasse hac hendrerit hymenaeos iaculis id imperdiet in inceptos integer interdum ipsum justo lacinia lacus laoreet lectus leo libero ligula litora lobortis lorem luctus maecenas magna magnis malesuada massa mattis mauris metus mi molestie mollis montes morbi mus nam nascetur natoque nec neque netus nibh nisi nisl non nostra nulla nullam nunc odio orci ornare parturient pede pellentesque penatibus per pharetra phasellus placerat platea porta porttitor posuere potenti praesent pretium primis proin pulvinar purus quam quis quisque rhoncus ridiculus risus rutrum sagittis sapien scelerisque sed sem semper senectus sit sociis sociosqu sodales sollicitudin suscipit suspendisse taciti tellus tempor tempus tincidunt torquent tortor tristique turpis ullamcorper ultrices ultricies urna ut varius vehicula vel velit venenatis vestibulum vitae vivamus viverra volutpat vulputate'.split() %}{% for w in lipsum(30, false, 99, 100).replace(',', '').replace('.', '').lower().split() %}{% if w not in words %}{% set ns.ok = false %}{% endif %}{% endfor %}{{ ns.ok }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `False`, err: ``},
	}
	addr := regexp.MustCompile(` at 0x[0-9a-f]+(>|&gt;)`)
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
