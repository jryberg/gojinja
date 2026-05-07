package environment

import (
	"context"
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

// Caller-supplied []any must REJECT mutation — the sandbox boundary is
// "values created inside the template can be mutated; host data cannot".
// The error is the new Undefined-aware message, naming the missing
// attribute rather than the bare Go type.
func TestCallerSuppliedSliceIsImmutable(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	tpl, err := e.FromString(`{% do hostList.append("nope") %}`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, err = tpl.RenderContext(context.Background(), map[string]any{
		"hostList": []any{"a", "b"},
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "append") {
		t.Fatalf("error should name the missing attribute %q, got: %v", "append", msg)
	}
	if strings.Contains(msg, "runtime.Undefined") {
		t.Fatalf("error should not leak the bare Go type name; got: %v", msg)
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
