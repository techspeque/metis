package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/techspeque/metis/internal/git"
	"github.com/techspeque/metis/internal/slice"
)

func init() {
	commitCmd.Flags().String("prefix", "", "Commit prefix (inferred from slice type if omitted)")
	commitCmd.Flags().StringP("message", "m", "", "Commit message (required unless using shortcuts)")
	commitCmd.Flags().Bool("brief", false, "Shortcut: add and commit the brief file")
	commitCmd.Flags().String("flip", "", "Shortcut: flip coded or reviewed and commit (coded|reviewed)")
	commitCmd.Flags().Bool("amend", false, "Amend the previous commit")
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

		// Handle shortcuts
		briefMode, _ := cmd.Flags().GetBool("brief")
		flipMode, _ := cmd.Flags().GetString("flip")
		amend, _ := cmd.Flags().GetBool("amend")

		switch {
		case briefMode:
			return commitBrief(ctx, l, sliceID)
		case flipMode == "coded":
			return commitFlipCoded(ctx, l, sliceID)
		case flipMode == "reviewed":
			return commitFlipReviewed(ctx, l, sliceID, result.AgentSlug)
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

func commitBrief(ctx *context, l interface{}, sliceID string) error {
	briefPath := filepath.Join(ctx.repoRoot, ctx.cfg.Paths.Briefs, sliceID+".md")
	if _, err := os.Stat(briefPath); os.IsNotExist(err) {
		return fmt.Errorf("brief not found at %s — create it first with 'metis brief %s --write'", briefPath, sliceID)
	}

	if err := git.Add(ctx.repoRoot, briefPath); err != nil {
		return err
	}

	message := git.FormatCommitMessage(ctx.cfg, sliceID, "docs", "slice brief")
	if err := git.Commit(ctx.repoRoot, message); err != nil {
		return err
	}
	fmt.Printf("Committed brief: %s\n", message)
	return nil
}

func commitFlipCoded(ctx *context, l interface {
	FlipCoded(string) error
	Save(string) error
}, sliceID string) error {
	// This is a simplified version — the real one uses the ledger type
	return commitFlipGeneric(ctx, sliceID, "coded")
}

func commitFlipReviewed(ctx *context, l interface{}, sliceID, agent string) error {
	return commitFlipGeneric(ctx, sliceID, "reviewed")
}

func commitFlipGeneric(ctx *context, sliceID string, which string) error {
	// Reload ledger to get the proper type
	ledgerObj, err := ctx.loadLedger()
	if err != nil {
		return err
	}

	switch which {
	case "coded":
		if err := ledgerObj.FlipCoded(sliceID); err != nil {
			return err
		}
	case "reviewed":
		if err := ledgerObj.FlipReviewed(sliceID, ""); err != nil {
			return err
		}
	default:
		return fmt.Errorf("invalid flip target: %s (use 'coded' or 'reviewed')", which)
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
	if err := git.Commit(ctx.repoRoot, message); err != nil {
		return err
	}

	fmt.Printf("Committed: %s\n", message)
	return nil
}

// unused but required for the interface approach — simplified for now
var _ slice.WorkType
