// Package instructions implements the dynamic agent contract generation engine.
// It assembles risk-scaled instructions from metis.yaml configuration.
package instructions

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/techspeque/metis/internal/config"
	"github.com/techspeque/metis/internal/slice"
)

// Generate assembles the full agent contract from configuration.
func Generate(cfg *config.Config, repoRoot string) string {
	var b strings.Builder
	for _, section := range allSections(cfg, repoRoot) {
		b.WriteString(section)
		b.WriteString("\n\n")
	}
	return strings.TrimSpace(b.String())
}

// GenerateForSlice assembles a risk-scaled contract for a specific slice.
func GenerateForSlice(cfg *config.Config, s *slice.Slice, repoRoot string) string {
	var b strings.Builder
	for _, section := range filteredSections(cfg, s.Risk, repoRoot) {
		b.WriteString(section)
		b.WriteString("\n\n")
	}
	return strings.TrimSpace(b.String())
}

// GenerateKickoff generates the session protocol.
func GenerateKickoff(cfg *config.Config, role string) string {
	var b strings.Builder

	b.WriteString("# Metis Session Protocol\n\n")
	b.WriteString("Follow these steps at the start of every session.\n\n")

	b.WriteString("## Step 1: Establish State\n\n")
	b.WriteString("```bash\n")
	fmt.Fprintf(&b, "git rev-parse --abbrev-ref HEAD   # must be '%s'\n", cfg.Project.IntegrationBranch)
	b.WriteString("git status                        # must be clean\n")
	b.WriteString("metis status                      # quick orientation\n")
	b.WriteString("```\n\n")
	b.WriteString("Dirty tree -> stop. Wrong branch -> stop. Report to human.\n\n")

	b.WriteString("## Step 2: Find Active Slice\n\n")
	b.WriteString("```bash\nmetis next\n```\n\n")
	b.WriteString("Trust this over any manual reading. If no slices remain, report \"backlog empty\" and stop.\n\n")

	b.WriteString("## Step 3: Self-Identify\n\n")
	b.WriteString("State identity in one line. Match against the required model slug from `metis next` output.\n")
	b.WriteString("- Match -> continue\n- No match -> stop, report which agent is needed\n\n")

	b.WriteString("## Step 4: Read Instructions\n\n")
	b.WriteString("```bash\nmetis instructions --for <slice-id>\n```\n\n")
	b.WriteString("Read the output. This is the risk-scaled contract plus contextual archaeology.\n\n")

	b.WriteString("## Step 5: Pre-flight Verification\n\n")
	b.WriteString("```bash\nmetis verify --pre\n```\n\n")
	b.WriteString("- Exit 2 (env failure) -> stop, report verbatim, do not modify code\n")
	b.WriteString("- Exit 1 (code failure before your changes) -> stop, report pre-existing breakage\n")
	b.WriteString("- Exit 0 -> continue\n\n")

	if role == "" || role == "coder" {
		b.WriteString("## Step 6a: Coder Flow\n\n")
		b.WriteString("1. **Read interfaces** — `metis interfaces` output (if configured)\n")
		b.WriteString("2. **Write brief** — `metis brief <id> --write`, edit it, `metis commit --brief`\n")
		b.WriteString("3. **Implement** — within declared scope only\n")
		b.WriteString("4. **Verify** — `metis verify --post`\n")
		b.WriteString("5. **Flip** — `metis commit --flip coded`\n")
		b.WriteString("6. **Report** — slice ID, files changed, verify result, what's next\n\n")
	}

	if role == "" || role == "reviewer" {
		b.WriteString("## Step 6b: Reviewer Flow\n\n")
		b.WriteString("1. **Locate commits** — `git log --oneline --grep \"<slice-id>\"`\n")
		b.WriteString("2. **Read brief** — `metis brief <id>`\n")
		b.WriteString("3. **Independent verify** — `metis verify --post`\n")
		b.WriteString("4. **Walk checklist** — one-line verdict per item, citing `file:line`\n")
		b.WriteString("5. **Verdict:**\n")
		b.WriteString("   - Pass -> `metis commit --flip reviewed` then `metis archive`\n")
		b.WriteString("   - Block -> `metis block <id> --severity ... --category ... --finding \"...\"`\n")
		b.WriteString("6. **Report** — slice ID, verdict, findings (if any), what's next\n\n")
	}

	return strings.TrimSpace(b.String())
}

