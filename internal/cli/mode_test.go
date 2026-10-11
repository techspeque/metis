package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/techspeque/metis/internal/config"
	"github.com/techspeque/metis/internal/slice"
)

// Slice beats phase beats risk beats project beats the default; a run's
// scoped override beats the persistent setting at its level, the unscoped
// one replaces only the project level.
func TestModePrecedence(t *testing.T) {
	dir := makeProject(t)
	plan := "plans/phase-9.md"
	if err := os.MkdirAll(filepath.Join(dir, "plans"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, plan), []byte("# Phase 9\n\n> Auto: manual\n\n## Workstream 9.1: X\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := &context{cfg: &config.Config{}, repoRoot: dir}
	s := &slice.Slice{ID: "phase-9-ws-9.1", Plan: plan, Risk: slice.RiskHigh}
	resolve := func(ov modeOverrides) modeChoice { return newModeResolver(ctx, ov).of(s) }

	if got := resolve(modeOverrides{}); got != (modeChoice{"manual", modeFromPhase}) {
		t.Errorf("plan header: %+v", got)
	}
	ctx.cfg.Auto.Phases = map[string]string{"phase-9": "step"}
	if got := resolve(modeOverrides{}); got != (modeChoice{"step", modeFromPhase}) {
		t.Errorf("auto.phases beats the plan header: %+v", got)
	}
	ctx.cfg.Auto.Mode = "auto"
	ctx.cfg.Auto.ByRisk = map[string]string{"high": "manual"}
	if got := resolve(modeOverrides{project: "auto"}); got.Mode != "step" {
		t.Errorf("an unscoped override never beats a phase: %+v", got)
	}
	if got := resolve(modeOverrides{phase: map[string]string{"phase-9": "auto"}}); got != (modeChoice{"auto", modeFromPhase}) {
		t.Errorf("a scoped phase override: %+v", got)
	}
	s.AutoMode = "auto"
	if got := resolve(modeOverrides{}); got != (modeChoice{"auto", modeFromSlice}) {
		t.Errorf("slice beats phase: %+v", got)
	}
	if got := resolve(modeOverrides{slice: map[string]string{s.ID: "manual"}}); got.Mode != "manual" {
		t.Errorf("a scoped slice override beats the slice's own: %+v", got)
	}

	s.AutoMode, s.Plan = "", ""
	ctx.cfg.Auto.Phases = nil
	if got := resolve(modeOverrides{}); got != (modeChoice{"manual", modeFromRisk}) {
		t.Errorf("risk: %+v", got)
	}
	ctx.cfg.Auto.ByRisk = nil
	if got := resolve(modeOverrides{}); got != (modeChoice{"auto", modeFromProject}) {
		t.Errorf("project: %+v", got)
	}
	ctx.cfg.Auto.Mode = ""
	if got := resolve(modeOverrides{}); got != (modeChoice{"step", modeFromDefault}) {
		t.Errorf("default: %+v", got)
	}
}

func runMode(t *testing.T, args ...string) error {
	t.Helper()
	resetFlags(t, modeCmd.Flags())
	setOutputFlag(t, "")
	rootCmd.SetArgs(append([]string{"mode"}, args...))
	t.Cleanup(func() { rootCmd.SetArgs([]string{}) })
	return rootCmd.Execute()
}

// metis mode stores a slice's mode in the ledger and a phase's in
// project.yaml; --clear restores the level above.
func TestModeCommand(t *testing.T) {
	dir := makeGitProjectWithLedger(t)

	if err := runMode(t, "feat-0001", "manual"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ledgerText(t, dir), "auto_mode: manual") {
		t.Errorf("ledger:\n%s", ledgerText(t, dir))
	}
	if err := runMode(t, "feat-0001", "--clear"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(ledgerText(t, dir), "auto_mode") {
		t.Errorf("--clear left the mode:\n%s", ledgerText(t, dir))
	}

	if err := runMode(t, "--phase", "phase-3", "auto"); err != nil {
		t.Fatal(err)
	}
	cfgText, _ := os.ReadFile(filepath.Join(dir, ".metis", "project.yaml"))
	if !strings.Contains(string(cfgText), "phase-3: auto") {
		t.Errorf("project.yaml:\n%s", cfgText)
	}
	if err := runMode(t, "--phase", "phase-3", "--clear"); err != nil {
		t.Fatal(err)
	}
	cfgText, _ = os.ReadFile(filepath.Join(dir, ".metis", "project.yaml"))
	if strings.Contains(string(cfgText), "phase-3") {
		t.Errorf("--phase --clear left the key:\n%s", cfgText)
	}
	if status := gitOut(t, dir, "status", "--short"); strings.TrimSpace(status) != "" {
		t.Errorf("mode changes are committed; tree:\n%s", status)
	}
	if err := runMode(t, "feat-0001", "sometimes"); err == nil {
		t.Error("an unknown mode is refused")
	}
}

func TestInitWritesTheChosenMode(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	setUserConfig(t, "", map[string]string{})
	resetFlags(t, initCmd.Flags())
	rootCmd.SetArgs([]string{"init", "--auto-mode", "auto"})
	t.Cleanup(func() { rootCmd.SetArgs([]string{}) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(filepath.Join(dir, ".metis", "project.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Auto.Mode != "auto" {
		t.Errorf("auto.mode = %q", cfg.Auto.Mode)
	}
	if cfg.Agents["human"].Surface != config.SurfaceHuman {
		t.Errorf("the human agent is added: %+v", cfg.Agents)
	}
	text, _ := os.ReadFile(filepath.Join(dir, ".metis", "project.yaml"))
	if !strings.Contains(string(text), "# phases:") || !strings.Contains(string(text), "# by_risk:") {
		t.Errorf("the examples are commented in:\n%s", text)
	}

	// Re-init on an existing project fills in what is missing only.
	if err := config.SetInFile(filepath.Join(dir, ".metis", "project.yaml"), "agents.claude-code/opus.surface", "claude-code"); err != nil {
		t.Fatal(err)
	}
	resetFlags(t, initCmd.Flags())
	rootCmd.SetArgs([]string{"init"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatal(err)
	}
	cfg, _ = config.Load(filepath.Join(dir, ".metis", "project.yaml"))
	if got := cfg.Agents["claude-code/opus"].Launch; len(got) == 0 || got[0] != "claude" {
		t.Errorf("init writes the launcher of a known surface: %q", got)
	}
	if cfg.Auto.Mode != "auto" {
		t.Errorf("re-init keeps the mode: %q", cfg.Auto.Mode)
	}
}
