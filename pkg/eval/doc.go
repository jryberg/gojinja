// Package eval is the tree-walking evaluator. It walks an *ast.Template
// against a *runtime.Context and streams output through an io.Writer.
// Replaces Jinja2's source-emitting compiler.py with a direct interpreter
// — see PHASE_09_evaluator.md.
package eval
