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
	Version         int              `yaml:"version" json:"version"`
	Project         ProjectConfig    `yaml:"project" json:"project"`
	Agents          map[string]Agent `yaml:"agents" json:"agents"`
	Routing         RoutingConfig    `yaml:"routing" json:"routing"`
	HotPaths        []string         `yaml:"hot_paths,omitempty" json:"hot_paths,omitempty"`
	AccuracyRules   []string         `yaml:"accuracy_rules,omitempty" json:"accuracy_rules,omitempty"`
	NonGoals        []string         `yaml:"non_goals,omitempty" json:"non_goals,omitempty"`
	Testing         []string         `yaml:"testing,omitempty" json:"testing,omitempty"`
	ReviewChecklist []string         `yaml:"review_checklist,omitempty" json:"review_checklist,omitempty"`
	Commands        CommandsConfig   `yaml:"commands" json:"commands"`
	Commits         CommitsConfig    `yaml:"commits" json:"commits"`
	Paths           PathsConfig      `yaml:"paths" json:"paths"`
}

// ProjectConfig holds project-level settings.
type ProjectConfig struct {
	Name              string           `yaml:"name" json:"name"`
	IntegrationBranch string           `yaml:"integration_branch" json:"integration_branch"`
	ReleaseBranch     string           `yaml:"release_branch" json:"release_branch"`
	Overview          string           `yaml:"overview,omitempty" json:"overview,omitempty"`
	Technology        TechnologyConfig `yaml:"technology,omitempty" json:"technology,omitempty"`
}

// TechnologyConfig holds informational technology details.
type TechnologyConfig struct {
	Language    string `yaml:"language,omitempty" json:"language,omitempty"`
	BuildSystem string `yaml:"build_system,omitempty" json:"build_system,omitempty"`
	TestRunner  string `yaml:"test_runner,omitempty" json:"test_runner,omitempty"`
	Linter      string `yaml:"linter,omitempty" json:"linter,omitempty"`
}

// Agent describes one configured agent identity.
type Agent struct {
	Surface string `yaml:"surface" json:"surface"`
	Model   string `yaml:"model" json:"model"`
	Label   string `yaml:"label" json:"label"`
}

// RoutingConfig determines which agents handle which risk levels.
type RoutingConfig struct {
	High   []string `yaml:"high,omitempty" json:"high,omitempty"`
	Medium []string `yaml:"medium,omitempty" json:"medium,omitempty"`
	Low    []string `yaml:"low,omitempty" json:"low,omitempty"`
	Review string   `yaml:"review" json:"review"`
}

// CommandsConfig holds the technology-specific commands Metis wraps.
type CommandsConfig struct {
	Verify     string `yaml:"verify" json:"verify"`
	EnvCheck   string `yaml:"env_check,omitempty" json:"env_check,omitempty"`
	Interfaces string `yaml:"interfaces,omitempty" json:"interfaces,omitempty"`
}

// CommitsConfig holds the commit convention settings.
type CommitsConfig struct {
	Prefixes       []string `yaml:"prefixes" json:"prefixes"`
	RequireSliceID bool     `yaml:"require_slice_id" json:"require_slice_id"`
	NoAttribution  bool     `yaml:"no_attribution" json:"no_attribution"`
	Format         string   `yaml:"format" json:"format"`
}

// PathsConfig holds the locations of Metis state files.
type PathsConfig struct {
	Ledger     string `yaml:"ledger" json:"ledger"`
	Archive    string `yaml:"archive" json:"archive"`
	Briefs     string `yaml:"briefs" json:"briefs"`
	Plans      string `yaml:"plans" json:"plans"`
	ADR        string `yaml:"adr" json:"adr"`
	Findings   string `yaml:"findings" json:"findings"`
	Runs       string `yaml:"runs" json:"runs"`
	Interfaces string `yaml:"interfaces" json:"interfaces"`
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
			Plans:      ".metis/plans/",
			ADR:        ".metis/adr/",
			Findings:   ".metis/findings.yaml",
			Runs:       ".metis/runs/",
			Interfaces: ".metis/interfaces.txt",
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
