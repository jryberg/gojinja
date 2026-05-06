package runtime

// Tuple is a fixed-size ordered sequence — gojinja's runtime
// representation of Python's tuple. The user-visible difference from a
// list is the repr (`(1, 2)` vs `[1, 2]`); iteration, indexing, and
// length all behave identically.
//
// `dict.items()`, the `items` filter, the `groupby` filter, and the for
// loop's tuple-target unpacking ("for k, v in ...") all produce tuples
// so renders match Python.
type Tuple []any

// Iter returns the underlying slice, allowing eval.toIterable to walk
// the tuple. We expose the slice rather than a callable so reflection
// and range-based code paths continue to work.
func (t Tuple) Iter() []any { return []any(t) }

// Len returns the number of elements.
func (t Tuple) Len() int { return len(t) }

// At returns element i. Out-of-range access is the caller's
// responsibility — the eval layer's GetItem already does bounds-checking.
func (t Tuple) At(i int) any { return t[i] }
