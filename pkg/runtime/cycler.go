package runtime

// Cycler cycles through a fixed list of values, returning the next value
// on each call. Mirrors jinja2.utils.Cycler. Templates use it via the
// `cycler` global: `{% set row = cycler('odd', 'even') %}{{ row.next() }}`.
type Cycler struct {
	items []any
	pos   int
}

// NewCycler builds a Cycler over items.
func NewCycler(items ...any) *Cycler { return &Cycler{items: items} }

// Reset rewinds to the first item.
func (c *Cycler) Reset() { c.pos = 0 }

// Next returns the current item and advances the cursor.
func (c *Cycler) Next() any {
	if len(c.items) == 0 {
		return nil
	}
	v := c.items[c.pos]
	c.pos = (c.pos + 1) % len(c.items)
	return v
}

// Current returns the value Next would return without advancing.
func (c *Cycler) Current() any {
	if len(c.items) == 0 {
		return nil
	}
	return c.items[c.pos]
}

// Get implements attribute access for `c.next`, `c.current`, `c.reset`.
// Methods are returned as zero-arg Go funcs so the env.Call bridge can
// invoke them.
func (c *Cycler) Get(attr string) (any, bool) {
	switch attr {
	case "next":
		return func() any { return c.Next() }, true
	case "current":
		return c.Current(), true
	case "reset":
		return func() { c.Reset() }, true
	}
	return nil, false
}

// Joiner emits a separator on every call but the first. Mirrors
// jinja2.utils.Joiner. Templates use it via the `joiner` global:
// `{% set j = joiner(', ') %}{% for x in xs %}{{ j() }}{{ x }}{% endfor %}`.
type Joiner struct {
	sep    string
	primed bool
}

// NewJoiner returns a Joiner that emits sep between items (default ", ").
func NewJoiner(sep string) *Joiner {
	if sep == "" {
		sep = ", "
	}
	return &Joiner{sep: sep}
}

// Call returns "" the first time and sep on every subsequent call.
func (j *Joiner) Call() string {
	if !j.primed {
		j.primed = true
		return ""
	}
	return j.sep
}
