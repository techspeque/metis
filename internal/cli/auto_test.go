package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

var (
	buildOnce sync.Once
	binDir    string
	buildErr  error
)

// metisBinary builds the metis binary once per test run: the fake agents
// call it like a real agent would.
func metisBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		_, file, _, _ := runtime.Caller(0)
		root := filepath.Join(filepath.Dir(file), "..", "..")
		binDir, buildErr = os.MkdirTemp("", "metis-bin-")
		if buildErr != nil {
			return
		}
		cmd := exec.Command("go", "build", "-o", filepath.Join(binDir, "metis"), "./cmd/metis")
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			buildErr = &buildError{err, string(out)}
		}
	})
	if buildErr != nil {
		t.Fatalf("building metis: %v", buildErr)
	}
	return binDir
}

type buildError struct {
	err error
	out string
}

func (e *buildError) Error() string { return e.err.Error() + "\n" + e.out }

func fakeAgentPath(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "testdata", "fake-agent.sh")
}

type autoSlice struct {
	id, plan, typ, reviewer, mode string
}

// makeAutoProject creates a git project on dev whose agents are fake
// agents playing the given actions, one per session.
func makeAutoProject(t *testing.T, autoYAML string, slices []autoSlice, actions ...string) string {
	t.Helper()
	bin := metisBinary(t)
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	agent := fakeAgentPath(t)
	cfg := `version: 1
project:
  name: autotest
  integration_branch: dev
agents:
  coder/a:
    surface: fake
    model: m-a
    label: Coder A
    launch: [sh, "` + agent + `", "{prompt_file}", coder/a]
  reviewer/b:
    surface: fake
    model: m-b
    label: Reviewer B
    launch: [sh, "` + agent + `", "{prompt_file}", reviewer/b]
  human:
    surface: human
    model: human
    label: Human
commands:
  verify: "true"
auto:
` + autoYAML + "\n"
	var ledger strings.Builder
	ledger.WriteString("version: 1\nslices:\n")
	for _, s := range slices {
		if s.plan == "" {
			s.plan = ".metis/plans/phase-1.md"
		}
		if s.typ == "" {
			s.typ = "feat"
		}
		if s.reviewer == "" {
			s.reviewer = "reviewer/b"
		}
		ledger.WriteString("  - id: " + s.id + "\n    title: " + s.id + "\n    type: " + s.typ +
			"\n    priority: p2\n    risk: low\n    coder: coder/a\n    reviewer: " + s.reviewer +
			"\n    plan: " + s.plan + "\n    plan_section: §1\n    coded: false\n    reviewed: false\n    review_cycles: 0\n    created: 2026-10-01\n")
		if s.mode != "" {
			ledger.WriteString("    auto_mode: " + s.mode + "\n")
		}
	}
	for path, content := range map[string]string{
		".metis/project.yaml": cfg,
		".metis/slices.yaml":  ledger.String(),
		".gitignore":          ".metis/runs/\n.metis/.lock\n",
	} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, path)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, path), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{
		{"init", "-q", "-b", "dev"},
		{"config", "user.email", "t@t.co"},
		{"config", "user.name", "T"},
		{"add", "-A"},
		{"commit", "-qm", "chore: fixture"},
	} {
		gitOut(t, dir, args...)
	}

	state := t.TempDir()
	script := filepath.Join(state, "script")
	if err := os.WriteFile(script, []byte(strings.Join(actions, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKE_SCRIPT", script)
	t.Setenv("FAKE_STATE", state)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Chdir(dir)
	return dir
}

// runAuto runs metis auto in-process and returns its output and exit code.
func runAuto(t *testing.T, args ...string) (string, int) {
	t.Helper()
	setOutputFlag(t, "")
	resetFlags(t, autoCmd.Flags())
	var out bytes.Buffer
	autoCmd.SetOut(&out)
	rootCmd.SetArgs(append([]string{"auto"}, args...))
	t.Cleanup(func() {
		rootCmd.SetArgs([]string{})
		autoCmd.SetOut(nil)
	})
	err := rootCmd.Execute()
	return out.String(), resolveExitCode(err)
}

func sessions(t *testing.T) int {
	t.Helper()
	data, _ := os.ReadFile(filepath.Join(os.Getenv("FAKE_STATE"), "count"))
	n := 0
	for _, c := range strings.TrimSpace(string(data)) {
		n = n*10 + int(c-'0')
	}
	return n
}

func archivedIDs(t *testing.T, dir string) string {
	t.Helper()
	data, _ := os.ReadFile(filepath.Join(dir, ".metis", "slices-done.yaml"))
	return string(data)
}

func expectStop(t *testing.T, out string, code, wantCode int, rule string) {
	t.Helper()
	if code != wantCode || !strings.Contains(out, "stopped ("+rule+")") {
		t.Fatalf("want stop %s (exit %d), got exit %d:\n%s", rule, wantCode, code, out)
	}
}

func TestAutoCleanPass(t *testing.T) {
	dir := makeAutoProject(t, "  mode: auto\n  branch: off", []autoSlice{{id: "s-1"}}, "code", "pass")
	out, code := runAuto(t)
	expectStop(t, out, code, 0, ruleBacklogEmpty)
	if !strings.Contains(archivedIDs(t, dir), "id: s-1") {
		t.Errorf("s-1 not archived")
	}
	if sessions(t) != 2 {
		t.Errorf("sessions = %d, want 2", sessions(t))
	}
	log, _ := os.ReadFile(filepath.Join(dir, ".metis", "runs", "auto.jsonl"))
	if strings.Count(string(log), `"event":"session"`) != 2 || !strings.Contains(string(log), `"agent":"reviewer/b","model":"m-b"`) {
		t.Errorf("run log:\n%s", log)
	}
	if status := gitOut(t, dir, "status", "--short"); strings.TrimSpace(status) != "" {
		t.Errorf("tree dirty after the run:\n%s", status)
	}
}

func TestAutoBlockThenPass(t *testing.T) {
	dir := makeAutoProject(t, "  mode: auto\n  branch: off", []autoSlice{{id: "s-1"}},
		"code", "block:src/s-1.txt:1 — no test for the empty case — add one", "code", "pass")
	out, code := runAuto(t)
	expectStop(t, out, code, 0, ruleBacklogEmpty)
	if a := archivedIDs(t, dir); !strings.Contains(a, "review_cycles: 1") {
		t.Errorf("archive:\n%s", a)
	}
	prompt, _ := os.ReadFile(filepath.Join(dir, ".metis", "runs", "s-1", "auto", "3-coder.prompt.md"))
	if !strings.Contains(string(prompt), "template rework") || !strings.Contains(string(prompt), "no test for the empty case") {
		t.Errorf("the third session should get the rework prompt:\n%s", prompt)
	}
}

func TestAutoStopsAtTheReviewCap(t *testing.T) {
	dir := makeAutoProject(t, "  mode: auto\n  branch: off", []autoSlice{{id: "s-1"}},
		"code", "block:alpha parser drops headers", "code", "block:beta timeout never fires", "code", "block:gamma docs disagree with flags", "code")
	out, code := runAuto(t)
	expectStop(t, out, code, 3, ruleMaxReviewCycles)
	if sessions(t) != 6 {
		t.Errorf("sessions = %d, want 6 (no rework after the third block)", sessions(t))
	}
	if l := ledgerText(t, dir); !strings.Contains(l, "reason: 'auto: max_review_cycles'") || !strings.Contains(l, "human: true") {
		t.Errorf("slice not parked:\n%s", l)
	}
	esc, err := os.ReadFile(filepath.Join(dir, ".metis", "runs", "s-1", "auto", "escalation.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"max_review_cycles", "metis wait s-1 --clear", "metis edit s-1 --max-review-cycles 4", "--agent human --slice s-1", "metis findings resolve f-003"} {
		if !strings.Contains(string(esc), want) {
			t.Errorf("escalation lacks %q:\n%s", want, esc)
		}
	}
}

func TestAutoStopsOnARepeatedFinding(t *testing.T) {
	makeAutoProject(t, "  mode: auto\n  branch: off", []autoSlice{{id: "s-1"}},
		"code", "block:src/s-1.txt:3 — the retry bound is missing — add it",
		"code", "block:re-raise f-001: src/s-1.txt:3 still has no retry bound")
	out, code := runAuto(t)
	expectStop(t, out, code, 3, ruleRepeatFinding)
	if !strings.Contains(out, "f-002 (review 2) re-raises f-001 (review 1)") {
		t.Errorf("detail:\n%s", out)
	}
}

func TestAutoStopsOnP1(t *testing.T) {
	makeAutoProject(t, "  mode: auto\n  branch: off", []autoSlice{{id: "s-1"}}, "code", "p1:src/s-1.txt:1 — token logged — drop it")
	out, code := runAuto(t)
	expectStop(t, out, code, 3, ruleP1)
}

func TestAutoStopsAtAGate(t *testing.T) {
	dir := makeAutoProject(t, "  mode: auto\n  branch: off", []autoSlice{{id: "phase-1-gate", typ: "gate"}})
	out, code := runAuto(t)
	expectStop(t, out, code, 3, ruleGate)
	if sessions(t) != 0 {
		t.Error("nothing should be launched for a gate")
	}
	if strings.Contains(ledgerText(t, dir), "waiting:") {
		t.Error("a gate stop must not park the gate: the person runs it")
	}
}

func TestAutoStopsBeforeAHumanReviewer(t *testing.T) {
	makeAutoProject(t, "  mode: auto\n  branch: off", []autoSlice{{id: "s-1", reviewer: "human"}}, "code")
	out, code := runAuto(t)
	expectStop(t, out, code, 3, ruleHumanAgent)
	if sessions(t) != 1 {
		t.Errorf("sessions = %d: only the coder runs", sessions(t))
	}
}

func TestAutoStopsWhenAnAgentAsksForAPerson(t *testing.T) {
	makeAutoProject(t, "  mode: auto\n  branch: off", []autoSlice{{id: "s-1"}}, "human:which SMS vendor?")
	out, code := runAuto(t)
	expectStop(t, out, code, 3, ruleNeedsHuman)
	if !strings.Contains(out, "which SMS vendor?") {
		t.Errorf("the question should be in the stop:\n%s", out)
	}
}

// A failed session is retried once; the second failure stops and parks.
// After the person clears the wait, a new run resumes from the ledger.
func TestAutoResumesAfterAStop(t *testing.T) {
	dir := makeAutoProject(t, "  mode: auto\n  branch: off", []autoSlice{{id: "s-1"}}, "code", "fail", "fail", "pass")
	out, code := runAuto(t)
	expectStop(t, out, code, 3, ruleSessionFailed)

	rootCmd.SetArgs([]string{"wait", "s-1", "--clear"})
	resetFlags(t, waitCmd.Flags())
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("wait --clear: %v", err)
	}
	out, code = runAuto(t)
	expectStop(t, out, code, 0, ruleBacklogEmpty)
	if !strings.Contains(archivedIDs(t, dir), "id: s-1") {
		t.Error("the resumed run should finish s-1")
	}
}

func TestAutoNoProgressStops(t *testing.T) {
	makeAutoProject(t, "  mode: auto\n  branch: off", []autoSlice{{id: "s-1"}}, "noop", "noop")
	out, code := runAuto(t)
	expectStop(t, out, code, 3, ruleNoProgress)
}

func TestAutoStopsOnADirtyTree(t *testing.T) {
	dir := makeAutoProject(t, "  mode: auto\n  branch: off", []autoSlice{{id: "s-1"}}, "code")
	if err := os.WriteFile(filepath.Join(dir, "stray.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code := runAuto(t)
	expectStop(t, out, code, 3, ruleDirtyTree)
	if !strings.Contains(out, "stray.txt") {
		t.Errorf("the stray file should be named:\n%s", out)
	}
}

func TestAutoStepStopsAfterOneSlice(t *testing.T) {
	dir := makeAutoProject(t, "  mode: step\n  branch: off", []autoSlice{{id: "s-1"}, {id: "s-2"}}, "code", "pass", "code", "pass")
	out, code := runAuto(t)
	expectStop(t, out, code, 0, ruleStepDone)
	if a := archivedIDs(t, dir); !strings.Contains(a, "id: s-1") || strings.Contains(a, "id: s-2") {
		t.Errorf("only s-1 should be done:\n%s", a)
	}
}

// An auto phase followed by a manual one stops at the boundary; a
// scoped --phase ... --mode auto enters it, an unscoped --mode auto does not.
func TestAutoModeBoundaries(t *testing.T) {
	slices := []autoSlice{
		{id: "phase-1-ws-1.1", plan: ".metis/plans/phase-1.md"},
		{id: "phase-2-ws-2.1", plan: ".metis/plans/phase-2.md"},
	}
	dir := makeAutoProject(t, "  mode: auto\n  branch: off\n  until: empty\n  phases:\n    phase-2: manual", slices, "code", "pass", "code", "pass")
	out, code := runAuto(t)
	expectStop(t, out, code, 0, ruleManual)
	if !strings.Contains(archivedIDs(t, dir), "phase-1-ws-1.1") {
		t.Error("phase 1 should be done")
	}

	out, code = runAuto(t, "--mode", "auto")
	expectStop(t, out, code, 0, ruleManual)

	out, code = runAuto(t, "--phase", "phase-2", "--mode", "auto")
	expectStop(t, out, code, 0, ruleBacklogEmpty)
	if !strings.Contains(archivedIDs(t, dir), "phase-2-ws-2.1") {
		t.Error("--phase phase-2 --mode auto should run phase 2")
	}
}

// An auto slice runs inside a manual phase.
func TestAutoSliceBeatsManualPhase(t *testing.T) {
	slices := []autoSlice{{id: "phase-2-ws-2.1", plan: ".metis/plans/phase-2.md", mode: "auto"}}
	dir := makeAutoProject(t, "  mode: auto\n  branch: off\n  phases:\n    phase-2: manual", slices, "code", "pass")
	out, code := runAuto(t)
	expectStop(t, out, code, 0, ruleBacklogEmpty)
	if !strings.Contains(archivedIDs(t, dir), "phase-2-ws-2.1") {
		t.Error("the auto slice should run")
	}
}

// Branch mode: a slice's work lands on dev only once it is reviewed.
func TestAutoBranchMergesOnlyOnReviewed(t *testing.T) {
	dir := makeAutoProject(t, "  mode: auto", []autoSlice{{id: "s-1"}, {id: "s-2"}},
		"code", "pass", "code", "p1:src/s-2.txt:1 — secret in the log — drop it")
	out, code := runAuto(t)
	expectStop(t, out, code, 3, ruleP1)

	if b := strings.TrimSpace(gitOut(t, dir, "rev-parse", "--abbrev-ref", "HEAD")); b != "auto/s-2" {
		t.Errorf("a stopped slice's branch is left checked out, got %s", b)
	}
	devFiles := gitOut(t, dir, "ls-tree", "-r", "--name-only", "dev")
	if !strings.Contains(devFiles, "src/s-1.txt") {
		t.Errorf("s-1 should be merged into dev:\n%s", devFiles)
	}
	if strings.Contains(devFiles, "src/s-2.txt") {
		t.Errorf("s-2 is not reviewed and must not reach dev:\n%s", devFiles)
	}
	if strings.Contains(gitOut(t, dir, "branch", "--list", "auto/s-1"), "auto/s-1") {
		t.Error("the merged branch should be deleted")
	}
}

func TestAutoDryRunLaunchesNothing(t *testing.T) {
	makeAutoProject(t, "  mode: auto\n  branch: off", []autoSlice{{id: "s-1"}}, "code")
	out, code := runAuto(t, "--dry-run")
	if code != 0 || !strings.Contains(out, "agent:    coder/a (model m-a)") || !strings.Contains(out, "template: code") {
		t.Errorf("dry run:\n%s", out)
	}
	if sessions(t) != 0 {
		t.Error("a dry run launched a session")
	}
}

func TestAutoStatus(t *testing.T) {
	makeAutoProject(t, "  mode: auto\n  branch: off", []autoSlice{{id: "s-1"}}, "code", "pass")
	runAuto(t)
	var st autoStatus
	resetFlags(t, autoCmd.Flags())
	setOutputFlag(t, "json")
	_ = autoCmd.Flags().Set("status", "true")
	captureJSON(t, autoCmd, nil, &st)
	if len(st.Done) != 1 || st.Sessions != 2 || st.Stop == nil || st.Stop.Rule != ruleBacklogEmpty {
		t.Errorf("status = %+v", st)
	}
}

// A slice parked on its auto branch stays the person's: a run from the
// work branch finds the wait there and launches nothing, and the earlier
// escalation is kept.
func TestAutoHonoursAWaitOnTheSliceBranch(t *testing.T) {
	dir := makeAutoProject(t, "  mode: auto", []autoSlice{{id: "s-1"}}, "code", "p1:src/s-1.txt:1 — secret logged — drop it", "code")
	out, code := runAuto(t)
	expectStop(t, out, code, 3, ruleP1)
	gitOut(t, dir, "checkout", "-q", "dev")

	out, code = runAuto(t)
	expectStop(t, out, code, 3, ruleAllWaiting)
	if sessions(t) != 2 {
		t.Errorf("sessions = %d: nothing may run on a parked slice", sessions(t))
	}
	esc, _ := os.ReadFile(filepath.Join(dir, ".metis", "runs", "s-1", "auto", "escalation.md"))
	if !strings.Contains(string(esc), "`p1`") {
		t.Errorf("the p1 escalation was replaced:\n%s", esc)
	}
}
