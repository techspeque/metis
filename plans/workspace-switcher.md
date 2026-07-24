# Workspace Switcher Plan

> Feature: user-level workspace registry + switcher for the human persona
> Agent persona behavior (cwd-based `.metis/` resolution) is unchanged.

---

## Summary

Metis serves two personas. The **agent** runs inside a repo and resolves the
project by walking up from cwd to find `metis.yaml` — this already works and
must not change. The **human** works across many projects and wants to
register workspaces once, then view/switch/operate on them from anywhere.

This plan adds a per-user config at `~/.metis/config.yaml` holding a named
workspace registry and an active selection, a `metis workspace` command group
to manage it, and a well-defined resolution order that keeps the sticky
selection from ever hijacking an agent session.

### Resolution order (the load-bearing decision)

`loadContext()` resolves the project root in this order:

1. `--workspace <name>` flag (persistent, root command) — explicit always wins
2. `METIS_WORKSPACE` env var — scripting/CI override
3. Upward discovery of `metis.yaml` from cwd (existing `config.FindConfig`)
4. `active` workspace from `~/.metis/config.yaml`
5. Error: "no metis project found — run `metis init` here, or
   `metis workspace use <name>`"

Rule 3 beating rule 4 is deliberate and non-negotiable: agents run as the
same OS user as the human and share `~/.metis/`. If the sticky selection
could override cwd discovery, the human switching workspaces would silently
redirect a concurrently running agent session in another repo. With this
ordering the switcher is purely additive — inside a repo, behavior is
byte-for-byte identical to today.

### User config file

Location: `~/.metis/config.yaml` (via `os.UserHomeDir()`; works on macOS and
Linux). The file is deliberately **not** named `metis.yaml` — home is on the
upward-discovery path for repos under `~`, and the resolver must never latch
onto the user config as a project.

```yaml
# ~/.metis/config.yaml — user-level metis configuration
active: metis
workspaces:
  metis: /Users/adam/Documents/GitHub/metis
  acme-api: /Users/adam/proj/acme-api
```

Constraints:
- `workspaces` values are absolute paths to directories containing `metis.yaml`
- `active` must be a key of `workspaces` (or empty)
- Missing file ≡ empty registry — never an error, nothing is created until
  the user (or `metis init`) writes to it

---

## Phase A — User Config Package

**Goal:** load/save/validate `~/.metis/config.yaml`.

| Step | Task | Output |
|------|------|--------|
| A.1 | New package `internal/userconfig/` — `UserConfig` struct (`Active string`, `Workspaces map[string]string`), YAML tags | `internal/userconfig/userconfig.go` |
| A.2 | `Path()` — `filepath.Join(os.UserHomeDir(), ".metis", "config.yaml")`; overridable via `METIS_USER_CONFIG` env var (needed for tests, useful for exotic setups) | same |
| A.3 | `Load()` — missing file returns zero-value config, no error; malformed YAML is an error naming the path | same |
| A.4 | `Save()` — `MkdirAll(~/.metis, 0o755)`, write 0o644, stable key order, comment header (match `writeConfig` style in `internal/cli/rule.go`) | same |
| A.5 | Helpers: `Add(name, path)` (rejects duplicate names, non-absolute paths), `Remove(name)` (clears `active` if it pointed there), `Use(name)` (errors on unknown name) | same |
| A.6 | Unit tests: round-trip, missing file, malformed file, add/remove/use edge cases, env-var path override | `userconfig_test.go` |

**Success criteria:** `go test ./internal/userconfig/...` green; no changes
to any existing package.

---

## Phase B — Resolution Integration

**Goal:** `loadContext()` implements the 5-step resolution order; commands
report which workspace they resolved and how.

| Step | Task | Output |
|------|------|--------|
| B.1 | Add persistent `--workspace` (short `-w`) string flag on root command, bound to a package var | `internal/cli/root.go` |
| B.2 | Extend `context` with `source` field: `flag`, `env`, `cwd`, or `active` | `internal/cli/context.go` |
| B.3 | Rewrite `loadContext()`: flag → env → `config.FindConfig(cwd)` → userconfig active → composite error. Named-workspace lookup errors if the name is unknown or the registered path no longer contains `metis.yaml` (message says the path and suggests `workspace list`) | same |
| B.4 | Provenance line: when `source` is not `cwd`, commands print `workspace: <name> (<path>) [via --workspace|METIS_WORKSPACE|active]` to stderr before output. Silent redirection is how wrong-target mistakes happen | same |
| B.5 | Tests: resolution order precedence (each rule beats the next), unknown name, stale path, agent-path regression test (cwd inside a repo ignores `active` entirely) | `context_test.go` |

**Success criteria:** all existing CLI tests pass unchanged; new precedence
tests green. Running any command inside a repo with a different `active`
workspace set operates on the repo, not the active workspace.

---

## Phase C — Workspace Command Group

**Goal:** `metis workspace` (alias `ws`) with `list`, `add`, `remove`, `use`,
`current`.

| Step | Task | Behavior |
|------|------|----------|
| C.1 | `workspace list` | Table: name, path, markers for `[active]` and `[missing]` (path gone or no `metis.yaml`). Missing is displayed, never auto-pruned — an unmounted volume is not a deleted project |
| C.2 | `workspace add <name> [path]` | Path defaults to cwd's discovered repo root; validates `metis.yaml` exists at target; rejects duplicate names |
| C.3 | `workspace remove <name>` | Removes registry entry only (never touches the project); clears `active` if it pointed there |
| C.4 | `workspace use <name>` | Sets `active`; errors on unknown name; prints confirmation with path |
| C.5 | `workspace current` | Prints active name + path, or "none" (exit 0 either way — informational) |
| C.6 | Tests via `METIS_USER_CONFIG` tempdir | `workspace_test.go` |

**Success criteria:** full add → list → use → current → remove lifecycle
works against a temp user config; `list` renders missing paths without error.

---

## Phase D — Init Integration + Docs

**Goal:** the registry populates itself as a side effect of normal use; docs
updated; agent surface untouched.

| Step | Task | Output |
|------|------|--------|
| D.1 | `metis init` auto-registers the workspace (name = project slug or directory basename; skip silently on name collision with a different path, mention with a one-liner otherwise) | `internal/cli/init.go` |
| D.2 | Docs: `docs/commands.md` (workspace group, `--workspace` flag), `docs/configuration.md` (user config schema + resolution order), README project-layout note | docs |
| D.3 | Verify generated surface adapters (`CLAUDE.md`, `AGENTS.md` templates) contain no mention of workspaces — the agent contract remains "operate on the repo you are in" | `internal/templates/`, `internal/surface/` (audit only) |
| D.4 | `metis check` unaffected — it validates the project, not the user config | audit only |

**Success criteria:** `metis init` in a fresh repo registers it;
`grep -ri workspace` over generated adapter output is empty.

---

## Non-goals

- **Version pinning / self-bootstrap wrapper** — orthogonal; separate plan
- **Per-user behavioral defaults overriding project config** — layering
  headache; not until a concrete need appears
- **Auto-pruning stale registry entries** — display-only `[missing]` marker
- **Workspace-scoped shell integration** (cd-on-use, prompts) — out of scope
