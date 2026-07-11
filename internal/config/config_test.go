package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParse_FullConfig(t *testing.T) {
	data := []byte(`
version: 1
project:
  name: test-project
  integration_branch: dev
  release_branch: main
  technology:
    language: go
    build_system: go
    test_runner: "go test ./..."
    linter: "go vet"
agents:
  claude-code/opus:
    surface: claude-code
    model: opus
    label: "Claude Code (Opus)"
  codex:
    surface: codex
    model: codex
    label: "Codex"
routing:
  high: [claude-code/opus]
  medium: [codex]
  low: [codex]
  review: cross-vendor
hot_paths:
  - src/auth/
accuracy_rules:
  - Do not hallucinate interfaces
non_goals:
  - Frontend
testing:
  - Mock at trust boundaries only
review_checklist:
  - Behavioral correctness
commands:
  verify: "go test ./..."
  env_check: "go version"
commits:
  prefixes: [feat, fix, refactor, docs, test, chore]
  require_slice_id: true
  no_attribution: true
  format: "{prefix}({slice_id}): {message}"
paths:
  ledger: .metis/slices.yaml
  archive: .metis/slices-done.yaml
  briefs: .metis/briefs/
  findings: .metis/findings.yaml
  runs: .metis/runs/
  interfaces: .metis/interfaces.txt
`)

	cfg, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if cfg.Project.Name != "test-project" {
		t.Errorf("Project.Name = %q, want %q", cfg.Project.Name, "test-project")
	}
	if cfg.Project.IntegrationBranch != "dev" {
		t.Errorf("IntegrationBranch = %q, want %q", cfg.Project.IntegrationBranch, "dev")
	}
	if len(cfg.Agents) != 2 {
		t.Errorf("len(Agents) = %d, want 2", len(cfg.Agents))
	}
	if cfg.Agents["claude-code/opus"].Label != "Claude Code (Opus)" {
		t.Errorf("agent label wrong: %q", cfg.Agents["claude-code/opus"].Label)
	}
	if cfg.Commands.Verify != "go test ./..." {
		t.Errorf("Commands.Verify = %q", cfg.Commands.Verify)
	}
	if len(cfg.HotPaths) != 1 || cfg.HotPaths[0] != "src/auth/" {
		t.Errorf("HotPaths = %v", cfg.HotPaths)
	}
}

func TestParse_MinimalConfig(t *testing.T) {
	data := []byte(`
version: 1
project:
  name: minimal
agents:
  claude-code/opus:
    surface: claude-code
    model: opus
    label: "Claude Code (Opus)"
commands:
  verify: "echo ok"
`)

	cfg, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	// Defaults should be applied
	if cfg.Project.IntegrationBranch != "dev" {
		t.Errorf("default IntegrationBranch = %q, want %q", cfg.Project.IntegrationBranch, "dev")
	}
	if cfg.Project.ReleaseBranch != "main" {
		t.Errorf("default ReleaseBranch = %q, want %q", cfg.Project.ReleaseBranch, "main")
	}
	if cfg.Paths.Ledger != ".metis/slices.yaml" {
		t.Errorf("default Paths.Ledger = %q", cfg.Paths.Ledger)
	}
	if !cfg.Commits.RequireSliceID {
		t.Error("default Commits.RequireSliceID should be true")
	}
	if !cfg.Commits.NoAttribution {
		t.Error("default Commits.NoAttribution should be true")
	}
	if cfg.Commits.Format != "{prefix}({slice_id}): {message}" {
		t.Errorf("default Commits.Format = %q", cfg.Commits.Format)
	}
}

func TestParse_InvalidYAML(t *testing.T) {
	data := []byte(`{invalid yaml: [}`)
	_, err := Parse(data)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	cfg := &Config{
		Version: 1,
		Project: ProjectConfig{Name: "test"},
		Agents: map[string]Agent{
			"opencode/opus": {Surface: "opencode", Model: "opus", Label: "opencode (Opus)"},
		},
		Routing: RoutingConfig{
			High:   []string{"opencode/opus"},
			Review: "cross-vendor",
		},
		Commands: CommandsConfig{Verify: "go test ./..."},
		Commits: CommitsConfig{
			Prefixes: []string{"feat", "fix"},
			Format:   "{prefix}({slice_id}): {message}",
		},
		Paths: PathsConfig{Ledger: ".metis/slices.yaml"},
	}

	errs := cfg.Validate()
	if len(errs) != 0 {
		t.Errorf("valid config has errors: %v", errs)
	}
}

func TestValidate_MissingProjectName(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Agents["test"] = Agent{Surface: "s", Model: "m", Label: "l"}
	cfg.Commands.Verify = "echo ok"

	errs := cfg.Validate()
	found := false
	for _, e := range errs {
		if e.Error() == "project.name is required" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'project.name is required' error")
	}
}

func TestValidate_MissingAgents(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Project.Name = "test"
	cfg.Commands.Verify = "echo ok"

	errs := cfg.Validate()
	found := false
	for _, e := range errs {
		if e.Error() == "at least one agent must be defined in agents" {
			found = true
		}
	}
	if !found {
		t.Error("expected agents required error")
	}
}

func TestValidate_InvalidRoutingSlug(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Project.Name = "test"
	cfg.Agents["real-agent"] = Agent{Surface: "s", Model: "m", Label: "l"}
	cfg.Routing.High = []string{"nonexistent-agent"}
	cfg.Commands.Verify = "echo ok"

	errs := cfg.Validate()
	found := false
	for _, e := range errs {
		if e != nil {
			found = true
		}
	}
	if !found {
		t.Error("expected routing slug error")
	}
}

func TestValidate_MissingVerifyCommand(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Project.Name = "test"
	cfg.Agents["agent"] = Agent{Surface: "s", Model: "m", Label: "l"}
	cfg.Commands.Verify = ""

	errs := cfg.Validate()
	found := false
	for _, e := range errs {
		if e.Error() == "commands.verify is required" {
			found = true
		}
	}
	if !found {
		t.Error("expected verify command required error")
	}
}

func TestLoad_FromFile(t *testing.T) {
	// Use the testdata fixture
	path := filepath.Join("..", "..", "testdata", "metis.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("testdata/metis.yaml not found")
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Project.Name != "sample-project" {
		t.Errorf("Project.Name = %q, want %q", cfg.Project.Name, "sample-project")
	}
	if len(cfg.Agents) != 3 {
		t.Errorf("len(Agents) = %d, want 3", len(cfg.Agents))
	}

	// Should pass validation
	errs := cfg.Validate()
	if len(errs) != 0 {
		t.Errorf("testdata config has validation errors: %v", errs)
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/metis.yaml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestFindConfig(t *testing.T) {
	// Create a temp directory structure
	tmp := t.TempDir()
	nested := filepath.Join(tmp, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write metis.yaml at root
	cfgPath := filepath.Join(tmp, "metis.yaml")
	if err := os.WriteFile(cfgPath, []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Should find it from nested dir
	found, err := FindConfig(nested)
	if err != nil {
		t.Fatalf("FindConfig() error: %v", err)
	}
	if found != cfgPath {
		t.Errorf("FindConfig() = %q, want %q", found, cfgPath)
	}
}

func TestFindConfig_NotFound(t *testing.T) {
	tmp := t.TempDir()
	_, err := FindConfig(tmp)
	if err == nil {
		t.Error("expected error when metis.yaml is not found")
	}
}
