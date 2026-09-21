package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// parsePackageFiles parses the non-test Go files in dir with comments and
// groups them by package name.
func parsePackageFiles(dir string) (*token.FileSet, map[string][]*ast.File, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}
	fset := token.NewFileSet()
	pkgs := map[string][]*ast.File{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.ParseComments)
		if err != nil {
			return nil, nil, err
		}
		pkgs[f.Name.Name] = append(pkgs[f.Name.Name], f)
	}
	return fset, pkgs, nil
}

// sortedPackageNames returns the keys of pkgs in lexical order.
func sortedPackageNames(pkgs map[string][]*ast.File) []string {
	names := make([]string, 0, len(pkgs))
	for n := range pkgs {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
