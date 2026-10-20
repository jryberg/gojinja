package environment

import (
	"testing"

	"github.com/jryberg/gojinja/pkg/varsutil"
)

// Expected values come from rendering the same templates with the Jinja2
// pinned in tools/parity/requirements.txt (that Python's pprint.pformat).
func TestPprintIsPformat(t *testing.T) {
	od, err := varsutil.JSONVars([]byte(ppVars))
	if err != nil {
		t.Fatal(err)
	}
	vars := map[string]any{}
	for _, k := range od.Keys() {
		vars[k.(string)], _ = od.Get(k)
	}
	cases := []struct{ src, want string }{
		{`{{ {'b': 1, 'a': [1, 2]}|pprint }}`, "{'a': [1, 2], 'b': 1}"},
		{`{{ 'x'|pprint }}|{{ 1|pprint }}|{{ 1.5|pprint }}|{{ none|pprint }}|{{ true|pprint }}|{{ (1,)|pprint }}|{{ ()|pprint }}|{{ []|pprint }}|{{ {}|pprint }}`, "'x'|1|1.5|None|True|(1,)|()|[]|{}"},
		{`{{ range(40)|list|pprint }}`, "[0,\n 1,\n 2,\n 3,\n 4,\n 5,\n 6,\n 7,\n 8,\n 9,\n 10,\n 11,\n 12,\n 13,\n 14,\n 15,\n 16,\n 17,\n 18,\n 19,\n 20,\n 21,\n 22,\n 23,\n 24,\n 25,\n 26,\n 27,\n 28,\n 29,\n 30,\n 31,\n 32,\n 33,\n 34,\n 35,\n 36,\n 37,\n 38,\n 39]"},
		{`{{ big|pprint }}`, "{'name00': {'id': 0, 'ok': True, 'tags': []},\n 'name01': {'id': 1, 'ok': False, 'tags': ['t0']},\n 'name02': {'id': 2, 'ok': True, 'tags': ['t0', 't1']},\n 'name03': {'id': 3, 'ok': False, 'tags': ['t0', 't1', 't2']},\n 'name04': {'id': 4, 'ok': True, 'tags': []},\n 'name05': {'id': 5, 'ok': False, 'tags': ['t0']},\n 'name06': {'id': 6, 'ok': True, 'tags': ['t0', 't1']},\n 'name07': {'id': 7, 'ok': False, 'tags': ['t0', 't1', 't2']}}"},
		{`{{ nested|pprint }}`, "{'a': None,\n 'z': [{'a': 1,\n        'b': [1,\n              2,\n              3,\n              1,\n              2,\n              3,\n              1,\n              2,\n              3,\n              1,\n              2,\n              3,\n              1,\n              2,\n              3,\n              1,\n              2,\n              3,\n              1,\n              2,\n              3,\n              1,\n              2,\n              3]},\n       ['xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',\n        'yyyyyyyyyyyyyyyyyyyyyyyyyyyyyy',\n        'zzzzzzzzzzzzzzzzzzzzzzzzzzzzzz']]}"},
		{`{{ long|pprint }}`, "('The quick brown fox jumps over the lazy dog. The quick brown fox jumps over '\n 'the lazy dog. The quick brown fox jumps over the lazy dog. The quick brown '\n 'fox jumps over the lazy dog. The quick brown fox jumps over the lazy dog. ')"},
		{`{{ [long]|pprint }}`, "['The quick brown fox jumps over the lazy dog. The quick brown fox jumps over '\n 'the lazy dog. The quick brown fox jumps over the lazy dog. The quick brown '\n 'fox jumps over the lazy dog. The quick brown fox jumps over the lazy dog. ']"},
		{`{{ {'k': long}|pprint }}`, "{'k': 'The quick brown fox jumps over the lazy dog. The quick brown fox jumps '\n      'over the lazy dog. The quick brown fox jumps over the lazy dog. The '\n      'quick brown fox jumps over the lazy dog. The quick brown fox jumps over '\n      'the lazy dog. '}"},
		{`{{ ('a' * 100)|pprint }}`, "'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'"},
		{`{{ ['a' * 100, 'b' * 30]|pprint }}`, "['aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',\n 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbb']"},
		{`{{ {1: 'a', 'b': 2, 0.5: 'c', none: 4}|pprint }}`, "{None: 4, 0.5: 'c', 1: 'a', 'b': 2}"},
		{`{{ ('é' * 50 + ' ' + 'ü' * 40)|pprint }}`, "('\u00e9\u00e9\u00e9\u00e9\u00e9\u00e9\u00e9\u00e9\u00e9\u00e9\u00e9\u00e9\u00e9\u00e9\u00e9\u00e9\u00e9\u00e9\u00e9\u00e9\u00e9\u00e9\u00e9\u00e9\u00e9\u00e9\u00e9\u00e9\u00e9\u00e9\u00e9\u00e9\u00e9\u00e9\u00e9\u00e9\u00e9\u00e9\u00e9\u00e9\u00e9\u00e9\u00e9\u00e9\u00e9\u00e9\u00e9\u00e9\u00e9\u00e9 '\n '\u00fc\u00fc\u00fc\u00fc\u00fc\u00fc\u00fc\u00fc\u00fc\u00fc\u00fc\u00fc\u00fc\u00fc\u00fc\u00fc\u00fc\u00fc\u00fc\u00fc\u00fc\u00fc\u00fc\u00fc\u00fc\u00fc\u00fc\u00fc\u00fc\u00fc\u00fc\u00fc\u00fc\u00fc\u00fc\u00fc\u00fc\u00fc\u00fc\u00fc')"},
		{`{{ ('line one\nline two is a bit longer than the others in this text\n' * 3)|pprint }}`, "('line one\\n'\n 'line two is a bit longer than the others in this text\\n'\n 'line one\\n'\n 'line two is a bit longer than the others in this text\\n'\n 'line one\\n'\n 'line two is a bit longer than the others in this text\\n')"},
		{`{{ ('<b>' ~ 'x' * 90)|e|pprint }}`, "Markup('&lt;b&gt;xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx')"},
		{`{{ q|pprint }}`, "Undefined"},
		{`{{ [(1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25)]|pprint }}`, "[(1,\n  2,\n  3,\n  4,\n  5,\n  6,\n  7,\n  8,\n  9,\n  10,\n  11,\n  12,\n  13,\n  14,\n  15,\n  16,\n  17,\n  18,\n  19,\n  20,\n  21,\n  22,\n  23,\n  24,\n  25)]"},
		{`{{ (('a' * 75),)|pprint }}`, "('aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',)"},
	}
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	for _, c := range cases {
		if got := render(t, e, c.src, vars); got != c.want {
			t.Errorf("%s =\n%s\nwant\n%s", c.src, got, c.want)
		}
	}
}

const ppVars = `{"big": {"name00": {"id": 0, "tags": [], "ok": true}, "name01": {"id": 1, "tags": ["t0"], "ok": false}, "name02": {"id": 2, "tags": ["t0", "t1"], "ok": true}, "name03": {"id": 3, "tags": ["t0", "t1", "t2"], "ok": false}, "name04": {"id": 4, "tags": [], "ok": true}, "name05": {"id": 5, "tags": ["t0"], "ok": false}, "name06": {"id": 6, "tags": ["t0", "t1"], "ok": true}, "name07": {"id": 7, "tags": ["t0", "t1", "t2"], "ok": false}}, "nested": {"z": [{"a": 1, "b": [1, 2, 3, 1, 2, 3, 1, 2, 3, 1, 2, 3, 1, 2, 3, 1, 2, 3, 1, 2, 3, 1, 2, 3]}, ["xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", "yyyyyyyyyyyyyyyyyyyyyyyyyyyyyy", "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"]], "a": null}, "long": "The quick brown fox jumps over the lazy dog. The quick brown fox jumps over the lazy dog. The quick brown fox jumps over the lazy dog. The quick brown fox jumps over the lazy dog. The quick brown fox jumps over the lazy dog. "}`