// allSections returns all instruction sections in assembly order.
func allSections(cfg *config.Config, repoRoot string) []string {
	sections := []string{sectionHeader(cfg)}

	// Overview is included at all levels — agents always need the full picture
	if ov := sectionOverview(cfg, repoRoot); ov != "" {
		sections = append(sections, ov)
	}

	sections = append(sections,
		sectionSessionProtocol(),
		sectionBranchCommit(cfg),
		sectionDoD(),
		sectionRoles(),
		sectionHotPaths(cfg),
		sectionScope(),
		sectionRouting(cfg),
		sectionTesting(cfg),
		sectionNonGoals(cfg),
		sectionAccuracyRules(cfg),
		sectionReviewChecklist(cfg),
		sectionFeedbackLoop(),
		sectionToolingMap(),
	)
	return sections
}

// filteredSections returns sections filtered by risk level.
func filteredSections(cfg *config.Config, risk slice.Risk, repoRoot string) []string {
	sections := []string{sectionHeader(cfg)}

	// Overview is included at all risk levels
	if ov := sectionOverview(cfg, repoRoot); ov != "" {
		sections = append(sections, ov)
	}

	sections = append(sections,
		sectionSessionProtocol(),
		sectionBranchCommit(cfg),
		sectionDoD(),
		sectionRoles(),
	)

	if risk == slice.RiskMedium || risk == slice.RiskHigh {
		sections = append(sections, sectionHotPaths(cfg))
	}

	sections = append(sections, sectionScope())

	if risk == slice.RiskHigh {
		sections = append(sections, sectionRouting(cfg))
	}

	sections = append(sections, sectionTesting(cfg))
	sections = append(sections, sectionNonGoals(cfg))

	if risk == slice.RiskMedium || risk == slice.RiskHigh {
		sections = append(sections, sectionAccuracyRules(cfg))
		sections = append(sections, sectionReviewChecklist(cfg))
	}

	if risk == slice.RiskHigh {
		sections = append(sections, sectionFeedbackLoop())
	}

	sections = append(sections, sectionToolingMap())
	return sections
}

func sectionHeader(cfg *config.Config) string {
	return fmt.Sprintf("# %s — Agent Contract\n\nThis contract governs all autonomous work in this repository.", cfg.Project.Name)
}

func sectionSessionProtocol() string {
	return "## Session Start Protocol\n\nEvery autonomous session begins by running `metis kickoff` from step 1.\nNo pasted prompt is needed. The CLI provides all context."
}

func sectionBranchCommit(cfg *config.Config) string {
	var b strings.Builder
	b.WriteString("## Branch & Commit Rules\n\n")
	fmt.Fprintf(&b, "- All work lands on the `%s` branch. Never commit to `%s`.\n", cfg.Project.IntegrationBranch, cfg.Project.ReleaseBranch)
	b.WriteString("- Use `metis commit` for all commits — it enforces format and strips attribution.\n")
	fmt.Fprintf(&b, "- Commit format: `%s`\n", cfg.Commits.Format)
	fmt.Fprintf(&b, "- Allowed prefixes: %s\n", strings.Join(cfg.Commits.Prefixes, ", "))
	b.WriteString("- Every commit subject contains the slice ID.\n")
	b.WriteString("- No AI attribution in commits (Co-Authored-By, Generated with, model names).")
	return b.String()
}

func sectionDoD() string {
	return `## Definition of Done

A slice is done only when ALL hold:
1. Implementation matches the brief in .metis/briefs/<slice-id>.md
2. Tests proportional to the testing rules exist and pass
3. ` + "`metis verify`" + ` is green, confirmed independently by the Reviewer
4. The Reviewer walked the checklist with no blocking findings
5. Ledger and brief are committed; commit subjects carry the slice ID`
}

func sectionRoles() string {
	return `## Roles

- **Coder** — implements one slice within its declared file scope; owns its tests
- **Reviewer** — reviews one slice against the checklist; re-runs verification independently; owns the sign-off
- **Human** — owns planning, scope conflicts, escalations, and release merges

Reviews are cross-vendor by default.`
}

func sectionHotPaths(cfg *config.Config) string {
	if len(cfg.HotPaths) == 0 {
		return "## Hot-Path Zones\n\n(None configured)"
	}
	var b strings.Builder
	b.WriteString("## Hot-Path Zones\n\nAny slice touching these paths is risk: high and gets full-depth reading:\n")
	for _, p := range cfg.HotPaths {
		fmt.Fprintf(&b, "- %s\n", p)
	}
	return b.String()
}

func sectionScope() string {
	return `## Scope Discipline

- Before any code, commit a brief declaring file scope
- Implement only within declared files
- Genuinely-required out-of-scope fixes go in the brief's "Out-of-scope touches" section
- If the slice needs a non-goal item, or scope differs materially from the plan, stop and report`
}

