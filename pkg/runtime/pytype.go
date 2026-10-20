package runtime

import (
	"fmt"
	"math/big"
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
	case *big.Int:
		return "int"
	case complex128:
		return "complex"
	case []any, *PyList, []string:
		return "list"
	case Tuple:
		return "tuple"
	case Bytes:
		return "bytes"
	case *OrderedDict:
		if x.IsProxy() {
			return "mappingproxy"
		}
		return "dict"
	case map[string]any, map[any]any:
		return "dict"
	case Undefined:
		return undefinedClass[x.Mode]
	case *Namespace:
		return "Namespace"
	case *Iterator:
		return x.TypeName()
	case *Callable:
		return x.TypeName()
	case *BlockReference:
		return "BlockReference"
	case *Range:
		return "range"
	case *DictView:
		return x.TypeName()
	case *LoopContext:
		return "LoopContext"
	case *Cycler:
		return "Cycler"
	case *Joiner:
		return "Joiner"
	}
	if t, ok := v.(interface{ TypeName() string }); ok {
		return t.TypeName()
	}
	switch reflect.ValueOf(v).Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "int"
	case reflect.Func:
		return "function"
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
	if _, ok := v.(Undefined); ok {
		return "jinja2.runtime." + name + " object"
	}
	if module, ok := pyModules[name]; ok {
		return module + "." + name + " object"
	}
	return name + " object"
}

// pyModules are the modules of the Python classes object_type_repr
// qualifies, by class name; builtins have none.
var pyModules = map[string]string{
	"Markup":            "markupsafe",
	"LoopContext":       "jinja2.runtime",
	"Macro":             "jinja2.runtime",
	"BlockReference":    "jinja2.runtime",
	"TemplateReference": "jinja2.runtime",
	"Namespace":         "jinja2.utils",
	"Cycler":            "jinja2.utils",
	"Joiner":            "jinja2.utils",
	"TemplateModule":    "jinja2.environment",
}
