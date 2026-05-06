// Package errors holds gojinja's error types. They mirror Jinja2's
// exception hierarchy from src/jinja2/exceptions.py:
//
//	TemplateError              (root)
//	  TemplateNotFound
//	    TemplatesNotFound
//	  TemplateSyntaxError
//	    TemplateAssertionError
//	  TemplateRuntimeError
//	    UndefinedError
//	    SecurityError
//	    FilterArgumentError
//
// The Go implementation uses error wrapping (errors.Is / errors.As) instead
// of class hierarchies. Each derived type embeds [TemplateError] so callers
// can match the broad category with errors.Is(err, &errors.TemplateError{})
// or, more usefully, errors.As(err, &target).
package errors

import (
	"errors"
	"fmt"
	"strings"
)

// TemplateError is the base type for all gojinja template errors.
type TemplateError struct {
	Message string
}

// Error satisfies the error interface.
func (e *TemplateError) Error() string {
	return e.Message
}

// NewTemplateError constructs a TemplateError with the given message.
func NewTemplateError(message string) *TemplateError {
	return &TemplateError{Message: message}
}

// AsTemplateError extracts the underlying *TemplateError if err is one of
// gojinja's typed template errors.
func AsTemplateError(err error) (*TemplateError, bool) {
	for cur := err; cur != nil; cur = errors.Unwrap(cur) {
		switch v := cur.(type) {
		case *TemplateError:
			return v, true
		case *TemplateNotFound:
			return v.TemplateError, true
		case *TemplatesNotFound:
			return v.TemplateError, true
		case *TemplateSyntaxError:
			return v.TemplateError, true
		case *TemplateAssertionError:
			return v.TemplateError, true
		case *TemplateRuntimeError:
			return v.TemplateError, true
		case *UndefinedError:
			return v.TemplateError, true
		case *SecurityError:
			return v.TemplateError, true
		case *FilterArgumentError:
			return v.TemplateError, true
		}
	}
	return nil, false
}

// TemplateNotFound is raised by a Loader when a template cannot be located.
// Name carries the failed template name; if Name is an [Undefined] value it
// means the lookup was attempted with an undefined variable.
type TemplateNotFound struct {
	*TemplateError
	Name      any
	Templates []any
}

// NewTemplateNotFound constructs a TemplateNotFound. If message is empty the
// name is used as the message (matching Jinja2). Pass an [Undefined] value
// for name when the user supplied an undefined template name; the caller is
// expected to surface the Undefined's own error message.
func NewTemplateNotFound(name any, message string) *TemplateNotFound {
	if message == "" {
		message = fmt.Sprintf("%v", name)
	}
	return &TemplateNotFound{
		TemplateError: NewTemplateError(message),
		Name:          name,
		Templates:     []any{name},
	}
}

func (e *TemplateNotFound) Error() string { return e.Message }

// TemplatesNotFound is raised when none of a list of names could be loaded.
type TemplatesNotFound struct {
	*TemplateError
	Templates []any
}

// NewTemplatesNotFound constructs a TemplatesNotFound. If message is empty a
// default message listing the names is generated.
func NewTemplatesNotFound(names []any, message string) *TemplatesNotFound {
	if message == "" {
		parts := make([]string, len(names))
		for i, n := range names {
			parts[i] = fmt.Sprintf("%v", n)
		}
		message = "none of the templates given were found: " + strings.Join(parts, ", ")
	}
	tmpls := append([]any(nil), names...)
	return &TemplatesNotFound{
		TemplateError: NewTemplateError(message),
		Templates:     tmpls,
	}
}

func (e *TemplatesNotFound) Error() string { return e.Message }

// TemplateSyntaxError reports a problem found while lexing or parsing a
// template. Lineno is 1-based; Source is filled in by the engine when
// available so the formatted error can include the offending source line.
type TemplateSyntaxError struct {
	*TemplateError
	Lineno     int
	Name       string
	Filename   string
	Source     string
	Translated bool
}

// NewTemplateSyntaxError constructs a TemplateSyntaxError.
func NewTemplateSyntaxError(message string, lineno int, name, filename string) *TemplateSyntaxError {
	return &TemplateSyntaxError{
		TemplateError: NewTemplateError(message),
		Lineno:        lineno,
		Name:          name,
		Filename:      filename,
	}
}

// Error formats the error with a "File ... line N" location and, if Source
// is set, the offending source line itself.
func (e *TemplateSyntaxError) Error() string {
	if e.Translated {
		return e.Message
	}
	location := fmt.Sprintf("line %d", e.Lineno)
	if name := e.Filename; name != "" {
		location = fmt.Sprintf("File %q, %s", name, location)
	} else if e.Name != "" {
		location = fmt.Sprintf("File %q, %s", e.Name, location)
	}
	out := []string{e.Message, "  " + location}
	if e.Source != "" {
		lines := strings.Split(e.Source, "\n")
		if e.Lineno >= 1 && e.Lineno <= len(lines) {
			out = append(out, "    "+strings.TrimSpace(lines[e.Lineno-1]))
		}
	}
	return strings.Join(out, "\n")
}

// TemplateAssertionError is a TemplateSyntaxError raised at compile time for
// non-syntactic problems (e.g. duplicate block names).
type TemplateAssertionError struct {
	*TemplateError
	Lineno     int
	Name       string
	Filename   string
	Source     string
	Translated bool
}

// NewTemplateAssertionError constructs one.
func NewTemplateAssertionError(message string, lineno int, name, filename string) *TemplateAssertionError {
	return &TemplateAssertionError{
		TemplateError: NewTemplateError(message),
		Lineno:        lineno,
		Name:          name,
		Filename:      filename,
	}
}

// Error formats identically to TemplateSyntaxError.
func (e *TemplateAssertionError) Error() string {
	tse := &TemplateSyntaxError{
		TemplateError: e.TemplateError,
		Lineno:        e.Lineno,
		Name:          e.Name,
		Filename:      e.Filename,
		Source:        e.Source,
		Translated:    e.Translated,
	}
	return tse.Error()
}

// TemplateRuntimeError is the base for runtime errors raised while rendering.
type TemplateRuntimeError struct {
	*TemplateError
}

// NewTemplateRuntimeError constructs one.
func NewTemplateRuntimeError(message string) *TemplateRuntimeError {
	return &TemplateRuntimeError{TemplateError: NewTemplateError(message)}
}

// Error returns the message.
func (e *TemplateRuntimeError) Error() string { return e.Message }

// UndefinedError is raised when an Undefined value is used in a way that
// requires a real value (arithmetic, attribute access, call, comparison
// other than equality).
type UndefinedError struct {
	*TemplateError
}

// NewUndefinedError constructs one.
func NewUndefinedError(message string) *UndefinedError {
	return &UndefinedError{TemplateError: NewTemplateError(message)}
}

func (e *UndefinedError) Error() string { return e.Message }

// SecurityError is raised when a sandboxed environment refuses an operation.
type SecurityError struct {
	*TemplateError
}

// NewSecurityError constructs one.
func NewSecurityError(message string) *SecurityError {
	return &SecurityError{TemplateError: NewTemplateError(message)}
}

func (e *SecurityError) Error() string { return e.Message }

// FilterArgumentError is raised when a filter receives invalid arguments.
type FilterArgumentError struct {
	*TemplateError
}

// NewFilterArgumentError constructs one.
func NewFilterArgumentError(message string) *FilterArgumentError {
	return &FilterArgumentError{TemplateError: NewTemplateError(message)}
}

func (e *FilterArgumentError) Error() string { return e.Message }
