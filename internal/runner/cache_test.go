package runner

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/techspeque/metis/internal/config"
	"github.com/techspeque/metis/internal/runs"
)

// A repository whose verify command counts its runs in a file outside it.
func cacheRepo(t *testing.T, verify string) (*config.Config, string, string, *runs.Store) {
	t.Helper()
	dir := t.TempDir()
	counter := filepath.Join(t.TempDir(), "runs")
	git := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	git("init", "-q")
	git("config", "user.email", "t@example.com")
	git("config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".metis/runs/\ncache/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "-A")
	git("commit", "-q", "-m", "init")
	cfg := config.DefaultConfig()
	cfg.Commands.Verify = strings.ReplaceAll(verify, "COUNTER", counter)
	return &cfg, dir, counter, runs.NewStore(filepath.Join(dir, cfg.Paths.Runs))
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func count(t *testing.T, counter string) int {
	data, err := os.ReadFile(counter)
	if err != nil {
		return 0
	}
	return strings.Count(string(data), "\n")
}

func verify(t *testing.T, cfg *config.Config, dir string, store *runs.Store, opts VerifyOptions) (int, *VerifyOutcome) {
	t.Helper()
	code, outcome, err := Verify(cfg, dir, "s-1", opts, store)
	if err != nil {
		t.Fatal(err)
	}
	return code, outcome
}

func TestVerifyReusesAGreenRunOfTheSameTree(t *testing.T) {
	cfg, dir, counter, store := cacheRepo(t, "echo run >> COUNTER")

	if code, out := verify(t, cfg, dir, store, VerifyOptions{Label: "pre"}); code != 0 || out.Cached != nil {
		t.Fatalf("first run: code %d, cached %v", code, out.Cached)
	}
	must(t, os.MkdirAll(filepath.Join(dir, "cache"), 0o755))
	must(t, os.WriteFile(filepath.Join(dir, "cache", "blob"), []byte("x"), 0o644))
	code, out := verify(t, cfg, dir, store, VerifyOptions{Label: "post"})
	if code != 0 || out.Cached == nil || count(t, counter) != 1 {
		t.Fatalf("same tree: code %d, cached %v, runs %d", code, out.Cached, count(t, counter))
	}
	log, exit, err := store.Read("s-1", "verify-post")
	if err != nil || exit != 0 || !strings.Contains(string(log), "cached: tree "+out.Cached.Tree) || !strings.Contains(string(log), "verify-pre.log") {
		t.Fatalf("cached log: exit %d err %v\n%s", exit, err, log)
	}

	if _, out := verify(t, cfg, dir, store, VerifyOptions{Force: true}); out.Cached != nil || count(t, counter) != 2 {
		t.Fatalf("--force: cached %v, runs %d", out.Cached, count(t, counter))
	}
	must(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main // edited\n"), 0o644))
	if _, out := verify(t, cfg, dir, store, VerifyOptions{}); out.Cached != nil || count(t, counter) != 3 {
		t.Fatalf("edited tree: cached %v, runs %d", out.Cached, count(t, counter))
	}
}

func TestVerifyRemembersOnlyAGreenRunThatLeftTheTreeAsItFoundIt(t *testing.T) {
	cfg, dir, counter, store := cacheRepo(t, "echo run >> COUNTER; exit 1")
	for i := 0; i < 2; i++ {
		if code, _ := verify(t, cfg, dir, store, VerifyOptions{}); code != ExitCodeFailure {
			t.Fatalf("failing verify exited %d", code)
		}
	}
	if count(t, counter) != 2 {
		t.Fatalf("a failing run was reused: runs %d", count(t, counter))
	}

	cfg, dir, counter, store = cacheRepo(t, "echo run >> COUNTER; echo $$ > generated.txt; echo x > cache/ignored")
	must(t, os.MkdirAll(filepath.Join(dir, "cache"), 0o755))
	verify(t, cfg, dir, store, VerifyOptions{})
	verify(t, cfg, dir, store, VerifyOptions{})
	if count(t, counter) != 2 {
		t.Fatalf("a run that rewrote the tree was reused: runs %d", count(t, counter))
	}
}

func TestVerifyCacheCanBeTurnedOff(t *testing.T) {
	cfg, dir, counter, store := cacheRepo(t, "echo run >> COUNTER")
	off := false
	cfg.Commands.VerifyCache = &off
	verify(t, cfg, dir, store, VerifyOptions{})
	if _, out := verify(t, cfg, dir, store, VerifyOptions{}); out.Cached != nil || count(t, counter) != 2 {
		t.Fatalf("verify_cache false: cached %v, runs %d", out.Cached, count(t, counter))
	}
}

func TestVerifyStillRunsTheEnvCheckOnACachedTree(t *testing.T) {
	cfg, dir, _, store := cacheRepo(t, "echo run >> COUNTER")
	up := filepath.Join(t.TempDir(), "service-up")
	must(t, os.WriteFile(up, nil, 0o644))
	cfg.Commands.EnvCheck = "test -f " + up
	verify(t, cfg, dir, store, VerifyOptions{})
	must(t, os.Remove(up))
	if code, _ := verify(t, cfg, dir, store, VerifyOptions{}); code != ExitEnvFailure {
		t.Fatalf("a service gone down on a cached tree exited %d", code)
	}
}
