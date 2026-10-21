package runs

import (
	"os"
	"path/filepath"
	"testing"
)

// One red log survived in a hundred and twenty slice directories because
// every verify overwrote the last. A verify log is never overwritten: the
// earlier attempt moves to <name>.<n>.log with its exit code.
func TestVerifyAttemptsAreKept(t *testing.T) {
	store := NewStore(t.TempDir())

	if err := store.Write("feat-0001", "verify-post", []byte("FAIL: TestA\n"), 1); err != nil {
		t.Fatal(err)
	}
	if err := store.Write("feat-0001", "verify-post", []byte("FAIL: TestB\n"), 1); err != nil {
		t.Fatal(err)
	}
	if err := store.Write("feat-0001", "verify-post", []byte("ok\n"), 0); err != nil {
		t.Fatal(err)
	}

	// The plain name is the latest, as every reader of verify-post expects.
	data, code, err := store.Read("feat-0001", "verify-post")
	if err != nil || code != 0 || string(data) != "ok\n" {
		t.Fatalf("latest = %q exit %d err %v", data, code, err)
	}

	attempts := store.Attempts("feat-0001", "verify-post")
	if len(attempts) != 2 || attempts[0] != "verify-post.1" || attempts[1] != "verify-post.2" {
		t.Fatalf("attempts = %v", attempts)
	}
	for i, want := range []string{"FAIL: TestA\n", "FAIL: TestB\n"} {
		data, code, err := store.Read("feat-0001", attempts[i])
		if err != nil || code != 1 || string(data) != want {
			t.Errorf("%s = %q exit %d err %v, want %q exit 1", attempts[i], data, code, err, want)
		}
	}
	if got := store.Attempts("feat-0001", "verify-pre"); len(got) != 0 {
		t.Error("attempts leaked across log names")
	}
}

// A green env-check runs before every verify and says nothing new; it is
// replaced in place. A red one is a fact about the machine and is kept.
func TestOnlyFailedNonVerifyLogsAreKept(t *testing.T) {
	store := NewStore(t.TempDir())
	_ = store.Write("feat-0001", "env-check", []byte("ok\n"), 0)
	_ = store.Write("feat-0001", "env-check", []byte("ok again\n"), 0)
	if got := store.Attempts("feat-0001", "env-check"); len(got) != 0 {
		t.Errorf("green env-check kept as %v", got)
	}

	_ = store.Write("feat-0001", "env-check", []byte("docker down\n"), 2)
	_ = store.Write("feat-0001", "env-check", []byte("ok\n"), 0)
	got := store.Attempts("feat-0001", "env-check")
	if len(got) != 1 {
		t.Fatalf("red env-check not kept: %v", got)
	}
	data, code, _ := store.Read("feat-0001", got[0])
	if code != 2 || string(data) != "docker down\n" {
		t.Errorf("kept attempt = %q exit %d", data, code)
	}
}

// Numbering continues from the highest kept attempt, so a deleted middle
// attempt never makes a later write overwrite an earlier one.
func TestAttemptNumbersNeverCollide(t *testing.T) {
	store := NewStore(t.TempDir())
	for i := 0; i < 4; i++ {
		_ = store.Write("feat-0001", "verify-post", []byte("x\n"), 1)
	}
	if err := os.Remove(filepath.Join(store.SliceDir("feat-0001"), "verify-post.2.log")); err != nil {
		t.Fatal(err)
	}
	_ = store.Write("feat-0001", "verify-post", []byte("y\n"), 0)
	got := store.Attempts("feat-0001", "verify-post")
	if len(got) != 3 || got[2] != "verify-post.4" {
		t.Errorf("attempts = %v, want the new one numbered 4", got)
	}
}
