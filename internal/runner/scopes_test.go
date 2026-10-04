package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/techspeque/metis/internal/config"
)

// A repository with two scopes whose commands count their runs apart from
// the whole command's.
func scopedRepo(t *testing.T) (*config.Config, string, map[string]string, func(opts VerifyOptions) (int, *VerifyOutcome)) {
	t.Helper()
	cfg, dir, whole, store := cacheRepo(t, "echo run >> COUNTER")
	counters := map[string]string{"whole": whole, "api": filepath.Join(t.TempDir(), "api"), "web": filepath.Join(t.TempDir(), "web")}
	cfg.Commands.VerifyScopes = []config.VerifyScope{
		{Name: "api", Paths: []string{"api/", "**/openapi.json"}, Command: "echo run >> " + counters["api"]},
		{Name: "web", Paths: []string{"web", "api/openapi.json"}, Command: "echo run >> " + counters["web"]},
	}
	for _, p := range []string{"api/a.go", "api/openapi.json", "web/w.ts"} {
		must(t, os.MkdirAll(filepath.Join(dir, filepath.Dir(p)), 0o755))
		must(t, os.WriteFile(filepath.Join(dir, p), []byte("v1\n"), 0o644))
	}
	return cfg, dir, counters, func(opts VerifyOptions) (int, *VerifyOutcome) {
		return verify(t, cfg, dir, store, opts)
	}
}

func counts(t *testing.T, counters map[string]string) map[string]int {
	t.Helper()
	out := map[string]int{}
	for name, path := range counters {
		out[name] = count(t, path)
	}
	return out
}

func TestScopedVerifyRunsOnlyTheScopesWhoseContentChanged(t *testing.T) {
	cfg, dir, counters, run := scopedRepo(t)

	// Nothing has passed: the whole command runs and covers every scope.
	if code, out := run(VerifyOptions{}); code != 0 || !out.Whole || out.Reason == "" {
		t.Fatalf("first run: code %d whole %v reason %q", code, out.Whole, out.Reason)
	}
	if c := counts(t, counters); c["whole"] != 1 || c["api"] != 0 || c["web"] != 0 {
		t.Fatalf("first run counts: %v", c)
	}

	// A change inside one scope runs that scope alone.
	must(t, os.WriteFile(filepath.Join(dir, "web", "w.ts"), []byte("v2\n"), 0o644))
	code, out := run(VerifyOptions{})
	if code != 0 || out.Whole || strings.Join(out.Ran, ",") != "web" || len(out.Reused) != 1 || out.Reused[0].Name != "api" {
		t.Fatalf("web change: code %d whole %v ran %v reused %d", code, out.Whole, out.Ran, len(out.Reused))
	}
	if c := counts(t, counters); c["whole"] != 1 || c["api"] != 0 || c["web"] != 1 {
		t.Fatalf("web change counts: %v", c)
	}

	// The same content again reuses both scopes.
	if code, out := run(VerifyOptions{}); code != 0 || len(out.Ran) != 0 || len(out.Reused) != 2 {
		t.Fatalf("unchanged: code %d ran %v reused %d", code, out.Ran, len(out.Reused))
	}

	// A file in two scopes runs both.
	must(t, os.WriteFile(filepath.Join(dir, "api", "openapi.json"), []byte("v2\n"), 0o644))
	if code, out := run(VerifyOptions{}); code != 0 || strings.Join(out.Ran, ",") != "api,web" {
		t.Fatalf("shared file: code %d ran %v", code, out.Ran)
	}

	// A file outside every scope runs the whole command.
	must(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main // v2\n"), 0o644))
	if code, out := run(VerifyOptions{}); code != 0 || !out.Whole {
		t.Fatalf("rest change: code %d whole %v", code, out.Whole)
	}
	if c := counts(t, counters); c["whole"] != 2 || c["api"] != 1 || c["web"] != 2 {
		t.Fatalf("rest change counts: %v", c)
	}

	// --force runs the whole command; --scope runs the named scope regardless.
	if code, out := run(VerifyOptions{Force: true}); code != 0 || !out.Whole {
		t.Fatalf("force: code %d whole %v", code, out.Whole)
	}
	if code, out := run(VerifyOptions{Scopes: []string{"api"}}); code != 0 || strings.Join(out.Ran, ",") != "api" {
		t.Fatalf("--scope: code %d ran %v", code, out.Ran)
	}
	if c := counts(t, counters); c["whole"] != 3 || c["api"] != 2 {
		t.Fatalf("force and scope counts: %v", c)
	}
	_, _, err := Verify(cfg, dir, "s-1", VerifyOptions{Scopes: []string{"nope"}}, nil)
	if err == nil || !strings.Contains(err.Error(), "no such scope") {
		t.Fatalf("unknown scope: %v", err)
	}
}

func TestAFailingScopeFailsVerifyAndIsNotRemembered(t *testing.T) {
	cfg, dir, counters, run := scopedRepo(t)
	if code, _ := run(VerifyOptions{}); code != 0 {
		t.Fatal("first run")
	}
	cfg.Commands.VerifyScopes[1].Command = "exit 3"
	must(t, os.WriteFile(filepath.Join(dir, "web", "w.ts"), []byte("v2\n"), 0o644))
	if code, out := run(VerifyOptions{}); code != ExitCodeFailure || strings.Join(out.Ran, ",") != "web" {
		t.Fatalf("failing scope: code %d ran %v", code, out.Ran)
	}
	if code, _ := run(VerifyOptions{}); code != ExitCodeFailure {
		t.Fatal("a failed scope must run again")
	}
	if c := counts(t, counters); c["api"] != 0 {
		t.Fatalf("api ran: %v", c)
	}
}

func TestScopePathPatterns(t *testing.T) {
	cases := []struct {
		pattern, path string
		want          bool
	}{
		{"api/", "api/a.go", true},
		{"api", "api/deep/b.go", true},
		{"api", "apix/a.go", false},
		{"go.mod", "go.mod", true},
		{"*.md", "README.md", true},
		{"*.md", "docs/x.md", false},
		{"**/*.md", "docs/x.md", true},
		{"**/*.md", "README.md", true},
		{"**/openapi.json", "a/internal/api/openapi.json", true},
		{"console/lib/**", "console/lib/api/x.ts", true},
		{"console/lib/**", "console/app/x.ts", false},
	}
	for _, c := range cases {
		if got := compileMatcher(c.pattern)(c.path); got != c.want {
			t.Errorf("%q against %q: got %v", c.pattern, c.path, got)
		}
	}
}
