# Metis VS Code Extension Plan

> Feature: `editors/vscode/` — a VS Code extension that turns metis into a
> one-window experience for the human persona.
> Prior art: SpecStudio (github.com/techspeque/specstudio, abandoned).
> CLI groundwork: #7 (`--output json`), #8 (`metis config`), #9 (agent JSON
> instructions) — all shipped in v0.0.4.

---

## Vision

Metis is complete for agents and terminal-native humans. What it lacks is a
cockpit: spec editing, ledger visibility, one-click session launching, and
the review loop made visible. SpecStudio tried to be that cockpit as a
standalone IDE and failed for one root cause: it owned the agent's process
(embedded PTY, proxied auth, bundled CLIs, raw API calls).

The extension inverts that. VS Code is the window the user already has open;
the extension is a **renderer over `.metis/` state and a spawn-and-detach
launcher** into VS Code's own integrated terminal. The user's stack collapses
from "multiple terminals + editor + custom app" to one window.

### Design laws (non-negotiable)

1. **Never own the agent's process.** Agents run in VS Code integrated
   terminals as normal processes: their own TUI, their own auth, their own
   lifecycle. The extension calls `createTerminal()` + `sendText()` and then
   never touches the terminal's stdio.
2. **Never bypass the CLI contract.** All reads go through `metis <cmd> -o
   json` or file-watching `.metis/` for change *detection* (watch to know
   when to re-run the CLI — never parse ledger YAML directly). All writes go
   through metis commands (`config set`, etc.). The extension must work
   against any metis ≥ its declared minimum, and metis must never need to
   know the extension exists.

Law 2 is also the exit strategy: an extension that only speaks the CLI
contract can be extracted to its own repo later with `git filter-repo` in an
afternoon.

### Non-features (SpecStudio lessons, kept deliberately)

- No embedded chat/agent UI; no parsing of agent output
- No auth handling of any kind — agent CLIs own their login flows
- No bundling of agent CLIs — they remain prereqs surfaced by `metis doctor`
- No plan/ledger editing that bypasses the protocol
- No webview terminal, ever

---

## Architecture

```
┌─ VS Code ────────────────────────────────────────────────┐
│  Sidebar (tree views)      Status bar      Webview panels │
│      │                        │                │          │
│      └────────────┬───────────┴────────────────┘          │
│                   ▼                                       │
│            MetisClient (TS service)                       │
│         exec: metis <cmd> -o json                         │
│         refresh: FileSystemWatcher on .metis/** ,         │
│                  metis.yaml                               │
│                   │                                       │
│  Terminal API ◄───┘ (launch sessions, spawn-and-detach)   │
└───────────────────┼──────────────────────────────────────┘
                    ▼
             metis binary (PATH)
                    ▼
             .metis/ + metis.yaml   ◄── agents write here too
```

- **MetisClient**: single choke point wrapping `child_process.execFile` of
  the metis binary with `-o json`, typed result interfaces, error surface
  (exit codes preserved), and a debounced refresh bus fed by file watchers.
- **State flow**: watcher fires → invalidate → re-run relevant CLI reads →
  update views. Agents running in terminals mutate `.metis/` through metis;
  the extension sees changes the same way it sees human changes. One state,
  no sync protocol.
- **Version handshake**: on activation run `metis version -o json`; compare
  against `MIN_METIS_VERSION` compiled into the extension. Too old → setup
  panel with upgrade instructions; missing → install instructions. Never
  hard-crash.

---

## Phase 0 — CLI Prerequisites (Go, ships as normal metis PRs)

Two small gaps the extension would otherwise have to hack around.

### 0.1 `metis doctor`

| Step | Task |
|------|------|
| 0.1.1 | New command `metis doctor`: checks metis version, project presence (`metis.yaml` found), config validity (reuse `check` internals), and for each configured agent surface whether its CLI is on PATH |
| 0.1.2 | Surface→binary detection table (claude-code→`claude`, opencode→`opencode`, codex→`codex`), overridable per agent (see 0.2) |
| 0.1.3 | `-o json` output: `{ok, checks: [{name, ok, detail, fix_hint}]}` |
| 0.1.4 | Text output: human checklist with ✓/✗; docs/commands.md entry |

