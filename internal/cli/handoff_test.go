package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

func resetFlags(t *testing.T, flags *pflag.FlagSet) {
	t.Helper()
	reset := func() {
		flags.VisitAll(func(f *pflag.Flag) {
			_ = f.Value.Set(f.DefValue)
			f.Changed = false
		})
	}
	reset()
	t.Cleanup(reset)
}

func TestHandoffJSON(t *testing.T) {
	makeGitProjectWithLedger(t)
	setOutputFlag(t, "json")
	resetFlags(t, handoffCmd.Flags())

	var out handoffResult
	captureJSON(t, handoffCmd, nil, &out)
	if out.Slice != "feat-0001" || out.Role != "Coder" || out.Template != "code" || out.AgentSlug != "claude-code/opus" {
		t.Errorf("handoff = %+v", out)
	}
	if !strings.Contains(out.Prompt, "metis brief feat-0001 --write") {
		t.Errorf("code prompt should ask for the brief:\n%s", out.Prompt)
	}

	_ = handoffCmd.Flags().Set("template", "review")
	captureJSON(t, handoffCmd, []string{"feat-0001"}, &out)
	if out.Role != "Reviewer" || out.AgentSlug != "opencode/opus" || out.Template != "review" {
		t.Errorf("forced review = %+v", out)
	}
}

func TestLaunchDryRunAndNextAutoFields(t *testing.T) {
	dir := makeGitProjectWithLedger(t)
	cfg := "version: 1\nagents:\n  claude-code/opus: {surface: claude-code, model: opus, label: O}\n  opencode/opus: {surface: opencode, model: opus, label: OC}\n"
	if err := os.WriteFile(filepath.Join(dir, ".metis", "project.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	setOutputFlag(t, "json")
	resetFlags(t, launchCmd.Flags())
	_ = launchCmd.Flags().Set("dry-run", "true")
	var v sessionView
	captureJSON(t, launchCmd, nil, &v)
	if v.AgentSlug != "claude-code/opus" || v.Model != "opus" || len(v.Argv) == 0 || v.Argv[0] != "claude" || v.Argv[2] != "{prompt}" {
		t.Errorf("dry run = %+v", v)
	}
	if !strings.HasSuffix(v.Log, filepath.Join("feat-0001", "auto", "1-coder.log")) {
		t.Errorf("log = %s", v.Log)
	}

	var next nextJSONOutput
	captureJSON(t, nextCmd, nil, &next)
	// The fixture slice has no plan section: auto would not run it.
	if next.Template != "code" || next.Auto != rulePlanning || next.AutoMode != "step" || next.AutoModeSource != "default" {
		t.Errorf("next = %+v", next)
	}
}
