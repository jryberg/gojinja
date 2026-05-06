package lexer

// Options controls how the lexer interprets source. The zero value is
// invalid; use [DefaultOptions] and override fields, or call
// [Options.WithDefaults] before constructing a [Lexer].
type Options struct {
	BlockStart          string // default "{%"
	BlockEnd            string // default "%}"
	VariableStart       string // default "{{"
	VariableEnd         string // default "}}"
	CommentStart        string // default "{#"
	CommentEnd          string // default "#}"
	LineStatementPrefix string // empty disables line statements
	LineCommentPrefix   string // empty disables line comments
	TrimBlocks          bool   // strip first newline after a block tag
	LstripBlocks        bool   // strip leading whitespace before a block tag (whole-line only)
	NewlineSequence     string // canonical output newline; "\n", "\r\n", or "\r"
	KeepTrailingNewline bool   // preserve final newline of source
	// MaxNestingDepth caps how deeply parens/brackets/braces may nest
	// before the lexer raises an error. Zero means use [DefaultMaxNestingDepth].
	// Bounding this prevents stack-overflow attacks from a hostile template.
	MaxNestingDepth int
}

// DefaultMaxNestingDepth caps bracket/paren/brace nesting. 200 is well above
// any realistic template; we round-trip Python's full test corpus comfortably.
const DefaultMaxNestingDepth = 200

// DefaultOptions returns the Jinja2-default lexer settings (matching
// `jinja2.defaults`).
func DefaultOptions() Options {
	return Options{
		BlockStart:          "{%",
		BlockEnd:            "%}",
		VariableStart:       "{{",
		VariableEnd:         "}}",
		CommentStart:        "{#",
		CommentEnd:          "#}",
		NewlineSequence:     "\n",
		MaxNestingDepth:     DefaultMaxNestingDepth,
	}
}

// WithDefaults returns a copy of o with any zero fields replaced by their
// defaults. This lets callers fill only the fields they care about.
func (o Options) WithDefaults() Options {
	d := DefaultOptions()
	if o.BlockStart == "" {
		o.BlockStart = d.BlockStart
	}
	if o.BlockEnd == "" {
		o.BlockEnd = d.BlockEnd
	}
	if o.VariableStart == "" {
		o.VariableStart = d.VariableStart
	}
	if o.VariableEnd == "" {
		o.VariableEnd = d.VariableEnd
	}
	if o.CommentStart == "" {
		o.CommentStart = d.CommentStart
	}
	if o.CommentEnd == "" {
		o.CommentEnd = d.CommentEnd
	}
	if o.NewlineSequence == "" {
		o.NewlineSequence = d.NewlineSequence
	}
	if o.MaxNestingDepth <= 0 {
		o.MaxNestingDepth = d.MaxNestingDepth
	}
	return o
}
