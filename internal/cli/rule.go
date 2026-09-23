package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/techspeque/metis/internal/config"
	"github.com/techspeque/metis/internal/findings"
	"github.com/techspeque/metis/internal/prompt"
)

func init() {
	ruleCmd.AddCommand(ruleAddCmd)
	ruleCmd.AddCommand(ruleListCmd)
	ruleCmd.AddCommand(rulePromoteCmd)
	ruleCmd.AddCommand(ruleRemoveCmd)
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

var ruleRemoveCmd = &cobra.Command{
	Use:     "remove [rule-number...]",
	Aliases: []string{"rm"},
	Short:   "Remove accuracy rules — pick from a list, or name them by number",
	Long: `Remove accuracy rules from .metis/project.yaml.

With no arguments on an interactive terminal, lists the current rules to
pick from (space selects, enter confirms). Otherwise name the rules by the
numbers 'metis rule list' shows:

  metis rule remove 2 5

Findings promoted to a removed rule stay promoted but lose their pointer;
findings promoted to later rules are renumbered to match.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadContext()
		if err != nil {
			return err
		}
		rules := ctx.cfg.AccuracyRules
		if len(rules) == 0 {
			return errors.New("no accuracy rules to remove")
		}

		var numbers []int
		if len(args) == 0 {
			if jsonOutput() || !interactive() {
				return errors.New("name the rules to remove by number (see 'metis rule list'), or run on an interactive terminal to pick them")
			}
			picked, err := selectRules(rules)
			if errors.Is(err, prompt.ErrCancelled) {
				_, err := fmt.Fprintln(cmd.OutOrStdout(), "Cancelled — no rules removed.")
				return err
			}
			if err != nil {
				return err
			}
			if len(picked) == 0 {
				_, err := fmt.Fprintln(cmd.OutOrStdout(), "No rules selected — nothing removed.")
				return err
			}
			for _, i := range picked {
				numbers = append(numbers, i+1)
			}
		} else {
			numbers, err = parseRuleNumbers(args, len(rules))
			if err != nil {
				return err
			}
		}

		gone := make(map[int]bool, len(numbers))
		for _, n := range numbers {
			gone[n] = true
		}
		var kept []string
		for i, r := range rules {
			if !gone[i+1] {
				kept = append(kept, r)
			}
		}

		findingsPath := filepath.Join(ctx.repoRoot, ctx.cfg.Paths.Findings)
		store, err := findings.Load(findingsPath)
		if err != nil {
			return err
		}
		findingsChanged := store.RenumberRules(numbers)

		ctx.cfg.AccuracyRules = kept
		if err := writeConfig(ctx.cfgPath, ctx.cfg); err != nil {
			return err
		}
		paths := []string{ctx.cfgPath}
		if findingsChanged {
			if err := store.Save(findingsPath); err != nil {
				return err
			}
			paths = append(paths, findingsPath)
		}

		// The rules are saved; a failed report must not stop the commit.
		for _, n := range numbers {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Removed accuracy rule #%d: %s\n", n, rules[n-1])
		}
		msg := "remove accuracy rule"
		if len(numbers) > 1 {
			msg = fmt.Sprintf("remove %d accuracy rules", len(numbers))
		}
		ctx.commitStateSoft("rules", msg, paths...)
		return nil
	},
}

// selectRules asks the user which rules to remove and returns their
// 0-based indexes. Tests replace it.
var selectRules = func(rules []string) ([]int, error) {
	return prompt.MultiSelectTerminal(os.Stdin, os.Stdout, "Select accuracy rules to remove:", rules)
}

// interactive reports whether a human is at the terminal. Tests replace it.
var interactive = func() bool {
	return prompt.IsTerminal(os.Stdin) && prompt.IsTerminal(os.Stdout)
}

// parseRuleNumbers reads 1-based rule numbers, each within 1..count,
// deduplicated and sorted.
func parseRuleNumbers(args []string, count int) ([]int, error) {
	seen := map[int]bool{}
	var out []int
	for _, a := range args {
		n, err := strconv.Atoi(strings.TrimPrefix(a, "#"))
		if err != nil {
			return nil, fmt.Errorf("%q is not a rule number (see 'metis rule list')", a)
		}
		if n < 1 || n > count {
			return nil, fmt.Errorf("no accuracy rule #%d — there are %d (see 'metis rule list')", n, count)
		}
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	sort.Ints(out)
	return out, nil
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
