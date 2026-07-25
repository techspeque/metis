// Package fsutil provides crash-safe file writes and inter-process locking
// for the .metis/ state files. Multiple metis processes (concurrent agent
// sessions) may mutate the same repository; every state write must therefore
// be atomic (write-temp-then-rename) and every read-modify-write sequence
// must hold the repository lock.
package fsutil

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
)

// WriteFileAtomic writes data to path via a temp file in the same directory
// followed by an atomic rename, so a crash mid-write can never leave a
// truncated file behind.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }() // no-op after successful rename

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("syncing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return fmt.Errorf("setting permissions: %w", err)
	}
	return os.Rename(tmpName, path)
}

// LockTimeout bounds how long a process waits for the repository lock.
const LockTimeout = 10 * time.Second

// AcquireLock takes the exclusive repository lock at lockPath (creating
// parent directories as needed), waiting up to LockTimeout. The returned
// release function is safe to call multiple times.
func AcquireLock(lockPath string) (release func(), err error) {
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return nil, fmt.Errorf("creating lock directory: %w", err)
	}

	fl := flock.New(lockPath)
	deadline := time.Now().Add(LockTimeout)
	for {
		ok, err := fl.TryLock()
		if err != nil {
			return nil, fmt.Errorf("acquiring repository lock: %w", err)
		}
		if ok {
			released := false
			return func() {
				if !released {
					released = true
					_ = fl.Unlock()
				}
			}, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("repository lock at %s held by another metis process for over %s — retry, or remove the file if no other metis is running", lockPath, LockTimeout)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
