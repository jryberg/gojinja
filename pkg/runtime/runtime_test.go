package runtime

import (
	stderrors "errors"
	"strings"
	"testing"

	gjerrors "github.com/jryberg/gojinja/pkg/errors"
)

// --------------------------------------------------------------------- missing

func TestMissingSentinel(t *testing.T) {
	if !IsMissing(Missing) {
		t.Error("IsMissing(Missing) = false")
	}
	if IsMissing(nil) {
		t.Error("IsMissing(nil) = true")
	}
	if got := Missing.String(); got != "missing" {
		t.Errorf("Missing.String() = %q, want \"missing\"", got)
	}
}

// --------------------------------------------------------------------- evalctx

type fakeEnv struct{ autoescape bool }

func (f fakeEnv) AutoescapeFor(string) bool { return f.autoescape }

func TestEvalContextNew(t *testing.T) {
	ec := NewEvalContext(fakeEnv{autoescape: true}, "page.html")
	if !ec.Autoescape {
		t.Error("Autoescape not propagated from env")
	}
	if ec.Volatile {
		t.Error("Volatile should default to false")
	}
	if ec.Name != "page.html" {
		t.Errorf("Name = %q", ec.Name)
	}
}

func TestEvalContextSaveRevert(t *testing.T) {
	ec := NewEvalContext(fakeEnv{autoescape: false}, "")
	saved := ec.Save()
	ec.Autoescape = true
	ec.Volatile = true
	ec.Revert(saved)
	if ec.Autoescape || ec.Volatile {
		t.Errorf("Revert did not restore: %#v", ec)
	}
}

// --------------------------------------------------------------------- namespace

func TestNamespaceSetGetItems(t *testing.T) {
	ns := NewNamespace(map[string]any{"a": 1})
	ns.Set("b", "two")
	if v, ok := ns.Get("a"); !ok || v != 1 {
		t.Errorf("Get(a) = %v,%v", v, ok)
	}
	if v, ok := ns.Get("b"); !ok || v != "two" {
		t.Errorf("Get(b) = %v,%v", v, ok)
	}
	if _, ok := ns.Get("missing"); ok {
		t.Error("Get(missing) should return false")
	}
	items := ns.Items()
	if len(items) != 2 || items[0].Name != "a" || items[1].Name != "b" {
		t.Errorf("Items not sorted: %#v", items)
	}
}

func TestNamespaceStringFormat(t *testing.T) {
	ns := NewNamespace(map[string]any{"k": "v"})
	got := ns.String()
	if !strings.Contains(got, "<Namespace ") || !strings.Contains(got, `"k"`) {
		t.Errorf("Namespace.String wrong shape: %q", got)
	}
}

// --------------------------------------------------------------------- undefined

// Mirrors jinja2 doctest: str(Undefined(name='foo')) == ''
func TestUndefinedBaseStringEmpty(t *testing.T) {
	u := NewBase("", "foo", nil, nil)
	if got := u.String(); got != "" {
		t.Errorf("Base.String() = %q, want empty", got)
	}
}

func TestUndefinedBaseBoolFalse(t *testing.T) {
	u := NewBase("", "foo", nil, nil)
	b, err := u.Bool()
	if err != nil || b {
		t.Errorf("Base.Bool() = %v,%v, want (false, nil)", b, err)
	}
}

func TestUndefinedBaseFailReturnsUndefinedError(t *testing.T) {
	u := NewBase("", "foo", nil, nil)
	err := u.Fail()
	var ue *gjerrors.UndefinedError
	if !stderrors.As(err, &ue) {
		t.Fatalf("Fail() did not return *UndefinedError, got %T", err)
	}
	if !strings.Contains(ue.Message, "foo") {
		t.Errorf("message missing name: %q", ue.Message)
	}
}

func TestUndefinedHintWins(t *testing.T) {
	u := NewBase("custom hint", "foo", nil, nil)
	if !strings.Contains(u.Message(), "custom hint") {
		t.Errorf("hint not used: %q", u.Message())
	}
}

func TestUndefinedAttrErrorMessage(t *testing.T) {
	u := NewBase("", "bar", "string-obj", nil)
	if !strings.Contains(u.Message(), "has no attribute") {
		t.Errorf("attribute message wrong: %q", u.Message())
	}
}

