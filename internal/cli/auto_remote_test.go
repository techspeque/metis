package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// makeRemoteProject is makeAutoProject with a bare origin, and with both
// agents remote: their launcher runs the fake agent in its own clone of
// origin and pushes what it commits. It returns the project and origin.
func makeRemoteProject(t *testing.T, autoYAML string, slices []autoSlice, actions ...string) (dir, origin string) {
	t.Helper()
	dir = makeAutoProject(t, autoYAML, slices, actions...)
	origin = filepath.Join(t.TempDir(), "origin.git")
	gitOut(t, dir, "init", "-q", "--bare", origin)
	gitOut(t, dir, "remote", "add", "origin", origin)
	agent := fakeAgentPath(t)
	remote := filepath.Join(filepath.Dir(agent), "fake-remote.sh")
	editProjectConfig(t, dir, func(cfg string) string {
		return strings.ReplaceAll(cfg, `launch: [sh, "`+agent+`",`, `remote: true
    transient_exit_codes: [91, 95]
    launch: [sh, "`+remote+`",`)
	})
	return dir, origin
}

// editProjectConfig rewrites .metis/project.yaml and commits it.
func editProjectConfig(t *testing.T, dir string, edit func(string) string) {
	t.Helper()
	path := filepath.Join(dir, ".metis", "project.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg := edit(string(data))
	if cfg == string(data) {
		t.Fatal("the project config was not changed")
	}
	if err := os.WriteFile(path, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	gitOut(t, dir, "commit", "-qam", "chore: agents")
}

// traceGit puts a git on PATH that records each command line before
// running the real git, for this process and every one it starts. It
// returns a function reporting the git subcommands run so far.
func traceGit(t *testing.T) func() []string {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	wrap := t.TempDir()
	trace := filepath.Join(wrap, "trace")
	script := "#!/bin/sh\necho \"$@\" >> \"" + trace + "\"\nexec \"" + realGit + "\" \"$@\"\n"
	if err := os.WriteFile(filepath.Join(wrap, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", wrap+string(os.PathListSeparator)+os.Getenv("PATH"))
	return func() []string {
		data, _ := os.ReadFile(trace)
		var subs []string
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			if f := strings.Fields(line); len(f) > 0 {
				subs = append(subs, f[0])
			}
		}
		return subs
	}
}

// runLaunch runs metis launch in-process for the dispatched slice.
func runLaunch(t *testing.T) error {
	t.Helper()
	setOutputFlag(t, "")
	resetFlags(t, launchCmd.Flags())
	return launchCmd.RunE(launchCmd, nil)
}

func count(list []string, want string) int {
	n := 0
	for _, s := range list {
		if s == want {
			n++
		}
	}
	return n
}

// The agents work in another clone and push: each session's branch is
// pushed first and fast-forwarded afterwards, so the flips are seen here
// and the slice completes. Landing it pushes the work branch and deletes
// origin's auto branch.
func TestAutoRemoteSessions(t *testing.T) {
	dir, origin := makeRemoteProject(t, "  mode: auto", []autoSlice{{id: "s-1"}}, "code", "pass")
	out, code := runAuto(t)
	expectStop(t, out, code, 0, ruleBacklogEmpty)
	if !strings.Contains(archivedIDs(t, dir), "id: s-1") || sessions(t) != 2 {
		t.Fatalf("s-1 should be done in 2 sessions, got %d:\n%s", sessions(t), out)
	}
	if files := gitOut(t, dir, "ls-tree", "-r", "--name-only", "dev"); !strings.Contains(files, "src/s-1.txt") {
		t.Errorf("the remote agent's work is not on dev:\n%s", files)
	}
	if here, there := gitOut(t, dir, "rev-parse", "dev"), gitOut(t, origin, "rev-parse", "dev"); here != there {
		t.Errorf("the work branch was not pushed: dev is %s here, %s on origin", strings.TrimSpace(here), strings.TrimSpace(there))
	}
	if b := gitOut(t, origin, "branch", "--list", "auto/*"); strings.TrimSpace(b) != "" {
		t.Errorf("origin still has the auto branch:\n%s", b)
	}
	if b := gitOut(t, dir, "branch", "--list", "auto/*"); strings.TrimSpace(b) != "" {
		t.Errorf("the merged branch should be deleted here:\n%s", b)
	}
	if status := gitOut(t, dir, "status", "--short"); strings.TrimSpace(status) != "" {
		t.Errorf("tree dirty after the run:\n%s", status)
	}
}

// auto.branch: off: the sessions run on the work branch, which is pushed
// again once the slice is archived.
func TestAutoRemoteSessionsOnTheWorkBranch(t *testing.T) {
	dir, origin := makeRemoteProject(t, "  mode: auto\n  branch: off", []autoSlice{{id: "s-1"}}, "code", "pass")
	out, code := runAuto(t)
	expectStop(t, out, code, 0, ruleBacklogEmpty)
	if !strings.Contains(archivedIDs(t, dir), "id: s-1") {
		t.Fatalf("s-1 not archived:\n%s", out)
	}
	if here, there := gitOut(t, dir, "rev-parse", "dev"), gitOut(t, origin, "rev-parse", "dev"); here != there {
		t.Errorf("the archive was not pushed: dev is %s here, %s on origin", strings.TrimSpace(here), strings.TrimSpace(there))
	}
}

// metis launch syncs around a remote session too.
func TestLaunchRemoteSession(t *testing.T) {
	dir, origin := makeRemoteProject(t, "  mode: auto", []autoSlice{{id: "s-1"}}, "code")
	if err := runLaunch(t); err != nil {
		t.Fatalf("launch: %v", err)
	}
	if l := ledgerText(t, dir); !strings.Contains(l, "coded: true") {
		t.Errorf("the remote flip is not in the ledger here:\n%s", l)
	}
	if here, there := gitOut(t, dir, "rev-parse", "dev"), gitOut(t, origin, "rev-parse", "dev"); here != there {
		t.Errorf("dev is %s here, %s on origin", strings.TrimSpace(here), strings.TrimSpace(there))
	}
}

// A branch that cannot be pushed stops the run before anything is
// launched; the slice is not parked.
func TestAutoStopsWhenTheBranchCannotBePushed(t *testing.T) {
	dir, _ := makeRemoteProject(t, "  mode: auto", []autoSlice{{id: "s-1"}}, "code", "pass")
	gitOut(t, dir, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "gone.git"))
	out, code := runAuto(t)
	expectStop(t, out, code, 3, ruleSync)
	if sessions(t) != 0 {
		t.Errorf("sessions = %d: nothing may be launched", sessions(t))
	}
	if strings.Contains(ledgerText(t, dir), "waiting:") {
		t.Error("a sync stop must not park the slice")
	}

	if err := runLaunch(t); err == nil || !strings.Contains(err.Error(), "could not be pushed to origin") || sessions(t) != 0 {
		t.Errorf("launch: err = %v, %d session(s)", err, sessions(t))
	}
}

// A commit made here while the remote agent works leaves two histories:
// the run stops, and neither is rewritten.
func TestAutoStopsWhenTheRemoteDiverged(t *testing.T) {
	dir, origin := makeRemoteProject(t, "  mode: auto", []autoSlice{{id: "s-1"}}, "code", "pass")
	editProjectConfig(t, dir, func(cfg string) string {
		return strings.Replace(cfg, `launch: [sh, "`, `launch: [sh, -c, 'git commit -q --allow-empty -m "chore: made here during the session" && exec sh "$0" "$@"', "`, 1)
	})
	out, code := runAuto(t)
	expectStop(t, out, code, 3, ruleSync)
	if sessions(t) != 1 || !strings.Contains(out, "cannot fast-forward") {
		t.Errorf("sessions = %d, want 1 and a branch that cannot fast-forward:\n%s", sessions(t), out)
	}
	if subject := gitOut(t, dir, "log", "-1", "--format=%s"); !strings.Contains(subject, "made here during the session") {
		t.Errorf("the local branch was rewritten: HEAD is %q", strings.TrimSpace(subject))
	}
	if subject := gitOut(t, origin, "log", "-1", "--format=%s", "auto/s-1"); strings.Contains(subject, "made here") {
		t.Errorf("origin's branch was rewritten: its head is %q", strings.TrimSpace(subject))
	}
	if b := strings.TrimSpace(gitOut(t, dir, "rev-parse", "--abbrev-ref", "HEAD")); b != "auto/s-1" {
		t.Errorf("on %s, want auto/s-1", b)
	}
	if status := gitOut(t, dir, "status"); strings.Contains(status, "rebase") || strings.Contains(status, "merging") {
		t.Errorf("an operation was left in progress:\n%s", status)
	}
	if strings.Contains(ledgerText(t, dir), "coded: true") {
		t.Error("the remote flip must not be read from a branch that did not fast-forward")
	}
}

// A launcher that exits with a transient code is waited out like a rate
// limit and the session run again: with no retries allowed, any charge to
// session_failed would stop the run.
func TestAutoWaitsOutATransientExitCode(t *testing.T) {
	dir, _ := makeRemoteProject(t, "  mode: auto\n  stop: { session_failed: 0 }", []autoSlice{{id: "s-1"}}, "code", "exit:91", "exit:95", "pass")
	untils := fakeSleep(t, nil)
	out, code := runAuto(t)
	expectStop(t, out, code, 0, ruleBacklogEmpty)
	if !strings.Contains(archivedIDs(t, dir), "id: s-1") || sessions(t) != 4 {
		t.Errorf("s-1 should be done in 4 sessions, got %d:\n%s", sessions(t), out)
	}
	if len(*untils) != 2 {
		t.Fatalf("waits = %d, want 2:\n%s", len(*untils), out)
	}
	expectWait(t, (*untils)[0], 15*time.Minute)
	expectWait(t, (*untils)[1], 30*time.Minute)
	ctx, err := loadContext()
	if err != nil {
		t.Fatal(err)
	}
	if n, wall := sliceBudgetUse(ctx.readAutoEvents(), "s-1"); n != 2 || wall > time.Minute {
		t.Errorf("budget use = %d session(s), %s: a transient session is charged to no budget", n, wall)
	}
	if log := gitOut(t, dir, "log", "--format=%s"); strings.Count(log, "wait: auto: transient") != 2 || strings.Count(log, "wait cleared") != 2 {
		t.Errorf("the waits are not in the history:\n%s", log)
	}
}

// Transient waits share auto.rate_limit.max_wait: past it the run stops
// with the slice waiting until a time.
func TestAutoTransientWaitsAreBounded(t *testing.T) {
	dir, _ := makeRemoteProject(t, "  mode: auto\n  rate_limit: { max_wait: 40m }", []autoSlice{{id: "s-1"}}, "code", "exit:91", "exit:91", "pass")
	untils := fakeSleep(t, nil)
	out, code := runAuto(t)
	expectStop(t, out, code, 3, ruleRateLimited)
	if len(*untils) != 1 || sessions(t) != 3 {
		t.Errorf("want one 15m wait and a stop before a 30m one: %d wait(s), %d session(s)\n%s", len(*untils), sessions(t), out)
	}
	if !strings.Contains(out, "exited 91, one of its transient_exit_codes") {
		t.Errorf("the stop should name the exit code:\n%s", out)
	}
	if l := ledgerText(t, dir); !strings.Contains(l, "reason: 'auto: transient'") || !strings.Contains(l, "until:") || strings.Contains(l, "human: true") {
		t.Errorf("the slice should wait until a time, not for a person:\n%s", l)
	}
}

// An exit code the agent does not list is a failed session, as before.
func TestAutoAnExitCodeNotListedIsAFailure(t *testing.T) {
	makeRemoteProject(t, "  mode: auto", []autoSlice{{id: "s-1"}}, "code", "exit:93", "exit:93")
	untils := fakeSleep(t, nil)
	out, code := runAuto(t)
	expectStop(t, out, code, 3, ruleSessionFailed)
	if len(*untils) != 0 || sessions(t) != 3 {
		t.Errorf("%d wait(s), %d session(s):\n%s", len(*untils), sessions(t), out)
	}
}

// Without remote, the sessions and the landing run no git push and no git
// fetch, even with an origin to reach.
func TestAutoWithoutRemoteNeverPushesOrFetches(t *testing.T) {
	dir := makeAutoProject(t, "  mode: auto", []autoSlice{{id: "s-1"}, {id: "s-2"}}, "code", "pass", "code")
	origin := filepath.Join(t.TempDir(), "origin.git")
	gitOut(t, dir, "init", "-q", "--bare", origin)
	gitOut(t, dir, "remote", "add", "origin", origin)
	ran := traceGit(t)

	out, code := runAuto(t, "--max-slices", "1")
	expectStop(t, out, code, 0, ruleMaxSlices)
	if !strings.Contains(archivedIDs(t, dir), "id: s-1") {
		t.Fatalf("s-1 not archived:\n%s", out)
	}
	if err := runLaunch(t); err != nil || sessions(t) != 3 {
		t.Fatalf("launch: %v, %d session(s)", err, sessions(t))
	}

	subs := ran()
	if count(subs, "commit") == 0 || count(subs, "merge") == 0 {
		t.Fatalf("the trace should hold the run's git commands, got %v", subs)
	}
	for _, sub := range []string{"push", "fetch", "pull"} {
		if n := count(subs, sub); n != 0 {
			t.Errorf("git %s ran %d time(s) without a remote agent", sub, n)
		}
	}
	if refs := gitOut(t, origin, "for-each-ref"); strings.TrimSpace(refs) != "" {
		t.Errorf("origin was written to:\n%s", refs)
	}
}

// The trace does see a push and a fetch when they happen.
func TestAutoRemoteSessionsPushAndFetch(t *testing.T) {
	makeRemoteProject(t, "  mode: auto", []autoSlice{{id: "s-1"}}, "code", "pass")
	ran := traceGit(t)
	out, code := runAuto(t)
	expectStop(t, out, code, 0, ruleBacklogEmpty)
	if subs := ran(); count(subs, "push") == 0 || count(subs, "fetch") == 0 {
		t.Errorf("no push or fetch in the trace: %v", subs)
	}
}
