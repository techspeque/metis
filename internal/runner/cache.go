package runner

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/techspeque/metis/internal/config"
	"github.com/techspeque/metis/internal/git"
)

// cacheEntries bounds the record of green runs.
const cacheEntries = 50

// Green is one passing verify, keyed by what it verified.
type Green struct {
	Key   string    `json:"key"`
	Tree  string    `json:"tree"`
	At    time.Time `json:"at"`
	Slice string    `json:"slice,omitempty"`
	Log   string    `json:"log,omitempty"`
}

// VerifyKey identifies what a verify run would verify: the working tree's
// content (ignored files and metis's own records excluded) and the
// commands that verify it. A different key is a different run.
func VerifyKey(cfg *config.Config, repoRoot string) (key, tree string, err error) {
	p := cfg.Paths
	tree, err = git.WorktreeTree(repoRoot, []string{p.Ledger, p.Archive, p.Briefs, p.Findings, p.Runs})
	if err != nil {
		return "", "", err
	}
	h := sha256.New()
	for _, part := range []string{tree, cfg.Commands.Verify, cfg.Commands.EnvCheck} {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), tree, nil
}

func cachePath(repoRoot string) (string, error) {
	return git.GitPath(repoRoot, filepath.Join("metis", "verify-cache.json"))
}

func readCache(path string) []Green {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var entries []Green
	if json.Unmarshal(data, &entries) != nil {
		return nil
	}
	return entries
}

// LookupGreen returns the recorded green run for key, if any.
func LookupGreen(repoRoot, key string) *Green {
	path, err := cachePath(repoRoot)
	if err != nil {
		return nil
	}
	for _, e := range readCache(path) {
		if e.Key == key {
			return &e
		}
	}
	return nil
}

// RecordGreen remembers a passing run, newest first, with a copy of its
// log beside the record: the slice's own log is rewritten by later runs.
func RecordGreen(repoRoot string, g *Green, log []byte) error {
	path, err := cachePath(repoRoot)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	logDir := filepath.Join(dir, "verify-logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return fmt.Errorf("verify cache: %w", err)
	}
	logPath := filepath.Join(logDir, g.Key+".log")
	if err := os.WriteFile(logPath, log, 0o644); err != nil {
		return fmt.Errorf("verify cache: %w", err)
	}
	g.Log = logPath
	if rel, err := filepath.Rel(repoRoot, logPath); err == nil {
		g.Log = filepath.ToSlash(rel)
	}

	entries := []Green{*g}
	for _, e := range readCache(path) {
		switch {
		case e.Key == g.Key:
		case len(entries) < cacheEntries:
			entries = append(entries, e)
		default:
			_ = os.Remove(filepath.Join(logDir, e.Key+".log"))
		}
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
