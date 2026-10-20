package runtime

import (
	"fmt"
	"reflect"

	"github.com/jryberg/gojinja/pkg/escape"
)

// PyTypeName is the Python type name templates see for v, as used in
// TypeError messages ('int', 'str', 'list', ...). Go values with no
// Python counterpart report their Go type.
func PyTypeName(v any) string {
	switch x := v.(type) {
	case nil, noneType:
		return "NoneType"
	case bool:
		return "bool"
	case string:
		return "str"
	case escape.Markup:
		return "Markup"
	case float32, float64:
		return "float"
	case []any, *PyList, []string:
		return "list"
	case Tuple:
		return "tuple"
	case Bytes:
		return "bytes"
	case map[string]any, map[any]any, *OrderedDict:
		return "dict"
	case Undefined:
		return undefinedClass[x.Mode]
	case *Namespace:
		return "Namespace"
	case *DictView:
		return x.TypeName()
	case *LoopContext:
		return "LoopContext"
	case *Cycler:
		return "Cycler"
	case *Joiner:
		return "Joiner"
	}
	switch reflect.ValueOf(v).Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "int"
	}
	return fmt.Sprintf("%T", v)
}

var undefinedClass = map[UndefinedMode]string{
	ModeBase:      "Undefined",
	ModeChainable: "ChainableUndefined",
	ModeDebug:     "DebugUndefined",
	ModeStrict:    "StrictUndefined",
}

// ObjectTypeRepr mirrors jinja2.utils.object_type_repr: "None" for None,
// "<type> object" for builtins and "<module>.<type> object" otherwise.
func ObjectTypeRepr(v any) string {
	if v == nil || v == None {
		return "None"
	}
	name := PyTypeName(v)
	switch v.(type) {
	case escape.Markup:
		return "markupsafe.Markup object"
	case Undefined, *LoopContext:
		return "jinja2.runtime." + name + " object"
	case *Namespace, *Cycler, *Joiner:
		return "jinja2.utils." + name + " object"
	}
	return name + " object"
}