### 0.2 Launch command mapping

| Step | Task |
|------|------|
| 0.2.1 | Add optional `command` field to `Agent` config (`agents.<slug>.command`, e.g. `claude`, `opencode run`); default derived from `surface` via the 0.1.2 table |
| 0.2.2 | `metis agents -o json`: list configured agents with slug, surface, model, label, resolved launch command |
| 0.2.3 | Docs: configuration.md (new field), commands.md (`agents`) |

**Success criteria:** the extension never hardcodes a vendor command; adding
a new agent surface is pure configuration.

---

## Phase 1 — Scaffold + CI

**Goal:** `editors/vscode/` builds, lints, tests in CI without touching the
Go pipeline.

| Step | Task |
|------|------|
| 1.1 | Scaffold `editors/vscode/`: `package.json` (publisher, engines.vscode, activationEvents on `workspaceContains:metis.yaml`), `tsconfig.json`, esbuild bundling, eslint |
| 1.2 | Extension entry: activate on metis project detection; version handshake; output channel for diagnostics |
| 1.3 | `MetisClient` service + typed interfaces mirroring the JSON shapes of `status`, `next`, `list`, `show`, `progress`, `findings`, `check`, `version`, `doctor`, `agents` |
| 1.4 | File watcher service: `.metis/**` + `metis.yaml`, debounced refresh bus |
| 1.5 | Unit tests (vitest) for client parsing + watcher debounce; integration harness via `@vscode/test-electron` |
| 1.6 | CI: new `extension` job in ci.yml with `paths: editors/vscode/**` filter; add `paths-ignore` for the Go jobs symmetrically |
| 1.7 | Repo hygiene: `.vscodeignore`, gitignore `node_modules`/`out`, README note that Go module consumers are unaffected |

**Success criteria:** CI green with extension building on its own job; Go
jobs skip on extension-only PRs and vice versa.

---

## Phase 2 — MVP Features (extension v0.1)

**Goal:** the "one window" experience: see state, launch sessions, know the
setup is sane.

| Step | Feature | Consumes |
|------|---------|----------|
| 2.1 | **Ledger sidebar**: tree of slices, status icons (pending/coding/reviewing/done/rework), grouped by status; click → detail panel | `list -o json`, `show -o json` |
| 2.2 | **Status bar item**: `slice-id · role · agent · 42%`; click focuses sidebar | `status -o json` |
| 2.3 | **Launch session**: button on active slice + command palette entry; opens integrated terminal named after the slice, runs the agent's launch command; role-aware (coder/reviewer) | `next -o json`, `agents -o json` |
| 2.4 | **Setup panel** (webview): metis found/version, project validity, per-agent CLI availability; each failing check shows its `fix_hint` with install links | `doctor -o json`, `check -o json` |
| 2.5 | Refresh loop: all views live-update on `.metis/` changes (agent flips a slice → sidebar updates without user action) | watcher |
| 2.6 | Integration tests: fixture project in `editors/vscode/test-fixtures/`, assert tree rendering + status bar against a real metis binary built in CI | — |

**Success criteria:** open a metis project in VS Code → see ledger and
status; click launch → agent session starts in a terminal running kickoff;
break the config → setup panel says exactly what and how to fix.

---

## Phase 3 — Human-Persona Layer (extension v0.2)

| Step | Feature | Consumes |
|------|---------|----------|
| 3.1 | **Progress dashboard** (webview): bars by stage, done/total, review-cycle counts | `progress -o json` |
| 3.2 | **Findings panel**: filterable table (severity/category/slice); right-click → "Promote to accuracy rule" | `findings -o json`, `rule promote` |
| 3.3 | **Overview drift notification**: on refresh, if drift detected show notification with "Run metis recon" action (runs in terminal — recon composes a slice, human should watch) | `check -o json` |
| 3.4 | **Brief + review helpers**: slice context menu → "Open brief", "Show commits" (quick-pick of `git log --grep <id>` opening VS Code native diffs) | fs, git |
| 3.5 | **Config editor** (webview form): render effective config, edits write through `config set` field-by-field; unknown-key/type errors surfaced inline | `config view -o json`, `config set` |

