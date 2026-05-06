package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
)

// Entry is a single registered filter, test, or global. The exact shape
// of the Go-side function isn't captured — just the name (the
// template-visible identifier) and the func identifier docgen can look
// up in the package's go/doc.
type Entry struct {
	Kind     string // "filter" | "test" | "global"
	Name     string // template-visible identifier (e.g. "upper")
	FuncName string // Go-side function identifier (e.g. "filterUpper")
	IsAlias  bool   // true if this entry shares its FuncName with another already-seen entry
}

// loadRegistries parses pkg/environment/{builtins.go, filters_extra.go,
// globals.go} and returns every filter / test / global registration.
func loadRegistries(repoRoot string) ([]Entry, error) {
	envDir := filepath.Join(repoRoot, "pkg", "environment")
	files := []string{
		filepath.Join(envDir, "builtins.go"),
		filepath.Join(envDir, "filters_extra.go"),
		filepath.Join(envDir, "globals.go"),
	}

	fset := token.NewFileSet()
	var entries []Entry
	seen := map[string]bool{} // "kind/funcName" → already seen
	for _, path := range files {
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		entries = append(entries, walkRegistrations(f, seen)...)
	}
	return entries, nil
}

// walkRegistrations finds assignments of the shape:
//
//	e.filters["NAME"] = Filter{Func: ident}
//	e.filters["NAME"] = Filter{Pass: ..., Func: ident}
//	e.tests["NAME"]   = Test{Func: ident}
//	e.globals["NAME"] = ident             (also: globalIdent(e), KwargsCallable(ident))
//
// The "kind" comes from the map name; the func ident is the leaf
// identifier of the RHS.
func walkRegistrations(f *ast.File, seen map[string]bool) []Entry {
	var out []Entry
	ast.Inspect(f, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
			return true
		}
		idx, ok := assign.Lhs[0].(*ast.IndexExpr)
		if !ok {
			return true
		}
		// LHS must be e.<map>[<string-literal>].
		sel, ok := idx.X.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		var kind string
		switch sel.Sel.Name {
		case "filters":
			kind = "filter"
		case "tests":
			kind = "test"
		case "globals":
			kind = "global"
		default:
			return true
		}
		nameLit, ok := idx.Index.(*ast.BasicLit)
		if !ok || nameLit.Kind != token.STRING {
			return true
		}
		name, err := stringLit(nameLit.Value)
		if err != nil {
			return true
		}
		funcName := extractFuncIdent(assign.Rhs[0])
		if funcName == "" {
			return true
		}
		key := kind + "/" + funcName
		alias := seen[key]
		seen[key] = true
		out = append(out, Entry{
			Kind:     kind,
			Name:     name,
			FuncName: funcName,
			IsAlias:  alias,
		})
		return true
	})
	return out
}

// extractFuncIdent walks an RHS expression and returns the bottom-most
// function identifier. Handles the small set of shapes the registry
// uses today:
//
//	filterUpper                     → "filterUpper"
//	Filter{Func: filterUpper}       → "filterUpper"
//	Filter{Pass: ..., Func: ident}  → "ident"
//	Test{Func: testEq}              → "testEq"
//	globalRange(e)                  → "globalRange"
//	KwargsCallable(globalDict)      → "globalDict"
//	namespaceCtor                   → "namespaceCtor"
func extractFuncIdent(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.CompositeLit:
		// Look for a `Func: ident` element.
		for _, elt := range x.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			key, ok := kv.Key.(*ast.Ident)
			if !ok || key.Name != "Func" {
				continue
			}
			if id, ok := kv.Value.(*ast.Ident); ok {
				return id.Name
			}
		}
	case *ast.CallExpr:
		// Either `KwargsCallable(globalDict)` — recurse on the only arg
		// (it should be an Ident), or `globalRange(e)` — return the
		// callee's name.
		if id, ok := x.Fun.(*ast.Ident); ok {
			if id.Name == "KwargsCallable" && len(x.Args) == 1 {
				if arg, ok := x.Args[0].(*ast.Ident); ok {
					return arg.Name
				}
			}
			return id.Name
		}
	}
	return ""
}

// stringLit unquotes a Go string literal (handles both raw and
// double-quoted forms). go/strconv.Unquote also works but pulling the
// import for one call is overkill.
func stringLit(s string) (string, error) {
	if len(s) < 2 {
		return "", fmt.Errorf("invalid string literal %q", s)
	}
	if s[0] == '`' && s[len(s)-1] == '`' {
		return s[1 : len(s)-1], nil
	}
	if s[0] == '"' && s[len(s)-1] == '"' {
		// Minimal unescape — registry literals never use anything fancy.
		return strings.ReplaceAll(s[1:len(s)-1], `\"`, `"`), nil
	}
	return "", fmt.Errorf("unrecognised string literal %q", s)
}

// isSymbolic reports whether name contains characters that would break
// directory paths or URL slugs. Used to prefer human-readable names as
// canonicals.
func isSymbolic(name string) bool {
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '_' || r == '-') {
			return true
		}
	}
	return false
}

// slugify makes name safe to use as a directory / file basename by
// replacing symbol characters with mnemonic stand-ins. For test
// operators only — letter-only names pass through unchanged.
func slugify(name string) string {
	switch name {
	case "==":
		return "op-eq"
	case "!=":
		return "op-ne"
	case "<":
		return "op-lt"
	case "<=":
		return "op-le"
	case ">":
		return "op-gt"
	case ">=":
		return "op-ge"
	}
	if !isSymbolic(name) {
		return name
	}
	out := make([]rune, 0, len(name))
	for _, r := range name {
		if (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '_' || r == '-' {
			out = append(out, r)
		} else {
			out = append(out, '_')
		}
	}
	return string(out)
}

// canonicalAndAliases returns the canonical name for a group of entries
// sharing a FuncName, plus the alias names. Preference order:
//  1. The first non-symbolic name in source order wins as canonical.
//  2. Failing that, the first entry in source order wins.
func canonicalAndAliases(group []Entry) (canonical Entry, aliases []string) {
	canonical = group[0]
	canonicalIdx := 0
	for i, e := range group {
		if !isSymbolic(e.Name) {
			canonical = e
			canonicalIdx = i
			break
		}
	}
	for i, e := range group {
		if i == canonicalIdx {
			continue
		}
		aliases = append(aliases, e.Name)
	}
	sort.Strings(aliases)
	return canonical, aliases
}
