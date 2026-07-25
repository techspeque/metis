package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

// runEdit executes 'metis edit' through the root command with clean flag
// state (pflag's Changed persists across Execute calls otherwise).
func runEdit(t *testing.T, args ...string) error {
	t.Helper()
	editCmd.Flags().Visit(func(f *pflag.Flag) {
		_ = f.Value.Set(f.DefValue)
		f.Changed = false
	})
	rootCmd.SetArgs(append([]string{"edit"}, args...))
	t.Cleanup(func() { rootCmd.SetArgs([]string{}) })
	return rootCmd.Execute()
}

// replaceInFile substitutes old with new in a file.
func replaceInFile(t *testing.T, path, old, new string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.ReplaceAll(string(data), old, new)), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestEditUpdatesFields(t *testing.T) {
	makeProjectWithLedger(t)
	setOutputFlag(t, "")

	if err := runEdit(t, "feat-0001", "--title", "Renamed", "--risk", "high", "--priority", "p1"); err != nil {
		t.Fatalf("edit: %v", err)
	}

	ctx, err := loadContext()
	if err != nil {
		t.Fatal(err)
	}
	l, err := ctx.loadLedger()
	if err != nil {
		t.Fatal(err)
	}
	s := l.FindByID("feat-0001")
	if s.Title != "Renamed" || string(s.Risk) != "high" || string(s.Priority) != "p1" {
		t.Errorf("slice after edit = %+v", s)
	}
	// Untouched fields survive.
	if s.Coder != "claude-code/opus" || s.Reviewer != "opencode/opus" {
		t.Errorf("unrelated fields changed: %+v", s)
	}
}

func TestEditValidation(t *testing.T) {
	makeProjectWithLedger(t)
	setOutputFlag(t, "")

	cases := [][]string{
		{"ghost", "--title", "x"},                       // unknown slice
		{"feat-0001"},                                   // no flags
		{"feat-0001", "--risk", "extreme"},              // bad risk
		{"feat-0001", "--priority", "p9"},               // bad priority
		{"feat-0001", "--blocked-by", "nope-1"},         // unknown dep
		{"feat-0001", "--blocked-by", "feat-0001"},      // self-block
		{"feat-0001", "--reviewer", "claude-code/opus"}, // same as coder
		{"feat-0001", "--plan", "p.md"},                 // plan without section
	}
	for _, args := range cases {
		if err := runEdit(t, args...); err == nil {
			t.Errorf("edit %v: want error, got nil", args)
		}
	}
}

func TestEditRefusesDoneSlice(t *testing.T) {
	dir := makeProjectWithLedger(t)
	setOutputFlag(t, "")
	ledger := filepath.Join(dir, ".metis", "slices.yaml")
	replaceInFile(t, ledger, "coded: false", "coded: true")
	replaceInFile(t, ledger, "reviewed: false", "reviewed: true")

	err := runEdit(t, "feat-0001", "--title", "x")
	if err == nil || !strings.Contains(err.Error(), "reopen") {
		t.Errorf("editing a done slice should point at 'metis reopen', got: %v", err)
	}
}
