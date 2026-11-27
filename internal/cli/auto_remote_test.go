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

// holdPush makes the fake remote launcher keep a session's commits on its
// node and exit 95, a session lost before its push. The function returned
// pushes them: the late push.
func holdPush(t *testing.T, branch string) (latePush func()) {
	t.Helper()
	state := os.Getenv("FAKE_STATE")
	hold := filepath.Join(state, "hold-push")
	if err := os.WriteFile(hold, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return func() {
		t.Helper()
		if err := os.Remove(hold); err != nil {
			t.Fatal(err)
		}
		gitOut(t, filepath.Join(state, "node"), "push", "-q", "origin", branch)
	}
}

// A lost session's commits reach origin late. The next session's branch is
// fast-forwarded to them before anything is launched, and the session is
// planned again from the ledger they brought: the coder's work is there,
// so the reviewer runs, not the coder a second time.
func TestAutoFastForwardsToALatePush(t *testing.T) {
	dir, origin := makeRemoteProject(t, "  mode: auto\n  branch: off", []autoSlice{{id: "s-1"}}, "code", "pass")
	latePush := holdPush(t, "dev")
	if code := resolveExitCode(runLaunch(t)); code != 95 {
		t.Fatalf("the held session should exit 95, got %d", code)
	}
	if strings.Contains(ledgerText(t, dir), "coded: true") {
		t.Fatal("the held session's flip must not be here yet")
	}
	latePush()

	out, code := runAuto(t)
	expectStop(t, out, code, 0, ruleBacklogEmpty)
	if !strings.Contains(out, "the coder session was not launched") || !strings.Contains(out, "reviewer reviewer/b") || strings.Count(out, " ended: ") != 1 {
		t.Errorf("the late commits should be taken and only the reviewer launched:\n%s", out)
	}
	if !strings.Contains(archivedIDs(t, dir), "id: s-1") || sessions(t) != 2 {
		t.Errorf("s-1 should be done in 2 sessions, got %d:\n%s", sessions(t), out)
	}
	if here, there := gitOut(t, dir, "rev-parse", "dev"), gitOut(t, origin, "rev-parse", "dev"); here != there {
		t.Errorf("dev is %s here, %s on origin", strings.TrimSpace(here), strings.TrimSpace(there))
	}
}

// Late commits that hold the reviewed flip finish the slice: no session is
// launched for it. metis launch, for its part, launches nothing and says
// to run it again.
func TestAutoALatePushMayFinishTheSlice(t *testing.T) {
	dir, _ := makeRemoteProject(t, "  mode: auto\n  branch: off", []autoSlice{{id: "s-1"}, {id: "s-2"}}, "code", "pass", "code")
	if err := runLaunch(t); err != nil {
		t.Fatalf("launch: %v", err)
	}
	latePush := holdPush(t, "dev")
	if code := resolveExitCode(runLaunch(t)); code != 95 {
		t.Fatalf("the held session should exit 95, got %d", code)
	}
	latePush()

	out, code := runAuto(t, "--max-slices", "1")
	expectStop(t, out, code, 0, ruleMaxSlices)
	if !strings.Contains(out, "fast-forwarded to them") || !strings.Contains(archivedIDs(t, dir), "id: s-1") || sessions(t) != 2 {
		t.Errorf("s-1 should be archived with no further session, got %d:\n%s", sessions(t), out)
	}

	latePush = holdPush(t, "dev")
	_ = runLaunch(t)
	latePush()
	if err := runLaunch(t); err == nil || !strings.Contains(err.Error(), "run metis launch again") || sessions(t) != 3 {
		t.Errorf("launch: err = %v, %d session(s)", err, sessions(t))
	}
	if !strings.Contains(ledgerText(t, dir), "coded: true") {
		t.Error("launch should have fast-forwarded to the late commits")
	}
}

// A branch that has diverged from origin's before the session stops the
// run with nothing launched and neither history rewritten.
func TestAutoStopsWhenTheRemoteDivergedBeforeTheSession(t *testing.T) {
	dir, origin := makeRemoteProject(t, "  mode: auto\n  branch: off", []autoSlice{{id: "s-1"}}, "code", "pass")
	gitOut(t, dir, "push", "-q", "origin", "dev")
	other := filepath.Join(t.TempDir(), "other")
	gitOut(t, dir, "clone", "-q", "-b", "dev", origin, other)
	gitOut(t, other, "-c", "user.email=o@t.co", "-c", "user.name=O", "commit", "-q", "--allow-empty", "-m", "chore: made elsewhere")
	gitOut(t, other, "push", "-q", "origin", "dev")
	gitOut(t, dir, "commit", "-q", "--allow-empty", "-m", "chore: made here")
	here, there := gitOut(t, dir, "rev-parse", "dev"), gitOut(t, origin, "rev-parse", "dev")

	out, code := runAuto(t)
	expectStop(t, out, code, 3, ruleSync)
	if sessions(t) != 0 || !strings.Contains(out, "has diverged from origin's before its session") {
		t.Errorf("sessions = %d, want none and a diverged branch:\n%s", sessions(t), out)
	}
	if err := runLaunch(t); err == nil || !strings.Contains(err.Error(), "has diverged from origin's") || sessions(t) != 0 {
		t.Errorf("launch: err = %v, %d session(s)", err, sessions(t))
	}
	if now := gitOut(t, dir, "rev-parse", "dev"); now != here {
		t.Errorf("the local branch was rewritten: %s, was %s", strings.TrimSpace(now), strings.TrimSpace(here))
	}
	if now := gitOut(t, origin, "rev-parse", "dev"); now != there {
		t.Errorf("origin's branch was rewritten: %s, was %s", strings.TrimSpace(now), strings.TrimSpace(there))
	}
	if strings.Contains(ledgerText(t, dir), "waiting:") {
		t.Error("a sync stop must not park the slice")
	}
}

// An interrupt during a remote session exits 130. The branch is still
// fetched and fast-forwarded, so what the agent pushed before the
// interrupt is here and the next run starts level with origin.
func TestAutoInterruptedDuringARemoteSession(t *testing.T) {
	dir, origin := makeRemoteProject(t, "  mode: auto", []autoSlice{{id: "s-1"}}, "code", "pass")
	// The coder's launcher: the remote session, then an interrupt to metis
	// auto while the launcher is still running.
	editProjectConfig(t, dir, func(cfg string) string {
		return strings.Replace(cfg, `launch: [sh, "`, `launch: [sh, -c, 'sh "$0" "$@"; kill -INT $PPID; sleep 30', "`, 1)
	})
	out, code := runAuto(t)
	expectStop(t, out, code, exitAutoInterrupted, ruleInterrupted)
	if sessions(t) != 1 {
		t.Errorf("sessions = %d, want 1", sessions(t))
	}
	if !strings.Contains(ledgerText(t, dir), "coded: true") {
		t.Errorf("what the agent pushed before the interrupt should be fetched:\n%s", out)
	}
	if here, there := gitOut(t, dir, "rev-parse", "auto/s-1"), gitOut(t, origin, "rev-parse", "auto/s-1"); here != there {
		t.Errorf("auto/s-1 is %s here, %s on origin", strings.TrimSpace(here), strings.TrimSpace(there))
	}
	if strings.Contains(ledgerText(t, dir), "waiting:") {
		t.Error("an interrupt must not park the slice")
	}
	if status := gitOut(t, dir, "status", "--short"); strings.TrimSpace(status) != "" {
		t.Errorf("tree dirty after the interrupt:\n%s", status)
	}

	out, code = runAuto(t)
	expectStop(t, out, code, 0, ruleBacklogEmpty)
	if !strings.Contains(archivedIDs(t, dir), "id: s-1") || sessions(t) != 2 {
		t.Errorf("the next run should finish s-1 with the review, got %d session(s):\n%s", sessions(t), out)
	}
}

// Origin refuses the work branch when a slice lands: the run stops with
// the slice archived here, and the next run pushes it once origin accepts.
func TestAutoRetriesTheLandingPush(t *testing.T) {
	dir, origin := makeRemoteProject(t, "  mode: auto", []autoSlice{{id: "s-1"}}, "code", "pass")
	closed := filepath.Join(origin, "dev-is-closed")
	hook := "#!/bin/sh\nif [ \"$1\" = refs/heads/dev ] && [ -f \"" + closed + "\" ]; then echo 'dev is closed' >&2; exit 1; fi\n"
	if err := os.MkdirAll(filepath.Join(origin, "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(origin, "hooks", "update"), []byte(hook), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(closed, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	out, code := runAuto(t)
	expectStop(t, out, code, 3, ruleSync)
	if !strings.Contains(out, "could not be pushed to origin") || !strings.Contains(archivedIDs(t, dir), "id: s-1") {
		t.Errorf("s-1 should be archived here and the push refused:\n%s", out)
	}
	if files := gitOut(t, dir, "ls-tree", "-r", "--name-only", "dev"); !strings.Contains(files, "src/s-1.txt") {
		t.Errorf("s-1 should be landed on dev here:\n%s", files)
	}
	if refs := gitOut(t, origin, "for-each-ref", "refs/heads/dev"); strings.TrimSpace(refs) != "" {
		t.Errorf("origin took dev:\n%s", refs)
	}

	// Still refused: the next run tries, and stops again.
	out, code = runAuto(t)
	expectStop(t, out, code, 3, ruleSync)

	if err := os.Remove(closed); err != nil {
		t.Fatal(err)
	}
	out, code = runAuto(t)
	expectStop(t, out, code, 0, ruleBacklogEmpty)
	if !strings.Contains(out, "s-1 landed but was not pushed: pushing") {
		t.Errorf("the push should be retried:\n%s", out)
	}
	if here, there := gitOut(t, dir, "rev-parse", "dev"), gitOut(t, origin, "rev-parse", "dev"); here != there {
		t.Errorf("dev is %s here, %s on origin", strings.TrimSpace(here), strings.TrimSpace(there))
	}
	if b := gitOut(t, origin, "branch", "--list", "auto/*"); strings.TrimSpace(b) != "" {
		t.Errorf("origin still has the auto branch:\n%s", b)
	}

	// Pushed: nothing is left to retry.
	ran := traceGit(t)
	out, code = runAuto(t)
	expectStop(t, out, code, 0, ruleBacklogEmpty)
	if n := count(ran(), "push"); n != 0 || strings.Contains(out, "not pushed") {
		t.Errorf("git push ran %d time(s) with nothing to retry:\n%s", n, out)
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
