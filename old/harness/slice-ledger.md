# Slice Ledger — Schema and Lifecycle

This document describes the schema of `plans/slices.yaml` (the dispatch ledger)
and `plans/slices-done.yaml` (the archive). All commentary lives here, never in
the YAML files themselves — the YAML is machine-rewritten and read every session.

## Schema version

The ledger is at `version: 2`.

## Top-level keys

| Key | Purpose |
|---|---|
| `version` | Schema version (currently `2`) |
| `agents` | Identity map: slug -> surface/model/plan_tag |
| `slices` | Ordered list of pending and in-flight slices |

## Agent identity map

```yaml
agents:
  <slug>:
    surface: <agent surface>   # claude-code, opencode, codex
    model: <model identifier>
    plan_tag: <human-readable label used in plan docs>
```

Agents self-identify at session start and match against these slugs.

## Slice fields

| Field | Type | Required | Description |
|---|---|---|---|
| `id` | string | yes | Unique slug; appears in commit subjects |
| `title` | string | yes | Human-readable description (decoration only) |
| `plan` | string | no | Path to the plan file (null for bespoke slices) |
| `plan_section` | string | conditional | Required when `plan` is set |
| `coder` | string | yes | Agent slug assigned to code this slice |
| `reviewer` | string | yes | Agent slug assigned to review (must != coder) |
| `risk` | enum | yes | `low`, `medium`, or `high` |
| `stage` | string | no | Optional taxonomy (prototype/mvp/beta) |
| `coded` | bool | yes | Flipped by Coder when implementation is done |
| `reviewed` | bool | yes | Flipped by Reviewer on approval |
| `review_cycles` | int | no | Bumped on each review block (default 0) |
| `notes` | string | no | Skip reasons, exceptions, clarifications |

## Lifecycle

1. **Active slice** = first slice where `coded: false` (role: Coder) OR where
   `coded: true` and `reviewed: false` (role: Reviewer).
2. **Coder done** -> flip `coded: true`.
3. **Reviewer approves** -> flip `reviewed: true`, then archive.
4. **Reviewer blocks** -> increment `review_cycles`, log finding, leave booleans.
5. **Redo** -> set both booleans false with a `notes:` reason.
6. **Skip** -> set both true with a `notes:` reason.

## Declaration order

Slices execute in declaration order. This is the dependency order — do not
reorder slices without understanding the implications.

## Special slice types

- `phase-N-gate` — validates a phase's composed system; "code" is integration
  scenarios + evidence report.
- `docs-recon-N` — sweeps plan/ADR drift against observed code.

## Archive

Fully-done slices (`coded && reviewed`) are moved from `plans/slices.yaml` to
`plans/slices-done.yaml` by the `archive` command to keep the active ledger lean.
