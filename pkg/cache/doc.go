// Package cache stores parsed/lowered template IR keyed by SHA-256 of
// (name|filename) and validated against a SHA-256 source checksum.
// Implementations: MemoryCache (LRU), FilesystemCache (atomic temp+rename).
// Equivalent in spirit to Jinja2's bccache.py, but with our own IR payload
// — never a Python-runtime serialization format.
package cache
