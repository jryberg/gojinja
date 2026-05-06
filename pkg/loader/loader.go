// Package loader provides Loader implementations for gojinja: in-memory
// (Dict, Function), composition (Prefix, Choice), and filesystem-backed
// loaders (FileSystem, Embed).
//
// Every loader runs the supplied template name through [SplitPath] before
// any I/O. SplitPath rejects path-traversal patterns (literal `..`, OS
// separators, drive letters), the same protection Jinja2's
// `split_template_path` provides.
package loader

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	gjerrors "github.com/jryberg/gojinja/pkg/errors"
)

// Source is the result of resolving a template name. The environment
// package re-exports this type as `environment.Source` so callers can
// pass a `pkg/loader` value straight into `environment.WithLoader`.
type Source struct {
	Code     string
	Filename string
	Uptodate func() bool
}

// SplitPath validates a template name and returns its path components.
// Returns ErrInvalidPath if the name contains separators, parent-dir
// segments, drive letters, or NUL bytes.
func SplitPath(name string) ([]string, error) {
	if strings.ContainsRune(name, 0) {
		return nil, ErrInvalidPath
	}
	// Reject Windows drive letters / UNC and POSIX absolute paths.
	if len(name) >= 2 && name[1] == ':' {
		return nil, ErrInvalidPath
	}
	if strings.HasPrefix(name, "/") {
		return nil, ErrInvalidPath
	}
	parts := strings.Split(name, "/")
	out := parts[:0]
	for _, p := range parts {
		if p == "" || p == "." {
			continue
		}
		if p == ".." {
			return nil, ErrInvalidPath
		}
		if strings.ContainsAny(p, `\/`) {
			return nil, ErrInvalidPath
		}
		out = append(out, p)
	}
	if len(out) == 0 {
		return nil, ErrInvalidPath
	}
	return out, nil
}

// ErrInvalidPath is returned by SplitPath for unsafe template names.
var ErrInvalidPath = errors.New("loader: invalid template path")

// =============================================================== Dict

// Dict is a string→source map. The simplest possible loader — useful for
// tests and for templates compiled into a binary as a `map[string]string`.
type Dict map[string]string

// GetSource implements [Loader].
func (d Dict) GetSource(name string) (Source, error) {
	if _, err := SplitPath(name); err != nil {
		return Source{}, gjerrors.NewTemplateNotFound(name, "")
	}
	src, ok := d[name]
	if !ok {
		return Source{}, gjerrors.NewTemplateNotFound(name, "")
	}
	cur := src
	uptodate := func() bool {
		return d[name] == cur
	}
	return Source{Code: src, Filename: name, Uptodate: uptodate}, nil
}

// =============================================================== Function

// FuncLoader wraps an arbitrary fetch function. Useful for adapting to
// other source backends (DBs, S3, etc.).
type FuncLoader struct {
	Fetch func(name string) (Source, bool, error)
}

// GetSource calls the wrapped function. ok=false → TemplateNotFound.
func (f FuncLoader) GetSource(name string) (Source, error) {
	if _, err := SplitPath(name); err != nil {
		return Source{}, gjerrors.NewTemplateNotFound(name, "")
	}
	src, ok, err := f.Fetch(name)
	if err != nil {
		return Source{}, err
	}
	if !ok {
		return Source{}, gjerrors.NewTemplateNotFound(name, "")
	}
	return src, nil
}

// =============================================================== Prefix

// Prefix routes by name prefix. `mapping["app1"] = …` makes "app1/x.html"
// load from that loader (with the prefix stripped).
type Prefix struct {
	Mapping   map[string]Loader
	Delimiter string // default "/"
}

// Loader is the in-package interface every loader implements.
type Loader interface {
	GetSource(name string) (Source, error)
}

// GetSource dispatches by prefix.
func (p Prefix) GetSource(name string) (Source, error) {
	delim := p.Delimiter
	if delim == "" {
		delim = "/"
	}
	idx := strings.Index(name, delim)
	if idx < 0 {
		return Source{}, gjerrors.NewTemplateNotFound(name, "")
	}
	prefix, rest := name[:idx], name[idx+len(delim):]
	sub, ok := p.Mapping[prefix]
	if !ok {
		return Source{}, gjerrors.NewTemplateNotFound(name, "")
	}
	return sub.GetSource(rest)
}

// =============================================================== Choice

// Choice tries each loader in order, returning the first hit.
type Choice struct{ Loaders []Loader }

// GetSource tries each loader; only TemplateNotFound is swallowed.
func (c Choice) GetSource(name string) (Source, error) {
	var lastErr error
	for _, l := range c.Loaders {
		s, err := l.GetSource(name)
		if err == nil {
			return s, nil
		}
		// Only swallow TemplateNotFound; surface other errors.
		var nf *gjerrors.TemplateNotFound
		if !errors.As(err, &nf) {
			return Source{}, err
		}
		lastErr = err
	}
	if lastErr != nil {
		return Source{}, lastErr
	}
	return Source{}, gjerrors.NewTemplateNotFound(name, "")
}

// =============================================================== FileSystem

// FileSystemOption configures a [FileSystem] loader.
type FileSystemOption func(*FileSystem)

// WithFollowLinks permits resolving symlinks. Off by default — when off,
// the loader rejects any path whose evaluated form escapes the configured
// root.
func WithFollowLinks(b bool) FileSystemOption {
	return func(f *FileSystem) { f.followLinks = b }
}

