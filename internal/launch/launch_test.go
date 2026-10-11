package launch

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/techspeque/metis/internal/config"
)

func TestTemplateAndArgv(t *testing.T) {
	tmpl, err := Template(config.Agent{Surface: "claude-code", Model: "opus"})
	if err != nil {
		t.Fatal(err)
	}
	got := Argv(tmpl, Vars{Prompt: "do it; rm -rf /", Model: "opus", Slice: "s-1"})
	want := []string{"claude", "-p", "do it; rm -rf /", "--model", "opus", "--permission-mode", "acceptEdits"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Argv = %q", got)
	}

	own := config.Agent{Surface: "claude-code", Launch: []string{"wrap", "--file={prompt_file}", "--slice", "{slice}"}}
	tmpl, _ = Template(own)
	got = Argv(tmpl, Vars{PromptFile: "/tmp/p.md", Slice: "s-2"})
	if !reflect.DeepEqual(got, []string{"wrap", "--file=/tmp/p.md", "--slice", "s-2"}) {
		t.Errorf("an agent's own launch wins: %q", got)
	}

	if _, err := Template(config.Agent{Surface: "human"}); !errors.Is(err, ErrNoLauncher) {
		t.Errorf("human: %v", err)
	}
	if _, err := Template(config.Agent{Surface: "unknown"}); !errors.Is(err, ErrNoLauncher) {
		t.Errorf("unknown surface: %v", err)
	}
}

func TestDisplayElidesThePrompt(t *testing.T) {
	got := Display([]string{"claude", "-p", "a long prompt", "--model", "opus"}, "a long prompt")
	if strings.Contains(got, "a long prompt") || !strings.Contains(got, "<prompt: 13 bytes>") {
		t.Errorf("Display = %s", got)
	}
}

func TestNextSessionAndSessions(t *testing.T) {
	dir := t.TempDir()
	if NextSession(dir) != 1 {
		t.Error("an empty directory starts at 1")
	}
	for _, f := range []string{"1-coder.log", "1-coder.prompt.md", "2-reviewer.log", "10-coder.log", "escalation.md"} {
		if err := os.WriteFile(filepath.Join(dir, f), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if n := NextSession(dir); n != 11 {
		t.Errorf("NextSession = %d", n)
	}
	logs := Sessions(dir)
	if len(logs) != 3 || filepath.Base(logs[2]) != "10-coder.log" {
		t.Errorf("Sessions = %v", logs)
	}
}

func TestRunCapturesOutputAndExitCode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	log := filepath.Join(t.TempDir(), "s.log")
	res, err := Run(context.Background(), []string{"sh", "-c", "echo hello; exit 3"}, t.TempDir(), log, time.Minute, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 3 || res.TimedOut || !res.Failed() {
		t.Errorf("result = %+v", res)
	}
	if data, _ := os.ReadFile(log); !strings.Contains(string(data), "hello") {
		t.Errorf("log = %q", data)
	}
	if _, err := Run(context.Background(), []string{"no-such-agent-cli"}, t.TempDir(), log, time.Minute, nil); err == nil {
		t.Error("a command that cannot start is an error")
	}
}

// A timeout terminates the whole process group, children included.
func TestRunTimeoutKillsTheGroup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process groups are unix")
	}
	dir := t.TempDir()
	marker := filepath.Join(dir, "child-survived")
	script := "(sleep 2; touch " + marker + ") & sleep 30"
	start := time.Now()
	res, err := Run(context.Background(), []string{"sh", "-c", script}, dir, filepath.Join(dir, "s.log"), 300*time.Millisecond, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !res.TimedOut || !res.Failed() {
		t.Errorf("result = %+v", res)
	}
	if time.Since(start) > 10*time.Second {
		t.Errorf("the timeout took %s", time.Since(start))
	}
	time.Sleep(2500 * time.Millisecond)
	if _, err := os.Stat(marker); err == nil {
		t.Error("the agent's child process outlived the timeout")
	}
}
