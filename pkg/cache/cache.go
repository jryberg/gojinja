// Package cache stores parsed template ASTs keyed by name. Two
// implementations are provided: [Memory] (LRU, in-process) and
// [Filesystem] (atomic temp+rename writes, magic+checksum header).
//
// gojinja's cache layer is the moral equivalent of Jinja2's
// `BytecodeCache`, but the payload is our own gob-encoded AST — never a
// Python-runtime serialization format. The on-disk format is:
//
//	[16 bytes magic+version]  "gojinja-ast-v1\n\n"
//	[64 bytes hex SHA-256 source checksum]
//	[gob-encoded *ast.Template]
//
// On read, the source's current checksum is recomputed and compared.
// Any mismatch makes the cached entry invalid (treated as a miss).
package cache

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/jryberg/gojinja/pkg/ast"
)

// magic is the header prefix written by [Filesystem]. Bumping the version
// suffix forces a full re-cache of the corpus.
const magic = "gojinja-ast-v1\n\n"

// SourceChecksum returns the canonical hex SHA-256 of source.
func SourceChecksum(source string) string {
	sum := sha256.Sum256([]byte(source))
	return hex.EncodeToString(sum[:])
}

// CacheKey returns a stable filesystem-safe key for (name, filename).
func CacheKey(name, filename string) string {
	sum := sha256.Sum256([]byte(name + "|" + filename))
	return hex.EncodeToString(sum[:16]) // 16 bytes = 32 hex chars
}

// Cache is the storage interface. Implementations must be safe for
// concurrent use.
type Cache interface {
	// Get returns the cached template if checksum matches.
	Get(key, checksum string) (*ast.Template, bool)
	// Put stores tpl under key with the supplied checksum.
	Put(key, checksum string, tpl *ast.Template) error
}

// =============================================================== Memory

// Memory is an in-process bounded cache. Eviction is naive (drops an
// arbitrary entry when over capacity); good enough until profiling
// indicates true LRU is needed.
//
// Example:
//
//	env, _ := environment.New(environment.WithExternalCache(
//	    cache.NewMemory(1000),
//	))
type Memory struct {
	cap int
	mu  sync.Mutex
	m   map[string]*memEntry
}

type memEntry struct {
	checksum string
	tpl      *ast.Template
}

// NewMemory returns a Memory with the given capacity. Capacity ≤ 0 is
// treated as 1 — gojinja never permits unbounded caches.
func NewMemory(capacity int) *Memory {
	if capacity <= 0 {
		capacity = 1
	}
	return &Memory{cap: capacity, m: map[string]*memEntry{}}
}

// Get implements [Cache].
func (m *Memory) Get(key, checksum string) (*ast.Template, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.m[key]
	if !ok || e.checksum != checksum {
		return nil, false
	}
	return e.tpl, true
}

// Put implements [Cache].
func (m *Memory) Put(key, checksum string, tpl *ast.Template) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.m) >= m.cap {
		for k := range m.m {
			delete(m.m, k)
			break
		}
	}
	m.m[key] = &memEntry{checksum: checksum, tpl: tpl}
	return nil
}

// =============================================================== Filesystem

// Filesystem stores cache entries under a directory using atomic
// temp+rename writes. Mismatched checksums on read are silently ignored
// (treated as misses), so a tampered or stale file just causes a recompile.
//
// Cache files start with the 16-byte magic header `gojinja-ast-v1\n\n`
// followed by the 64-char hex SHA-256 of the source, then the
// `gob`-encoded `*ast.Template`. See the package doc for the exact
// layout.
//
// Example:
//
//	c, _ := cache.NewFilesystem("/var/cache/myapp/templates")
//	env, _ := environment.New(environment.WithExternalCache(c))
type Filesystem struct {
	dir     string
	pattern string
	mu      sync.Mutex
}

// defaultFilenamePattern is used when no SetPattern call has been made.
// `%s` is replaced by the cache key.
const defaultFilenamePattern = "__gojinja_%s.cache"

// NewFilesystem returns a Filesystem cache rooted at dir, creating dir
// (and any parents) if it doesn't exist. Cache files use a default
// filename pattern; call [Filesystem.SetPattern] to override.
func NewFilesystem(dir string) (*Filesystem, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &Filesystem{dir: dir, pattern: defaultFilenamePattern}, nil
}

// SetPattern sets the filename pattern used by this cache. Pass an empty
// string to restore the default. The pattern must contain exactly one
// `%s`, which is replaced by the cache key. Returns the receiver so it
// can chain in builder-style code.
func (f *Filesystem) SetPattern(pattern string) *Filesystem {
	f.mu.Lock()
	defer f.mu.Unlock()
	if pattern == "" {
		pattern = defaultFilenamePattern
	}
	f.pattern = pattern
	return f
}

// Get implements [Cache].
func (f *Filesystem) Get(key, checksum string) (*ast.Template, bool) {
	path := filepath.Join(f.dir, fmt.Sprintf(f.pattern, key))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	if len(data) < len(magic)+64 {
		return nil, false
	}
	if string(data[:len(magic)]) != magic {
		return nil, false
	}
	stored := string(data[len(magic) : len(magic)+64])
	if stored != checksum {
		return nil, false
	}
	payload := data[len(magic)+64:]
	var tpl ast.Template
	dec := gob.NewDecoder(bytes.NewReader(payload))
	if err := dec.Decode(&tpl); err != nil {
		return nil, false
	}
	return &tpl, true
}

// Put implements [Cache] using atomic temp+rename.
func (f *Filesystem) Put(key, checksum string, tpl *ast.Template) error {
	if len(checksum) != 64 {
		return errors.New("cache: checksum must be 64-char hex")
	}
	var payload bytes.Buffer
	enc := gob.NewEncoder(&payload)
	if err := enc.Encode(tpl); err != nil {
		return err
	}
	full := bytes.Buffer{}
	full.WriteString(magic)
	full.WriteString(checksum)
	full.Write(payload.Bytes())

	f.mu.Lock()
	defer f.mu.Unlock()
	tmp, err := os.CreateTemp(f.dir, ".gojinja-*.tmp")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(full.Bytes()); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	finalPath := filepath.Join(f.dir, fmt.Sprintf(f.pattern, key))
	if err := os.Rename(tmp.Name(), finalPath); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return nil
}

// Clear removes every cache file under f's directory matching f.pattern.
func (f *Filesystem) Clear() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	entries, err := os.ReadDir(f.dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	prefix, suffix := splitPattern(f.pattern)
	for _, e := range entries {
		name := e.Name()
		if len(name) < len(prefix)+len(suffix) {
			continue
		}
		if name[:len(prefix)] != prefix || name[len(name)-len(suffix):] != suffix {
			continue
		}
		_ = os.Remove(filepath.Join(f.dir, name))
	}
	return nil
}

func splitPattern(p string) (prefix, suffix string) {
	for i := 0; i < len(p)-1; i++ {
		if p[i] == '%' && p[i+1] == 's' {
			return p[:i], p[i+2:]
		}
	}
	return p, ""
}
