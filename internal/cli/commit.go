package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/techspeque/metis/internal/git"
	"github.com/techspeque/metis/internal/runs"
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

		switch {
		case briefMode:
			return commitBrief(ctx, sliceID)
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

func commitBrief(ctx *context, sliceID string) error {
	briefPath := filepath.Join(ctx.repoRoot, ctx.cfg.Paths.Briefs, sliceID+".md")
	if _, err := os.Stat(briefPath); os.IsNotExist(err) {
		return fmt.Errorf("brief not found at %s — create it first with 'metis brief %s --write'", briefPath, sliceID)
	}

	if err := git.Add(ctx.repoRoot, briefPath); err != nil {
		return err
	}

	message := git.FormatCommitMessage(ctx.cfg, sliceID, "docs", "slice brief")
	if err := git.CommitPaths(ctx.repoRoot, message, briefPath); err != nil {
		return err
	}
	fmt.Printf("Committed brief: %s\n", message)
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
		briefPath := filepath.Join(ctx.repoRoot, ctx.cfg.Paths.Briefs, sliceID+".md")
		if _, err := os.Stat(briefPath); os.IsNotExist(err) {
			return fmt.Errorf("cannot flip coded: no brief at %s — 'metis brief %s --write', edit it, 'metis commit --brief'", briefPath, sliceID)
		}
		store := runs.NewStore(filepath.Join(ctx.repoRoot, ctx.cfg.Paths.Runs))
		_, exitCode, err := store.Read(sliceID, "verify-post")
		if err != nil {
			return fmt.Errorf("cannot flip coded: no verify-post run recorded for %s — run 'metis verify --post' first", sliceID)
		}
		if exitCode != 0 {
			return fmt.Errorf("cannot flip coded: last 'metis verify --post' for %s exited %d — fix and re-verify", sliceID, exitCode)
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
			return fmt.Errorf("cannot flip reviewed: 'metis log %s --validate' fails — resolve the audit (or block the slice) first", sliceID)
		}
		if err := ledgerObj.FlipReviewed(sliceID, agent); err != nil {
			return err
		}
	}

	if err := ctx.saveLedger(ledgerObj); err != nil {
		return err
	}

	// Stage the ledger
	if err := git.Add(ctx.repoRoot, ctx.ledgerPath()); err != nil {
		return err
	}

	prefix := "chore"
	message := git.FormatCommitMessage(ctx.cfg, sliceID, prefix, "flip "+which)
	if err := git.CommitPaths(ctx.repoRoot, message, ctx.ledgerPath()); err != nil {
		return err
	}

	fmt.Printf("Committed: %s\n", message)
	return nil
}
