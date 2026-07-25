package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/techspeque/metis/internal/config"
	"github.com/techspeque/metis/internal/findings"
)

func init() {
	ruleCmd.AddCommand(ruleAddCmd)
	ruleCmd.AddCommand(ruleListCmd)
	ruleCmd.AddCommand(rulePromoteCmd)
	rootCmd.AddCommand(ruleCmd)
}

var ruleCmd = &cobra.Command{
	Use:   "rule",
	Short: "Manage accuracy rules",
}

var ruleAddCmd = &cobra.Command{
	Use:   "add <rule text>",
	Short: "Add a new accuracy rule to .metis/project.yaml",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadContext()
		if err != nil {
			return err
		}

		// Add rule to config
		ctx.cfg.AccuracyRules = append(ctx.cfg.AccuracyRules, args[0])

		// Rewrite .metis/project.yaml
		if err := writeConfig(ctx.cfgPath, ctx.cfg); err != nil {
			return err
		}

		fmt.Printf("Added accuracy rule #%d: %s\n", len(ctx.cfg.AccuracyRules), args[0])
		ctx.commitStateSoft("rules", "add accuracy rule", ctx.cfgPath)
		return nil
	},
}

var ruleListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show all accuracy rules",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadContext()
		if err != nil {
			return err
		}

		if jsonOutput() {
			rules := ctx.cfg.AccuracyRules
			if rules == nil {
				rules = []string{}
			}
			return printJSON(cmd, rules)
		}

		if len(ctx.cfg.AccuracyRules) == 0 {
			fmt.Println("No accuracy rules configured.")
			return nil
		}

		fmt.Println("Accuracy Rules:")
		for i, r := range ctx.cfg.AccuracyRules {
			fmt.Printf("  %d. %s\n", i+1, r)
		}
		return nil
	},
}

var rulePromoteCmd = &cobra.Command{
	Use:   "promote <finding-id>",
	Short: "Promote a finding to an accuracy rule",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadContext()
		if err != nil {
			return err
		}

		findingsPath := filepath.Join(ctx.repoRoot, ctx.cfg.Paths.Findings)
		store, err := findings.Load(findingsPath)
		if err != nil {
			return err
		}

		f := store.FindByID(args[0])
		if f == nil {
			return fmt.Errorf("finding %q not found", args[0])
		}

		if f.Status == "promoted" {
			return fmt.Errorf("finding %q is already promoted", args[0])
		}

		// Add to accuracy rules
		ctx.cfg.AccuracyRules = append(ctx.cfg.AccuracyRules, f.Finding)
		ruleIdx := len(ctx.cfg.AccuracyRules)

		// Mark finding as promoted
		f.Status = "promoted"
		f.PromotedTo = &ruleIdx

		// Save both
		if err := writeConfig(ctx.cfgPath, ctx.cfg); err != nil {
			return err
		}
		if err := store.Save(findingsPath); err != nil {
			return err
		}

		fmt.Printf("Promoted %s to accuracy rule #%d: %s\n", f.ID, ruleIdx, f.Finding)
		ctx.commitStateSoft(f.Slice, "promote finding "+f.ID+" to rule", ctx.cfgPath, findingsPath)
		return nil
	},
}

// writeConfig writes the config back to .metis/project.yaml.
// This is a simplified approach — it rewrites the entire file.
func writeConfig(path string, cfg *config.Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	// Add a header comment
	header := "# .metis/project.yaml — project configuration for Metis\n"
	content := header + string(data)

	// Fix any trailing issues
	content = strings.TrimRight(content, "\n") + "\n"

	return os.WriteFile(path, []byte(content), 0o644)
}
