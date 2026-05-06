// Parity orchestrator: walks tools/parity/external/cache/, runs each
// fetched template through gojinja and through the Python parity.py
// helper, and reports diff vs. parity per project.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	gj "github.com/jryberg/gojinja"
	"github.com/jryberg/gojinja/pkg/ast"
	gjenv "github.com/jryberg/gojinja/pkg/environment"
	"github.com/jryberg/gojinja/pkg/lexer"
	"github.com/jryberg/gojinja/pkg/parser"
	"github.com/jryberg/gojinja/pkg/runtime"
)

// item is one template under test.
type item struct {
	ID         string `json:"id"`          // "<project>:<rel>"
	Path       string `json:"path"`        // absolute filesystem path
	LoaderRoot string `json:"loader_root"` // empty for parse mode
	project    string // not serialized — for grouping
	rel        string
}

// pyResult mirrors the shape parity.py emits.
type pyResult struct {
	ID  string `json:"id"`
	OK  bool   `json:"ok"`
	Out string `json:"out,omitempty"`
	Err string `json:"err,omitempty"`
}

// goResult is the gojinja-side outcome for one item.
type goResult struct {
	ID  string
	OK  bool
	Out string
	Err string
}

// runParity is the harness entrypoint. mode is either "parse" or "render".
func runParity(cacheDir, mode, pyScript, only string, verbose bool) error {
	items, err := discoverItems(cacheDir, mode, only)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return fmt.Errorf("no items discovered under %s (run with -fetch first?)", cacheDir)
	}
	fmt.Printf("Discovered %d template(s) across %d project(s); mode=%s\n",
		len(items), countProjects(items), mode)

	pyOut, err := callPython(pyScript, mode, items)
	if err != nil {
		return fmt.Errorf("python helper: %w", err)
	}
	pyMap := indexByID(pyOut)

	goOut := runGo(items, mode)

	report := make(map[string]*projectReport)
	for _, it := range items {
		r := report[it.project]
		if r == nil {
			r = &projectReport{name: it.project}
			report[it.project] = r
		}
		r.total++

		py, ok := pyMap[it.ID]
		if !ok {
			r.harnessErr++
			r.harnessNotes = append(r.harnessNotes, fmt.Sprintf("%s: missing python result", it.ID))
			continue
		}
		go_ := goOut[it.ID]

		switch {
		case py.OK && go_.OK:
			r.bothOK++
			if isRender(mode) && py.Out != go_.Out {
				r.outputDiff++
				r.diffNotes = append(r.diffNotes, it.ID)
			}
		case !py.OK && !go_.OK:
			r.bothFail++
		case py.OK && !go_.OK:
			r.pyOnlyOK++
			r.divNotes = append(r.divNotes,
				fmt.Sprintf("py-only: %s | go-err: %s", it.ID, oneLine(go_.Err)))
		case !py.OK && go_.OK:
			r.goOnlyOK++
			r.divNotes = append(r.divNotes,
				fmt.Sprintf("go-only: %s | py-err: %s", it.ID, oneLine(py.Err)))
		}

		if verbose {
			fmt.Printf("  %-50s py=%v go=%v\n", it.ID, py.OK, go_.OK)
		}
	}

	printReport(report, mode)

	// Non-zero exit if either engine produced an outright divergence
	// (one parsed/rendered, the other didn't) or a render-mode byte
	// diff. Both-fail is parity, not a failure.
	for _, r := range report {
		if r.pyOnlyOK > 0 || r.goOnlyOK > 0 || r.outputDiff > 0 {
			return fmt.Errorf("parity divergences detected")
		}
	}
	return nil
}

type projectReport struct {
	name         string
	total        int
	bothOK       int
	bothFail     int
	pyOnlyOK     int
	goOnlyOK     int
	outputDiff   int
	harnessErr   int
	divNotes     []string
	diffNotes    []string
	harnessNotes []string
}

