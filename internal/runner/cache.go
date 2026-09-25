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

// RecordGreen remembers a passing run, newest first.
func RecordGreen(repoRoot string, g *Green) error {
	path, err := cachePath(repoRoot)
	if err != nil {
		return err
	}
	entries := []Green{*g}
	for _, e := range readCache(path) {
		if e.Key != g.Key && len(entries) < cacheEntries {
			entries = append(entries, e)
		}
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("verify cache: %w", err)
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
