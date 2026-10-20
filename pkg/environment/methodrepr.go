package environment

import (
	"math/big"
	"reflect"

	"github.com/jryberg/gojinja/pkg/escape"
	"github.com/jryberg/gojinja/pkg/runtime"
)

// markupPythonMethods are the Markup methods markupsafe defines in
// Python, which print as bound methods; the others are str's built-ins.
var markupPythonMethods = map[string]bool{
	"join": true, "striptags": true, "unescape": true,
}

// bindMethod gives v, the method attr of a builtin value obj, the form
// Python prints it in: a built-in method, a bound method of a class
// defined in Python, or the function the sandbox wraps str.format in.
// Other values, and the methods of the caller's Go types, are kept.
func bindMethod(obj any, attr string, v any) any {
	if _, ok := v.(*runtime.Callable); ok || reflect.ValueOf(v).Kind() != reflect.Func {
		return v
	}
	switch x := obj.(type) {
	case string:
		if attr == "format" || attr == "format_map" {
			return runtime.NewFunction("str."+attr, v)
		}
	case escape.Markup:
		switch {
		case attr == "format" || attr == "format_map":
			return runtime.NewFunction("Markup."+attr, v)
		case attr == "escape":
			// A classmethod, bound to the class.
			return runtime.NewBoundMethod("Markup", attr, nil, "<class 'markupsafe.Markup'>", v)
		case markupPythonMethods[attr] || markupWrapped[attr]:
			return runtime.NewBoundMethod("Markup", attr, x, "", v)
		}
	case *runtime.Cycler:
		return runtime.NewBoundMethod("Cycler", attr, x, "", v)
	case *runtime.LoopContext:
		return runtime.NewBoundMethod("LoopContext", attr, x, "", v)
	case dictClass:
		return runtime.NewBuiltinMethod(attr, nil, "type", v)
	case float64, float32:
		if attr == "fromhex" {
			return runtime.NewBuiltinMethod(attr, nil, "type", v)
		}
	case bool, int, int64, *big.Int:
		if attr == "from_bytes" {
			return runtime.NewBuiltinMethod(attr, nil, "type", v)
		}
	case *runtime.OrderedDict, map[string]any, map[any]any:
		if attr == "fromkeys" {
			return runtime.NewBuiltinMethod(attr, nil, "type", v)
		}
	case []any, *runtime.PyList, runtime.Tuple, runtime.Bytes, *runtime.Range, *runtime.DictView,
		*runtime.Iterator, complex128:
	default:
		return v
	}
	return runtime.NewBuiltinMethod(attr, obj, runtime.PyTypeName(obj), v)
}
