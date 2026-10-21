package cli

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/techspeque/metis/internal/findings"
	"github.com/techspeque/metis/internal/git"
	"github.com/techspeque/metis/internal/slice"
)

func init() {
	findingsResolveCmd.Flags().String("commit", "", "The fixing commit (any revision git resolves; recorded as its full hash)")
	findingsResolveCmd.Flags().String("test", "", "The test that shows the fix (name)")
	findingsResolveCmd.Flags().String("ref", "", "The file:line that shows the fix (must exist at --commit)")
	findingsResolveCmd.Flags().String("duplicate-of", "", "Close as a duplicate of this finding instead of a fix")
	findingsResolveCmd.Flags().String("superseded-by", "", "Close as superseded by this finding instead of a fix")
	findingsResolveCmd.Flags().String("note", "", "Anything else worth recording with the resolution")
	findingsCmd.AddCommand(findingsResolveCmd)

	findingsWaiveCmd.Flags().String("reason", "", "Why the finding is accepted as it stands (required)")
	findingsWaiveCmd.Flags().String("by", "", "Who accepts it: a person, or an agent slug acting on a recorded decision (required)")
	findingsWaiveCmd.Flags().String("residual", "", "What stays wrong, in one line")
	findingsWaiveCmd.Flags().String("owner", "", "Who revisits it")
	findingsWaiveCmd.Flags().String("revisit", "", "When it is looked at again: a date or the triggering event")
	findingsCmd.AddCommand(findingsWaiveCmd)

	findingsReclassifyCmd.Flags().String("severity", "", "Corrected severity: P1|P2|P3")
	findingsReclassifyCmd.Flags().String("category", "", "Corrected category (same set as metis block)")
	findingsReclassifyCmd.Flags().Bool("exclude", false, "Leave the row out of the severity and category counts (an aggregator, or a row recorded before severities meant anything)")
	findingsReclassifyCmd.Flags().String("reason", "", "Why the classification as raised was wrong (required)")
	findingsReclassifyCmd.Flags().String("by", "", "Who corrects it (required)")
	findingsCmd.AddCommand(findingsReclassifyCmd)

	findingsRecordCmd.Flags().String("severity", "P3", "Severity: P1|P2|P3")
	findingsRecordCmd.Flags().String("category", "", "Category (same set as metis block)")
	findingsRecordCmd.Flags().String("finding", "", "Description of the observation (required)")
	findingsRecordCmd.Flags().String("file", "", "The file the observation is about (default: the first file:line in its text)")
	findingsRecordCmd.Flags().String("follow-on", "", "The earlier finding this one follows (a recurrence, or a fix that did not finish)")
	findingsCmd.AddCommand(findingsRecordCmd)

	findingsCmd.Flags().String("severity", "", "Filter by severity: P1|P2|P3")
	findingsCmd.Flags().String("category", "", "Filter by category")
	findingsCmd.Flags().String("slice", "", "Filter by slice ID")
	findingsCmd.Flags().String("status", "", "Filter by status: "+strings.Join(findings.Statuses, "|"))
	findingsCmd.Flags().Bool("stats", false, "Show summary statistics")
	rootCmd.AddCommand(findingsCmd)
}

// fillAgentStats joins findings with the ledger and archive to produce the
// per-coder routing evidence: slices owned, review blocks received, and
// first-pass completions (done with zero review cycles).
func fillAgentStats(ctx *context, store *findings.Store, stats *findings.Stats) {
	l, err := ctx.loadLedger()
	if err != nil {
		return
	}
	archive, err := ctx.loadArchive()
	if err != nil {
		return
	}

	sliceCoder := map[string]string{}
	agg := map[string]findings.AgentStats{}
	all := append(append([]slice.Slice{}, l.Slices...), archive.Slices...)
	for i := range all {
		s := &all[i]
		if s.Coder == "" {
			continue
		}
		sliceCoder[s.ID] = s.Coder
		as := agg[s.Coder]
		as.Slices++
		if s.IsDone() {
			as.Done++
			if s.ReviewCycles == 0 {
				as.FirstPass++
			}
		}
		agg[s.Coder] = as
	}
	for i := range store.Findings {
		if store.Findings[i].Excluded {
			continue
		}
		if coder, ok := sliceCoder[store.Findings[i].Slice]; ok {
			as := agg[coder]
			as.Blocks++
			agg[coder] = as
		}
	}
	stats.ByAgent = agg
}

