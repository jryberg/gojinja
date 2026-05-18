package runtime

import (
	"reflect"
	"testing"
)

func TestNormalizeForTemplate_StringMapBecomesOrderedDict(t *testing.T) {
	in := map[string]any{"z": 1, "a": 2, "m": 3}
	out, ok := NormalizeForTemplate(in).(*OrderedDict)
	if !ok {
		t.Fatalf("NormalizeForTemplate(map[string]any) = %T, want *OrderedDict", out)
	}
	got := make([]string, 0, out.Len())
	for _, k := range out.Keys() {
		got = append(got, k.(string))
	}
	want := []string{"a", "m", "z"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("keys = %v, want %v (lex-sorted)", got, want)
	}
	if v, _ := out.Get("a"); v != 2 {
		t.Errorf("Get(a) = %v, want 2", v)
	}
}

func TestNormalizeForTemplate_NestedStringMapIsRecursive(t *testing.T) {
	in := map[string]any{
		"outer": map[string]any{"z": 1, "a": 2},
	}
	out := NormalizeForTemplate(in).(*OrderedDict)
	inner, _ := out.Get("outer")
	innerOD, ok := inner.(*OrderedDict)
	if !ok {
		t.Fatalf("inner = %T, want *OrderedDict", inner)
	}
	got := make([]string, 0, innerOD.Len())
	for _, k := range innerOD.Keys() {
		got = append(got, k.(string))
	}
	want := []string{"a", "z"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("nested keys = %v, want %v", got, want)
	}
}

func TestNormalizeForTemplate_OrderedDictPreservesOuterOrder(t *testing.T) {
	od := NewOrderedDict()
	od.Set("zeta", map[string]any{"y": 1, "b": 2})
	od.Set("alpha", 1)
	out := NormalizeForTemplate(od).(*OrderedDict)

	if out != od {
		t.Errorf("NormalizeForTemplate returned a fresh pointer; want same pointer")
	}
	keys := out.Keys()
	if !reflect.DeepEqual(keys, []any{"zeta", "alpha"}) {
		t.Errorf("outer keys = %v, want [zeta alpha] (insertion order preserved)", keys)
	}

	nested, _ := out.Get("zeta")
	nestedOD, ok := nested.(*OrderedDict)
	if !ok {
		t.Fatalf("nested zeta = %T, want *OrderedDict", nested)
	}
	nestedKeys := make([]string, 0, nestedOD.Len())
	for _, k := range nestedOD.Keys() {
		nestedKeys = append(nestedKeys, k.(string))
	}
	if !reflect.DeepEqual(nestedKeys, []string{"b", "y"}) {
		t.Errorf("nested keys = %v, want [b y] (lex-sorted)", nestedKeys)
	}
}

func TestNormalizeForTemplate_AnySliceWalks(t *testing.T) {
	in := []any{
		map[string]any{"z": 1, "a": 2},
		"plain",
		42,
	}
	out := NormalizeForTemplate(in).([]any)
	if len(out) != 3 {
		t.Fatalf("len = %d, want 3", len(out))
	}
	first, ok := out[0].(*OrderedDict)
	if !ok {
		t.Fatalf("element 0 = %T, want *OrderedDict", out[0])
	}
	keys := make([]string, 0)
	for _, k := range first.Keys() {
		keys = append(keys, k.(string))
	}
	if !reflect.DeepEqual(keys, []string{"a", "z"}) {
		t.Errorf("nested map keys = %v, want [a z]", keys)
	}
	if out[1] != "plain" {
		t.Errorf("string passthrough lost: %v", out[1])
	}
	if out[2] != 42 {
		t.Errorf("int passthrough lost: %v", out[2])
	}
}

func TestNormalizeForTemplate_TypedMapSliceWalks(t *testing.T) {
	in := []map[string]any{
		{"z": 1, "a": 2},
		{"name": "B"},
	}
	out := NormalizeForTemplate(in).([]any)
	if len(out) != 2 {
		t.Fatalf("len = %d, want 2", len(out))
	}
	first, ok := out[0].(*OrderedDict)
	if !ok {
		t.Fatalf("element 0 = %T, want *OrderedDict", out[0])
	}
	got := make([]string, 0)
	for _, k := range first.Keys() {
		got = append(got, k.(string))
	}
	if !reflect.DeepEqual(got, []string{"a", "z"}) {
		t.Errorf("keys = %v, want [a z]", got)
	}
}

func TestNormalizeForTemplate_AnyMapSorted(t *testing.T) {
	in := map[any]any{2: "b", 1: "a", 10: "c"}
	out := NormalizeForTemplate(in).(*OrderedDict)
	keys := out.Keys()
	// Stringified form: "1" < "10" < "2".
	want := []any{1, 10, 2}
	if !reflect.DeepEqual(keys, want) {
		t.Errorf("keys = %v, want %v (by stringified form)", keys, want)
	}
}

func TestNormalizeForTemplate_PrimitivesPassthrough(t *testing.T) {
	cases := []any{"hello", 42, 3.14, true, false, nil}
	for _, c := range cases {
		got := NormalizeForTemplate(c)
		if got != c {
			t.Errorf("NormalizeForTemplate(%v) = %v, want %v", c, got, c)
		}
	}
}

func TestNormalizeForTemplate_NilMapTypedNil(t *testing.T) {
	var nilStr map[string]any
	got := NormalizeForTemplate(nilStr)
	od, ok := got.(*OrderedDict)
	if !ok {
		t.Fatalf("nil map[string]any → %T, want *OrderedDict (typed nil)", got)
	}
	if od != nil {
		t.Errorf("nil map[string]any should yield typed-nil *OrderedDict, got %#v", od)
	}
}
