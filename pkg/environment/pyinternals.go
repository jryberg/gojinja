package environment

import (
	"strings"

	"github.com/jryberg/gojinja/pkg/runtime"
)

// pyInternalAttr is an internal attribute that unsandboxed Python
// templates can read: obj.__class__, a class's __name__, __qualname__
// and __module__, and an Undefined's _undefined_name and
// _undefined_hint. The sandbox refuses all of them.
func pyInternalAttr(obj any, attr string) (any, bool) {
	if attr == "__class__" {
		return runtime.NewClass(pyQualifiedType(obj), nil), true
	}
	if c, ok := obj.(*runtime.Callable); ok {
		if name, isClass := c.ClassName(); isClass {
			module, base := "builtins", name
			if i := strings.LastIndexByte(name, '.'); i >= 0 {
				module, base = name[:i], name[i+1:]
			}
			switch attr {
			case "__name__", "__qualname__":
				return base, true
			case "__module__":
				return module, true
			}
		}
	}
	if u, ok := obj.(runtime.Undefined); ok {
		switch attr {
		case "_undefined_name":
			if u.Name == "" {
				return nil, true
			}
			return u.Name, true
		case "_undefined_hint":
			if u.Hint == "" {
				return nil, true
			}
			return u.Hint, true
		}
	}
	return nil, false
}

// pyQualifiedType is the module-qualified name of obj's Python type, as
// its class prints it: str, markupsafe.Markup, jinja2.utils.Cycler.
func pyQualifiedType(obj any) string {
	if obj == nil || obj == runtime.None {
		return "NoneType"
	}
	return strings.TrimSuffix(runtime.ObjectTypeRepr(obj), " object")
}
