// Package environment hosts the Environment and Template types — the main
// user-facing API surface. Constructor options follow gojinja's safe-by-
// default stance (sandbox on, autoescape on, host env off, bounded range
// and cache); see docs/divergences.md.
package environment
