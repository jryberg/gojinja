package native

import (
	"context"
	"reflect"
	"testing"

	"github.com/jryberg/gojinja/pkg/environment"
)

func mustEnv(t *testing.T) *environment.Environment {
	t.Helper()
	e, err := environment.New(environment.WithAutoescape(environment.AutoescapeNever{}))
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestConcatTypes(t *testing.T) {
	cases := []struct {
		in   string
		want any
	}{
		{"42", int64(42)},
		{"3.14", 3.14},
		{"true", true},
		{"false", false},
		{"null", nil},
		{`"hi"`, "hi"},
		{"[1, 2, 3]", []any{int64(1), int64(2), int64(3)}},
		{`{"a": 1}`, map[string]any{"a": int64(1)}},
		{"hello world", "hello world"}, // not a literal — stays string
	}
	for _, c := range cases {
		got := Concat(c.in)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("Concat(%q) = %v(%T), want %v(%T)", c.in, got, got, c.want, c.want)
		}
	}
}

func TestRenderReturnsTypedValue(t *testing.T) {
	e := mustEnv(t)
	tpl, err := e.FromString("{{ a + b }}")
	if err != nil {
		t.Fatal(err)
	}
	v, err := Render(context.Background(), tpl, map[string]any{"a": 2, "b": 3})
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := v.(int64); !ok || got != 5 {
		t.Fatalf("got %v(%T), want int64(5)", v, v)
	}
}
