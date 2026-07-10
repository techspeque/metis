// Package config handles loading and validating the metis.yaml configuration.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the full metis.yaml configuration.
type Config struct {
	Version         int              `yaml:"version"`
	Project         ProjectConfig    `yaml:"project"`
	Agents          map[string]Agent `yaml:"agents"`
	Routing         RoutingConfig    `yaml:"routing"`
	HotPaths        []string         `yaml:"hot_paths,omitempty"`
	AccuracyRules   []string         `yaml:"accuracy_rules,omitempty"`
	NonGoals        []string         `yaml:"non_goals,omitempty"`
	Testing         []string         `yaml:"testing,omitempty"`
	ReviewChecklist []string         `yaml:"review_checklist,omitempty"`
	Commands        CommandsConfig   `yaml:"commands"`
	Commits         CommitsConfig    `yaml:"commits"`
	Paths           PathsConfig      `yaml:"paths"`
}

// ProjectConfig holds project-level settings.
type ProjectConfig struct {
	Name              string           `yaml:"name"`
	IntegrationBranch string           `yaml:"integration_branch"`
	ReleaseBranch     string           `yaml:"release_branch"`
	Technology        TechnologyConfig `yaml:"technology,omitempty"`
}

// TechnologyConfig holds informational technology details.
type TechnologyConfig struct {
	Language    string `yaml:"language,omitempty"`
	BuildSystem string `yaml:"build_system,omitempty"`
	TestRunner  string `yaml:"test_runner,omitempty"`
	Linter      string `yaml:"linter,omitempty"`
}

// Agent describes one configured agent identity.
type Agent struct {
	Surface string `yaml:"surface"`
	Model   string `yaml:"model"`
	Label   string `yaml:"label"`
}

// RoutingConfig determines which agents handle which risk levels.
type RoutingConfig struct {
	High   []string `yaml:"high,omitempty"`
	Medium []string `yaml:"medium,omitempty"`
	Low    []string `yaml:"low,omitempty"`
	Review string   `yaml:"review"`
}

// CommandsConfig holds the technology-specific commands Metis wraps.
type CommandsConfig struct {
	Verify     string `yaml:"verify"`
	EnvCheck   string `yaml:"env_check,omitempty"`
	Interfaces string `yaml:"interfaces,omitempty"`
}

// CommitsConfig holds the commit convention settings.
type CommitsConfig struct {
	Prefixes       []string `yaml:"prefixes"`
	RequireSliceID bool     `yaml:"require_slice_id"`
	NoAttribution  bool     `yaml:"no_attribution"`
	Format         string   `yaml:"format"`
}

// PathsConfig holds the locations of Metis state files.
type PathsConfig struct {
	Ledger     string `yaml:"ledger"`
	Archive    string `yaml:"archive"`
	Briefs     string `yaml:"briefs"`
	Findings   string `yaml:"findings"`
	Runs       string `yaml:"runs"`
	Interfaces string `yaml:"interfaces"`
}

// DefaultConfig returns a Config with all default values applied.
// These match Appendix C of the spec.
func DefaultConfig() Config {
	return Config{
		Version: 1,
		Project: ProjectConfig{
			IntegrationBranch: "dev",
			ReleaseBranch:     "main",
		},
		Agents:  make(map[string]Agent),
		Routing: RoutingConfig{Review: "cross-vendor"},
		Commits: CommitsConfig{
			Prefixes:       []string{"feat", "fix", "refactor", "docs", "test", "chore"},
			RequireSliceID: true,
			NoAttribution:  true,
			Format:         "{prefix}({slice_id}): {message}",
		},
		Paths: PathsConfig{
			Ledger:     ".metis/slices.yaml",
			Archive:    ".metis/slices-done.yaml",
			Briefs:     ".metis/briefs/",
			Findings:   ".metis/findings.yaml",
			Runs:       ".metis/runs/",
			Interfaces: "docs/generated/interfaces.txt",
		},
	}
}

// Load reads and parses a metis.yaml file, applying defaults for missing fields.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	return Parse(data)
}

// Parse parses metis.yaml content from bytes, applying defaults for missing fields.
func Parse(data []byte) (*Config, error) {
	cfg := DefaultConfig()

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	// Ensure agents map is not nil even if empty in YAML
	if cfg.Agents == nil {
		cfg.Agents = make(map[string]Agent)
	}

	return &cfg, nil
}

// FindConfig searches for metis.yaml starting from the given directory,
// walking up to the repository root. Returns the path if found.
func FindConfig(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", fmt.Errorf("resolving path: %w", err)
	}

	for {
		candidate := filepath.Join(dir, "metis.yaml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}

		// Stop at filesystem root
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("metis.yaml not found (searched from %s to filesystem root)", startDir)
}

// LoadFromDir finds and loads metis.yaml starting from the given directory.
func LoadFromDir(dir string) (*Config, error) {
	path, err := FindConfig(dir)
	if err != nil {
		return nil, err
	}
	return Load(path)
}
