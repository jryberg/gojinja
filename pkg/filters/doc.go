// Package filters is intentionally empty.
//
// Built-in filters are registered directly on the Environment in
// `pkg/environment/builtins.go` and `pkg/environment/filters_extra.go`
// to avoid an import cycle (filters call back into the engine for
// sandboxed attribute access). This package is preserved as a future
// hook for out-of-tree filter packs.
package filters
