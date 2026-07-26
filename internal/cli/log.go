package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/techspeque/metis/internal/brief"
	"github.com/techspeque/metis/internal/git"
	"github.com/techspeque/metis/internal/slice"
)

func init() {
	logCmd.Flags().Bool("validate", false, "Audit the slice's commits: format compliance and files vs the brief's owned_paths")
	rootCmd.AddCommand(logCmd)
}

var logCmd = &cobra.Command{
	Use:   "log <id>",
	Short: "Show a slice's commit history",
	Long: `Lists every commit referencing the slice ID, oldest first.

With --validate, audits the history deterministically — the reviewer's
scope-measurement tool:
  - every subject matches the commit format with an allowed prefix
  - no attribution lines survive in any message
  - every touched file falls inside the brief's declared owned_paths
    (metis state files and the brief itself are always in scope)

Exit code 1 when any check fails.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadContext()
		if err != nil {
			return err
		}
		sliceID := args[0]

		commits, err := git.SliceCommits(ctx.repoRoot, sliceID)
		if err != nil {
			return err
		}

		validate, _ := cmd.Flags().GetBool("validate")
		if !validate {
			if jsonOutput() {
				if commits == nil {
					commits = []git.SliceCommit{}
				}
				return printJSON(cmd, commits)
			}
			if len(commits) == 0 {
				fmt.Printf("No commits reference %s\n", sliceID)
				return nil
			}
			for _, c := range commits {
				fmt.Printf("%s %s\n", c.Hash, c.Subject)
			}
			return nil
		}

		report := auditSlice(ctx, sliceID, commits)

		if jsonOutput() {
			if err := printJSON(cmd, report); err != nil {
				return err
			}
		} else {
			printAuditText(&report)
		}
		if !report.OK {
			return exitWithCode(cmd, 1)
		}
		return nil
	},
}

// auditReport is the JSON shape of 'metis log --validate'.
type auditReport struct {
	Slice           string        `json:"slice"`
	OK              bool          `json:"ok"`
	Gate            bool          `json:"gate"`
	FirstCommit     string        `json:"first_commit,omitempty"`
	LastCommit      string        `json:"last_commit,omitempty"`
	ScopeVerifiable bool          `json:"scope_verifiable"`
	OwnedPaths      []string      `json:"owned_paths"`
	Commits         []auditCommit `json:"commits"`
	OutOfScope      []string      `json:"out_of_scope_files"`
}

type auditCommit struct {
	Hash    string   `json:"hash"`
	Subject string   `json:"subject"`
	Issues  []string `json:"issues"`
}

func auditSlice(ctx *context, sliceID string, commits []git.SliceCommit) auditReport {
	report := auditReport{Slice: sliceID, OK: true, OutOfScope: []string{}}

	// Gate slices validate composition, not file edits — the scope audit
	// does not apply to them.
	if l, err := ctx.loadLedger(); err == nil {
		if s := l.FindByID(sliceID); s != nil && s.Type == slice.TypeGate {
			report.Gate = true
		}
	}
	if !report.Gate {
		if archive, err := ctx.loadArchive(); err == nil {
			for i := range archive.Slices {
				if archive.Slices[i].ID == sliceID && archive.Slices[i].Type == slice.TypeGate {
					report.Gate = true
				}
			}
		}
	}

	// Scope contract from the committed brief.
	briefRel := filepath.Join(ctx.cfg.Paths.Briefs, sliceID+".md")
	if data, err := os.ReadFile(filepath.Join(ctx.repoRoot, briefRel)); err == nil {
		report.OwnedPaths = brief.ParseOwnedPaths(string(data))
	}
	report.ScopeVerifiable = len(report.OwnedPaths) > 0
	if report.OwnedPaths == nil {
		report.OwnedPaths = []string{}
	}

	seenOutOfScope := map[string]bool{}
	for _, c := range commits {
		ac := auditCommit{Hash: c.Hash, Subject: c.Subject, Issues: []string{}}

		if !strings.Contains(c.Subject, sliceID) {
			ac.Issues = append(ac.Issues, "subject does not contain the slice ID")
		}
		prefixOK := false
		for _, p := range ctx.cfg.Commits.Prefixes {
			if strings.HasPrefix(c.Subject, p+"(") {
				prefixOK = true
				break
			}
		}
		if !prefixOK {
			ac.Issues = append(ac.Issues, fmt.Sprintf("subject prefix not in allowed set (%s)", strings.Join(ctx.cfg.Commits.Prefixes, ", ")))
		}
		full := c.Subject + "\n" + c.Body
		if ctx.cfg.Commits.NoAttribution && git.StripAttribution(full) != strings.TrimRight(full, "\n") {
			ac.Issues = append(ac.Issues, "message contains attribution lines")
		}

		for _, f := range c.Files {
			if strings.HasPrefix(f, ".metis/") || f == briefRel {
				continue // metis state and the brief are always in scope
			}
			if report.ScopeVerifiable && !brief.InScope(f, report.OwnedPaths) && !seenOutOfScope[f] {
				seenOutOfScope[f] = true
				report.OutOfScope = append(report.OutOfScope, f)
			}
		}

		if len(ac.Issues) > 0 {
			report.OK = false
		}
		report.Commits = append(report.Commits, ac)
	}
	if len(report.OutOfScope) > 0 {
		report.OK = false
	}
	// An undeclared scope is a protocol violation for normal slices — a
	// brief without owned_paths must not pass the audit silently.
	if !report.Gate && !report.ScopeVerifiable {
		report.OK = false
	}
	if report.Commits == nil {
		report.Commits = []auditCommit{}
	}
	if len(commits) > 0 {
		report.FirstCommit = commits[0].Hash
		report.LastCommit = commits[len(commits)-1].Hash
	}
	return report
}

func printAuditText(r *auditReport) {
	fmt.Printf("Audit: %s (%d commit(s))\n", r.Slice, len(r.Commits))
	for _, c := range r.Commits {
		mark := "ok"
		if len(c.Issues) > 0 {
			mark = "FAIL"
		}
		fmt.Printf("  [%s] %s %s\n", mark, c.Hash, c.Subject)
		for _, issue := range c.Issues {
			fmt.Printf("        - %s\n", issue)
		}
	}
	switch {
	case r.Gate:
		fmt.Println("Scope: gate slice — scope audit not applicable")
	case !r.ScopeVerifiable:
		fmt.Println("Scope: FAIL — brief declares no owned_paths (scope is a contract; declare it)")
	case len(r.OutOfScope) > 0:
		fmt.Println("Scope: VIOLATIONS — files outside the brief's owned_paths:")
		for _, f := range r.OutOfScope {
			fmt.Printf("  - %s\n", f)
		}
	default:
		fmt.Println("Scope: all touched files within owned_paths")
	}
	if r.OK {
		fmt.Println("Verdict: PASS")
	} else {
		fmt.Println("Verdict: FAIL")
	}
}
