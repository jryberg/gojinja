package environment

import (
	"fmt"
	"reflect"
	"strings"

	gjerrors "github.com/jryberg/gojinja/pkg/errors"
	"github.com/jryberg/gojinja/pkg/escape"
	"github.com/jryberg/gojinja/pkg/runtime"
	"github.com/jryberg/gojinja/pkg/sandbox"
)

// unsafeAttr applies SandboxedEnvironment's attribute rule to obj.attr.
// An attribute starting with an underscore, or one exposing the engine
// (a generator's frame or code, a class's mro), is unsafe. When obj has
// it, the result is an Undefined that raises SecurityError once used;
// ok=false leaves the lookup to go on, as for an attribute obj lacks,
// which Python then looks up as an item.
func (e *Environment) unsafeAttr(obj any, attr string) (any, bool) {
	if !e.sandboxed {
		return nil, false
	}
	goInternal := !strings.HasPrefix(attr, "_") && !sandbox.IsSafeAttribute(obj, attr)
	if !goInternal && !strings.HasPrefix(attr, "_") && !internalAttr(obj, attr) {
		return nil, false
	}
	if !goInternal && !pyHasAttr(obj, attr) {
		return nil, false
	}
	msg := fmt.Sprintf("access to attribute %s of %s object is unsafe.", escape.Repr(attr), escape.Repr(runtime.PyTypeName(obj)))
	return e.undefined(msg, attr, obj, func(m string) error { return gjerrors.NewSecurityError(m) }), true
}

// internalAttr is is_internal_attribute for the names without an
// underscore it rejects.
func internalAttr(obj any, attr string) bool {
	switch x := obj.(type) {
	case *runtime.Iterator:
		return x.TypeName() == "generator" && (attr == "gi_frame" || attr == "gi_code")
	case *runtime.Callable:
		return x.TypeName() == "type" && attr == "mro"
	case dictClass:
		return attr == "mro"
	}
	return false
}

// pyHasAttr reports whether the Python counterpart of obj has attr.
func pyHasAttr(obj any, attr string) bool {
	if internalAttr(obj, attr) {
		return true
	}
	key := runtime.PyTypeName(obj)
	switch x := obj.(type) {
	case *runtime.Namespace:
		if _, ok := x.Get(attr); ok {
			return true
		}
	case *runtime.Callable:
		if name, ok := x.ClassName(); ok {
			key = "type:" + name
		}
	case dictClass:
		key = "type:dict"
	}
	attrs, ok := pyPrivateAttrs[key]
	if !ok {
		// The caller's Go values are objects of their own classes, whose
		// fields are attributes.
		attrs = pyPrivateAttrs["object"]
		t := reflect.TypeOf(obj)
		for t != nil && t.Kind() == reflect.Pointer {
			t = t.Elem()
		}
		if t != nil && t.Kind() == reflect.Struct {
			if _, ok := t.FieldByName(attr); ok {
				return true
			}
		}
	}
	return attrs[attr]
}
