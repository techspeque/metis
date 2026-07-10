# CAESAR Planning Prompt

You are a senior software architect. Your task is to produce the complete
planning artifacts for CAESAR — a greenfield Go project that must be built by
autonomous coding agents operating under a structured harness.

**Your output will be consumed by machines.** Workstreams become slices; acceptance
criteria become Definitions of Done; suggested packages become file-scope
declarations. Be precise and testable.

---

## Source material

**`docs/OVERVIEW.MD` is the primary source of truth.** It is the authoritative
product specification — every architectural decision, every module boundary,
every interface shape, every phase breakdown, and every success criterion in
your outputs must be traceable back to a specific section of the OVERVIEW. When
in doubt about what to build, the OVERVIEW decides. The other files constrain
*how* the plan is structured and executed, not *what* is built.

Read these files in full before producing anything:

1. `docs/OVERVIEW.MD` — **PRIMARY.** The complete product specification: purpose,
   research objective, design principles, non-goals, application flow, modules
   (§7.1–7.11), evaluator implementations (§8.1–8.12), schemas (§11–14), API
   (§15), CLI (§16), configuration (§17), package structure (§18), testing
   requirements (§19), phases (§21), MVP criteria (§22), dissertation target
   (§23), success criteria (§24). Everything you plan must derive from this.
2. `AGENTS.md` — **PROCESS CONSTRAINTS.** The agent contract: hot-path zones §6,
   testing rules §9, non-goals §10, accuracy rules §11, model routing §8.
   These constrain how work is structured and assigned, not what is built.
3. `harness/HARNESS.md` — **STRUCTURE TEMPLATE.** Part A (planning regime), §5
   (ledger schema), Part C (greenfield cold-start). Defines the format and
   lifecycle of your outputs.
4. `harness/slice-ledger.md` — **FIELD REFERENCE.** Ledger schema for slices.yaml.

---

## Outputs required

Produce these artifacts in order. Each is a separate file committed together.
Every output must cite the OVERVIEW section(s) it derives from.

### 1. `docs/ARCHITECTURE.md`

A concise (< 300 lines) system architecture document covering:

- **System boundary:** what CAESAR is and is not (derive from OVERVIEW §1, §4)
- **Component diagram** (text/mermaid): the module graph from OVERVIEW §7 showing
  data flow between API, domain, classifier, planner, evaluators, scoring, trace,
  storage, LLM, experiments, export
- **Package layout:** the Go package tree from OVERVIEW §18, with one-sentence
  purpose per package
- **Key interfaces:** the Evaluator interface (OVERVIEW §7.5), the Storage
  interface, the LLM Client interface — declared as Go code blocks
- **Data flow:** the simplified flow from OVERVIEW §5 expanded with package names
- **Configuration:** from OVERVIEW §17
- **Deployment model:** single binary + SQLite file (derive from §4, §17)
- **Extension points:** where AIMux integration would attach (OVERVIEW §3.3)
- **Guiding constraints:** the 10 design principles from OVERVIEW §3, restated as
  architectural invariants

Keep it factual and structural. No rationale essays — those go in ADRs.

### 2. `docs/ROADMAP.md`

A phase-level roadmap. Derive from OVERVIEW §21 but restructure for the harness:

- **Phase 0 — Bootstrap:** foundational ADRs + interface contracts + project
  scaffold + verify gate. This phase has no feature code — it creates the
  contracts later phases consume. (See HARNESS.md Part C, Step 2.)
- **Phases 1–7:** map from OVERVIEW §21, with:
  - Phase title and goal (one sentence)
  - Success criteria (from the overview)
  - Key risks or dependencies
  - Which evaluation modes become testable at this phase boundary
- **MVP gate** after Phase 3 (map to OVERVIEW §22 — deterministic eval works
  end-to-end without LLM)
- **Dissertation gate** after Phase 7 (map to OVERVIEW §23)

State the guiding constraints that phases must preserve (from OVERVIEW §3):
- Deterministic before LLM
- Hard gates override weighted scores
- System works without LLM access through Phase 4
- Each phase ends with a composition gate

### 3. Architecture Decision Records

Produce 5–7 foundational ADRs in `docs/adr/`. Use the template at
`docs/adr/_template.md`. Number them 0001–000N. Mandatory decisions:

- **ADR-0001: Go module structure** — single module, `internal/` packages, no
  `pkg/` (rationale: dissertation prototype, not a library)
