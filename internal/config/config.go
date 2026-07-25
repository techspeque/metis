// Package config handles loading and validating the .metis/project.yaml configuration.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the full .metis/project.yaml configuration.
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

// FileName is the project configuration file, relative to the repo root.
// It lives inside .metis/ (project.yaml) alongside the rest of the project
// state; the file is distinct from the user-level ~/.metis/config.yaml.
const FileName = ".metis/project.yaml"

// LegacyFileName is the pre-v0.0.5 location at the repo root. Discovery
// still accepts it (with a deprecation warning); 'metis init' migrates it.
const LegacyFileName = "metis.yaml"

// IsLegacyPath reports whether a discovered config path uses the deprecated
// repo-root location.
func IsLegacyPath(path string) bool {
	return filepath.Base(filepath.Dir(path)) != ".metis"
}

// RootFromConfigPath returns the repo root for a discovered config path,
// handling both the current (.metis/project.yaml) and legacy (root
// metis.yaml) locations.
func RootFromConfigPath(cfgPath string) string {
	dir := filepath.Dir(cfgPath)
	if filepath.Base(dir) == ".metis" {
		return filepath.Dir(dir)
	}
	return dir
}

// FindConfigIn returns the config path directly under the given repo root,
// preferring the current location over the legacy one.
func FindConfigIn(root string) (string, error) {
	current := filepath.Join(root, FileName)
	if _, err := os.Stat(current); err == nil {
		return current, nil
	}
	legacy := filepath.Join(root, LegacyFileName)
	if _, err := os.Stat(legacy); err == nil {
		return legacy, nil
	}
	return "", fmt.Errorf("no %s (or legacy %s) found at %s", FileName, LegacyFileName, root)
}

// Load reads and parses a project configuration file, applying defaults for
// missing fields.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	return Parse(data)
}

// Parse parses project configuration content from bytes, applying defaults
// for missing fields.
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

// FindConfig searches for the project configuration starting from the given
// directory, walking up to the filesystem root. At each level the current
// location (.metis/project.yaml) is preferred over the legacy root metis.yaml.
func FindConfig(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", fmt.Errorf("resolving path: %w", err)
	}

	for {
		if path, err := FindConfigIn(dir); err == nil {
			return path, nil
		}

		// Stop at filesystem root
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("%s not found (searched from %s to filesystem root)", FileName, startDir)
}

// LoadFromDir finds and loads the project configuration starting from the
// given directory.
func LoadFromDir(dir string) (*Config, error) {
	path, err := FindConfig(dir)
	if err != nil {
		return nil, err
	}
	return Load(path)
}
