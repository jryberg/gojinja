package environment

import (
	"context"
	"github.com/jryberg/gojinja/pkg/varsutil"
	"strings"
	"testing"
)

// =============================================================== PyList methods
//
// These tests cover the in-template list mutation methods (`append`,
// `extend`, `insert`, `pop`, `remove`, `clear`, `reverse`, `sort`)
// reached via {% set xs = [] %}{% do xs.method(...) %}. The byte-equal
// parity harness (tools/parity) covers Python conformance for the same
// patterns; these unit tests assert the Go-side behaviour in isolation
// so a regression here surfaces without requiring a Python environment.

func TestPyListAppendInDoExtension(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	src := `{%- set xs = [] -%}{% for i in items -%}{%- do xs.append(i) -%}{% endfor %}{{ xs|join(',') }}`
	got := render(t, e, src, map[string]any{"items": []any{"a", "b", "c"}})
	if got != "a,b,c" {
		t.Fatalf("append: got %q", got)
	}
}

func TestPyListExtend(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	src := `{%- set xs = [1, 2] -%}{%- do xs.extend([3, 4, 5]) -%}{{ xs|join(',') }}`
	if got := render(t, e, src, nil); got != "1,2,3,4,5" {
		t.Fatalf("extend: got %q", got)
	}
}

func TestPyListInsertNegativeIndex(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	src := `{%- set xs = ['a', 'd'] -%}{%- do xs.insert(1, 'b') -%}{%- do xs.insert(-1, 'c') -%}{{ xs|join(',') }}`
	if got := render(t, e, src, nil); got != "a,b,c,d" {
		t.Fatalf("insert: got %q", got)
	}
}

func TestPyListPopDefaultAndIndexed(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	src := `{%- set xs = ['a', 'b', 'c'] -%}{%- set tail = xs.pop() -%}{%- set head = xs.pop(0) -%}{{ head }}|{{ tail }}|{{ xs|join(',') }}`
	if got := render(t, e, src, nil); got != "a|c|b" {
		t.Fatalf("pop: got %q", got)
	}
}

func TestPyListRemoveFirstMatch(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	src := `{%- set xs = ['a', 'b', 'a', 'c'] -%}{%- do xs.remove('a') -%}{{ xs|join(',') }}`
	if got := render(t, e, src, nil); got != "b,a,c" {
		t.Fatalf("remove: got %q", got)
	}
}

func TestPyListReverseAndSort(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	src := `{%- set xs = [3, 1, 2] -%}{%- do xs.reverse() -%}{{ xs|join(',') }}|{%- do xs.sort() -%}{{ xs|join(',') }}|{%- do xs.sort(reverse=true) -%}{{ xs|join(',') }}`
	if got := render(t, e, src, nil); got != "2,1,3|1,2,3|3,2,1" {
		t.Fatalf("reverse/sort: got %q", got)
	}
}

func TestPyListClear(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	src := `{%- set xs = [1, 2, 3] -%}{%- do xs.clear() -%}{{ xs|length }}`
	if got := render(t, e, src, nil); got != "0" {
		t.Fatalf("clear: got %q", got)
	}
}

