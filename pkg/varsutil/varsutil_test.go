package varsutil

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestNormalizeJSONNumbers_Scalars(t *testing.T) {
	cases := []struct {
		in   any
		want any
	}{
		{json.Number("1"), int64(1)},
		{json.Number("-130000"), int64(-130000)},
		{json.Number("1.0"), 1.0},
		{json.Number("1.5"), 1.5},
		{json.Number("1e3"), 1000.0},
		{json.Number("not-a-number"), "not-a-number"},
		{"plain string", "plain string"},
		{true, true},
		{nil, nil},
	}
	for _, c := range cases {
		got := NormalizeJSONNumbers(c.in)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("NormalizeJSONNumbers(%v) = %v (%T), want %v (%T)", c.in, got, got, c.want, c.want)
		}
	}
}

func TestNormalizeJSONNumbers_Recursive(t *testing.T) {
	in := map[string]any{
		"int":   json.Number("130000"),
		"float": json.Number("130000.0"),
		"nested": map[string]any{
			"list": []any{json.Number("1"), json.Number("2.5"), "x"},
		},
	}
	want := map[string]any{
		"int":   int64(130000),
		"float": 130000.0,
		"nested": map[string]any{
			"list": []any{int64(1), 2.5, "x"},
		},
	}
	got := NormalizeJSONNumbers(in)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestJSONVars_PythonAligned(t *testing.T) {
	raw := []byte(`{"n_int": 130000, "n_float": 130000.0, "nested": {"items": [1, 2.5]}}`)
	got, err := JSONVars(raw)
	if err != nil {
		t.Fatal(err)
	}
	if v := got["n_int"]; v != int64(130000) {
		t.Errorf("n_int: got %v (%T), want int64(130000)", v, v)
	}
	if v := got["n_float"]; v != 130000.0 {
		t.Errorf("n_float: got %v (%T), want float64(130000)", v, v)
	}
	nested := got["nested"].(map[string]any)
	items := nested["items"].([]any)
	if items[0] != int64(1) {
		t.Errorf("items[0]: got %v (%T), want int64(1)", items[0], items[0])
	}
	if items[1] != 2.5 {
		t.Errorf("items[1]: got %v (%T), want float64(2.5)", items[1], items[1])
	}
}

func TestJSONVars_NonObjectErrors(t *testing.T) {
	if _, err := JSONVars([]byte(`[1, 2, 3]`)); err == nil {
		t.Fatal("expected error for JSON array")
	}
	if _, err := JSONVars([]byte(`42`)); err == nil {
		t.Fatal("expected error for JSON scalar")
	}
	if _, err := JSONVars([]byte(`{nope`)); err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}
