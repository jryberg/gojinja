// Package globals is intentionally empty.
//
// The built-in template globals (`range`, `dict`, `lipsum`, `cycler`,
// `joiner`, `namespace`) are registered directly on the Environment in
// `pkg/environment/builtins.go` and `pkg/environment/lipsum.go` to
// avoid an import cycle. This package is preserved as a future hook
// for out-of-tree global packs.
package globals
