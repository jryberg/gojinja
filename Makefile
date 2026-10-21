.PHONY: test vet lint audit tidy build parity parity-venv signatures signatures-check pyunicode pyunicode-check codectables codectables-check unicodenames unicodenames-check cjktables cjktables-check idnatables idnatables-check htmlentities htmlentities-check pyattrs pyattrs-check lipsum lipsum-check parity-external parity-external-fetch ci gofmt-check precommit hooks-install hooks-uninstall docs docs-serve docs-check

GO ?= go

build:
	$(GO) build ./...

test:
	$(GO) test -race ./...

vet:
	$(GO) vet ./...

lint:
	@if command -v staticcheck >/dev/null 2>&1; then \
		staticcheck ./...; \
	else \
		echo "staticcheck not installed; skipping"; \
	fi

# Smoke audit: greps for things that should never appear.
# Real audit lives in docs/security-audit-checklist.md and is run per-phase.
audit: docs-check
	@echo "==> no unsafe package use outside internal/"
	@! grep -rEn '^[[:space:]]*"unsafe"[[:space:]]*$$' --include='*.go' pkg cmd
	@echo "==> no os/exec use"
	@! grep -rEn '^[[:space:]]*"os/exec"[[:space:]]*$$' --include='*.go' pkg cmd
	@echo "==> no CGo"
	@! grep -rEn '^[[:space:]]*"C"[[:space:]]*$$|//#cgo' --include='*.go' pkg cmd
	@echo "==> third-party module imports flagged for review"
	@! grep -rhEn '^\s*"[^"]+/[^"]+"' --include='*.go' pkg cmd | grep -E '^\s*"[a-z0-9.-]+\.[a-z]+/' | grep -v 'github.com/jryberg/gojinja/' || true
	@echo "audit smoke checks passed"

tidy:
	$(GO) mod tidy

# Parity runs Python Jinja2 from a project venv built from the pinned,
# hash-locked tools/parity/requirements.txt — never the system copy, so
# every machine and CI compare against the same Jinja2 release. The venv
# is rebuilt whenever the requirements or the pinned Python change.
#
# The CPython release is pinned too, in tools/parity/.python-version:
# the generated tables follow it, and a patch release can change them.
# PARITY_BASE_PYTHON is the interpreter the venv is built from; it
# defaults to uv's copy of the pinned release, else python3.
PARITY_VENV ?= .venv-parity
PARITY_PYTHON := $(PARITY_VENV)/bin/python
PARITY_PYTHON_VERSION := $(shell cat tools/parity/.python-version)
PARITY_BASE_PYTHON ?= $(shell uv python find $(PARITY_PYTHON_VERSION) 2>/dev/null || echo python3)

$(PARITY_VENV)/.installed: tools/parity/requirements.txt tools/parity/.python-version
	@v=$$($(PARITY_BASE_PYTHON) -c 'import platform; print(platform.python_version())') \
		&& [ "$$v" = "$(PARITY_PYTHON_VERSION)" ] \
		|| { echo "parity needs CPython $(PARITY_PYTHON_VERSION), but $(PARITY_BASE_PYTHON) is $$v; set PARITY_BASE_PYTHON (see DEVELOPMENT.md)"; exit 1; }
	rm -rf $(PARITY_VENV)
	$(PARITY_BASE_PYTHON) -m venv $(PARITY_VENV)
	$(PARITY_PYTHON) -m pip install --quiet --require-hashes -r tools/parity/requirements.txt
	@touch $@

parity-venv: $(PARITY_VENV)/.installed
	@$(PARITY_PYTHON) -c 'import platform, sys; from importlib.metadata import version as v; p = platform.python_version(); print("parity venv: python", p, "jinja2", v("jinja2"), "markupsafe", v("markupsafe")); sys.exit(p != sys.argv[1] and f"parity venv runs Python {p}, not the pinned {sys.argv[1]}; delete $(PARITY_VENV) and rerun")' $(PARITY_PYTHON_VERSION)

# Builtin filter/test signatures are generated from the pinned Jinja2;
# signatures-check fails when they no longer match it.
SIGNATURES_GO := pkg/environment/signatures_table.go

signatures: parity-venv
	$(PARITY_PYTHON) -I -B tools/parity/signatures.py > $(SIGNATURES_GO)
	gofmt -w $(SIGNATURES_GO)

