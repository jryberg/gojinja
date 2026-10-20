package runtime

// NewGroupTuple returns the groupby filter's (grouper, list) tuple,
// Jinja2's _GroupTuple namedtuple. It stays a plain Tuple, so it unpacks,
// prints and compares as one; the list points back at the tuple's first
// slot, which is what gives this tuple, and no other holding the same
// list, its .grouper and .list attributes.
func NewGroupTuple(grouper any, list *PyList) Tuple {
	t := Tuple{grouper, list}
	list.groupHead = &t[0]
	return t
}

// GroupTupleAttr returns the grouper or list attribute of a tuple made by
// NewGroupTuple.
func GroupTupleAttr(t Tuple, attr string) (any, bool) {
	if len(t) != 2 || (attr != "grouper" && attr != "list") {
		return nil, false
	}
	if l, ok := t[1].(*PyList); !ok || l.groupHead != &t[0] {
		return nil, false
	}
	if attr == "grouper" {
		return t[0], true
	}
	return t[1], true
}
