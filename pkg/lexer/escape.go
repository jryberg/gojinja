package lexer

import (
	"strings"

	"github.com/jryberg/gojinja/internal/pyescape"
)

// decodeStringLiteral interprets a Jinja2 string literal value (the text
// between the quotes, newlines already normalised) the way Jinja2's lexer
// does: value.encode("ascii", "backslashreplace").decode("unicode-escape").
// All of Python's string escapes apply, \N{NAME} included; an unknown
// escape keeps its backslash. The error is the bare reason Jinja2 reports
// (e.g. "truncated \xXX escape").
func decodeStringLiteral(value string) (string, error) {
	if !strings.ContainsRune(value, '\\') {
		return value, nil
	}
	var sb strings.Builder
	return pyescape.Decode(pyescape.BackslashReplace(value), false, &sb, pyescape.Strict)
}
