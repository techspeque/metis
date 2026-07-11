package config

import "fmt"

// Validate checks the configuration for structural errors.
// Returns a list of all validation errors found.
func (c *Config) Validate() []error {
	var errs []error

	// Version must be 1
	if c.Version != 1 {
		errs = append(errs, fmt.Errorf("unsupported config version: %d (expected 1)", c.Version))
	}

	// Project name required
	if c.Project.Name == "" {
		errs = append(errs, fmt.Errorf("project.name is required"))
	}

	// At least one agent must be defined
	if len(c.Agents) == 0 {
		errs = append(errs, fmt.Errorf("at least one agent must be defined in agents"))
	}

	// Validate agent entries
	for slug, agent := range c.Agents {
		if agent.Surface == "" {
			errs = append(errs, fmt.Errorf("agent %q: surface is required", slug))
		}
		if agent.Model == "" {
			errs = append(errs, fmt.Errorf("agent %q: model is required", slug))
		}
		if agent.Label == "" {
			errs = append(errs, fmt.Errorf("agent %q: label is required", slug))
		}
	}

	// Validate routing references — all slugs must exist in agents map
	errs = append(errs, c.validateRoutingSlugs("routing.high", c.Routing.High)...)
	errs = append(errs, c.validateRoutingSlugs("routing.medium", c.Routing.Medium)...)
	errs = append(errs, c.validateRoutingSlugs("routing.low", c.Routing.Low)...)

	// Verify command is required
	if c.Commands.Verify == "" {
		errs = append(errs, fmt.Errorf("commands.verify is required"))
	}

	// Commit prefixes must be non-empty
	if len(c.Commits.Prefixes) == 0 {
		errs = append(errs, fmt.Errorf("commits.prefixes must have at least one entry"))
	}

	// Commit format must be non-empty
	if c.Commits.Format == "" {
		errs = append(errs, fmt.Errorf("commits.format is required"))
	}

	// Paths validation — ledger must be set
	if c.Paths.Ledger == "" {
		errs = append(errs, fmt.Errorf("paths.ledger is required"))
	}

	return errs
}

// validateRoutingSlugs checks that all slugs in a routing list exist in the agents map.
func (c *Config) validateRoutingSlugs(field string, slugs []string) []error {
	var errs []error
	for _, slug := range slugs {
		if _, ok := c.Agents[slug]; !ok {
			errs = append(errs, fmt.Errorf("%s: agent %q not found in agents map", field, slug))
		}
	}
	return errs
}

// IsValid returns true if the config has no validation errors.
func (c *Config) IsValid() bool {
	return len(c.Validate()) == 0
}
