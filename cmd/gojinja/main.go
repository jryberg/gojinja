// Command gojinja renders a Jinja2 template from disk against a JSON
// variables file. Usage:
//
//	gojinja render --template path.j2 [--vars vars.json] [--out -]
//	              [--root DIR ...] [--unsafe] [--no-autoescape]
//	              [--host-env]
//
// By default rendering is sandboxed, autoescape is on, and host
// environment variables are not exposed. --unsafe, --no-autoescape, and
// --host-env exist for explicit opt-out (templates from trusted sources
// only).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/jryberg/gojinja/pkg/environment"
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
  --template FILE     template file to render (required)
  --vars FILE         JSON file mapping variable names to values
  --out FILE          output destination ('-' for stdout, default '-')
  --root DIR          allowlisted root for {%% include %%} (repeatable)
  --unsafe            disable the sandbox (only for trusted templates)
  --no-autoescape     disable HTML autoescape
  --host-env          register the env() global, backed by os.Getenv
  --max-range N       maximum range() size (default 100000)
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	switch os.Args[1] {
	case "render":
		if err := cmdRender(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "version":
		fmt.Printf("gojinja %s (commit %s, built %s)\n", version, commit, date)
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		fmt.Fprintln(os.Stderr, "unknown command:", os.Args[1])
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
}

func cmdRender(args []string) error {
	fs := flag.NewFlagSet("render", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var (
		tplPath      string
		varsPath     string
		outPath      = "-"
		roots        rootList
		unsandbox    bool
		noAutoescape bool
		hostEnv      bool
		maxRange     int
	)
	fs.StringVar(&tplPath, "template", "", "")
	fs.StringVar(&varsPath, "vars", "", "")
	fs.StringVar(&outPath, "out", "-", "")
	fs.Var(&roots, "root", "")
	fs.BoolVar(&unsandbox, "unsafe", false, "")
	fs.BoolVar(&noAutoescape, "no-autoescape", false, "")
	fs.BoolVar(&hostEnv, "host-env", false, "")
	fs.IntVar(&maxRange, "max-range", 100_000, "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if tplPath == "" {
		return errors.New("--template is required")
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
	if len(roots) == 0 {
		// Auto-include the template's directory as the loader root.
		roots = append(roots, filepath.Dir(tplPath))
	}
	fsLoader, err := loader.NewFileSystem(roots)
	if err != nil {
		return err
	}
	opts = append(opts, environment.WithLoader(fsLoader))
	if unsandbox {
		opts = append(opts, environment.WithUnsafe())
	}
	if noAutoescape {
		opts = append(opts, environment.WithAutoescape(environment.AutoescapeNever{}))
	}
	if hostEnv {
		opts = append(opts, environment.WithHostEnv())
	}

	env, err := environment.New(opts...)
	if err != nil {
		return err
	}

	src, err := os.ReadFile(tplPath)
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
		_, err = io.WriteString(os.Stdout, out)
		return err
	}
	return os.WriteFile(outPath, []byte(out), 0o644)
}

// rootList implements flag.Value for repeatable --root flags.
type rootList []string

func (r *rootList) String() string     { return "" }
func (r *rootList) Set(v string) error { *r = append(*r, v); return nil }
func (r rootList) Get() any            { return []string(r) }
