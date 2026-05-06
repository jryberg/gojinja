package ast

import (
	"fmt"
	"reflect"
	"strings"
)

// Dump produces a human-readable representation of the AST useful for
// tests. Mirrors jinja2.nodes.Node.dump in shape: `Name(name='foo', ctx='load')`.
// Lists are written `[a, b]`, nil values as `None`.
func Dump(n Node) string {
	var b strings.Builder
	dumpNode(&b, n)
	return b.String()
}

func dumpNode(b *strings.Builder, n Node) {
	if n == nil {
		b.WriteString("None")
		return
	}
	v := reflect.ValueOf(n)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			b.WriteString("None")
			return
		}
		v = v.Elem()
	}
	t := v.Type()
	b.WriteString(t.Name())
	b.WriteByte('(')
	first := true
	dumpFields(b, v, t, &first)
	b.WriteByte(')')
}

// dumpFields walks v's fields, recursing into anonymous embedded data
// structs (binexprBase, unaryexprBase) but skipping pure-infrastructure
// embeds (posBase, exprBase, stmtBase, helperBase, literalBase).
func dumpFields(b *strings.Builder, v reflect.Value, t reflect.Type, first *bool) {
	for i := 0; i < v.NumField(); i++ {
		f := t.Field(i)
		if f.Anonymous {
			if isInfrastructureType(f.Type) {
				continue
			}
			fv := v.Field(i)
			ft := f.Type
			if ft.Kind() == reflect.Ptr {
				if fv.IsNil() {
					continue
				}
				fv = fv.Elem()
				ft = ft.Elem()
			}
			dumpFields(b, fv, ft, first)
			continue
		}
		if !f.IsExported() {
			continue
		}
		if !*first {
			b.WriteString(", ")
		}
		*first = false
		b.WriteString(toLowerInitial(f.Name))
		b.WriteByte('=')
		dumpValue(b, v.Field(i))
	}
}

// isInfrastructureType is true for the embed-only base structs that carry
// position / interface markers but no user-visible AST data.
func isInfrastructureType(t reflect.Type) bool {
	switch t.Name() {
	case "posBase", "exprBase", "stmtBase", "helperBase", "literalBase":
		return true
	}
	return false
}

func dumpValue(b *strings.Builder, v reflect.Value) {
	switch v.Kind() {
	case reflect.Ptr, reflect.Interface:
		if v.IsNil() {
			b.WriteString("None")
			return
		}
		if n, ok := v.Interface().(Node); ok {
			dumpNode(b, n)
			return
		}
		dumpValue(b, v.Elem())
	case reflect.Slice:
		b.WriteByte('[')
		for i := 0; i < v.Len(); i++ {
			if i > 0 {
				b.WriteString(", ")
			}
			dumpValue(b, v.Index(i))
		}
		b.WriteByte(']')
	case reflect.String:
		fmt.Fprintf(b, "%q", v.String())
	case reflect.Bool:
		if v.Bool() {
			b.WriteString("True")
		} else {
			b.WriteString("False")
		}
	default:
		if v.CanInterface() {
			vi := v.Interface()
			if n, ok := vi.(Node); ok {
				dumpNode(b, n)
				return
			}
			fmt.Fprintf(b, "%v", vi)
			return
		}
		b.WriteString("?")
	}
}

// toLowerInitial converts CamelCase to lowercase initial only — `Lineno`
// stays `lineno`, `WithContext` becomes `withContext`. We don't fully
// snake_case (parity with Python field names is via the parser's choice
// of struct field names, which we keep camelCase here).
func toLowerInitial(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	if r[0] >= 'A' && r[0] <= 'Z' {
		r[0] = r[0] + 32
	}
	return string(r)
}
