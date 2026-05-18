// Command parity drives the Python-vs-gojinja parity harness.
//
// For each `name.j2` + `name.vars.json` pair under ./corpus/, it renders
// the template with gojinja, runs ./parity.py against the same inputs,
// and diffs the two outputs byte-by-byte. Each case is also rendered
// multiple times via gojinja (`-stability` flag) to assert
// non-determinism never sneaks back in — Go's randomised map iteration
// has historically leaked into rendered output when a template iterates
// a context dict, and the stability loop is a regression gate.
//
// Usage from the repo root:
//
//	go run ./tools/parity
//	make parity
//
// The Python side is invoked via the `python3` on $PATH; ensure
// `python3 -m pip install jinja2` is available. Names matching the
// optional -filter flag run; the rest are skipped.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	gj "github.com/jryberg/gojinja"
	"github.com/jryberg/gojinja/pkg/lexer"
	"github.com/jryberg/gojinja/pkg/runtime"
	"github.com/jryberg/gojinja/pkg/varsutil"
)

func main() {
	corpus := flag.String("corpus", "tools/parity/corpus", "path to the corpus directory")
	pyScript := flag.String("py", "tools/parity/parity.py", "path to the Python parity script")
	filter := flag.String("filter", "", "only run cases whose basename contains this substring")
	verbose := flag.Bool("v", false, "log per-case status")
	stability := flag.Int("stability", 3, "render each case this many times with gojinja and fail if the outputs differ across runs (1 disables the check)")
	flag.Parse()

	cases, err := discover(*corpus)
	if err != nil {
		fail("discover: %v", err)
	}
	sort.Strings(cases)

	var failures []string
	for _, name := range cases {
		if *filter != "" && !strings.Contains(name, *filter) {
			continue
		}
		ok, err := runCase(*corpus, name, *pyScript, *stability)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", name, err))
			fmt.Printf("FAIL %s: %v\n", name, err)
			continue
		}
		if !ok {
			failures = append(failures, name)
			continue
		}
		if *verbose {
			fmt.Printf("OK   %s\n", name)
		}
	}

	if len(failures) > 0 {
		fmt.Printf("\n%d failure(s)\n", len(failures))
		os.Exit(1)
	}
	fmt.Printf("all %d case(s) pass\n", len(cases))
}

// discover returns the case names under dir. A case is either:
//   - flat: `<name>.j2` + `<name>.vars.json`
//   - directory: `<name>/_main.j2` + `<name>/_main.vars.json` plus zero or
//     more sibling `*.j2` templates loadable by `{% extends %}` etc.
func discover(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() {
			mainPath := filepath.Join(dir, n, "_main.j2")
			varsPath := filepath.Join(dir, n, "_main.vars.json")
			if _, err := os.Stat(mainPath); err != nil {
				continue
			}
			if _, err := os.Stat(varsPath); err != nil {
				return nil, fmt.Errorf("%s: missing _main.vars.json", n)
			}
			names = append(names, n)
			continue
		}
		if !strings.HasSuffix(n, ".j2") {
			continue
		}
		base := strings.TrimSuffix(n, ".j2")
		varsPath := filepath.Join(dir, base+".vars.json")
		if _, err := os.Stat(varsPath); err != nil {
			return nil, fmt.Errorf("%s: missing %s", n, filepath.Base(varsPath))
		}
		names = append(names, base)
	}
	return names, nil
}

// caseLayout resolves a case name to its template path, vars path, and
// the optional sibling-template directory (for multi-file cases).
type caseLayout struct {
	tmplPath  string
	varsPath  string
	loaderDir string // empty for flat cases
}

func resolveCase(dir, name string) caseLayout {
	multi := filepath.Join(dir, name)
	if st, err := os.Stat(multi); err == nil && st.IsDir() {
		return caseLayout{
			tmplPath:  filepath.Join(multi, "_main.j2"),
			varsPath:  filepath.Join(multi, "_main.vars.json"),
			loaderDir: multi,
		}
	}
	return caseLayout{
		tmplPath: filepath.Join(dir, name+".j2"),
		varsPath: filepath.Join(dir, name+".vars.json"),
	}
}

