package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/techspeque/metis/internal/config"
	"github.com/techspeque/metis/internal/findings"
	"github.com/techspeque/metis/internal/prompt"
)

// makeProjectWithRules creates a project with four accuracy rules and a
// findings store whose promoted findings point at rules 2 and 4, moves cwd
// into it, and returns its path.
func makeProjectWithRules(t *testing.T) string {
	t.Helper()
	dir := makeProject(t)
	cfg := "version: 1\naccuracy_rules:\n  - rule one\n  - rule two\n  - rule three\n  - rule four\n"
	if err := os.WriteFile(filepath.Join(dir, ".metis", "project.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	store := `findings:
  - id: f-001
    slice: feat-0001
    severity: P2
    category: maint
    finding: rule two
    status: promoted
    promoted_to: 2
  - id: f-002
    slice: feat-0001
    severity: P2
    category: maint
    finding: rule four
    status: promoted
    promoted_to: 4
`
	if err := os.WriteFile(filepath.Join(dir, ".metis", "findings.yaml"), []byte(store), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	setOutputFlag(t, "")
	return dir
}

func stubRulePrompt(t *testing.T, isTTY bool, pick func([]string) ([]int, error)) {
	t.Helper()
	prevInteractive, prevSelect := interactive, selectRules
	interactive = func() bool { return isTTY }
	selectRules = pick
	t.Cleanup(func() { interactive, selectRules = prevInteractive, prevSelect })
}

func runRuleRemove(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	ruleRemoveCmd.SetOut(&out)
	t.Cleanup(func() { ruleRemoveCmd.SetOut(nil) })
	err := ruleRemoveCmd.RunE(ruleRemoveCmd, args)
	return out.String(), err
}

func loadRules(t *testing.T, dir string) []string {
	t.Helper()
	cfg, err := config.Load(filepath.Join(dir, ".metis", "project.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return cfg.AccuracyRules
}

func loadPromotedTo(t *testing.T, dir string) []*int {
	t.Helper()
	store, err := findings.Load(filepath.Join(dir, ".metis", "findings.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var out []*int
	for i := range store.Findings {
		out = append(out, store.Findings[i].PromotedTo)
	}
	return out
}

func TestRuleRemove_ByNumber(t *testing.T) {
	dir := makeProjectWithRules(t)
	stubRulePrompt(t, true, func([]string) ([]int, error) {
		t.Fatal("rule numbers on the command line must not open the picker")
		return nil, nil
	})

	out, err := runRuleRemove(t, "3", "#2", "3")
	if err != nil {
		t.Fatal(err)
	}
	if got := loadRules(t, dir); !reflect.DeepEqual(got, []string{"rule one", "rule four"}) {
		t.Errorf("rules = %v", got)
	}
	if !strings.Contains(out, "Removed accuracy rule #2: rule two") || !strings.Contains(out, "Removed accuracy rule #3: rule three") {
		t.Errorf("output = %q", out)
	}
	p := loadPromotedTo(t, dir)
	if p[0] != nil {
		t.Errorf("finding promoted to removed rule 2 should lose its pointer, got %d", *p[0])
	}
	if p[1] == nil || *p[1] != 2 {
		t.Errorf("finding promoted to rule 4 should now point at rule 2, got %v", p[1])
	}
}

func TestRuleRemove_Interactive(t *testing.T) {
	dir := makeProjectWithRules(t)
	var offered []string
	stubRulePrompt(t, true, func(rules []string) ([]int, error) {
		offered = rules
		return []int{0, 3}, nil
	})

	if _, err := runRuleRemove(t); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(offered, []string{"rule one", "rule two", "rule three", "rule four"}) {
		t.Errorf("picker should be offered the current rules, got %v", offered)
	}
	if got := loadRules(t, dir); !reflect.DeepEqual(got, []string{"rule two", "rule three"}) {
		t.Errorf("rules = %v", got)
	}
	p := loadPromotedTo(t, dir)
	if p[0] == nil || *p[0] != 1 || p[1] != nil {
		t.Errorf("promoted_to after removing rules 1 and 4 = %v", p)
	}
}

func TestRuleRemove_InteractiveCancelOrEmpty(t *testing.T) {
	for name, pick := range map[string]func([]string) ([]int, error){
		"cancel": func([]string) ([]int, error) { return nil, prompt.ErrCancelled },
		"empty":  func([]string) ([]int, error) { return nil, nil },
	} {
		t.Run(name, func(t *testing.T) {
			dir := makeProjectWithRules(t)
			stubRulePrompt(t, true, pick)
			out, err := runRuleRemove(t)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out, "no rules removed") && !strings.Contains(out, "nothing removed") {
				t.Errorf("output = %q", out)
			}
			if got := loadRules(t, dir); len(got) != 4 {
				t.Errorf("rules must be untouched, got %v", got)
			}
		})
	}
}

func TestRuleRemove_Errors(t *testing.T) {
	dir := makeProjectWithRules(t)
	stubRulePrompt(t, false, func([]string) ([]int, error) {
		t.Fatal("picker must not open off a terminal")
		return nil, nil
	})

	cases := map[string][]string{
		"no args off a terminal": nil,
		"out of range":           {"5"},
		"zero":                   {"0"},
		"not a number":           {"two"},
	}
	for name, args := range cases {
		if _, err := runRuleRemove(t, args...); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
	if got := loadRules(t, dir); len(got) != 4 {
		t.Errorf("a rejected removal must not touch the rules, got %v", got)
	}

	// With no rules at all there is nothing to pick from.
	if err := os.WriteFile(filepath.Join(dir, ".metis", "project.yaml"), []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runRuleRemove(t, "1"); err == nil || !strings.Contains(err.Error(), "no accuracy rules") {
		t.Errorf("no rules: err = %v", err)
	}
}
