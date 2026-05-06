.PHONY: test vet lint audit tidy build parity parity-external parity-external-fetch ci

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
audit:
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
