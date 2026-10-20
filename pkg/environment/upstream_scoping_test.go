package environment

import (
	"context"
	"strings"
	"testing"

	"github.com/jryberg/gojinja/pkg/lexer"
	"github.com/jryberg/gojinja/pkg/loader"
	"github.com/jryberg/gojinja/pkg/varsutil"
)

// TestUpstreamScopingAndInheritance holds functional copies of Jinja2's scoping and inheritance tests (test_core_tags.py, test_regression.py, test_inheritance.py). Expected values come from the Jinja2 pinned in
// tools/parity/requirements.txt; err is the expected error's message, or
// "*" when only an error is expected.
func TestUpstreamScopingAndInheritance(t *testing.T) {
	cases := []struct {
		name, src, get string
		tpls           map[string]string
		vars           string
		trim           bool
		want, err      string
	}{
		{name: "ForElseScopingItem", src: `{% for item in [] %}{% else %}{{ item }}{% endfor %}`, get: "", tpls: nil, vars: `{"item": 42}`, trim: false, want: `42`, err: ``},
		{name: "ForScope", src: `{% for item in seq %}{% endfor %}{{ item }}`, get: "", tpls: nil, vars: `{"seq": [0, 1, 2, 3, 4, 5, 6, 7, 8, 9]}`, trim: false, want: ``, err: ``},
		{name: "ForNoniter", src: `{% for item in none %}...{% endfor %}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `'NoneType' object is not iterable`},
		{name: "ForScopedSpecialVar", src: `{% for s in seq %}[{{ loop.first }}{% for c in s %}|{{ loop.first }}{% endfor %}]{% endfor %}`, get: "", tpls: nil, vars: `{"seq": ["ab", "cd"]}`, trim: false, want: `[True|True|False][False|True|False]`, err: ``},
		{name: "ForScopedLoopVar1", src: `{% for x in seq %}{{ loop.first }}{% for y in seq %}{% endfor %}{% endfor %}`, get: "", tpls: nil, vars: `{"seq": "ab"}`, trim: false, want: `TrueFalse`, err: ``},
		{name: "ForScopedLoopVar2", src: `{% for x in seq %}{% for y in seq %}{{ loop.first }}{% endfor %}{% endfor %}`, get: "", tpls: nil, vars: `{"seq": "ab"}`, trim: false, want: `TrueFalseTrueFalse`, err: ``},
		{name: "ForRecursiveEmptyLoopIter", src: `
        {%- for item in foo recursive -%}{%- endfor -%}
        `, get: "", tpls: nil, vars: `{"foo": []}`, trim: false, want: ``, err: ``},
		{name: "ForCallInLoop", src: `
        {%- macro do_something() -%}
            [{{ caller() }}]
        {%- endmacro %}

        {%- for i in [1, 2, 3] %}
            {%- call do_something() -%}
                {{ i }}
            {%- endcall %}
        {%- endfor -%}
        `, get: "", tpls: nil, vars: `{}`, trim: false, want: `[1][2][3]`, err: ``},
		{name: "ForScopingBug", src: `
        {%- for item in foo %}...{{ item }}...{% endfor %}
        {%- macro item(a) %}...{{ a }}...{% endmacro %}
        {{- item(2) -}}
        `, get: "", tpls: nil, vars: `{"foo": [1]}`, trim: false, want: `...1......2...`, err: ``},
		{name: "ForIntendedScopingWithSet1", src: `{% for item in seq %}{{ x }}{% set x = item %}{{ x }}{% endfor %}`, get: "", tpls: nil, vars: `{"x": 0, "seq": [1, 2, 3]}`, trim: false, want: `010203`, err: ``},
		{name: "ForIntendedScopingWithSet2", src: `{% set x = 9 %}{% for item in seq %}{{ x }}{% set x = item %}{{ x }}{% endfor %}`, get: "", tpls: nil, vars: `{"x": 0, "seq": [1, 2, 3]}`, trim: false, want: `919293`, err: ``},
		{name: "IfNoScope1", src: `{% if a %}{% set foo = 1 %}{% endif %}{{ foo }}`, get: "", tpls: nil, vars: `{"a": true}`, trim: false, want: `1`, err: ``},
		{name: "IfNoScope2", src: `{% if true %}{% set foo = 1 %}{% endif %}{{ foo }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1`, err: ``},
		{name: "MacroScoping", src: `{% macro level1(data1) %}
{% macro level2(data2) %}{{ data1 }}|{{ data2 }}{% endmacro %}
{{ level2('bar') }}{% endmacro %}
{{ level1('foo') }}`, get: "", tpls: nil, vars: `{}`, trim: true, want: `foo|bar`, err: ``},
		{name: "With", src: `        {% with a=42, b=23 -%}
            {{ a }} = {{ b }}
        {% endwith -%}
            {{ a }} = {{ b }}        `, get: "", tpls: nil, vars: `{"a": 1, "b": 2}`, trim: false, want: `        42 = 23
        1 = 2        `, err: ``},
		{name: "WithArgumentScoping", src: `{%- with a=1, b=2, c=b, d=e, e=5 -%}
            {{ a }}|{{ b }}|{{ c }}|{{ d }}|{{ e }}
        {%- endwith -%}
        `, get: "", tpls: nil, vars: `{"b": 3, "e": 4}`, trim: false, want: `1|2|3|4|5`, err: ``},
		{name: "AssignedScoping1", src: `
        {%- for item in (1, 2, 3, 4) -%}
            [{{ item }}]
        {%- endfor %}
        {{- item -}}
        `, get: "", tpls: nil, vars: `{"item": 42}`, trim: false, want: `[1][2][3][4]42`, err: ``},
		{name: "AssignedScoping2", src: `
        {%- for item in (1, 2, 3, 4) -%}
            [{{ item }}]
        {%- endfor %}
        {%- set item = 42 %}
        {{- item -}}
        `, get: "", tpls: nil, vars: `{}`, trim: false, want: `[1][2][3][4]42`, err: ``},
		{name: "AssignedScoping3", src: `
        {%- set item = 42 %}
        {%- for item in (1, 2, 3, 4) -%}
            [{{ item }}]
        {%- endfor %}
        {{- item -}}
        `, get: "", tpls: nil, vars: `{}`, trim: false, want: `[1][2][3][4]42`, err: ``},
		{name: "ClosureScoping1", src: `
        {%- set wrapper = "<FOO>" %}
        {%- for item in (1, 2, 3, 4) %}
            {%- macro wrapper() %}[{{ item }}]{% endmacro %}
            {{- wrapper() }}
        {%- endfor %}
        {{- wrapper -}}
        `, get: "", tpls: nil, vars: `{}`, trim: false, want: `[1][2][3][4]<FOO>`, err: ``},
		{name: "ClosureScoping2", src: `
        {%- for item in (1, 2, 3, 4) %}
            {%- macro wrapper() %}[{{ item }}]{% endmacro %}
            {{- wrapper() }}
        {%- endfor %}
        {%- set wrapper = "<FOO>" %}
        {{- wrapper -}}
        `, get: "", tpls: nil, vars: `{}`, trim: false, want: `[1][2][3][4]<FOO>`, err: ``},
		{name: "ClosureScoping3", src: `
        {%- for item in (1, 2, 3, 4) %}
            {%- macro wrapper() %}[{{ item }}]{% endmacro %}
            {{- wrapper() }}
        {%- endfor %}
        {{- wrapper -}}
        `, get: "", tpls: nil, vars: `{"wrapper": 23}`, trim: false, want: `[1][2][3][4]23`, err: ``},
		{name: "ExtendsOutputBugs1", src: `{% if expr %}{% extends "parent.html" %}{% endif %}[[{% block title %}title{% endblock %}]]{% for item in [1, 2, 3] %}({{ item }}){% endfor %}`, get: "", tpls: map[string]string{"parent.html": `(({% block title %}{% endblock %}))`}, vars: `{"expr": false}`, trim: false, want: `[[title]](1)(2)(3)`, err: ``},
		{name: "ExtendsOutputBugs2", src: `{% if expr %}{% extends "parent.html" %}{% endif %}[[{% block title %}title{% endblock %}]]{% for item in [1, 2, 3] %}({{ item }}){% endfor %}`, get: "", tpls: map[string]string{"parent.html": `(({% block title %}{% endblock %}))`}, vars: `{"expr": true}`, trim: false, want: `((title))`, err: ``},
		{name: "OldMacroLoopScopingBug", src: `{% for i in (1, 2) %}{{ i }}{% endfor %}{% macro i() %}3{% endmacro %}{{ i() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `123`, err: ``},
		{name: "PartialConditionalAssignments1", src: `{% if b %}{% set a = 42 %}{% endif %}{{ a }}`, get: "", tpls: nil, vars: `{"a": 23}`, trim: false, want: `23`, err: ``},
		{name: "PartialConditionalAssignments2", src: `{% if b %}{% set a = 42 %}{% endif %}{{ a }}`, get: "", tpls: nil, vars: `{"b": true}`, trim: false, want: `42`, err: ``},
		{name: "StackedLocalsScopingBugTwoframe", src: `
            {% set x = 1 %}
            {% for item in foo %}
                {% if item == 1 %}
                    {% set x = 2 %}
                {% endif %}
            {% endfor %}
            {{ x }}
        `, get: "", tpls: nil, vars: `{"foo": [1]}`, trim: false, want: `
            
            
                
                    
                
            
            1
        `, err: ``},
		{name: "BlockSetWithExtends", src: `{% extends "main" %}{% set x %}42{% endset %}`, get: "", tpls: map[string]string{"main": `{% block body %}[{{ x }}]{% endblock %}`}, vars: `{}`, trim: false, want: `[42]`, err: ``},
		{name: "NestedForElse", src: `{% for x in y %}{{ loop.index0 }}{% else %}{% for i in range(3) %}{{ i }}{% endfor %}{% endfor %}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `012`, err: ``},
		{name: "MacroVarBug", src: `
        {% set i = 1 %}
        {% macro test() %}
            {% for i in range(0, 10) %}{{ i }}{% endfor %}
        {% endmacro %}{{ test() }}
        `, get: "", tpls: nil, vars: `{}`, trim: false, want: `
        
        
            0123456789
        
        `, err: ``},
		{name: "MacroVarBugAdvanced", src: `
        {% macro outer() %}
            {% set i = 1 %}
            {% macro test() %}
                {% for i in range(0, 10) %}{{ i }}{% endfor %}
            {% endmacro %}{{ test() }}
        {% endmacro %}{{ outer() }}
        `, get: "", tpls: nil, vars: `{}`, trim: false, want: `
        
            
            
                0123456789
            
        
        `, err: ``},
		{name: "MacroScopingRegression", src: `
        {% set n=[1,2,3,4,5] %}
        {% for n in [[1,2,3], [3,4,5], [5,6,7]] %}

        {% macro x(l) %}
          {{ l.pop() }}
          {% if l %}{{ x(l) }}{% endif %}
        {% endmacro %}

        {{ x(n) }}

        {% endfor %}
        `, get: "", tpls: nil, vars: `{}`, trim: false, want: `
        
        

        

        
          3
          
          2
          
          1
          
        
        
        

        

        

        
          5
          
          4
          
          3
          
        
        
        

        

        

        
          7
          
          6
          
          5
          
        
        
        

        
        `, err: ``},
		{name: "ScopesAndBlocksA", src: ``, get: "a.html", tpls: map[string]string{"a.html": `
                {%- set foo = 'bar' -%}
                {% include 'x.html' -%}
            `, "b.html": `
                {%- set foo = 'bar' -%}
                {% block test %}{% include 'x.html' %}{% endblock -%}
                `, "c.html": `
                {%- set foo = 'bar' -%}
                {% block test %}{% set foo = foo
                    %}{% include 'x.html' %}{% endblock -%}
            `, "x.html": `{{ foo }}|{{ test }}`}, vars: `{"test": "x"}`, trim: false, want: `bar|x`, err: ``},
		{name: "ScopesAndBlocksB", src: ``, get: "b.html", tpls: map[string]string{"a.html": `
                {%- set foo = 'bar' -%}
                {% include 'x.html' -%}
            `, "b.html": `
                {%- set foo = 'bar' -%}
                {% block test %}{% include 'x.html' %}{% endblock -%}
                `, "c.html": `
                {%- set foo = 'bar' -%}
                {% block test %}{% set foo = foo
                    %}{% include 'x.html' %}{% endblock -%}
            `, "x.html": `{{ foo }}|{{ test }}`}, vars: `{"test": "x"}`, trim: false, want: `bar|x`, err: ``},
		{name: "ScopesAndBlocksC", src: ``, get: "c.html", tpls: map[string]string{"a.html": `
                {%- set foo = 'bar' -%}
                {% include 'x.html' -%}
            `, "b.html": `
                {%- set foo = 'bar' -%}
                {% block test %}{% include 'x.html' %}{% endblock -%}
                `, "c.html": `
                {%- set foo = 'bar' -%}
                {% block test %}{% set foo = foo
                    %}{% include 'x.html' %}{% endblock -%}
            `, "x.html": `{{ foo }}|{{ test }}`}, vars: `{"test": "x"}`, trim: false, want: `bar|x`, err: ``},
		{name: "ScopesAndInclude", src: ``, get: "child.html", tpls: map[string]string{"include.html": `{{ var }}`, "base.html": `{% include "include.html" %}`, "child.html": `{% extends "base.html" %}{% set var = 42 %}`}, vars: `{}`, trim: false, want: `42`, err: ``},
		{name: "CallerScoping", src: `
        {% macro detail(icon, value) -%}
          {% if value -%}
            <p><span class="fa fa-fw fa-{{ icon }}"></span>
                {%- if caller is undefined -%}
                    {{ value }}
                {%- else -%}
                    {{ caller(value, *varargs) }}
                {%- endif -%}</p>
          {%- endif %}
        {%- endmacro %}


        {% macro link_detail(icon, value, href) -%}
          {% call(value, href) detail(icon, value, href) -%}
            <a href="{{ href }}">{{ value }}</a>
          {%- endcall %}
        {%- endmacro %}
        {{- link_detail("circle", "Index", "/") }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `
        


        <p><span class="fa fa-fw fa-circle"></span><a href="/">Index</a></p>`, err: ``},
		{name: "VariableReuse1", src: `{% for x in x.y %}{{ x }}{% endfor %}`, get: "", tpls: nil, vars: `{"x": {"y": [0, 1, 2]}}`, trim: false, want: `012`, err: ``},
		{name: "VariableReuse2", src: `{% for x in x.y %}{{ loop.index0 }}|{{ x }}{% endfor %}`, get: "", tpls: nil, vars: `{"x": {"y": [0, 1, 2]}}`, trim: false, want: `0|01|12|2`, err: ``},
		{name: "VariableReuse3", src: `{% for x in x.y recursive %}{{ x }}{% endfor %}`, get: "", tpls: nil, vars: `{"x": {"y": [0, 1, 2]}}`, trim: false, want: `012`, err: ``},
		{name: "ScopedBlockRegression", src: `{% set x = 1 %}{% with x = 2 %}{% block y scoped %}{{ x }}{% endblock %}{% endwith %}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `2`, err: ``},
		{name: "RecursiveLoopFilter", src: `
        <?xml version="1.0" encoding="UTF-8"?>
        <urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
          {%- for page in [site.root] if page.url != this recursive %}
          <url><loc>{{ page.url }}</loc></url>
          {{- loop(page.children) }}
          {%- endfor %}
        </urlset>
        `, get: "", tpls: nil, vars: `{"this": "/foo", "site": {"root": {"url": "/", "children": [{"url": "/foo"}, {"url": "/bar"}]}}}`, trim: false, want: `
        <?xml version="1.0" encoding="UTF-8"?>
        <urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
          <url><loc>/</loc></url>
          <url><loc>/bar</loc></url>
        </urlset>
        `, err: ``},
		{name: "RecursiveLoopBug", src: `{%- for value in values recursive %}1{% else %}0{% endfor -%}`, get: "", tpls: nil, vars: `{"values": []}`, trim: false, want: `0`, err: ``},
		{name: "ScopedBlockLoopVars", src: `Start
{% for i in ["foo", "bar"] -%}
{% block body scoped -%}
{{ loop.index }}) {{ i }}{% if loop.last %} last{% endif -%}
{%- endblock %}
{% endfor -%}
End`, get: "", tpls: nil, vars: `{}`, trim: false, want: `Start
1) foo
2) bar last
End`, err: ``},
		{name: "NestedLoopScoping", src: `{% set output %}{% for x in [1,2,3] %}hello{% endfor %}{% endset %}{{ output }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `hellohellohello`, err: ``},
		{name: "LoadParameterWhenSetInAllIfBranches", src: `{% if True %}{{ a.b }}{% set a = 1 %}{% elif False %}{% set a = 2 %}{% else %}{% set a = 3 %}{% endif %}{{ a }}`, get: "", tpls: nil, vars: `{"a": {"b": 0}}`, trim: false, want: `01`, err: ``},
		{name: "InheritanceLayout", src: ``, get: "layout", tpls: map[string]string{"layout": `|{% block block1 %}block 1 from layout{% endblock %}
|{% block block2 %}block 2 from layout{% endblock %}
|{% block block3 %}
{% block block4 %}nested block 4 from layout{% endblock %}
{% endblock %}|`, "level1": `{% extends "layout" %}
{% block block1 %}block 1 from level1{% endblock %}`, "level2": `{% extends "level1" %}
{% block block2 %}{% block block5 %}nested block 5 from level2{%
endblock %}{% endblock %}`, "level3": `{% extends "level2" %}
{% block block5 %}block 5 from level3{% endblock %}
{% block block4 %}block 4 from level3{% endblock %}
`, "level4": `{% extends "level3" %}
{% block block3 %}block 3 from level4{% endblock %}
`, "working": `{% extends "layout" %}
{% block block1 %}
  {% if false %}
    {% block block2 %}
      this should work
    {% endblock %}
  {% endif %}
{% endblock %}
`, "doublee": `{% extends "layout" %}
{% extends "layout" %}
{% block block1 %}
  {% if false %}
    {% block block2 %}
      this should work
    {% endblock %}
  {% endif %}
{% endblock %}
`}, vars: `{}`, trim: true, want: `|block 1 from layout|block 2 from layout|nested block 4 from layout|`, err: ``},
		{name: "InheritanceLevel1", src: ``, get: "level1", tpls: map[string]string{"layout": `|{% block block1 %}block 1 from layout{% endblock %}
|{% block block2 %}block 2 from layout{% endblock %}
|{% block block3 %}
{% block block4 %}nested block 4 from layout{% endblock %}
{% endblock %}|`, "level1": `{% extends "layout" %}
{% block block1 %}block 1 from level1{% endblock %}`, "level2": `{% extends "level1" %}
{% block block2 %}{% block block5 %}nested block 5 from level2{%
endblock %}{% endblock %}`, "level3": `{% extends "level2" %}
{% block block5 %}block 5 from level3{% endblock %}
{% block block4 %}block 4 from level3{% endblock %}
`, "level4": `{% extends "level3" %}
{% block block3 %}block 3 from level4{% endblock %}
`, "working": `{% extends "layout" %}
{% block block1 %}
  {% if false %}
    {% block block2 %}
      this should work
    {% endblock %}
  {% endif %}
{% endblock %}
`, "doublee": `{% extends "layout" %}
{% extends "layout" %}
{% block block1 %}
  {% if false %}
    {% block block2 %}
      this should work
    {% endblock %}
  {% endif %}
{% endblock %}
`}, vars: `{}`, trim: true, want: `|block 1 from level1|block 2 from layout|nested block 4 from layout|`, err: ``},
		{name: "InheritanceLevel2", src: ``, get: "level2", tpls: map[string]string{"layout": `|{% block block1 %}block 1 from layout{% endblock %}
|{% block block2 %}block 2 from layout{% endblock %}
|{% block block3 %}
{% block block4 %}nested block 4 from layout{% endblock %}
{% endblock %}|`, "level1": `{% extends "layout" %}
{% block block1 %}block 1 from level1{% endblock %}`, "level2": `{% extends "level1" %}
{% block block2 %}{% block block5 %}nested block 5 from level2{%
endblock %}{% endblock %}`, "level3": `{% extends "level2" %}
{% block block5 %}block 5 from level3{% endblock %}
{% block block4 %}block 4 from level3{% endblock %}
`, "level4": `{% extends "level3" %}
{% block block3 %}block 3 from level4{% endblock %}
`, "working": `{% extends "layout" %}
{% block block1 %}
  {% if false %}
    {% block block2 %}
      this should work
    {% endblock %}
  {% endif %}
{% endblock %}
`, "doublee": `{% extends "layout" %}
{% extends "layout" %}
{% block block1 %}
  {% if false %}
    {% block block2 %}
      this should work
    {% endblock %}
  {% endif %}
{% endblock %}
`}, vars: `{}`, trim: true, want: `|block 1 from level1|nested block 5 from level2|nested block 4 from layout|`, err: ``},
		{name: "InheritanceLevel3", src: ``, get: "level3", tpls: map[string]string{"layout": `|{% block block1 %}block 1 from layout{% endblock %}
|{% block block2 %}block 2 from layout{% endblock %}
|{% block block3 %}
{% block block4 %}nested block 4 from layout{% endblock %}
{% endblock %}|`, "level1": `{% extends "layout" %}
{% block block1 %}block 1 from level1{% endblock %}`, "level2": `{% extends "level1" %}
{% block block2 %}{% block block5 %}nested block 5 from level2{%
endblock %}{% endblock %}`, "level3": `{% extends "level2" %}
{% block block5 %}block 5 from level3{% endblock %}
{% block block4 %}block 4 from level3{% endblock %}
`, "level4": `{% extends "level3" %}
{% block block3 %}block 3 from level4{% endblock %}
`, "working": `{% extends "layout" %}
{% block block1 %}
  {% if false %}
    {% block block2 %}
      this should work
    {% endblock %}
  {% endif %}
{% endblock %}
`, "doublee": `{% extends "layout" %}
{% extends "layout" %}
{% block block1 %}
  {% if false %}
    {% block block2 %}
      this should work
    {% endblock %}
  {% endif %}
{% endblock %}
`}, vars: `{}`, trim: true, want: `|block 1 from level1|block 5 from level3|block 4 from level3|`, err: ``},
		{name: "InheritanceLevel4", src: ``, get: "level4", tpls: map[string]string{"layout": `|{% block block1 %}block 1 from layout{% endblock %}
|{% block block2 %}block 2 from layout{% endblock %}
|{% block block3 %}
{% block block4 %}nested block 4 from layout{% endblock %}
{% endblock %}|`, "level1": `{% extends "layout" %}
{% block block1 %}block 1 from level1{% endblock %}`, "level2": `{% extends "level1" %}
{% block block2 %}{% block block5 %}nested block 5 from level2{%
endblock %}{% endblock %}`, "level3": `{% extends "level2" %}
{% block block5 %}block 5 from level3{% endblock %}
{% block block4 %}block 4 from level3{% endblock %}
`, "level4": `{% extends "level3" %}
{% block block3 %}block 3 from level4{% endblock %}
`, "working": `{% extends "layout" %}
{% block block1 %}
  {% if false %}
    {% block block2 %}
      this should work
    {% endblock %}
  {% endif %}
{% endblock %}
`, "doublee": `{% extends "layout" %}
{% extends "layout" %}
{% block block1 %}
  {% if false %}
    {% block block2 %}
      this should work
    {% endblock %}
  {% endif %}
{% endblock %}
`}, vars: `{}`, trim: true, want: `|block 1 from level1|block 5 from level3|block 3 from level4|`, err: ``},
		{name: "InheritanceWorking", src: ``, get: "working", tpls: map[string]string{"layout": `|{% block block1 %}block 1 from layout{% endblock %}
|{% block block2 %}block 2 from layout{% endblock %}
|{% block block3 %}
{% block block4 %}nested block 4 from layout{% endblock %}
{% endblock %}|`, "level1": `{% extends "layout" %}
{% block block1 %}block 1 from level1{% endblock %}`, "level2": `{% extends "level1" %}
{% block block2 %}{% block block5 %}nested block 5 from level2{%
endblock %}{% endblock %}`, "level3": `{% extends "level2" %}
{% block block5 %}block 5 from level3{% endblock %}
{% block block4 %}block 4 from level3{% endblock %}
`, "level4": `{% extends "level3" %}
{% block block3 %}block 3 from level4{% endblock %}
`, "working": `{% extends "layout" %}
{% block block1 %}
  {% if false %}
    {% block block2 %}
      this should work
    {% endblock %}
  {% endif %}
{% endblock %}
`, "doublee": `{% extends "layout" %}
{% extends "layout" %}
{% block block1 %}
  {% if false %}
    {% block block2 %}
      this should work
    {% endblock %}
  {% endif %}
{% endblock %}
`}, vars: `{}`, trim: true, want: `|  |      this should work
    |nested block 4 from layout|`, err: ``},
		{name: "InheritanceDoubleExtends", src: ``, get: "doublee", tpls: map[string]string{"layout": `|{% block block1 %}block 1 from layout{% endblock %}
|{% block block2 %}block 2 from layout{% endblock %}
|{% block block3 %}
{% block block4 %}nested block 4 from layout{% endblock %}
{% endblock %}|`, "level1": `{% extends "layout" %}
{% block block1 %}block 1 from level1{% endblock %}`, "level2": `{% extends "level1" %}
{% block block2 %}{% block block5 %}nested block 5 from level2{%
endblock %}{% endblock %}`, "level3": `{% extends "level2" %}
{% block block5 %}block 5 from level3{% endblock %}
{% block block4 %}block 4 from level3{% endblock %}
`, "level4": `{% extends "level3" %}
{% block block3 %}block 3 from level4{% endblock %}
`, "working": `{% extends "layout" %}
{% block block1 %}
  {% if false %}
    {% block block2 %}
      this should work
    {% endblock %}
  {% endif %}
{% endblock %}
`, "doublee": `{% extends "layout" %}
{% extends "layout" %}
{% block block1 %}
  {% if false %}
    {% block block2 %}
      this should work
    {% endblock %}
  {% endif %}
{% endblock %}
`}, vars: `{}`, trim: true, want: ``, err: `extended multiple times`},
		{name: "InheritanceSuper", src: ``, get: "c", tpls: map[string]string{"a": `{% block intro %}INTRO{% endblock %}|BEFORE|{% block data %}INNER{% endblock %}|AFTER`, "b": `{% extends "a" %}{% block data %}({{ super() }}){% endblock %}`, "c": `{% extends "b" %}{% block intro %}--{{ super() }}--{% endblock %}
{% block data %}[{{ super() }}]{% endblock %}`}, vars: `{}`, trim: false, want: `--INTRO--|BEFORE|[(INNER)]|AFTER`, err: ``},
		{name: "InheritanceReuseBlocks", src: `{{ self.foo() }}|{% block foo %}42{% endblock %}|{{ self.foo() }}`, get: "", tpls: nil, vars: `{}`, trim: true, want: `42|42|42`, err: ``},
		{name: "InheritancePreserveBlocks", src: ``, get: "b", tpls: map[string]string{"a": `{% if false %}{% block x %}A{% endblock %}{% endif %}{{ self.x() }}`, "b": `{% extends "a" %}{% block x %}B{{ super() }}{% endblock %}`}, vars: `{}`, trim: false, want: `BA`, err: ``},
		{name: "InheritanceDynamic1", src: ``, get: "child", tpls: map[string]string{"default1": `DEFAULT1{% block x %}{% endblock %}`, "default2": `DEFAULT2{% block x %}{% endblock %}`, "child": `{% extends default %}{% block x %}CHILD{% endblock %}`}, vars: `{"default": "default1"}`, trim: false, want: `DEFAULT1CHILD`, err: ``},
		{name: "InheritanceDynamic2", src: ``, get: "child", tpls: map[string]string{"default1": `DEFAULT1{% block x %}{% endblock %}`, "default2": `DEFAULT2{% block x %}{% endblock %}`, "child": `{% extends default %}{% block x %}CHILD{% endblock %}`}, vars: `{"default": "default2"}`, trim: false, want: `DEFAULT2CHILD`, err: ``},
		{name: "InheritanceMulti1", src: ``, get: "child", tpls: map[string]string{"default1": `DEFAULT1{% block x %}{% endblock %}`, "default2": `DEFAULT2{% block x %}{% endblock %}`, "child": `{% if default %}{% extends default %}{% else %}{% extends 'default1' %}{% endif %}{% block x %}CHILD{% endblock %}`}, vars: `{"default": "default2"}`, trim: false, want: `DEFAULT2CHILD`, err: ``},
		{name: "InheritanceMulti2", src: ``, get: "child", tpls: map[string]string{"default1": `DEFAULT1{% block x %}{% endblock %}`, "default2": `DEFAULT2{% block x %}{% endblock %}`, "child": `{% if default %}{% extends default %}{% else %}{% extends 'default1' %}{% endif %}{% block x %}CHILD{% endblock %}`}, vars: `{"default": "default1"}`, trim: false, want: `DEFAULT1CHILD`, err: ``},
		{name: "InheritanceMulti3", src: ``, get: "child", tpls: map[string]string{"default1": `DEFAULT1{% block x %}{% endblock %}`, "default2": `DEFAULT2{% block x %}{% endblock %}`, "child": `{% if default %}{% extends default %}{% else %}{% extends 'default1' %}{% endif %}{% block x %}CHILD{% endblock %}`}, vars: `{}`, trim: false, want: `DEFAULT1CHILD`, err: ``},
		{name: "InheritanceScopedBlock", src: `{% extends 'default.html' %}{% block item %}{{ item }}{% endblock %}`, get: "", tpls: map[string]string{"default.html": `{% for item in seq %}[{% block item scoped %}{% endblock %}]{% endfor %}`}, vars: `{"seq": [0, 1, 2, 3, 4]}`, trim: false, want: `[0][1][2][3][4]`, err: ``},
		{name: "InheritanceSuperInScopedBlock", src: `{% extends "default.html" %}{% block item %}{{ super() }}|{{ item * 2 }}{% endblock %}`, get: "", tpls: map[string]string{"default.html": `{% for item in seq %}[{% block item scoped %}{{ item }}{% endblock %}]{% endfor %}`}, vars: `{"seq": [0, 1, 2, 3, 4]}`, trim: false, want: `[0|0][1|2][2|4][3|6][4|8]`, err: ``},
		{name: "InheritanceScopedBlockAfterInheritance", src: ``, get: "index.html", tpls: map[string]string{"layout.html": `
            {% block useless %}{% endblock %}
            `, "index.html": `
            {%- extends 'layout.html' %}
            {% from 'helpers.html' import foo with context %}
            {% block useless %}
                {% for x in [1, 2, 3] %}
                    {% block testing scoped %}
                        {{ foo(x) }}
                    {% endblock %}
                {% endfor %}
            {% endblock %}
            `, "helpers.html": `
            {% macro foo(x) %}{{ the_foo + x }}{% endmacro %}
            `}, vars: `{"the_foo": 42}`, trim: false, want: `
            
                
                    
                        43
                    
                
                    
                        44
                    
                
                    
                        45
                    
                
            
            `, err: ``},
		{name: "InheritanceLevel1Required", src: ``, get: "level1", tpls: map[string]string{"default": `{% block x required %}{# comment #}
 {% endblock %}`, "level1": `{% extends 'default' %}{% block x %}[1]{% endblock %}`}, vars: `{}`, trim: false, want: `[1]`, err: ``},
		{name: "InheritanceLevel2Required1", src: ``, get: "level1", tpls: map[string]string{"default": `{% block x required %}{% endblock %}`, "level1": `{% extends 'default' %}{% block x %}[1]{% endblock %}`, "level2": `{% extends 'default' %}{% block x %}[2]{% endblock %}`}, vars: `{}`, trim: false, want: `[1]`, err: ``},
		{name: "InheritanceLevel2Required2", src: ``, get: "level2", tpls: map[string]string{"default": `{% block x required %}{% endblock %}`, "level1": `{% extends 'default' %}{% block x %}[1]{% endblock %}`, "level2": `{% extends 'default' %}{% block x %}[2]{% endblock %}`}, vars: `{}`, trim: false, want: `[2]`, err: ``},
		{name: "InheritanceLevel3Required1", src: ``, get: "level1", tpls: map[string]string{"default": `{% block x required %}{% endblock %}`, "level1": `{% extends 'default' %}`, "level2": `{% extends 'level1' %}{% block x %}[2]{% endblock %}`, "level3": `{% extends 'level2' %}`}, vars: `{}`, trim: false, want: ``, err: `Required block 'x' not found`},
		{name: "InheritanceLevel3Required2", src: ``, get: "level2", tpls: map[string]string{"default": `{% block x required %}{% endblock %}`, "level1": `{% extends 'default' %}`, "level2": `{% extends 'level1' %}{% block x %}[2]{% endblock %}`, "level3": `{% extends 'level2' %}`}, vars: `{}`, trim: false, want: `[2]`, err: ``},
		{name: "InheritanceLevel3Required3", src: ``, get: "level3", tpls: map[string]string{"default": `{% block x required %}{% endblock %}`, "level1": `{% extends 'default' %}`, "level2": `{% extends 'level1' %}{% block x %}[2]{% endblock %}`, "level3": `{% extends 'level2' %}`}, vars: `{}`, trim: false, want: `[2]`, err: ``},
		{name: "InheritanceInvalidRequiredEmpty", src: ``, get: "top", tpls: map[string]string{"empty": `{% block x required %}{% endblock %}`, "blank": `{% block x required %} {# c #}{% endblock %}`, "text": `{% block x required %}data {# c #}{% endblock %}`, "block": `{% block x required %}{% block y %}{% endblock %}{% endblock %}`, "if": `{% block x required %}{% if true %}{% endif %}{% endblock %}`, "top": `{% extends t %}{% block x %}CHILD{% endblock %}`}, vars: `{"t": "empty"}`, trim: false, want: `CHILD`, err: ``},
		{name: "InheritanceInvalidRequiredBlank", src: ``, get: "top", tpls: map[string]string{"empty": `{% block x required %}{% endblock %}`, "blank": `{% block x required %} {# c #}{% endblock %}`, "text": `{% block x required %}data {# c #}{% endblock %}`, "block": `{% block x required %}{% block y %}{% endblock %}{% endblock %}`, "if": `{% block x required %}{% if true %}{% endif %}{% endblock %}`, "top": `{% extends t %}{% block x %}CHILD{% endblock %}`}, vars: `{"t": "blank"}`, trim: false, want: `CHILD`, err: ``},
		{name: "InheritanceInvalidRequiredText", src: ``, get: "top", tpls: map[string]string{"empty": `{% block x required %}{% endblock %}`, "blank": `{% block x required %} {# c #}{% endblock %}`, "text": `{% block x required %}data {# c #}{% endblock %}`, "block": `{% block x required %}{% block y %}{% endblock %}{% endblock %}`, "if": `{% block x required %}{% if true %}{% endif %}{% endblock %}`, "top": `{% extends t %}{% block x %}CHILD{% endblock %}`}, vars: `{"t": "text"}`, trim: false, want: ``, err: `Required blocks can only contain comments or whitespace`},
		{name: "InheritanceInvalidRequiredBlock", src: ``, get: "top", tpls: map[string]string{"empty": `{% block x required %}{% endblock %}`, "blank": `{% block x required %} {# c #}{% endblock %}`, "text": `{% block x required %}data {# c #}{% endblock %}`, "block": `{% block x required %}{% block y %}{% endblock %}{% endblock %}`, "if": `{% block x required %}{% if true %}{% endif %}{% endblock %}`, "top": `{% extends t %}{% block x %}CHILD{% endblock %}`}, vars: `{"t": "block"}`, trim: false, want: ``, err: `Required blocks can only contain comments or whitespace`},
		{name: "InheritanceInvalidRequiredIf", src: ``, get: "top", tpls: map[string]string{"empty": `{% block x required %}{% endblock %}`, "blank": `{% block x required %} {# c #}{% endblock %}`, "text": `{% block x required %}data {# c #}{% endblock %}`, "block": `{% block x required %}{% block y %}{% endblock %}{% endblock %}`, "if": `{% block x required %}{% if true %}{% endif %}{% endblock %}`, "top": `{% extends t %}{% block x %}CHILD{% endblock %}`}, vars: `{"t": "if"}`, trim: false, want: ``, err: `Required blocks can only contain comments or whitespace`},
		{name: "InheritanceRequiredWithScope1", src: ``, get: "child1", tpls: map[string]string{"default1": `{% for item in seq %}[{% block item scoped required %}{% endblock %}]{% endfor %}`, "child1": `{% extends 'default1' %}{% block item %}{{ item }}{% endblock %}`, "default2": `{% for item in seq %}[{% block item required scoped %}{% endblock %}]{% endfor %}`, "child2": `{% extends 'default2' %}{% block item %}{{ item }}{% endblock %}`}, vars: `{"seq": [0, 1, 2]}`, trim: false, want: `[0][1][2]`, err: ``},
		{name: "InheritanceRequiredWithScope2", src: ``, get: "child2", tpls: map[string]string{"default1": `{% for item in seq %}[{% block item scoped required %}{% endblock %}]{% endfor %}`, "child1": `{% extends 'default1' %}{% block item %}{{ item }}{% endblock %}`, "default2": `{% for item in seq %}[{% block item required scoped %}{% endblock %}]{% endfor %}`, "child2": `{% extends 'default2' %}{% block item %}{{ item }}{% endblock %}`}, vars: `{"seq": [0, 1, 2]}`, trim: false, want: ``, err: `expected token 'end of statement block', got 'scoped'`},
		{name: "InheritanceDuplicateScoped", src: ``, get: "child", tpls: map[string]string{"default1": `{% for item in seq %}[{% block item scoped scoped %}}{{% endblock %}}]{{% endfor %}}`, "default2": `{% for item in seq %}[{% block item required required %}}{{% endblock %}}]{{% endfor %}}`, "child": `{% if default %}{% extends default %}{% else %}{% extends 'default1' %}{% endif %}{%- block x %}CHILD{% endblock %}`}, vars: `{"default": "default1", "seq": [0, 1, 2]}`, trim: false, want: ``, err: `expected token 'end of statement block', got 'scoped'`},
		{name: "InheritanceDuplicateRequired", src: ``, get: "child", tpls: map[string]string{"default1": `{% for item in seq %}[{% block item scoped scoped %}}{{% endblock %}}]{{% endfor %}}`, "default2": `{% for item in seq %}[{% block item required required %}}{{% endblock %}}]{{% endfor %}}`, "child": `{% if default %}{% extends default %}{% else %}{% extends 'default1' %}{% endif %}{%- block x %}CHILD{% endblock %}`}, vars: `{"default": "default2", "seq": [0, 1, 2]}`, trim: false, want: ``, err: `expected token 'end of statement block', got 'required'`},
		{name: "InheritanceFixedMacroScopingBug", src: ``, get: "test.html", tpls: map[string]string{"test.html": `        {% extends 'details.html' %}

        {% macro my_macro() %}
        my_macro
        {% endmacro %}

        {% block inner_box %}
            {{ my_macro() }}
        {% endblock %}
            `, "details.html": `        {% extends 'standard.html' %}

        {% macro my_macro() %}
        my_macro
        {% endmacro %}

        {% block content %}
            {% block outer_box %}
                outer_box
                {% block inner_box %}
                    inner_box
                {% endblock %}
            {% endblock %}
        {% endblock %}
        `, "standard.html": `
        {% block content %}&nbsp;{% endblock %}
        `}, vars: `{}`, trim: false, want: `                
        
            
                outer_box
                
            
        my_macro
        
        
            
        
        `, err: ``},
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
