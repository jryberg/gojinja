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

// runBuild generates per-name reference pages for filters, tests, and
// globals.
//
// Resolution per registered (kind, name):
//  1. If docs/site/content/templates/<kind>s/<name>.md exists (a hand-
//     written override), leave it alone — its filename in the override
//     directory takes precedence over any generated content.
//  2. Otherwise, look up the godoc on the underlying private function
//     in pkg/environment. If present, render it into
//     {kind}s/_generated/<name>.md with a "generated, file an override
//     to customise" header.
//  3. If neither exists, print a warning. The check subcommand turns
//     this into a non-zero exit.
//
// Always rewrites the per-kind _index.md as a sorted table of contents
// so the nav and on-page TOC stay in sync with the registry.
func runBuild(repoRoot, outDir string) error {
	entries, err := loadRegistries(repoRoot)
	if err != nil {
		return err
	}
	pkgDocs, err := loadPackageDocs(filepath.Join(repoRoot, "pkg", "environment"))
	if err != nil {
		return fmt.Errorf("load environment docs: %w", err)
	}
	// Group entries by (kind, FuncName) so each group is "one canonical
	// + zero or more aliases".
	groupsByKindFunc := map[string]map[string][]Entry{}
	for _, e := range entries {
		if _, ok := groupsByKindFunc[e.Kind]; !ok {
			groupsByKindFunc[e.Kind] = map[string][]Entry{}
		}
		groupsByKindFunc[e.Kind][e.FuncName] = append(groupsByKindFunc[e.Kind][e.FuncName], e)
	}
	for kind, byFunc := range groupsByKindFunc {
		dir := filepath.Join(outDir, pluralise(kind))
		genDir := filepath.Join(dir, "_generated")
		if err := os.MkdirAll(genDir, 0o755); err != nil {
			return err
		}
		if err := wipeGenerated(genDir); err != nil {
			return err
		}
		var missing []string
		var groups []groupSummary
		for _, group := range byFunc {
			canonical, aliases := canonicalAndAliases(group)
			gs := groupSummary{
				Canonical: canonical.Name,
				Slug:      slugify(canonical.Name),
				Aliases:   aliases,
				FuncName:  canonical.FuncName,
			}
			overridePath := filepath.Join(dir, gs.Slug+".md")
			if _, err := os.Stat(overridePath); err == nil {
				gs.HasOverride = true
				groups = append(groups, gs)
				continue
			}
			content, ok := renderFromGodoc(pkgDocs, kind, canonical, aliases)
			if !ok {
				missing = append(missing,
					fmt.Sprintf("%s/%s (no override, no godoc on %s)",
						kind, canonical.Name, canonical.FuncName))
				groups = append(groups, gs)
				continue
			}
			genPath := filepath.Join(genDir, gs.Slug+".md")
			if err := os.WriteFile(genPath, []byte(content), 0o644); err != nil {
				return err
			}
			groups = append(groups, gs)
		}
		if err := writeIndex(dir, kind, groups); err != nil {
			return err
		}
		for _, m := range missing {
			fmt.Fprintln(os.Stderr, "WARN: missing docs:", m)
		}
	}
	return nil
}

// groupSummary is one row in the per-kind index: the canonical name,
// its slug (sanitised filename), its aliases, and whether the rendered
// page came from a hand-authored override or generated from godoc.
type groupSummary struct {
	Canonical   string
	Slug        string
	Aliases     []string
	FuncName    string
	HasOverride bool
}

// pluralise returns the on-disk subdir for a kind ("filter" → "filters").
func pluralise(kind string) string {
	switch kind {
	case "filter", "test", "global":
		return kind + "s"
	}
	return kind
}

func wipeGenerated(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		_ = os.Remove(filepath.Join(dir, e.Name()))
	}
	return nil
}

// loadPackageDocs reads every Go file under dir and returns go/doc data
// for the package — used to look up a func's godoc by identifier.
func loadPackageDocs(dir string) (*doc.Package, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	for _, p := range pkgs {
		// `Package` may panic on synthetic test packages; skip names that
		// end in "_test".
		if strings.HasSuffix(p.Name, "_test") {
			continue
		}
		return doc.New(p, "github.com/jryberg/gojinja/pkg/environment", doc.AllDecls), nil
	}
	return nil, fmt.Errorf("no non-test package found under %s", dir)
}

