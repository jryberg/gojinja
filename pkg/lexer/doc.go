// Package lexer turns template source into a TokenStream. Token kinds and
// the state machine mirror Jinja2's lexer.py. Whitespace control (-/+
// markers, trim_blocks, lstrip_blocks, keep_trailing_newline) and
// configurable line-statement / line-comment prefixes are handled here.
package lexer
