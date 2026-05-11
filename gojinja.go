// Package gojinja is a Jinja2-compatible template engine for Go.
//
// gojinja parses templates written in the Jinja2 syntax and renders them
// against Go data. Output parity with the reference Python implementation
// (https://github.com/pallets/jinja) is the primary correctness criterion.
//
// # Quick start
//
//	env, _ := gojinja.New(gojinja.WithAutoescape(gojinja.AutoescapeNever{}))
//	tpl, _ := env.FromString("Hello, {{ name | upper }}!")
//	out, _ := tpl.Render(map[string]any{"name": "World"})
//	// → "Hello, WORLD!"
//
// # Security stance
//
// gojinja flips several Jinja2 defaults to safe-by-default values. The
// full list of intentional divergences is documented in
// docs/divergences.md; the short version:
//
//   - The default Environment is sandboxed. Use [WithUnsafe] to opt out.
//   - HTML autoescape is enabled by default.
//   - Host environment access is disabled by default. Use [WithHostEnv]
//     to opt in.
//   - [FileSystemLoader] requires an explicit allowlisted root and rejects
//     symlink traversal that escapes the root.
//   - The `range` global is bounded; the limit is configurable.
//
// # Package layout
//
// Most consumers only need this top-level package, which re-exports the
// types and constructors they typically reach for. Specialized use cases
// (custom AST inspection, the lexer, parity tooling) can import the
// per-feature subpackages directly.
package gojinja

import (
	"github.com/jryberg/gojinja/pkg/cache"
	"github.com/jryberg/gojinja/pkg/environment"
	"github.com/jryberg/gojinja/pkg/escape"
	"github.com/jryberg/gojinja/pkg/loader"
	"github.com/jryberg/gojinja/pkg/runtime"
	"github.com/jryberg/gojinja/pkg/varsutil"
)

// =============================================================== Environment

// Environment is the central configuration object. Construct with [New].
type Environment = environment.Environment

// Template is a compiled template ready to render. Returned by
// [Environment.FromString] and [Environment.GetTemplate].
type Template = environment.Template

// Option configures an [Environment] at construction.
type Option = environment.Option

// New constructs an Environment with safe defaults. Pass [Option]s to
// override specific settings.
func New(opts ...Option) (*Environment, error) { return environment.New(opts...) }

// =============================================================== Options

// WithLoader installs a [Loader].
var WithLoader = environment.WithLoader

// WithAutoescape replaces the autoescape policy (default: [AutoescapeAlways]).
var WithAutoescape = environment.WithAutoescape

// WithGlobal registers (or replaces) a global by name.
var WithGlobal = environment.WithGlobal

// WithFilter registers a filter under a name.
var WithFilter = environment.WithFilter

// WithTest registers a test under a name.
var WithTest = environment.WithTest

// WithUndefined chooses the undefined-variant produced for missing names.
var WithUndefined = environment.WithUndefined

// WithUnsafe disables the default sandbox. Reserved for trusted templates;
// never enable this for user-supplied content.
var WithUnsafe = environment.WithUnsafe

// WithHostEnv permits host environment-variable access. Off by default.
var WithHostEnv = environment.WithHostEnv

// WithRangeLimit caps the size of any single `range()` call.
var WithRangeLimit = environment.WithRangeLimit

// WithCacheSize bounds the in-memory template cache.
var WithCacheSize = environment.WithCacheSize

// WithExternalCache plugs a persistent AST cache (typically a [cache.Filesystem]).
var WithExternalCache = environment.WithExternalCache

// WithLexerOptions tweaks lexer-level settings (block markers, whitespace, etc.).
var WithLexerOptions = environment.WithLexerOptions

// =============================================================== Autoescape

// AutoescapePolicy decides whether to autoescape a given template.
type AutoescapePolicy = environment.AutoescapePolicy

// AutoescapeAlways enables autoescape for every template. The default.
type AutoescapeAlways = environment.AutoescapeAlways

// AutoescapeNever disables autoescape entirely.
type AutoescapeNever = environment.AutoescapeNever

// AutoescapeByExtension enables autoescape based on the template name's
// extension.
type AutoescapeByExtension = environment.AutoescapeByExtension

// =============================================================== Loaders

// Loader is the contract a template-source provider implements (env-side).
type Loader = environment.Loader

// Source is a loaded template's source text plus metadata.
type Source = environment.Source

// DictLoader is an in-memory map of name→source.
type DictLoader = loader.Dict

// FileSystemLoader reads templates from one or more allowlisted directories.
type FileSystemLoader = loader.FileSystem

// EmbedLoader reads templates from an [embed.FS] under a path prefix.
type EmbedLoader = loader.Embed

// PrefixLoader routes templates by name prefix to sub-loaders.
type PrefixLoader = loader.Prefix

// ChoiceLoader tries each loader in order, returning the first hit.
type ChoiceLoader = loader.Choice

// FuncLoader wraps an arbitrary fetch function.
type FuncLoader = loader.FuncLoader

// CachedLoader memoises a Loader's results in process.
type CachedLoader = loader.Cached

// NewFileSystemLoader builds a [FileSystemLoader] over the given roots.
var NewFileSystemLoader = loader.NewFileSystem

// NewEmbedLoader builds an [EmbedLoader] over fsys rooted at prefix.
var NewEmbedLoader = loader.NewEmbed

// NewCachedLoader wraps an existing Loader with in-memory memoisation.
var NewCachedLoader = loader.NewCached

// =============================================================== Cache

// Cache is the persistent AST cache contract. Pass an implementation to
// [WithExternalCache] for cold-start speed-ups.
type Cache = cache.Cache

// MemoryCache is an in-process LRU-bounded AST cache.
type MemoryCache = cache.Memory

// FilesystemCache is an on-disk AST cache (atomic writes, magic+checksum).
type FilesystemCache = cache.Filesystem

// NewMemoryCache constructs a MemoryCache.
var NewMemoryCache = cache.NewMemory

// NewFilesystemCache constructs a FilesystemCache rooted at dir.
var NewFilesystemCache = cache.NewFilesystem

// =============================================================== Runtime

// Undefined is the default undefined-variant. Other variants live in
// [pkg/runtime] and are wired via [WithUndefined].
type Undefined = runtime.Undefined

// Namespace is the runtime container for `{% set ns.attr = ... %}` writes.
type Namespace = runtime.Namespace

// =============================================================== Helpers

// Markup re-exports [pkg/escape.Markup] for callers that want to mark
// pre-escaped HTML safe for output.
//
// Callers should usually let the engine produce Markup itself; this is
// here for advanced cases (custom filters, embedding rendered HTML in
// data structures, etc.).
type Markup = escape.Markup

// =============================================================== JSON vars

// JSONVars decodes JSON bytes into a map[string]any with Python-aligned
// numeric types — integer literals become int64, fractional / exponent
// literals become float64. Use this in preference to encoding/json when
// feeding variables to a template; vanilla json.Unmarshal collapses all
// numbers to float64, which causes `{{ x }}` of an integer to render as
// "1.0" instead of "1" and silently breaks parity with Python Jinja2.
var JSONVars = varsutil.JSONVars

// NormalizeJSONNumbers walks a value decoded with json.Decoder +
// UseNumber and converts every json.Number to int64 or float64 (mirroring
// Python's json.loads). Use this when you drive the JSON decoder
// yourself; otherwise reach for [JSONVars].
var NormalizeJSONNumbers = varsutil.NormalizeJSONNumbers
