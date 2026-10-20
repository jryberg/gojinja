package runtime

import (
	"errors"
	"fmt"
)

// Next returns an iterator's next item, or ok=false once it is exhausted.
type Next func() (item any, ok bool, err error)

// Iterator is a Python iterator: the generator that filters such as map,
// select and unique return, or the reverse iterator of the reverse
// filter. It is lazy and can be consumed only once. Like Python's, it is
// always true, has no length and is equal only to itself.
type Iterator struct {
	typ string // the Python type: "generator", "list_reverseiterator", ...
	fn  string // a generator's function name

	// start runs a generator's body up to its loop, on the first next.
	start func() (Next, error)
	next  Next

	started, finished bool

	// yieldFrom is the type of the iterator a generator delegates to
	// with `yield from`, and yieldIter that iterator once started.
	yieldFrom string
	yieldIter *Iterator
}

// NewGenerator returns the generator a Python generator function fn
// returns. start runs on the first next: argument checks and the setup
// before the loop, returning the function producing each item.
func NewGenerator(fn string, start func() (Next, error)) *Iterator {
	return &Iterator{typ: "generator", fn: fn, start: start}
}

// NewDelegatingGenerator is NewGenerator for a generator whose body is
// `yield from` an iterator of type yieldFrom, which gi_yieldfrom shows.
func NewDelegatingGenerator(fn, yieldFrom string, start func() (Next, error)) *Iterator {
	it := NewGenerator(fn, start)
	it.yieldFrom = yieldFrom
	return it
}

// NewIterator returns an iterator of Python type typ over next.
func NewIterator(typ string, next Next) *Iterator {
	return &Iterator{typ: typ, next: next, started: true}
}

// TypeName is the Python type name.
func (it *Iterator) TypeName() string { return it.typ }

// Next advances the iterator. A generator that raised is finished, as in
// Python.
func (it *Iterator) Next() (any, bool, error) {
	if it.finished {
		return nil, false, nil
	}
	if !it.started {
		it.started = true
		next, err := it.start()
		it.start = nil
		if err != nil {
			it.finish()
			return nil, false, err
		}
		it.next = next
	}
	v, ok, err := it.next()
	if err != nil || !ok {
		it.finish()
		return nil, false, err
	}
	return v, true, nil
}

func (it *Iterator) finish() {
	it.finished, it.next, it.yieldIter = true, nil, nil
}

// String is Python's repr, with the iterator's own address.
func (it *Iterator) String() string {
	if it.typ == "generator" {
		return fmt.Sprintf("<generator object %s at %p>", it.fn, it)
	}
	return fmt.Sprintf("<%s object at %p>", it.typ, it)
}

// ErrStopIteration is Python's StopIteration raised by a callable, such
// as send on an exhausted generator. Jinja2's Context.call turns it into
// an Undefined.
var ErrStopIteration = errors.New("value was undefined because a callable raised a StopIteration exception")

// Get implements a generator's public attributes and methods.
func (it *Iterator) Get(attr string) (any, bool) {
	if it.typ != "generator" {
		return nil, false
	}
	switch attr {
	case "gi_running":
		return false, true
	case "gi_suspended":
		return it.started && !it.finished, true
	case "gi_yieldfrom":
		if it.yieldFrom == "" || !it.started || it.finished {
			return nil, true
		}
		if it.yieldIter == nil {
			it.yieldIter = NewIterator(it.yieldFrom, func() (any, bool, error) {
				if it.finished {
					return nil, false, nil
				}
				return it.next()
			})
		}
		return it.yieldIter, true
	case "send":
		return func(args []any, kwargs map[string]any) (any, error) {
			switch {
			case len(kwargs) > 0:
				return nil, errors.New("generator.send() takes no keyword arguments")
			case len(args) != 1:
				return nil, fmt.Errorf("generator.send() takes exactly one argument (%d given)", len(args))
			case args[0] != nil && !it.started:
				return nil, errors.New("can't send non-None value to a just-started generator")
			}
			v, ok, err := it.Next()
			if err != nil {
				return nil, err
			}
			if !ok {
				return nil, ErrStopIteration
			}
			return v, nil
		}, true
	case "close":
		return func(args []any, kwargs map[string]any) (any, error) {
			switch {
			case len(kwargs) > 0:
				return nil, errors.New("generator.close() takes no keyword arguments")
			case len(args) != 0:
				return nil, fmt.Errorf("generator.close() takes no arguments (%d given)", len(args))
			}
			it.started = true
			it.finish()
			return nil, nil
		}, true
	case "throw":
		return func(args []any, kwargs map[string]any) (any, error) {
			switch {
			case len(kwargs) > 0:
				return nil, errors.New("generator.throw() takes no keyword arguments")
			case len(args) == 0:
				return nil, errors.New("throw expected at least 1 argument, got 0")
			case len(args) > 3:
				return nil, fmt.Errorf("throw expected at most 3 arguments, got %d", len(args))
			}
			// No template value is an exception.
			return nil, fmt.Errorf("exceptions must be classes or instances deriving from BaseException, not %s", PyTypeName(args[0]))
		}, true
	}
	return nil, false
}