func printReport(rs map[string]*projectReport, mode string) {
	keys := make([]string, 0, len(rs))
	for k := range rs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	fmt.Println()
	fmt.Println("Parity report")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Printf("%-18s %5s %7s %9s %9s %9s %7s\n",
		"project", "total", "both-ok", "both-fail", "py-only", "go-only", "diff")
	fmt.Println(strings.Repeat("-", 70))
	tot := projectReport{name: "TOTAL"}
	for _, k := range keys {
		r := rs[k]
		fmt.Printf("%-18s %5d %7d %9d %9d %9d %7d\n",
			r.name, r.total, r.bothOK, r.bothFail, r.pyOnlyOK, r.goOnlyOK, r.outputDiff)
		tot.total += r.total
		tot.bothOK += r.bothOK
		tot.bothFail += r.bothFail
		tot.pyOnlyOK += r.pyOnlyOK
		tot.goOnlyOK += r.goOnlyOK
		tot.outputDiff += r.outputDiff
	}
	fmt.Println(strings.Repeat("-", 70))
	fmt.Printf("%-18s %5d %7d %9d %9d %9d %7d\n",
		tot.name, tot.total, tot.bothOK, tot.bothFail, tot.pyOnlyOK, tot.goOnlyOK, tot.outputDiff)

	for _, k := range keys {
		r := rs[k]
		if len(r.divNotes) == 0 && len(r.diffNotes) == 0 && len(r.harnessNotes) == 0 {
			continue
		}
		fmt.Println()
		fmt.Printf("[%s] divergences\n", r.name)
		const cap = 8
		notes := r.divNotes
		if len(notes) > cap {
			fmt.Printf("  (showing first %d of %d)\n", cap, len(notes))
			notes = notes[:cap]
		}
		for _, n := range notes {
			fmt.Printf("  - %s\n", n)
		}
		if isRender(mode) && len(r.diffNotes) > 0 {
			more := r.diffNotes
			if len(more) > cap {
				fmt.Printf("  (byte-diff: first %d of %d)\n", cap, len(more))
				more = more[:cap]
			} else {
				fmt.Printf("  byte-diff cases:\n")
			}
			for _, n := range more {
				fmt.Printf("  - %s\n", n)
			}
		}
		for _, n := range r.harnessNotes {
			fmt.Printf("  ! %s\n", n)
		}
	}
}

func runGo(items []item, mode string) map[string]goResult {
	out := make(map[string]goResult, len(items))
	for _, it := range items {
		out[it.ID] = runGoOne(it, mode)
	}
	return out
}

// runGoOne runs the gojinja side for a single item with panic recovery.
// A panic in the engine surface is itself a divergence — Python Jinja2
// raises an exception, we want to tag the panic and keep going so the
// harness reports every offender in one pass.
func runGoOne(it item, mode string) (res goResult) {
	defer func() {
		if r := recover(); r != nil {
			res = goResult{ID: it.ID, Err: fmt.Sprintf("panic: %v", r)}
		}
	}()
	src, err := os.ReadFile(it.Path)
	if err != nil {
		return goResult{ID: it.ID, Err: err.Error()}
	}
	switch mode {
	case "parse":
		return goParse(it.ID, string(src))
	case "render":
		return goRender(it.ID, string(src), it.LoaderRoot)
	case "render-loose":
		return goRenderLoose(it.ID, string(src), it.LoaderRoot)
	default:
		return goResult{ID: it.ID, Err: "unknown mode"}
	}
}

func goParse(id, src string) goResult {
	// Parse-mode mirrors Python's `env.parse(source)`: lex + parse, no
	// compile-time validation. Going through env.FromString would also
	// run the filter/test name check (which Jinja2 performs at compile
	// time, not parse time) and produce false-positive divergences.
	lex := lexer.DefaultOptions()
	lex.KeepTrailingNewline = true
	l, err := lexer.New(lex)
	if err != nil {
		return goResult{ID: id, Err: err.Error()}
	}
	stream, err := l.Tokenize(src, "", "")
	if err != nil {
		return goResult{ID: id, Err: err.Error()}
	}
	if _, err := parser.New(stream).Parse(); err != nil {
		return goResult{ID: id, Err: err.Error()}
	}
	return goResult{ID: id, OK: true}
}

