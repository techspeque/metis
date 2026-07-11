package instructions

import (
	"strings"
	"testing"

	"github.com/techspeque/metis/internal/config"
	"github.com/techspeque/metis/internal/slice"
)

func testConfig() *config.Config {
	return &config.Config{
		Version: 1,
		Project: config.ProjectConfig{
			Name:              "test-project",
			IntegrationBranch: "dev",
			ReleaseBranch:     "main",
		},
		Agents: map[string]config.Agent{
			"opencode/opus": {Surface: "opencode", Model: "opus", Label: "opencode (Opus)"},
		},
		Routing: config.RoutingConfig{
			High:   []string{"opencode/opus"},
			Review: "cross-vendor",
		},
		HotPaths:        []string{"src/auth/"},
		AccuracyRules:   []string{"Do not hallucinate interfaces"},
		NonGoals:        []string{"Frontend"},
		Testing:         []string{"Mock at trust boundaries only"},
		ReviewChecklist: []string{"Behavioral correctness"},
		Commands:        config.CommandsConfig{Verify: "go test ./..."},
		Commits: config.CommitsConfig{
			Prefixes: []string{"feat", "fix"},
			Format:   "{prefix}({slice_id}): {message}",
		},
	}
}

func TestGenerate_ContainsAllSections(t *testing.T) {
	cfg := testConfig()
	out := Generate(cfg, "")

	sections := []string{
		"Agent Contract",
		"Session Start Protocol",
		"Branch & Commit Rules",
		"Definition of Done",
		"Roles",
		"Hot-Path Zones",
		"Scope Discipline",
		"Model Routing",
		"Testing Rules",
		"Non-Goals",
		"Accuracy Rules",
		"Review Checklist",
		"Feedback Loop",
		"Tooling Map",
	}
	for _, s := range sections {
		if !strings.Contains(out, s) {
			t.Errorf("Generate() missing section %q", s)
		}
	}
}

func TestGenerate_IncludesConfigValues(t *testing.T) {
	cfg := testConfig()
	out := Generate(cfg, "")

	if !strings.Contains(out, "test-project") {
		t.Error("missing project name")
	}
	if !strings.Contains(out, "dev") {
		t.Error("missing integration branch")
	}
	if !strings.Contains(out, "src/auth/") {
		t.Error("missing hot path")
	}
	if !strings.Contains(out, "Do not hallucinate") {
		t.Error("missing accuracy rule")
	}
	if !strings.Contains(out, "Frontend") {
		t.Error("missing non-goal")
	}
}

func TestGenerateForSlice_LowRisk(t *testing.T) {
	cfg := testConfig()
	s := &slice.Slice{Risk: slice.RiskLow}
	out := GenerateForSlice(cfg, s, "")

	// Low risk should NOT include hot paths, accuracy rules, review checklist, routing, feedback
	if strings.Contains(out, "Hot-Path Zones") {
		t.Error("low risk should not include hot paths")
	}
	if strings.Contains(out, "Accuracy Rules") {
		t.Error("low risk should not include accuracy rules")
	}
	if strings.Contains(out, "Model Routing") {
		t.Error("low risk should not include routing")
	}
	if strings.Contains(out, "Feedback Loop") {
		t.Error("low risk should not include feedback loop")
	}

	// Should still include core sections
	if !strings.Contains(out, "Definition of Done") {
		t.Error("low risk should include DoD")
	}
	if !strings.Contains(out, "Tooling Map") {
		t.Error("low risk should include tooling map")
	}
}

func TestGenerateForSlice_MediumRisk(t *testing.T) {
	cfg := testConfig()
	s := &slice.Slice{Risk: slice.RiskMedium}
	out := GenerateForSlice(cfg, s, "")

	// Medium should include hot paths and accuracy rules
	if !strings.Contains(out, "Hot-Path Zones") {
		t.Error("medium risk should include hot paths")
	}
	if !strings.Contains(out, "Accuracy Rules") {
		t.Error("medium risk should include accuracy rules")
	}
	// But not routing or feedback
	if strings.Contains(out, "Model Routing") {
		t.Error("medium risk should not include routing")
	}
	if strings.Contains(out, "Feedback Loop") {
		t.Error("medium risk should not include feedback loop")
	}
}

func TestGenerateForSlice_HighRisk(t *testing.T) {
	cfg := testConfig()
	s := &slice.Slice{Risk: slice.RiskHigh}
	out := GenerateForSlice(cfg, s, "")

	// High risk includes everything
	sections := []string{
		"Hot-Path Zones",
		"Accuracy Rules",
		"Model Routing",
		"Feedback Loop",
	}
	for _, sec := range sections {
		if !strings.Contains(out, sec) {
			t.Errorf("high risk should include %q", sec)
		}
	}
}

func TestGenerateKickoff(t *testing.T) {
	cfg := testConfig()
	out := GenerateKickoff(cfg, "")

	checks := []string{
		"Metis Session Protocol",
		"Step 1: Establish State",
		"Step 2: Find Active Slice",
		"Step 3: Self-Identify",
		"Step 4: Read Instructions",
		"Step 5: Pre-flight Verification",
		"Step 6a: Coder Flow",
		"Step 6b: Reviewer Flow",
		"must be 'dev'",
	}
	for _, c := range checks {
		if !strings.Contains(out, c) {
			t.Errorf("kickoff missing %q", c)
		}
	}
}

func TestGenerateKickoff_CoderOnly(t *testing.T) {
	cfg := testConfig()
	out := GenerateKickoff(cfg, "coder")

	if !strings.Contains(out, "Step 6a: Coder Flow") {
		t.Error("coder kickoff missing coder flow")
	}
	if strings.Contains(out, "Step 6b: Reviewer Flow") {
		t.Error("coder kickoff should not include reviewer flow")
	}
}

func TestGenerateKickoff_ReviewerOnly(t *testing.T) {
	cfg := testConfig()
	out := GenerateKickoff(cfg, "reviewer")

	if strings.Contains(out, "Step 6a: Coder Flow") {
		t.Error("reviewer kickoff should not include coder flow")
	}
	if !strings.Contains(out, "Step 6b: Reviewer Flow") {
		t.Error("reviewer kickoff missing reviewer flow")
	}
}

func TestGenerateKickoff_DirtyTreeResume(t *testing.T) {
	cfg := testConfig()
	out := GenerateKickoff(cfg, "")

	if !strings.Contains(out, "Dirty tree (uncommitted changes)") {
		t.Error("kickoff missing dirty tree resume section")
	}
	if !strings.Contains(out, "Files are in scope") {
		t.Error("kickoff missing in-scope resume guidance")
	}
}