// loadFindings opens the project's findings store and returns it with its
// path.
func loadFindings(ctx *context) (*findings.Store, string, error) {
	path := filepath.Join(ctx.repoRoot, ctx.cfg.Paths.Findings)
	store, err := findings.Load(path)
	if err != nil {
		return nil, "", err
	}
	return store, path, nil
}

var findingsResolveCmd = &cobra.Command{
	Use:   "resolve <finding-id>",
	Short: "Close a finding with the evidence of its fix, or as a duplicate",
	Long: `Closes a finding. A fix names the commit that made it and what shows it:

  metis findings resolve f-012 --commit 4e16005 --test TestRefusesUserinfo
  metis findings resolve f-012 --commit 4e16005 --ref internal/oidc/issuer.go:88

The commit must resolve in this repository and is recorded as its full
hash; a --ref file must exist at that commit. A finding that another one
carries closes by disposition instead of an invented fix:

  metis findings resolve f-013 --duplicate-of f-012
  metis findings resolve f-013 --superseded-by f-020

A note alone is not a resolution. A finding accepted as it stands is not
resolved either: 'metis findings waive' records that on its own terms.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadContext()
		if err != nil {
			return err
		}
		store, path, err := loadFindings(ctx)
		if err != nil {
			return err
		}
		id := args[0]
		commit, _ := cmd.Flags().GetString("commit")
		test, _ := cmd.Flags().GetString("test")
		ref, _ := cmd.Flags().GetString("ref")
		dup, _ := cmd.Flags().GetString("duplicate-of")
		sup, _ := cmd.Flags().GetString("superseded-by")
		note, _ := cmd.Flags().GetString("note")

		var how string
		switch {
		case dup != "" && sup != "":
			return fmt.Errorf("--duplicate-of and --superseded-by are different dispositions; pass one")
		case dup != "" || sup != "":
			if commit != "" || test != "" || ref != "" {
				return fmt.Errorf("a disposition closes the finding without a fix of its own; drop --commit, --test and --ref, or resolve it as a fix")
			}
			disposition, of := "duplicate", dup
			if sup != "" {
				disposition, of = "superseded", sup
			}
			if err := store.Dispose(id, disposition, of); err != nil {
				return err
			}
			how = disposition + " of " + of
		case commit == "" || (test == "" && ref == ""):
			return fmt.Errorf("resolving %s needs evidence: --commit <rev> with --test <name> or --ref <file:line> for a fix, --duplicate-of/--superseded-by <id> for a disposition, or 'metis findings waive %s' to accept it as it stands", id, id)
		default:
			hash, err := git.RevParse(ctx.repoRoot, commit)
			if err != nil {
				return fmt.Errorf("--commit %q does not resolve in this repository: %w", commit, err)
			}
			evidence, err := resolveEvidence(ctx.repoRoot, hash, test, ref)
			if err != nil {
				return err
			}
			if err := store.Resolve(id, hash, evidence, note); err != nil {
				return err
			}
			how = fmt.Sprintf("commit %s; %s", hash[:12], evidence)
		}

		if err := store.Save(path); err != nil {
			return err
		}
		f := store.FindByID(id)
		fmt.Printf("Resolved %s (%s): %s\n", id, how, f.Finding)
		ctx.commitStateSoft(f.Slice, "resolve finding "+id, path)
		return nil
	},
}

// resolveEvidence validates and formats a fix's evidence: a test name, a
// file:line that exists at the fixing commit, or both.
func resolveEvidence(repoRoot, hash, test, ref string) (string, error) {
	var parts []string
	if test != "" {
		parts = append(parts, "test "+test)
	}
	if ref != "" {
		file, line, ok := strings.Cut(ref, ":")
		if !ok || file == "" || strings.Trim(line, "0123456789") != "" || line == "" {
			return "", fmt.Errorf("--ref %q is not file:line", ref)
		}
		if _, err := git.FileAt(repoRoot, hash, file); err != nil {
			return "", fmt.Errorf("--ref %s: %s does not exist at commit %s", ref, file, hash[:12])
		}
		parts = append(parts, ref)
	}
	return strings.Join(parts, "; "), nil
}

var findingsWaiveCmd = &cobra.Command{
	Use:   "waive <finding-id>",
	Short: "Accept a finding as it stands, on record",
	Long: `Marks a finding waived: not fixed, accepted. Severity, category and the
text stay as raised; the waiver records who accepts it, when, why, what
stays wrong, who revisits it and when. 'metis findings --stats' counts
waived apart from resolved, so a pass is never read as a fix.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadContext()
		if err != nil {
			return err
		}
		store, path, err := loadFindings(ctx)
		if err != nil {
			return err
		}
		w := findings.Waiver{}
		w.Reason, _ = cmd.Flags().GetString("reason")
		w.By, _ = cmd.Flags().GetString("by")
		w.Residual, _ = cmd.Flags().GetString("residual")
		w.Owner, _ = cmd.Flags().GetString("owner")
		w.Revisit, _ = cmd.Flags().GetString("revisit")
		if err := store.Waive(args[0], &w); err != nil {
			return err
		}
		if err := store.Save(path); err != nil {
			return err
		}
		f := store.FindByID(args[0])
		fmt.Printf("Waived %s by %s: %s\n", args[0], w.By, f.Finding)
		ctx.commitStateSoft(f.Slice, "waive finding "+args[0], path)
		return nil
	},
}

