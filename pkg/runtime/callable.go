package runtime

import (
	"fmt"

	"github.com/jryberg/gojinja/pkg/escape"
)

// Callable is a builtin callable as a template sees it: a method bound
// to a value, a function or a class. It calls Fn, and prints and
// compares like its Python counterpart.
type Callable struct {
	// Fn is the Go function a call runs.
	Fn any

	kind     callableKind
	name     string // the method's name, or the qualified name
	self     any    // the bound value
	selfType string // a built-in method's receiver type
	selfRepr string // a bound method's receiver repr, when not repr(self)
}

type callableKind int

const (
	builtinMethod callableKind = iota
	boundMethod
	function
	class
)

// NewBuiltinMethod is a method of a builtin type bound to self, of
// Python type selfType: `<built-in method name of selfType object>`.
func NewBuiltinMethod(name string, self any, selfType string, fn any) *Callable {
	return &Callable{Fn: fn, kind: builtinMethod, name: name, self: self, selfType: selfType}
}

// NewBoundMethod is a method defined in Python on class, bound to self:
// `<bound method class.name of repr(self)>`. A non-empty selfRepr
// replaces repr(self), as for a classmethod.
func NewBoundMethod(class, name string, self any, selfRepr string, fn any) *Callable {
	return &Callable{Fn: fn, kind: boundMethod, name: class + "." + name, self: self, selfRepr: selfRepr}
}

// NewFunction is a Python function with the qualified name qualname.
func NewFunction(qualname string, fn any) *Callable {
	return &Callable{Fn: fn, kind: function, name: qualname}
}

// NewClass is a Python class with the qualified name qualname, such as
// jinja2.utils.Cycler.
func NewClass(qualname string, fn any) *Callable {
	return &Callable{Fn: fn, kind: class, name: qualname}
}

// TypeName is the Python type name.
func (c *Callable) TypeName() string {
	switch c.kind {
	case boundMethod:
		return "method"
	case function:
		return "function"
	case class:
		return "type"
	}
	return "builtin_function_or_method"
}

// String is Python's repr.
func (c *Callable) String() string {
	switch c.kind {
	case boundMethod:
		self := c.selfRepr
		if self == "" {
			self = escape.Repr(c.self)
		}
		return fmt.Sprintf("<bound method %s of %s>", c.name, self)
	case function:
		return fmt.Sprintf("<function %s at %p>", c.name, c)
	case class:
		return fmt.Sprintf("<class '%s'>", c.name)
	}
	return fmt.Sprintf("<built-in method %s of %s object at %p>", c.name, c.selfType, c)
}

// ClassName is a class's qualified name.
func (c *Callable) ClassName() (string, bool) { return c.name, c.kind == class }

// IsMethod reports whether c is a bound method, and gives its name and
// receiver. Two methods are equal when both match.
func (c *Callable) IsMethod() (name string, self any, ok bool) {
	return c.name, c.self, c.kind == builtinMethod || c.kind == boundMethod
}
