package main

import (
	"fmt"
	"go/doc"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const modPath = "github.com/jryberg/gojinja"

// runSymbols enumerates every exported symbol in the root package and
// each pkg/* package, then emits a single Markdown table mapping
// (package, name) → first-line synopsis → pkg.go.dev permalink.
//
// Pin the link version with the MIKE_VERSION env var (the docs
// workflow sets it). Falls back to "latest".
func runSymbols(repoRoot, outPath string) error {
	version := os.Getenv("MIKE_VERSION")
	if version == "" {
		version = "latest"
	}
	// Skip ast / lexer / parser / idtracking / debug / optimizer / errors /
	// eval as those are mostly engine internals not surfaced to users.
	// pkg/loader, pkg/cache, pkg/environment, pkg/runtime, pkg/sandbox,
	// pkg/escape, pkg/native, pkg/ext, pkg/meta, pkg/filters, pkg/tests,
	// pkg/globals — and the root package — make the cut.
	include := []string{
		"",
		"pkg/environment",
		"pkg/loader",
		"pkg/cache",
		"pkg/runtime",
		"pkg/sandbox",
		"pkg/escape",
		"pkg/native",
		"pkg/ext",
		"pkg/meta",
		"pkg/ast",
	}
	type row struct {
		Package  string // import path suffix (e.g. "pkg/environment")
		Symbol   string
		Kind     string // "type" / "func" / "var" / "const"
		Synopsis string
	}
	var rows []row
	for _, sub := range include {
		dir := filepath.Join(repoRoot, sub)
		pkg, err := loadDocPackage(dir, modPath+"/"+sub)
		if err != nil {
			fmt.Fprintln(os.Stderr, "docgen symbols: skip", sub, ":", err)
			continue
		}
		// Convert leading-slash trim — the root pkg has sub == "".
		pkgRel := sub
		for _, fn := range pkg.Funcs {
			if !exported(fn.Name) {
				continue
			}
			rows = append(rows, row{pkgRel, fn.Name, "func", firstLine(fn.Doc)})
		}
		for _, ty := range pkg.Types {
			if !exported(ty.Name) {
				continue
			}
			rows = append(rows, row{pkgRel, ty.Name, "type", firstLine(ty.Doc)})
			for _, fn := range ty.Funcs {
				if !exported(fn.Name) {
					continue
				}
				rows = append(rows, row{pkgRel, fn.Name, "func", firstLine(fn.Doc)})
			}
		}
		for _, v := range pkg.Vars {
			for _, name := range v.Names {
				if !exported(name) {
					continue
				}
				rows = append(rows, row{pkgRel, name, "var", firstLine(v.Doc)})
			}
		}
		for _, c := range pkg.Consts {
			for _, name := range c.Names {
				if !exported(name) {
					continue
				}
				rows = append(rows, row{pkgRel, name, "const", firstLine(c.Doc)})
			}
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Package != rows[j].Package {
			return rows[i].Package < rows[j].Package
		}
		return rows[i].Symbol < rows[j].Symbol
	})

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("# Symbols index\n\n")
	fmt.Fprintf(&b,
		"Generated from godoc. Each row links to the symbol's reference on\n"+
			"pkg.go.dev, pinned to **%s**. Set `MIKE_VERSION` to repin.\n\n",
		version)
	curPkg := "<<sentinel>>"
	for _, r := range rows {
		if r.Package != curPkg {
			fmt.Fprintf(&b, "\n## `%s`\n\n", pkgImportPath(r.Package))
			b.WriteString("| Symbol | Kind | Synopsis | Reference |\n")
			b.WriteString("|---|---|---|---|\n")
			curPkg = r.Package
		}
		link := pkgGoDevURL(r.Package, r.Symbol, version)
		fmt.Fprintf(&b, "| `%s` | %s | %s | [pkg.go.dev](%s) |\n",
			r.Symbol, r.Kind, escapeMD(r.Synopsis), link)
	}
	return os.WriteFile(outPath, []byte(b.String()), 0o644)
}

func loadDocPackage(dir, importPath string) (*doc.Package, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	for _, p := range pkgs {
		if strings.HasSuffix(p.Name, "_test") {
			continue
		}
		if p.Name == "main" {
			continue
		}
		return doc.New(p, importPath, doc.AllDecls), nil
	}
	return nil, fmt.Errorf("no Go package under %s", dir)
}

func exported(name string) bool {
	return name != "" && name[0] >= 'A' && name[0] <= 'Z'
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if i := strings.Index(s, "\n"); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

func pkgImportPath(sub string) string {
	if sub == "" {
		return modPath
	}
	return modPath + "/" + sub
}

func pkgGoDevURL(sub, symbol, version string) string {
	base := "https://pkg.go.dev/" + pkgImportPath(sub)
	if version != "" && version != "latest" {
		base += "@" + version
	}
	return base + "#" + symbol
}

func escapeMD(s string) string {
	if s == "" {
		return "—"
	}
	// Replace `{%` and `{{` with HTML-entity equivalents to avoid
	// triggering the include-markdown plugin (which treats `{% include
	// ... %}` and `{% include-markdown ... %}` as preprocessor
	// directives). Synopses commonly contain literal Jinja syntax in
	// godoc examples, so this escape is required.
	r := strings.NewReplacer(
		"|", `\|`,
		"\n", " ",
		"\r", " ",
		"{%", "&#123;%",
		"{{", "&#123;{",
	)
	return r.Replace(s)
}
