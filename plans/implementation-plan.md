# Metis Implementation Plan

> Source spec: `OVERVIEW.md` (1825 lines)
> Module: `github.com/techspeque/metis`
> Branch: `dev`
> Old harness context: `old/` (Python dispatcher, Makefile, HARNESS.md)

---

## Summary

Metis is a Go CLI that replaces the ad-hoc Python/Makefile agent harness with a
single binary providing deterministic dispatch, verification, git enforcement,
instructions generation, and progress tracking for AI coding agents.

The implementation is divided into 8 phases (0-7), ordered by dependency:
foundation first, then core domain, then the systems that consume the domain.

---

## Phase 0 — Bootstrap (Project Skeleton)

**Goal:** A buildable Go project with the correct package structure.
`go build ./...` and `go vet ./...` must pass.

| Step | Task | Output |
|------|------|--------|
| 0.1 | `go mod init github.com/techspeque/metis` | `go.mod` |
| 0.2 | Add dependencies: cobra, yaml.v3, fatih/color | `go.mod`, `go.sum` |
| 0.3 | Create `cmd/metis/main.go` entry point | calls `cli.Execute()` |
| 0.4 | Create `internal/cli/root.go` | cobra root cmd + `--version` |
| 0.5 | Create `internal/slice/types.go` | `Slice` struct, enums (stubs) |
| 0.6 | Create `internal/config/config.go` | Config struct types |
| 0.7 | Create `internal/ledger/ledger.go` | Ledger struct types |
| 0.8 | Create placeholder packages | brief, instructions, runner, git, surface, findings, progress, seed, runs |
| 0.9 | Create `testdata/metis.yaml` | Sample config (from spec Section 3) |
| 0.10 | Create `testdata/slices.yaml` | Sample ledger (from Appendix E) |
| 0.11 | Verify build passes | `go build ./...` + `go vet ./...` |
| 0.12 | Commit bootstrap to `dev` | First implementation commit |

**Success criteria:** `go run ./cmd/metis --version` prints version string.

---

## Phase 1 — Core Domain + Config

**Goal:** Full data types with validation, config loading with defaults.

| Step | Task | Package |
|------|------|---------|
| 1.1 | Full slice domain types: enums with `String()`, validation, YAML tags | `internal/slice/` |
| 1.2 | Slice ID generation (`type-NNNN`) + parsing | `internal/slice/` |
| 1.3 | Config `Load()` — parse `metis.yaml` with defaults (Appendix C) | `internal/config/` |
| 1.4 | Config validation — required fields, agent slugs exist in routing, commands non-empty, no self-review | `internal/config/` |
| 1.5 | Unit tests for config + slice packages | `*_test.go` |

**Success criteria:** `go test ./internal/config/... ./internal/slice/...` green.

---

## Phase 2 — Ledger + Dispatch

**Goal:** The dispatch engine works end-to-end. `metis next` returns the correct
slice with the correct role.

| Step | Task | Package |
|------|------|---------|
| 2.1 | Ledger Load/Save (YAML CRUD for `.metis/slices.yaml`) | `internal/ledger/` |
| 2.2 | Dispatch algorithm: priority sort (p0>p1>p2>p3), `blocked_by` resolution, role detection (coded=false -> Coder, coded=true+reviewed=false -> Reviewer) | `internal/ledger/` |
| 2.3 | Lifecycle ops: flip coded/reviewed, block (increment review_cycles, reset coded), skip (set both true + notes), reopen (reset both + notes) | `internal/ledger/` |
| 2.4 | Ledger validation (`check --ledger`): unique IDs, valid slugs, valid risk/priority/type, no reviewed&&!coded, coder!=reviewer, plan_section when plan set, no circular blocked_by | `internal/ledger/` |
| 2.5 | CLI commands: `next` (with `--json`, `--quiet`), `add`, `edit`, `list`, `show`, `skip`, `reopen`, `archive` | `internal/cli/` |
| 2.6 | Unit tests for dispatch + lifecycle | `*_test.go` |

**Success criteria:** `metis next` on sample ledger returns correct slice; `metis add` creates valid entries; `metis check --ledger` validates correctly.

