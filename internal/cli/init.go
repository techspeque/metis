package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/techspeque/metis/internal/config"
	"github.com/techspeque/metis/internal/surface"
)

func init() {
	initCmd.Flags().String("from", "", "Non-interactive: read existing metis.yaml and scaffold")
	rootCmd.AddCommand(initCmd)
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new Metis project",
	Long: `Sets up the .metis/ directory structure and generates surface adapters.
Use --from metis.yaml for non-interactive mode.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		from, _ := cmd.Flags().GetString("from")

		var cfg *config.Config
		var repoRoot string

		if from != "" {
			// Non-interactive: load existing config
			loaded, err := config.Load(from)
			if err != nil {
				return err
			}
			cfg = loaded

			abs, err := filepath.Abs(from)
			if err != nil {
				return err
			}
			repoRoot = filepath.Dir(abs)
		} else {
			// Minimal interactive mode — create a basic config
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			repoRoot = cwd

			cfg = &config.Config{
				Version: 1,
				Project: config.ProjectConfig{
					Name:              filepath.Base(repoRoot),
					IntegrationBranch: "dev",
					ReleaseBranch:     "main",
				},
				Agents: map[string]config.Agent{},
				Commands: config.CommandsConfig{
					Verify: "echo 'no verify configured'",
				},
				Commits: config.CommitsConfig{
					Prefixes:       []string{"feat", "fix", "refactor", "docs", "test", "chore"},
					RequireSliceID: true,
					NoAttribution:  true,
					Format:         "{prefix}({slice_id}): {message}",
				},
				Paths: config.PathsConfig{
					Ledger:     ".metis/slices.yaml",
					Archive:    ".metis/slices-done.yaml",
					Briefs:     ".metis/briefs/",
					Findings:   ".metis/findings.yaml",
					Runs:       ".metis/runs/",
					Interfaces: "docs/generated/interfaces.txt",
				},
			}

			// Write metis.yaml if it doesn't exist
			cfgPath := filepath.Join(repoRoot, "metis.yaml")
			if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
				if err := writeConfig(cfgPath, cfg); err != nil {
					return err
				}
				fmt.Println("Created metis.yaml")
			} else {
				fmt.Println("metis.yaml already exists — using it")
				cfg, err = config.Load(cfgPath)
				if err != nil {
					return err
				}
			}
		}

		// Scaffold .metis/ directory
		dirs := []string{
			filepath.Join(repoRoot, ".metis"),
			filepath.Join(repoRoot, ".metis", "briefs"),
			filepath.Join(repoRoot, ".metis", "runs"),
			filepath.Join(repoRoot, "docs", "generated"),
			filepath.Join(repoRoot, "docs", "adr"),
		}
		for _, dir := range dirs {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("creating directory %s: %w", dir, err)
			}
		}

		// Create .gitkeep files
		gitkeeps := []string{
			filepath.Join(repoRoot, ".metis", "briefs", ".gitkeep"),
			filepath.Join(repoRoot, ".metis", "runs", ".gitkeep"),
		}
		for _, gk := range gitkeeps {
			if _, err := os.Stat(gk); os.IsNotExist(err) {
				os.WriteFile(gk, []byte{}, 0o644)
			}
		}

		// Create empty ledger if it doesn't exist
		ledgerPath := filepath.Join(repoRoot, cfg.Paths.Ledger)
		if _, err := os.Stat(ledgerPath); os.IsNotExist(err) {
			os.MkdirAll(filepath.Dir(ledgerPath), 0o755)
			os.WriteFile(ledgerPath, []byte("version: 1\nslices: []\n"), 0o644)
			fmt.Println("Created .metis/slices.yaml")
		}

		// Create empty findings if it doesn't exist
		findingsPath := filepath.Join(repoRoot, cfg.Paths.Findings)
		if _, err := os.Stat(findingsPath); os.IsNotExist(err) {
			os.WriteFile(findingsPath, []byte("findings: []\n"), 0o644)
			fmt.Println("Created .metis/findings.yaml")
		}

		// Generate surface adapters
		if err := surface.Generate(cfg, repoRoot); err != nil {
			return fmt.Errorf("generating surface adapters: %w", err)
		}

		// Create ADR template
		adrTemplate := filepath.Join(repoRoot, "docs", "adr", "_template.md")
		if _, err := os.Stat(adrTemplate); os.IsNotExist(err) {
			os.WriteFile(adrTemplate, []byte(adrTemplateContent), 0o644)
		}

		// Add .metis/runs/ to .gitignore if not already there
		gitignorePath := filepath.Join(repoRoot, ".gitignore")
		appendToGitignore(gitignorePath, ".metis/runs/")

		fmt.Println("\nMetis initialized successfully!")
		fmt.Println("  .metis/          — state directory")
		fmt.Println("  CLAUDE.md        — Claude Code adapter")
		fmt.Println("  AGENTS.md        — full agent contract")
		fmt.Println("  opencode.json    — opencode adapter")
		fmt.Printf("\nNext: add agents to metis.yaml, then run 'metis seed <plan>' or 'metis add'.\n")
		return nil
	},
}

func appendToGitignore(path, entry string) {
	data, _ := os.ReadFile(path)
	content := string(data)
	if !containsLine(content, entry) {
		if len(content) > 0 && content[len(content)-1] != '\n' {
			content += "\n"
		}
		content += entry + "\n"
		os.WriteFile(path, []byte(content), 0o644)
	}
}

func containsLine(content, line string) bool {
	for _, l := range splitLines(content) {
		if l == line {
			return true
		}
	}
	return false
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i, c := range s {
		if c == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

const adrTemplateContent = `# ADR-NNNN: <decision title>

- **Status:** Proposed | Accepted | Superseded by ADR-MMMM | Deprecated
- **Date:** YYYY-MM-DD
- **Decision drivers:** <why this decision, why now>

## Context

<The problem, the forces, the constraints.>

## Decision

<What we are doing. Imperative and specific.>

## Consequences

<Positive and negative. What gets easier, what gets harder.>

## Alternatives considered

<Other options and why not.>
`
