package environment

import (
	"context"
	"strings"
	"testing"

	"github.com/jryberg/gojinja/pkg/lexer"
	"github.com/jryberg/gojinja/pkg/loader"
	"github.com/jryberg/gojinja/pkg/varsutil"
)

// TestContainers holds list, tuple and dict methods, aliasing, keyword order and dict keys checked against Jinja2. Expected values come from the Jinja2 pinned in
// tools/parity/requirements.txt; err is the expected error's message, or
// "*" when only an error is expected.
func TestContainers(t *testing.T) {
	cases := []struct {
		name, src, get string
		tpls           map[string]string
		vars           string
		trim           bool
		want, err      string
	}{
		{name: "Method1", src: `{{ [].append() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `list.append() takes exactly one argument (0 given)`},
		{name: "Method2", src: `{{ [].append(1, 2) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `list.append() takes exactly one argument (2 given)`},
		{name: "Method3", src: `{{ [].append(x=1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `list.append() takes no keyword arguments`},
		{name: "Method4", src: `{{ [].insert(1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `insert expected 2 arguments, got 1`},
		{name: "Method5", src: `{{ [].insert('a', 1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `'str' object cannot be interpreted as an integer`},
		{name: "Method6", src: `{{ [1].pop(1, 2) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `pop expected at most 1 argument, got 2`},
		{name: "Method7", src: `{{ [1].pop('a') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `'str' object cannot be interpreted as an integer`},
		{name: "Method8", src: `{{ [].pop() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `pop from empty list`},
		{name: "Method9", src: `{{ [1].pop(5) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `pop index out of range`},
		{name: "Method10", src: `{{ [1].pop(-2) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `pop index out of range`},
		{name: "Method11", src: `{{ [].remove(1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `list.remove(x): x not in list`},
		{name: "Method12", src: `{{ [1].remove() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `list.remove() takes exactly one argument (0 given)`},
		{name: "Method13", src: `{{ [1].index(2) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `2 is not in list`},
		{name: "Method14", src: `{{ [1].index() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `index expected at least 1 argument, got 0`},
		{name: "Method15", src: `{{ [1].index(1, 'a') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `slice indices must be integers or have an __index__ method`},
		{name: "Method16", src: `{{ [1, 2, 1].index(1, 1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `2`, err: ``},
		{name: "Method17", src: `{{ [1, 2, 1].index(1, -1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `2`, err: ``},
		{name: "Method18", src: `{{ [1, 2, 1].index(1, 0, 1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `0`, err: ``},
		{name: "Method19", src: `{{ [1].count() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `list.count() takes exactly one argument (0 given)`},
		{name: "Method20", src: `{{ [1, 'a'].sort() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `'<' not supported between instances of 'str' and 'int'`},
		{name: "Method21", src: `{{ [3, 1].sort(1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `sort() takes no positional arguments`},
		{name: "Method22", src: `{{ [3, 1].sort(reverse=1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `None`, err: ``},
		{name: "Method23", src: `{{ [3, 1].sort(key=none, reverse=true) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `None`, err: ``},
		{name: "Method24", src: `{{ [3, 1].sort(bad=1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `'bad' is an invalid keyword argument for sort()`},
		{name: "Method25", src: `{{ [].extend(1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `'int' object is not iterable`},
		{name: "Method26", src: `{{ [].extend() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `list.extend() takes exactly one argument (0 given)`},
		{name: "Method27", src: `{{ [].copy(1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `list.copy() takes no arguments (1 given)`},
		{name: "Method28", src: `{{ [].clear(1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `list.clear() takes no arguments (1 given)`},
		{name: "Method29", src: `{{ [].reverse(1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `list.reverse() takes no arguments (1 given)`},
		{name: "Method30", src: `{{ {}.get() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `get expected at least 1 argument, got 0`},
		{name: "Method31", src: `{{ {}.get(1, 2, 3) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `get expected at most 2 arguments, got 3`},
		{name: "Method32", src: `{{ {}.keys(1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `dict.keys() takes no arguments (1 given)`},
		{name: "Method33", src: `{{ {}.pop() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `pop expected at least 1 argument, got 0`},
		{name: "Method34", src: `{{ {}.pop('z') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `'z'`},
		{name: "Method35", src: `{{ {}.popitem() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `'popitem(): dictionary is empty'`},
		{name: "Method36", src: `{{ {}.setdefault() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `setdefault expected at least 1 argument, got 0`},
		{name: "Method37", src: `{{ {}.update(1, 2) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `update expected at most 1 argument, got 2`},
		{name: "Method38", src: `{{ {}.update([1]) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `cannot convert dictionary update sequence element #0 to a sequence`},
		{name: "Method39", src: `{{ {}.update(1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `'int' object is not iterable`},
		{name: "Method40", src: `{{ {}.update([[1, 2, 3]]) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `dictionary update sequence element #0 has length 3; 2 is required`},
		{name: "Method41", src: `{{ {}.fromkeys() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `fromkeys expected at least 1 argument, got 0`},
		{name: "Method42", src: `{{ {}.fromkeys(1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `'int' object is not iterable`},
		{name: "Method43", src: `{{ {}.copy(1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `dict.copy() takes no arguments (1 given)`},
		{name: "Method44", src: `{{ {}.clear(1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `dict.clear() takes no arguments (1 given)`},
		{name: "Method45", src: `{{ {}.values(1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `dict.values() takes no arguments (1 given)`},
		{name: "Method46", src: `{{ {}.items(1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `dict.items() takes no arguments (1 given)`},
		{name: "Method47", src: `{{ {}.get(x=1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `dict.get() takes no keyword arguments`},
		{name: "Method48", src: `{{ {}.update({'a': 1}, b=2) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `None`, err: ``},
		{name: "Method49", src: `{{ {}.update([('a', 1)], b=2) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `None`, err: ``},
		{name: "Method50", src: `{{ (1,).count() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `tuple.count() takes exactly one argument (0 given)`},
		{name: "Method51", src: `{{ (1,).index(2) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `tuple.index(x): x not in tuple`},
		{name: "Method52", src: `{{ (1, 2).index(2, 0, 1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `tuple.index(x): x not in tuple`},
		{name: "Method53", src: `{{ [[1]].index([1]) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `0`, err: ``},
		{name: "Method54", src: `{{ [1].pop(1.0) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `'float' object cannot be interpreted as an integer`},
		{name: "Method55", src: `{{ [1].insert(0.5, 1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `'float' object cannot be interpreted as an integer`},
		{name: "Method56", src: `{{ [1,2,3].pop(true) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `2`, err: ``},
		{name: "Method57", src: `{{ {1: 2}.pop(1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `2`, err: ``},
		{name: "Method58", src: `{{ {'a': 1}.pop('a', 2, 3) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `pop expected at most 2 arguments, got 3`},
		{name: "Method59", src: `{{ [].insert(0, x=1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `list.insert() takes no keyword arguments`},
		{name: "Method60", src: `{{ [].pop(x=1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `list.pop() takes no keyword arguments`},
		{name: "Method61", src: `{{ [1].index(1, x=1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `list.index() takes no keyword arguments`},
		{name: "Method62", src: `{{ [].count(x=1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `list.count() takes no keyword arguments`},
		{name: "Method63", src: `{{ {}.popitem(1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `dict.popitem() takes no arguments (1 given)`},
		{name: "Method64", src: `{{ (1,).index(x=1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `tuple.index() takes no keyword arguments`},
		{name: "Method65", src: `{{ dict.fromkeys([1], 2) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `{1: 2}`, err: ``},
		{name: "Method66", src: `{{ dict.fromkeys([1], 2, 3) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `fromkeys expected at most 2 arguments, got 3`},
		{name: "Method67", src: `{{ {}.fromkeys([1], value=2) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `dict.fromkeys() takes no keyword arguments`},
		{name: "Method68", src: `{{ [1].index(1, 0, 1, 2) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `index expected at most 3 arguments, got 4`},
		{name: "Method69", src: `{{ {'a':1}.keys() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `dict_keys(['a'])`, err: ``},
		{name: "Method70", src: `{{ {'a':1}.values() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `dict_values([1])`, err: ``},
		{name: "Method71", src: `{{ {'a':1}.items() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `dict_items([('a', 1)])`, err: ``},
		{name: "Method72", src: `{{ {'a':1}.keys()|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `['a']`, err: ``},
		{name: "Method73", src: `{{ {'a': 1}.items()|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[('a', 1)]`, err: ``},
		{name: "Method74", src: `{{ 'a' in {'a': 1}.keys() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `True`, err: ``},
		{name: "Method75", src: `{{ ('a', 1) in {'a': 1}.items() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `True`, err: ``},
		{name: "Method76", src: `{{ 1 in {'a': 1}.values() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `True`, err: ``},
		{name: "Method77", src: `{{ {'a':1}.keys() == {'a':1}.keys() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `True`, err: ``},
		{name: "Method78", src: `{{ {'a':1}.keys() == ['a'] }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `False`, err: ``},
		{name: "Method79", src: `{{ {'a':1}.items() == {'a':1}.items() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `True`, err: ``},
		{name: "Method80", src: `{{ {'a':1}.values() == {'a':1}.values() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `False`, err: ``},
		{name: "Method81", src: `{{ {'a':1}.keys()|length }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1`, err: ``},
		{name: "Method82", src: `{{ dict }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<class 'dict'>`, err: ``},
		{name: "Method83", src: `{{ dict(1, 2) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `dict expected at most 1 argument, got 2`},
		{name: "Method84", src: `{{ dict([('a', 1), ('b', 2)]) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `{'a': 1, 'b': 2}`, err: ``},
		{name: "Method85", src: `{{ dict(a=1)|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `['a']`, err: ``},
		{name: "Method86", src: `{{ {'b': 1, 'a': 2}.keys()|sort }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `['a', 'b']`, err: ``},
		{name: "Method87", src: `{{ [3, 1, 2]|sort }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[1, 2, 3]`, err: ``},
		{name: "Method88", src: `{{ [(1, 2)]|unique|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[(1, 2)]`, err: ``},
		{name: "Method89", src: `{{ [1, 1.0, true]|unique|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[1]`, err: ``},
		{name: "Method90", src: `{{ {(1, 2): 3}[(1, 2)] }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `3`, err: ``},
		{name: "Method91", src: `{{ {1: 'a', 1.0: 'b', true: 'c'} }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `{1: 'c'}`, err: ``},
		{name: "Method92", src: `{{ {[1]: 2} }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `unhashable type: 'list'`},
		{name: "Method93", src: `{{ [1] in {} }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `unhashable type: 'list'`},
		{name: "Method94", src: `{{ {}.get([1]) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `unhashable type: 'list'`},
		{name: "Method95", src: `{{ {1: 2}.get(1.0) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `2`, err: ``},
		{name: "Method96", src: `{{ {'a': 1}['a'|e] }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1`, err: ``},
		{name: "Method97", src: `{{ {'a': 1}.keys()|join(',') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `a`, err: ``},
		{name: "Method98", src: `{{ {'a': 1}.items()|first }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `('a', 1)`, err: ``},
		{name: "Method99", src: `{{ {}.keys()|first }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: ``},
		{name: "Alias1", src: `{% set a = [] %}{% set b = [] %}{% do b.append(a) %}{% do a.append(1) %}{{ b }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[[1]]`, err: ``},
		{name: "Alias2", src: `{% set a = [] %}{% set b = [a] %}{% do a.append(1) %}{{ b }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[[1]]`, err: ``},
		{name: "Alias3", src: `{% set a = [] %}{% set d = {'k': a} %}{% do a.append(1) %}{{ d }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `{'k': [1]}`, err: ``},
		{name: "Alias4", src: `{% set a = [] %}{% set d = {} %}{% do d.update({'k': a}) %}{% do a.append(1) %}{{ d }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `{'k': [1]}`, err: ``},
		{name: "Alias5", src: `{% set a = [] %}{% set d = {} %}{% do d.update(k=a) %}{% do a.append(1) %}{{ d }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `{'k': [1]}`, err: ``},
		{name: "Alias6", src: `{% set a = [] %}{% set d = {} %}{% do d.setdefault('k', a) %}{% do a.append(1) %}{{ d }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `{'k': [1]}`, err: ``},
		{name: "Alias7", src: `{% set a = [] %}{% set b = [] %}{% do b.extend([a]) %}{% do a.append(1) %}{{ b }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[[1]]`, err: ``},
		{name: "Alias8", src: `{% set a = [] %}{% set b = [] %}{% do b.insert(0, a) %}{% do a.append(1) %}{{ b }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[[1]]`, err: ``},
		{name: "Alias9", src: `{% set ns = namespace(l=[]) %}{% set x = ns.l %}{% do x.append(1) %}{{ ns.l }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[1]`, err: ``},
		{name: "Alias10", src: `{% set a = [] %}{% set ns = namespace(l=a) %}{% do a.append(1) %}{{ ns.l }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[1]`, err: ``},
		{name: "Alias11", src: `{% set a = [1] %}{% set b = a %}{% do b.append(2) %}{{ a }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[1, 2]`, err: ``},
		{name: "Alias12", src: `{% set a = [] %}{% set b = [] %}{% do b.append(a) %}{{ b[0] is sameas a }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `True`, err: ``},
		{name: "Alias13", src: `{% set a = {} %}{% set b = [] %}{% do b.append(a) %}{% do a.update(x=1) %}{{ b }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[{'x': 1}]`, err: ``},
		{name: "Alias14", src: `{% set rows = [] %}{% for i in [1, 2] %}{% set row = [] %}{% do rows.append(row) %}{% do row.append(i) %}{% endfor %}{{ rows }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[[1], [2]]`, err: ``},
		{name: "Alias15", src: `{% set a = [] %}{% set t = (a,) %}{% do a.append(1) %}{{ t }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `([1],)`, err: ``},
		{name: "Alias16", src: `{% set a = [] %}{% set b = [] %}{% do b.append(a) %}{% do b[0].append(9) %}{{ a }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[9]`, err: ``},
		{name: "Alias17", src: `{% set a = [3, 1] %}{% set b = [a] %}{% do a.sort() %}{{ b }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[[1, 3]]`, err: ``},
		{name: "Alias18", src: `{% set a = [] %}{% set c = a.copy() %}{% do a.append(1) %}{{ c }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[]`, err: ``},
		{name: "Alias19", src: `{% set a = [] %}{% set b = [] %}{% do b.append(a) %}{% do b.remove(a) %}{{ b }}|{{ [a].index(a) }}|{{ [a, a].count(a) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[]|0|2`, err: ``},
		{name: "KwOrder1", src: `{{ dict(b=1, a=2, c=3) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `{'b': 1, 'a': 2, 'c': 3}`, err: ``},
		{name: "KwOrder2", src: `{{ namespace(b=1, a=2) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<Namespace {'b': 1, 'a': 2}>`, err: ``},
		{name: "KwOrder3", src: `{% macro m() %}{{ kwargs }}{% endmacro %}{{ m(b=1, a=2, c=3) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `{'b': 1, 'a': 2, 'c': 3}`, err: ``},
		{name: "KwOrder4", src: `{% set d = {} %}{% do d.update(b=1, a=2) %}{{ d }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `{'b': 1, 'a': 2}`, err: ``},
		{name: "KwOrder5", src: `{{ {'z': 1}|xmlattr }}{{ dict(z=1, y=2)|xmlattr }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ` z="1" z="1" y="2"`, err: ``},
		{name: "KwOrder6", src: `{{ dict({'z': 1}, b=2, a=3) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `{'z': 1, 'b': 2, 'a': 3}`, err: ``},
		{name: "KwOrder7", src: `{% macro m() %}{{ kwargs }}{% endmacro %}{{ m(**{'b': 1, 'a': 2}) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `{'b': 1, 'a': 2}`, err: ``},
		{name: "KwOrder8", src: `{% macro m() %}{{ kwargs|list }}{% endmacro %}{{ m(y=1, x=2) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `['y', 'x']`, err: ``},
		{name: "Key1", src: `{{ {(1, 2): 'x'} }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `{(1, 2): 'x'}`, err: ``},
		{name: "Key2", src: `{{ {1: 'a', 1.0: 'b', true: 'c'} }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `{1: 'c'}`, err: ``},
		{name: "Key3", src: `{{ {'a': 1}['a'|e] }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1`, err: ``},
		{name: "Key4", src: `{{ {[1]: 2} }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `unhashable type: 'list'`},
		{name: "Key5", src: `{{ {1: 'a'}[1.0] }}|{{ {1: 'a'}[true] }}|{{ 1.0 in {1: 'a'} }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `a|a|True`, err: ``},
		{name: "Key6", src: `{% set d = {} %}{% do d.update({(1,): 'x'}) %}{{ d }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `{(1,): 'x'}`, err: ``},
		{name: "Key7", src: `{{ dict([((1, 2), 'x')]) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `{(1, 2): 'x'}`, err: ``},
		{name: "Key8", src: `{{ [(1, 2), (1, 2)]|unique|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[(1, 2)]`, err: ``},
		{name: "Key9", src: `{{ {1: 2}.get(1.0) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `2`, err: ``},
		{name: "Key10", src: `{{ {(1, 2): 'x'}[(1, 2)] }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `x`, err: ``},
		{name: "HostList1", src: `{% do x.append(3) %}{{ x }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `[1, 2, 3]`, err: ``},
		{name: "HostList2", src: `{{ x.copy() }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `[1, 2]`, err: ``},
		{name: "HostList3", src: `{% set y = x %}{% do y.reverse() %}{{ x }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `[2, 1]`, err: ``},
		{name: "Xmlattr1", src: `{{ {'b': 1, 'a': '<x>', 'c': none, 'd': zz}|xmlattr }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ` b="1" a="&lt;x&gt;"`, err: ``},
		{name: "Xmlattr2", src: `{{ {'a b': 1}|xmlattr }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `Invalid character in attribute name: 'a b'`},
		{name: "Xmlattr3", src: `{{ {'a/b': 1}|xmlattr }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `Invalid character in attribute name: 'a/b'`},
		{name: "Xmlattr4", src: `{{ {'a>b': 1}|xmlattr }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `Invalid character in attribute name: 'a>b'`},
		{name: "Xmlattr5", src: `{{ {'<k>': 1}|xmlattr }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `Invalid character in attribute name: '<k>'`},
		{name: "Xmlattr6", src: `{{ {1: 2}|xmlattr }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `expected string or bytes-like object, got 'int'`},
		{name: "Xmlattr7", src: `{{ {'a': 1}|xmlattr(false) }}|{{ {'a': 1}|xmlattr(autospace=false) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `a="1"|a="1"`, err: ``},
		{name: "Xmlattr8", src: `{{ {}|xmlattr }}|{{ {'a': none}|xmlattr }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `|`, err: ``},
		{name: "Xmlattr9", src: `{{ [1]|xmlattr }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `'list' object has no attribute 'items'`},
		{name: "Xmlattr10", src: `{{ {'a': 'x'|safe}|xmlattr }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ` a="x"`, err: ``},
		{name: "Xmlattr11", src: `{{ {'a': [1, '<']}|xmlattr }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ` a="[1, &#39;&lt;&#39;]"`, err: ``},
		{name: "Cycle1", src: `{% set d = {'a': 1} %}{% do d.update({'self': d}) %}{{ d }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `{'a': 1, 'self': {...}}`, err: ``},
		{name: "Cycle2", src: `{% set d = {'a': 1} %}{% do d.update({'self': d}) %}{{ d|tojson }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `Circular reference detected`},
		{name: "Cycle3", src: `{% set d = {'a': 1} %}{% do d.update({'self': d}) %}{{ d == d }}|{{ d in [d] }}|{{ d|length }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `True|True|2`, err: ``},
		{name: "Cycle4", src: `{% set d = {'a': 1} %}{% do d.update({'self': d}) %}{{ '%s' % (d,) }}|{{ '{}'.format(d) }}|{{ d|string }}|{{ d ~ '' }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `{'a': 1, 'self': {...}}|{'a': 1, 'self': {...}}|{'a': 1, 'self': {...}}|{'a': 1, 'self': {...}}`, err: ``},
		{name: "Cycle5", src: `{% set d = {'a': 1} %}{% do d.update({'l': [d]}) %}{{ d }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `{'a': 1, 'l': [{...}]}`, err: ``},
		{name: "Cycle6", src: `{% set ns = namespace(a=1) %}{% set ns.self = ns %}{{ ns }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<Namespace {'a': 1, 'self': <Namespace {...}>}>`, err: ``},
		{name: "Cycle7", src: `{% set ns = namespace(a=1) %}{% set ns.l = [ns] %}{{ ns }}|{{ ns.l }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<Namespace {'a': 1, 'l': [<Namespace {...}>]}>|[<Namespace {'a': 1, 'l': [...]}>]`, err: ``},
		{name: "Cycle8", src: `{% set ns = namespace(a=1) %}{% set ns.self = ns %}{{ ns == ns }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `True`, err: ``},
		{name: "Cycle9", src: `{% set d = {} %}{% do d.update({'d': d}) %}{{ d.d.d.d }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `{'d': {...}}`, err: ``},
		{name: "Cycle10", src: `{{ namespace(a=[1, 'x'], b={'k': none}) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `<Namespace {'a': [1, 'x'], 'b': {'k': None}}>`, err: ``},
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
