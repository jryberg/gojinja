package runtime

import (
	"fmt"

	"github.com/jryberg/gojinja/pkg/escape"
)

// Range is Python's range object, which range() returns: an immutable
// arithmetic sequence that prints as range(start, stop[, step]) and
// slices into another range.
type Range struct {
	Start, Stop, Step int
}

// Len is the number of members.
func (r *Range) Len() int {
	switch {
	case r.Step > 0 && r.Start < r.Stop:
		return (r.Stop - r.Start + r.Step - 1) / r.Step
	case r.Step < 0 && r.Start > r.Stop:
		return (r.Start - r.Stop - r.Step - 1) / -r.Step
	}
	return 0
}

// At returns the i-th member (0 <= i < Len()).
func (r *Range) At(i int) int { return r.Start + i*r.Step }

// List returns the members.
func (r *Range) List() []any {
	n := r.Len()
	out := make([]any, n)
	for i := range n {
		out[i] = r.At(i)
	}
	return out
}

// String is Python's repr.
func (r *Range) String() string {
	if r.Step == 1 {
		return fmt.Sprintf("range(%d, %d)", r.Start, r.Stop)
	}
	return fmt.Sprintf("range(%d, %d, %d)", r.Start, r.Stop, r.Step)
}

// Equal is range ==: two ranges are equal when they hold the same
// members, whatever their arguments.
func (r *Range) Equal(o *Range) bool {
	n := r.Len()
	if n != o.Len() {
		return false
	}
	return n == 0 || r.Start == o.Start && (n == 1 || r.Step == o.Step)
}

// hashKey files equal ranges together.
func (r *Range) hashKey() string {
	switch n := r.Len(); n {
	case 0:
		return "r0;"
	case 1:
		return fmt.Sprintf("r1:%d;", r.Start)
	default:
		return fmt.Sprintf("r%d:%d:%d;", n, r.Start, r.Step)
	}
}

// index returns the position of the int x in r.
func (r *Range) index(x int) (int, bool) {
	if r.Len() == 0 || (x-r.Start)%r.Step != 0 {
		return 0, false
	}
	i := (x - r.Start) / r.Step
	return i, i >= 0 && i < r.Len()
}

// Contains is `x in r`: arithmetic for an int (or bool), a scan with eq
// for anything else, as CPython does.
func (r *Range) Contains(x any, eq func(a, b any) (bool, error)) (bool, error) {
	if i, ok := rangeInt(x); ok {
		_, found := r.index(i)
		return found, nil
	}
	for i := range r.Len() {
		if ok, err := eq(r.At(i), x); err != nil || ok {
			return ok, err
		}
	}
	return false, nil
}

// Get exposes start, stop, step, index() and count().
func (r *Range) Get(attr string) (any, bool) {
	switch attr {
	case "start":
		return r.Start, true
	case "stop":
		return r.Stop, true
	case "step":
		return r.Step, true
	case "index", "count":
		return func(args []any, kwargs map[string]any) (any, error) {
			if len(kwargs) > 0 {
				return nil, fmt.Errorf("range.%s() takes no keyword arguments", attr)
			}
			if len(args) != 1 {
				return nil, fmt.Errorf("range.%s() takes exactly one argument (%d given)", attr, len(args))
			}
			// Only numbers equal a range's ints.
			pos, found := 0, false
			if i, ok := rangeInt(args[0]); ok {
				pos, found = r.index(i)
			}
			if attr == "count" {
				if found {
					return 1, nil
				}
				return 0, nil
			}
			if !found {
				return nil, fmt.Errorf("%s is not in range", escape.Repr(args[0]))
			}
			return pos, nil
		}, true
	}
	return nil, false
}

// rangeInt returns x as an int when Python would test range membership
// arithmetically: ints, bools and integral floats.
func rangeInt(x any) (int, bool) {
	switch v := x.(type) {
	case bool:
		if v {
			return 1, true
		}
		return 0, true
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		if v == float64(int(v)) {
			return int(v), true
		}
	}
	return 0, false
}
