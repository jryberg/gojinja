// docgen generates Markdown reference pages for the gojinja docs site
// (docs/site/) directly from the Go source.
//
// Subcommands:
//
//	docgen build   --out docs/site/content/templates
//	    Walks pkg/environment/{builtins.go,filters_extra.go,globals.go} for
//	    filter / test / global registrations. For each registered name,
//	    resolves a hand-written override page or falls back to the
//	    structured godoc on the underlying Go function. Writes per-name
//	    Markdown under {filters,tests,globals}/_generated/<name>.md and a
//	    sorted _index.md per kind.
//
//	docgen symbols --out docs/site/content/api/symbols.md
//	    Walks every package under pkg/* (and the root gojinja package) and
//	    emits a single Markdown table of every exported symbol → first-line
//	    summary → pkg.go.dev link. The pkg.go.dev version comes from
//	    MIKE_VERSION (set by the docs workflow); falls back to "latest".
//
//	docgen check
//	    Runs build to a temp directory and fails non-zero if any registered
//	    filter / test / global has neither an override page nor a godoc
//	    comment on its underlying Go function. Wired into `make docs-check`
//	    and `make audit`.
//
// Stdlib-only by policy. No external Go dependencies.
package main

import (
	"flag"
	"fmt"
	"os"
)

const usage = `usage: docgen <subcommand> [flags]

Subcommands:
  build    Generate per-name reference pages under docs/site/content/templates/.
  symbols  Generate the API symbols index at docs/site/content/api/symbols.md.
  check    Verify every registered filter/test/global has either an override
           page or a godoc comment on its underlying func.

Run "docgen <subcommand> --help" for subcommand-specific flags.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	sub := os.Args[1]
	args := os.Args[2:]
	switch sub {
	case "build":
		fs := flag.NewFlagSet("build", flag.ExitOnError)
		out := fs.String("out", "docs/site/content/templates", "output directory")
		repo := fs.String("repo", ".", "repository root")
		_ = fs.Parse(args)
		if err := runBuild(*repo, *out); err != nil {
			fmt.Fprintln(os.Stderr, "docgen build:", err)
			os.Exit(1)
		}
	case "symbols":
		fs := flag.NewFlagSet("symbols", flag.ExitOnError)
		out := fs.String("out", "docs/site/content/api/symbols.md", "output file")
		repo := fs.String("repo", ".", "repository root")
		_ = fs.Parse(args)
		if err := runSymbols(*repo, *out); err != nil {
			fmt.Fprintln(os.Stderr, "docgen symbols:", err)
			os.Exit(1)
		}
	case "check":
		fs := flag.NewFlagSet("check", flag.ExitOnError)
		repo := fs.String("repo", ".", "repository root")
		_ = fs.Parse(args)
		if err := runCheck(*repo); err != nil {
			fmt.Fprintln(os.Stderr, "docgen check:", err)
			os.Exit(1)
		}
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "docgen: unknown subcommand %q\n%s", sub, usage)
		os.Exit(2)
	}
}
