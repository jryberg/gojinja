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

// TestGenerators holds the generators that filters return, the reverse iterators, lazy for loops and slicing checked against Jinja2. Expected values come from the Jinja2 pinned in
// tools/parity/requirements.txt; err is the expected error's message, or
// "*" when only an error is expected. Object addresses in reprs are
// masked.
func TestGenerators(t *testing.T) {
	cases := []struct {
		name, src, get string
		tpls           map[string]string
		vars           string
		trim           bool
		want, err      string
	}{
		{name: "Generator1", src: `{{ [1]|map('string') }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `<generator object sync_do_map at 0x…>`, err: ``},
		{name: "Generator2", src: `{{ [1]|select }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `<generator object select_or_reject at 0x…>`, err: ``},
		{name: "Generator3", src: `{{ [1]|reject }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `<generator object select_or_reject at 0x…>`, err: ``},
		{name: "Generator4", src: `{{ [{'a':1}]|selectattr('a') }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `<generator object select_or_reject at 0x…>`, err: ``},
		{name: "Generator5", src: `{{ [{'a':1}]|rejectattr('a') }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `<generator object select_or_reject at 0x…>`, err: ``},
		{name: "Generator6", src: `{{ [1]|unique }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `<generator object sync_do_unique at 0x…>`, err: ``},
		{name: "Generator7", src: `{{ [1]|batch(1) }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `<generator object do_batch at 0x…>`, err: ``},
		{name: "Generator8", src: `{{ [1]|slice(1) }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `<generator object sync_do_slice at 0x…>`, err: ``},
		{name: "Generator9", src: `{{ {'a':1}|items }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `<generator object do_items at 0x…>`, err: ``},
		{name: "Generator10", src: `{{ [1,2]|reverse }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `<list_reverseiterator object at 0x…>`, err: ``},
		{name: "Generator11", src: `{{ (1,2)|reverse }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `<reversed object at 0x…>`, err: ``},
		{name: "Generator12", src: `{{ {'a':1}|reverse }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `<dict_reversekeyiterator object at 0x…>`, err: ``},
		{name: "Generator13", src: `{{ range(3)|reverse }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `<range_iterator object at 0x…>`, err: ``},
		{name: "Generator14", src: `{{ {'a':1}.items()|reverse }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `<dict_reverseitemiterator object at 0x…>`, err: ``},
		{name: "Generator15", src: `{{ {'a':1}.keys()|reverse }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `<dict_reversekeyiterator object at 0x…>`, err: ``},
		{name: "Generator16", src: `{{ {'a':1}.values()|reverse }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `<dict_reversevalueiterator object at 0x…>`, err: ``},
		{name: "Generator17", src: `{{ 'abc'|reverse }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `cba`, err: ``},
		{name: "Generator18", src: `{{ x|reverse }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `<list_reverseiterator object at 0x…>`, err: ``},
		{name: "Generator19", src: `{{ ([1]|map('string'))|reverse }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `['1']`, err: ``},
		{name: "Generator20", src: `{{ ('ab'|e)|reverse }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `ba`, err: ``},
		{name: "Generator21", src: `{{ none|reverse }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `argument must be iterable`},
		{name: "Generator22", src: `{{ 5|reverse }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `argument must be iterable`},
		{name: "Generator23", src: `{{ []|select|length }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `object of type 'generator' has no len()`},
		{name: "Generator24", src: `{{ ([]|select) is true }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `False`, err: ``},
		{name: "Generator25", src: `{{ 'y' if []|select else 'n' }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `y`, err: ``},
		{name: "Generator26", src: `{{ ([1,2]|select)|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `[1, 2]`, err: ``},
		{name: "Generator27", src: `{{ [1]|map('string') is iterable }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `True`, err: ``},
		{name: "Generator28", src: `{{ [1]|map('string') is sequence }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `False`, err: ``},
		{name: "Generator29", src: `{{ [1]|map('string') is mapping }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `False`, err: ``},
		{name: "Generator30", src: `{{ ([1,2]|select)|first }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `1`, err: ``},
		{name: "Generator31", src: `{{ ([]|select)|first }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: ``},
		{name: "Generator32", src: `{{ ([1,2]|select)|sum }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `3`, err: ``},
		{name: "Generator33", src: `{{ ([1,2]|select)|max }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `2`, err: ``},
		{name: "Generator34", src: `{{ ([3,1]|select)|sort }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `[1, 3]`, err: ``},
		{name: "Generator35", src: `{{ ([1,2]|select)|join(',') }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `1,2`, err: ``},
		{name: "Generator36", src: `{{ ([1,2]|select)|reverse }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `[2, 1]`, err: ``},
		{name: "Generator37", src: `{{ ([1,2]|select)|last }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `'generator' object is not reversible`},
		{name: "Generator38", src: `{{ ([1,2]|reverse)|last }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `'list_reverseiterator' object is not reversible`},
		{name: "Generator39", src: `{{ ([1,2]|select)|random }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `object of type 'generator' has no len()`},
		{name: "Generator40", src: `{{ ([1,2]|select)|count }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `object of type 'generator' has no len()`},
		{name: "Generator41", src: `{{ ([1,2]|select)|tojson }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `Object of type generator is not JSON serializable`},
		{name: "Generator42", src: `{{ ([1,2]|reverse)|tojson }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `Object of type list_reverseiterator is not JSON serializable`},
		{name: "Generator43", src: `{{ ([1,2]|select)[0] }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: ``},
		{name: "Generator44", src: `{{ ([1,2]|select).bogus }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: ``},
		{name: "Generator45", src: `{{ ([1,2]|select).gi_running }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `False`, err: ``},
		{name: "Generator46", src: `{{ ([1,2]|select).gi_suspended }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `False`, err: ``},
		{name: "Generator47", src: `{{ ([1,2]|select).gi_frame }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: ``},
		{name: "Generator48", src: `{{ ([1,2]|select).send(none) }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `1`, err: ``},
		{name: "Generator49", src: `{{ ([1,2]|select).close() }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `None`, err: ``},
		{name: "Generator50", src: `{{ ([1,2]|select).gi_yieldfrom }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `None`, err: ``},
		{name: "Generator51", src: `{{ ([1,2]|select)[1:] }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `'generator' object is not subscriptable`},
		{name: "Generator52", src: `{{ ([1,2]|select).send(1) }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `can't send non-None value to a just-started generator`},
		{name: "Generator53", src: `{{ ([1,2]|select).throw(1) }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `exceptions must be classes or instances deriving from BaseException, not int`},
		{name: "Generator54", src: `{{ ([1,2]|select) == ([1,2]|select) }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `False`, err: ``},
		{name: "Generator55", src: `{{ 2 in ([1,2]|select) }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `True`, err: ``},
		{name: "Generator56", src: `{{ {([1]|select): 1}|length }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `1`, err: ``},
		{name: "Generator57", src: `{{ x|items|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `Can only get item pairs from a mapping.`},
		{name: "Generator58", src: `{{ ([1,2]|select)|batch(1)|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `[[1], [2]]`, err: ``},
		{name: "Generator59", src: `{{ ([1,2]|select)|slice(2)|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `[[1], [2]]`, err: ``},
		{name: "Generator60", src: `{{ ([1,2]|select)|unique|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `[1, 2]`, err: ``},
		{name: "Generator61", src: `{{ ([1,2]|select)|default(3)|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `[1, 2]`, err: ``},
		{name: "Generator62", src: `{{ ([1,2]|select) < ([1,2]|select) }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `'<' not supported between instances of 'generator' and 'generator'`},
		{name: "Generator63", src: `{{ ([1,2]|select)|dictsort }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `'generator' object has no attribute 'items'`},
		{name: "Generator64", src: `{{ ([1,2]|select)|items|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `Can only get item pairs from a mapping.`},
		{name: "Generator65", src: `{{ [1,2]|items|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `Can only get item pairs from a mapping.`},
		{name: "Generator66", src: `{{ none|items|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `Can only get item pairs from a mapping.`},
		{name: "Generator67", src: `{% set g = [1,2]|items %}ok|{{ [1,2]|items }}|{{ [1]|map() }}|{{ [1]|select(5) }}|{{ x|map('upper')|list }}|{{ none|map('upper')|list }}|{{ [1]|unique(attribute='a.b') }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `ok|<generator object do_items at 0x…>|<generator object sync_do_map at 0x…>|<generator object select_or_reject at 0x…>|['1', '2']|[]|<generator object sync_do_unique at 0x…>`, err: ``},
		{name: "Generator68", src: `{{ ([1]|map())|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `map requires a filter argument`},
		{name: "Generator69", src: `{{ ([1]|select(5))|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `No test named 5.`},
		{name: "Generator70", src: `{{ ([1]|unique(attribute='a.b'))|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `'int object' has no attribute 'a'`},
		{name: "Generator71", src: `{{ ([1,2]|reverse)|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `[2, 1]`, err: ``},
		{name: "Generator72", src: `{{ ((1,2)|reverse)|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `[2, 1]`, err: ``},
		{name: "Generator73", src: `{{ ({'a':1,'b':2}|reverse)|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `['b', 'a']`, err: ``},
		{name: "Generator74", src: `{{ (range(3)|reverse)|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `[2, 1, 0]`, err: ``},
		{name: "Generator75", src: `{{ ({'a':1}.items()|reverse)|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `[('a', 1)]`, err: ``},
		{name: "Generator76", src: `{{ ('ab'.encode('ascii')|reverse)|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `[98, 97]`, err: ``},
		{name: "Generator77", src: `{{ ([1,2]|select)|string|length > 10 }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `True`, err: ``},
		{name: "Generator78", src: `{{ ([1,2]|select) ~ '' }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `<generator object select_or_reject at 0x…>`, err: ``},
		{name: "Generator79", src: `{{ [[1,2]]|select|urlencode }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `1=2`, err: ``},
		{name: "Generator80", src: `{{ ([1]|select)|xmlattr }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `'generator' object has no attribute 'items'`},
		{name: "Generator81", src: `{{ [1,2]|slice(2, 'x')|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `[[1, 'x'], [2, 'x']]`, err: ``},
		{name: "Generator82", src: `{{ [1,2,3]|slice(2, 'x')|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `[[1, 2], [3, 'x']]`, err: ``},
		{name: "Generator83", src: `{{ [1]|slice(-1)|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `[]`, err: ``},
		{name: "Generator84", src: `{{ [1,2,3]|slice(true)|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `[[1, 2, 3]]`, err: ``},
		{name: "Generator85", src: `{{ []|slice(2)|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `[[], []]`, err: ``},
		{name: "Generator86", src: `{{ []|select(5)|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `[]`, err: ``},
		{name: "Generator87", src: `{{ [1,2]|batch(0)|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `[[], [1, 2]]`, err: ``},
		{name: "Generator88", src: `{{ [1,2]|batch('a')|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `[[1, 2]]`, err: ``},
		{name: "Generator89", src: `{{ [1,2]|batch(1.0)|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `[[1], [2]]`, err: ``},
		{name: "Generator90", src: `{{ [1,2,3]|batch(2, 0)|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `[[1, 2], [3, 0]]`, err: ``},
		{name: "Generator91", src: `{{ [1]|batch(3, 'x')|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `[[1, 'x', 'x']]`, err: ``},
		{name: "Generator92", src: `{{ [1]|batch(-1, 'x')|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `[[1]]`, err: ``},
		{name: "Generator93", src: `{{ [1]|slice('a')|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `unsupported operand type(s) for //: 'int' and 'str'`},
		{name: "Generator94", src: `{{ [1]|slice(0)|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `integer division or modulo by zero`},
		{name: "Generator95", src: `{{ [1]|slice(2.0)|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `'float' object cannot be interpreted as an integer`},
		{name: "Generator96", src: `{{ [1]|map(5)|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `No filter named 5.`},
		{name: "Generator97", src: `{{ [1]|select('nope')|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `No test named 'nope'.`},
		{name: "Generator98", src: `{{ [1]|map('nope')|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `No filter named 'nope'.`},
		{name: "Generator99", src: `{{ 5|map('string')|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `'int' object is not iterable`},
		{name: "Generator100", src: `{{ 5|select|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `'int' object is not iterable`},
		{name: "Generator101", src: `{{ 5|unique|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `'int' object is not iterable`},
		{name: "Generator102", src: `{{ 5|batch(2)|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `'int' object is not iterable`},
		{name: "Generator103", src: `{{ 5|slice(2)|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `'int' object is not iterable`},
		{name: "Generator104", src: `{{ (5|items)|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `Can only get item pairs from a mapping.`},
		{name: "Generator105", src: `{{ 0|map('string')|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `[]`, err: ``},
		{name: "Generator106", src: `{{ {'a':1}|map('upper')|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `['A']`, err: ``},
		{name: "Generator107", src: `{{ 'ab'|map('upper')|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `['A', 'B']`, err: ``},
		{name: "Generator108", src: `{% set g = [1,2,3]|select %}{{ g|first }}{{ g|list }}{{ g|list }}|{% set g = [1]|select %}A{{ g.send(none) }}B{{ g.send(none) }}C|{{ g.send(none) is defined }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `1[2, 3][]|A1BC|False`, err: ``},
		{name: "Generator109", src: `{% set g = [1,2]|select %}{{ g|first }}{{ g.gi_suspended }}{{ g.close() }}{{ g|list }}{{ g.gi_suspended }}{{ g.send(1) }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `1TrueNone[]False`, err: ``},
		{name: "Generator110", src: `{% set g = {'a':1,'b':2}|items %}{{ g.gi_yieldfrom }}{{ g|first }}{{ g.gi_yieldfrom is sameas g.gi_yieldfrom }}{{ g.gi_yieldfrom|list }}{{ g|list }}{{ g.gi_yieldfrom }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `None('a', 1)True[('b', 2)][]None`, err: ``},
		{name: "Generator111", src: `{% set g = [1]|select %}{{ g.send() }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `generator.send() takes exactly one argument (0 given)`},
		{name: "Generator112", src: `{% set g = [1]|select %}{{ g.send(1, 2) }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `generator.send() takes exactly one argument (2 given)`},
		{name: "Generator113", src: `{% set g = [1]|select %}{{ g.send(value=1) }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `generator.send() takes no keyword arguments`},
		{name: "Generator114", src: `{% set g = [1]|select %}{{ g.close(1) }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `generator.close() takes no arguments (1 given)`},
		{name: "Generator115", src: `{% set g = [1]|select %}{{ g.throw() }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `throw expected at least 1 argument, got 0`},
		{name: "Generator116", src: `{% set g = [1]|select %}{{ g.throw(1, 2, 3, 4) }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `throw expected at most 3 arguments, got 4`},
		{name: "Generator117", src: `{% set g = [1]|select %}{{ g.throw(none) }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `exceptions must be classes or instances deriving from BaseException, not NoneType`},
		{name: "Generator118", src: `{% set g = [1,2,3,4]|select %}{% for x in g %}{{ x }}{% if x == 2 %}{% break %}{% endif %}{% endfor %}|{{ g|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `12|[3, 4]`, err: ``},
		{name: "Generator119", src: `{% set g = [1,2,3]|select %}{% for x in g %}{{ loop.index }}/{{ loop.length }}/{{ loop.revindex }}{{ loop.last }};{% endfor %}|{{ g|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `1/3/3False;2/3/2False;3/3/1True;|[]`, err: ``},
		{name: "Generator120", src: `{% set g = [1,2,3]|select %}{% for x in g %}{{ x }}{{ loop.nextitem }}{{ loop.last }}{{ loop.previtem }};{% endfor %}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `12False;23False1;3True2;`, err: ``},
		{name: "Generator121", src: `{% for x in []|select %}a{% else %}E{% endfor %}|{% for x in [1]|select if x > 5 %}a{% else %}E{% endfor %}|{% for a, b in {'x':1}|items %}{{ a }}{{ b }}{% endfor %}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `E|E|x1`, err: ``},
		{name: "Generator122", src: `{% set xs = [1,2] %}{% set g = xs|map('string') %}{% do xs.append(3) %}{{ g|list }}|{% set ys = [1,2] %}{% set r = ys|reverse %}{% do ys.append(9) %}{{ r|list }}|{% set zs = [1,2,3] %}{% set r = zs|reverse %}{% do zs.pop() %}{{ r|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `['1', '2', '3']|[2, 1]|[]`, err: ``},
		{name: "Generator123", src: `{% set xs = [1,2] %}{% for x in xs %}{{ x }}{% if loop.index < 3 %}{% do xs.append(x * 10) %}{% endif %}{% endfor %}|{{ xs }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `121020|[1, 2, 10, 20]`, err: ``},
		{name: "Generator124", src: `{% set xs = [1,2,3] %}{% for x in xs %}{{ x }}{{ loop.length }}{% if loop.first %}{% do xs.append(4) %}{% endif %}{% endfor %}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `13233343`, err: ``},
		{name: "Generator125", src: `{% set d = {'a':1} %}{% for k in d %}{% do d.update({'b':2}) %}{% endfor %}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `dictionary changed size during iteration`},
		{name: "Generator126", src: `{% set d = {'a':1, 'b': 2} %}{% for k in d %}{{ k }}{% do d.update({'b':3}) %}{% endfor %}|{% set d = {'a':1} %}{% set g = d|items %}{% do d.update({'a':5}) %}{{ g|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `ab|[('a', 5)]`, err: ``},
		{name: "Generator127", src: `{% set d = {'a':1} %}{% set g = d|items %}{{ g|first }}{% do d.update({'b':5}) %}{{ g|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `dictionary changed size during iteration`},
		{name: "Generator128", src: `{% set ns = namespace(c=0) %}{% for x in [1,2,3,4] if ns.c < 2 %}{% set ns.c = ns.c + 1 %}{{ x }}{% endfor %}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `12`, err: ``},
		{name: "Generator129", src: `{% set ns = namespace(c=0) %}{% for x in [1,2,3,4] if ns.c < 2 %}{% set ns.c = ns.c + 1 %}{{ x }}{{ loop.length }}{% endfor %}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `14243444`, err: ``},
		{name: "Generator130", src: `{% for x in [1,2] %}{{ loop.cycle() }}{% endfor %}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `no items for cycling given`},
		{name: "Generator131", src: `{% for x in [1,2] %}{{ loop|length }}{{ loop.cycle('a','b') }}{% endfor %}|{% for x in [1]|select %}{{ loop }}{% endfor %}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `2a2b|<LoopContext 1/1>`, err: ``},
		{name: "Generator132", src: `{% for x in [[1,[2]],[3]] recursive %}{{ x[0] }}{% if x[1] %}({{ loop(x[1]|map('list')) }}){% endif %}{% endfor %}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `'int' object is not iterable`},
		{name: "Generator133", src: `{% set g = [1,2]|select %}{{ g|join(',') }}|{{ g|join(',') }}|{{ ([3,1]|select)|min }}|{{ ([]|select)|min }}|{{ [1,2]|select|map('string')|join }}|{{ ([1,2,3]|batch(2)|first) }}|{{ ([1,2,3]|slice(2)|first) }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `1,2||1||12|[1, 2]|[1, 2]`, err: ``},
		{name: "Generator134", src: `{% set g = ['b','a']|select %}{{ g|sort }}{{ g|sort }}|{% set g = [1,2]|select %}{{ g|sum }}{{ g|sum }}|{{ ([1,2]|select)|groupby('real')|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `['a', 'b'][]|30|[(1, [1]), (2, [2])]`, err: ``},
		{name: "Generator135", src: `{{ (['1','x']|map('int'))|first }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `1`, err: ``},
		{name: "Generator136", src: `{{ ([{'a':1}, 5]|map(attribute='a'))|first }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `1`, err: ``},
		{name: "Generator137", src: `{% set g = [1,2]|select %}{% set a, b = g %}{{ a }}{{ b }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `12`, err: ``},
		{name: "Generator138", src: `{% set g = [1,2,3]|select %}{% set a, b = g %}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `too many values to unpack (expected 2)`},
		{name: "Generator139", src: `{{ [([1]|select)|list] }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `[[1]]`, err: ``},
		{name: "Generator140", src: `{{ ([1,2]|select) is defined }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `True`, err: ``},
		{name: "Generator141", src: `{{ ([1,2]|select) is none }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `False`, err: ``},
		{name: "Generator142", src: `{{ ([1,2]|select) is sameas ([1,2]|select) }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `False`, err: ``},
		{name: "Generator143", src: `{{ (([1]|select)|pprint)[:20] }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `<generator object se`, err: ``},
		{name: "Generator144", src: `{{ ','.join([1,2]|map('string')) }}|{% set l = [] %}{% do l.extend([1,2]|select) %}{{ l }}|{{ dict({'a':1}|items) }}|{{ (([1,2]|select)|list)|length }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `1,2|[1, 2]|{'a': 1}|2`, err: ``},
		{name: "Generator145", src: `{{ (5)[1:] }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: ``},
		{name: "Generator146", src: `{{ none[1:] }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: ``},
		{name: "Generator147", src: `{{ (5)[0] }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: ``},
		{name: "Generator148", src: `{{ ({"a":1})[1:] }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: ``},
		{name: "Generator149", src: `{{ (5)[1:] ~ "" }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: ``},
		{name: "Generator150", src: `{{ [1,2][x[0]:] }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `[2]`, err: ``},
		{name: "Generator151", src: `{{ "ab"[x[0]:] }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `b`, err: ``},
		{name: "Generator152", src: `{{ ([1,2]|reverse)[1:] }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: ``},
		{name: "Generator153", src: `{{ ([1,2]|reverse)[0] }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: ``},
		{name: "Generator154", src: `{{ ([1,2]|sort)[1:] }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `[2]`, err: ``},
		{name: "Generator155", src: `{{ ('abc'|upper)[1:] }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `BC`, err: ``},
		{name: "Generator156", src: `{{ (1 + 2)[1:] }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: ``},
		{name: "Generator157", src: `{{ ('a' ~ 'b')[1:] }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `b`, err: ``},
		{name: "Generator158", src: `{{ (5 if true else 6)[1:] }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: ``},
		{name: "Generator159", src: `{{ ((5)|string)[1:] }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: ``},
		{name: "Generator160", src: `{{ (none|default('ab'))[1:] }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: ``},
		{name: "Generator161", src: `{{ x[1:] }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `[2]`, err: ``},
		{name: "Generator162", src: `{% set y = 5 %}{{ y[1:] }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `'int' object is not subscriptable`},
		{name: "Generator163", src: `{% set y = none %}{{ y[1:] }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `'NoneType' object is not subscriptable`},
		{name: "Generator164", src: `{% set y = {'a': 1} %}{{ y[1:] }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `slice(1, None, None)`},
		{name: "Generator165", src: `{% set y = 5 %}{{ (y)[1:] if true }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `'int' object is not subscriptable`},
		{name: "Generator166", src: `{{ u[1:] }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `'u' is undefined`},
		{name: "Generator167", src: `{{ ([1]|select)[1:] }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `'generator' object is not subscriptable`},
		{name: "Generator168", src: `{{ ([1]|map('string'))[:1] }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `'generator' object is not subscriptable`},
		{name: "Generator169", src: `{% set y = 2 %}{{ (range(5))[y:] }}|{{ (range(5)|list)[1:] }}|{{ (range(10))[1:5:2] }}|{% set s = 'abc' %}{{ s[1:] }}|{% set t = (1,2) %}{{ t[::-1] }}|{{ (x|string)[1:] }}|{{ ('a'|e)[x[0]-5:] }}|{{ ('<a>'|e)[1:] }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `range(2, 5)|[1, 2, 3, 4]|range(1, 5, 2)|bc|(2, 1)|1, 2]|a|lt;a&gt;`, err: ``},
		{name: "Generator170", src: `{% set y = -5 %}{{ (y|abs)[1:] }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `'int' object is not subscriptable`},
		{name: "Generator171", src: `{% set y = 5 %}{{ y.real[1:] }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `'int' object is not subscriptable`},
		{name: "Generator172", src: `{% set ns = namespace(a=1) %}{{ ns[1:] }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `'Namespace' object is not subscriptable`},
		{name: "Generator173", src: `{% set d = {'a': 1} %}{{ (d.keys())[1:] }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `'dict_keys' object is not subscriptable`},
		{name: "Generator174", src: `{{ ({'a':1}.keys())[1:] }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `'dict_keys' object is not subscriptable`},
		{name: "Generator175", src: `{{ (none)[:] }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: ``},
		{name: "Generator176", src: `{% set d = {'a': 1} %}{{ (d|items)[1:] }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `'generator' object is not subscriptable`},
		{name: "Generator177", src: `{% for i in [1] %}{{ loop[1:] }}{% endfor %}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `'LoopContext' object is not subscriptable`},
		{name: "Generator178", src: `{{ (lipsum)[1:] }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `'function' object is not subscriptable`},
		{name: "Generator179", src: `{{ (cycler(1,2))[1:] }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `'Cycler' object is not subscriptable`},
		{name: "Generator180", src: `{{ x[1:]|list }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: `[2]`, err: ``},
		{name: "Generator181", src: `{{ x.bogus[1:] is defined }}`, get: "", tpls: nil, vars: `{"x": [1, 2]}`, trim: false, want: ``, err: `'list object' has no attribute 'bogus'`},
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
