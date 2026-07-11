package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/techspeque/metis/internal/config"
	"github.com/techspeque/metis/internal/ledger"
)

// context holds the loaded config and paths for CLI commands.
type context struct {
	cfg      *config.Config
	cfgPath  string
	repoRoot string
}

// loadContext finds and loads the metis.yaml config from the current directory.
func loadContext() (*context, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getting working directory: %w", err)
	}

	cfgPath, err := config.FindConfig(cwd)
	if err != nil {
		return nil, err
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, err
	}

	return &context{
		cfg:      cfg,
		cfgPath:  cfgPath,
		repoRoot: filepath.Dir(cfgPath),
	}, nil
}

// ledgerPath returns the absolute path to the ledger file.
func (c *context) ledgerPath() string {
	return filepath.Join(c.repoRoot, c.cfg.Paths.Ledger)
}

// archivePath returns the absolute path to the archive file.
func (c *context) archivePath() string {
	return filepath.Join(c.repoRoot, c.cfg.Paths.Archive)
}

// loadLedger loads the ledger from the configured path.
func (c *context) loadLedger() (*ledger.Ledger, error) {
	return ledger.Load(c.ledgerPath())
}

// saveLedger saves the ledger to the configured path.
func (c *context) saveLedger(l *ledger.Ledger) error {
	return l.Save(c.ledgerPath())
}

// loadArchive loads the archive from the configured path.
func (c *context) loadArchive() (*ledger.Archive, error) {
	return ledger.LoadArchive(c.archivePath())
}

// saveArchive saves the archive to the configured path.
func (c *context) saveArchive(a *ledger.Archive) error {
	return a.Save(c.archivePath())
}

// agentSlugs returns the set of valid agent slugs from the config.
func (c *context) agentSlugs() map[string]bool {
	slugs := make(map[string]bool)
	for slug := range c.cfg.Agents {
		slugs[slug] = true
	}
	return slugs
}

// overviewPath returns the absolute path to the overview file, or empty if not configured.
func (c *context) overviewPath() string {
	if c.cfg.Project.Overview == "" {
		return ""
	}
	return filepath.Join(c.repoRoot, c.cfg.Project.Overview)
}

// overviewHashPath returns the path to the stored overview hash file.
func (c *context) overviewHashPath() string {
	return filepath.Join(c.repoRoot, ".metis", "overview.hash")
}

// computeOverviewHash reads the overview file and returns its SHA256 hash.
// Returns empty string if overview is not configured or file doesn't exist.
func (c *context) computeOverviewHash() string {
	path := c.overviewPath()
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// storeOverviewHash writes the current overview hash to .metis/overview.hash.
func (c *context) storeOverviewHash() error {
	hash := c.computeOverviewHash()
	if hash == "" {
		return nil
	}
	dir := filepath.Dir(c.overviewHashPath())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(c.overviewHashPath(), []byte(hash), 0o644)
}

// checkOverviewDrift compares stored hash with current overview.
// Returns: "ok", "drifted", "no-baseline", or "not-configured".
func (c *context) checkOverviewDrift() string {
	if c.cfg.Project.Overview == "" {
		return "not-configured"
	}
	current := c.computeOverviewHash()
	if current == "" {
		return "not-configured"
	}
	stored, err := os.ReadFile(c.overviewHashPath())
	if err != nil {
		return "no-baseline"
	}
	if string(stored) == current {
		return "ok"
	}
	return "drifted"
}
