package ast

import "encoding/gob"

// Register every concrete AST node with encoding/gob so the cache layer
// can serialize/deserialize Templates whose Node-typed fields hold the
// real implementations. Listed exhaustively to match the catalog in
// stmt.go / expr.go / binop.go / unaryop.go / compare.go.
func init() {
	register(
		// Bases (not strictly required, but harmless).
		&Template{},
		// Statements.
		&Output{}, &Extends{}, &For{}, &If{}, &Macro{}, &CallBlock{},
		&FilterBlock{}, &With{}, &Block{}, &Include{}, &Import{},
		&FromImport{}, &ExprStmt{}, &Assign{}, &AssignBlock{},
		&Continue{}, &Break{}, &Scope{}, &OverlayScope{},
		&EvalContextModifier{}, &ScopedEvalContextModifier{},
		// Expressions.
		&Name{}, &NSRef{}, &Const{}, &TemplateData{}, &Tuple{}, &List{},
		&Dict{}, &Pair{}, &Keyword{}, &CondExpr{}, &Filter{}, &Test{},
		&Call{}, &Getitem{}, &Getattr{}, &Slice{}, &Concat{}, &Operand{},
		&Compare{}, &EnvironmentAttribute{}, &ExtensionAttribute{},
		&ImportedName{}, &InternalName{}, &MarkSafe{}, &MarkSafeIfAutoescape{},
		&ContextReference{}, &DerivedContextReference{},
		// Binary ops.
		&Add{}, &Sub{}, &Mul{}, &Div{}, &FloorDiv{}, &Mod{}, &Pow{}, &And{}, &Or{},
		// Unary ops.
		&Not{}, &Neg{}, &UAdd{},
	)
}

func register(nodes ...Node) {
	for _, n := range nodes {
		gob.Register(n)
	}
}
