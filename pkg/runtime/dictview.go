package runtime

// DictView is what dict.keys(), dict.values() and dict.items() return:
// a live view of the dict, so later changes to it show through.
type DictView struct {
	Dict *OrderedDict
	Kind string // "keys", "values" or "items"
}

// Items returns the view's current members: keys, values, or (key,
// value) tuples.
func (v *DictView) Items() []any {
	switch v.Kind {
	case "keys":
		return v.Dict.Keys()
	case "values":
		return v.Dict.Values()
	}
	return v.Dict.Items()
}

// Len is the number of members.
func (v *DictView) Len() int { return v.Dict.Len() }

// TypeName is the Python type name, as in dict_keys.
func (v *DictView) TypeName() string { return "dict_" + v.Kind }

// ReprView returns what the repr shows: dict_keys([...]).
func (v *DictView) ReprView() (string, []any) { return v.TypeName(), v.Items() }

// Contains reports Python's `x in view`: key membership for keys,
// (key, value) membership for items. Values views scan with eq.
func (v *DictView) Contains(x any, eq func(a, b any) (bool, error)) (bool, error) {
	switch v.Kind {
	case "keys":
		if _, err := HashKey(x); err != nil {
			return false, err
		}
		_, ok := v.Dict.Get(x)
		return ok, nil
	case "items":
		t, ok := x.(Tuple)
		if !ok || len(t) != 2 {
			return false, nil
		}
		val, found := v.Dict.Get(t[0])
		if !found {
			return false, nil
		}
		return eq(val, t[1])
	}
	for _, it := range v.Dict.Values() {
		if ok, err := eq(x, it); err != nil || ok {
			return ok, err
		}
	}
	return false, nil
}
