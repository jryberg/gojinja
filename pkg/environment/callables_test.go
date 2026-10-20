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

// TestCallableRepr holds the reprs, equality and calls of builtin methods, functions, classes and block references, and Markup's own methods, checked against Jinja2. Expected values come from the Jinja2 pinned in
// tools/parity/requirements.txt; err is the expected error's message, or
// "*" when only an error is expected. Object addresses in reprs are
// masked.
func TestCallableRepr(t *testing.T) {
	cases := []struct {
		name, src, get string
		tpls           map[string]string
		vars           string
		trim           bool
		want, err      string
	}{
		{name: "Callable1", src: `{{ [1].append }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<built-in method append of list object at 0x…>`, err: ``},
		{name: "Callable2", src: `{{ (1,).count }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<built-in method count of tuple object at 0x…>`, err: ``},
		{name: "Callable3", src: `{{ {'a':1}.items }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<built-in method items of dict object at 0x…>`, err: ``},
		{name: "Callable4", src: `{{ {'a':1}.get }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<built-in method get of dict object at 0x…>`, err: ``},
		{name: "Callable5", src: `{{ 'a'.upper }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<built-in method upper of str object at 0x…>`, err: ``},
		{name: "Callable6", src: `{{ 'a'.format }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<function str.format at 0x…>`, err: ``},
		{name: "Callable7", src: `{{ 'a'.encode('ascii').decode }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<built-in method decode of bytes object at 0x…>`, err: ``},
		{name: "Callable8", src: `{{ (1).bit_length }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<built-in method bit_length of int object at 0x…>`, err: ``},
		{name: "Callable9", src: `{{ (1.5).hex }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<built-in method hex of float object at 0x…>`, err: ``},
		{name: "Callable10", src: `{{ (1.0).fromhex }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<built-in method fromhex of type object at 0x…>`, err: ``},
		{name: "Callable11", src: `{{ (0).from_bytes }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<built-in method from_bytes of type object at 0x…>`, err: ``},
		{name: "Callable12", src: `{{ true.bit_length }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<built-in method bit_length of bool object at 0x…>`, err: ``},
		{name: "Callable13", src: `{{ range(3).index }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<built-in method index of range object at 0x…>`, err: ``},
		{name: "Callable14", src: `{{ {'a':1}.keys().isdisjoint }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<built-in method isdisjoint of dict_keys object at 0x…>`, err: ``},
		{name: "Callable15", src: `{{ ('a'|e).upper }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<bound method Markup.upper of Markup('a')>`, err: ``},
		{name: "Callable16", src: `{{ ('a'|e).format }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<function Markup.format at 0x…>`, err: ``},
		{name: "Callable17", src: `{{ ('a'|e).join }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<bound method Markup.join of Markup('a')>`, err: ``},
		{name: "Callable18", src: `{{ ('a'|e).striptags }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<bound method Markup.striptags of Markup('a')>`, err: ``},
		{name: "Callable19", src: `{{ ('a'|e).unescape }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<bound method Markup.unescape of Markup('a')>`, err: ``},
		{name: "Callable20", src: `{{ ('a'|e).escape }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<bound method Markup.escape of <class 'markupsafe.Markup'>>`, err: ``},
		{name: "Callable21", src: `{{ namespace }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<class 'jinja2.utils.Namespace'>`, err: ``},
		{name: "Callable22", src: `{{ namespace(a=1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<Namespace {'a': 1}>`, err: ``},
		{name: "Callable23", src: `{{ cycler(1,2) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<jinja2.utils.Cycler object at 0x…>`, err: ``},
		{name: "Callable24", src: `{{ cycler(1,2).next }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<bound method Cycler.next of <jinja2.utils.Cycler object at 0x…>>`, err: ``},
		{name: "Callable25", src: `{{ cycler(1,2).reset }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<bound method Cycler.reset of <jinja2.utils.Cycler object at 0x…>>`, err: ``},
		{name: "Callable26", src: `{{ joiner(',') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<jinja2.utils.Joiner object at 0x…>`, err: ``},
		{name: "Callable27", src: `{{ range }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<function safe_range at 0x…>`, err: ``},
		{name: "Callable28", src: `{{ dict }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<class 'dict'>`, err: ``},
		{name: "Callable29", src: `{{ lipsum }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<function generate_lorem_ipsum at 0x…>`, err: ``},
		{name: "Callable30", src: `{{ dict.fromkeys }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<built-in method fromkeys of type object at 0x…>`, err: ``},
		{name: "Callable31", src: `{{ {}.fromkeys }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<built-in method fromkeys of type object at 0x…>`, err: ``},
		{name: "Callable32", src: `{{ ([1]|select).send }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<built-in method send of generator object at 0x…>`, err: ``},
		{name: "Callable33", src: `{{ ([1]|select).close }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<built-in method close of generator object at 0x…>`, err: ``},
		{name: "Callable34", src: `{{ self }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<TemplateReference None>`, err: ``},
		{name: "Callable35", src: `{% macro m() %}{% endmacro %}{{ m }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<Macro 'm'>`, err: ``},
		{name: "Callable36", src: `{% for i in [1] %}{{ loop.cycle }}|{{ loop.changed }}|{{ loop }}{% endfor %}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<bound method LoopContext.cycle of <LoopContext 1/1>>|<bound method LoopContext.changed of <LoopContext 1/1>>|<LoopContext 1/1>`, err: ``},
		{name: "Callable37", src: `{% macro m() %}{{ caller }}{% endmacro %}{% call m() %}x{% endcall %}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<Macro anonymous>`, err: ``},
		{name: "Callable38", src: `{% block b %}{% endblock %}{{ self.b }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<jinja2.runtime.BlockReference object at 0x…>`, err: ``},
		{name: "Callable39", src: `{{ [[1].append] }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[<built-in method append of list object at 0x…>]`, err: ``},
		{name: "Callable40", src: `{{ [1].append is callable }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `True`, err: ``},
		{name: "Callable41", src: `{{ [1].append == [1].append }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `False`, err: ``},
		{name: "Callable42", src: `{{ ('a'|e).upper ~ '' }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<bound method Markup.upper of Markup('a')>`, err: ``},
		{name: "Callable43", src: `{{ 'a'.upper|tojson }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `Object of type builtin_function_or_method is not JSON serializable`},
		{name: "Callable44", src: `{{ {'a'.upper: 1}|length }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1`, err: ``},
		{name: "Callable45", src: `{{ x.append }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `'x' is undefined`},
		{name: "Callable46", src: `{{ x.pop }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `'x' is undefined`},
		{name: "Callable47", src: `{{ x.index }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `'x' is undefined`},
		{name: "Callable48", src: `{% set d = {'a': 1, 'b': 2} %}{{ d.keys().isdisjoint([3]) }}|{{ d.keys().isdisjoint("ab") }}|{{ d.items().isdisjoint([("a", 1)]) }}|{{ d.items().isdisjoint([["a", 1]]) }}|{{ d.items().isdisjoint([("a", [1])]) }}|{{ d.items().isdisjoint([1]) }}|{{ d.values().isdisjoint }}|{{ d.keys().isdisjoint(d) }}|{{ d.keys().isdisjoint(d.items()) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `True|False|False|True|True|True||False|True`, err: ``},
		{name: "Callable49", src: `{% set d = {'a': 1} %}{{ d.keys().isdisjoint([[1]]) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `unhashable type: 'list'`},
		{name: "Callable50", src: `{% set d = {'a': 1} %}{{ d.keys().isdisjoint(5) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `'int' object is not iterable`},
		{name: "Callable51", src: `{% set d = {'a': 1} %}{{ d.keys().isdisjoint() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `dict_keys.isdisjoint() takes exactly one argument (0 given)`},
		{name: "Callable52", src: `{% set d = {'a': 1} %}{{ d.keys().isdisjoint(x=[1]) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `dict_keys.isdisjoint() takes no keyword arguments`},
		{name: "Callable53", src: `{% for s in ['a < b > c', '<a', 'x <!-- y', 'a<!--<b>-->c', '&amp;lt; &notin &notit; &#0; &#x110000; &#128; &AMP &amp', 'a\x1cb', 'tab\tnl\n', '<p>&#39;x&#39;</p>', '&#xD800;', '&#65;&#x41;&nbsp;x', '&#9;&#13;&#x80;', '&ThickSpace;&nGt;&acE;x&#x;&#;&#xFFFE;&#1114111;&#x0d;'] %}{{ (s|safe).striptags()|pprint }} {{ (s|safe).unescape()|pprint }} {{ s|striptags|pprint }};{% endfor %}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `'a c' 'a < b > c' 'a c';'<a' '<a' '<a';'x <!-- y' 'x <!-- y' 'x <!-- y';'ac' 'a<!--<b>-->c' 'ac';'&lt; ¬in ¬it; � � € & &' '&lt; ¬in ¬it; � � € & &' '&lt; ¬in ¬it; � � € & &';'a b' 'a\x1cb' 'a b';'tab nl' 'tab\tnl\n' 'tab nl';"'x'" "<p>'x'</p>" "'x'";'�' '�' '�';'AA\xa0x' 'AA\xa0x' 'AA\xa0x';'\t\r€' '\t\r€' '\t\r€';'\u205f\u200a≫⃒∾̳x&#x;&#;\r' '\u205f\u200a≫⃒∾̳x&#x;&#;\r' '\u205f\u200a≫⃒∾̳x&#x;&#;\r';`, err: ``},
		{name: "Callable54", src: `{{ ('<a>'|safe).escape('<b>') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `&lt;b&gt;`, err: ``},
		{name: "Callable55", src: `{{ ('x'|safe).escape(s='<i>') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `Markup.escape() got some positional-only arguments passed as keyword arguments: 's'`},
		{name: "Callable56", src: `{{ ('x'|safe).escape(5) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `5`, err: ``},
		{name: "Callable57", src: `{{ ('x'|safe).escape('a'|safe) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `a`, err: ``},
		{name: "Callable58", src: `{{ (('<b>x</b>'|safe).striptags()) is escaped }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `False`, err: ``},
		{name: "Callable59", src: `{{ ('&lt;'|safe).unescape() is escaped }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `False`, err: ``},
		{name: "Callable60", src: `{{ ('x'|safe).striptags(1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `Markup.striptags() takes 1 positional argument but 2 were given`},
		{name: "Callable61", src: `{{ ('x'|safe).striptags(a=1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `Markup.striptags() got an unexpected keyword argument 'a'`},
		{name: "Callable62", src: `{{ ('x'|safe).escape() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `Markup.escape() missing 1 required positional argument: 's'`},
		{name: "Callable63", src: `{{ ('x'|safe).escape(1, 2) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `Markup.escape() takes 2 positional arguments but 3 were given`},
		{name: "Callable64", src: `{{ ('x'|safe).startswith }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<built-in method startswith of Markup object at 0x…>`, err: ``},
		{name: "Callable65", src: `{{ ('x'|safe).isdigit }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<built-in method isdigit of Markup object at 0x…>`, err: ``},
		{name: "Callable66", src: `{{ ('x'|safe).encode }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<built-in method encode of Markup object at 0x…>`, err: ``},
		{name: "Callable67", src: `{{ ('x'|safe).split }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<bound method Markup.split of Markup('x')>`, err: ``},
		{name: "Callable68", src: `{{ ('x'|safe).casefold }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<bound method Markup.casefold of Markup('x')>`, err: ``},
		{name: "Callable69", src: `{{ ('x'|safe).format_map }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<function Markup.format_map at 0x…>`, err: ``},
		{name: "Callable70", src: `{{ 'x'.format_map }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<function str.format_map at 0x…>`, err: ``},
		{name: "Callable71", src: `{% set c = cycler(1, 2) %}{{ c.next == c.next }}|{{ c.next is sameas c.next }}|{% set l = [1] %}{{ l.append == l.append }}|{{ {l.append: 1, l.append: 2}|length }}|{{ 'a'.upper == 'a'.upper }}|{{ [1].append == [1].append }}|{{ (range == range) }}|{{ lipsum is callable }}|{{ cycler is callable }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `True|False|True|1|True|False|True|True|True`, err: ``},
		{name: "Callable72", src: `{{ 'a'.upper|tojson }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `Object of type builtin_function_or_method is not JSON serializable`},
		{name: "Callable73", src: `{{ lipsum|tojson }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `Object of type function is not JSON serializable`},
		{name: "Callable74", src: `{{ cycler(1)|tojson }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `Object of type Cycler is not JSON serializable`},
		{name: "Callable75", src: `{{ cycler(1,2) is callable }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `False`, err: ``},
		{name: "Callable76", src: `{{ joiner() is callable }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `True`, err: ``},
		{name: "Callable77", src: `{{ 'a'.upper is callable }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `True`, err: ``},
		{name: "Callable78", src: `{{ range(2) is callable }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `False`, err: ``},
		{name: "Callable79", src: `{% for s in ['&amp;lt; &notin &notit; &#0; &#x110000; &#128; &AMP &amp', '&ThickSpace;&nGt;&acE;x&#x;&#;&#xFFFE;&#1114111;&#x0d;', '&#99999999999999999999;&#x41&#65x&#X41;&amp&ampx&amp;x', '&a&ab&abc&#&#x&#xg&; &nbsp&nbspx&copy2&;', '&' ~ 'x' * 40 ~ ';&lt' ~ '&é;&Aacute&aacute;&notindot;&NotEqualTilde;', '&#xd800;&#xDFFF;&#1;&#x7f;&#x9f;&#xfdd0;&#x1fffe;&#x80;&#x81;&#x9F;&#13;&#9;&#10;'] %}{{ (s|safe).unescape()|pprint }}|{{ s|striptags|pprint }};{% endfor %}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `'&lt; ¬in ¬it; � � € & &'|'&lt; ¬in ¬it; � � € & &';'\u205f\u200a≫⃒∾̳x&#x;&#;\r'|'\u205f\u200a≫⃒∾̳x&#x;&#;\r';'�AAxA&&x&x'|'�AAxA&&x&x';'&a&ab&abc&#&#x&#xg&; \xa0\xa0x©2&;'|'&a&ab&abc&#&#x&#xg&; \xa0\xa0x©2&;';'&xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx;<&é;Áá⋵̸≂̸'|'&xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx;<&é;Áá⋵̸≂̸';'��Ÿ€\x81Ÿ\r\t\n'|'��Ÿ€\x81Ÿ\r\t\n';`, err: ``},
		{name: "BlockReference", src: ``, get: "child", tpls: map[string]string{"base": `{% block b %}B{% endblock %}{% block c %}C{% endblock %}`, "child": `{% extends "base" %}{% block b %}{{ super }}|{{ super() }}|{{ self.c }}|{{ self.c() }}|{{ self.c.super }}|{{ self.c == self.c }}|{{ self.c is callable }}|{{ self }}|{{ [self.c] }}|{{ super.super }}{% endblock %}`}, vars: `{}`, trim: false, want: `<jinja2.runtime.BlockReference object at 0x…>|B|<jinja2.runtime.BlockReference object at 0x…>|C||False|True|<TemplateReference 'child'>|[<jinja2.runtime.BlockReference object at 0x…>]|C`, err: ``},
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
