// Package runs manages verification run log storage in .metis/runs/.
// It handles writing timestamped logs and reading them back for review.
package runs

import (
	"fmt"
	"os"
	"path/filepath"
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
func (s *Store) Write(sliceID, logName string, data []byte, exitCode int) error {
	dir := s.SliceDir(sliceID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating runs directory: %w", err)
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
	fmt.Sscanf(string(exitData), "%d", &exitCode)
	return data, exitCode, nil
}

// Exists checks if a log file exists for the given slice and log name.
func (s *Store) Exists(sliceID, logName string) bool {
	logPath := filepath.Join(s.SliceDir(sliceID), logName+".log")
	_, err := os.Stat(logPath)
	return err == nil
}

// List returns all log names available for a slice.
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