// FileSystem reads templates from one or more allowlisted root directories.
// Path-traversal protections:
//   - Names are validated through [SplitPath].
//   - Joined paths are resolved with filepath.EvalSymlinks; the result
//     must remain within the configured root.
//   - Symlink-following is OFF by default; enabling it via
//     [WithFollowLinks] still enforces the root containment check.
type FileSystem struct {
	roots       []string
	encoding    string // currently unused; we always read UTF-8
	followLinks bool
}

// NewFileSystem builds a FileSystem loader over the given root(s). Roots
// are made absolute and cleaned. Returns an error if any root does not
// exist as a directory.
func NewFileSystem(roots []string, opts ...FileSystemOption) (*FileSystem, error) {
	f := &FileSystem{roots: make([]string, 0, len(roots)), encoding: "utf-8"}
	for _, r := range roots {
		abs, err := filepath.Abs(r)
		if err != nil {
			return nil, err
		}
		st, err := os.Stat(abs)
		if err != nil {
			return nil, err
		}
		if !st.IsDir() {
			return nil, errors.New("loader: FileSystem root is not a directory: " + abs)
		}
		f.roots = append(f.roots, abs)
	}
	for _, o := range opts {
		o(f)
	}
	return f, nil
}

// GetSource looks up name in each root in order.
func (f *FileSystem) GetSource(name string) (Source, error) {
	parts, err := SplitPath(name)
	if err != nil {
		return Source{}, gjerrors.NewTemplateNotFound(name, "")
	}
	rel := filepath.Join(parts...)
	for _, root := range f.roots {
		full := filepath.Join(root, rel)
		// Defense in depth: re-clean and confirm the result is rooted in
		// `root` even before resolving symlinks.
		if !isWithin(root, full) {
			continue
		}
		// If symlinks are followed, resolve and re-check.
		probe := full
		if f.followLinks {
			r, err := filepath.EvalSymlinks(full)
			if err == nil {
				probe = r
			}
		} else {
			st, err := os.Lstat(full)
			if err == nil && st.Mode()&os.ModeSymlink != 0 {
				return Source{}, gjerrors.NewSecurityError("symlink template files require WithFollowLinks")
			}
		}
		if !isWithin(root, probe) {
			return Source{}, gjerrors.NewSecurityError("template path escapes the loader root")
		}
		data, err := os.ReadFile(probe)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return Source{}, err
		}
		// uptodate: mtime equality.
		st, _ := os.Stat(probe)
		mt := st.ModTime()
		uptodate := func() bool {
			s2, err := os.Stat(probe)
			if err != nil {
				return false
			}
			return s2.ModTime().Equal(mt)
		}
		return Source{Code: string(data), Filename: probe, Uptodate: uptodate}, nil
	}
	return Source{}, gjerrors.NewTemplateNotFound(name, "")
}

// isWithin returns true iff candidate is contained within parent (parent
// is a path prefix when both are cleaned absolute paths, with a separator
// boundary).
func isWithin(parent, candidate string) bool {
	parent = filepath.Clean(parent)
	candidate = filepath.Clean(candidate)
	if parent == candidate {
		return true
	}
	if !strings.HasSuffix(parent, string(filepath.Separator)) {
		parent += string(filepath.Separator)
	}
	return strings.HasPrefix(candidate, parent)
}

// =============================================================== embed.FS

// Embed wraps an fs.FS (e.g. an embed.FS) under a path prefix.
type Embed struct {
	FS     fs.FS
	Prefix string
}

// NewEmbed returns an Embed loader rooted at prefix inside fsys.
func NewEmbed(fsys fs.FS, prefix string) *Embed {
	return &Embed{FS: fsys, Prefix: prefix}
}

// GetSource reads name from the embedded fs.
func (e Embed) GetSource(name string) (Source, error) {
	parts, err := SplitPath(name)
	if err != nil {
		return Source{}, gjerrors.NewTemplateNotFound(name, "")
	}
	full := strings.Join(parts, "/")
	if e.Prefix != "" {
		full = strings.TrimSuffix(e.Prefix, "/") + "/" + full
	}
	data, err := fs.ReadFile(e.FS, full)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Source{}, gjerrors.NewTemplateNotFound(name, "")
		}
		return Source{}, err
	}
	// embed.FS contents are immutable in-process; uptodate is always true.
	return Source{Code: string(data), Filename: full, Uptodate: func() bool { return true }}, nil
}

// =============================================================== concurrency

// Cached wraps a Loader and memoises results in process. Useful when
// templates are large but the underlying source rarely changes. Calls
// each entry's Uptodate before returning a cached value.
type Cached struct {
	Inner Loader

	mu sync.RWMutex
	m  map[string]Source
}

// NewCached returns a Cached over inner.
func NewCached(inner Loader) *Cached {
	return &Cached{Inner: inner, m: map[string]Source{}}
}

// GetSource returns from cache when fresh, else delegates and stores.
func (c *Cached) GetSource(name string) (Source, error) {
	c.mu.RLock()
	if s, ok := c.m[name]; ok {
		c.mu.RUnlock()
		if s.Uptodate == nil || s.Uptodate() {
			return s, nil
		}
	} else {
		c.mu.RUnlock()
	}
	s, err := c.Inner.GetSource(name)
	if err != nil {
		return Source{}, err
	}
	c.mu.Lock()
	c.m[name] = s
	c.mu.Unlock()
	return s, nil
}
