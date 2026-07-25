package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/techspeque/metis/internal/config"
	"github.com/techspeque/metis/internal/surface"
	"github.com/techspeque/metis/internal/templates"
	"github.com/techspeque/metis/internal/userconfig"
)

func init() {
	initCmd.Flags().String("from", "", "Non-interactive: read existing .metis/project.yaml and scaffold")
	rootCmd.AddCommand(initCmd)
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new Metis project",
	Long: `Sets up the .metis/ directory structure and generates surface adapters.
Use --from .metis/project.yaml for non-interactive mode.`,
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
			repoRoot = config.RootFromConfigPath(abs)
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
					Plans:      ".metis/plans/",
					ADR:        ".metis/adr/",
					Findings:   ".metis/findings.yaml",
					Runs:       ".metis/runs/",
					Interfaces: ".metis/interfaces.txt",
				},
			}

			// The config lives at .metis/project.yaml; migrate a legacy
			// root metis.yaml if present, otherwise create or reuse.
			cfgPath := filepath.Join(repoRoot, config.FileName)
			legacyPath := filepath.Join(repoRoot, config.LegacyFileName)
			if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
				return fmt.Errorf("creating .metis directory: %w", err)
			}

			_, haveCurrent := os.Stat(cfgPath)
			_, haveLegacy := os.Stat(legacyPath)
			switch {
			case haveCurrent == nil:
				fmt.Printf("%s already exists — using it\n", config.FileName)
				cfg, err = config.Load(cfgPath)
				if err != nil {
					return err
				}
			case haveLegacy == nil:
				if err := os.Rename(legacyPath, cfgPath); err != nil {
					return fmt.Errorf("migrating %s to %s: %w", config.LegacyFileName, config.FileName, err)
				}
				fmt.Printf("Migrated %s -> %s (remember to commit the move)\n", config.LegacyFileName, config.FileName)
				cfg, err = config.Load(cfgPath)
				if err != nil {
					return err
				}
			default:
				if err := writeConfig(cfgPath, cfg); err != nil {
					return err
				}
				fmt.Printf("Created %s\n", config.FileName)
			}
		}

		// Scaffold .metis/ directory
		dirs := []string{
			filepath.Join(repoRoot, ".metis"),
			filepath.Join(repoRoot, ".metis", "briefs"),
			filepath.Join(repoRoot, ".metis", "plans"),
			filepath.Join(repoRoot, ".metis", "adr"),
			filepath.Join(repoRoot, ".metis", "runs"),
		}
		for _, dir := range dirs {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("creating directory %s: %w", dir, err)
			}
		}

		// Create .gitkeep files
		gitkeeps := []string{
			filepath.Join(repoRoot, ".metis", "briefs", ".gitkeep"),
			filepath.Join(repoRoot, ".metis", "plans", ".gitkeep"),
			filepath.Join(repoRoot, ".metis", "runs", ".gitkeep"),
		}
		for _, gk := range gitkeeps {
			if _, err := os.Stat(gk); os.IsNotExist(err) {
				if err := os.WriteFile(gk, []byte{}, 0o644); err != nil {
					return fmt.Errorf("creating gitkeep %s: %w", gk, err)
				}
			}
		}

		// Create empty ledger if it doesn't exist
		ledgerPath := filepath.Join(repoRoot, cfg.Paths.Ledger)
		if _, err := os.Stat(ledgerPath); os.IsNotExist(err) {
			if err := os.MkdirAll(filepath.Dir(ledgerPath), 0o755); err != nil {
				return fmt.Errorf("creating ledger directory: %w", err)
			}
			if err := os.WriteFile(ledgerPath, []byte("version: 1\nslices: []\n"), 0o644); err != nil {
				return fmt.Errorf("creating ledger file: %w", err)
			}
			fmt.Println("Created .metis/slices.yaml")
		}

		// Create empty findings if it doesn't exist
		findingsPath := filepath.Join(repoRoot, cfg.Paths.Findings)
		if _, err := os.Stat(findingsPath); os.IsNotExist(err) {
			if err := os.WriteFile(findingsPath, []byte("findings: []\n"), 0o644); err != nil {
				return fmt.Errorf("creating findings file: %w", err)
			}
			fmt.Println("Created .metis/findings.yaml")
		}

		// Generate surface adapters
		if err := surface.Generate(cfg, repoRoot); err != nil {
			return fmt.Errorf("generating surface adapters: %w", err)
		}

		// Create ADR template (uses the rich template from templates package)
		adrTemplate := filepath.Join(repoRoot, ".metis", "adr", "_template.md")
		if _, err := os.Stat(adrTemplate); os.IsNotExist(err) {
			if err := os.WriteFile(adrTemplate, []byte(templates.ADRTemplate), 0o644); err != nil {
				return fmt.Errorf("creating ADR template: %w", err)
			}
		}

		// Write document templates
		templatesDir := filepath.Join(repoRoot, ".metis", "templates")
		if err := templates.WriteAll(templatesDir); err != nil {
			return fmt.Errorf("writing templates: %w", err)
		}

		// Ignore transient state: verification logs and the process lock
		gitignorePath := filepath.Join(repoRoot, ".gitignore")
		appendToGitignore(gitignorePath, ".metis/runs/")
		appendToGitignore(gitignorePath, ".metis/.lock")

		// Register this project in the user-level workspace registry so the
		// registry populates itself through normal use. Registration failures
		// (e.g. name collision with a different project) never fail init.
		registerWorkspace(cfg, repoRoot)

		fmt.Println("\nMetis initialized successfully!")
		fmt.Println("  .metis/            — state directory")
		fmt.Println("  .metis/templates/  — document templates (for agents)")
		fmt.Println("  CLAUDE.md          — Claude Code adapter (points to AGENTS.md)")
		fmt.Println("  AGENTS.md          — governance + full agent contract")
		fmt.Println("  opencode.json      — opencode adapter")
		fmt.Printf("\nNext: write your OVERVIEW.md, set project.overview in .metis/project.yaml,\n")
		fmt.Printf("      add agents, then ask an agent to plan Phase 0 using\n")
		fmt.Printf("      the template in .metis/templates/plan.md\n")
		return nil
	},
}

// registerWorkspace adds the project to ~/.metis/config.yaml under the
// project name (falling back to the directory basename). A name collision
// with a different path is skipped silently — init must not fail over it.
func registerWorkspace(cfg *config.Config, repoRoot string) {
	name := cfg.Project.Name
	if name == "" {
		name = filepath.Base(repoRoot)
	}

	uc, err := userconfig.Load()
	if err != nil {
		return
	}
	if err := uc.Add(name, repoRoot); err != nil {
		return
	}
	if err := uc.Save(); err != nil {
		return
	}
	fmt.Printf("Registered workspace %q in the user registry\n", name)
}

func appendToGitignore(path, entry string) {
	data, _ := os.ReadFile(path)
	content := string(data)
	if !containsLine(content, entry) {
		if len(content) > 0 && content[len(content)-1] != '\n' {
			content += "\n"
		}
		content += entry + "\n"
		_ = os.WriteFile(path, []byte(content), 0o644)
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
