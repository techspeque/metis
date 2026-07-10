# CAESAR — Makefile
# Harness and verification targets for agent-driven development.

SHELL := /bin/bash
.DEFAULT_GOAL := help

DISPATCHER := python3 harness/tools/dispatch.py
CAESAR_GOCACHE ?= /tmp/caesar-go-build-cache
CAESAR_GOMODCACHE ?= /tmp/caesar-go-mod-cache
export GOCACHE := $(CAESAR_GOCACHE)
export GOMODCACHE := $(CAESAR_GOMODCACHE)

# ─── Harness targets ─────────────────────────────────────────────────────────

.PHONY: next-slice
next-slice: ## Print active slice, role, required model, reading list
	@$(DISPATCHER) next

.PHONY: slices-check
slices-check: ## Lint the ledger (CI)
	@$(DISPATCHER) check

.PHONY: slices-archive
slices-archive: ## Move done slices to the archive
	@$(DISPATCHER) archive

.PHONY: progress
progress: ## Show slice completion dashboard with progress bars
	@python3 harness/tools/progress.py

# ─── Verification targets ────────────────────────────────────────────────────

.PHONY: env-check
env-check: ## Verify the development environment is sound
	@echo "=== CAESAR env-check ==="
	@echo -n "Go toolchain: " && go version || { echo "VERDICT: ENVIRONMENT FAILURE — Go not found on PATH."; echo "Do NOT modify code, tests, or config to make verify pass."; echo "Stop and report this verbatim."; exit 1; }
	@echo -n "Python3: " && python3 --version || { echo "VERDICT: ENVIRONMENT FAILURE — python3 not found."; exit 1; }
	@python3 -c "import yaml" 2>/dev/null || { echo "VERDICT: ENVIRONMENT FAILURE — pyyaml not installed (pip3 install pyyaml)."; echo "Do NOT modify code to work around this."; exit 1; }
	@if [ -f go.mod ]; then \
		GOCACHE=$$(go env GOCACHE 2>/dev/null) || { echo "VERDICT: ENVIRONMENT FAILURE — unable to inspect Go build cache."; echo "Do NOT modify code, tests, or config to make verify pass."; exit 1; }; \
		mkdir -p "$$GOCACHE" 2>/dev/null || { echo "VERDICT: ENVIRONMENT FAILURE — Go build cache is not writable."; echo "Do NOT modify code, tests, or config to make verify pass."; exit 1; }; \
		test -w "$$GOCACHE" || { echo "VERDICT: ENVIRONMENT FAILURE — Go build cache is not writable."; echo "Do NOT modify code, tests, or config to make verify pass."; exit 1; }; \
		GOMODCACHE=$$(go env GOMODCACHE 2>/dev/null) || { echo "VERDICT: ENVIRONMENT FAILURE — unable to inspect Go module cache."; echo "Do NOT modify code, tests, or config to make verify pass."; exit 1; }; \
		mkdir -p "$$GOMODCACHE" 2>/dev/null || { echo "VERDICT: ENVIRONMENT FAILURE — Go module cache is not writable."; echo "Do NOT modify code, tests, or config to make verify pass."; exit 1; }; \
		test -w "$$GOMODCACHE" || { echo "VERDICT: ENVIRONMENT FAILURE — Go module cache is not writable."; echo "Do NOT modify code, tests, or config to make verify pass."; exit 1; }; \
	fi
	@echo "env-check: OK"

.PHONY: verify
verify: env-check slices-check ## The full verification gate: build + lint + test
	@echo "=== CAESAR verify ==="
	@if [ -f go.mod ]; then \
		echo "--- go build ---"; \
		go build ./... || exit 1; \
		echo "--- go vet ---"; \
		go vet ./... || exit 1; \
		echo "--- go test ---"; \
		go test ./... || exit 1; \
		echo "--- caesar help ---"; \
		go run ./cmd/caesar --help >/dev/null || exit 1; \
	else \
		echo "(no go.mod yet — skeleton only, build checks skipped)"; \
	fi
	@echo "verify: ALL GREEN"

.PHONY: interfaces
interfaces: ## Regenerate docs/generated/interfaces.txt
	@echo "=== Regenerating interface summary ==="
	@if [ -f go.mod ]; then \
		set -eu; \
		tmp=$$(mktemp docs/generated/interfaces.txt.XXXXXX); \
		trap 'rm -f "$$tmp"' EXIT; \
		echo "# CAESAR — Generated Interface Summary" > "$$tmp"; \
		echo "# Generated: $$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$$tmp"; \
		echo "" >> "$$tmp"; \
		for pkg in $$(go list ./internal/...); do \
			echo "## $$pkg" >> "$$tmp"; \
			go doc "$$pkg" >> "$$tmp"; \
			echo "" >> "$$tmp"; \
		done; \
		grep -q '^package ' "$$tmp"; \
		grep -q '^type ' "$$tmp"; \
		mv "$$tmp" docs/generated/interfaces.txt; \
		echo "interfaces.txt regenerated."; \
	else \
		echo "(no go.mod yet — nothing to generate)"; \
	fi

# ─── Backup ───────────────────────────────────────────────────────────────────

BACKUP_TIMESTAMP := $(shell date -u +%Y%m%d-%H%M%S)
BACKUP_FILE := harness/backups/harness-backup-$(BACKUP_TIMESTAMP).tar.gz

# Files that are always safe to back up (harness machinery)
HARNESS_CORE := \
	harness/AGENT-KICKOFF.md \
	harness/HARNESS.md \
	harness/PLAN.md \
	harness/REFACTOR.md \
	harness/BOOTSTRAP.MD \
	harness/README.md \
	harness/slice-ledger.md \
	harness/review-findings.md \
	harness/tools/dispatch.py \
	harness/tools/progress.py \
	AGENTS.md \
	CLAUDE.md \
	opencode.json \
	.claude/settings.json \
	Makefile \
	docs/adr

# Slice state (ledger + briefs) — backed up but may drift from repo state
HARNESS_SLICES := \
	plans/slices.yaml \
	plans/slices-done.yaml \
	plans/briefs

.PHONY: harness-backup
harness-backup: ## Back up all harness files to a timestamped tarball
	@mkdir -p harness/backups
	@echo "=== Harness backup ==="
	@echo "Backing up harness core + slice state..."
	@echo ""
	@echo "WARNING: Slice state (plans/slices.yaml, plans/briefs/) is included."
	@echo "If you restore this backup into a repo whose code has moved forward,"
	@echo "the ledger's coded/reviewed state may not match the actual commits."
	@echo "Reconciliation via 'make slices-check' and manual inspection will be"
	@echo "required after any restore."
	@echo ""
	@tar -czf $(BACKUP_FILE) \
		$(HARNESS_CORE) \
		$(shell for f in $(HARNESS_SLICES); do [ -e "$$f" ] && echo "$$f"; done) \
		2>/dev/null || true
	@echo "Backup created: $(BACKUP_FILE)"
	@echo "Size: $$(du -h $(BACKUP_FILE) | cut -f1)"

.PHONY: harness-backup-core
harness-backup-core: ## Back up harness machinery only (no slice state)
	@mkdir -p harness/backups
	@echo "=== Harness core backup (no slice state) ==="
	@tar -czf $(BACKUP_FILE) $(HARNESS_CORE) 2>/dev/null || true
	@echo "Backup created: $(BACKUP_FILE)"
	@echo "Size: $$(du -h $(BACKUP_FILE) | cut -f1)"

# ─── Help ────────────────────────────────────────────────────────────────────

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-18s\033[0m %s\n", $$1, $$2}'
