package escape

import (
	"fmt"
	"math/big"
	"reflect"
	"strings"
)

// MaxDepth bounds how deeply repr, comparisons and serialisation recurse
// into nested containers. CPython stops near 10,000 levels with a
// RecursionError; unbounded, a template-built structure nested millions
// deep would overflow the Go stack.
const MaxDepth = 20000

// RecursionError is what [Repr] and [SoftStr] panic with for a value
// nested deeper than [MaxDepth], as they cannot return an error; the
// evaluator recovers it and fails the render with it.
type RecursionError struct{}

func (RecursionError) Error() string {
	return "maximum recursion depth exceeded while getting the repr of an object"
}

// viewRepr is implemented by dict views, whose repr is dict_keys([...]).
type viewRepr interface {
	ReprView() (string, []any)
}

// attrsRepr is implemented by objects whose repr is `<Name {attrs}>`
// (Jinja2's Namespace), kept without an import on pkg/runtime.
type attrsRepr interface {
	ReprAttrs() (name string, keys []string, values []any)
}

// reprState carries one repr call through nested containers: its depth
// and the containers being printed, which CPython shows as `[...]`,
// `(...)` or `{...}` when they contain themselves.
type reprState struct {
	depth  int
	active map[reprKey]bool
}

type reprKey struct {
	ptr  uintptr
	len  int
	kind reflect.Kind
}

// enter marks container v as being printed, returning false when it
// already is (a cycle) and a func to call when done.
func (st *reprState) enter(v any, n int) (func(), bool) {
	st.depth++
	if st.depth > MaxDepth {
		panic(RecursionError{})
	}
	rv := reflect.ValueOf(v)
	key := reprKey{kind: rv.Kind(), len: n}
	switch rv.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Slice:
		key.ptr = rv.Pointer()
	}
	if key.ptr != 0 {
		if st.active[key] {
			st.depth--
			return nil, false
		}
		if st.active == nil {
			st.active = map[reprKey]bool{}
		}
		st.active[key] = true
	}
	return func() {
		st.depth--
		if key.ptr != 0 {
			delete(st.active, key)
		}
	}, true
}

func (st *reprState) repr(v any) string {
	switch x := v.(type) {
	case nil:
		return "None"
	case bool:
		if x {
			return "True"
		}
		return "False"
	case string:
		return pyStringRepr(x)
	case Markup:
		return "Markup(" + pyStringRepr(string(x)) + ")"
	case undefinedRepr:
		return "Undefined"
	case []byte:
		return pyStringRepr(string(x))
	case float32:
		return pyFloatStr(float64(x))
	case float64:
		return pyFloatStr(x)
	case *big.Int:
		return intRepr(x)
	case complex128:
		return complexRepr(x)
	case []any:
		return st.list(v, x)
	case tupleRepr:
		return st.tuple(v, x.Iter())
	case map[string]any:
		return st.dict(v, stringMapToAny(x))
	case map[any]any:
		return st.dict(v, x)
	case orderedDictRepr:
		if p, ok := x.(proxyRepr); ok {
			if target, ok := p.ProxyTarget(); ok {
				return "mappingproxy(" + st.repr(target) + ")"
			}
		}
		return st.orderedDict(x)
	case viewRepr:
		name, items := x.ReprView()
		return name + "(" + st.list(v, items) + ")"
	case listRepr:
		return st.list(v, x.Items())
	case attrsRepr:
		return st.attrs(x)
	case fmt.Stringer:
		return x.String()
	}
	return fmt.Sprint(v)
}

// list renders Python's `[item, item, ...]`.
func (st *reprState) list(id any, items []any) string {
	leave, ok := st.enter(id, len(items))
	if !ok {
		return "[...]"
	}
	defer leave()
	var b strings.Builder
	b.WriteByte('[')
	for i, it := range items {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(st.repr(it))
	}
	b.WriteByte(']')
	return b.String()
}

// tuple renders a tuple: `(1,)` for a singleton, `(a, b, ...)` otherwise.
func (st *reprState) tuple(id any, items []any) string {
	leave, ok := st.enter(id, len(items))
	if !ok {
		return "(...)"
	}
	defer leave()
	var b strings.Builder
	b.WriteByte('(')
	for i, it := range items {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(st.repr(it))
	}
	if len(items) == 1 {
		b.WriteByte(',')
	}
	b.WriteByte(')')
	return b.String()
}

// orderedDict renders an OrderedDict in insertion order, as Python 3.7+
// dicts iterate.
func (st *reprState) orderedDict(d orderedDictRepr) string {
	keys := d.Keys()
	leave, ok := st.enter(d, -1)
	if !ok {
		return "{...}"
	}
	defer leave()
	var b strings.Builder
	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteString(", ")
		}
		v, _ := d.Get(k)
		b.WriteString(st.repr(k))
		b.WriteString(": ")
		b.WriteString(st.repr(v))
	}
	b.WriteByte('}')
	return b.String()
}

// dict renders a Go map as `{key: value, ...}`. Keys are sorted (Go maps
// have no insertion order) so output is deterministic.
func (st *reprState) dict(id any, m map[any]any) string {
	leave, ok := st.enter(id, -1)
	if !ok {
		return "{...}"
	}
	defer leave()
	keys := make([]any, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sortAnySlice(keys)
	var b strings.Builder
	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(st.repr(k))
		b.WriteString(": ")
		b.WriteString(st.repr(m[k]))
	}
	b.WriteByte('}')
	return b.String()
}

// attrs renders `<Name {'key': value, ...}>`; the attribute dict is the
// part CPython shows as `{...}` on a cycle.
func (st *reprState) attrs(a attrsRepr) string {
	name, keys, values := a.ReprAttrs()
	leave, ok := st.enter(a, -1)
	if !ok {
		return "<" + name + " {...}>"
	}
	defer leave()
	var b strings.Builder
	b.WriteString("<" + name + " {")
	for i, k := range keys {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(pyStringRepr(k))
		b.WriteString(": ")
		b.WriteString(st.repr(values[i]))
	}
	b.WriteString("}>")
	return b.String()
}
