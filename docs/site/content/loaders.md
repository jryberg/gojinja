# Loaders

A **loader** turns a template name into source bytes. Every consumer of
`Environment.GetTemplate(name)` goes through one. gojinja ships seven
built-ins, all in `pkg/loader` and re-exported from the root package.

| Loader | Use when |
|---|---|
| [`DictLoader`](#dictloader) | Templates live in memory (tests, fixtures, small CLIs) |
| [`FileSystemLoader`](#filesystemloader) | Templates live on disk under one or more allowlisted roots |
| [`EmbedLoader`](#embedloader) | Templates are baked into the binary via `//go:embed` |
| [`PrefixLoader`](#prefixloader) | Route by name prefix (`admin/foo` → admin loader) |
| [`ChoiceLoader`](#choiceloader) | Try several loaders in order, first hit wins |
| [`FuncLoader`](#funcloader) | Wrap an arbitrary fetch function (DB, HTTP, etc.) |
| [`CachedLoader`](#cachedloader) | Memoise an expensive loader's results in process |

!!! note
    The narrative content for each loader lives on this page in a follow-up
    update. For now, see the [README's
    loaders section](https://github.com/jryberg/gojinja#loaders-7) and the
    full reference on
    [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/loader).

## DictLoader

In-memory `map[string]string` of name → source. Convenient for tests.

## FileSystemLoader

Allowlisted on-disk loader. Rejects names containing NUL bytes, drive
letters, `..` traversal, or symlinks that escape the allowlisted roots
(unless `loader.WithFollowLinks(true)` is set). See
[divergences](divergences.md) for the security defaults.

## EmbedLoader

Reads from an `embed.FS` rooted at a path prefix. The Go-idiomatic
replacement for Python Jinja2's `PackageLoader`.

## PrefixLoader

Routes templates by name prefix to sub-loaders. `{"admin/": adminLoader}`
sends `admin/dashboard.html` to `adminLoader` with the prefix stripped.

## ChoiceLoader

Tries each loader in order, returning the first hit.

## FuncLoader

Wraps an arbitrary fetch function. Use for database-backed templates,
HTTP-fetched templates, or any other custom source.

## CachedLoader

Memoises another loader's results. The cache is in-process and unbounded
by default — pair with a [persistent cache](caching.md) for cold-start
speed-ups across processes.
