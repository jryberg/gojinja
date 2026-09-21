package environment

import (
	"context"
	"strings"
	"testing"
)

// Expected values come from rendering the same templates with Python
// Jinja2 3.1.6.
func TestTupleLiterals(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	cases := []struct{ src, want string }{
		{`{{ (1,) }}|{{ () }}|{{ (1, 'a', none) }}|{% set t = 1, 2 %}{{ t }}`, "(1,)|()|(1, 'a', None)|(1, 2)"},
		{`{{ ('a','b')|length }}|{{ ('a','b')|first }}|{{ ('a','b')|last }}|{{ ('a','b')|join('-') }}|{{ ('x',)|list }}|{{ (3,1,2)|sort }}|{{ (3,1,2)|min }}|{{ (1,2)|reverse|list }}`,
			"2|a|b|a-b|['x']|[1, 2, 3]|1|[2, 1]"},
		{`{{ 'a' in ('a',) }}|{{ (1,)+(2,) }}|{{ (1,)*2 }}|{{ [1]*2 }}|{{ () is iterable }}|{{ not () }}|{{ (1,2).count(1) }}|{{ (1,2).index(2) }}`,
			"True|(1, 2)|(1, 1)|[1, 1]|True|True|1|1"},
		{`{% for a, b in [(1,2),(3,4)] %}{{a}}{{b}}{% endfor %}|{{ (1,2,3)[1:] }}|{{ '-'.join(('a','b')) }}|{{ dict([('a',1)]) }}|{{ 'ab'.startswith(('x','a')) }}`,
			"1234|(2, 3)|a-b|{'a': 1}|True"},
		{`{% macro m() %}{{ varargs }}{% endmacro %}{{ m(1, 2) }}|{{ not {} }}|{{ [[1], ('t',)] }}`,
			"(1, 2)|True|[[1], ('t',)]"},
	}
	for _, c := range cases {
		if got := render(t, e, c.src, nil); got != c.want {
			t.Errorf("%s = %q, want %q", c.src, got, c.want)
		}
	}
}

func TestListPlusTupleRaises(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	tpl, err := e.FromString(`{{ [1] + (2,) }}`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tpl.RenderContext(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), `can only concatenate list (not "tuple") to list`) {
		t.Fatalf("want concatenation TypeError, got %v", err)
	}
}
