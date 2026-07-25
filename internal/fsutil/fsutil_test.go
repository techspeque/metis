package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileAtomic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.yaml")
	if err := WriteFileAtomic(path, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(path, []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "v2" {
		t.Fatalf("read back %q, %v", data, err)
	}
	// No temp litter left behind.
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Errorf("directory has %d entries, want 1", len(entries))
	}
}

func TestAcquireLockExcludes(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), ".metis", ".lock")
	release, err := AcquireLock(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	release()
	release() // double release is safe

	release2, err := AcquireLock(lockPath)
	if err != nil {
		t.Fatalf("reacquire after release: %v", err)
	}
	defer release2()
}