func goRender(id, src, loaderRoot string) goResult {
	lex := lexer.DefaultOptions()
	lex.KeepTrailingNewline = true
	opts := []gj.Option{
		gj.WithAutoescape(gj.AutoescapeNever{}),
		gj.WithLexerOptions(lex),
	}
	if loaderRoot != "" {
		fl, err := gj.NewFileSystemLoader([]string{loaderRoot})
		if err != nil {
			return goResult{ID: id, Err: "loader: " + err.Error()}
		}
		opts = append(opts, gj.WithLoader(fl))
	}
	env, err := gj.New(opts...)
	if err != nil {
		return goResult{ID: id, Err: err.Error()}
	}
	t, err := env.FromString(src)
	if err != nil {
		return goResult{ID: id, Err: err.Error()}
	}
	rendered, err := t.RenderContext(context.Background(), map[string]any{})
	if err != nil {
		return goResult{ID: id, Err: err.Error()}
	}
	return goResult{ID: id, OK: true, Out: rendered}
}

func discoverItems(cacheDir, mode, only string) ([]item, error) {
	var items []item
	for _, p := range projects {
		if only != "" && only != p.Name {
			continue
		}
		root := filepath.Join(cacheDir, p.Name)
		if _, err := os.Stat(root); err != nil {
			continue
		}
		err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if info.IsDir() {
				if info.Name() == "__LICENSE__" {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(path, "__MANIFEST__.json") {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			it := item{
				ID:      p.Name + ":" + filepath.ToSlash(rel),
				Path:    path,
				project: p.Name,
				rel:     rel,
			}
			if isRender(mode) && p.LoaderRoot != "" {
				it.LoaderRoot = filepath.Join(root, p.LoaderRoot)
			}
			items = append(items, it)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items, nil
}

func countProjects(items []item) int {
	seen := map[string]struct{}{}
	for _, it := range items {
		seen[it.project] = struct{}{}
	}
	return len(seen)
}

func callPython(script, mode string, items []item) ([]pyResult, error) {
	job := struct {
		Mode  string `json:"mode"`
		Items []item `json:"items"`
	}{Mode: mode, Items: items}
	in, err := json.Marshal(job)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command("python3", script)
	cmd.Stdin = bytes.NewReader(in)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%v: %s", err, stderr.String())
	}
	var out []pyResult
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		return nil, fmt.Errorf("parse python output: %w (stderr: %s)", err, stderr.String())
	}
	return out, nil
}

func indexByID(rs []pyResult) map[string]pyResult {
	out := make(map[string]pyResult, len(rs))
	for _, r := range rs {
		out[r.ID] = r
	}
	return out
}

func oneLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = s[:i]
	}
	if len(s) > 200 {
		s = s[:200] + "…"
	}
	return s
}

// looseLoader wraps a primary loader and falls back to an empty
// template for any name the primary doesn't resolve. Mirrors the Python
// _StubLoader so {% extends "missing" %} / {% include "missing" %}
// produce the same parity outcome on both sides.
type looseLoader struct{ inner gj.Loader }

func (l looseLoader) GetSource(name string) (gj.Source, error) {
	if l.inner != nil {
		if s, err := l.inner.GetSource(name); err == nil {
			return s, nil
		}
	}
	return gj.Source{Code: "", Filename: name}, nil
}