- **ADR-0002: SQLite as sole storage backend** — no PostgreSQL, embed via
  `modernc.org/sqlite` or `mattn/go-sqlite3`; migrations in code
- **ADR-0003: Evaluator interface contract** — the `Evaluator` interface shape,
  why a single interface (not per-type), registration via a registry
- **ADR-0004: Hard gates override weighted scores** — scoring semantics, why
  short-circuit, how to report
- **ADR-0005: LLM adapter behind interface** — provider-agnostic, mock-first for
  testing, structured response contract
- **ADR-0006: REST JSON API only** — no gRPC, standard library router or
  chi/echo, versioning approach

Optional (include if material to Phase 0/1):
- ADR-0007: Structured logging approach (slog)
- ADR-0008: Configuration loading (env + YAML file)

Update `docs/adr/README.md` index with all new ADRs.

### 4. `plans/caesar-implementation-plan.md`

The detailed implementation plan. Structure per HARNESS.md §A.4.

**Derivation rule:** The plan decomposes OVERVIEW §21 (phases) into workstreams.
Each workstream's tasks come from the corresponding OVERVIEW module section
(§7.1–7.11 for modules, §8.1–8.12 for evaluators). Acceptance criteria come from
the OVERVIEW's success criteria (per phase) and the specific inputs/outputs
defined for each module/evaluator. If something is in the plan but not in the
OVERVIEW, it must be justified by a direct dependency. If something is in the
OVERVIEW but not in the plan, explain why it is deferred or covered implicitly.

Structure:

```
Plan
└── Phase N
    └── Workstream N.M
        ├── Tasks
        ├── Suggested packages
        └── Acceptance criteria
```

**Critical rules for workstream design:**

- Each workstream = exactly one slice in the ledger
- Workstreams should be completable in a single agent session (2–4 hours of
  focused work, ~500–1500 lines of code including tests)
- Acceptance criteria must be **testable** — "code compiles", "test X passes",
  "endpoint returns Y" — never vague ("well-structured", "clean")
- Order workstreams by dependency: later workstreams may consume interfaces from
  earlier ones, but never the reverse
- Tag each workstream with risk level: `low`, `medium`, or `high`
- Tag each with coder assignment using the routing rules:
  - `risk: high` + hot-path (evaluators, scoring, planner, LLM) ->
    `claude-code/opus` or `opencode/opus`
  - `risk: medium` (API, storage, CLI, config, export) -> `codex`
  - `risk: low` (docs, fixtures, reconciliation) -> `codex`

**Phase 0 workstreams** (mandatory, from HARNESS.md Part C Step 2):

