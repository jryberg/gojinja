// Command gojinja renders a Jinja2 template from disk or stdin against a
// JSON variables file. Usage:
//
//	gojinja render --template path.j2 [--vars vars.json] [--out -]
//	              [--root DIR ...] [--unsafe] [--no-autoescape]
//	              [--host-env | --env-mapping] [--filter NAME ...]
//
// By default rendering is sandboxed, autoescape is on, host environment
// variables are not exposed, and no non-Jinja2 filters are registered.
// --unsafe, --no-autoescape, --host-env, --env-mapping and --filter exist
// for explicit opt-out (templates from trusted sources only).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jryberg/gojinja/pkg/environment"
	"github.com/jryberg/gojinja/pkg/filters"
	"github.com/jryberg/gojinja/pkg/loader"
	"github.com/jryberg/gojinja/pkg/varsutil"
)

// Build-time metadata. Overridden via -X ldflags by GoReleaser.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

const usage = `gojinja — render a Jinja2-compatible template

Usage:
  gojinja render --template FILE [flags]
  gojinja version

Flags:
  --template FILE     template file to render, '-' for stdin (required)
  --vars FILE         JSON file mapping variable names to values
  --out FILE          output destination ('-' for stdout, default '-')
  --root DIR          allowlisted root for {%% include %%} (repeatable)
  --unsafe            disable the sandbox (only for trusted templates)
  --no-autoescape     disable HTML autoescape
  --host-env          register the env() global, backed by os.Getenv
  --env-mapping       register the env mapping of the host environment,
                      like Python's os.environ
  --filter NAME       enable an opt-in non-Jinja2 filter (repeatable): %s
  --max-range N       maximum range() size (default 100000)
`

func main() {
	if len(os.Args) < 2 {
		printUsage(os.Stderr)
		os.Exit(2)
	}
	switch os.Args[1] {
	case "render":
		if err := cmdRender(os.Args[2:], os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "version":
		fmt.Printf("gojinja %s (commit %s, built %s)\n", version, commit, date)
	case "-h", "--help", "help":
		printUsage(os.Stdout)
	default:
		fmt.Fprintln(os.Stderr, "unknown command:", os.Args[1])
		printUsage(os.Stderr)
		os.Exit(2)
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintf(w, usage, strings.Join(filters.Names(), ", "))
}

func cmdRender(args []string, stdin io.Reader, stdout io.Writer) error {
	fs := flag.NewFlagSet("render", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var (
		tplPath      string
		varsPath     string
		outPath      = "-"
		roots        listFlag
		filterNames  listFlag
		unsandbox    bool
		noAutoescape bool
		hostEnv      bool
		envMapping   bool
		maxRange     int
	)
	fs.StringVar(&tplPath, "template", "", "")
	fs.StringVar(&varsPath, "vars", "", "")
	fs.StringVar(&outPath, "out", "-", "")
	fs.Var(&roots, "root", "")
	fs.Var(&filterNames, "filter", "")
	fs.BoolVar(&unsandbox, "unsafe", false, "")
	fs.BoolVar(&noAutoescape, "no-autoescape", false, "")
	fs.BoolVar(&hostEnv, "host-env", false, "")
	fs.BoolVar(&envMapping, "env-mapping", false, "")
	fs.IntVar(&maxRange, "max-range", 100_000, "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if tplPath == "" {
		return errors.New("--template is required")
	}
	if hostEnv && envMapping {
		return errors.New("--host-env and --env-mapping both define env; pick one")
	}

	var vars any
	if varsPath != "" {
		raw, err := os.ReadFile(varsPath)
		if err != nil {
			return err
		}
		vars, err = varsutil.JSONVars(raw)
		if err != nil {
			return fmt.Errorf("parsing %s: %w", varsPath, err)
		}
	}

	opts := []environment.Option{
		environment.WithRangeLimit(maxRange),
	}
	// A file template gets its own directory as the loader root; a stdin
	// template gets a loader only when --root is given.
	if len(roots) == 0 && tplPath != "-" {
		roots = append(roots, filepath.Dir(tplPath))
	}
	if len(roots) > 0 {
		fsLoader, err := loader.NewFileSystem(roots)
		if err != nil {
			return err
		}
		opts = append(opts, environment.WithLoader(fsLoader))
	}
	if unsandbox {
		opts = append(opts, environment.WithUnsafe())
	}
	if noAutoescape {
		opts = append(opts, environment.WithAutoescape(environment.AutoescapeNever{}))
	}
	if hostEnv {
		opts = append(opts, environment.WithHostEnv())
	}
	if envMapping {
		opts = append(opts, environment.WithHostEnvMap())
	}
	for _, name := range filterNames {
		f, ok := filters.Lookup(name)
		if !ok {
			return fmt.Errorf("unknown filter %q (available: %s)", name, strings.Join(filters.Names(), ", "))
		}
		opts = append(opts, environment.WithFilter(name, f))
	}

	env, err := environment.New(opts...)
	if err != nil {
		return err
	}

	var src []byte
	if tplPath == "-" {
		src, err = io.ReadAll(stdin)
	} else {
		src, err = os.ReadFile(tplPath)
	}
	if err != nil {
		return err
	}
	tpl, err := env.FromString(string(src))
	if err != nil {
		return err
	}
	out, err := tpl.RenderContext(context.Background(), vars)
	if err != nil {
		return err
	}

	if outPath == "-" {
		_, err = io.WriteString(stdout, out)
		return err
	}
	return os.WriteFile(outPath, []byte(out), 0o644)
}

// listFlag implements flag.Value for repeatable string flags.
type listFlag []string

func (l *listFlag) String() string     { return "" }
func (l *listFlag) Set(v string) error { *l = append(*l, v); return nil }
func (l listFlag) Get() any            { return []string(l) }
