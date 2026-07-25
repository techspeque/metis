// Package userconfig handles the per-user configuration at ~/.metis/config.yaml.
// It holds the workspace registry and the active workspace selection used by
// the human persona; project resolution from cwd always takes precedence.
package userconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/techspeque/metis/internal/fsutil"
)

// EnvPath overrides the user config location when set (used by tests and
// exotic setups).
const EnvPath = "METIS_USER_CONFIG"

// UserConfig represents ~/.metis/config.yaml.
type UserConfig struct {
	// Active is the name of the currently selected workspace, or empty.
	Active string `yaml:"active,omitempty"`
	// Workspaces maps workspace names to absolute project root paths.
	Workspaces map[string]string `yaml:"workspaces,omitempty"`
}

// Path returns the location of the user config file (~/.metis/config.yaml).
// The name is deliberately distinct from the project config
// (.metis/project.yaml): ~/.metis/ is on the upward-discovery path for repos
// under ~, and project resolution must never latch onto the user config.
func Path() (string, error) {
	if override := os.Getenv(EnvPath); override != "" {
		return override, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	return filepath.Join(home, ".metis", "config.yaml"), nil
}

// Load reads the user config. A missing file is not an error — it returns an
// empty config, since nothing exists until the user (or metis init) writes it.
func Load() (*UserConfig, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &UserConfig{Workspaces: map[string]string{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading user config %s: %w", path, err)
	}

	var uc UserConfig
	if err := yaml.Unmarshal(data, &uc); err != nil {
		return nil, fmt.Errorf("parsing user config %s: %w", path, err)
	}
	if uc.Workspaces == nil {
		uc.Workspaces = map[string]string{}
	}
	return &uc, nil
}

// Save writes the user config, creating ~/.metis/ if needed. Workspace keys
// are emitted in stable (sorted) order by yaml.Marshal on the map.
func (uc *UserConfig) Save() error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating user config directory: %w", err)
	}

	data, err := yaml.Marshal(uc)
	if err != nil {
		return fmt.Errorf("marshaling user config: %w", err)
	}

	header := "# ~/.metis/config.yaml — user-level configuration for Metis\n"
	content := header + strings.TrimRight(string(data), "\n") + "\n"
	return fsutil.WriteFileAtomic(path, []byte(content), 0o644)
}

// Add registers a workspace. The path must be absolute; the name must not
// already be registered with a different path.
func (uc *UserConfig) Add(name, path string) error {
	if name == "" {
		return fmt.Errorf("workspace name cannot be empty")
	}
	if !filepath.IsAbs(path) {
		return fmt.Errorf("workspace path must be absolute: %s", path)
	}
	if existing, ok := uc.Workspaces[name]; ok && existing != path {
		return fmt.Errorf("workspace %q already registered at %s", name, existing)
	}
	uc.Workspaces[name] = path
	return nil
}

// Remove unregisters a workspace. If it was the active workspace, the active
// selection is cleared. The project itself is never touched.
func (uc *UserConfig) Remove(name string) error {
	if _, ok := uc.Workspaces[name]; !ok {
		return fmt.Errorf("workspace %q is not registered", name)
	}
	delete(uc.Workspaces, name)
	if uc.Active == name {
		uc.Active = ""
	}
	return nil
}

// Use sets the active workspace.
func (uc *UserConfig) Use(name string) error {
	if _, ok := uc.Workspaces[name]; !ok {
		return fmt.Errorf("workspace %q is not registered (see 'metis workspace list')", name)
	}
	uc.Active = name
	return nil
}

// Names returns the registered workspace names in sorted order.
func (uc *UserConfig) Names() []string {
	names := make([]string, 0, len(uc.Workspaces))
	for name := range uc.Workspaces {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
