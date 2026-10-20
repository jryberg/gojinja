package runtime

import "fmt"

// LoopContext is the value the Jinja `loop` variable holds inside a `{% for %}`
// body, a port of jinja2.runtime.LoopContext. It pulls items from the
// iterable one at a time, peeking one ahead for `loop.last` and
// `loop.nextitem`; `loop.length` of an iterable without a length, such
// as a generator or a filtered loop, reads the rest of it.
type LoopContext struct {
	next Next
	// size is the iterable's len(), nil when it has none.
	size func() int

	index0 int
	length int
	sized  bool

	pending        []any // items read ahead to find the length
	after          any
	hasAfter       bool
	current        any
	before         any
	hasLastChanged bool
	lastChanged    any

	depth0  int
	recurse func(iterable any, depth int) (any, error)
	// undefined makes the environment's Undefined for previtem and
	// nextitem.
	undefined func(hint string) any

	// Equal is Python equality, which Changed compares with; the
	// evaluator sets it.
	Equal func(a, b any) bool
}

// NewLoopContext constructs a LoopContext over the items next produces.
// size is the iterable's len(), or nil. depth is 0 for top-level loops,
// ≥ 1 for recursive ones; undefined makes previtem's and nextitem's
// Undefined.
func NewLoopContext(next Next, size func() int, depth int, recurse func(any, int) (any, error), undefined func(hint string) any) *LoopContext {
	return &LoopContext{next: next, size: size, index0: -1, depth0: depth, recurse: recurse, undefined: undefined}
}

func (l *LoopContext) pull() (any, bool, error) {
	if len(l.pending) > 0 {
		v := l.pending[0]
		l.pending = l.pending[1:]
		return v, true, nil
	}
	return l.next()
}

// Length returns the iterable's length, reading the rest of an iterable
// that has none.
func (l *LoopContext) Length() (int, error) {
	if l.sized {
		return l.length, nil
	}
	if l.size != nil {
		l.length = l.size()
	} else {
		for {
			v, ok, err := l.next()
			if err != nil {
				return 0, err
			}
			if !ok {
				break
			}
			l.pending = append(l.pending, v)
		}
		l.length = len(l.pending) + l.index0 + 1
		if l.hasAfter {
			l.length++
		}
	}
	l.sized = true
	return l.length, nil
}

// Iterate advances to the next item, returning ok=false at the end of
// the loop.
func (l *LoopContext) Iterate() (any, bool, error) {
	var rv any
	if l.hasAfter {
		rv, l.after, l.hasAfter = l.after, nil, false
	} else {
		v, ok, err := l.pull()
		if err != nil || !ok {
			return nil, false, err
		}
		rv = v
	}
	l.index0++
	l.before, l.current = l.current, rv
	return rv, true, nil
}

// peek returns the next item without advancing, ok=false at the end.
func (l *LoopContext) peek() (any, bool, error) {
	if l.hasAfter {
		return l.after, true, nil
	}
	v, ok, err := l.pull()
	if err != nil || !ok {
		return nil, false, err
	}
	l.after, l.hasAfter = v, true
	return v, true, nil
}

// Index returns the 1-based current index.
func (l *LoopContext) Index() int { return l.index0 + 1 }

// Index0 returns the 0-based current index.
func (l *LoopContext) Index0() int { return l.index0 }

// Revindex returns 1-based countdown from the end.
func (l *LoopContext) Revindex() (int, error) {
	n, err := l.Length()
	return n - l.index0, err
}

// Revindex0 returns 0-based countdown from the end.
func (l *LoopContext) Revindex0() (int, error) {
	n, err := l.Length()
	return n - l.index0 - 1, err
}

// First reports whether the current iteration is the first.
func (l *LoopContext) First() bool { return l.index0 == 0 }

// Last reports whether the current iteration is the last, reading the
// next item early.
func (l *LoopContext) Last() (bool, error) {
	_, ok, err := l.peek()
	return !ok, err
}

// Depth is 1-based; top-level loop is 1.
func (l *LoopContext) Depth() int { return l.depth0 + 1 }

// Depth0 is 0-based.
func (l *LoopContext) Depth0() int { return l.depth0 }

// PrevItem returns the previous iteration's value, or Undefined if first.
func (l *LoopContext) PrevItem() any {
	if l.First() {
		return l.undefined("there is no previous item")
	}
	return l.before
}

// NextItem returns the next iteration's value, or Undefined if last.
func (l *LoopContext) NextItem() (any, error) {
	v, ok, err := l.peek()
	if err != nil {
		return nil, err
	}
	if !ok {
		return l.undefined("there is no next item"), nil
	}
	return v, nil
}

// Cycle returns args[index0 % len(args)]; used for round-robin classnames etc.
func (l *LoopContext) Cycle(args ...any) (any, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("no items for cycling given")
	}
	return args[l.index0%len(args)], nil
}

// Changed reports whether values differ from the previous call's (true
// on the first call), as Jinja2's loop.changed(*values) does.
func (l *LoopContext) Changed(values ...any) bool {
	cur := Tuple(values)
	if l.hasLastChanged && l.Equal != nil && l.Equal(l.lastChanged, cur) {
		return false
	}
	l.hasLastChanged, l.lastChanged = true, cur
	return true
}

// Call renders the loop body again over iterable, one level deeper:
// `loop(children)` in a recursive loop.
func (l *LoopContext) Call(iterable any) (any, error) {
	if l.recurse == nil {
		//lint:ignore ST1005 Jinja2's exact message
		return nil, fmt.Errorf("The loop must have the 'recursive' marker to be called recursively.")
	}
	return l.recurse(iterable, l.depth0+1)
}

// String is Jinja2's LoopContext repr. Finding the length can fail;
// the repr then shows what is known.
func (l *LoopContext) String() string {
	n, _ := l.Length()
	return fmt.Sprintf("<LoopContext %d/%d>", l.Index(), n)
}

// AttrGetter is implemented by template-visible objects that resolve
// their own attributes, such as macros.
type AttrGetter interface {
	Get(attr string) (any, bool)
}

// FallibleAttrGetter is an [AttrGetter] whose attributes can fail, as
// loop.length does when reading the rest of a generator raises.
type FallibleAttrGetter interface {
	GetAttr(attr string) (any, bool, error)
}

// GetAttr implements [FallibleAttrGetter].
func (l *LoopContext) GetAttr(attr string) (any, bool, error) {
	var v any
	var err error
	switch attr {
	case "index":
		v = l.Index()
	case "index0":
		v = l.Index0()
	case "revindex":
		v, err = l.Revindex()
	case "revindex0":
		v, err = l.Revindex0()
	case "first":
		v = l.First()
	case "last":
		v, err = l.Last()
	case "length":
		v, err = l.Length()
	case "depth":
		v = l.Depth()
	case "depth0":
		v = l.Depth0()
	case "previtem":
		v = l.PrevItem()
	case "nextitem":
		v, err = l.NextItem()
	case "cycle":
		v = l.Cycle
	case "changed":
		v = l.Changed
	default:
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return v, true, nil
}
