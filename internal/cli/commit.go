package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/techspeque/metis/internal/git"
	"github.com/techspeque/metis/internal/runs"
	"github.com/techspeque/metis/internal/slice"
)

func init() {
	commitCmd.Flags().String("prefix", "", "Commit prefix (inferred from slice type if omitted)")
	commitCmd.Flags().StringP("message", "m", "", "Commit message (required unless using shortcuts)")
	commitCmd.Flags().Bool("brief", false, "Shortcut: add and commit the brief file")
	commitCmd.Flags().String("flip", "", "Shortcut: flip coded or reviewed and commit (coded|reviewed)")
	commitCmd.Flags().Bool("amend", false, "Amend the previous commit")
	commitCmd.Flags().String("agent", "", "Your agent slug (required with --flip reviewed for cross-vendor validation)")
	commitCmd.Flags().String("slice", "", "The slice ID you were dispatched (errors if dispatch has moved on)")
	rootCmd.AddCommand(commitCmd)
}

var commitCmd = &cobra.Command{
	Use:   "commit",
	Short: "Create a git commit with enforced conventions",
	Long: `Wrapper around git commit that enforces branch, format, and attribution rules.
The commit subject is formatted as: {prefix}({slice_id}): {message}`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadContext()
		if err != nil {
			return err
		}

		// Validate branch
		if err := git.ValidateBranch(ctx.repoRoot, ctx.cfg.Project.IntegrationBranch); err != nil {
			return fmt.Errorf("branch check failed: %w", err)
		}

		// Get active slice
		l, err := ctx.loadLedger()
		if err != nil {
			return err
		}
		result := l.Next()
		if result == nil {
			return fmt.Errorf("no active slice — cannot commit")
		}

		sliceID := result.Slice.ID
		sliceType := result.Slice.Type

		// Bind to the dispatched slice: an agent passes the ID it received
		// from 'metis next'; if a higher-priority slice arrived in between,
		// fail loudly instead of silently acting on the wrong slice.
		if claimed, _ := cmd.Flags().GetString("slice"); claimed != "" && claimed != sliceID {
			return fmt.Errorf("slice mismatch: the active slice is %s but you passed --slice %s — if %s is what you were dispatched, dispatch has moved on; re-run 'metis next' and report to the human", sliceID, claimed, claimed)
		}

		// Handle shortcuts
		briefMode, _ := cmd.Flags().GetBool("brief")
		flipMode, _ := cmd.Flags().GetString("flip")
		amend, _ := cmd.Flags().GetBool("amend")

		agentFlag, _ := cmd.Flags().GetString("agent")

		// A flag that is quietly ignored reads as a flag that was honoured.
		// Shortcuts build their own message and commit their own paths, so
		// say so rather than dropping the caller's intent on the floor.
		if briefMode && flipMode != "" {
			return fmt.Errorf("--brief and --flip are separate shortcuts; run them one at a time")
		}
		if amend && (briefMode || flipMode != "") {
			return fmt.Errorf("--amend does not apply to --brief or --flip (they commit metis state, not your work)")
		}
		if agentFlag != "" && flipMode != "reviewed" {
			return fmt.Errorf("--agent applies to --flip reviewed only")
		}

		switch {
		case briefMode:
			briefMsg, _ := cmd.Flags().GetString("message")
			return commitBrief(ctx, sliceID, briefMsg)
		case flipMode == "coded":
			return commitFlip(ctx, sliceID, "coded", "")
		case flipMode == "reviewed":
			// Cross-vendor review needs the caller's identity — the slug
			// the reviewer stated at self-identification. Without it the
			// check would just compare two ledger fields to each other.
			if agentFlag == "" && !ctx.allowSelfReview() {
				return fmt.Errorf("identify yourself: metis commit --flip reviewed --agent <your-slug> (the slug you matched in the session protocol)")
			}
			if agentFlag != "" && !ctx.agentSlugs()[agentFlag] {
				return fmt.Errorf("unknown agent slug %q (configured agents: metis config get agents)", agentFlag)
			}
			if ctx.allowSelfReview() {
				agentFlag = "" // single-agent mode: skip the coder comparison
			}
			return commitFlip(ctx, sliceID, "reviewed", agentFlag)
		case flipMode != "":
			return fmt.Errorf("invalid flip target: %s (use 'coded' or 'reviewed')", flipMode)
		}

		// Normal commit
		message, _ := cmd.Flags().GetString("message")
		if message == "" {
			return fmt.Errorf("--message is required")
		}

		// -m takes the bare message; metis adds the prefix. If the caller
		// already included a formatted prefix, strip it instead of doubling.
		for _, p := range ctx.cfg.Commits.Prefixes {
			formatted := p + "(" + sliceID + "): "
			if strings.HasPrefix(message, formatted) {
				message = strings.TrimPrefix(message, formatted)
				fmt.Fprintf(os.Stderr, "note: stripped %q from -m — metis adds the prefix; pass the bare message\n", formatted)
				break
			}
		}

		// metis commit does not auto-stage; fail with guidance, not a raw
		// git error, when nothing is staged.
		if staged, err := git.HasStagedChanges(ctx.repoRoot); err == nil && !staged {
			return fmt.Errorf("nothing staged — 'git add' your changed files first (metis commit commits the index)")
		}

		prefix, _ := cmd.Flags().GetString("prefix")
		if prefix == "" {
			prefix = git.InferPrefix(sliceType)
		}

		if err := git.ValidatePrefix(ctx.cfg, prefix); err != nil {
			return err
		}

		// Format and strip attribution
		fullMessage := git.FormatCommitMessage(ctx.cfg, sliceID, prefix, message)
		if ctx.cfg.Commits.NoAttribution {
			fullMessage = git.StripAttribution(fullMessage)
		}

		// Execute commit
		if amend {
			if err := git.CommitAmend(ctx.repoRoot, fullMessage); err != nil {
				return err
			}
			fmt.Printf("Amended: %s\n", fullMessage)
		} else {
			if err := git.Commit(ctx.repoRoot, fullMessage); err != nil {
				return err
			}
			fmt.Printf("Committed: %s\n", fullMessage)
		}
		return nil
	},
}