var findingsReclassifyCmd = &cobra.Command{
	Use:   "reclassify <finding-id>",
	Short: "Correct a finding's severity or category, keeping the original",
	Long: `Corrects a finding's classification with provenance: the classification
as raised is kept on the finding, and every correction records who made
it, when and why. --exclude leaves a row out of the severity and category
counts (an aggregator such as "cycle 3 blocked on f-12..f-15", or a row
recorded before severities meant anything); it stays in the ledger and
in the total.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadContext()
		if err != nil {
			return err
		}
		store, path, err := loadFindings(ctx)
		if err != nil {
			return err
		}
		c := findings.Correction{}
		c.Severity, _ = cmd.Flags().GetString("severity")
		c.Category, _ = cmd.Flags().GetString("category")
		c.Excluded, _ = cmd.Flags().GetBool("exclude")
		c.Reason, _ = cmd.Flags().GetString("reason")
		c.By, _ = cmd.Flags().GetString("by")
		if c.Severity == "" && c.Category == "" && !c.Excluded {
			return fmt.Errorf("nothing to correct: pass --severity, --category or --exclude")
		}
		if err := store.Reclassify(args[0], &c); err != nil {
			return err
		}
		if err := store.Save(path); err != nil {
			return err
		}
		f := store.FindByID(args[0])
		fmt.Printf("Reclassified %s: [%s/%s] excluded=%v (as raised: [%s/%s])\n", args[0], f.Severity, f.Category, f.Excluded, f.Original.Severity, f.Original.Category)
		ctx.commitStateSoft(f.Slice, "reclassify finding "+args[0], path)
		return nil
	},
}

var findingsRecordCmd = &cobra.Command{
	Use:   "record <slice-id>",
	Short: "Record a non-blocking advisory observation",
	Long: `Records an advisory finding without touching the slice's lifecycle —
unlike 'metis block', which resets coded and forces a rework cycle. Use for
worth-remembering observations that don't warrant blocking the review.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadContext()
		if err != nil {
			return err
		}
		text, _ := cmd.Flags().GetString("finding")
		if text == "" {
			return fmt.Errorf("--finding is required")
		}
		severity, _ := cmd.Flags().GetString("severity")
		category, _ := cmd.Flags().GetString("category")
		store, path, err := loadFindings(ctx)
		if err != nil {
			return err
		}
		id := store.AddWithStatus(args[0], severity, category, text, "advisory")
		file, _ := cmd.Flags().GetString("file")
		cycle := 0
		if l, err := ctx.loadLedger(); err == nil {
			cycle = reviewOf(l.FindByID(args[0]))
		}
		store.FindByID(id).Place(cycle, file)
		if parent, _ := cmd.Flags().GetString("follow-on"); parent != "" {
			if err := store.Follow(id, parent); err != nil {
				return err
			}
		}
		if err := store.Save(path); err != nil {
			return err
		}
		fmt.Printf("Advisory %s recorded: [%s/%s] %s\n", id, severity, category, text)
		ctx.commitStateSoft(args[0], "record advisory "+id, path)
		return nil
	},
}

