package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/techspeque/metis/internal/userconfig"
)

// makeProject creates a directory containing a minimal metis.yaml and
// returns its path (symlinks resolved, so paths compare cleanly on macOS).
func makeProject(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "metis.yaml"), []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// setUserConfig points METIS_USER_CONFIG at a temp file and writes the given
// registry to it.
func setUserConfig(t *testing.T, active string, workspaces map[string]string) {
	t.Helper()
	t.Setenv(userconfig.EnvPath, filepath.Join(t.TempDir(), "config.yaml"))
	uc := &userconfig.UserConfig{Active: active, Workspaces: workspaces}
	if err := uc.Save(); err != nil {
		t.Fatal(err)
	}
}

// setWorkspaceFlag sets the package-level --workspace flag value for the
// duration of the test.
func setWorkspaceFlag(t *testing.T, value string) {
	t.Helper()
	prev := workspaceFlag
	workspaceFlag = value
	t.Cleanup(func() { workspaceFlag = prev })
}

// chdirOutsideProject moves cwd to a directory with no metis.yaml anywhere
// on the walk-up path (a fresh temp dir).
func chdirOutsideProject(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())
}

func TestResolveFlagBeatsEnvAndCwd(t *testing.T) {
	flagProj := makeProject(t)
	envProj := makeProject(t)
	cwdProj := makeProject(t)
	setUserConfig(t, "", map[string]string{"flagws": flagProj, "envws": envProj})
	setWorkspaceFlag(t, "flagws")
	t.Setenv(envWorkspace, "envws")
	t.Chdir(cwdProj)

	ctx, err := loadContext()
	if err != nil {
		t.Fatalf("loadContext() error: %v", err)
	}
	if ctx.repoRoot != flagProj {
		t.Errorf("repoRoot = %s, want flag workspace %s", ctx.repoRoot, flagProj)
	}
	if ctx.source != sourceFlag || ctx.wsName != "flagws" {
		t.Errorf("source = %s/%s, want flag/flagws", ctx.source, ctx.wsName)
	}
}

func TestResolveEnvBeatsCwd(t *testing.T) {
	envProj := makeProject(t)
	cwdProj := makeProject(t)
	setUserConfig(t, "", map[string]string{"envws": envProj})
	setWorkspaceFlag(t, "")
	t.Setenv(envWorkspace, "envws")
	t.Chdir(cwdProj)

	ctx, err := loadContext()
	if err != nil {
		t.Fatalf("loadContext() error: %v", err)
	}
	if ctx.repoRoot != envProj {
		t.Errorf("repoRoot = %s, want env workspace %s", ctx.repoRoot, envProj)
	}
	if ctx.source != sourceEnv {
		t.Errorf("source = %s, want env", ctx.source)
	}
}

// TestResolveCwdBeatsActive is the agent-safety regression test: inside a
// repo, the active workspace selection must be ignored entirely.
func TestResolveCwdBeatsActive(t *testing.T) {
	activeProj := makeProject(t)
	cwdProj := makeProject(t)
	setUserConfig(t, "other", map[string]string{"other": activeProj})
	setWorkspaceFlag(t, "")
	t.Setenv(envWorkspace, "")
	t.Chdir(cwdProj)

	ctx, err := loadContext()
	if err != nil {
		t.Fatalf("loadContext() error: %v", err)
	}
	if ctx.repoRoot != cwdProj {
		t.Errorf("repoRoot = %s, want cwd project %s — active workspace must never override cwd", ctx.repoRoot, cwdProj)
	}
	if ctx.source != sourceCwd {
		t.Errorf("source = %s, want cwd", ctx.source)
	}
}

func TestResolveActiveWhenOutsideProject(t *testing.T) {
	activeProj := makeProject(t)
	setUserConfig(t, "myws", map[string]string{"myws": activeProj})
	setWorkspaceFlag(t, "")
	t.Setenv(envWorkspace, "")
	chdirOutsideProject(t)

	ctx, err := loadContext()
	if err != nil {
		t.Fatalf("loadContext() error: %v", err)
	}
	if ctx.repoRoot != activeProj {
		t.Errorf("repoRoot = %s, want active workspace %s", ctx.repoRoot, activeProj)
	}
	if ctx.source != sourceActive || ctx.wsName != "myws" {
		t.Errorf("source = %s/%s, want active/myws", ctx.source, ctx.wsName)
	}
}

func TestResolveUnknownWorkspaceName(t *testing.T) {
	setUserConfig(t, "", map[string]string{})
	setWorkspaceFlag(t, "nope")

	_, err := loadContext()
	if err == nil {
		t.Fatal("loadContext() with unknown workspace: want error")
	}
	if !strings.Contains(err.Error(), "nope") || !strings.Contains(err.Error(), "workspace list") {
		t.Errorf("error %q should name the workspace and suggest 'workspace list'", err)
	}
}

func TestResolveStaleWorkspacePath(t *testing.T) {
	staleDir := t.TempDir() // no metis.yaml inside
	setUserConfig(t, "", map[string]string{"stale": staleDir})
	setWorkspaceFlag(t, "stale")

	_, err := loadContext()
	if err == nil {
		t.Fatal("loadContext() with stale workspace path: want error")
	}
	if !strings.Contains(err.Error(), "stale") || !strings.Contains(err.Error(), "no metis.yaml") {
		t.Errorf("error %q should explain the stale path", err)
	}
}

func TestResolveNothingFound(t *testing.T) {
	setUserConfig(t, "", map[string]string{})
	setWorkspaceFlag(t, "")
	t.Setenv(envWorkspace, "")
	chdirOutsideProject(t)

	_, err := loadContext()
	if err == nil {
		t.Fatal("loadContext() with no project anywhere: want error")
	}
	if !strings.Contains(err.Error(), "metis init") || !strings.Contains(err.Error(), "workspace use") {
		t.Errorf("error %q should suggest both 'metis init' and 'metis workspace use'", err)
	}
}