signatures-check: parity-venv
	@$(PARITY_PYTHON) -I -B tools/parity/signatures.py | gofmt | diff -u $(SIGNATURES_GO) - \
		|| { echo "$(SIGNATURES_GO) is stale for the pinned Jinja2; run 'make signatures'"; exit 1; }

# All the Unicode data template output depends on (str predicates and
# case mappings, regex classes, identifiers), generated from the venv's
# Python so it never follows the Go toolchain's Unicode version.
PYUNICODE_GO := internal/pyunicode/pyunicode_table.go

pyunicode: parity-venv
	$(PARITY_PYTHON) -I -B tools/parity/pyunicode.py > $(PYUNICODE_GO)
	gofmt -w $(PYUNICODE_GO)

pyunicode-check: parity-venv
	@$(PARITY_PYTHON) -I -B tools/parity/pyunicode.py | gofmt | diff -q $(PYUNICODE_GO) - >/dev/null \
		|| { echo "$(PYUNICODE_GO) is stale for the pinned Python; run 'make pyunicode'"; exit 1; }

# Python's codec tables and aliases, generated from the venv's Python.
CODECTABLES_GO := pkg/environment/codectables_table.go

codectables: parity-venv
	$(PARITY_PYTHON) -I -B tools/parity/codectables.py > $(CODECTABLES_GO)
	gofmt -w $(CODECTABLES_GO)

codectables-check: parity-venv
	@$(PARITY_PYTHON) -I -B tools/parity/codectables.py | gofmt | diff -q $(CODECTABLES_GO) - >/dev/null \
		|| { echo "$(CODECTABLES_GO) is stale for the pinned Python; run 'make codectables'"; exit 1; }

# Unicode character names for \N{...}, generated from the venv's Python.
UNICODENAMES_GO := internal/unicodename/names_table.go

unicodenames: parity-venv
	$(PARITY_PYTHON) -I -B tools/parity/unicodenames.py > $(UNICODENAMES_GO)
	gofmt -w $(UNICODENAMES_GO)

unicodenames-check: parity-venv
	@$(PARITY_PYTHON) -I -B tools/parity/unicodenames.py | gofmt | diff -q $(UNICODENAMES_GO) - >/dev/null \
		|| { echo "$(UNICODENAMES_GO) is stale for the pinned Python; run 'make unicodenames'"; exit 1; }

# Python's idna codec data: stringprep tables and the Unicode data its
# NFKC uses.
IDNATABLES_GO := pkg/environment/idnatables_table.go

idnatables: parity-venv
	$(PARITY_PYTHON) -I -B tools/parity/idnatables.py | gofmt > $(IDNATABLES_GO).tmp && mv $(IDNATABLES_GO).tmp $(IDNATABLES_GO)

idnatables-check: parity-venv
	@$(PARITY_PYTHON) -I -B tools/parity/idnatables.py | gofmt | diff -q $(IDNATABLES_GO) - >/dev/null \
		|| { echo "$(IDNATABLES_GO) is stale for the pinned Python; run 'make idnatables'"; exit 1; }

# The HTML5 character references behind Python's html.unescape.
HTMLENTITIES_GO := pkg/escape/html5_table.go

htmlentities: parity-venv
	$(PARITY_PYTHON) -I -B tools/parity/htmlentities.py | gofmt > $(HTMLENTITIES_GO).tmp && mv $(HTMLENTITIES_GO).tmp $(HTMLENTITIES_GO)

htmlentities-check: parity-venv
	@$(PARITY_PYTHON) -I -B tools/parity/htmlentities.py | gofmt | diff -q $(HTMLENTITIES_GO) - >/dev/null \
		|| { echo "$(HTMLENTITIES_GO) is stale for the pinned Python; run 'make htmlentities'"; exit 1; }

# The underscore attributes of Python's objects, for the sandbox.
PYATTRS_GO := pkg/environment/pyattrs_table.go

pyattrs: parity-venv
	$(PARITY_PYTHON) -I -B tools/parity/pyattrs.py | gofmt > $(PYATTRS_GO).tmp && mv $(PYATTRS_GO).tmp $(PYATTRS_GO)

pyattrs-check: parity-venv
	@$(PARITY_PYTHON) -I -B tools/parity/pyattrs.py | gofmt | diff -q $(PYATTRS_GO) - >/dev/null \
		|| { echo "$(PYATTRS_GO) is stale for the pinned Python; run 'make pyattrs'"; exit 1; }

# The words Jinja2's lipsum global draws from.
LIPSUM_GO := pkg/environment/lipsum_table.go

