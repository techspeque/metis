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
		warnings = append(warnings, "surface adapters are stale (config or metis version changed since last generate) — run 'metis surface generate'")
	}

	return warnings
}

func writeCLAUDE(cfg *config.Config, repoRoot string) error {
	content := `# CLAUDE.md

This repository is governed by ` + "`AGENTS.md`" + `. Read it now.
Run ` + "`metis kickoff`" + ` immediately at session start. No other action first.

Identity: state your model; it must match the ` + "`agent_slug`" + ` field of
` + "`metis next -o json`" + `.
`
	return os.WriteFile(filepath.Join(repoRoot, "CLAUDE.md"), []byte(content), 0o644)
}

func writeAGENTS(cfg *config.Config, repoRoot string) error {
	preamble := fmt.Sprintf(`# Agent Contract — %s

This repository is managed by Metis. ALL autonomous work follows the
protocol below. These rules are non-negotiable.

## Mandatory

Run `+"`metis kickoff`"+` from step 1 at the start of every session.
Do NOT skip this. Do NOT start work without following the protocol.

## Hard Rules

1. ONE slice at a time — `+"`metis next`"+` decides which, not you
2. Brief BEFORE code — commit scope contract before implementation
3. Scope is a contract — only touch files declared in your brief
4. Cross-vendor review — you cannot review your own work
5. `+"`metis commit`"+` for all commits — enforces format, strips attribution
6. STOP on environment failure — do not modify code to fix a broken sandbox
7. Dirty tree with in-scope files — resume the interrupted session (read brief, check git log, continue)
8. Dirty tree with out-of-scope files — STOP and report to human
9. Reality beats documents — if code contradicts plan, fix the document
10. No planning in execution — do not re-scope or invent additional work
11. Report mismatches — if you're the wrong agent for this slice, STOP
12. Trust the tools — do not walk YAML, compare slugs, or evaluate booleans manually
13. Exact values come from `+"`-o json`"+` — every read command supports it; never parse human-readable output

---

`, cfg.Project.Name)

	contract := instructions.Generate(cfg, repoRoot)
	content := preamble + contract + "\n"
	return os.WriteFile(filepath.Join(repoRoot, "AGENTS.md"), []byte(content), 0o644)
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
		"attribution":         map[string]string{"commit": "", "pr": ""},
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

// toolVersion participates in the staleness hash so that upgrading metis
// (which may change generated adapter content) flags existing adapters as
// stale even when the project config is unchanged. Set via SetVersion.
var toolVersion = "dev"

// SetVersion records the metis version used in the adapter staleness hash.
func SetVersion(v string) {
	toolVersion = v
}

func configHash(cfg *config.Config) string {
	// Hash the config fields that affect surface adapter content, salted
	// with the metis version that generated them.
	data, _ := json.Marshal(cfg)
	data = append(data, []byte(toolVersion)...)
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