func commitBrief(ctx *context, sliceID, message string) error {
	briefPath := filepath.Join(ctx.repoRoot, ctx.cfg.Paths.Briefs, sliceID+".md")
	if _, err := os.Stat(briefPath); os.IsNotExist(err) {
		return fmt.Errorf("brief not found at %s — create it first with 'metis brief %s --write'", briefPath, sliceID)
	}

	if err := git.Add(ctx.repoRoot, briefPath); err != nil {
		return err
	}

	subject := "slice brief"
	if message != "" {
		subject = "slice brief: " + message
	}
	// Metis must not author a commit its own audit would reject: the prefix
	// has to be one the project allows, and the caller's message goes through
	// the same attribution stripping as a normal commit.
	const prefix = "docs"
	if err := git.ValidatePrefix(ctx.cfg, prefix); err != nil {
		return fmt.Errorf("cannot commit the brief: %w — brief commits use the %q prefix, so it must stay in commits.prefixes", err, prefix)
	}
	full := git.FormatCommitMessage(ctx.cfg, sliceID, prefix, subject)
	if ctx.cfg.Commits.NoAttribution {
		full = git.StripAttribution(full)
	}
	if err := git.CommitPaths(ctx.repoRoot, full, briefPath); err != nil {
		return err
	}
	fmt.Printf("Committed brief: %s\n", full)
	return nil
}