---

## Phase 3 — Verification + Runner

**Goal:** `metis verify` runs the configured command pipeline with proper exit
codes and output capture.

| Step | Task | Package |
|------|------|---------|
| 3.1 | Command runner: execute shell command, capture combined stdout+stderr, timestamp headers | `internal/runner/` |
| 3.2 | Run log storage: write to `.metis/runs/<slice-id>/`, read back | `internal/runs/` |
| 3.3 | Env-check logic: run configured `commands.env_check`, exit 2 on failure with VERDICT banner | `internal/runner/` |
| 3.4 | Verify pipeline: env-check -> ledger check -> verify command, exit codes 0/1/2/3 | `internal/runner/` |
| 3.5 | CLI commands: `env-check`, `verify` (with `--pre`/`--post`), `check` (with `--config`/`--ledger`) | `internal/cli/` |

**Success criteria:** `metis verify --post` captures output to correct log file, returns appropriate exit code.

---

## Phase 4 — Git Enforcement

**Goal:** `metis commit` enforces branch, format, attribution rules.

| Step | Task | Package |
|------|------|---------|
| 4.1 | Branch validation: current branch must match `project.integration_branch` | `internal/git/` |
| 4.2 | Commit message formatting: `{prefix}({slice_id}): {message}`, prefix inference from slice type (feat->feat, fix->fix, refactor->refactor, debt->refactor, remove->refactor, chore->chore, security->fix, gate->docs, recon->docs) | `internal/git/` |
| 4.3 | Attribution stripping: remove Co-Authored-By, Generated with, model name lines | `internal/git/` |
| 4.4 | `metis commit` with shortcuts: `--brief` (add+commit brief), `--flip coded` (flip+add+commit), `--flip reviewed` (same for review), `--amend` | `internal/cli/` |
| 4.5 | Commit log queries: `metis log <id>` shows slice history, `--validate` checks all commits | `internal/git/` |

**Success criteria:** `metis commit --message "add handler"` produces correctly formatted commit; attribution is stripped.

---

## Phase 5 — Brief + Instructions Engine

**Goal:** `metis instructions --for <id>` generates the full risk-scaled agent
contract dynamically.

| Step | Task | Package |
|------|------|---------|
| 5.1 | Per-type brief templates: feat/fix/security/chore (standard), refactor/debt (migration+contract), remove (cleanup checklist), gate (composition) | `internal/brief/` |
| 5.2 | Brief rendering: fill template from slice metadata, `--write` creates file | `internal/brief/` |
| 5.3 | Instructions assembly: 14 sections in spec order (header, session protocol, branch/commit, DoD, roles, hot-paths, scope, routing, testing, non-goals, accuracy rules, review checklist, feedback loop, tooling map) | `internal/instructions/` |
| 5.4 | Risk scaling: `--for <id>` filters sections per risk table (low/medium/high) | `internal/instructions/` |
| 5.5 | Contextual augmentation: append prior briefs (overlapping owned_paths), plan section, active findings | `internal/instructions/` |
| 5.6 | Kickoff generation: step-by-step session protocol with configured branch names, commands | `internal/instructions/` |
| 5.7 | CLI commands: `brief` (`--write`), `instructions` (`--for`, `--json`), `kickoff` (`--role`) | `internal/cli/` |

**Success criteria:** `metis instructions --for <id>` outputs complete markdown contract; sections filtered correctly by risk.

---

## Phase 6 — Surface Adapters + Findings + Seed + Progress

**Goal:** Complete the remaining subsystems.

| Step | Task | Package |
|------|------|---------|
| 6.1 | Surface adapter generation: write CLAUDE.md, AGENTS.md (from instructions output), opencode.json, .claude/settings.json | `internal/surface/` |
| 6.2 | Surface validation: hash-based staleness check, warn if config changed since last generate | `internal/surface/` |
| 6.3 | Findings store: CRUD for `.metis/findings.yaml`, auto-increment IDs | `internal/findings/` |
| 6.4 | Findings stats: per-agent, per-category, per-severity aggregation; first-pass acceptance rate | `internal/findings/` |
| 6.5 | Rules management: `metis rule add "..."`, `metis rule list`, `metis rule promote <finding-id>` | `internal/findings/` |
| 6.6 | Plan seed parser: parse markdown plan -> extract workstreams -> generate slice entries; `--dry-run`, `--append`, `--phase`, `--interactive` | `internal/seed/` |
| 6.7 | Progress dashboard: per-phase bars, overall %, by-stage, quality stats, active/next; `metis status` one-liner | `internal/progress/` |

