// Package surface generates and validates agent surface adapter files
// (CLAUDE.md, AGENTS.md, opencode.json, .claude/settings.json).
package surface

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/techspeque/metis/internal/config"
	"github.com/techspeque/metis/internal/instructions"
)

// Generate writes all surface adapter files from current config.
func Generate(cfg *config.Config, repoRoot string) error {
	if err := writeCLAUDE(cfg, repoRoot); err != nil {
		return err
	}
	if err := writeAGENTS(cfg, repoRoot); err != nil {
		return err
	}
	if err := writeOpencode(repoRoot); err != nil {
		return err
	}
	if err := writeClaudeSettings(repoRoot); err != nil {
		return err
	}
	if err := writeHashFile(cfg, repoRoot); err != nil {
		return err
	}
	return nil
}

// Validate checks that adapter files exist and are not stale.
func Validate(cfg *config.Config, repoRoot string) []string {
	var warnings []string

	files := []string{"CLAUDE.md", "AGENTS.md", "opencode.json", ".claude/settings.json"}
	for _, f := range files {
		path := filepath.Join(repoRoot, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			warnings = append(warnings, fmt.Sprintf("surface adapter missing: %s", f))
		}
	}

	// Check staleness via hash
	hashPath := filepath.Join(repoRoot, ".metis", "surface.hash")
	stored, err := os.ReadFile(hashPath)
	if err != nil {
		if !os.IsNotExist(err) {
			warnings = append(warnings, "cannot read surface hash file")
		} else {
			warnings = append(warnings, "surface adapters may be stale (no hash file — run 'metis surface generate')")
		}
		return warnings
	}

	current := configHash(cfg)
	if string(stored) != current {
		warnings = append(warnings, "surface adapters are stale (config changed since last generate)")
	}

	return warnings
}

func writeCLAUDE(cfg *config.Config, repoRoot string) error {
	content := `# CLAUDE.md
Run ` + "`metis kickoff`" + ` from step 1 at the start of every session. No pasted
prompt is needed. For full contract details: ` + "`metis instructions`" + `.

Identity: state your model as one of the slugs from ` + "`metis next`" + ` output.
`
	return os.WriteFile(filepath.Join(repoRoot, "CLAUDE.md"), []byte(content), 0o644)
}

func writeAGENTS(cfg *config.Config, repoRoot string) error {
	content := instructions.Generate(cfg)
	return os.WriteFile(filepath.Join(repoRoot, "AGENTS.md"), []byte(content+"\n"), 0o644)
}

func writeOpencode(repoRoot string) error {
	data := map[string]interface{}{
		"$schema":      "https://opencode.ai/config.json",
		"instructions": []string{"AGENTS.md"},
	}
	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(repoRoot, "opencode.json"), append(content, '\n'), 0o644)
}

func writeClaudeSettings(repoRoot string) error {
	dir := filepath.Join(repoRoot, ".claude")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data := map[string]interface{}{
		"attribution":      map[string]string{"commit": "", "pr": ""},
		"includeCoAuthoredBy": false,
	}
	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "settings.json"), append(content, '\n'), 0o644)
}

func writeHashFile(cfg *config.Config, repoRoot string) error {
	dir := filepath.Join(repoRoot, ".metis")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	hash := configHash(cfg)
	return os.WriteFile(filepath.Join(dir, "surface.hash"), []byte(hash), 0o644)
}

func configHash(cfg *config.Config) string {
	// Hash the config fields that affect surface adapter content
	data, _ := json.Marshal(cfg)
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
