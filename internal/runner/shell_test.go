package runner

import (
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// A verify line is written for sh. Off Windows that is what runs it; on
// Windows the same line runs under the bash Git installs, when there is
// one, so a project's configuration does not fork by platform.
func TestShellFor(t *testing.T) {
	gitRoot := filepath.Join("C:", "Program Files", "Git")
	gitExe := filepath.Join(gitRoot, "cmd", "git.exe")
	gitBash := filepath.Join(gitRoot, "bin", "bash.exe")
	usrBash := filepath.Join(gitRoot, "usr", "bin", "bash.exe")
	noGit := func(string) (string, error) { return "", errors.New("not found") }
	withGit := func(string) (string, error) { return gitExe, nil }
	only := func(paths ...string) func(string) bool {
		return func(p string) bool {
			for _, q := range paths {
				if p == q {
					return true
				}
			}
			return false
		}
	}

	for name, tc := range map[string]struct {
		goos, configured string
		lookPath         func(string) (string, error)
		exists           func(string) bool
		want             Shell
	}{
		"linux":                   {"linux", "", noGit, only(), Shell{"sh", "-c"}},
		"darwin ignores override": {"darwin", "pwsh -Command", noGit, only(), Shell{"sh", "-c"}},
		"windows override":        {"windows", "pwsh -NoProfile -Command", withGit, only(gitBash), Shell{"pwsh", "-NoProfile", "-Command"}},
		"windows git bash":        {"windows", "", withGit, only(gitBash), Shell{gitBash, "-c"}},
		"windows usr bash":        {"windows", "", withGit, only(usrBash), Shell{usrBash, "-c"}},
		"windows git from mingw64 on PATH": {
			"windows", "", func(string) (string, error) { return filepath.Join(gitRoot, "mingw64", "bin", "git.exe"), nil }, only(gitBash), Shell{gitBash, "-c"},
		},
		"windows git from bin on PATH": {
			"windows", "", func(string) (string, error) { return filepath.Join(gitRoot, "bin", "git.exe"), nil }, only(usrBash), Shell{usrBash, "-c"},
		},
		"windows git without bash": {
			"windows", "", withGit, only(), Shell{"cmd", "/C"},
		},
		"windows no git": {"windows", "", noGit, only(gitBash), Shell{"cmd", "/C"}},
	} {
		t.Run(name, func(t *testing.T) {
			got := shellFor(tc.goos, tc.configured, tc.lookPath, tc.exists)
			if got.String() != tc.want.String() {
				t.Errorf("shellFor = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParseShell(t *testing.T) {
	if got := ParseShell("pwsh -NoProfile -Command"); got.String() != "pwsh -NoProfile -Command" {
		t.Errorf("ParseShell = %q", got)
	}
	if got := ParseShell("  "); got.String() != DefaultShell().String() {
		t.Errorf("blank should mean the default, got %q", got)
	}
}

// Whatever the platform default resolves to, it runs a command line and
// returns its output and exit code; this is the contract verify relies on.
func TestDefaultShellRunsACommand(t *testing.T) {
	result := Run("echo shell-ok", "", 0)
	if result.ExitCode != 0 || !strings.Contains(string(result.Output), "shell-ok") {
		t.Fatalf("exit %d output %q under %s", result.ExitCode, result.Output, DefaultShell())
	}
	if runtime.GOOS == "windows" {
		t.Logf("windows shell: %s", DefaultShell())
	}
}
