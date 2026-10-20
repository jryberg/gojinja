package environment

import (
	"context"
	"strings"
	"testing"

	"github.com/jryberg/gojinja/pkg/lexer"
	"github.com/jryberg/gojinja/pkg/loader"
	"github.com/jryberg/gojinja/pkg/varsutil"
)

// TestRange holds Python's range object (Jinja2's sandboxed safe_range) checked against Jinja2. Expected values come from the Jinja2 pinned in
// tools/parity/requirements.txt; err is the expected error's message, or
// "*" when only an error is expected.
func TestRange(t *testing.T) {
	cases := []struct {
		name, src, get string
		tpls           map[string]string
		vars           string
		trim           bool
		want, err      string
	}{
		{name: "Range1", src: `{{ range() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `range expected at least 1 argument, got 0`},
		{name: "Range2", src: `{{ range(1,2,3,4) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `range expected at most 3 arguments, got 4`},
		{name: "Range3", src: `{{ range(x=1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `safe_range() got an unexpected keyword argument 'x'`},
		{name: "Range4", src: `{{ range(1.5) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `'float' object cannot be interpreted as an integer`},
		{name: "Range5", src: `{{ range('a') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `'str' object cannot be interpreted as an integer`},
		{name: "Range6", src: `{{ range(1, 2, 0) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `range() arg 3 must not be zero`},
		{name: "Range7", src: `{{ range(200000) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `Range too big. The sandbox blocks ranges larger than MAX_RANGE (100000).`},
		{name: "Range8", src: `{{ range(true) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `range(0, 1)`, err: ``},
		{name: "Range9", src: `{{ range(5)[7] }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: ``},
		{name: "Range10", src: `{{ range(5)[-1] }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `4`, err: ``},
		{name: "Range11", src: `{{ range(5)['a'] }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: ``},
		{name: "Range12", src: `{{ range(10)[::-2] }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `range(9, -1, -2)`, err: ``},
		{name: "Range13", src: `{{ range(10)[2:100:3] }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `range(2, 10, 3)`, err: ``},
		{name: "Range14", src: `{{ range(5).index(9) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `9 is not in range`},
		{name: "Range15", src: `{{ range(5).index(2.0) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `2`, err: ``},
		{name: "Range16", src: `{{ range(5).count(1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1`, err: ``},
		{name: "Range17", src: `{{ range(5).index() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `range.index() takes exactly one argument (0 given)`},
		{name: "Range18", src: `{{ range(3) + [1] }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `unsupported operand type(s) for +: 'range' and 'list'`},
		{name: "Range19", src: `{{ range(3) * 2 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `unsupported operand type(s) for *: 'range' and 'int'`},
		{name: "Range20", src: `{{ range(3) < range(4) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `'<' not supported between instances of 'range' and 'range'`},
		{name: "Range21", src: `{{ range(3)|tojson }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `Object of type range is not JSON serializable`},
		{name: "Range22", src: `{{ range(0) == range(2, 2) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `True`, err: ``},
		{name: "Range23", src: `{{ range(3) == [0, 1, 2] }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `False`, err: ``},
		{name: "Range24", src: `{{ range(0, 3) == range(3) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `True`, err: ``},
		{name: "Range25", src: `{{ 3.0 in range(5) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `True`, err: ``},
		{name: "Range26", src: `{{ true in range(2) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `True`, err: ``},
		{name: "Range27", src: `{{ 'a' in range(2) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `False`, err: ``},
		{name: "Range28", src: `{{ range(3)|reverse|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[2, 1, 0]`, err: ``},
		{name: "Range29", src: `{{ range(3)|last }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `2`, err: ``},
		{name: "Range30", src: `{{ range(5)|random in range(5) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `True`, err: ``},
		{name: "Range31", src: `{{ range(3)|pprint }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `range(0, 3)`, err: ``},
		{name: "Range32", src: `{{ {range(3): 1} }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `{range(0, 3): 1}`, err: ``},
		{name: "Range33", src: `{{ range(3)|sum }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `3`, err: ``},
		{name: "Range34", src: `{{ range(3)|batch(2)|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[[0, 1], [2]]`, err: ``},
		{name: "Range35", src: `{{ (range(3))[1:2]|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[1]`, err: ``},
		{name: "Range36", src: `{{ range(5)|length }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `5`, err: ``},
		{name: "Range37", src: `{{ range(3) is sequence }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `True`, err: ``},
		{name: "Range38", src: `{{ range(3)|unique|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[0, 1, 2]`, err: ``},
		{name: "Range39", src: `{{ range(3).start + 1 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1`, err: ``},
		{name: "Range40", src: `{{ range(3)|join('-') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `0-1-2`, err: ``},
		{name: "Range41", src: `{{ range(0)|first }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: ``},
		{name: "Range42", src: `{{ range(1, 4)|list|sum }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `6`, err: ``},
		{name: "Range43", src: `{{ range(-5, 5, 3)|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[-5, -2, 1, 4]`, err: ``},
		{name: "Range44", src: `{{ range(5, -5, -3) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `range(5, -5, -3)`, err: ``},
		{name: "Range45", src: `{{ range(5)[1:-1] }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `range(1, 4)`, err: ``},
		{name: "Range46", src: `{{ range(3)[3:1] }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `range(3, 1)`, err: ``},
		{name: "Range47", src: `{{ '%s' % (range(3),) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `range(0, 3)`, err: ``},
		{name: "Range48", src: `{{ range(5) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `range(0, 5)`, err: ``},
		{name: "Range49", src: `{{ range(1, 10, 2) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `range(1, 10, 2)`, err: ``},
		{name: "Range50", src: `{{ range(10)[2:5] }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `range(2, 5)`, err: ``},
		{name: "Range51", src: `{{ range(5).start }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `0`, err: ``},
		{name: "Range52", src: `{{ range(5).stop }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `5`, err: ``},
		{name: "Range53", src: `{{ range(5).step }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1`, err: ``},
		{name: "Range54", src: `{{ range(0) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `range(0, 0)`, err: ``},
		{name: "Range55", src: `{{ range(-3)|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[]`, err: ``},
		{name: "Range56", src: `{{ range(5, 0, -1)|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[5, 4, 3, 2, 1]`, err: ``},
		{name: "Range57", src: `{{ range(3) is iterable }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `True`, err: ``},
		{name: "Range58", src: `{{ range(3)|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[0, 1, 2]`, err: ``},
		{name: "Range59", src: `{{ range(3)|string }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `range(0, 3)`, err: ``},
		{name: "Range60", src: `{{ range(3) ~ '' }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `range(0, 3)`, err: ``},
		{name: "Range61", src: `{{ range(4)|slice(2)|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[[0, 1], [2, 3]]`, err: ``},
		{name: "Range62", src: `{{ range(3)|map('string')|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `['0', '1', '2']`, err: ``},
		{name: "Range63", src: `{{ range(3)|first }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `0`, err: ``},
		{name: "Range64", src: `{{ range(10)|select('odd')|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[1, 3, 5, 7, 9]`, err: ``},
		{name: "Range65", src: `{{ range(3)|sort(reverse=true) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[2, 1, 0]`, err: ``},
		{name: "Range66", src: `{{ range(3)[0:0] }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `range(0, 0)`, err: ``},
		{name: "Range67", src: `{{ range(5)[::0] }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `slice step cannot be zero`},
		{name: "Range68", src: `{{ range(5) is mapping }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `False`, err: ``},
		{name: "Range69", src: `{{ range(2)|min }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `0`, err: ``},
		{name: "Range70", src: `{{ range(3)|max }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `2`, err: ``},
		{name: "Range71", src: `{{ range(3).index(1, 2) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `range.index() takes exactly one argument (2 given)`},
		{name: "Range72", src: `{{ range(3).count(x=1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `range.count() takes no keyword arguments`},
		{name: "Range73", src: `{{ range(5)[1.0] }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: ``},
		{name: "Range74", src: `{{ range(1, 2) == range(1, 3, 5) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `True`, err: ``},
		{name: "Range75", src: `{{ not range(0) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `True`, err: ``},
		{name: "Range76", src: `{{ range(3)|dictsort }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `'range' object has no attribute 'items'`},
		{name: "Range77", src: `{{ {}.fromkeys(range(2)) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `{0: None, 1: None}`, err: ``},
		{name: "Range78", src: `{{ dict.fromkeys(range(2), 0) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `{0: 0, 1: 0}`, err: ``},
		{name: "Range79", src: `{{ [1, 2, 3][range(1)|length] }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `2`, err: ``},
		{name: "Range80", src: `{{ range(3)|tojson }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `Object of type range is not JSON serializable`},
		{name: "Range81", src: `{% for i in range(3) %}{{ loop.index }}{{ loop.length }}{% endfor %}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `132333`, err: ``},
		{name: "Range82", src: `{% set r = range(3) %}{% for i in r %}{% for j in r %}{{ i }}{{ j }}{% endfor %}{% endfor %}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `000102101112202122`, err: ``},
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
