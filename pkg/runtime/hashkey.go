package runtime

import (
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"

	"github.com/jryberg/gojinja/pkg/escape"
)

// HashKey returns the key under which Python's dicts and sets file v:
// two values get the same key exactly when Python treats them as the
// same key — 1, 1.0 and True; a str and a Markup with the same text;
// tuples element by element. Lists, dicts and dict views are unhashable
// and yield Python's TypeError.
func HashKey(v any) (string, error) {
	var b strings.Builder
	if err := writeHashKey(&b, v); err != nil {
		return "", err
	}
	return b.String(), nil
}

func writeHashKey(b *strings.Builder, v any) error {
	switch x := v.(type) {
	case nil, noneType:
		b.WriteString("n")
		return nil
	case bool:
		if x {
			b.WriteString("i1;")
		} else {
			b.WriteString("i0;")
		}
		return nil
	case string:
		writeHashString(b, 's', x)
		return nil
	case escape.Markup:
		writeHashString(b, 's', string(x))
		return nil
	case Bytes:
		writeHashString(b, 'b', string(x))
		return nil
	case float64:
		writeHashFloat(b, x)
		return nil
	case float32:
		writeHashFloat(b, float64(x))
		return nil
	case Tuple:
		fmt.Fprintf(b, "t%d(", len(x))
		for _, e := range x {
			if err := writeHashKey(b, e); err != nil {
				return err
			}
		}
		b.WriteByte(')')
		return nil
	case *Range:
		b.WriteString(x.hashKey())
		return nil
	case Undefined:
		// Every Undefined of a kind compares equal to the others.
		fmt.Fprintf(b, "u%d", x.Mode)
		return nil
	case *PyList, []any, []string, *OrderedDict, map[string]any, map[any]any, *DictView:
		return fmt.Errorf("unhashable type: '%s'", PyTypeName(v))
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		fmt.Fprintf(b, "i%d;", rv.Int())
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		fmt.Fprintf(b, "i%d;", rv.Uint())
		return nil
	case reflect.Pointer, reflect.Func, reflect.Chan, reflect.UnsafePointer:
		// Other objects hash by identity, as Python's default __hash__.
		fmt.Fprintf(b, "p%s:%x;", rv.Type(), rv.Pointer())
		return nil
	case reflect.Slice, reflect.Map:
		return fmt.Errorf("unhashable type: '%s'", PyTypeName(v))
	}
	if rv.Type().Comparable() {
		s := fmt.Sprintf("%T:%#v", v, v)
		writeHashString(b, 'o', s)
		return nil
	}
	return fmt.Errorf("unhashable type: '%s'", PyTypeName(v))
}

func writeHashString(b *strings.Builder, tag byte, s string) {
	b.WriteByte(tag)
	b.WriteString(strconv.Itoa(len(s)))
	b.WriteByte(':')
	b.WriteString(s)
}

// writeHashFloat files an integral float with the int of the same value,
// as Python's hash(1.0) == hash(1).
func writeHashFloat(b *strings.Builder, f float64) {
	if f == math.Trunc(f) && math.Abs(f) < 1<<63 {
		fmt.Fprintf(b, "i%d;", int64(f))
		return
	}
	b.WriteString("f" + strconv.FormatFloat(f, 'g', -1, 64) + ";")
}

// hashKeyOf is HashKey for internal callers that cannot fail: an
// unhashable key is filed by its identity, so Go-side code can never
// crash a dict. Template-facing paths check HashKey first.
func hashKeyOf(v any) string {
	if k, err := HashKey(v); err == nil {
		return k
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Slice:
		return fmt.Sprintf("x%s:%x;", rv.Type(), rv.Pointer())
	}
	return fmt.Sprintf("x%T:%p;", v, &v)
}