**Success criteria:** `metis surface generate` writes correct adapter files; `metis progress` shows dashboard; `metis seed plan.md --dry-run` parses correctly.

---

## Phase 7 — Init + Polish

**Goal:** Production-ready CLI with proper UX.

| Step | Task |
|------|------|
| 7.1 | `metis init` — interactive prompts (project name, language, branches, agents, hot paths, commands); `--from metis.yaml` non-interactive mode; scaffolds `.metis/` directory |
| 7.2 | Exit codes: all 12 per Appendix A (0=success, 1=general, 2=env, 3=ledger, 4=config, 5=git, 6=brief missing, 7=run missing, 8=validation, 9=dispatch, 10=backlog empty, 11=model mismatch) |
| 7.3 | Output modes: `--json` (structured JSON) and `--quiet` (minimal, for scripting) across all commands |
| 7.4 | Polish: error messages with actionable suggestions, help text with examples, `--version` with build info (version, commit, date via ldflags) |
| 7.5 | Integration tests: end-to-end scenarios with temp git repos testing full workflows (init -> add -> next -> brief -> verify -> commit -> flip -> archive) |

**Success criteria:** `metis init` creates a fully working setup; all commands have `--help`; integration tests pass.

---

## Dependencies Between Phases

```
Phase 0 (bootstrap)
    |
    v
Phase 1 (domain + config)
    |
    v
Phase 2 (ledger + dispatch)
   / \
  v   v
Phase 3 (verify)    Phase 4 (git)    Phase 5 (brief + instructions)
  \       |         /
   v      v        v
   Phase 6 (surface, findings, seed, progress)
           |
           v
       Phase 7 (init + polish)
```

Phases 3, 4, and 5 can be implemented in parallel once Phase 2 is complete (they
depend on ledger/config but not on each other).

---

## Key Design Decisions

1. **Cobra for CLI** — standard Go CLI framework, supports subcommands, flags, help generation.
2. **yaml.v3** — handles both config and ledger; preserves key order.
3. **No database** — all state is YAML files in `.metis/`. Simple, git-friendly, auditable.
4. **No network** — Metis is a local tool. Zero telemetry.
5. **Exit codes are contracts** — agents parse exit codes to distinguish env failure from code failure.
6. **Internal packages** — nothing is exported. The CLI is the only public interface.
7. **Technology agnosticism** — Metis runs configured commands as opaque strings; it knows nothing about Go/Rust/Python.

---

## Mapping: Old Harness -> Metis

| Old (manual) | Metis (automated) | Phase |
|---|---|---|
| `tools/dispatch.py next` | `metis next` | 2 |
| `tools/dispatch.py check` | `metis check --ledger` | 2 |
| `tools/dispatch.py archive` | `metis archive` | 2 |
| `tools/progress.py` | `metis progress` / `metis status` | 6 |
| `Makefile env-check` | `metis env-check` | 3 |
| `Makefile verify` | `metis verify` | 3 |
| `Makefile interfaces` | `metis interfaces` | 3 |
| Manual AGENTS.md editing | `metis instructions` / `metis surface generate` | 5, 6 |
| Manual commit formatting | `metis commit` | 4 |
| Manual findings table | `metis block` / `metis findings` | 6 |
| Manual rule editing | `metis rule add/promote` | 6 |
| Manual ledger YAML editing | `metis add/edit/seed` | 2, 6 |
| Copy-paste AGENT-KICKOFF.md | `metis kickoff` | 5 |

---

## Reference

- Full specification: `OVERVIEW.md`
- Old harness (context): `old/harness/HARNESS.md`
- Old dispatcher: `old/harness/tools/dispatch.py`
- Old Makefile: `old/Makefile`
