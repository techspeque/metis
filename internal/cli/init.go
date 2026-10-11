package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/techspeque/metis/internal/config"
	"github.com/techspeque/metis/internal/launch"
	"github.com/techspeque/metis/internal/prompt"
	"github.com/techspeque/metis/internal/surface"
	"github.com/techspeque/metis/internal/templates"
	"github.com/techspeque/metis/internal/userconfig"
)

func init() {
	initCmd.Flags().String("from", "", "Non-interactive: read existing .metis/project.yaml and scaffold")
	initCmd.Flags().String("auto-mode", "", "How metis auto runs: step (slice by slice), auto (unattended up to each gate) or manual (not at all); asked when interactive")
	rootCmd.AddCommand(initCmd)
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new Metis project",
	Long: `Sets up the .metis/ directory structure and generates surface adapters.
Non-interactive; configure afterwards with 'metis config set' (the Next
steps it prints are copy-pasteable). Use --from <config> to scaffold from
an existing configuration file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		from, _ := cmd.Flags().GetString("from")
		autoMode, _ := cmd.Flags().GetString("auto-mode")
		if autoMode != "" && !config.ValidMode(autoMode) {
			return fmt.Errorf("--auto-mode %q is not a mode (valid: step, auto, manual)", autoMode)
		}

		var cfg *config.Config
		var repoRoot string
		var cfgPath string
		created := false

		if from != "" {
			// Non-interactive: load existing config
			loaded, err := config.Load(from)
			if err != nil {
				return err
			}
			cfg = loaded
			cfgPath = from

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
				ReviewChecklist: []string{
					"Behavioral correctness",
					"Security and authorization correctness",
					"Scope discipline (diff vs. the committed brief)",
					"Test sufficiency",
					"Architectural fit — no duplicated/hallucinated interfaces; matches ADRs",
					"Maintainability",
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
			cfgPath = filepath.Join(repoRoot, config.FileName)
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
				if autoMode == "" {
					autoMode = askAutoMode()
				}
				cfg.Auto.Mode = autoMode
				cfg.Agents["human"] = humanAgentConfig
				if err := writeConfig(cfgPath, cfg); err != nil {
					return err
				}
				if err := appendAutoExamples(cfgPath); err != nil {
					return err
				}
				created = true
				fmt.Printf("Created %s (metis auto mode: %s)\n", config.FileName, autoMode)
			}
		}

		if !created {
			if err := ensureAutoSetup(cfgPath, cfg, autoMode); err != nil {
				return err
			}
			reloaded, err := config.Load(cfgPath)
			if err != nil {
				return err
			}
			cfg = reloaded
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
		fmt.Println("\nNext steps (copy-paste, adjust slugs/commands to taste):")
		fmt.Println("  git checkout -b " + cfg.Project.IntegrationBranch + "   # agents only work on the integration branch")
		fmt.Println("  metis config set project.overview OVERVIEW.md")
		fmt.Println("  metis config set commands.verify \"go test ./...\"")
		fmt.Println("  metis config set agents.claude-code/opus.surface claude-code")
		fmt.Println("  metis config set agents.claude-code/opus.model opus")
		fmt.Println("  metis config set agents.claude-code/opus.label \"Claude Code (Opus)\"")
		fmt.Println("  metis surface generate   # regenerate adapters after config changes")
		fmt.Println("  metis config set auto.mode step   # how metis auto runs: step | auto | manual")
		fmt.Println("  metis check              # green means ready to plan")
		fmt.Println("\nThen write OVERVIEW.md and ask an agent to plan Phase 0 using")
		fmt.Println("the template in .metis/templates/plan.md — seed it with 'metis seed'.")
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

// humanAgentConfig is the agent a person reviews or codes as: metis auto
// stops before its sessions, and an escalation's human review override
// flips as it.
var humanAgentConfig = config.Agent{Surface: config.SurfaceHuman, Model: "human", Label: "Human"}

// askAutoMode asks a person at a terminal how metis auto should run; the
// answer defaults to step, as does a non-interactive init.
func askAutoMode() string {
	if !prompt.IsTerminal(os.Stdin) {
		return config.ModeStep
	}
	fmt.Print("How should `metis auto` run? slice by slice (step) / unattended up to each gate (auto) / not at all (manual) [step]: ")
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "a", "auto", "unattended":
		return config.ModeAuto
	case "m", "manual", "not at all":
		return config.ModeManual
	default:
		return config.ModeStep
	}
}

// autoExamples are appended to a new project.yaml, under its auto section.
const autoExamples = `  # Per phase and per risk, overriding auto.mode (metis mode sets these):
  # phases: { phase-9: auto, phase-10: manual }
  # by_risk: { high: step }
`

func appendAutoExamples(path string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(autoExamples); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// ensureAutoSetup brings an existing project.yaml up to what init writes
// for metis auto, changing only what is missing: auto.mode when one was
// chosen, the human agent, and a launch for each agent whose surface metis
// knows.
func ensureAutoSetup(path string, cfg *config.Config, mode string) error {
	if mode != "" && mode != cfg.Auto.Mode {
		if err := config.SetInFile(path, "auto.mode", mode); err != nil {
			return err
		}
		fmt.Printf("Set auto.mode = %s\n", mode)
	}
	if humanAgent(cfg) == "" {
		if _, taken := cfg.Agents["human"]; !taken {
			for key, v := range map[string]string{"surface": humanAgentConfig.Surface, "model": humanAgentConfig.Model, "label": humanAgentConfig.Label} {
				if err := config.SetInFile(path, "agents.human."+key, v); err != nil {
					return err
				}
			}
			fmt.Println("Added the human agent (agents.human)")
		}
	}
	for _, slug := range sortedKeys(cfg.Agents) {
		a := cfg.Agents[slug]
		argv, known := launch.Defaults[a.Surface]
		if !known || len(a.Launch) > 0 || strings.Contains(slug, ".") {
			continue
		}
		if err := config.SetInFile(path, "agents."+slug+".launch", strings.Join(argv, ",")); err != nil {
			return err
		}
		fmt.Printf("Wrote the default launcher for %s (agents.%s.launch)\n", slug, slug)
	}
	return nil
}
