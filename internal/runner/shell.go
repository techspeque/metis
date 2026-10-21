package runner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Shell is the interpreter a configured command line runs under: the
// program and the arguments that precede the command, e.g. sh -c.
type Shell []string

// ParseShell reads a configured shell such as "pwsh -NoProfile -Command".
// Empty means the platform default.
func ParseShell(configured string) Shell {
	if fields := strings.Fields(configured); len(fields) > 0 {
		return Shell(fields)
	}
	return DefaultShell()
}

// DefaultShell is the shell for this platform. Everywhere but Windows it
// is sh -c. On Windows it is METIS_SHELL when set, else the bash that Git
// for Windows installs beside git.exe (so a project's verify line written
// for sh keeps working), else cmd /C.
func DefaultShell() Shell {
	return shellFor(runtime.GOOS, os.Getenv("METIS_SHELL"), exec.LookPath, func(p string) bool {
		_, err := os.Stat(p)
		return err == nil
	})
}

// shellFor is DefaultShell with its platform inputs injected.
func shellFor(goos, configured string, lookPath func(string) (string, error), exists func(string) bool) Shell {
	if goos != "windows" {
		return Shell{"sh", "-c"}
	}
	if fields := strings.Fields(configured); len(fields) > 0 {
		return Shell(fields)
	}
	if gitPath, err := lookPath("git"); err == nil {
		// Git for Windows keeps bash at <root>\bin\bash.exe and
		// <root>\usr\bin\bash.exe, and git.exe one or two levels below the
		// root depending on which of its directories is on PATH (cmd\,
		// bin\, mingw64\bin\). Either bash is the one a verify line written
		// for sh expects. The bash.exe on PATH may instead be the WSL
		// launcher, which runs nothing in this tree, so it is not used.
		dir := filepath.Dir(gitPath)
		for level := 0; level < 3; level++ {
			dir = filepath.Dir(dir)
			for _, rel := range []string{"bin/bash.exe", "usr/bin/bash.exe"} {
				if candidate := filepath.Join(dir, filepath.FromSlash(rel)); exists(candidate) {
					return Shell{candidate, "-c"}
				}
			}
		}
	}
	return Shell{"cmd", "/C"}
}

// Command builds the process that runs command under the shell in dir.
func (s Shell) Command(ctx context.Context, command, dir string) *exec.Cmd {
	if len(s) == 0 {
		s = DefaultShell()
	}
	args := append(append([]string{}, s[1:]...), command)
	cmd := exec.CommandContext(ctx, s[0], args...)
	cmd.Dir = dir
	return cmd
}

// String renders the shell for a log line.
func (s Shell) String() string {
	return strings.Join(s, " ")
}