// A caller-supplied []any is a Python list in the template, which
// Jinja2's sandbox lets the template mutate; the template works on a
// copy, so the caller's slice is left untouched.
func TestCallerSuppliedSliceIsCopied(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	tpl, err := e.FromString(`{% do hostList.append("c") %}{{ hostList }}`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	host := []any{"a", "b"}
	out, err := tpl.RenderContext(context.Background(), map[string]any{"hostList": host})
	if err != nil || out != "['a', 'b', 'c']" {
		t.Fatalf("got %q, %v; want ['a', 'b', 'c']", out, err)
	}
	if len(host) != 2 {
		t.Fatalf("caller's slice was mutated: %v", host)
	}
}

// =============================================================== OrderedDict methods

func TestOrderedDictUpdate(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	src := `{%- set d = {'a': 1, 'b': 2} -%}{%- do d.update({'b': 20, 'c': 3}) -%}` +
		`{%- for k in d.keys()|sort -%}{{ k }}={{ d[k] }},{%- endfor -%}`
	if got := render(t, e, src, nil); got != "a=1,b=20,c=3," {
		t.Fatalf("update: got %q", got)
	}
}

func TestOrderedDictPopWithDefault(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	src := `{%- set d = {'a': 1} -%}{%- set v = d.pop('missing', 'fallback') -%}{{ v }}|{{ d|length }}`
	if got := render(t, e, src, nil); got != "fallback|1" {
		t.Fatalf("pop default: got %q", got)
	}
}

func TestOrderedDictPopMissingNoDefault(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	tpl, err := e.FromString(`{%- set d = {'a': 1} -%}{%- set v = d.pop('missing') -%}{{ v }}`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, err := tpl.RenderContext(context.Background(), nil); err == nil {
		t.Fatalf("expected KeyError, got nil")
	}
}

func TestOrderedDictSetdefault(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	src := `{%- set d = {'x': 1} -%}{%- set xv = d.setdefault('x', 99) -%}{%- set yv = d.setdefault('y', 42) -%}` +
		`{{ xv }}|{{ yv }}|{{ d['x'] }}|{{ d['y'] }}`
	if got := render(t, e, src, nil); got != "1|42|1|42" {
		t.Fatalf("setdefault: got %q", got)
	}
}

func TestOrderedDictCopy(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	// Mutating the copy must not affect the original; insertion order
	// is preserved through the copy.
	src := `{%- set d = {'b': 2, 'a': 1} -%}` +
		`{%- set c = d.copy() -%}` +
		`{%- do c.update({'c': 3}) -%}` +
		`d=` +
		`{%- for k in d.keys() -%}{{ k }}{%- endfor -%}` +
		`|c=` +
		`{%- for k in c.keys() -%}{{ k }}{%- endfor -%}`
	if got := render(t, e, src, nil); got != "d=ba|c=bac" {
		t.Fatalf("copy: got %q", got)
	}
}

func TestDictCopyOnCallerMap(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	// copy() on a caller-supplied map returns a working dict the
	// template can subscript and iterate.
	src := `{%- set c = d.copy() -%}{{ c['name'] }}|{{ c|length }}`
	got := render(t, e, src, map[string]any{"d": map[string]any{"name": "Alice", "age": int64(30)}})
	if got != "Alice|2" {
		t.Fatalf("copy on caller map: got %q", got)
	}
}

func TestOrderedDictPopitem(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	// LIFO: 'c' popped first, then 'b'. Remaining contains 'a' only.
	src := `{%- set d = {'a': 1, 'b': 2, 'c': 3} -%}` +
		`{{ d.popitem() }}|{{ d.popitem() }}|len={{ d|length }}|left=` +
		`{%- for k, v in d.items() -%}{{ k }}={{ v }}{%- endfor -%}`
	if got := render(t, e, src, nil); got != "('c', 3)|('b', 2)|len=1|left=a=1" {
		t.Fatalf("popitem: got %q", got)
	}
}

func TestOrderedDictPopitemEmpty(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	tpl, err := e.FromString(`{%- set d = {} -%}{{ d.popitem() }}`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, err := tpl.RenderContext(context.Background(), nil); err == nil {
		t.Fatalf("expected error on popitem of empty dict, got nil")
	}
}

func TestOrderedDictClear(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	src := `{%- set d = {'a': 1, 'b': 2} -%}{%- do d.clear() -%}{{ d|length }}`
	if got := render(t, e, src, nil); got != "0" {
		t.Fatalf("clear: got %q", got)
	}
}

// =============================================================== Undefined-as-callee error

// When a template calls something that resolves to runtime.Undefined,
// the old error was "value of type runtime.Undefined is not callable" —
// useless without the original template + binding context. The new
// message names the missing identifier or attribute path so logs are
// debuggable without reproducing the render.
func TestUndefinedNameAsCalleeProducesDescriptiveError(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	tpl, err := e.FromString(`{{ ghost() }}`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, err = tpl.RenderContext(context.Background(), nil)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "ghost") {
		t.Fatalf("error should name the missing identifier %q, got: %v", "ghost", msg)
	}
	if strings.Contains(msg, "runtime.Undefined") {
		t.Fatalf("error must not surface the bare Go type, got: %v", msg)
	}
}

func TestUndefinedAttrAsCalleeProducesDescriptiveError(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	tpl, err := e.FromString(`{{ obj.missing_method() }}`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, err = tpl.RenderContext(context.Background(), map[string]any{
		"obj": map[string]any{"present": "y"},
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "missing_method") {
		t.Fatalf("error should name the missing attribute, got: %v", msg)
	}
}

// A template mutating host data works on a copy: the caller's values and
// later renders with the same variables are unaffected.
func TestHostDataIsolation(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	tpl, err := e.FromString(`{% do l.append(3) %}{% do d.update(b=2) %}{% do d.a.append(9) %}{{ l }}|{{ d }}`)
	if err != nil {
		t.Fatal(err)
	}
	ordered, err := varsutil.JSONVars([]byte(`{"l": [1, 2], "d": {"a": [1]}}`))
	if err != nil {
		t.Fatal(err)
	}
	plain := map[string]any{"l": []any{1, 2}, "d": map[string]any{"a": []any{1}}}
	for _, vars := range []any{ordered, plain} {
		for i := 0; i < 2; i++ {
			out, err := tpl.RenderContext(context.Background(), vars)
			if err != nil || out != "[1, 2, 3]|{'a': [1, 9], 'b': 2}" {
				t.Fatalf("render %d with %T: got %q, %v", i, vars, out, err)
			}
		}
	}
	if l := plain["l"].([]any); len(l) != 2 {
		t.Errorf("caller's list was mutated: %v", l)
	}
	if a := plain["d"].(map[string]any)["a"].([]any); len(a) != 1 {
		t.Errorf("caller's nested list was mutated: %v", a)
	}
}
