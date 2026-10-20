package runtime

// missingType is the type of [Missing], a sentinel that marks "no value
// was supplied" in places where nil/false would be ambiguous (e.g. the
// initial state of a [LoopContext] _before_ field, or "no default for
// macro parameter").
type missingType struct{}

// Missing is the singleton sentinel. Compare with `v == runtime.Missing`.
var Missing missingType

// String makes Missing print as "missing", matching Jinja2's repr.
func (missingType) String() string { return "missing" }

// noneType is the type of [None].
type noneType struct{}

// None marks an [Undefined] produced by a lookup on Python's None
// (`none.x`, `none['x']`), where Undefined.Obj would otherwise be nil,
// which means the variable itself is missing.
var None noneType

// String makes None print as Python's None.
func (noneType) String() string { return "None" }

// IsMissing returns true iff v is the [Missing] sentinel.
func IsMissing(v any) bool {
	_, ok := v.(missingType)
	return ok
}
