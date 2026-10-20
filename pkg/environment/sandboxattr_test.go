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

// TestSandboxAttributes holds SandboxedEnvironment's unsafe attributes, which become an Undefined raising SecurityError on use, and the dict views' mappingproxy, checked against Jinja2. Expected values come from the Jinja2 pinned in
// tools/parity/requirements.txt; err is the expected error's message, or
// "*" when only an error is expected. Object addresses in reprs are
// masked.
func TestSandboxAttributes(t *testing.T) {
	cases := []struct {
		name, src, get string
		tpls           map[string]string
		vars           string
		trim           bool
		want, err      string
	}{
		{name: "Sandbox1", src: `{% set d = {'_id': 7, '__x': 8, 'a': {'_b': 9}} %}{{ d._id }}|{{ d['_id'] }}|{{ d.__x }}|{{ d.a._b }}|{{ d|attr('_id') }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `7|7|8|9|`, err: ``},
		{name: "Sandbox2", src: `{{ x.__class__ }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: ``},
		{name: "Sandbox3", src: `{{ x.__class__ is defined }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `False`, err: ``},
		{name: "Sandbox4", src: `{{ x.__class__|default('d') }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `d`, err: ``},
		{name: "Sandbox5", src: `{{ 'y' if x.__class__ else 'n' }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `n`, err: ``},
		{name: "Sandbox6", src: `{{ x.__nope__ }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: ``},
		{name: "Sandbox7", src: `{{ x._private }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: ``},
		{name: "Sandbox8", src: `{{ x.__class__ is undefined }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `True`, err: ``},
		{name: "Sandbox9", src: `{{ [x.__class__] }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `[Undefined]`, err: ``},
		{name: "Sandbox10", src: `{{ x.__class__|string }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: ``},
		{name: "Sandbox11", src: `{{ x.__class__ ~ '!' }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `!`, err: ``},
		{name: "Sandbox12", src: `{{ x.__class__|length }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `0`, err: ``},
		{name: "Sandbox13", src: `{{ x.__class__|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `[]`, err: ``},
		{name: "Sandbox14", src: `{{ x.__class__.__name__ }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `access to attribute '__class__' of 'list' object is unsafe.`},
		{name: "Sandbox15", src: `{{ x.__class__() }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `access to attribute '__class__' of 'list' object is unsafe.`},
		{name: "Sandbox16", src: `{{ x.__class__[0] }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `access to attribute '__class__' of 'list' object is unsafe.`},
		{name: "Sandbox17", src: `{{ x.__class__ + 1 }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `access to attribute '__class__' of 'list' object is unsafe.`},
		{name: "Sandbox18", src: `{{ x['__class__'] }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: ``},
		{name: "Sandbox19", src: `{{ x['__nope__'] }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: ``},
		{name: "Sandbox20", src: `{{ x|attr('__class__') }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: ``},
		{name: "Sandbox21", src: `{{ 'a'.__len__ }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: ``},
		{name: "Sandbox22", src: `{{ joiner().__call__ }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: ``},
		{name: "Sandbox23", src: `{{ cycler(1).__dict__ }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: ``},
		{name: "Sandbox24", src: `{{ lipsum.__globals__ }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: ``},
		{name: "Sandbox25", src: `{{ namespace(a=1).__dict__ }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: ``},
		{name: "Sandbox26", src: `{{ ([1]|select).gi_frame }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: ``},
		{name: "Sandbox27", src: `{{ ([1]|select).gi_code }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: ``},
		{name: "Sandbox28", src: `{{ ''.__class__.__mro__ }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `access to attribute '__class__' of 'str' object is unsafe.`},
		{name: "Sandbox29", src: `{{ x.__class__.__mro__[1] }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `access to attribute '__class__' of 'list' object is unsafe.`},
		{name: "Sandbox30", src: `{{ ([1]|select).gi_frame.f_globals }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `access to attribute 'gi_frame' of 'generator' object is unsafe.`},
		{name: "Sandbox31", src: `{% set ns = namespace(_a=1) %}{{ ns._a }}|{% set ns._b = 2 %}{{ ns._b }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `|`, err: ``},
		{name: "Sandbox32", src: `{{ x.__class__ == x.__class__ }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `True`, err: ``},
		{name: "Sandbox33", src: `{{ x.__class__ is none }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `False`, err: ``},
		{name: "Sandbox34", src: `{{ x.__class__ in [1] }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `False`, err: ``},
		{name: "Sandbox35", src: `{{ x.__class__|tojson }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `Object of type Undefined is not JSON serializable`},
		{name: "Sandbox36", src: `{% for i in x.__class__ %}{{ i }}{% endfor %}ok|{{ x.__class__|pprint }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `ok|Undefined`, err: ``},
		{name: "Sandbox37", src: `{{ u.__class__ }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: ``},
		{name: "Sandbox38", src: `{{ u.__class__ is defined }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `False`, err: ``},
		{name: "Sandbox39", src: `{{ x.y.__class__ }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: ``},
		{name: "Sandbox40", src: `{{ u._x }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `'u' is undefined`},
		{name: "Sandbox41", src: `{% macro m() %}{% endmacro %}{{ m.__class__ }}|{{ m._x }}|{{ m.__call__ }}|{% for i in [1] %}{{ loop.__class__ }}|{{ loop._x }}{% endfor %}|{{ self.__class__ }}|{{ cycler.__init__ }}|{{ cycler.mro }}|{{ dict.mro }}|{{ range.__name__ }}|{{ lipsum.__code__ }}|{{ lipsum.__name__ }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `|||||||||||`, err: ``},
		{name: "Sandbox42", src: `{{ '{0.__class__}'.format(x) }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: ``},
		{name: "Sandbox43", src: `{{ '{0._x}'.format(x) }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: ``},
		{name: "Sandbox44", src: `{{ '{0[__class__]}'.format({'__class__': 5}) }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `5`, err: ``},
		{name: "Sandbox45", src: `{{ {'__class__': 5}['__class__'] }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `5`, err: ``},
		{name: "Sandbox46", src: `{{ {'__class__': 5}.__class__ }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: ``},
		{name: "Sandbox47", src: `{{ ({'a':1}|attr('__class__')) }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: ``},
		{name: "Sandbox48", src: `{{ ({'_a':1}|attr('_a')) }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: ``},
		{name: "Sandbox49", src: `{{ (x|attr('_nope')) }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: ``},
		{name: "Sandbox50", src: `{{ '{0.__class__.__name__}'.format(x) }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `access to attribute '__class__' of 'list' object is unsafe.`},
		{name: "Sandbox51", src: `{{ cycler.mro() }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `access to attribute 'mro' of 'type' object is unsafe.`},
		{name: "Sandbox52", src: `{% macro m() %}{% endmacro %}{{ m.__class__.__name__ }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `access to attribute '__class__' of 'Macro' object is unsafe.`},
		{name: "Sandbox53", src: `{{ x._nope.y }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `'list object' has no attribute '_nope'`},
		{name: "Sandbox54", src: `{% set ns = namespace(_a=1, b=2) %}{{ ns['_a'] }}|{{ ns['b'] }}|{{ ns|attr('_a') }}|{{ ns.__class__ }}|{{ '{0._a}'.format(ns) }}|{{ '%(_a)s' % {'_a': 3} }}|{{ ns._a is defined }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `|2||||3|False`, err: ``},
		{name: "Sandbox55", src: `{% set ns = namespace(_a=1) %}{{ ns._a + 1 }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `access to attribute '_a' of 'Namespace' object is unsafe.`},
		{name: "Sandbox56", src: `{{ x['__class__'].__name__ }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `access to attribute '__class__' of 'list' object is unsafe.`},
		{name: "Sandbox57", src: `{{ (x|attr('__class__')).__name__ }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `access to attribute '__class__' of 'list' object is unsafe.`},
		{name: "Sandbox58", src: `{% set d = {"a": 1} %}{% set p = d.keys().mapping %}{{ p }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `{'a': 1}`, err: ``},
		{name: "Sandbox59", src: `{% set d = {"a": 1} %}{% set p = d.keys().mapping %}{{ [p] }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `[mappingproxy({'a': 1})]`, err: ``},
		{name: "Sandbox60", src: `{% set d = {"a": 1} %}{% set p = d.keys().mapping %}{{ p|pprint }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `mappingproxy({'a': 1})`, err: ``},
		{name: "Sandbox61", src: `{% set d = {"a": 1} %}{% set p = d.keys().mapping %}{{ p.get("a") }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `1`, err: ``},
		{name: "Sandbox62", src: `{% set d = {"a": 1} %}{% set p = d.keys().mapping %}{{ p.copy() }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `{'a': 1}`, err: ``},
		{name: "Sandbox63", src: `{% set d = {"a": 1} %}{% set p = d.keys().mapping %}{{ [p.copy()] }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `[{'a': 1}]`, err: ``},
		{name: "Sandbox64", src: `{% set d = {"a": 1} %}{% set p = d.keys().mapping %}{{ p.keys() }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `dict_keys(['a'])`, err: ``},
		{name: "Sandbox65", src: `{% set d = {"a": 1} %}{% set p = d.keys().mapping %}{{ p.update }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: ``},
		{name: "Sandbox66", src: `{% set d = {"a": 1} %}{% set p = d.keys().mapping %}{{ p.items }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `<built-in method items of mappingproxy object at 0x…>`, err: ``},
		{name: "Sandbox67", src: `{% set d = {"a": 1} %}{% set p = d.keys().mapping %}{{ p == d }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `True`, err: ``},
		{name: "Sandbox68", src: `{% set d = {"a": 1} %}{% set p = d.keys().mapping %}{{ p.a }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `1`, err: ``},
		{name: "Sandbox69", src: `{% set d = {"a": 1} %}{% set p = d.keys().mapping %}{{ p["a"] }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `1`, err: ``},
		{name: "Sandbox70", src: `{% set d = {"a": 1} %}{% set p = d.keys().mapping %}{{ p|length }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `1`, err: ``},
		{name: "Sandbox71", src: `{% set d = {"a": 1} %}{% set p = d.keys().mapping %}{{ p|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `['a']`, err: ``},
		{name: "Sandbox72", src: `{% set d = {"a": 1} %}{% set p = d.keys().mapping %}{{ p is mapping }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `True`, err: ``},
		{name: "Sandbox73", src: `{% set d = {"a": 1} %}{% set p = d.keys().mapping %}{{ p|items|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `[('a', 1)]`, err: ``},
		{name: "Sandbox74", src: `{% set d = {"a": 1} %}{% set p = d.keys().mapping %}{{ p|dictsort }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `[('a', 1)]`, err: ``},
		{name: "Sandbox75", src: `{% set d = {"a": 1} %}{% set p = d.keys().mapping %}{{ "a" in p }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `True`, err: ``},
		{name: "Sandbox76", src: `{% set d = {"a": 1} %}{% set p = d.keys().mapping %}{{ p|tojson }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `Object of type mappingproxy is not JSON serializable`},
		{name: "Sandbox77", src: `{% set d = {"a": 1} %}{% set p = d.keys().mapping %}{{ {p: 1} }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `unhashable type: 'dict'`},
		{name: "Sandbox78", src: `{% set d = {"a": 1} %}{% set p = d.keys().mapping %}{{ p|string }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `{'a': 1}`, err: ``},
		{name: "Sandbox79", src: `{% set d = {"a": 1} %}{% set p = d.keys().mapping %}{{ (d.update({"b": 2}) or p) }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `{'a': 1, 'b': 2}`, err: ``},
		{name: "Sandbox80", src: `{% set d = {"a": 1} %}{% set p = d.keys().mapping %}{{ p.__class__ }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: ``},
		{name: "Sandbox81", src: `{% set d = {"a": 1} %}{% set p = d.keys().mapping %}{{ p.pop }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: ``},
		{name: "Sandbox82", src: `{% set d = {"a": 1} %}{% set p = d.keys().mapping %}{{ p.setdefault }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: ``},
		{name: "Sandbox83", src: `{% set d = {"a": 1} %}{% set p = d.keys().mapping %}{{ p.fromkeys }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: ``},
		{name: "Sandbox84", src: `{% set d = {"a": 1} %}{% set p = d.keys().mapping %}{{ p|attr("get") }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `<built-in method get of mappingproxy object at 0x…>`, err: ``},
		{name: "Sandbox85", src: `{% set d = {"a": 1} %}{% set p = d.keys().mapping %}{{ p.values()|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `[1]`, err: ``},
		{name: "Sandbox86", src: `{% set d = {"a": 1} %}{% set p = d.keys().mapping %}{{ p.mapping }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: ``},
		{name: "Sandbox87", src: `{% set d = {"a": 1} %}{% set p = d.keys().mapping %}{{ d.values().mapping }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `{'a': 1}`, err: ``},
		{name: "Sandbox88", src: `{% set d = {"a": 1} %}{% set p = d.keys().mapping %}{{ d.items().mapping == d }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `True`, err: ``},
		{name: "Sandbox89", src: `{% set d = {"a": 1} %}{% set p = d.keys().mapping %}{{ p.get("zz", 5) }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `5`, err: ``},
		{name: "Sandbox90", src: `{% set d = {"a": 1} %}{% set p = d.keys().mapping %}{{ p.copy() is mapping }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `True`, err: ``},
		{name: "Sandbox91", src: `{% set d = {"a": 1} %}{% set p = d.keys().mapping %}{{ p.copy().update }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `<built-in method update of dict object at 0x…>`, err: ``},
		{name: "Sandbox92", src: `{% set d = {"a": 1} %}{% set p = d.keys().mapping %}{% for k, v in p.items() %}{{ k }}{{ v }}{% endfor %}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `a1`, err: ``},
		{name: "Sandbox93", src: `{% set d = {"a": 1} %}{% set p = d.keys().mapping %}{% for k in p %}{% do d.update({"z": 1}) %}{% endfor %}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `dictionary changed size during iteration`},
		{name: "Sandbox94", src: `{% set d = {"a": 1} %}{% set p = d.keys().mapping %}{{ p.keys().mapping }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `{'a': 1}`, err: ``},
		{name: "Sandbox95", src: `{% set d = {"a": 1} %}{% set p = d.keys().mapping %}{{ p|pprint }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `mappingproxy({'a': 1})`, err: ``},
		{name: "Sandbox96", src: `{% set d = {"a": 1} %}{% set p = d.keys().mapping %}{{ p.update({"q": 1}) }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `'mappingproxy object' has no attribute 'update'`},
		{name: "Sandbox97", src: `{% set d = {"a": 1} %}{% set p = d.keys().mapping %}{{ p ~ "" }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `{'a': 1}`, err: ``},
		{name: "Sandbox98", src: `{% set d = {"a": 1} %}{% set p = d.keys().mapping %}{{ p|e }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `{&#39;a&#39;: 1}`, err: ``},
		{name: "Sandbox99", src: `{% set d = {"a": 1} %}{% set p = d.keys().mapping %}{{ "%s|%r" % (p, p) }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `{'a': 1}|mappingproxy({'a': 1})`, err: ``},
		{name: "Sandbox100", src: `{% set d = {"a": 1} %}{% set p = d.keys().mapping %}{{ "{}|{!r}".format(p, p) }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `{'a': 1}|mappingproxy({'a': 1})`, err: ``},
		{name: "Sandbox101", src: `{% set d = {"a": 1} %}{% set p = d.keys().mapping %}{{ p is sameas p }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `True`, err: ``},
		{name: "Sandbox102", src: `{% set d = {"a": 1} %}{% set p = d.keys().mapping %}{{ p.get }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `<built-in method get of mappingproxy object at 0x…>`, err: ``},
		{name: "Sandbox103", src: `{% set d = {"a": 1} %}{% set p = d.keys().mapping %}{{ p.keys().mapping is sameas p }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `False`, err: ``},
		{name: "Sandbox104", src: `{% set d = dict() %}{% for i in range(12) %}{% do d.update({"key" ~ i: "value number " ~ i}) %}{% endfor %}{{ d.keys().mapping|pprint }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `mappingproxy({'key0': 'value number 0',
              'key1': 'value number 1',
              'key10': 'value number 10',
              'key11': 'value number 11',
              'key2': 'value number 2',
              'key3': 'value number 3',
              'key4': 'value number 4',
              'key5': 'value number 5',
              'key6': 'value number 6',
              'key7': 'value number 7',
              'key8': 'value number 8',
              'key9': 'value number 9'})`, err: ``},
		{name: "Sandbox105", src: `{{ {'a':1}|attr('items') }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `<built-in method items of dict object at 0x…>`, err: ``},
		{name: "Sandbox106", src: `{{ [1]|attr('append') }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `<built-in method append of list object at 0x…>`, err: ``},
		{name: "Sandbox107", src: `{{ 'a'|attr('upper') }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `<built-in method upper of str object at 0x…>`, err: ``},
		{name: "Sandbox108", src: `{{ ('a'|e)|attr('upper') }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `<bound method Markup.upper of Markup('a')>`, err: ``},
		{name: "Sandbox109", src: `{% set x = [1] %}{% do (x|attr('append'))(5) %}{{ x }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `[1, 5]`, err: ``},
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
