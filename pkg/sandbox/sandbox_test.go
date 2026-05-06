package sandbox

import "testing"

func TestIsSafeAttributeRejectsUnderscore(t *testing.T) {
	if IsSafeAttribute(struct{}{}, "_x") {
		t.Fatal("underscore-prefixed should be unsafe")
	}
	if IsSafeAttribute(struct{}{}, "") {
		t.Fatal("empty should be unsafe")
	}
}

func TestIsSafeAttributeAllowsRegular(t *testing.T) {
	if !IsSafeAttribute(struct{}{}, "Field") {
		t.Fatal("regular name should be safe")
	}
}

func TestIsSafeCallableUnsafeWrapper(t *testing.T) {
	if IsSafeCallable(UnsafeFunc{Fn: func() {}}) {
		t.Fatal("UnsafeFunc should not be callable")
	}
}

func TestIsSafeCallableNil(t *testing.T) {
	if IsSafeCallable(nil) {
		t.Fatal("nil should not be callable")
	}
}

func TestIsMutatingMethod(t *testing.T) {
	cases := map[string]bool{
		"append": true,
		"clear":  true,
		"sort":   true,
		"length": false,
		"foo":    false,
	}
	for k, want := range cases {
		if got := IsMutatingMethod(k); got != want {
			t.Errorf("IsMutatingMethod(%q) = %v, want %v", k, got, want)
		}
	}
}
