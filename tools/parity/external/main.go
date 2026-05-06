// External parity harness — drives the gojinja-vs-Python-Jinja2 comparison
// across templates fetched from ten upstream open-source projects (see
// SOURCES.md for the canonical list and licensing).
//
// Usage from the repo root:
//
//	go run ./tools/parity/external -fetch              # download tarballs
//	go run ./tools/parity/external                     # parse-parity over cache
//	go run ./tools/parity/external -render             # render-parity over cache
//	go run ./tools/parity/external -only sphinx        # restrict to one project
//	go run ./tools/parity/external -fetch -render -v   # all of the above
//
// The fetched cache lives under tools/parity/external/cache/ and is git-
// ignored. License files are written next to each project's templates as
// __LICENSE__/<basename>; the original upstream commit is recorded in
// __MANIFEST__.json. Re-running -fetch overwrites the cache.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	var (
		cacheDir = flag.String("cache", "tools/parity/external/cache", "where fetched templates live")
		pyScript = flag.String("py", "tools/parity/external/parity.py", "path to the Python parity helper")
		fetch    = flag.Bool("fetch", false, "download/refresh the cache before validating")
		render   = flag.Bool("render", false, "run render-parity (default is parse-parity)")
		loose    = flag.Bool("loose", false, "loose render mode: stub unknown filters/tests, ChainableUndefined, fallback loader")
		verbose  = flag.Bool("v", false, "print per-item parity results")
		only     = flag.String("only", "", "limit to one project name (e.g. sphinx, mkdocs)")
		validate = flag.Bool("validate", true, "run validation after fetch (off if you only want to refresh the cache)")
	)
	flag.Parse()

	abs, err := filepath.Abs(*cacheDir)
	if err != nil {
		fail("resolve cache: %v", err)
	}

	if *fetch {
		if err := fetchAll(abs); err != nil {
			fail("fetch: %v", err)
		}
		fmt.Printf("Fetch complete: %s\n", abs)
		if !*validate {
			return
		}
	}

	mode := "parse"
	switch {
	case *loose && !*render:
		fail("-loose requires -render")
	case *loose:
		mode = "render-loose"
	case *render:
		mode = "render"
	}
	if err := runParity(abs, mode, *pyScript, *only, *verbose); err != nil {
		fail("%v", err)
	}
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
