package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLookup(t *testing.T) {
	cfg := &Config{
		Version: 1,
		Project: ProjectConfig{Name: "demo"},
		Agents: map[string]Agent{
			"claude-code/opus": {Surface: "claude-code", Model: "opus", Label: "Claude"},
		},
		Commits: CommitsConfig{RequireSliceID: true, Prefixes: []string{"feat", "fix"}},
	}

	tests := []struct {
		path string
		want any
	}{
		{"version", 1},
		{"project.name", "demo"},
		{"agents.claude-code/opus.model", "opus"},
		{"commits.require_slice_id", true},
	}
	for _, tt := range tests {
		got, err := Lookup(cfg, tt.path)
		if err != nil {
			t.Errorf("Lookup(%q): %v", tt.path, err)
			continue
		}
		if got != tt.want {
			t.Errorf("Lookup(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}

	if v, err := Lookup(cfg, "commits.prefixes"); err != nil {
		t.Errorf("Lookup(commits.prefixes): %v", err)
	} else if s, ok := v.([]string); !ok || len(s) != 2 {
		t.Errorf("Lookup(commits.prefixes) = %#v", v)
	}
}

func TestLookupErrors(t *testing.T) {
	cfg := &Config{}

	if _, err := Lookup(cfg, "nope"); err == nil || !strings.Contains(err.Error(), "valid:") {
		t.Errorf("unknown top-level key should list valid keys, got: %v", err)
	}
	if _, err := Lookup(cfg, "project.nope"); err == nil {
		t.Error("unknown nested key: want error")
	}
	if _, err := Lookup(cfg, "agents.ghost.model"); err == nil {
		t.Error("missing map entry: want error")
	}
	if _, err := Lookup(cfg, "version.deeper"); err == nil {
		t.Error("descending into a scalar: want error")
	}
}

func TestPathType(t *testing.T) {
	for path, kind := range map[string]string{
		"project.name":             "string",
		"commits.require_slice_id": "bool",
		"version":                  "int",
		"routing.high":             "slice",
		"agents.any-slug.model":    "string", // map entry need not exist
	} {
		typ, err := PathType(path)
		if err != nil {
			t.Errorf("PathType(%q): %v", path, err)
			continue
		}
		if !strings.Contains(strings.ToLower(typ.Kind().String()), kind) {
			t.Errorf("PathType(%q) = %s, want %s", path, typ.Kind(), kind)
		}
	}

	if _, err := PathType("bogus.key"); err == nil {
		t.Error("PathType on unknown key: want error")
	}
}

const commentedConfig = `# metis.yaml — project configuration for Metis
version: 1
project:
  name: demo # the project name
  # branches follow gitflow
  integration_branch: dev
  release_branch: main
commands:
  verify: go test ./... # keep fast
`

func writeTempConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "metis.yaml")
	if err := os.WriteFile(path, []byte(commentedConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSetInFilePreservesComments(t *testing.T) {
	path := writeTempConfig(t)

	if err := SetInFile(path, "project.integration_branch", "develop"); err != nil {
		t.Fatalf("SetInFile: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out := string(data)

	for _, comment := range []string{
		"# metis.yaml — project configuration for Metis",
		"# the project name",
		"# branches follow gitflow",
		"# keep fast",
	} {
		if !strings.Contains(out, comment) {
			t.Errorf("comment %q lost after edit:\n%s", comment, out)
		}
	}
	if !strings.Contains(out, "integration_branch: develop") {
		t.Errorf("value not updated:\n%s", out)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("result does not load: %v", err)
	}
	if cfg.Project.IntegrationBranch != "develop" {
		t.Errorf("IntegrationBranch = %q", cfg.Project.IntegrationBranch)
	}
}

func TestSetInFileTypes(t *testing.T) {
	path := writeTempConfig(t)

	if err := SetInFile(path, "commits.require_slice_id", "false"); err != nil {
		t.Fatalf("bool set: %v", err)
	}
	if err := SetInFile(path, "routing.high", "a/one, b/two"); err != nil {
		t.Fatalf("list set: %v", err)
	}
	if err := SetInFile(path, "agents.new-agent.model", "opus"); err != nil {
		t.Fatalf("map-entry set with intermediate creation: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Commits.RequireSliceID {
		t.Error("require_slice_id should be false")
	}
	if len(cfg.Routing.High) != 2 || cfg.Routing.High[1] != "b/two" {
		t.Errorf("routing.high = %v", cfg.Routing.High)
	}
	if cfg.Agents["new-agent"].Model != "opus" {
		t.Errorf("agents.new-agent.model = %q", cfg.Agents["new-agent"].Model)
	}
}

func TestSetInFileRejectsBadInput(t *testing.T) {
	path := writeTempConfig(t)

	if err := SetInFile(path, "commits.require_slice_id", "maybe"); err == nil {
		t.Error("non-boolean for bool key: want error")
	}
	if err := SetInFile(path, "version", "one"); err == nil {
		t.Error("non-integer for int key: want error")
	}
	if err := SetInFile(path, "unknown.key", "x"); err == nil {
		t.Error("unknown key: want error")
	}
	if err := SetInFile(path, "agents", "x"); err == nil {
		t.Error("setting a whole map: want error")
	}
	if err := SetInFile(path, "project", "x"); err == nil {
		t.Error("setting a whole struct: want error")
	}

	// None of the rejected writes may have altered the file.
	data, _ := os.ReadFile(path)
	if string(data) != commentedConfig {
		t.Errorf("file changed by a rejected set:\n%s", data)
	}
}
