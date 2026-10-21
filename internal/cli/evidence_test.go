package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/techspeque/metis/internal/brief"
	"github.com/techspeque/metis/internal/config"
	"github.com/techspeque/metis/internal/findings"
	"github.com/techspeque/metis/internal/git"
)

// writeBrief puts a brief for feat-0001 on disk, uncommitted.
func writeBrief(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, ".metis", "briefs", "feat-0001.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// writeFindings seeds the findings store with two open findings on
// feat-0001, f-001 and f-002.
func writeFindings(t *testing.T, dir string) {
	t.Helper()
	store := `findings:
  - id: f-001
    date: "2026-10-01"
    slice: feat-0001
    severity: P2
    category: security
    finding: issuer query string reflected into the startup error at internal/oidc/issuer.go:88
    status: open
  - id: f-002
    date: "2026-10-02"
    slice: feat-0001
    severity: P2
    category: security
    finding: the same issuer reflection, seen from the other refusal branch
    status: open
`
	if err := os.WriteFile(filepath.Join(dir, ".metis", "findings.yaml"), []byte(store), 0o644); err != nil {
		t.Fatal(err)
	}
}

func loadStore(t *testing.T, dir string) *findings.Store {
	t.Helper()
	store, err := findings.Load(filepath.Join(dir, ".metis", "findings.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return store
}

// runWith sets the command's flags for one call and resets them after.
func runWith(t *testing.T, cmd *cobra.Command, flags map[string]string, args ...string) error {
	t.Helper()
	resetFlags(t, cmd.Flags())
	for k, v := range flags {
		if err := cmd.Flags().Set(k, v); err != nil {
			t.Fatalf("set --%s: %v", k, err)
		}
	}
	return cmd.RunE(cmd, args)
}

// The scope audit and the brief commit read the same parser. A brief whose
// owned_paths the audit cannot measure used to commit, flip coded, and fail
// review a cycle later — the single most repeated finding in one project's
// history. Now it does not commit.
func TestCommitBriefRefusesAScopeTheAuditCannotRead(t *testing.T) {
	dir := makeDevProject(t)
	setOutputFlag(t, "")

	for name, brief := range map[string]string{
		"annotated entry":  "## Declared file scope\n\n- **owned_paths:** internal/api/ (handlers only)\n- **read_only_paths:** internal/model/\n",
		"no scope section": "## Goal\n\nShip it.\n",
		"placeholder":      "- **owned_paths:** exact files or directories this slice may edit (plain paths)\n",
		"owned twice":      "- **owned_paths:** internal/api/\n- **read_only_paths:** internal/api/\n",
	} {
		t.Run(name, func(t *testing.T) {
			writeBrief(t, dir, brief)
			setCommitFlags(t, map[string]string{"brief": "true"})
			err := commitCmd.RunE(commitCmd, nil)
			if err == nil {
				t.Fatal("the brief committed")
			}
			if !strings.Contains(err.Error(), "cannot commit the brief") {
				t.Errorf("error = %q, want it to name the brief", err)
			}
			if log := gitOut(t, dir, "log", "--format=%s"); strings.Contains(log, "slice brief") {
				t.Error("a brief commit landed despite the refusal")
			}
		})
	}
}

// Read-only work declares "none", commits, and the audit then holds the
// slice to it: a touched file is out of scope, not unverifiable.
func TestCommitBriefAcceptsNoneAndTheAuditMeasuresAgainstIt(t *testing.T) {
	dir := makeDevProject(t)
	setOutputFlag(t, "")
	writeBrief(t, dir, "## Declared file scope\n\n- **owned_paths:** none\n- **read_only_paths:** everything — this slice reads and reports\n")
	setCommitFlags(t, map[string]string{"brief": "true"})
	if err := commitCmd.RunE(commitCmd, nil); err != nil {
		t.Fatalf("a read-only brief was refused: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "notes.md"), []byte("edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitOut(t, dir, "add", "notes.md")
	gitOut(t, dir, "commit", "-qm", "feat(feat-0001): edit a file anyway")

	ctx, err := loadContext()
	if err != nil {
		t.Fatal(err)
	}
	commits, err := git.SliceCommits(dir, "feat-0001")
	if err != nil {
		t.Fatal(err)
	}
	report := auditSlice(ctx, "feat-0001", commits)
	if !report.ScopeVerifiable {
		t.Fatal("owned_paths: none read as no scope")
	}
	if !brief.InScope("notes.md", report.OutOfScope) {
		t.Errorf("out of scope = %v, want notes.md among them", report.OutOfScope)
	}
	if report.OK {
		t.Error("the audit passed a read-only slice that edited a file")
	}
}

// A well-formed brief commits as before; a gate brief has no file scope
// to declare and is not held to one.
func TestCommitBriefAcceptsTheConventionAndExemptsGates(t *testing.T) {
	dir := makeDevProject(t)
	setOutputFlag(t, "")
	writeBrief(t, dir, "- **owned_paths:**\n  - internal/api/\n  - docs/copy.md — added in review cycle 1\n- **read_only_paths:** internal/model/\n")
	setCommitFlags(t, map[string]string{"brief": "true"})
	if err := commitCmd.RunE(commitCmd, nil); err != nil {
		t.Fatalf("a conventional brief was refused: %v", err)
	}

	replaceInFile(t, filepath.Join(dir, ".metis", "slices.yaml"), "type: feat", "type: gate")
	gitOut(t, dir, "commit", "-aqm", "chore(feat-0001): gate")
	writeBrief(t, dir, "# Gate\n\n## Scenarios\n\n- the composed run\n")
	setCommitFlags(t, map[string]string{"brief": "true"})
	if err := commitCmd.RunE(commitCmd, nil); err != nil {
		t.Fatalf("a gate brief was held to a file scope: %v", err)
	}
}

// "Fixed per brief" forty-four times in one afternoon is not a ledger of
// fixes. A resolution names the commit and what shows it, or it does not
// resolve.
func TestFindingsResolveNeedsEvidence(t *testing.T) {
	dir := makeDevProject(t)
	setOutputFlag(t, "")
	writeFindings(t, dir)

	for name, flags := range map[string]map[string]string{
		"note alone":          {"note": "fixed per brief"},
		"commit alone":        {"commit": "HEAD"},
		"evidence alone":      {"test": "TestIssuerRefused"},
		"unknown commit":      {"commit": "0123456789abcdef0123456789abcdef01234567", "test": "TestIssuerRefused"},
		"ref not at commit":   {"commit": "HEAD", "ref": "internal/oidc/issuer.go:88"},
		"ref without a line":  {"commit": "HEAD", "ref": ".metis/slices.yaml"},
		"two dispositions":    {"duplicate-of": "f-002", "superseded-by": "f-002"},
		"disposition and fix": {"duplicate-of": "f-002", "commit": "HEAD", "test": "TestX"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := runWith(t, findingsResolveCmd, flags, "f-001"); err == nil {
				t.Fatal("resolved without evidence")
			}
			if loadStore(t, dir).FindByID("f-001").Status != "open" {
				t.Error("a refused resolution changed the status")
			}
		})
	}

	head := strings.TrimSpace(gitOut(t, dir, "rev-parse", "HEAD"))
	if err := runWith(t, findingsResolveCmd, map[string]string{"commit": "HEAD", "ref": ".metis/slices.yaml:1", "test": "TestIssuerRefused"}, "f-001"); err != nil {
		t.Fatal(err)
	}
	f := loadStore(t, dir).FindByID("f-001")
	if f.Status != "resolved" || f.Commit != head || !strings.Contains(f.Evidence, "TestIssuerRefused") || !strings.Contains(f.Evidence, ".metis/slices.yaml:1") {
		t.Errorf("resolved finding = %+v", f)
	}
	if status := gitOut(t, dir, "status", "--short"); strings.TrimSpace(status) != "" {
		t.Errorf("tree dirty after resolve:\n%s", status)
	}

	if err := runWith(t, findingsResolveCmd, map[string]string{"duplicate-of": "f-001"}, "f-002"); err != nil {
		t.Fatal(err)
	}
	if f := loadStore(t, dir).FindByID("f-002"); f.Status != "resolved" || f.Disposition != "duplicate" || f.Of != "f-001" {
		t.Errorf("disposed finding = %+v", f)
	}
}

// A waiver is recorded as a waiver — never as resolved — with the terms
// that let someone revisit it.
func TestFindingsWaiveRecordsTheDecision(t *testing.T) {
	dir := makeDevProject(t)
	setOutputFlag(t, "")
	writeFindings(t, dir)

	if err := runWith(t, findingsWaiveCmd, map[string]string{"reason": "accepted for alpha"}, "f-001"); err == nil {
		t.Fatal("waived without saying who")
	}
	if err := runWith(t, findingsWaiveCmd, map[string]string{"reason": "accepted for alpha", "by": "human", "residual": "the query still reaches the log", "revisit": "first external issuer"}, "f-001"); err != nil {
		t.Fatal(err)
	}
	f := loadStore(t, dir).FindByID("f-001")
	if f.Status != "waived" || f.Severity != "P2" || f.Waiver == nil || f.Waiver.By != "human" || f.Waiver.Revisit != "first external issuer" {
		t.Errorf("waived finding = %+v (waiver %+v)", f, f.Waiver)
	}

	setOutputFlag(t, "json")
	var stats findings.Stats
	resetFlags(t, findingsCmd.Flags())
	if err := findingsCmd.Flags().Set("stats", "true"); err != nil {
		t.Fatal(err)
	}
	captureJSON(t, findingsCmd, nil, &stats)
	if stats.Waived != 1 || stats.ByStatus["waived"] != 1 || stats.ByStatus["resolved"] != 0 {
		t.Errorf("stats = %+v, want the waiver counted apart from fixes", stats)
	}
	if status := gitOut(t, dir, "status", "--short"); strings.TrimSpace(status) != "" {
		t.Errorf("tree dirty after waive:\n%s", status)
	}
}

// A correction keeps the classification as raised and takes an aggregator
// row out of the counts without taking it out of the ledger.
func TestFindingsReclassifyKeepsTheOriginal(t *testing.T) {
	dir := makeDevProject(t)
	setOutputFlag(t, "")
	writeFindings(t, dir)

	if err := runWith(t, findingsReclassifyCmd, map[string]string{"severity": "P1"}, "f-001"); err == nil {
		t.Fatal("reclassified without provenance")
	}
	if err := runWith(t, findingsReclassifyCmd, map[string]string{"reason": "why", "by": "human"}, "f-001"); err == nil {
		t.Fatal("reclassified with nothing to change")
	}
	if err := runWith(t, findingsReclassifyCmd, map[string]string{"severity": "P1", "reason": "secret reflection is P1 by rule 23", "by": "human"}, "f-001"); err != nil {
		t.Fatal(err)
	}
	if err := runWith(t, findingsReclassifyCmd, map[string]string{"exclude": "true", "reason": "aggregator of f-001's branches", "by": "human"}, "f-002"); err != nil {
		t.Fatal(err)
	}
	store := loadStore(t, dir)
	f := store.FindByID("f-001")
	if f.Severity != "P1" || f.Original == nil || f.Original.Severity != "P2" || len(f.Corrections) != 1 {
		t.Errorf("corrected finding = %+v", f)
	}
	if !store.FindByID("f-002").Excluded {
		t.Error("exclusion not recorded")
	}

	setOutputFlag(t, "json")
	var stats findings.Stats
	resetFlags(t, findingsCmd.Flags())
	if err := findingsCmd.Flags().Set("stats", "true"); err != nil {
		t.Fatal(err)
	}
	captureJSON(t, findingsCmd, nil, &stats)
	if stats.Total != 2 || stats.Counted != 1 || stats.Excluded != 1 || stats.BySeverity["P1"] != 1 || stats.BySeverity["P2"] != 0 {
		t.Errorf("stats = %+v, want total 2, counted 1, P1 1", stats)
	}
}

// A later-cycle finding that continues an earlier one says so. The link
// is the reviewer's statement, so a parent that does not exist is refused
// and the block is not recorded on a bad link.
func TestBlockFollowOn(t *testing.T) {
	dir := makeDevProject(t)
	setOutputFlag(t, "")
	writeFindings(t, dir)
	replaceInFile(t, filepath.Join(dir, ".metis", "slices.yaml"), "coded: false", "coded: true")
	gitOut(t, dir, "commit", "-aqm", "chore(feat-0001): coded")

	rootCmd.SetArgs([]string{"block", "feat-0001", "--severity", "P2", "--category", "security", "--finding", "the fix stripped the query but not the fragment", "--follow-on", "f-001"})
	t.Cleanup(func() { rootCmd.SetArgs([]string{}) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("block: %v", err)
	}
	store := loadStore(t, dir)
	f := store.FindByID("f-003")
	if f == nil || f.FollowOn != "f-001" {
		t.Fatalf("follow-on not recorded: %+v", f)
	}
	if chain := store.Chain("f-003"); len(chain) != 1 || chain[0] != "f-001" {
		t.Errorf("chain = %v", chain)
	}
	if status := gitOut(t, dir, "status", "--short"); strings.TrimSpace(status) != "" {
		t.Errorf("tree dirty after block:\n%s", status)
	}

	resetFlags(t, blockCmd.Flags())
	rootCmd.SetArgs([]string{"block", "feat-0001", "--finding", "another", "--follow-on", "f-404"})
	if err := rootCmd.Execute(); err == nil {
		t.Fatal("a follow-on of a missing finding was accepted")
	}
	if loadStore(t, dir).FindByID("f-004") != nil {
		t.Error("the finding was recorded on a bad link")
	}
}

// A promoted rule is the invariant, not the instance. Rule text that
// repeats the finding, or carries a file:line, is refused; what lands
// names the check that enforces it and the finding it came from.
func TestRulePromoteRewritesTheFinding(t *testing.T) {
	dir := makeDevProject(t)
	setOutputFlag(t, "")
	writeFindings(t, dir)
	finding := loadStore(t, dir).FindByID("f-001").Finding

	for name, flags := range map[string]map[string]string{
		"no text":   {},
		"pasted":    {"text": finding},
		"file:line": {"text": "redactIssuer must mask the query at internal/oidc/issuer.go:88"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := runWith(t, rulePromoteCmd, flags, "f-001"); err == nil {
				t.Fatal("promoted a finding as pasted")
			}
			if loadStore(t, dir).FindByID("f-001").Status == "promoted" {
				t.Error("the finding was promoted despite the refusal")
			}
		})
	}

	err := runWith(t, rulePromoteCmd, map[string]string{
		"text":        "No content of a secret-bearing value reaches an error or a log, whichever branch refuses it",
		"enforced-by": "TestSecretsNeverReflected",
	}, "f-001")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(filepath.Join(dir, ".metis", "project.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.AccuracyRules) != 1 {
		t.Fatalf("rules = %v", cfg.AccuracyRules)
	}
	rule := cfg.AccuracyRules[0]
	if !strings.HasPrefix(rule, "No content of a secret-bearing value") || !strings.Contains(rule, "enforced by: TestSecretsNeverReflected") || !strings.Contains(rule, "from f-001") {
		t.Errorf("rule = %q", rule)
	}
	if strings.Contains(rule, "issuer.go") {
		t.Errorf("rule carries the instance: %q", rule)
	}
	f := loadStore(t, dir).FindByID("f-001")
	if f.Status != "promoted" || f.PromotedTo == nil || *f.PromotedTo != 1 {
		t.Errorf("finding after promotion = %+v", f)
	}
}
