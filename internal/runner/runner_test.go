package runner

import (
	"runtime"
	"testing"
)

func TestRun_Success(t *testing.T) {
	result := Run("echo hello", "")
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
	if string(result.Output) != "hello\n" {
		t.Errorf("Output = %q, want %q", string(result.Output), "hello\n")
	}
}

func TestRun_Failure(t *testing.T) {
	result := Run("exit 1", "")
	if result.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", result.ExitCode)
	}
}

func TestRun_SpecificExitCode(t *testing.T) {
	result := Run("exit 42", "")
	if result.ExitCode != 42 {
		t.Errorf("ExitCode = %d, want 42", result.ExitCode)
	}
}

func TestRun_CapturesStderr(t *testing.T) {
	result := Run("echo err >&2", "")
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
	if string(result.Output) != "err\n" {
		t.Errorf("Output = %q, want stderr captured", string(result.Output))
	}
}

func TestRun_CombinedOutput(t *testing.T) {
	result := Run("echo out && echo err >&2", "")
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
	// Both stdout and stderr should be captured
	out := string(result.Output)
	if len(out) == 0 {
		t.Error("expected combined output")
	}
}

func TestRun_WorkingDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("pwd not available on windows")
	}
	result := Run("pwd", "/tmp")
	if result.ExitCode != 0 {
		t.Fatalf("ExitCode = %d", result.ExitCode)
	}
	// /tmp might be a symlink to /private/tmp on macOS
	out := string(result.Output)
	if out != "/tmp\n" && out != "/private/tmp\n" {
		t.Errorf("Output = %q, want /tmp or /private/tmp", out)
	}
}

func TestResult_FormatLog(t *testing.T) {
	result := &Result{
		Command:  "echo test",
		Output:   []byte("test output"),
		ExitCode: 0,
	}
	log := result.FormatLog("feat-0001")
	logStr := string(log)

	if len(logStr) == 0 {
		t.Fatal("FormatLog returned empty")
	}
	// Should contain the command
	if !contains(logStr, "echo test") {
		t.Error("log should contain command")
	}
	// Should contain the slice ID
	if !contains(logStr, "feat-0001") {
		t.Error("log should contain slice ID")
	}
	// Should contain exit code
	if !contains(logStr, "exit code: 0") {
		t.Error("log should contain exit code")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
