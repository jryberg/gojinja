// Package tests is intentionally empty.
//
// Built-in Jinja2 tests (predicates used with `is`, e.g. `x is defined`)
// are registered directly on the Environment in
// `pkg/environment/builtins.go` to avoid an import cycle (22 unique tests
// + 17 aliases). This package is preserved as a future hook for
// out-of-tree test packs.
//
// The package name "tests" refers to Jinja2 tests, not Go _test.go files.
package tests