var findingsCmd = &cobra.Command{
	Use:   "findings",
	Short: "Show review findings",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadContext()
		if err != nil {
			return err
		}

		store, _, err := loadFindings(ctx)
		if err != nil {
			return err
		}

		showStats, _ := cmd.Flags().GetBool("stats")
		if showStats {
			stats := store.GetStats()
			fillAgentStats(ctx, store, &stats)
			if jsonOutput() {
				return printJSON(cmd, stats)
			}
			fmt.Printf("Total findings: %d (counted: %d; excluded by correction: %d; waived: %d; recurrences: %d)\n\n",
				stats.Total, stats.Counted, stats.Excluded, stats.Waived, stats.Recurrences)
			if len(stats.ByStatus) > 0 {
				fmt.Println("By Status:")
				for _, status := range findings.Statuses {
					if count := stats.ByStatus[status]; count > 0 {
						fmt.Printf("  %-12s %d\n", status, count)
					}
				}
			}
			if len(stats.BySeverity) > 0 {
				fmt.Println("\nBy Severity (counted rows):")
				for _, sev := range sortedKeys(stats.BySeverity) {
					fmt.Printf("  %s: %d\n", sev, stats.BySeverity[sev])
				}
			}
			if len(stats.ByCategory) > 0 {
				fmt.Println("\nBy Category (counted rows):")
				for _, cat := range sortedKeys(stats.ByCategory) {
					fmt.Printf("  %-12s %d\n", cat, stats.ByCategory[cat])
				}
			}
			if len(stats.ByAgent) > 0 {
				fmt.Println("\nBy Agent (routing evidence):")
				fmt.Printf("  %-24s %7s %7s %10s\n", "coder", "slices", "blocks", "first-pass")
				for _, agent := range sortedKeys(stats.ByAgent) {
					as := stats.ByAgent[agent]
					rate := "-"
					if as.Done > 0 {
						rate = fmt.Sprintf("%d%%", as.FirstPass*100/as.Done)
					}
					fmt.Printf("  %-24s %7d %7d %10s\n", agent, as.Slices, as.Blocks, rate)
				}
			}
			return nil
		}

		severity, _ := cmd.Flags().GetString("severity")
		category, _ := cmd.Flags().GetString("category")
		sliceID, _ := cmd.Flags().GetString("slice")
		status, _ := cmd.Flags().GetString("status")
		if status != "" && !findings.ValidStatus(status) {
			return fmt.Errorf("--status must be one of %s", strings.Join(findings.Statuses, "|"))
		}

		results := store.FilterStatus(severity, category, sliceID, status)

		if jsonOutput() {
			if results == nil {
				results = []findings.Finding{}
			}
			return printJSON(cmd, results)
		}

		if len(results) == 0 {
			fmt.Println("No findings.")
			return nil
		}

		fmt.Printf("%-5s %-4s %-12s %-9s %-20s %s\n", "ID", "Sev", "Category", "Status", "Slice", "Finding")
		fmt.Println("───────────────────────────────────────────────────────────────────────────────────────")
		for _, f := range results {
			finding := f.Finding
			if len(finding) > 40 {
				finding = finding[:37] + "..."
			}
			if f.FollowOn != "" {
				finding = "↳ " + f.FollowOn + " " + finding
			}
			fmt.Printf("%-5s %-4s %-12s %-9s %-20s %s\n", f.ID, f.Severity, f.Category, f.Status, f.Slice, finding)
		}
		return nil
	},
}

// closedStatus reports whether a finding status needs no further action.
func closedStatus(status string) bool {
	return (&findings.Finding{Status: status}).Closed()
}

// reviewOf is the review (1-based) a finding recorded now belongs to: the
// review in progress on a coded slice, or the one that just blocked it.
func reviewOf(s *slice.Slice) int {
	if s == nil {
		return 0
	}
	if s.Coded {
		return s.ReviewCycles + 1
	}
	return s.ReviewCycles
}

// sortedKeys is a map's keys in ascending order, so a listing reads the
// same way twice.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
