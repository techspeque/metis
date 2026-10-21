// Package runs manages verification run log storage in .metis/runs/.
// It handles writing timestamped logs and reading them back for review.
package runs

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Store manages run logs for slices.
type Store struct {
	BaseDir string // e.g., /repo/.metis/runs
}

// NewStore creates a store rooted at the given directory.
func NewStore(baseDir string) *Store {
	return &Store{BaseDir: baseDir}
}

// SliceDir returns the directory for a specific slice's runs.
func (s *Store) SliceDir(sliceID string) string {
	return filepath.Join(s.BaseDir, sliceID)
}

// Write writes a log file for the given slice and log name.
// logName should be like "env-check", "verify-pre", "verify-post".
//
// An earlier attempt is not overwritten when it carries information: a
// verify-* log, or any log whose run failed, is first moved aside as
// <name>.<n>.log with its exit code (see Attempts), so the failures that
// explain how a slice got to green survive the run that got it there.
func (s *Store) Write(sliceID, logName string, data []byte, exitCode int) error {
	dir := s.SliceDir(sliceID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating runs directory: %w", err)
	}
	if err := s.keepAttempt(sliceID, logName); err != nil {
		return err
	}

	logPath := filepath.Join(dir, logName+".log")
	if err := os.WriteFile(logPath, data, 0o644); err != nil {
		return fmt.Errorf("writing log %s: %w", logPath, err)
	}

	exitPath := filepath.Join(dir, logName+".exit")
	if err := os.WriteFile(exitPath, []byte(fmt.Sprintf("%d\n", exitCode)), 0o644); err != nil {
		return fmt.Errorf("writing exit code %s: %w", exitPath, err)
	}

	return nil
}

// keepAttempt moves the current <name>.log and .exit to the next numbered
// attempt when the log is worth keeping: every verify-* run, and every run
// that failed (a red env-check explains a stalled slice as well as a red
// verify does). A green env-check or interfaces log is replaced in place.
func (s *Store) keepAttempt(sliceID, logName string) error {
	dir := s.SliceDir(sliceID)
	logPath := filepath.Join(dir, logName+".log")
	if _, err := os.Stat(logPath); err != nil {
		return nil
	}
	_, exitCode, err := s.Read(sliceID, logName)
	if !strings.HasPrefix(logName, "verify-") && err == nil && exitCode == 0 {
		return nil
	}
	n := 1
	if kept := s.Attempts(sliceID, logName); len(kept) > 0 {
		last := kept[len(kept)-1]
		n, _ = strconv.Atoi(strings.TrimPrefix(last, logName+"."))
		n++
	}
	kept := fmt.Sprintf("%s.%d", logName, n)
	if err := os.Rename(logPath, filepath.Join(dir, kept+".log")); err != nil {
		return fmt.Errorf("keeping attempt %s: %w", kept, err)
	}
	exitPath := filepath.Join(dir, logName+".exit")
	if _, err := os.Stat(exitPath); err == nil {
		if err := os.Rename(exitPath, filepath.Join(dir, kept+".exit")); err != nil {
			return fmt.Errorf("keeping attempt %s: %w", kept, err)
		}
	}
	return nil
}

// Attempts returns the kept earlier attempts of a log, oldest first, as
// names Read accepts ("verify-post.1", "verify-post.2", ...). The current
// log stays under its plain name and is not listed.
func (s *Store) Attempts(sliceID, logName string) []string {
	entries, err := os.ReadDir(s.SliceDir(sliceID))
	if err != nil {
		return nil
	}
	var numbers []int
	prefix := logName + "."
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".log") {
			continue
		}
		n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(name, prefix), ".log"))
		if err != nil || n < 1 {
			continue
		}
		numbers = append(numbers, n)
	}
	sort.Ints(numbers)
	names := make([]string, 0, len(numbers))
	for _, n := range numbers {
		names = append(names, fmt.Sprintf("%s.%d", logName, n))
	}
	return names
}

// Read reads a log file for the given slice and log name.
// Returns the log content, exit code, and any error.
func (s *Store) Read(sliceID, logName string) ([]byte, int, error) {
	dir := s.SliceDir(sliceID)

	logPath := filepath.Join(dir, logName+".log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		return nil, 0, fmt.Errorf("reading log %s: %w", logPath, err)
	}

	exitPath := filepath.Join(dir, logName+".exit")
	exitData, err := os.ReadFile(exitPath)
	if err != nil {
		return data, 0, fmt.Errorf("reading exit code %s: %w", exitPath, err)
	}

	var exitCode int
	if _, err := fmt.Sscanf(string(exitData), "%d", &exitCode); err != nil {
		return data, 0, fmt.Errorf("parsing exit code %s: %w", exitPath, err)
	}
	return data, exitCode, nil
}

// Exists checks if a log file exists for the given slice and log name.
func (s *Store) Exists(sliceID, logName string) bool {
	logPath := filepath.Join(s.SliceDir(sliceID), logName+".log")
	_, err := os.Stat(logPath)
	return err == nil
}

// List returns all log names available for a slice, kept attempts included.
func (s *Store) List(sliceID string) ([]string, error) {
	dir := s.SliceDir(sliceID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var names []string
	for _, e := range entries {
		name := e.Name()
		if filepath.Ext(name) == ".log" {
			names = append(names, name[:len(name)-4])
		}
	}
	return names, nil
}