lipsum: parity-venv
	$(PARITY_PYTHON) -I -B tools/parity/lipsum.py | gofmt > $(LIPSUM_GO).tmp && mv $(LIPSUM_GO).tmp $(LIPSUM_GO)

lipsum-check: parity-venv
	@$(PARITY_PYTHON) -I -B tools/parity/lipsum.py | gofmt | diff -q $(LIPSUM_GO) - >/dev/null \
		|| { echo "$(LIPSUM_GO) is stale for the pinned Python; run 'make lipsum'"; exit 1; }

# Stateless CJK codec tables (gzip binary), generated by probing the venv's
# Python codecs. The check regenerates them in full and compares the
# uncompressed bytes, so an edited or stale blob fails `make parity` in CI.
CJKTABLES := pkg/environment/cjk_tables.bin.gz

cjktables: parity-venv
	$(PARITY_PYTHON) -I -B tools/parity/cjktables.py > $(CJKTABLES).tmp && mv $(CJKTABLES).tmp $(CJKTABLES)

cjktables-check: parity-venv
	@$(PARITY_PYTHON) -I -B tools/parity/cjktables.py --check $(CJKTABLES)

# Parity harness: renders every corpus template through both gojinja and
# Python Jinja2 (via tools/parity/parity.py) and diffs the outputs.
parity: parity-venv signatures-check pyunicode-check codectables-check unicodenames-check cjktables-check idnatables-check htmlentities-check pyattrs-check lipsum-check
	$(GO) run ./tools/parity -python $(PARITY_PYTHON)

# External parity: validates gojinja against real-world templates fetched
# from ten upstream open-source projects (see
# tools/parity/external/SOURCES.md). Run -fetch once to populate the
# cache (network access required, GH_TOKEN respected for rate limits).
parity-external-fetch: parity-venv
	$(GO) run ./tools/parity/external -python $(PARITY_PYTHON) -fetch -validate=false

parity-external: parity-venv
	$(GO) run ./tools/parity/external -python $(PARITY_PYTHON)
	$(GO) run ./tools/parity/external -python $(PARITY_PYTHON) -render

# Aggregate target mirroring what CI runs (without the OS matrix or
# external parity). Run this before opening a PR.
ci: build vet test audit parity
	@echo "ci: all checks passed"

# Documentation site (mkdocs-material + mike). See docs/site/README.md.
# `make docs` regenerates filter/test/global pages from the registry, then
# builds the static site under docs/site/site/.
docs:
	@if [ -d tools/docgen ]; then \
	  $(GO) run ./tools/docgen build   --out docs/site/content/templates; \
	  $(GO) run ./tools/docgen symbols --out docs/site/content/api/symbols.md; \
	fi
	cd docs/site && mkdocs build --strict

docs-serve:
	@if [ -d tools/docgen ]; then \
	  $(GO) run ./tools/docgen build   --out docs/site/content/templates; \
	  $(GO) run ./tools/docgen symbols --out docs/site/content/api/symbols.md; \
	fi
	cd docs/site && mkdocs serve

# CI gate: every registered filter/test/global must have either a hand-written
# override or a structured godoc on its underlying func. Skips silently until
# tools/docgen lands so this target is safe to wire into `audit` from day one.
docs-check:
	@if [ -d tools/docgen ]; then \
	  $(GO) run ./tools/docgen check; \
	else \
	  echo "tools/docgen not present yet; skipping docs-check"; \
	fi

# Verify all tracked .go files are gofmt-clean. Whole-tree check; the
# pre-commit hook only checks staged files.
gofmt-check:
	@bad=$$(gofmt -l $$(git ls-files '*.go') 2>/dev/null); \
	 if [ -n "$$bad" ]; then \
	   echo "gofmt: unformatted files:"; echo "$$bad"; \
	   echo "fix with: gofmt -w <files>"; \
	   exit 1; \
	 fi

# Fast lint subset matching the .githooks/pre-commit hook's non-gofmt steps.
# (gofmt is staged-only in the hook; run `make gofmt-check` for a whole-tree pass.)
precommit: vet audit lint
	@echo "precommit: passed"

# Wire up the in-repo .githooks/ directory. Run once per clone.
hooks-install:
	@git config core.hooksPath .githooks
	@chmod +x .githooks/*
	@echo "hooks: installed (core.hooksPath=.githooks)"

hooks-uninstall:
	@git config --unset core.hooksPath || true
	@echo "hooks: uninstalled (core.hooksPath cleared)"
