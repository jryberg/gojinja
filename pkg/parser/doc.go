// Package parser turns a TokenStream into an *ast.Template. Implements the
// 13-level operator precedence ladder, every built-in statement tag, and
// extension-tag dispatch. Mirrors Jinja2's parser.py.
package parser