// runCase renders one case with gojinja and Python, comparing the
// outputs byte-by-byte. Also re-renders with gojinja `stability` times
// (when ≥2) to assert determinism — Go's map iteration is randomised,
// and historic bugs have leaked that into template output.
// Returns (true, nil) on parity + stability, (false, nil) when outputs
// differ (a diff is printed), or an error for harness failures.
func runCase(dir, name, pyScript string, stability int) (bool, error) {
	c := resolveCase(dir, name)
	src, err := os.ReadFile(c.tmplPath)
	if err != nil {
		return false, err
	}
	rawVars, err := os.ReadFile(c.varsPath)
	if err != nil {
		return false, err
	}
	vars, err := varsutil.JSONVars(rawVars)
	if err != nil {
		return false, fmt.Errorf("vars.json: %w", err)
	}

	goOut, err := renderGo(string(src), vars, c.loaderDir)
	if err != nil {
		return false, fmt.Errorf("gojinja render: %w", err)
	}
	// Stability check: re-render with the same vars; fail on any drift.
	for i := 1; i < stability; i++ {
		out, err := renderGo(string(src), vars, c.loaderDir)
		if err != nil {
			return false, fmt.Errorf("gojinja render (rerun %d): %w", i, err)
		}
		if out != goOut {
			fmt.Printf("NON-DETERMINISTIC %s (rerun %d)\n", name, i)
			fmt.Printf("--- run 0\n%s\n--- run %d\n%s\n---\n",
				quote(goOut), i, quote(out))
			return false, nil
		}
	}

	pyOut, err := renderPython(pyScript, c.tmplPath, c.varsPath, c.loaderDir)
	if err != nil {
		return false, fmt.Errorf("python render: %w", err)
	}
	if goOut == pyOut {
		return true, nil
	}
	fmt.Printf("DIFF %s\n", name)
	fmt.Printf("--- gojinja\n%s\n--- python\n%s\n---\n",
		quote(goOut), quote(pyOut))
	return false, nil
}

func renderGo(src string, vars *runtime.OrderedDict, loaderDir string) (string, error) {
	lexOpts := lexer.DefaultOptions()
	lexOpts.KeepTrailingNewline = true
	opts := []gj.Option{
		gj.WithAutoescape(gj.AutoescapeNever{}),
		gj.WithLexerOptions(lexOpts),
	}
	if loaderDir != "" {
		// Multi-file case: every sibling .j2 (except _main.j2) is
		// loadable via the configured DictLoader so `{% extends %}`
		// resolves the same names Python sees.
		dl, err := loadDictLoader(loaderDir)
		if err != nil {
			return "", err
		}
		opts = append(opts, gj.WithLoader(dl))
	}
	env, err := gj.New(opts...)
	if err != nil {
		return "", err
	}
	t, err := env.FromString(src)
	if err != nil {
		return "", err
	}
	return t.RenderContext(context.Background(), vars)
}

// loadDictLoader builds a Loader from every `*.j2` in dir other than
// `_main.j2`. Names are stripped of the `.j2` suffix so
// `{% extends "base" %}` resolves `base.j2`.
func loadDictLoader(dir string) (gj.Loader, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	m := map[string]string{}
	for _, e := range entries {
		n := e.Name()
		if !strings.HasSuffix(n, ".j2") || n == "_main.j2" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			return nil, err
		}
		m[strings.TrimSuffix(n, ".j2")] = string(raw)
	}
	return mapLoader(m), nil
}

// mapLoader is an in-memory name→source map. Mirrors loader.Dict, kept
// local to the parity tool because the upstream Dict's not-found error
// is wrapped in TemplateNotFound — this one returns a plain error so
// the parity script's logging stays simple.
type mapLoader map[string]string

func (m mapLoader) GetSource(name string) (gj.Source, error) {
	src, ok := m[name]
	if !ok {
		return gj.Source{}, fmt.Errorf("template not found: %s", name)
	}
	return gj.Source{Code: src, Filename: name}, nil
}

func renderPython(script, tmpl, vars, loaderDir string) (string, error) {
	args := []string{script, tmpl, vars}
	if loaderDir != "" {
		args = append(args, loaderDir)
	}
	cmd := exec.Command("python3", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%v: %s", err, stderr.String())
	}
	return stdout.String(), nil
}

func quote(s string) string {
	// Quote newlines + non-printables visibly so a multi-line diff still
	// fits in the terminal.
	r := strings.ReplaceAll(s, "\n", "\\n\n")
	return r
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(2)
}