// passthroughFilter mirrors Python's `lambda value, *a, **k: value` —
// the value flows through unchanged regardless of args/kwargs. Used to
// stub project-specific filters (Ansible's to_yaml, dbt's is_list,
// mkdocs-material's url, …) so unknown filter names render rather than
// abort. Both engines install the identical stub so any subsequent
// divergence is real.
func passthroughFilter(_ *gj.Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (any, error) {
	return value, nil
}

func passthroughTest(_ *gj.Environment, _ *runtime.Context, _ any, _ []any, _ map[string]any) (bool, error) {
	return true, nil
}

// scanFilterTestNames walks the AST and returns every Filter and Test
// name referenced in src. Mirrors the Python side's
// `_scan_filter_test_names`. Both sides parse the same source, so the
// returned name sets agree.
func scanFilterTestNames(src string) (filters, tests map[string]struct{}) {
	filters = map[string]struct{}{}
	tests = map[string]struct{}{}
	lex := lexer.DefaultOptions()
	lex.KeepTrailingNewline = true
	l, err := lexer.New(lex)
	if err != nil {
		return
	}
	stream, err := l.Tokenize(src, "", "")
	if err != nil {
		return
	}
	tpl, err := parser.New(stream).Parse()
	if err != nil {
		return
	}
	ast.Walk(tpl, ast.VisitorFunc(func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.Filter:
			filters[x.Name] = struct{}{}
		case *ast.Test:
			tests[x.Name] = struct{}{}
		}
		return true
	}))
	return
}

func goRenderLoose(id, src, loaderRoot string) goResult {
	lex := lexer.DefaultOptions()
	lex.KeepTrailingNewline = true
	opts := []gj.Option{
		gj.WithAutoescape(gj.AutoescapeNever{}),
		gj.WithLexerOptions(lex),
		gj.WithUndefined(runtime.NewChainable),
	}
	var inner gj.Loader
	if loaderRoot != "" {
		fl, err := gj.NewFileSystemLoader([]string{loaderRoot})
		if err != nil {
			return goResult{ID: id, Err: "loader: " + err.Error()}
		}
		inner = fl
	}
	opts = append(opts, gj.WithLoader(looseLoader{inner: inner}))

	// Mirror the Python helper: only register a passthrough for filters
	// and tests that the env doesn't already know about. We need a
	// throwaway env first to learn the stdlib registry — registering
	// the stub *first* and the stdlib *second* would still flip the
	// loser/winner since gj.New applies options in order, but querying
	// the registry directly is clearer and matches Python's
	// `if name not in env.filters`.
	probe, err := gj.New(opts...)
	if err != nil {
		return goResult{ID: id, Err: err.Error()}
	}
	known := stringSet(probe.Filters())
	knownTests := stringSet(probe.Tests())

	filters, tests := scanFilterTestNames(src)
	for name := range filters {
		if _, ok := known[name]; ok {
			continue
		}
		opts = append(opts, gj.WithFilter(name, gjenv.Filter{Func: passthroughFilter}))
	}
	for name := range tests {
		if _, ok := knownTests[name]; ok {
			continue
		}
		opts = append(opts, gj.WithTest(name, gjenv.Test{Func: passthroughTest}))
	}

	env, err := gj.New(opts...)
	if err != nil {
		return goResult{ID: id, Err: err.Error()}
	}
	t, err := env.FromString(src)
	if err != nil {
		return goResult{ID: id, Err: err.Error()}
	}
	rendered, err := t.RenderContext(context.Background(), map[string]any{})
	if err != nil {
		return goResult{ID: id, Err: err.Error()}
	}
	return goResult{ID: id, OK: true, Out: rendered}
}

func stringSet(xs []string) map[string]struct{} {
	out := make(map[string]struct{}, len(xs))
	for _, x := range xs {
		out[x] = struct{}{}
	}
	return out
}

// isRender reports whether mode triggers render-style behaviour
// (loader root, output diff). Both "render" and "render-loose" qualify.
func isRender(mode string) bool {
	return mode == "render" || mode == "render-loose"
}

// keep io reachable so unused-import linter stays quiet on partial builds.
var _ = io.Discard
