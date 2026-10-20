package runtime

import (
	"fmt"
	"strings"
)

// Bytes is Python's bytes, produced by str.encode. str() and repr() of
// bytes are both the b'...' literal.
type Bytes []byte

// String returns the Python repr, which is also what str(bytes) gives.
func (b Bytes) String() string {
	quote := byte('\'')
	if strings.IndexByte(string(b), '\'') >= 0 && strings.IndexByte(string(b), '"') < 0 {
		quote = '"'
	}
	var sb strings.Builder
	sb.WriteByte('b')
	sb.WriteByte(quote)
	for _, c := range b {
		switch {
		case c == quote || c == '\\':
			sb.WriteByte('\\')
			sb.WriteByte(c)
		case c == '\t':
			sb.WriteString(`\t`)
		case c == '\n':
			sb.WriteString(`\n`)
		case c == '\r':
			sb.WriteString(`\r`)
		case c < 0x20 || c >= 0x7f:
			fmt.Fprintf(&sb, `\x%02x`, c)
		default:
			sb.WriteByte(c)
		}
	}
	sb.WriteByte(quote)
	return sb.String()
}
