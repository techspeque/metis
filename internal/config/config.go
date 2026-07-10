// Package config handles loading and validating the metis.yaml configuration.
package config

// Config represents the full metis.yaml configuration.
type Config struct {
	Version int            `yaml:"version"`
	Project ProjectConfig  `yaml:"project"`
	Agents  map[string]Agent `yaml:"agents"`
	Routing RoutingConfig  `yaml:"routing"`
	HotPaths []string      `yaml:"hot_paths,omitempty"`
	AccuracyRules []string `yaml:"accuracy_rules,omitempty"`
	NonGoals []string      `yaml:"non_goals,omitempty"`
	Testing  []string      `yaml:"testing,omitempty"`
	ReviewChecklist []string `yaml:"review_checklist,omitempty"`
	Commands CommandsConfig `yaml:"commands"`
	Commits  CommitsConfig  `yaml:"commits"`
	Paths    PathsConfig    `yaml:"paths"`
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
