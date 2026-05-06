// Package debug renders gojinja errors with template name, line, column,
// and a source excerpt. We do not synthesize fake Go stack frames the way
// Jinja2's debug.py rewrites Python tracebacks; instead, errors are
// structured Go errors that carry the template location.
package debug
