package surface

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/techspeque/metis/internal/config"
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
		Routing:         config.RoutingConfig{High: []string{"opencode/opus"}, Review: "cross-vendor"},
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

func TestGenerate_CreatesAllFiles(t *testing.T) {
	tmp := t.TempDir()
	cfg := testConfig()

	if err := Generate(cfg, tmp); err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	files := []string{
		"CLAUDE.md",
		"AGENTS.md",
		"opencode.json",
		filepath.Join(".claude", "settings.json"),
		filepath.Join(".metis", "surface.hash"),
	}
	for _, f := range files {
		path := filepath.Join(tmp, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Generate() did not create %s", f)
		}
	}
}

func TestGenerate_CLAUDEContent(t *testing.T) {
	tmp := t.TempDir()
	cfg := testConfig()
	_ = Generate(cfg, tmp)

	data, _ := os.ReadFile(filepath.Join(tmp, "CLAUDE.md"))
	content := string(data)

	if !strings.Contains(content, "AGENTS.md") {
		t.Error("CLAUDE.md should point to AGENTS.md")
	}
	if !strings.Contains(content, "metis kickoff") {
		t.Error("CLAUDE.md should mention metis kickoff")
	}
}

func TestGenerate_AGENTSContent(t *testing.T) {
	tmp := t.TempDir()
	cfg := testConfig()
	_ = Generate(cfg, tmp)

	data, _ := os.ReadFile(filepath.Join(tmp, "AGENTS.md"))
	content := string(data)

	// Governance preamble
	if !strings.Contains(content, "Agent Contract — test-project") {
		t.Error("AGENTS.md missing governance header")
	}
	if !strings.Contains(content, "Hard Rules") {
		t.Error("AGENTS.md missing hard rules")
	}
	if !strings.Contains(content, "ONE slice at a time") {
		t.Error("AGENTS.md missing rule 1")
	}

	// Full contract
	if !strings.Contains(content, "Session Start Protocol") {
		t.Error("AGENTS.md missing full contract")
	}
	if !strings.Contains(content, "Tooling Map") {
		t.Error("AGENTS.md missing tooling map")
	}
}

func TestGenerate_OpencodeJSON(t *testing.T) {
	tmp := t.TempDir()
	cfg := testConfig()
	_ = Generate(cfg, tmp)

	data, _ := os.ReadFile(filepath.Join(tmp, "opencode.json"))
	content := string(data)

	if !strings.Contains(content, "AGENTS.md") {
		t.Error("opencode.json should reference AGENTS.md")
	}
	if !strings.Contains(content, "$schema") {
		t.Error("opencode.json missing schema")
	}
}

func TestValidate_AllPresent(t *testing.T) {
	tmp := t.TempDir()
	cfg := testConfig()
	_ = Generate(cfg, tmp)

	warnings := Validate(cfg, tmp)
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
}

func TestValidate_MissingFiles(t *testing.T) {
	tmp := t.TempDir()
	cfg := testConfig()

	warnings := Validate(cfg, tmp)
	if len(warnings) == 0 {
		t.Error("expected warnings for missing files")
	}
}

func TestValidate_Stale(t *testing.T) {
	tmp := t.TempDir()
	cfg := testConfig()
	_ = Generate(cfg, tmp)

	// Modify config to make hash stale
	cfg.Project.Name = "modified-project"
	warnings := Validate(cfg, tmp)

	found := false
	for _, w := range warnings {
		if strings.Contains(w, "stale") {
			found = true
		}
	}
	if !found {
		t.Error("expected staleness warning after config change")
	}
}