// Mirrors jinja2 ChainableUndefined doctest:
// foo.bar['baz'] returns the same undefined.
func TestUndefinedChainableGetAttrItemReturnsSelf(t *testing.T) {
	u := NewChainable("", "foo", nil, nil)
	v, err := u.GetAttr("bar")
	if err != nil {
		t.Fatalf("GetAttr returned err: %v", err)
	}
	got, ok := v.(Undefined)
	if !ok || got.Mode != ModeChainable {
		t.Fatalf("GetAttr returned %#v", v)
	}
	v2, err := got.GetItem("baz")
	if err != nil {
		t.Fatalf("GetItem returned err: %v", err)
	}
	if _, ok := v2.(Undefined); !ok {
		t.Fatalf("GetItem returned %#v", v2)
	}
}

// Base undefined fails on getattr.
func TestUndefinedBaseGetAttrFails(t *testing.T) {
	u := NewBase("", "foo", nil, nil)
	if _, err := u.GetAttr("anything"); err == nil {
		t.Error("expected error from base.GetAttr")
	}
}

// Mirrors jinja2 DebugUndefined: str(DebugUndefined(name='foo')) == '{{ foo }}'
func TestUndefinedDebugString(t *testing.T) {
	u := NewDebug("", "foo", nil, nil)
	if got := u.String(); got != "{{ foo }}" {
		t.Errorf("Debug.String() = %q, want \"{{ foo }}\"", got)
	}
}

func TestUndefinedDebugStringWithHint(t *testing.T) {
	u := NewDebug("custom", "foo", nil, nil)
	if got := u.String(); !strings.Contains(got, "custom") {
		t.Errorf("Debug.String with hint = %q", got)
	}
}

// Mirrors StrictUndefined: str() and bool() raise.
func TestUndefinedStrictBoolErrors(t *testing.T) {
	u := NewStrict("", "foo", nil, nil)
	if _, err := u.Bool(); err == nil {
		t.Error("Strict.Bool() should error")
	}
}

func TestUndefinedStrictEqualErrors(t *testing.T) {
	u := NewStrict("", "foo", nil, nil)
	if _, err := u.Equal(u); err == nil {
		t.Error("Strict.Equal() should error")
	}
}

// IsUndefined dispatch
func TestIsUndefinedDetectsAllVariants(t *testing.T) {
	cases := []any{
		NewBase("", "x", nil, nil),
		NewChainable("", "x", nil, nil),
		NewDebug("", "x", nil, nil),
		NewStrict("", "x", nil, nil),
	}
	for _, c := range cases {
		if !IsUndefined(c) {
			t.Errorf("IsUndefined(%T) = false", c)
		}
	}
}

func TestIsUndefinedFalseForOtherTypes(t *testing.T) {
	for _, v := range []any{nil, "", 0, false, []any{}} {
		if IsUndefined(v) {
			t.Errorf("IsUndefined(%v) = true", v)
		}
	}
}

// __eq__ semantics: two undefineds of the same Mode are equal.
func TestUndefinedEqualityByMode(t *testing.T) {
	a := NewBase("", "a", nil, nil)
	b := NewBase("", "b", nil, nil)
	eq, err := a.Equal(b)
	if err != nil || !eq {
		t.Errorf("Equal across same Mode = %v,%v", eq, err)
	}
	c := NewChainable("", "c", nil, nil)
	eq, err = a.Equal(c)
	if err != nil || eq {
		t.Errorf("Equal across modes = %v,%v", eq, err)
	}
}

// Custom exception constructor (sandbox uses SecurityError instead of UndefinedError)
func TestUndefinedCustomExc(t *testing.T) {
	u := NewBase("blocked", "x", nil, func(msg string) error {
		return gjerrors.NewSecurityError(msg)
	})
	err := u.Fail()
	var se *gjerrors.SecurityError
	if !stderrors.As(err, &se) {
		t.Fatalf("expected SecurityError, got %T", err)
	}
	if se.Message != "blocked" {
		t.Errorf("message = %q", se.Message)
	}
}

// --------------------------------------------------------------------- passarg

func TestPassArgString(t *testing.T) {
	cases := map[PassArg]string{
		PassNone:        "pass_none",
		PassContext:     "pass_context",
		PassEvalContext: "pass_eval_context",
		PassEnvironment: "pass_environment",
	}
	for k, want := range cases {
		if got := k.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", k, got, want)
		}
	}
}