// renderFromGodoc looks up the func by name and renders its godoc as
// the body of a generated reference page. Returns ok=false when the
// func has no godoc comment.
func renderFromGodoc(pkg *doc.Package, kind string, canonical Entry, aliases []string) (string, bool) {
	godoc := lookupFuncDoc(pkg, canonical.FuncName)
	if strings.TrimSpace(godoc) == "" {
		return "", false
	}
	slug := slugify(canonical.Name)
	var b strings.Builder
	fmt.Fprintf(&b, "# `%s`\n\n", canonical.Name)
	if len(aliases) > 0 {
		fmt.Fprintf(&b, "*Aliases: `%s`*\n\n", strings.Join(aliases, "`, `"))
	}
	fmt.Fprintf(&b,
		"!!! note \"Generated\"\n"+
			"    This page is generated from the godoc comment on `%s`\n"+
			"    in `pkg/environment`. To customize, drop a hand-written file at\n"+
			"    `docs/site/content/templates/%s/%s.md`.\n\n",
		canonical.FuncName, pluralise(kind), slug)
	body := normaliseGodoc(godoc)
	b.WriteString(body)
	if !strings.HasSuffix(body, "\n") {
		b.WriteString("\n")
	}
	return b.String(), true
}

// lookupFuncDoc returns the godoc text for the named func, or "".
func lookupFuncDoc(pkg *doc.Package, name string) string {
	for _, f := range pkg.Funcs {
		if f.Name == name {
			return f.Doc
		}
	}
	for _, t := range pkg.Types {
		for _, f := range t.Funcs {
			if f.Name == name {
				return f.Doc
			}
		}
		for _, m := range t.Methods {
			if m.Name == name {
				return m.Doc
			}
		}
	}
	return ""
}

// normaliseGodoc strips the leading "// funcName " prefix from the
// godoc comment so the rendered page reads naturally — the func name is
// already in the H1.
func normaliseGodoc(godoc string) string {
	lines := strings.Split(strings.TrimRight(godoc, "\n"), "\n")
	if len(lines) > 0 {
		// Strip a leading "funcName implements ..." sentence — leaves
		// the body in place.
		first := lines[0]
		if i := strings.Index(first, ": "); i > 0 && strings.Contains(first[:i], " ") {
			lines[0] = strings.ToUpper(first[i+2:i+3]) + first[i+3:]
		}
	}
	// Drop common leading "Signature:" / "Example:" headers and replace
	// with bolded sub-heads for nicer rendering.
	var out []string
	for _, l := range lines {
		switch {
		case strings.HasPrefix(l, "Signature:"):
			out = append(out, "**Signature:** `"+strings.TrimSpace(l[len("Signature:"):])+"`")
		case strings.HasPrefix(l, "Example:"):
			out = append(out, "**Example:**")
		case strings.HasPrefix(l, "Behavior:"):
			out = append(out, "**Behavior:** "+strings.TrimSpace(l[len("Behavior:"):]))
		case strings.HasPrefix(l, "Divergence from Python:"), strings.HasPrefix(l, "Divergences from Python:"):
			out = append(out, "**Divergences from Python:** "+strings.TrimSpace(l[strings.Index(l, ":")+1:]))
		default:
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}

// writeIndex writes a sorted Markdown table of contents for a kind.
func writeIndex(dir, kind string, groups []groupSummary) error {
	sort.Slice(groups, func(i, j int) bool { return groups[i].Canonical < groups[j].Canonical })

	var b strings.Builder
	fmt.Fprintf(&b, "# %s reference\n\n", strings.ToUpper(kind[:1])+kind[1:]+"s")
	fmt.Fprintf(&b, "Auto-generated from `pkg/environment`. To customize a page, drop a\n")
	fmt.Fprintf(&b, "hand-written `<slug>.md` next to this file (it overrides the\n")
	fmt.Fprintf(&b, "generated content).\n\n")
	fmt.Fprintf(&b, "| Name | Aliases | Page |\n")
	fmt.Fprintf(&b, "|---|---|---|\n")
	for _, g := range groups {
		aliases := "—"
		if len(g.Aliases) > 0 {
			aliases = "`" + strings.Join(g.Aliases, "`, `") + "`"
		}
		page := g.Slug + ".md"
		if !g.HasOverride {
			page = "_generated/" + g.Slug + ".md"
		}
		fmt.Fprintf(&b, "| [`%s`](%s) | %s | %s |\n",
			g.Canonical, page, aliases, mark(g.HasOverride))
	}
	indexPath := filepath.Join(dir, "index.md")
	return os.WriteFile(indexPath, []byte(b.String()), 0o644)
}

func mark(override bool) string {
	if override {
		return "hand-authored"
	}
	return "from godoc"
}
