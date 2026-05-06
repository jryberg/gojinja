// Package ast defines the gojinja AST. It mirrors jinja2.nodes.*. The
// package is pure data: it has no runtime dependency on the evaluator.
// Constant folding is a separate concern handled in [Expr.AsConst] which
// uses [EvalCtx] to know whether autoescape is on. A real Environment-bound
// EvalContext is wired by the evaluator and converted to [EvalCtx] at the
// boundary.
package ast

import (
	"errors"
)

// Pos is the source position attached to every Node. Line numbers are
// 1-based; 0 means "unset". Mirrors the `lineno` attribute on jinja2.nodes.Node.
type Pos struct {
	Lineno int
}

// Position returns the line number, satisfying [Node].
func (p Pos) Position() Pos { return p }

// SetLineno sets the line number on this position. Used by the parser
// while building nodes top-down.
func (p *Pos) SetLineno(line int) { p.Lineno = line }

// Node is the marker every AST node implements. Implementations are
// either statements ([Stmt]), expressions ([Expr]), or helpers ([Helper]).
type Node interface {
	// Position returns the source line of the node.
	Position() Pos
	// astNode is unexported so external packages can't accidentally
	// implement Node.
	astNode()
}

// Stmt is the marker for statement nodes (ones that appear in a list of
// template body elements rather than inside an expression).
type Stmt interface {
	Node
	stmtNode()
}

// Expr is the marker for expression nodes. Every Expr supports [AsConst]
// for the constant folder; nodes that cannot be folded return
// [ErrImpossible].
type Expr interface {
	Node
	exprNode()
	// AsConst evaluates the expression to a constant Go value, returning
	// [ErrImpossible] if folding isn't possible (e.g. the expression
	// references a name, or calls a side-effecting filter).
	AsConst(ctx EvalCtx) (any, error)
	// CanAssign reports whether this expression is a valid assignment
	// target (used by the parser when reading `set x = ...`).
	CanAssign() bool
}

// Literal is the marker for literal expressions: Const, TemplateData,
// Tuple, List, Dict.
type Literal interface {
	Expr
	literalNode()
}

// Helper is the marker for non-Stmt, non-Expr nodes like [Pair], [Keyword],
// [Operand].
type Helper interface {
	Node
	helperNode()
}

// EvalCtx is the slice of evaluation context that constant folding cares
// about. The evaluator builds a real one and passes it down; tests and
// trivial cases can construct one in-place.
type EvalCtx struct {
	Autoescape bool
	Volatile   bool
}

// ErrImpossible signals that a node could not be folded to a constant.
// Mirrors jinja2.nodes.Impossible.
var ErrImpossible = errors.New("ast: expression is not a constant")

// IsImpossible reports whether err originates from a folder bailing out.
func IsImpossible(err error) bool { return errors.Is(err, ErrImpossible) }

// posBase is embedded by every concrete node. It supplies Position(),
// astNode(), and the line-number storage in one place.
type posBase struct{ Pos }

func (posBase) astNode() {}

// stmtBase is embedded by every Stmt to mark it.
type stmtBase struct{ posBase }

func (stmtBase) stmtNode() {}

// exprBase is embedded by every Expr. It wires astNode/exprNode and
// provides a default AsConst that returns ErrImpossible — concrete nodes
// override.
type exprBase struct{ posBase }

func (exprBase) exprNode() {}

// AsConst is the default fold — concrete Expr types embed exprBase and
// override AsConst when they can produce a constant value.
func (exprBase) AsConst(EvalCtx) (any, error) { return nil, ErrImpossible }

// CanAssign is the default — most Expr nodes are not valid assignment
// targets. Names, NSRefs, Tuples, Getitem, and Getattr override.
func (exprBase) CanAssign() bool { return false }

// literalBase marks an Expr as a Literal.
type literalBase struct{ exprBase }

func (literalBase) literalNode() {}

// helperBase marks a Helper.
type helperBase struct{ posBase }

func (helperBase) helperNode() {}
