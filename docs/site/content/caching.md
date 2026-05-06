# Caching

gojinja has two layers of caching:

1. **In-process template cache.** The `Environment` keeps a bounded LRU of
   parsed `*Template` values. Configure with `gj.WithCacheSize(n)`.
2. **External AST cache.** A persistent cache of parsed AST values keyed
   by source-text checksum. Plug one in with
   `gj.WithExternalCache(cache)`. Two implementations ship out of the
   box: `MemoryCache` (testing) and `FilesystemCache` (production
   cold-start speed-ups).

## FilesystemCache

`FilesystemCache` writes one file per template under a directory you
choose. Files are written **atomically** (temp file + rename) and carry
a magic header plus a SHA-256 checksum so a corrupted or
wrong-version cache file is rejected on read instead of crashing.

```go
import "github.com/jryberg/gojinja/pkg/cache"

c, _ := cache.NewFilesystem("/var/cache/myapp/templates")
env, _ := gj.New(gj.WithExternalCache(c))
```

The on-disk format is **gojinja-specific** (a `gob`-encoded `*ast.Template`
with a magic + checksum header) — **not** Python Jinja2's bytecode format.
See the [divergences](divergences.md) page.

## Custom cache backends

`Cache` is a small interface (`Get(key) ([]byte, bool, error)` + `Put(key, data) error`).
Implement it against Memcached, Redis, or any other store.

!!! note
    The full narrative for each cache type lives on this page in a follow-up
    update. The full reference is on
    [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/cache).
