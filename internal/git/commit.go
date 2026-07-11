package git

import (
	"fmt"
	"strings"

	"github.com/techspeque/metis/internal/config"
	"github.com/techspeque/metis/internal/slice"
)

// FormatCommitMessage creates a commit message following the configured format.
// Default format: {prefix}({slice_id}): {message}
func FormatCommitMessage(cfg *config.Config, sliceID, prefix, message string) string {
	format := cfg.Commits.Format
	if format == "" {
		format = "{prefix}({slice_id}): {message}"
	}

	r := strings.NewReplacer(
		"{prefix}", prefix,
		"{slice_id}", sliceID,
		"{message}", message,
	)
	return r.Replace(format)
}

// InferPrefix returns the default commit prefix for a given work type.
func InferPrefix(wt slice.WorkType) string {
	return wt.CommitPrefix()
}

// ValidatePrefix checks if a prefix is in the allowed list.
func ValidatePrefix(cfg *config.Config, prefix string) error {
	for _, p := range cfg.Commits.Prefixes {
		if p == prefix {
			return nil
		}
	}
	return fmt.Errorf("invalid commit prefix %q (allowed: %s)", prefix, strings.Join(cfg.Commits.Prefixes, ", "))
}

// StripAttribution removes AI attribution lines from a commit message.
// Removes lines matching:
// - Co-Authored-By: ...
// - Generated with ...
// - Lines containing common model names
func StripAttribution(message string) string {
	lines := strings.Split(message, "\n")
	var clean []string

	for _, line := range lines {
		lower := strings.ToLower(strings.TrimSpace(line))

		// Skip Co-Authored-By lines
		if strings.HasPrefix(lower, "co-authored-by:") {
			continue
		}

		// Skip "Generated with" lines
		if strings.HasPrefix(lower, "generated with") {
			continue
		}

		// Skip lines with common AI model references
		if containsModelName(lower) {
			continue
		}

		clean = append(clean, line)
	}

	// Trim trailing empty lines
	for len(clean) > 0 && strings.TrimSpace(clean[len(clean)-1]) == "" {
		clean = clean[:len(clean)-1]
	}

	return strings.Join(clean, "\n")
}

// containsModelName checks if a line contains a known AI model name reference.
func containsModelName(lower string) bool {
	modelNames := []string{
		"claude", "gpt-4", "gpt-3", "gpt4", "gpt3",
		"openai", "anthropic", "codex",
		"copilot", "cursor", "aider",
	}
	for _, name := range modelNames {
		if strings.Contains(lower, name) {
			return true
		}
	}
	return false
}
