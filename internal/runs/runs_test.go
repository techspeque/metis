package runs

import (
	"testing"
)

func TestStore_WriteAndRead(t *testing.T) {
	tmp := t.TempDir()
	store := NewStore(tmp)

	data := []byte("test log content\n")
	if err := store.Write("feat-0001", "verify-post", data, 0); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	readData, exitCode, err := store.Read("feat-0001", "verify-post")
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if string(readData) != string(data) {
		t.Errorf("Read data = %q, want %q", string(readData), string(data))
	}
	if exitCode != 0 {
		t.Errorf("exit code = %d, want 0", exitCode)
	}
}

func TestStore_WriteNonZeroExit(t *testing.T) {
	tmp := t.TempDir()
	store := NewStore(tmp)

	if err := store.Write("fix-0001", "env-check", []byte("failed\n"), 2); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	_, exitCode, err := store.Read("fix-0001", "env-check")
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if exitCode != 2 {
		t.Errorf("exit code = %d, want 2", exitCode)
	}
}

func TestStore_Exists(t *testing.T) {
	tmp := t.TempDir()
	store := NewStore(tmp)

	if store.Exists("feat-0001", "verify-post") {
		t.Error("should not exist before write")
	}

	if err := store.Write("feat-0001", "verify-post", []byte("ok"), 0); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	if !store.Exists("feat-0001", "verify-post") {
		t.Error("should exist after write")
	}
}

func TestStore_List(t *testing.T) {
	tmp := t.TempDir()
	store := NewStore(tmp)

	// Empty
	names, err := store.List("feat-0001")
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(names) != 0 {
		t.Errorf("expected empty list, got %v", names)
	}

	// Write a few logs
	store.Write("feat-0001", "env-check", []byte("ok"), 0)
	store.Write("feat-0001", "verify-pre", []byte("ok"), 0)
	store.Write("feat-0001", "verify-post", []byte("ok"), 0)

	names, err = store.List("feat-0001")
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(names) != 3 {
		t.Errorf("expected 3 logs, got %d: %v", len(names), names)
	}
}

func TestStore_ReadNotFound(t *testing.T) {
	tmp := t.TempDir()
	store := NewStore(tmp)

	_, _, err := store.Read("nonexistent", "verify-post")
	if err == nil {
		t.Error("expected error for non-existent log")
	}
}
