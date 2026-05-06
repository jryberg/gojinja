// Package sandbox implements the safety predicates and operator
// interception layer that every gojinja Environment uses by default. Maps
// Jinja2's sandbox.py: is_safe_attribute, is_safe_callable, intercepted
// binops/unops, format-string sandboxing, immutable-collection guard, and
// safe_range.
package sandbox
