package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// runCheck verifies every registered filter / test / global has either a
// hand-authored override page under
// docs/site/content/templates/<kind>s/<name>.md OR a non-empty godoc
// comment on the underlying Go function.
//
// Wired into make audit so PRs that introduce a new filter without docs
// fail CI alongside go test / go vet / parity.
//
// Exit codes:
//   - 0: every registered name has a doc source
//   - 1: at least one is missing (errors printed to stderr)
func runCheck(repoRoot string) error {
	entries, err := loadRegistries(repoRoot)
	if err != nil {
		return err
	}
	pkgDocs, err := loadPackageDocs(filepath.Join(repoRoot, "pkg", "environment"))
	if err != nil {
		return fmt.Errorf("load environment docs: %w", err)
	}
	contentDir := filepath.Join(repoRoot, "docs", "site", "content", "templates")
	// Group by (kind, FuncName) so we check coverage per canonical, not
	// per alias.
	byKindFunc := map[string]map[string][]Entry{}
	for _, e := range entries {
		if _, ok := byKindFunc[e.Kind]; !ok {
			byKindFunc[e.Kind] = map[string][]Entry{}
		}
		byKindFunc[e.Kind][e.FuncName] = append(byKindFunc[e.Kind][e.FuncName], e)
	}
	var missing []string
	for kind, byFunc := range byKindFunc {
		for _, group := range byFunc {
			canonical, _ := canonicalAndAliases(group)
			slug := slugify(canonical.Name)
			overridePath := filepath.Join(contentDir, pluralise(kind), slug+".md")
			if _, err := os.Stat(overridePath); err == nil {
				continue
			}
			godoc := lookupFuncDoc(pkgDocs, canonical.FuncName)
			if strings.TrimSpace(godoc) != "" {
				continue
			}
			missing = append(missing,
				fmt.Sprintf("%s %q (Go func %s) lacks both an override page and a godoc comment",
					kind, canonical.Name, canonical.FuncName))
		}
	}
	if len(missing) > 0 {
		fmt.Fprintln(os.Stderr, "docgen check: documentation coverage gaps:")
		for _, m := range missing {
			fmt.Fprintln(os.Stderr, "  -", m)
		}
		fmt.Fprintln(os.Stderr,
			"\nFix one of:\n"+
				"  - add a structured godoc comment to the underlying func, or\n"+
				"  - drop a hand-written page at docs/site/content/templates/<kind>s/<name>.md")
		os.Exit(1)
	}
	fmt.Println("docgen check: all registered filters / tests / globals have docs")
	return nil
}