**Success criteria:** the review loop is visible end-to-end in the editor:
progress → findings → promote to rule, without opening a terminal manually.

---

## Phase 4 — Marketplace Release

| Step | Task |
|------|------|
| 4.1 | Extension identity: name `metis`, publisher account, icon, marketplace README (screenshots, the two design laws, prereqs) |
| 4.2 | Release workflow `.github/workflows/extension-release.yml`: triggers on `ext-v*` tags (disjoint from goreleaser's `v*`), runs build + tests, `vsce publish` with `VSCE_PAT` secret |
| 4.3 | Version policy: extension versions independently (`ext-v0.1.0`); `MIN_METIS_VERSION` bumped only when the extension adopts a newer JSON field; CHANGELOG.md in `editors/vscode/` |
| 4.4 | Also publish to Open VSX (Cursor/Windsurf users without MS marketplace) |
| 4.5 | Root README: "Editor Integration" section; docs/vscode.md user guide |

**Success criteria:** `git tag ext-v0.1.0 && git push origin ext-v0.1.0`
publishes to both marketplaces; installing the extension in a fresh VS Code
against a fresh clone reaches a working session launch with no manual steps
beyond what the setup panel prescribes.

---

## Phase 5 — Later (explicitly deferred)

- **Workspace quick-pick**: `workspace list -o json` → open folder picker
- **Auto-download of the metis binary** (gopls pattern) when not on PATH —
  v1 requires PATH install via the setup panel to keep scope tight
- **`metis watch`** event stream replacing file watchers (only if watcher
  proves noisy on large repos)
- **Session observability**: findings trends, review-cycle history
- **JetBrains plugin** (`editors/jetbrains/`) reusing the same CLI contract

---

## Testing Strategy

| Layer | Tool | What |
|-------|------|------|
| Client unit | vitest | JSON parsing against fixture outputs captured from the real binary; error paths (bad exit, missing binary, version mismatch) |
| Contract | CI step | Build metis from the same commit, run each read command against a fixture project, validate output parses into the TS interfaces — this is the drift alarm between Go and TS |
| Integration | @vscode/test-electron | Tree/status rendering, launch command construction (assert terminal creation args, do not actually run agents), setup panel states |
| Manual | checklist in `editors/vscode/TESTING.md` | Real session launch with a real agent CLI before each `ext-v*` tag |

The contract test (row 2) is the load-bearing one: it turns "extension broke
because JSON changed" from a user bug report into a CI failure on the PR
that changed the JSON.

---

## Risks

| Risk | Mitigation |
|------|-----------|
| JSON contract drift between Go and TS | Contract test in CI (above); TS interfaces live in one file mirroring the Go output structs |
| File watcher noise on large `.metis/runs/` | Watch only ledger, findings, briefs, metis.yaml; runs/ excluded |
| Monorepo toolchain friction (Node in a Go repo) | Path-filtered CI; extension fully self-contained under `editors/vscode/`; extraction stays cheap (design law 2) |
| Marketplace publisher/secret management | 4.2 uses a single `VSCE_PAT` secret; document rotation in CONTRIBUTING.md |
| Terminal launch differences per agent CLI | 0.2 launch mapping is config, not code; `doctor` verifies before launch offers |

---

## Sequencing Summary

```
Phase 0 (Go: doctor + agents/launch mapping)   ~2 short PRs
Phase 1 (scaffold + CI)                        ~1 PR
Phase 2 (MVP: sidebar, status, launch, setup)  ~2-3 PRs → ext-v0.1.0
Phase 3 (dashboard, findings, drift, config)   ~2-3 PRs → ext-v0.2.0
Phase 4 (marketplace)                          ~1 PR (can land with ext-v0.1.0)
```

Each phase is independently useful; stopping after Phase 2 still delivers
the core promise (one window: see state, launch sessions, sane setup).
