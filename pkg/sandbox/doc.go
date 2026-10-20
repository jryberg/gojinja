// Package sandbox holds the safety predicates every gojinja Environment
// applies by default, after Jinja2's sandbox.py: IsSafeAttribute
// (is_safe_attribute), IsSafeCallable (is_safe_callable) with the
// UnsafeFunc marker, and IsMutatingMethod (modifies_known_mutable), which
// the default sandbox, like SandboxedEnvironment, leaves to callers.
package sandbox
