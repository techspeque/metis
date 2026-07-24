package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/techspeque/metis/internal/userconfig"
)

// TestWorkspaceLifecycle exercises add → use → remove against a temp
// user config, verifying registry state after each step.
func TestWorkspaceLifecycle(t *testing.T) {
	t.Setenv(userconfig.EnvPath, filepath.Join(t.TempDir(), "config.yaml"))
	proj := makeProject(t)

	if err := workspaceAddCmd.RunE(workspaceAddCmd, []string{"myws", proj}); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := workspaceUseCmd.RunE(workspaceUseCmd, []string{"myws"}); err != nil {
		t.Fatalf("use: %v", err)
	}

	uc, err := userconfig.Load()
	if err != nil {
		t.Fatal(err)
	}
	if uc.Active != "myws" || uc.Workspaces["myws"] != proj {
		t.Errorf("after add+use: %+v", uc)
	}

	if err := workspaceRemoveCmd.RunE(workspaceRemoveCmd, []string{"myws"}); err != nil {
		t.Fatalf("remove: %v", err)
	}
	uc, err = userconfig.Load()
	if err != nil {
		t.Fatal(err)
	}
	if uc.Active != "" || len(uc.Workspaces) != 0 {
		t.Errorf("after remove: %+v, want empty registry and no active", uc)
	}
}

func TestWorkspaceAddDefaultsToCwdRepoRoot(t *testing.T) {
	t.Setenv(userconfig.EnvPath, filepath.Join(t.TempDir(), "config.yaml"))
	proj := makeProject(t)

	sub := filepath.Join(proj, "internal", "deep")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sub)

	if err := workspaceAddCmd.RunE(workspaceAddCmd, []string{"here"}); err != nil {
		t.Fatalf("add without path: %v", err)
	}

	uc, err := userconfig.Load()
	if err != nil {
		t.Fatal(err)
	}
	if uc.Workspaces["here"] != proj {
		t.Errorf("registered path = %s, want discovered repo root %s", uc.Workspaces["here"], proj)
	}
}

func TestWorkspaceAddRejectsNonProject(t *testing.T) {
	t.Setenv(userconfig.EnvPath, filepath.Join(t.TempDir(), "config.yaml"))
	empty := t.TempDir()

	if err := workspaceAddCmd.RunE(workspaceAddCmd, []string{"bad", empty}); err == nil {
		t.Fatal("add on a directory without metis.yaml: want error")
	}
}

func TestWorkspaceUseUnknown(t *testing.T) {
	t.Setenv(userconfig.EnvPath, filepath.Join(t.TempDir(), "config.yaml"))

	if err := workspaceUseCmd.RunE(workspaceUseCmd, []string{"ghost"}); err == nil {
		t.Fatal("use unknown workspace: want error")
	}
}

// TestWorkspaceListHandlesMissingPath verifies list does not error when a
// registered path no longer contains a project.
func TestWorkspaceListHandlesMissingPath(t *testing.T) {
	t.Setenv(userconfig.EnvPath, filepath.Join(t.TempDir(), "config.yaml"))
	uc := &userconfig.UserConfig{Workspaces: map[string]string{"gone": "/does/not/exist"}}
	if err := uc.Save(); err != nil {
		t.Fatal(err)
	}

	if err := workspaceListCmd.RunE(workspaceListCmd, nil); err != nil {
		t.Errorf("list with missing path should not error: %v", err)
	}
}
