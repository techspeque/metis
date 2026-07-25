package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/techspeque/metis/internal/config"
	"github.com/techspeque/metis/internal/fsutil"
	"github.com/techspeque/metis/internal/ledger"
	"github.com/techspeque/metis/internal/userconfig"
)

// repoLockRelease releases the repository lock acquired by loadContext.
// Execute calls releaseRepoLock after the command finishes; commands that
// never load a context never take the lock.
var repoLockRelease func()

func releaseRepoLock() {
	if repoLockRelease != nil {
		repoLockRelease()
		repoLockRelease = nil
	}
}

// Workspace resolution sources, in precedence order.
const (
	sourceFlag   = "flag"   // --workspace <name>
	sourceEnv    = "env"    // METIS_WORKSPACE
	sourceCwd    = "cwd"    // upward discovery from the working directory
	sourceActive = "active" // active workspace in ~/.metis/config.yaml
)

// envWorkspace selects a registered workspace by name, overriding cwd discovery.
const envWorkspace = "METIS_WORKSPACE"

// context holds the loaded config and paths for CLI commands.
type context struct {
	cfg      *config.Config
	cfgPath  string
	repoRoot string
	// source records how the workspace was resolved (sourceFlag, sourceEnv,
	// sourceCwd, or sourceActive); wsName is the registry name when the
	// resolution went through the user config.
	source string
	wsName string
}

// loadContext resolves the project to operate on, in precedence order:
// --workspace flag, METIS_WORKSPACE env, upward discovery of metis.yaml from
// cwd, then the active workspace from the user config. Cwd discovery beating
// the active workspace is deliberate: agents share the user config with the
// human, and switching workspaces must never redirect an agent running
// inside a repo.
func loadContext() (*context, error) {
	cfgPath, source, wsName, err := resolveConfigPath()
	if err != nil {
		return nil, err
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, err
	}

	repoRoot := config.RootFromConfigPath(cfgPath)

	// Serialize concurrent metis processes on this repository: every
	// command's load→mutate→save sequence runs under the lock, so two
	// agent sessions can never lose each other's ledger updates.
	if repoLockRelease == nil {
		release, err := fsutil.AcquireLock(filepath.Join(repoRoot, ".metis", ".lock"))
		if err != nil {
			return nil, err
		}
		repoLockRelease = release
	}

	ctx := &context{
		cfg:      cfg,
		cfgPath:  cfgPath,
		repoRoot: repoRoot,
		source:   source,
		wsName:   wsName,
	}

	if config.IsLegacyPath(cfgPath) {
		fmt.Fprintf(os.Stderr, "warning: %s at the repo root is deprecated — run 'metis init' to migrate it to %s\n",
			config.LegacyFileName, config.FileName)
	}

	// Anything other than cwd discovery is a redirection the user should see.
	if source != sourceCwd {
		via := map[string]string{
			sourceFlag:   "--workspace",
			sourceEnv:    envWorkspace,
			sourceActive: "active",
		}[source]
		fmt.Fprintf(os.Stderr, "workspace: %s (%s) [via %s]\n", wsName, ctx.repoRoot, via)
	}

	return ctx, nil
}

// resolveConfigPath returns the metis.yaml path, the resolution source, and
// the workspace name (when resolved via the registry).
func resolveConfigPath() (string, string, string, error) {
	if workspaceFlag != "" {
		path, err := workspacePath(workspaceFlag)
		return path, sourceFlag, workspaceFlag, err
	}

	if name := os.Getenv(envWorkspace); name != "" {
		path, err := workspacePath(name)
		return path, sourceEnv, name, err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", "", "", fmt.Errorf("getting working directory: %w", err)
	}
	if cfgPath, err := config.FindConfig(cwd); err == nil {
		return cfgPath, sourceCwd, "", nil
	}

	uc, err := userconfig.Load()
	if err != nil {
		return "", "", "", err
	}
	if uc.Active != "" {
		path, err := workspacePath(uc.Active)
		return path, sourceActive, uc.Active, err
	}

	return "", "", "", fmt.Errorf(
		"no metis project found (searched from %s to filesystem root, no active workspace) — run 'metis init' here, or 'metis workspace use <name>'",
		cwd)
}

// workspacePath resolves a registered workspace name to its project config path.
func workspacePath(name string) (string, error) {
	uc, err := userconfig.Load()
	if err != nil {
		return "", err
	}
	root, ok := uc.Workspaces[name]
	if !ok {
		return "", fmt.Errorf("workspace %q is not registered (see 'metis workspace list')", name)
	}
	cfgPath, err := config.FindConfigIn(root)
	if err != nil {
		return "", fmt.Errorf("workspace %q points to %s, but no %s found there (see 'metis workspace list')", name, root, config.FileName)
	}
	return cfgPath, nil
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
