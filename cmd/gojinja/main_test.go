package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func run(t *testing.T, stdin string, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := cmdRender(args, strings.NewReader(stdin), &out)
	return out.String(), err
}

func TestRenderFromStdin(t *testing.T) {
	vars := filepath.Join(t.TempDir(), "vars.json")
	if err := os.WriteFile(vars, []byte(`{"name": "World"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := run(t, "Hello {{ name }}!", "--template", "-", "--vars", vars)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "Hello World!" {
		t.Fatalf("got %q", got)
	}
}

func TestRenderFromFile(t *testing.T) {
	dir := t.TempDir()
	tpl := filepath.Join(dir, "t.j2")
	if err := os.WriteFile(tpl, []byte(`{% include "inc.j2" %}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "inc.j2"), []byte("included"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := run(t, "", "--template", tpl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "included" {
		t.Fatalf("got %q", got)
	}
}

// TestStdinHasNoLoaderWithoutRoot confirms a stdin template cannot
// include files unless --root is given.
func TestStdinHasNoLoaderWithoutRoot(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "inc.j2"), []byte("included"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	if _, err := run(t, `{% include "inc.j2" %}`, "--template", "-"); err == nil {
		t.Fatal("expected include to fail without --root")
	}
	got, err := run(t, `{% include "inc.j2" %}`, "--template", "-", "--root", dir)
	if err != nil {
		t.Fatalf("unexpected error with --root: %v", err)
	}
	if got != "included" {
		t.Fatalf("got %q", got)
	}
}

func TestEnvMapping(t *testing.T) {
	t.Setenv("GOJINJA_CLI_VAR", "hello")
	got, err := run(t, "{{ env['GOJINJA_CLI_VAR'] }} {{ env.get('GOJINJA_CLI_MISSING', 'dflt') }}",
		"--template", "-", "--env-mapping")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "hello dflt" {
		t.Fatalf("got %q", got)
	}
}

func TestHostEnvAndEnvMappingConflict(t *testing.T) {
	if _, err := run(t, "x", "--template", "-", "--host-env", "--env-mapping"); err == nil {
		t.Fatal("expected error when both --host-env and --env-mapping are set")
	}
}

func TestFilterFlag(t *testing.T) {
	got, err := run(t, "{{ 'aGVsbG8='|base64decode }}", "--template", "-", "--filter", "base64decode")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "hello" {
		t.Fatalf("got %q", got)
	}
	if _, err := run(t, "{{ 'aGVsbG8='|base64decode }}", "--template", "-"); err == nil {
		t.Fatal("expected base64decode to be unknown without --filter")
	}
}

func TestUnknownFilterFlag(t *testing.T) {
	_, err := run(t, "x", "--template", "-", "--filter", "nope")
	if err == nil || !strings.Contains(err.Error(), "base64decode") {
		t.Fatalf("got %v, want an unknown-filter error listing base64decode", err)
	}
}

// TestJinjaconfEquivalent renders the flags base images use in place of
// jinjaconf.py: unsandboxed, no autoescape, env mapping, base64decode,
// and a mutating `do` on a list.
func TestJinjaconfEquivalent(t *testing.T) {
	t.Setenv("GOJINJA_CLI_B64", "PGE+")
	src := "{% set urls = [] %}{% do urls.append(env['GOJINJA_CLI_B64']|base64decode) %}{{ urls|join(',') }}"
	got, err := run(t, src, "--template", "-", "--env-mapping", "--unsafe", "--no-autoescape", "--filter", "base64decode")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "<a>" {
		t.Fatalf("got %q", got)
	}
}
