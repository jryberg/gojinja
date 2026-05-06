package runtime

import "fmt"

// LoopContext is the value the Jinja `loop` variable holds inside a `{% for %}`
// body. It tracks the current iteration index, the wrapped iterable, and
// peeks ahead so `loop.last` and `loop.nextitem` can be answered.
//
// gojinja's LoopContext mirrors jinja2.runtime.LoopContext except it is
// constructed from a Go slice (parsed once) rather than from an arbitrary
// Python iterable. The evaluator turns iterables into []any before
// constructing LoopContext.
type LoopContext struct {
	items []any
	idx   int // current index, -1 before first iteration

	depth0 int

	lastChanged any
	hasLastVal  bool

	// recurse, when set, is the closure for `{% for %}` recursive loops.
	recurse func(items []any, depth int) (string, error)
}

// NewLoopContext constructs a LoopContext over items. depth is 0 for top-
// level loops, ≥ 1 for recursive ones.
func NewLoopContext(items []any, depth int, recurse func([]any, int) (string, error)) *LoopContext {
	return &LoopContext{items: items, idx: -1, depth0: depth, recurse: recurse}
}

// Length returns the iterable's total length (1-based).
func (l *LoopContext) Length() int { return len(l.items) }

// Iterate prepares LoopContext for the next iteration and returns the
// current value plus ok=false at the end of the loop.
func (l *LoopContext) Iterate() (any, bool) {
	l.idx++
	if l.idx >= len(l.items) {
		return nil, false
	}
	return l.items[l.idx], true
}

// Index returns the 1-based current index.
func (l *LoopContext) Index() int { return l.idx + 1 }

// Index0 returns the 0-based current index.
func (l *LoopContext) Index0() int { return l.idx }

// Revindex returns 1-based countdown from the end.
func (l *LoopContext) Revindex() int { return len(l.items) - l.idx }

// Revindex0 returns 0-based countdown from the end.
func (l *LoopContext) Revindex0() int { return len(l.items) - l.idx - 1 }

// First reports whether the current iteration is the first.
func (l *LoopContext) First() bool { return l.idx == 0 }

// Last reports whether the current iteration is the last.
func (l *LoopContext) Last() bool { return l.idx == len(l.items)-1 }

// Depth is 1-based; top-level loop is 1.
func (l *LoopContext) Depth() int { return l.depth0 + 1 }

// Depth0 is 0-based.
func (l *LoopContext) Depth0() int { return l.depth0 }

// PrevItem returns the previous iteration's value, or Undefined if first.
func (l *LoopContext) PrevItem() any {
	if l.idx <= 0 {
		return NewBase("there is no previous item", "previtem", nil, nil)
	}
	return l.items[l.idx-1]
}

// NextItem returns the next iteration's value, or Undefined if last.
func (l *LoopContext) NextItem() any {
	if l.idx+1 >= len(l.items) {
		return NewBase("there is no next item", "nextitem", nil, nil)
	}
	return l.items[l.idx+1]
}

// Cycle returns args[index0 % len(args)]; used for round-robin classnames etc.
func (l *LoopContext) Cycle(args ...any) any {
	if len(args) == 0 {
		return NewBase("no items for cycle", "cycle", nil, nil)
	}
	return args[l.idx%len(args)]
}

// Changed reports whether `value` differs from the value of the previous
// `Changed` call. The first call always returns true.
func (l *LoopContext) Changed(value any) bool {
	if !l.hasLastVal {
		l.hasLastVal = true
		l.lastChanged = value
		return true
	}
	if !equal(l.lastChanged, value) {
		l.lastChanged = value
		return true
	}
	return false
}

// Call delegates to the recursive closure passed at construction. Errors
// if the loop wasn't created with `recursive`.
func (l *LoopContext) Call(items []any) (string, error) {
	if l.recurse == nil {
		return "", fmt.Errorf("loop is not marked recursive")
	}
	return l.recurse(items, l.depth0+1)
}

// Get implements attribute access for the template's `loop.<name>` syntax.
func (l *LoopContext) Get(attr string) (any, bool) {
	switch attr {
	case "index":
		return l.Index(), true
	case "index0":
		return l.Index0(), true
	case "revindex":
		return l.Revindex(), true
	case "revindex0":
		return l.Revindex0(), true
	case "first":
		return l.First(), true
	case "last":
		return l.Last(), true
	case "length":
		return l.Length(), true
	case "depth":
		return l.Depth(), true
	case "depth0":
		return l.Depth0(), true
	case "previtem":
		return l.PrevItem(), true
	case "nextitem":
		return l.NextItem(), true
	case "cycle":
		return l.Cycle, true
	case "changed":
		return l.Changed, true
	}
	return nil, false
}

// equal compares two values for the Changed test. Uses == when types are
// comparable; falls back to false for slices/maps (always treated as
// changed, mirroring Python's `!=` on non-hashables — `Changed` is
// expected to be called with hashable inputs).
func equal(a, b any) bool {
	defer func() {
		recover() //nolint:errcheck — defensive against incomparable types
	}()
	return a == b
}
