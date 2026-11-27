package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
		if strings.Contains(e.Error(), "at least one agent must be defined") {
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

func TestValidate_VerifyScopes(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Project.Name = "test"
	cfg.Agents["agent"] = Agent{Surface: "s", Model: "m", Label: "l"}
	cfg.Commands.VerifyScopes = []VerifyScope{
		{Name: "api", Paths: []string{"api/"}, Command: "make api"},
		{Name: "api", Paths: nil, Command: ""},
		{Name: "", Paths: []string{"web/"}, Command: "make web"},
	}
	var got []string
	for _, e := range cfg.Validate() {
		got = append(got, e.Error())
	}
	for _, want := range []string{
		`commands.verify_scopes: scope "api" is declared twice`,
		"commands.verify_scopes[1]: paths are required",
		"commands.verify_scopes[1]: command is required",
		"commands.verify_scopes[2]: name is required",
	} {
		found := false
		for _, g := range got {
			if g == want {
				found = true
			}
		}
		if !found {
			t.Errorf("missing %q in %v", want, got)
		}
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

// Metis writes files outside .metis/ — the project overview and the surface
// adapters — as a side effect of seeding plans and regenerating the surface.
// The scope audit exempts what ManagedPaths reports, so anything missing here
// is reported to a reviewer as a scope violation no brief can fix.
func TestManagedPaths(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Project.Overview = "OVERVIEW.md"

	for _, f := range []string{
		"OVERVIEW.md",
		"AGENTS.md",
		"CLAUDE.md",
		"opencode.json",
		".claude/settings.json",
		"metis.yaml",
		".metis/slices.yaml",
		".metis/briefs/feat-0001.md",
		".metis/runs/feat-0001/verify-post.log",
	} {
		if !cfg.IsManagedPath(f) {
			t.Errorf("IsManagedPath(%q) = false, want true", f)
		}
	}

	// Project source must still be measured against the brief.
	for _, f := range []string{
		"README.md",
		"src/main.go",
		"docs/OVERVIEW.md",
		".claude/agents/reviewer.md",
		"CLAUDE.md.bak",
		"opencode.json5",
	} {
		if cfg.IsManagedPath(f) {
			t.Errorf("IsManagedPath(%q) = true, want false", f)
		}
	}
}

// A relocated path stays managed: the exemption follows config, it is not a
// hardcoded .metis/ prefix.
func TestManagedPathsFollowsRelocatedConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Paths.Plans = "docs/plans/"
	cfg.Project.Overview = "docs/spec/OVERVIEW.md"

	for _, f := range []string{"docs/plans/phase-1.md", "docs/spec/OVERVIEW.md"} {
		if !cfg.IsManagedPath(f) {
			t.Errorf("IsManagedPath(%q) = false for a relocated managed path", f)
		}
	}
	if cfg.IsManagedPath("docs/other.md") {
		t.Error("relocating plans must not exempt the whole docs/ tree")
	}
}

// An unset overview must not exempt the repo root.
func TestManagedPathsEmptyOverview(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Project.Overview != "" {
		t.Fatalf("fixture assumption broken: overview = %q", cfg.Project.Overview)
	}
	if cfg.IsManagedPath("anything.md") {
		t.Error("empty overview exempted an unrelated file")
	}
}

func TestValidate_RateLimit(t *testing.T) {
	cfg, err := Parse([]byte(`version: 1
project: { name: test }
agents:
  codex/gpt:
    surface: codex
    model: gpt
    label: Codex
    rate_limit:
      exit_codes: [75]
      patterns: ["usage limit", "rate limit("]
      reset_pattern: "resets? (?:at|in) (.+"
commands: { verify: "true" }
auto:
  rate_limit: { max_wait: soon, backoff: -5m }
`))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range cfg.Validate() {
		got = append(got, e.Error())
	}
	all := strings.Join(got, "\n")
	for _, want := range []string{
		`agent "codex/gpt": rate_limit pattern "rate limit(" does not compile`,
		`agent "codex/gpt": rate_limit pattern "resets? (?:at|in) (.+" does not compile`,
		`auto.rate_limit.max_wait "soon" is not a positive duration`,
		`auto.rate_limit.backoff "-5m" is not a positive duration`,
	} {
		if !strings.Contains(all, want) {
			t.Errorf("missing %q in:\n%s", want, all)
		}
	}
	if len(got) != 4 {
		t.Errorf("errors:\n%s", all)
	}
	if rl := cfg.Agents["codex/gpt"].RateLimit; rl == nil || len(rl.ExitCodes) != 1 || rl.ExitCodes[0] != 75 {
		t.Errorf("rate_limit = %+v", rl)
	}
}

func TestValidate_RemoteAgent(t *testing.T) {
	cfg, err := Parse([]byte(`version: 1
project: { name: test }
agents:
  claude-code/opus:
    surface: claude-code
    model: opus
    label: Claude
    remote: true
    transient_exit_codes: [91, 95]
    launch: [gaia, run, --node, studio, --wait, --, claude, -p, "{prompt}"]
  codex/gpt:
    surface: codex
    model: gpt
    label: Codex
    transient_exit_codes: [0, 256]
  human:
    surface: human
    model: human
    label: Human
    remote: true
commands: { verify: "true" }
`))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range cfg.Validate() {
		got = append(got, e.Error())
	}
	all := strings.Join(got, "\n")
	for _, want := range []string{
		`agent "codex/gpt": transient_exit_codes: 0 is not a failing exit code (1 to 255)`,
		`agent "codex/gpt": transient_exit_codes: 256 is not a failing exit code (1 to 255)`,
		`agent "human": a human agent is not launched: remote and transient_exit_codes do not apply`,
	} {
		if !strings.Contains(all, want) {
			t.Errorf("missing %q in:\n%s", want, all)
		}
	}
	if len(got) != 3 {
		t.Errorf("errors:\n%s", all)
	}
	a := cfg.Agents["claude-code/opus"]
	if !a.Remote || !a.Transient(91) || !a.Transient(95) || a.Transient(93) || a.Transient(-1) {
		t.Errorf("agent = %+v", a)
	}
	if b := cfg.Agents["codex/gpt"]; b.Remote {
		t.Error("remote is off unless set")
	}
	if _, err := Parse([]byte("version: 1\nagents:\n  a/b: { surface: codex, model: m, label: L, remote: sometimes }\n")); err == nil {
		t.Error("remote: sometimes should not parse")
	}
}

func TestAutoRateLimitDefaults(t *testing.T) {
	var r AutoRateLimit
	if r.MaxWaitTime() != 6*time.Hour {
		t.Errorf("max_wait = %s", r.MaxWaitTime())
	}
	for waits, want := range []time.Duration{15 * time.Minute, 30 * time.Minute, time.Hour, time.Hour, time.Hour} {
		if got := r.BackoffAfter(waits); got != want {
			t.Errorf("backoff after %d = %s, want %s", waits, got, want)
		}
	}
	r = AutoRateLimit{MaxWait: "90m", Backoff: "2h"}
	if r.MaxWaitTime() != 90*time.Minute || r.BackoffAfter(0) != 2*time.Hour || r.BackoffAfter(3) != 2*time.Hour {
		t.Errorf("configured: %s, %s, %s", r.MaxWaitTime(), r.BackoffAfter(0), r.BackoffAfter(3))
	}
}
