.PHONY: test vet lint audit tidy build parity parity-external parity-external-fetch ci gofmt-check precommit hooks-install hooks-uninstall docs docs-serve docs-check

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

# Parity harness: renders every corpus template through both gojinja and
# Python Jinja2 (via tools/parity/parity.py) and diffs the outputs.
# Requires `python3` with `jinja2` installed on $PATH.
parity:
	$(GO) run ./tools/parity

# External parity: validates gojinja against real-world templates fetched
# from ten upstream open-source projects (see
# tools/parity/external/SOURCES.md). Run -fetch once to populate the
# cache (network access required, GH_TOKEN respected for rate limits).
parity-external-fetch:
	$(GO) run ./tools/parity/external -fetch -validate=false

parity-external:
	$(GO) run ./tools/parity/external
	$(GO) run ./tools/parity/external -render

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
