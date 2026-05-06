package loader

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gjerrors "github.com/jryberg/gojinja/pkg/errors"
)

// =============================================================== SplitPath

func TestSplitPathRejectsTraversal(t *testing.T) {
	cases := []string{
		"../etc/passwd",
		"a/../b",
		"/abs/path",
		"foo/../../etc",
		"a\x00b",
		`C:\windows`,
		`a\b`,
		"",
	}
	for _, c := range cases {
		if _, err := SplitPath(c); err == nil {
			t.Errorf("SplitPath(%q) should fail", c)
		}
	}
}

func TestSplitPathAccepts(t *testing.T) {
	cases := map[string][]string{
		"foo.html":             {"foo.html"},
		"a/b/c.html":           {"a", "b", "c.html"},
		"./a.html":             {"a.html"},
		"a//b.html":            {"a", "b.html"},
		"./a/./b.html":         {"a", "b.html"},
	}
	for in, want := range cases {
		got, err := SplitPath(in)
		if err != nil {
			t.Errorf("SplitPath(%q) err=%v", in, err)
			continue
		}
		if strings.Join(got, "/") != strings.Join(want, "/") {
			t.Errorf("SplitPath(%q) = %v, want %v", in, got, want)
		}
	}
}

// =============================================================== Dict

func TestDictLoaderHit(t *testing.T) {
	d := Dict{"hello.html": "<h1>{{ x }}</h1>"}
	s, err := d.GetSource("hello.html")
	if err != nil {
		t.Fatal(err)
	}
	if s.Code != "<h1>{{ x }}</h1>" {
		t.Fatal(s.Code)
	}
	if !s.Uptodate() {
		t.Fatal("uptodate should be true initially")
	}
}

func TestDictLoaderMiss(t *testing.T) {
	d := Dict{}
	_, err := d.GetSource("nope")
	if err == nil {
		t.Fatal("expected not-found")
	}
}

func TestDictLoaderInvalidPath(t *testing.T) {
	d := Dict{"x": "ok"}
	_, err := d.GetSource("../x")
	if err == nil {
		t.Fatal("expected error for traversal name")
	}
}

// =============================================================== Choice

func TestChoiceFallsThrough(t *testing.T) {
	a := Dict{"a.html": "A"}
	b := Dict{"b.html": "B"}
	c := Choice{Loaders: []Loader{a, b}}
	s, err := c.GetSource("b.html")
	if err != nil {
		t.Fatal(err)
	}
	if s.Code != "B" {
		t.Fatalf("got %q", s.Code)
	}
}

// =============================================================== Prefix

func TestPrefix(t *testing.T) {
	p := Prefix{
		Mapping: map[string]Loader{
			"app": Dict{"index.html": "X"},
		},
	}
	s, err := p.GetSource("app/index.html")
	if err != nil {
		t.Fatal(err)
	}
	if s.Code != "X" {
		t.Fatalf("got %q", s.Code)
	}
}

// =============================================================== FileSystem

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFileSystemHit(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.html", "Hello")
	fs, err := NewFileSystem([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	s, err := fs.GetSource("a.html")
	if err != nil {
		t.Fatal(err)
	}
	if s.Code != "Hello" {
		t.Fatalf("got %q", s.Code)
	}
}

func TestFileSystemRejectsTraversalName(t *testing.T) {
	dir := t.TempDir()
	fs, err := NewFileSystem([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	_, err = fs.GetSource("../etc/passwd")
	if err == nil {
		t.Fatal("expected not-found / invalid")
	}
}

func TestFileSystemRejectsSymlinkEscape(t *testing.T) {
	dir := t.TempDir()
	out := t.TempDir()
	writeFile(t, out, "secret.txt", "secret")
	if err := os.Symlink(filepath.Join(out, "secret.txt"), filepath.Join(dir, "leak")); err != nil {
		t.Skip("symlinks not supported on this filesystem")
	}
	fs, err := NewFileSystem([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	_, err = fs.GetSource("leak")
	if err == nil {
		t.Fatal("expected SecurityError on symlink without follow-links")
	}
}

func TestFileSystemNotFoundOmitsAbsPath(t *testing.T) {
	// Security: a TemplateNotFound for an unknown name must reveal only
	// the requested name, not the absolute filesystem path of the
	// loader root. (Files inside the root are allowed to surface their
	// path in syntax errors — Python Jinja2 does the same — but a miss
	// must not.)
	dir := t.TempDir()
	fs, err := NewFileSystem([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	_, err = fs.GetSource("does-not-exist.j2")
	if err == nil {
		t.Fatal("expected error for missing template")
	}
	if strings.Contains(err.Error(), dir) {
		t.Fatalf("error message leaks loader root %q: %v", dir, err)
	}
}

// =============================================================== Cached

func TestCachedRespectsUptodate(t *testing.T) {
	calls := 0
	stale := false
	inner := FuncLoader{
		Fetch: func(name string) (Source, bool, error) {
			calls++
			return Source{Code: "v" + string(rune('0'+calls)), Uptodate: func() bool { return !stale }}, true, nil
		},
	}
	c := NewCached(inner)
	s, _ := c.GetSource("x")
	if s.Code != "v1" || calls != 1 {
		t.Fatalf("first: %q calls=%d", s.Code, calls)
	}
	// Same call hits cache.
	s, _ = c.GetSource("x")
	if calls != 1 {
		t.Fatalf("expected cache hit, got %d calls", calls)
	}
	// Mark stale, expect refetch.
	stale = true
	s, _ = c.GetSource("x")
	if calls != 2 {
		t.Fatalf("expected refetch, got %d calls", calls)
	}
	stale = false
}

// =============================================================== TemplateNotFound type matching

func TestNotFoundIsTemplateNotFound(t *testing.T) {
	d := Dict{}
	_, err := d.GetSource("nope")
	var nf *gjerrors.TemplateNotFound
	if !errors.As(err, &nf) {
		t.Fatalf("expected *TemplateNotFound, got %T (%v)", err, err)
	}
}
