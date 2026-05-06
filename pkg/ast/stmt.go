package ast

// Template is the outermost wrapper node — every parsed source becomes a
// Template. body is the list of top-level [Stmt]s.
type Template struct {
	posBase
	Body []Node
}

func (*Template) astNode() {}

// Output is a "print" group: a sequence of expressions that all render to
// the output. The parser emits one Output per template-text+variable run.
type Output struct {
	stmtBase
	Nodes []Expr
}

// Extends represents `{% extends template %}`.
type Extends struct {
	stmtBase
	Template Expr
}

// For is the `{% for ... %}` loop. Test is the optional inline filter
// (`{% for x in xs if cond %}`); Recursive is the `recursive` modifier.
type For struct {
	stmtBase
	Target    Node
	Iter      Node
	Body      []Node
	Else      []Node
	Test      Node // may be nil
	Recursive bool
}

// If is `{% if test %} body ... {% elif ... %} ... {% else %} ... {% endif %}`.
// Each elif is itself an If node carrying its own Test/Body/Elif/Else.
type If struct {
	stmtBase
	Test Node
	Body []Node
	Elif []*If
	Else []Node
}

// Macro is `{% macro name(args, defaults) %} body {% endmacro %}`.
type Macro struct {
	stmtBase
	Name     string
	Args     []*Name
	Defaults []Expr
	Body     []Node
}

// CallBlock is `{% call name(args) %} body {% endcall %}`.
type CallBlock struct {
	stmtBase
	Call     *Call
	Args     []*Name
	Defaults []Expr
	Body     []Node
}

// FilterBlock is `{% filter name(args) %} body {% endfilter %}`.
type FilterBlock struct {
	stmtBase
	Body   []Node
	Filter *Filter
}

// With is `{% with x=1, y=2 %} body {% endwith %}`.
type With struct {
	stmtBase
	Targets []Expr
	Values  []Expr
	Body    []Node
}

// Block is `{% block name %} body {% endblock %}`. The Scoped and Required
// flags map directly to the parser's `scoped`/`required` modifiers.
type Block struct {
	stmtBase
	Name     string
	Body     []Node
	Scoped   bool
	Required bool
}

// Include is `{% include template [with/without context] [ignore missing] %}`.
type Include struct {
	stmtBase
	Template      Expr
	WithContext   bool
	IgnoreMissing bool
}

// Import is `{% import template as target [with/without context] %}`.
type Import struct {
	stmtBase
	Template    Expr
	Target      string
	WithContext bool
}

// FromImport is `{% from template import name1, name2 as alias [with/without context] %}`.
// Each Names entry is either a single name (alias = "") or (name, alias).
type FromImport struct {
	stmtBase
	Template    Expr
	Names       []ImportName
	WithContext bool
}

// ImportName is one entry in [FromImport.Names].
type ImportName struct {
	Name  string
	Alias string // "" if no `as` clause
}

// ExprStmt evaluates an expression and discards the result — used by the
// `do` extension.
type ExprStmt struct {
	stmtBase
	Node Node
}

// Assign is `{% set target = expr %}`.
type Assign struct {
	stmtBase
	Target Expr
	Node   Node
}

// AssignBlock is `{% set target %} body {% endset %}`, optionally with a
// trailing filter.
type AssignBlock struct {
	stmtBase
	Target Expr
	Filter *Filter // optional
	Body   []Node
}

// Continue is the `{% continue %}` directive (loopcontrols extension).
type Continue struct{ stmtBase }

// Break is the `{% break %}` directive (loopcontrols extension).
type Break struct{ stmtBase }

// Scope is an artificial scope the compiler may introduce; it has no
// surface syntax.
type Scope struct {
	stmtBase
	Body []Node
}

// OverlayScope inserts a context dict into a sub-scope. Used by
// extensions like `i18n`.
type OverlayScope struct {
	stmtBase
	Context Expr
	Body    []Node
}

// EvalContextModifier flips eval-context flags (autoescape, etc.).
// `{% autoescape false %}` compiles to one of these.
type EvalContextModifier struct {
	stmtBase
	Options []*Keyword
}

// ScopedEvalContextModifier modifies the eval context within a body and
// reverts on exit.
type ScopedEvalContextModifier struct {
	stmtBase
	Options []*Keyword
	Body    []Node
}
