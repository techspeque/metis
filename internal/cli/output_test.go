package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

// setOutputFlag sets the package-level --output flag value for the test.
func setOutputFlag(t *testing.T, value string) {
	t.Helper()
	prev := outputFlag
	outputFlag = value
	t.Cleanup(func() { outputFlag = prev })
}

// makeProjectWithLedger creates a project whose ledger contains one pending
// slice, moves cwd into it, and returns its path.
func makeProjectWithLedger(t *testing.T) string {
	t.Helper()
	dir := makeProject(t)
	if err := os.MkdirAll(filepath.Join(dir, ".metis"), 0o755); err != nil {
		t.Fatal(err)
	}
	ledger := `version: 1
slices:
  - id: feat-0001
    title: "Test slice"
    type: feat
    priority: p2
    risk: low
    coder: claude-code/opus
    reviewer: opencode/opus
    coded: false
    reviewed: false
    review_cycles: 0
    created: 2026-07-25
`
	if err := os.WriteFile(filepath.Join(dir, ".metis", "slices.yaml"), []byte(ledger), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	return dir
}

// captureJSON runs a command's RunE with its output captured and unmarshals
// the result into v.
func captureJSON(t *testing.T, cmd *cobra.Command, args []string, v any) {
	t.Helper()
	var out bytes.Buffer
	cmd.SetOut(&out)
	t.Cleanup(func() { cmd.SetOut(nil) })

	if err := cmd.RunE(cmd, args); err != nil {
		t.Fatalf("%s: %v", cmd.Name(), err)
	}
	if err := json.Unmarshal(out.Bytes(), v); err != nil {
		t.Fatalf("%s output is not valid JSON: %v\n%s", cmd.Name(), err, out.String())
	}
}

func TestOutputFormatResolution(t *testing.T) {
	setOutputFlag(t, "")
	t.Setenv(envOutput, "")
	if got := outputFormat(); got != "text" {
		t.Errorf("default format = %q, want text", got)
	}

	t.Setenv(envOutput, "json")
	if got := outputFormat(); got != "json" {
		t.Errorf("env format = %q, want json", got)
	}

	setOutputFlag(t, "text")
	if got := outputFormat(); got != "text" {
		t.Errorf("flag should beat env, got %q", got)
	}

	setOutputFlag(t, "xml")
	if err := validateOutputFormat(); err == nil {
		t.Error("validateOutputFormat should reject xml")
	}
	setOutputFlag(t, "json")
	if err := validateOutputFormat(); err != nil {
		t.Errorf("validateOutputFormat rejected json: %v", err)
	}
}

func TestStatusJSON(t *testing.T) {
	makeProjectWithLedger(t)
	setOutputFlag(t, "json")

	var out struct {
		State     string `json:"state"`
		ID        string `json:"id"`
		AgentSlug string `json:"agent_slug"`
		Total     int    `json:"total"`
	}
	captureJSON(t, statusCmd, nil, &out)

	if out.State != "active" || out.ID != "feat-0001" || out.Total != 1 {
		t.Errorf("status JSON = %+v", out)
	}
	if out.AgentSlug != "claude-code/opus" {
		t.Errorf("agent_slug = %q", out.AgentSlug)
	}
}

func TestListJSON(t *testing.T) {
	makeProjectWithLedger(t)
	setOutputFlag(t, "json")

	var rows []struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Title  string `json:"title"`
	}
	captureJSON(t, listCmd, nil, &rows)

	if len(rows) != 1 || rows[0].ID != "feat-0001" || rows[0].Status != "pending" {
		t.Errorf("list JSON = %+v", rows)
	}
}

func TestNextJSONViaGlobalFlag(t *testing.T) {
	makeProjectWithLedger(t)
	setOutputFlag(t, "json")

	var out struct {
		Active bool   `json:"active"`
		ID     string `json:"id"`
		Role   string `json:"role"`
	}
	captureJSON(t, nextCmd, nil, &out)

	if !out.Active || out.ID != "feat-0001" || out.Role != "Coder" {
		t.Errorf("next JSON = %+v", out)
	}
}

func TestVersionJSON(t *testing.T) {
	setOutputFlag(t, "json")

	var out map[string]string
	captureJSON(t, versionCmd, nil, &out)

	for _, key := range []string{"version", "commit", "date", "go_version", "os", "arch"} {
		if out[key] == "" {
			t.Errorf("version JSON missing %q: %v", key, out)
		}
	}
}

func TestWorkspaceListJSON(t *testing.T) {
	proj := makeProjectWithLedger(t)
	setUserConfig(t, "myws", map[string]string{"myws": proj, "gone": "/does/not/exist"})
	setOutputFlag(t, "json")

	var rows []struct {
		Name    string `json:"name"`
		Active  bool   `json:"active"`
		Missing bool   `json:"missing"`
	}
	captureJSON(t, workspaceListCmd, nil, &rows)

	if len(rows) != 2 {
		t.Fatalf("workspace list JSON = %+v", rows)
	}
	// Names() sorts, so "gone" comes first.
	if !rows[0].Missing || rows[0].Active {
		t.Errorf("gone row = %+v, want missing, not active", rows[0])
	}
	if rows[1].Name != "myws" || !rows[1].Active || rows[1].Missing {
		t.Errorf("myws row = %+v, want active, not missing", rows[1])
	}
}

func TestWorkspaceCurrentJSONNone(t *testing.T) {
	setUserConfig(t, "", map[string]string{})
	setOutputFlag(t, "json")

	var out bytes.Buffer
	workspaceCurrentCmd.SetOut(&out)
	t.Cleanup(func() { workspaceCurrentCmd.SetOut(nil) })

	if err := workspaceCurrentCmd.RunE(workspaceCurrentCmd, nil); err != nil {
		t.Fatal(err)
	}
	if got := string(bytes.TrimSpace(out.Bytes())); got != "null" {
		t.Errorf("current with no active workspace = %q, want null", got)
	}
}

func TestCheckJSONReportsFailure(t *testing.T) {
	// Minimal config (no agents) fails validation.
	makeProjectWithLedger(t)
	setOutputFlag(t, "json")

	var out bytes.Buffer
	checkCmd.SetOut(&out)
	t.Cleanup(func() { checkCmd.SetOut(nil) })

	err := checkCmd.RunE(checkCmd, nil)
	if err == nil {
		t.Fatal("check on invalid config: want non-nil error for exit code")
	}

	var result struct {
		OK     bool `json:"ok"`
		Config *struct {
			OK     bool     `json:"ok"`
			Errors []string `json:"errors"`
		} `json:"config"`
	}
	if jsonErr := json.Unmarshal(out.Bytes(), &result); jsonErr != nil {
		t.Fatalf("check output is not valid JSON: %v\n%s", jsonErr, out.String())
	}
	if result.OK || result.Config == nil || result.Config.OK || len(result.Config.Errors) == 0 {
		t.Errorf("check JSON = %+v, want config failure details", result)
	}
}
