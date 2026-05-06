package runtime

import (
	"fmt"

	gjerrors "github.com/jryberg/gojinja/pkg/errors"
)

// UndefinedMode selects the behavioral variant of an Undefined value:
//
//	ModeBase       — the default; prints empty, iterates empty, is falsy,
//	                 errors only on real use (arithmetic, call, getitem,
//	                 inequality comparison).
//	ModeChainable  — base + getattr/getitem return the same Undefined.
//	ModeDebug      — base, but printing yields "{{ name }}"-style placeholder.
//	ModeStrict     — every observation errors. Even printing or boolean test.
//
// Selecting a mode is a per-Environment choice (see PHASE_10_environment.md).
type UndefinedMode int

const (
	ModeBase UndefinedMode = iota
	ModeChainable
	ModeDebug
	ModeStrict
)

// Undefined is gojinja's representation of "no value at this name". It
// mirrors jinja2.runtime.Undefined and its variants. Code that operates on
// values must check for Undefined before doing arithmetic, calls,
// comparisons, etc.; the helper [FailOnUndefined] is the canonical guard.
//
// In Python, the corresponding behavior is implemented by overriding
// __add__ / __getattr__ / etc. on the class. Go has no operator overloading,
// so the *evaluator* is responsible for dispatching to [Undefined.Fail]
// when an op encounters an Undefined.
type Undefined struct {
	Mode UndefinedMode
	Hint string
	Name string
	Obj  any
	// ExcConstructor builds the error returned by Fail. Defaults to
	// errors.NewUndefinedError. Sandbox-related Undefineds use
	// errors.NewSecurityError.
	ExcConstructor func(string) error
}

// IsUndefined satisfies the [Undefiner] interface and serves as the type
// marker mirrored by Python's `isinstance(x, Undefined)`.
func (u Undefined) IsUndefined() bool { return true }

// Undefiner is implemented by every Undefined value. Use [IsUndefined] to
// test arbitrary values.
type Undefiner interface {
	IsUndefined() bool
}

// IsUndefined reports whether v is an Undefined value (any mode).
func IsUndefined(v any) bool {
	if v == nil {
		return false
	}
	u, ok := v.(Undefiner)
	return ok && u.IsUndefined()
}

// String renders the Undefined to its visible text form. ModeDebug emits a
// `{{ … }}` placeholder; ModeStrict returns "" but the evaluator should
// never call String on a Strict — it must call [Undefined.Bool] or call
// chains that explicitly check IsStrict first. Returning "" rather than
// panicking keeps Go's fmt printing safe for log lines.
func (u Undefined) String() string {
	switch u.Mode {
	case ModeDebug:
		return u.debugForm()
	default:
		return ""
	}
}

// Bool returns the truthiness; for Strict it returns an error.
func (u Undefined) Bool() (bool, error) {
	if u.Mode == ModeStrict {
		return false, u.Fail()
	}
	return false, nil
}

// Iter returns an empty iterator; for Strict it returns an error.
func (u Undefined) Iter() ([]any, error) {
	if u.Mode == ModeStrict {
		return nil, u.Fail()
	}
	return nil, nil
}

// Equal returns true iff other is an Undefined of the same Mode. Strict
// fails even on equality, mirroring jinja2.runtime.StrictUndefined.__eq__.
func (u Undefined) Equal(other any) (bool, error) {
	if u.Mode == ModeStrict {
		return false, u.Fail()
	}
	o, ok := other.(Undefined)
	if !ok {
		return false, nil
	}
	return o.Mode == u.Mode, nil
}

// GetAttr returns the value of accessing a named attribute on the Undefined.
// ModeChainable returns the same Undefined; every other mode fails.
func (u Undefined) GetAttr(name string) (any, error) {
	if u.Mode == ModeChainable {
		return u, nil
	}
	return nil, u.Fail()
}

// GetItem mirrors GetAttr for subscript access.
func (u Undefined) GetItem(arg any) (any, error) {
	if u.Mode == ModeChainable {
		return u, nil
	}
	return nil, u.Fail()
}

// Fail returns the configured undefined-error for u.
func (u Undefined) Fail() error {
	exc := u.ExcConstructor
	if exc == nil {
		exc = func(msg string) error { return gjerrors.NewUndefinedError(msg) }
	}
	return exc(u.Message())
}

// Message builds the undefined-message text mirroring Python's
// _undefined_message property.
func (u Undefined) Message() string {
	if u.Hint != "" {
		return u.Hint
	}
	if u.Obj == nil || IsMissing(u.Obj) {
		return fmt.Sprintf("%q is undefined", u.Name)
	}
	return fmt.Sprintf("%s has no attribute %q", typeRepr(u.Obj), u.Name)
}

// debugForm builds the `{{ … }}` placeholder used by ModeDebug. The exact
// format mirrors jinja2.runtime.DebugUndefined.__str__.
func (u Undefined) debugForm() string {
	switch {
	case u.Hint != "":
		return fmt.Sprintf("{{ undefined value printed: %s }}", u.Hint)
	case u.Obj == nil || IsMissing(u.Obj):
		return fmt.Sprintf("{{ %s }}", u.Name)
	default:
		return fmt.Sprintf("{{ no such element: %s[%q] }}", typeRepr(u.Obj), u.Name)
	}
}

// typeRepr mimics jinja2.utils.object_type_repr: short, human-friendly
// label for an arbitrary Go value's type.
func typeRepr(obj any) string {
	if obj == nil {
		return "None"
	}
	return fmt.Sprintf("%T object", obj)
}

// UndefinedFactory is the constructor signature an Environment registers
// to mint Undefined values during rendering. The defaults below match the
// Python class hierarchy.
type UndefinedFactory func(hint, name string, obj any, exc func(string) error) Undefined

// NewBase returns a ModeBase Undefined.
func NewBase(hint, name string, obj any, exc func(string) error) Undefined {
	return Undefined{Mode: ModeBase, Hint: hint, Name: name, Obj: obj, ExcConstructor: exc}
}

// NewChainable returns a ModeChainable Undefined.
func NewChainable(hint, name string, obj any, exc func(string) error) Undefined {
	return Undefined{Mode: ModeChainable, Hint: hint, Name: name, Obj: obj, ExcConstructor: exc}
}

// NewDebug returns a ModeDebug Undefined.
func NewDebug(hint, name string, obj any, exc func(string) error) Undefined {
	return Undefined{Mode: ModeDebug, Hint: hint, Name: name, Obj: obj, ExcConstructor: exc}
}

// NewStrict returns a ModeStrict Undefined.
func NewStrict(hint, name string, obj any, exc func(string) error) Undefined {
	return Undefined{Mode: ModeStrict, Hint: hint, Name: name, Obj: obj, ExcConstructor: exc}
}