// commitFlip flips a lifecycle flag and commits the ledger in one step. For
// "reviewed", the dispatching agent slug is passed through so the ledger can
// enforce cross-vendor review (reviewer != coder).
func commitFlip(ctx *context, sliceID, which, agent string) error {
	// Reload ledger to get the proper type
	ledgerObj, err := ctx.loadLedger()
	if err != nil {
		return err
	}

	switch which {
	case "coded":
		// Deterministic preconditions, not honor system: the brief must be
		// committed and the post-implementation verify must have passed.
		briefRel := filepath.Join(ctx.cfg.Paths.Briefs, sliceID+".md")
		briefPath := filepath.Join(ctx.repoRoot, briefRel)
		if _, err := os.Stat(briefPath); os.IsNotExist(err) {
			return fmt.Errorf("cannot flip coded: no brief at %s — 'metis brief %s --write', edit it, 'metis commit --brief'", briefPath, sliceID)
		}
		// On disk is not enough. The scope audit reads the contract at HEAD,
		// so an uncommitted brief flips coded here and then fails review with
		// "brief exists but is not committed" — the precondition has to be
		// the same one the reviewer will measure.
		if _, err := git.FileAtHead(ctx.repoRoot, briefRel); err != nil {
			return fmt.Errorf("cannot flip coded: the brief at %s is not committed — the scope audit reads the contract at HEAD; run 'metis commit --brief'", briefRel)
		}
		// Gate slices produce an evidence report, not product code — the
		// verify-post precondition applies to code-bearing slices only.
		if s := ledgerObj.FindByID(sliceID); s == nil || s.Type != slice.TypeGate {
			store := runs.NewStore(filepath.Join(ctx.repoRoot, ctx.cfg.Paths.Runs))
			_, exitCode, err := store.Read(sliceID, "verify-post")
			if err != nil {
				return fmt.Errorf("cannot flip coded: no verify-post run recorded for %s — run 'metis verify --post' first", sliceID)
			}
			if exitCode != 0 {
				return fmt.Errorf("cannot flip coded: last 'metis verify --post' for %s exited %d — fix and re-verify", sliceID, exitCode)
			}
		}
		if err := ledgerObj.FlipCoded(sliceID); err != nil {
			return err
		}
	case "reviewed":
		// The review sign-off is gated on the deterministic audit: commit
		// format and scope must pass 'metis log --validate' first.
		commits, err := git.SliceCommits(ctx.repoRoot, sliceID)
		if err != nil {
			return err
		}
		report := auditSlice(ctx, sliceID, commits)
		if !report.OK {
			return fmt.Errorf("cannot flip reviewed: the audit fails — %s\nresolve it (or block the slice) first; 'metis log %s --validate' shows the full report", auditFailureReason(&report), sliceID)
		}
		if err := ledgerObj.FlipReviewed(sliceID, agent); err != nil {
			return err
		}
	}

	// The flip is a state transition plus its commit. If the commit fails the
	// ledger must not keep the new flag: 'metis next' would report the slice
	// as reviewed with nothing in history to show for it, and the audit trail
	// the flip exists to create would be missing.
	before, err := os.ReadFile(ctx.ledgerPath())
	if err != nil {
		return fmt.Errorf("reading the ledger before the flip: %w", err)
	}
	rollback := func() {
		if werr := os.WriteFile(ctx.ledgerPath(), before, 0o644); werr != nil {
			fmt.Fprintf(os.Stderr, "warning: could not roll the ledger back to its pre-flip state: %v\n", werr)
			return
		}
		// Resync the index too, or the reverted content stays staged.
		_ = git.Add(ctx.repoRoot, ctx.ledgerPath())
	}

	if err := ctx.saveLedger(ledgerObj); err != nil {
		return err
	}

	// Stage the ledger
	if err := git.Add(ctx.repoRoot, ctx.ledgerPath()); err != nil {
		rollback()
		return err
	}

	prefix := "chore"
	if err := git.ValidatePrefix(ctx.cfg, prefix); err != nil {
		rollback()
		return fmt.Errorf("cannot flip %s: %w — metis state commits use the %q prefix, so it must stay in commits.prefixes", which, err, prefix)
	}
	message := git.FormatCommitMessage(ctx.cfg, sliceID, prefix, "flip "+which)
	if err := git.CommitPaths(ctx.repoRoot, message, ctx.ledgerPath()); err != nil {
		rollback()
		return err
	}

	fmt.Printf("Committed: %s\n", message)
	return nil
}

// auditFailureReason summarises why the audit said no, so the flip's error
// names the blocker instead of sending the caller to another command to find
// out. Scope violations come first: they are the common case and the one that
// looks like a metis fault until you see the file list.
func auditFailureReason(r *auditReport) string {
	var parts []string
	switch {
	case r.BriefUncommitted:
		parts = append(parts, "the brief is not committed (the contract is read at HEAD)")
	case !r.Gate && !r.ScopeVerifiable:
		parts = append(parts, "the brief declares no owned_paths")
	}
	if len(r.OutOfScope) > 0 {
		parts = append(parts, fmt.Sprintf("%d file(s) outside owned_paths: %s", len(r.OutOfScope), strings.Join(r.OutOfScope, ", ")))
	}
	var bad []string
	for _, c := range r.Commits {
		for _, issue := range c.Issues {
			bad = append(bad, fmt.Sprintf("%s (%s)", c.Hash, issue))
		}
	}
	if len(bad) > 0 {
		parts = append(parts, "commit issues: "+strings.Join(bad, "; "))
	}
	if len(parts) == 0 {
		return "see the audit report"
	}
	return strings.Join(parts, "; ")
}
