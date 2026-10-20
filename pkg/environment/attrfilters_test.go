package environment

import (
	"context"
	"strings"
	"testing"

	"github.com/jryberg/gojinja/pkg/lexer"
	"github.com/jryberg/gojinja/pkg/loader"
	"github.com/jryberg/gojinja/pkg/varsutil"
)

// TestAttributeFilters holds sort, groupby, the filters taking an attribute, wordcount and tojson checked against Jinja2. Expected values come from the Jinja2 pinned in
// tools/parity/requirements.txt; err is the expected error's message, or
// "*" when only an error is expected.
func TestAttributeFilters(t *testing.T) {
	cases := []struct {
		name, src, get string
		tpls           map[string]string
		vars           string
		trim           bool
		want, err      string
	}{
		{name: "AttributeFilter1", src: `{{ [{'a':2,'b':1},{'a':1,'b':2},{'a':1,'b':1}]|sort(attribute='a,b') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[{'a': 1, 'b': 1}, {'a': 1, 'b': 2}, {'a': 2, 'b': 1}]`, err: ``},
		{name: "AttributeFilter2", src: `{{ [{'a':2,'b':1},{'a':1,'b':2},{'a':1,'b':1}]|sort(attribute='a,b', reverse=true) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[{'a': 2, 'b': 1}, {'a': 1, 'b': 2}, {'a': 1, 'b': 1}]`, err: ``},
		{name: "AttributeFilter3", src: `{{ [{'a':'B','b':1},{'a':'a','b':2}]|sort(attribute='a') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[{'a': 'a', 'b': 2}, {'a': 'B', 'b': 1}]`, err: ``},
		{name: "AttributeFilter4", src: `{{ [{'a':'B','b':1},{'a':'a','b':2}]|sort(attribute='a', case_sensitive=true) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[{'a': 'B', 'b': 1}, {'a': 'a', 'b': 2}]`, err: ``},
		{name: "AttributeFilter5", src: `{{ [[3,'b'],[1,'a'],[2,'c']]|sort(attribute=0) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[[1, 'a'], [2, 'c'], [3, 'b']]`, err: ``},
		{name: "AttributeFilter6", src: `{{ [[3,'b'],[1,'a'],[2,'c']]|sort(attribute='1') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[[1, 'a'], [3, 'b'], [2, 'c']]`, err: ``},
		{name: "AttributeFilter7", src: `{{ [[3,'b'],[1,'a']]|sort(attribute='0,1') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[[1, 'a'], [3, 'b']]`, err: ``},
		{name: "AttributeFilter8", src: `{{ [{'a':{'b':2}},{'a':{'b':1}}]|sort(attribute='a.b') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[{'a': {'b': 1}}, {'a': {'b': 2}}]`, err: ``},
		{name: "AttributeFilter9", src: `{{ [{'a':1},{'b':1}]|sort(attribute='a') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `'dict object' has no attribute 'a'`},
		{name: "AttributeFilter10", src: `{{ ['b','A','c']|sort }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `['A', 'b', 'c']`, err: ``},
		{name: "AttributeFilter11", src: `{{ ['b','A','c']|sort(true) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `['c', 'b', 'A']`, err: ``},
		{name: "AttributeFilter12", src: `{{ ['b','A','c']|sort(false, true) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `['A', 'b', 'c']`, err: ``},
		{name: "AttributeFilter13", src: `{{ [3,1,2]|sort(reverse=1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[3, 2, 1]`, err: ``},
		{name: "AttributeFilter14", src: `{{ [{'a':1,'b':'x'},{'a':1,'b':'X'}]|sort(attribute='a, b') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[{'a': 1, 'b': 'x'}, {'a': 1, 'b': 'X'}]`, err: ``},
		{name: "AttributeFilter15", src: `{{ [1,'a']|sort }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `'<' not supported between instances of 'str' and 'int'`},
		{name: "AttributeFilter16", src: `{% for g in [{'a':1},{'a':2},{'a':1}]|groupby('a') %}{{ g.grouper }}:{{ g.list|length }};{% endfor %}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1:2;2:1;`, err: ``},
		{name: "AttributeFilter17", src: `{% for g in [{'a':1},{'a':2},{'a':1}]|groupby('a') %}{{ g['grouper'] }}:{{ g['list'] }}|{{ g }}|{{ g[0] }}|{{ g|length }}|{{ g == (1, [{'a':1},{'a':1}]) }};{% endfor %}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1:[{'a': 1}, {'a': 1}]|(1, [{'a': 1}, {'a': 1}])|1|2|True;2:[{'a': 2}]|(2, [{'a': 2}])|2|2|False;`, err: ``},
		{name: "AttributeFilter18", src: `{% for k, v in [{'a':10},{'a':9},{'a':10}]|groupby('a') %}{{ k }}={{ v|length }};{% endfor %}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `9=1;10=2;`, err: ``},
		{name: "AttributeFilter19", src: `{% for k, v in [{'a':'B'},{'a':'b'},{'a':'A'}]|groupby('a') %}{{ k }}={{ v|length }};{% endfor %}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `A=1;B=2;`, err: ``},
		{name: "AttributeFilter20", src: `{% for k, v in [{'a':'B'},{'a':'b'},{'a':'A'}]|groupby('a', case_sensitive=true) %}{{ k }}={{ v|length }};{% endfor %}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `A=1;B=1;b=1;`, err: ``},
		{name: "AttributeFilter21", src: `{% for k, v in [{'a':1},{'b':2},{'a':1}]|groupby('a', default='z') %}{{ k }}={{ v|length }};{% endfor %}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `'<' not supported between instances of 'str' and 'int'`},
		{name: "AttributeFilter22", src: `{% for k, v in [{'a':1},{'b':2}]|groupby('a') %}{{ k }}={{ v|length }};{% endfor %}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `'dict object' has no attribute 'a'`},
		{name: "AttributeFilter23", src: `{% for k, v in [[1,'x'],[2,'y'],[1,'z']]|groupby(0) %}{{ k }}={{ v }};{% endfor %}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1=[[1, 'x'], [1, 'z']];2=[[2, 'y']];`, err: ``},
		{name: "AttributeFilter24", src: `{% for k, v in [{'a':{'b':1}},{'a':{'b':2}}]|groupby('a.b') %}{{ k }}={{ v }};{% endfor %}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1=[{'a': {'b': 1}}];2=[{'a': {'b': 2}}];`, err: ``},
		{name: "AttributeFilter25", src: `{{ ([{'a':1}]|groupby('a'))[0].grouper }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1`, err: ``},
		{name: "AttributeFilter26", src: `{{ ([{'a':1}]|groupby('a'))[0][1:] }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `([{'a': 1}],)`, err: ``},
		{name: "AttributeFilter27", src: `{{ (([{'a':1}]|groupby('a'))[0][0:2]).grouper }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: ``},
		{name: "AttributeFilter28", src: `{{ ([{'a':1}]|groupby('a'))|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[(1, [{'a': 1}])]`, err: ``},
		{name: "AttributeFilter29", src: `{% set g = ([{'a':1}]|groupby('a'))[0] %}{{ g.list.append(5) }}{{ g }}|{{ g.count(1) }}|{{ g.index(1) }}|{{ g.bogus }}|{{ (1, 2).grouper }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `None(1, [{'a': 1}, 5])|1|0||`, err: ``},
		{name: "AttributeFilter30", src: `{% for k, v in [{'a':1.0},{'a':1},{'a':true}]|groupby('a') %}{{ k }}={{ v|length }};{% endfor %}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1.0=3;`, err: ``},
		{name: "AttributeFilter31", src: `{{ [1,2]|groupby('real') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[(1, [1]), (2, [2])]`, err: ``},
		{name: "AttributeFilter32", src: `{{ ['b','A','a']|min }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `A`, err: ``},
		{name: "AttributeFilter33", src: `{{ ['b','A','a']|max }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `b`, err: ``},
		{name: "AttributeFilter34", src: `{{ ['b','A','a']|min(true) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `A`, err: ``},
		{name: "AttributeFilter35", src: `{{ [{'a':2},{'a':1}]|min(attribute='a') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `{'a': 1}`, err: ``},
		{name: "AttributeFilter36", src: `{{ [{'a':2},{'b':1}]|max(attribute='a') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `'dict object' has no attribute 'a'`},
		{name: "AttributeFilter37", src: `{{ ['a','A','b']|unique|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `['a', 'b']`, err: ``},
		{name: "AttributeFilter38", src: `{{ ['a','A','b']|unique(true)|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `['a', 'A', 'b']`, err: ``},
		{name: "AttributeFilter39", src: `{{ [{'a':1},{'a':1},{'a':2}]|unique(attribute='a')|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[{'a': 1}, {'a': 2}]`, err: ``},
		{name: "AttributeFilter40", src: `{{ [{'a':1},{'b':2}]|map(attribute='a')|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[1, Undefined]`, err: ``},
		{name: "AttributeFilter41", src: `{{ [{'a':1},{'b':2}]|map(attribute='a', default='d')|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[1, 'd']`, err: ``},
		{name: "AttributeFilter42", src: `{{ [{'a':1}]|map(attribute='a', x=1)|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `Unexpected keyword argument 'x'`},
		{name: "AttributeFilter43", src: `{{ [[1,2]]|map(attribute=1)|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[2]`, err: ``},
		{name: "AttributeFilter44", src: `{{ ['a-b']|map('replace', '-', '+')|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `['a+b']`, err: ``},
		{name: "AttributeFilter45", src: `{{ ['a']|map('default', 'x', boolean=true)|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `['a']`, err: ``},
		{name: "AttributeFilter46", src: `{{ ['ab']|map('truncate', 1, end='')|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `['ab']`, err: ``},
		{name: "AttributeFilter47", src: `{{ [{'a':1},{'a':2}]|sum(attribute='a') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `3`, err: ``},
		{name: "AttributeFilter48", src: `{{ [[1],[2]]|sum(attribute=0) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `3`, err: ``},
		{name: "AttributeFilter49", src: `{{ {'b':1,'A':2}|dictsort }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[('A', 2), ('b', 1)]`, err: ``},
		{name: "AttributeFilter50", src: `{{ {'b':1,'A':2}|dictsort(true) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[('A', 2), ('b', 1)]`, err: ``},
		{name: "AttributeFilter51", src: `{{ {'b':2,'a':1}|dictsort(by='value', reverse=true) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[('b', 2), ('a', 1)]`, err: ``},
		{name: "AttributeFilter52", src: `{{ [[1,0],[0,1]]|selectattr(0)|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[[1, 0]]`, err: ``},
		{name: "AttributeFilter53", src: `{{ [[1,0],[0,1]]|rejectattr(1)|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[[1, 0]]`, err: ``},
		{name: "AttributeFilter54", src: `{{ [[1,0],[0,1]]|selectattr('1')|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[[0, 1]]`, err: ``},
		{name: "AttributeFilter55", src: `{{ [{'a':{'b':1}},{'a':{'b':0}}]|selectattr('a.b')|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[{'a': {'b': 1}}]`, err: ``},
		{name: "AttributeFilter56", src: `{{ [{'a':1},{}]|selectattr('a', 'defined')|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[{'a': 1}]`, err: ``},
		{name: "AttributeFilter57", src: `{{ [{'a':1},{'a':3}]|selectattr('a', 'gt', 2)|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[{'a': 3}]`, err: ``},
		{name: "AttributeFilter58", src: `{{ [{'a':'x'}]|selectattr('a', 'equalto', 'x')|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[{'a': 'x'}]`, err: ``},
		{name: "AttributeFilter59", src: `{{ [{'a':'x'},{'a':'y'}]|join(',', attribute='a') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `x,y`, err: ``},
		{name: "AttributeFilter60", src: `{{ [[1],[2]]|join(',', attribute=0) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1,2`, err: ``},
		{name: "AttributeFilter61", src: `{{ [{'a':{'b':'x'}}]|join(attribute='a.b') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `x`, err: ``},
		{name: "AttributeFilter62", src: `{{ [{'a':1},{}]|join('-', attribute='a') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1-`, err: ``},
		{name: "AttributeFilter63", src: `{{ [{'items': 5}]|map(attribute='items')|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[5]`, err: ``},
		{name: "AttributeFilter64", src: `{{ [{'a':1}]|sort(attribute='items')|length }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1`, err: ``},
		{name: "AttributeFilter65", src: `{{ [{'keys':2},{'keys':1}]|sort(attribute='keys') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[{'keys': 1}, {'keys': 2}]`, err: ``},
		{name: "AttributeFilter66", src: `{{ [{'a': none}, {'a': 1}]|map(attribute='a', default=5)|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[None, 1]`, err: ``},
		{name: "AttributeFilter67", src: `{{ [{'a': {}}, {'a': {'b': 1}}]|map(attribute='a.b', default=7)|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[7, 1]`, err: ``},
		{name: "AttributeFilter68", src: `{{ [1,2,3]|select('odd')|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[1, 3]`, err: ``},
		{name: "AttributeFilter69", src: `{{ [1,2,3]|reject('odd')|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[2]`, err: ``},
		{name: "AttributeFilter70", src: `{{ [1,5,3]|select('gt', 2)|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[5, 3]`, err: ``},
		{name: "AttributeFilter71", src: `{{ [none, 0, 1]|select|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[1]`, err: ``},
		{name: "AttributeFilter72", src: `{{ [{'a':1}]|selectattr|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `Missing parameter for attribute name`},
		{name: "AttributeFilter73", src: `{{ none|selectattr|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[]`, err: ``},
		{name: "AttributeFilter74", src: `{{ ['a','B']|select('lower')|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `['a']`, err: ``},
		{name: "AttributeFilter75", src: `{{ [[1,0],[0,1]]|selectattr(0, 'eq', 1)|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[[1, 0]]`, err: ``},
		{name: "AttributeFilter76", src: `{{ [{'a':[1]}]|selectattr('a.0')|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[{'a': [1]}]`, err: ``},
		{name: "AttributeFilter77", src: `{{ 'a b  c\nd'|wordcount }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `4`, err: ``},
		{name: "AttributeFilter78", src: `{{ ''|wordcount }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `0`, err: ``},
		{name: "AttributeFilter79", src: `{{ 'héllo wörld'|wordcount }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `2`, err: ``},
		{name: "AttributeFilter80", src: `{{ 'naïve café'|wordcount }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `2`, err: ``},
		{name: "AttributeFilter81", src: `{{ '日本語 テキスト'|wordcount }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `2`, err: ``},
		{name: "AttributeFilter82", src: `{{ 'a_b c-d'|wordcount }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `3`, err: ``},
		{name: "AttributeFilter83", src: `{{ '٣٤ ² ½ Ⅻ'|wordcount }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `4`, err: ``},
		{name: "AttributeFilter84", src: `{{ 'été'|wordcount }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `2`, err: ``},
		{name: "AttributeFilter85", src: "{{ 'x\u200dy'|wordcount }}", get: "", tpls: nil, vars: `{}`, trim: false, want: `2`, err: ``},
		{name: "AttributeFilter86", src: `{{ 5|wordcount }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1`, err: ``},
		{name: "AttributeFilter87", src: `{{ 'ǅungla ʰa'|wordcount }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `2`, err: ``},
		{name: "AttributeFilter88", src: `{{ {'a':1}|tojson }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `{"a": 1}`, err: ``},
		{name: "AttributeFilter89", src: `{{ [1,'<a>',none,true,1.5]|tojson }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[1, "\u003ca\u003e", null, true, 1.5]`, err: ``},
		{name: "AttributeFilter90", src: `{{ 'é'|tojson }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `"\u00e9"`, err: ``},
		{name: "AttributeFilter91", src: `{{ {'b':1,'a':2}|tojson }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `{"a": 2, "b": 1}`, err: ``},
		{name: "AttributeFilter92", src: `{{ {'a':[1,{'b':2}]}|tojson(indent=2) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `{
  "a": [
    1,
    {
      "b": 2
    }
  ]
}`, err: ``},
		{name: "AttributeFilter93", src: `{{ '日本\U0001F600'|tojson }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `"\u65e5\u672c\ud83d\ude00"`, err: ``},
		{name: "AttributeFilter94", src: `{{ {'ключ': 'знач'}|tojson }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `{"\u043a\u043b\u044e\u0447": "\u0437\u043d\u0430\u0447"}`, err: ``},
		{name: "AttributeFilter95", src: `{{ {'ключ': 'знач'}|tojson(2) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `{
  "\u043a\u043b\u044e\u0447": "\u0437\u043d\u0430\u0447"
}`, err: ``},
		{name: "AttributeFilter96", src: `{{ 'a\x7fb\x00c\x1f\n\t\b\f\r"\\'|tojson }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `"a\u007fb\u0000c\u001f\n\t\b\f\r\"\\"`, err: ``},
		{name: "AttributeFilter97", src: `{{ '  '|tojson }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `"\u2028\u2029"`, err: ``},
		{name: "AttributeFilter98", src: `{{ "it's &"|tojson }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `"it\u0027s \u0026"`, err: ``},
		{name: "AttributeFilter99", src: `{{ 'ÿ'|tojson }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `"\u00ff"`, err: ``},
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