func sectionRouting(cfg *config.Config) string {
	var b strings.Builder
	b.WriteString("## Model Routing\n\n")
	if len(cfg.Routing.High) > 0 {
		fmt.Fprintf(&b, "- High risk: %s\n", strings.Join(cfg.Routing.High, ", "))
	}
	if len(cfg.Routing.Medium) > 0 {
		fmt.Fprintf(&b, "- Medium risk: %s\n", strings.Join(cfg.Routing.Medium, ", "))
	}
	if len(cfg.Routing.Low) > 0 {
		fmt.Fprintf(&b, "- Low risk: %s\n", strings.Join(cfg.Routing.Low, ", "))
	}
	fmt.Fprintf(&b, "- Review: %s", cfg.Routing.Review)
	return b.String()
}

func sectionTesting(cfg *config.Config) string {
	if len(cfg.Testing) == 0 {
		return "## Testing Rules\n\n(None configured)"
	}
	var b strings.Builder
	b.WriteString("## Testing Rules\n\n")
	for _, r := range cfg.Testing {
		fmt.Fprintf(&b, "- %s\n", r)
	}
	return b.String()
}

func sectionNonGoals(cfg *config.Config) string {
	if len(cfg.NonGoals) == 0 {
		return "## Non-Goals\n\n(None configured)"
	}
	var b strings.Builder
	b.WriteString("## Non-Goals (Do Not Implement)\n\n")
	for _, ng := range cfg.NonGoals {
		fmt.Fprintf(&b, "- %s\n", ng)
	}
	return b.String()
}

func sectionAccuracyRules(cfg *config.Config) string {
	if len(cfg.AccuracyRules) == 0 {
		return "## Accuracy Rules\n\n(None configured)"
	}
	var b strings.Builder
	b.WriteString("## Accuracy Rules\n\nProject invariants that must never be violated:\n")
	for i, r := range cfg.AccuracyRules {
		fmt.Fprintf(&b, "%d. %s\n", i+1, r)
	}
	return b.String()
}

func sectionReviewChecklist(cfg *config.Config) string {
	if len(cfg.ReviewChecklist) == 0 {
		return "## Review Checklist\n\n(None configured)"
	}
	var b strings.Builder
	b.WriteString("## Review Checklist\n\nWalk in order; one-line verdict per item citing file:line evidence:\n")
	for i, item := range cfg.ReviewChecklist {
		fmt.Fprintf(&b, "%d. %s\n", i+1, item)
	}
	return b.String()
}

func sectionFeedbackLoop() string {
	return `## Feedback Loop

- Every blocking review finding is logged via ` + "`metis block`" + `
- Findings tracked in .metis/findings.yaml
- Recurring failures graduate into new accuracy rules (` + "`metis rule promote`" + `)
- review_cycles per slice provides routing evidence
- Phase gates validate composed system behavior`
}

func sectionToolingMap() string {
	return `## Tooling Map

| Command | Purpose |
|---|---|
| ` + "`metis next`" + ` | Find active slice, role, required model |
| ` + "`metis kickoff`" + ` | Session protocol steps |
| ` + "`metis instructions --for <id>`" + ` | Risk-scaled contract for a slice |
| ` + "`metis brief <id> --write`" + ` | Generate brief template |
| ` + "`metis verify --pre`" + ` | Pre-flight verification |
| ` + "`metis verify --post`" + ` | Post-implementation verification |
| ` + "`metis env-check`" + ` | Environment soundness check |
| ` + "`metis interfaces`" + ` | Regenerate API summary |
| ` + "`metis commit -m \"...\"`" + ` | Commit with enforced format |
| ` + "`metis commit --brief`" + ` | Commit the brief |
| ` + "`metis commit --flip coded`" + ` | Flip coded and commit |
| ` + "`metis commit --flip reviewed`" + ` | Flip reviewed and commit |
| ` + "`metis block <id>`" + ` | Block a slice during review |
| ` + "`metis archive`" + ` | Move done slices to archive |
| ` + "`metis check`" + ` | Validate config + ledger |
| ` + "`metis status`" + ` | One-line progress summary |`
}

func sectionOverview(cfg *config.Config, repoRoot string) string {
	if cfg.Project.Overview == "" {
		return ""
	}

	overviewPath := filepath.Join(repoRoot, cfg.Project.Overview)
	data, err := os.ReadFile(overviewPath)
	if err != nil {
		return ""
	}

	content := strings.TrimSpace(string(data))
	if content == "" {
		return ""
	}

	return fmt.Sprintf("## Project Overview\n\nSource: `%s`\n\n%s", cfg.Project.Overview, content)
}
