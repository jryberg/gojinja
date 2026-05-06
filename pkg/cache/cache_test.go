package cache

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jryberg/gojinja/pkg/ast"
)

func sampleTemplate() *ast.Template {
	tpl := &ast.Template{
		Body: []ast.Node{
			&ast.Output{Nodes: []ast.Expr{&ast.Const{Value: "hi"}}},
		},
	}
	return tpl
}

func TestSourceChecksumStable(t *testing.T) {
	a := SourceChecksum("hello")
	b := SourceChecksum("hello")
	c := SourceChecksum("world")
	if a != b {
		t.Fatal("identical inputs produced different checksums")
	}
	if a == c {
		t.Fatal("different inputs produced identical checksums")
	}
	if len(a) != 64 {
		t.Fatalf("checksum len = %d, want 64", len(a))
	}
}

func TestCacheKeyDeterministic(t *testing.T) {
	if CacheKey("a", "b") != CacheKey("a", "b") {
		t.Fatal("non-deterministic key")
	}
	if CacheKey("a", "b") == CacheKey("a", "c") {
		t.Fatal("filename ignored")
	}
}

// =============================================================== Memory

func TestMemoryRoundTrip(t *testing.T) {
	m := NewMemory(8)
	tpl := sampleTemplate()
	if err := m.Put("k1", "checksum1", tpl); err != nil {
		t.Fatal(err)
	}
	got, ok := m.Get("k1", "checksum1")
	if !ok {
		t.Fatal("Get reported miss")
	}
	if got != tpl {
		t.Fatal("Get returned different template")
	}
}

func TestMemoryChecksumMismatchMisses(t *testing.T) {
	m := NewMemory(8)
	_ = m.Put("k", "v1", sampleTemplate())
	if _, ok := m.Get("k", "v2"); ok {
		t.Fatal("expected miss on checksum mismatch")
	}
}

func TestMemoryEviction(t *testing.T) {
	m := NewMemory(2)
	_ = m.Put("a", "x", sampleTemplate())
	_ = m.Put("b", "x", sampleTemplate())
	_ = m.Put("c", "x", sampleTemplate())
	if len(m.m) > 2 {
		t.Fatalf("expected ≤2 entries, got %d", len(m.m))
	}
}

func TestMemoryRejectsNonPositiveCap(t *testing.T) {
	m := NewMemory(0)
	if m.cap < 1 {
		t.Fatal("capacity not clamped")
	}
}

// =============================================================== Filesystem

func TestFilesystemRoundTrip(t *testing.T) {
	dir := t.TempDir()
	fsCache, err := NewFilesystem(dir)
	if err != nil {
		t.Fatal(err)
	}
	tpl := sampleTemplate()
	checksum := SourceChecksum("source")
	if err := fsCache.Put("abc", checksum, tpl); err != nil {
		t.Fatal(err)
	}
	got, ok := fsCache.Get("abc", checksum)
	if !ok {
		t.Fatal("Get reported miss")
	}
	if got == nil || len(got.Body) != 1 {
		t.Fatalf("Get returned %#v", got)
	}
}

func TestFilesystemChecksumMismatch(t *testing.T) {
	dir := t.TempDir()
	fsCache, _ := NewFilesystem(dir)
	_ = fsCache.Put("abc", SourceChecksum("v1"), sampleTemplate())
	if _, ok := fsCache.Get("abc", SourceChecksum("v2")); ok {
		t.Fatal("expected miss on checksum mismatch")
	}
}

func TestFilesystemTamperedFileRejected(t *testing.T) {
	dir := t.TempDir()
	fsCache, _ := NewFilesystem(dir)
	checksum := SourceChecksum("source")
	if err := fsCache.Put("abc", checksum, sampleTemplate()); err != nil {
		t.Fatal(err)
	}
	// Overwrite the file with garbage; Get must report a miss.
	files, _ := os.ReadDir(dir)
	if len(files) != 1 {
		t.Fatalf("expected 1 cache file, got %d", len(files))
	}
	full := filepath.Join(dir, files[0].Name())
	if err := os.WriteFile(full, []byte("garbage"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := fsCache.Get("abc", checksum); ok {
		t.Fatal("expected miss on tampered file")
	}
}

func TestFilesystemAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	fsCache, _ := NewFilesystem(dir)
	if err := fsCache.Put("k", SourceChecksum("v"), sampleTemplate()); err != nil {
		t.Fatal(err)
	}
	files, _ := os.ReadDir(dir)
	for _, f := range files {
		// No leftover .tmp files.
		if filepath.Ext(f.Name()) == ".tmp" {
			t.Fatalf("temp file leaked: %s", f.Name())
		}
	}
}

func TestFilesystemClear(t *testing.T) {
	dir := t.TempDir()
	fsCache, _ := NewFilesystem(dir)
	_ = fsCache.Put("a", SourceChecksum("v"), sampleTemplate())
	_ = fsCache.Put("b", SourceChecksum("v"), sampleTemplate())
	if err := fsCache.Clear(); err != nil {
		t.Fatal(err)
	}
	if _, ok := fsCache.Get("a", SourceChecksum("v")); ok {
		t.Fatal("Get should miss after Clear")
	}
}
