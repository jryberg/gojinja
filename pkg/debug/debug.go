// Package debug provides error wrapping that attaches template-source
// context to runtime errors. We do not synthesize fake Go stack frames
// the way Jinja2's debug.py rewrites Python tracebacks; instead, errors
// are wrapped in a [TraceError] that carries (template name, line, source
// excerpt) so the formatted message points at the template, not the
// engine.
package debug

import (
	"errors"
	"fmt"
	"strings"
)

// TraceError wraps an underlying engine error with a template location.
// Its Error() method formats:
//
//	<original message>
//	  in template "<name>", line <N>
//	      <source line>
//
// Multiple TraceErrors can stack (e.g. an include error inside a parent
// template) — Errorf handles unwrapping cleanly.
type TraceError struct {
	Inner    error
	Template string
	Lineno   int
	Source   string
}

// Wrap returns a TraceError around err. If err is nil the result is nil
// (so callers can use it unconditionally).
func Wrap(err error, template string, lineno int, source string) error {
	if err == nil {
		return nil
	}
	if _, ok := err.(*TraceError); ok {
		// Already wrapped; don't double-wrap to avoid noisy traces.
		return err
	}
	return &TraceError{Inner: err, Template: template, Lineno: lineno, Source: source}
}

// Error formats with template location and source excerpt.
func (e *TraceError) Error() string {
	var b strings.Builder
	b.WriteString(e.Inner.Error())
	if e.Template != "" {
		fmt.Fprintf(&b, "\n  in template %q", e.Template)
		if e.Lineno > 0 {
			fmt.Fprintf(&b, ", line %d", e.Lineno)
		}
	} else if e.Lineno > 0 {
		fmt.Fprintf(&b, "\n  at line %d", e.Lineno)
	}
	if e.Source != "" && e.Lineno > 0 {
		lines := strings.Split(e.Source, "\n")
		if e.Lineno >= 1 && e.Lineno <= len(lines) {
			b.WriteString("\n      ")
			b.WriteString(strings.TrimSpace(lines[e.Lineno-1]))
		}
	}
	return b.String()
}

// Unwrap returns the underlying error so [errors.Is] / [errors.As] reach it.
func (e *TraceError) Unwrap() error { return e.Inner }

// Is matches against the underlying error too.
func (e *TraceError) Is(target error) bool { return errors.Is(e.Inner, target) }