- `phase-0-ws-0.1` — Foundational ADRs (commit the ADRs from output #3 above).
  Risk `high`. Output: ADRs only.
- `phase-0-ws-0.2` — Interface contracts and package seams. Risk `high`. Create
  empty packages (`internal/domain`, `internal/evaluators`, `internal/scoring`,
  etc.) with the core interface definitions (`Evaluator`, `Store`, `LLMClient`,
  etc.) and `go.mod`. No implementation. This is what makes `make interfaces`
  meaningful from workstream 0.3 onward.
- `phase-0-ws-0.3` — Project scaffold and verify gate. Risk `medium`. Wire up
  `go.mod`, basic `cmd/caesar/main.go`, structured logging, `make verify` with
  real go build/vet/test. After this, the pre-flight check is no longer vacuous.

**Phase 1 workstreams** — break OVERVIEW §21 Phase 1 into ~4–6 workstreams:

- Domain model types (entities from OVERVIEW §7.2)
- Config loading (OVERVIEW §17)
- SQLite storage — schema + migrations + CRUD for tasks/traces
- Basic REST API — health + create/get task + submit trace
- Dataset loader (JSONL/YAML -> TaskSpec + AgentTrace)

**Phase 2 workstreams** — break OVERVIEW §21 Phase 2 into ~6–8 workstreams:

- Evaluator registry + base types
- Required fields evaluator (OVERVIEW §8.2)
- Exact match evaluator (OVERVIEW §8.3)
- Tool choice evaluator + forbidden tool gate (OVERVIEW §8.4)
- Tool argument evaluator (OVERVIEW §8.5)
- Policy gate evaluator (OVERVIEW §8.9)
- Scoring engine — hard gates + weighted scoring (OVERVIEW §7.6)
- Integration: POST /tasks/{id}/evaluate endpoint wired end-to-end

**Phases 3–7** — break similarly. Each phase ends with a `phase-N-gate`
workstream that validates composition.

**Sizing guidance:**
- Total workstreams across all phases: 35–50 (not more — each is one agent
  session; too many creates dispatch overhead)
- Phase 0: 3 workstreams
- Phases 1–2: 10–14 workstreams (these are the MVP core)
- Phases 3–7: 20–30 workstreams
- Add 1 `phase-N-gate` per phase (7 gates total)
- Add 1–2 `docs-recon-N` slices at phase 3 and phase 6 boundaries

### 5. `plans/slices.yaml`

Seed the ledger from the plan. For each workstream, produce a slice entry per
the schema in `harness/slice-ledger.md`:

```yaml
- id: phase-N-ws-N.M
  title: <from workstream title>
  plan: plans/caesar-implementation-plan.md
  plan_section: <section reference>
  coder: <agent slug per routing>
  reviewer: <different agent slug — cross-vendor>
  risk: <low|medium|high>
  stage: prototype    # Phase 0-1 = prototype, Phase 2-4 = mvp, Phase 5-7 = beta
  coded: false
  reviewed: false
  review_cycles: 0
```

**Routing rules for reviewer assignment (cross-vendor):**
- If coder is `claude-code/opus` -> reviewer is `codex` or `opencode/opus`
- If coder is `opencode/opus` -> reviewer is `claude-code/opus` or `codex`
- If coder is `codex` -> reviewer is `claude-code/opus` or `opencode/opus`

Distribute review load roughly evenly across the three agents. Prefer giving
high-risk reviews to `claude-code/opus` or `opencode/opus`.

---

## Constraints to observe

1. **Non-goals (AGENTS.md §10):** Do not plan workstreams for: frontend/UI, full
   AIMux integration, OpenTelemetry ingestion, live monitoring, real data,
   multi-tenant, auth/RBAC, complex orchestration, Kubernetes, gRPC.

2. **Accuracy rules (AGENTS.md §11):** The plan must not create parallel
   evaluator abstractions, must keep domain model in `internal/domain/`, must
   treat hard gates as overriding weighted scores, must not depend on LLM for
   Phases 0–4.

3. **Testing rules (AGENTS.md §9):** Every workstream that produces code must
   have acceptance criteria that include specific test expectations. Mock only at
   trust boundaries. LLM evaluators use mock client. Integration tests use
   in-memory SQLite.

4. **Hot-path awareness (AGENTS.md §6):** Workstreams touching `internal/evaluators/`,
   `internal/scoring/`, `internal/planner/`, `internal/storage/sqlite/migrations.go`,
   or `internal/llm/` must be tagged `risk: high`.

5. **Build priority (OVERVIEW §26):** Domain model -> Evaluator interface ->
   Rule-based planner -> Deterministic evaluators -> Trace schema -> Scoring
   engine -> Storage/exports -> LLM judge last.

6. **Single interface principle:** All evaluators implement one `Evaluator`
   interface. The registry maps names to implementations. No type-specific
   interfaces.

---

## Quality criteria for the plan

The plan is ready for execution when:

- [ ] Every workstream has 2–5 specific, testable acceptance criteria
- [ ] Every workstream lists the exact packages it touches
- [ ] Dependency order is explicit (no workstream consumes an interface defined
  in a later workstream)
- [ ] Phase 0 creates enough structure for `make interfaces` to produce useful
  output
- [ ] Phase gates exist and their criteria reference composition (not individual
  units)
- [ ] The MVP gate (end of Phase 3) maps to OVERVIEW §22 success criteria
- [ ] The dissertation gate (end of Phase 7) maps to OVERVIEW §23
- [ ] Risk tags are consistent with AGENTS.md §6 hot-path zones
- [ ] Cross-vendor review is honored (coder != reviewer, different surfaces)
- [ ] Total workstream count is 35–50 (manageable dispatch)
- [ ] `make slices-check` passes on the seeded ledger
- [ ] Every workstream is traceable to a specific OVERVIEW section (§N.M) — if
  you cannot cite the OVERVIEW section a workstream derives from, the workstream
  is either out of scope or the OVERVIEW has a gap (report the gap, do not
  silently invent features)
- [ ] All 12 evaluator types from OVERVIEW §8.1–8.12 appear in the plan
- [ ] All 11 modules from OVERVIEW §7.1–7.11 appear in the plan
- [ ] The plan's phase success criteria match OVERVIEW §21 verbatim or are strict
  subsets (never invent new success criteria)
