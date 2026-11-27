package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/techspeque/metis/internal/config"
	"github.com/techspeque/metis/internal/findings"
	"github.com/techspeque/metis/internal/git"
	"github.com/techspeque/metis/internal/ledger"
	"github.com/techspeque/metis/internal/threeway"
)

// The state files metis merges on parsed data.
const (
	stateLedger   = "ledger"
	stateArchive  = "archive"
	stateFindings = "findings"
)

func init() {
	stateMergeCmd.Flags().String("path", "", "The file's path in the repository (git's %P), which names its kind (default: <ours>)")
	stateMergeCmd.Flags().String("kind", "", "The file's kind when its name does not say: ledger, archive or findings")
	stateCmd.AddCommand(stateMergeCmd)
	rootCmd.AddCommand(stateCmd)
}

var stateCmd = &cobra.Command{
	Use:   "state",
	Short: "Work on the state files themselves",
}

var stateMergeCmd = &cobra.Command{
	Use:   "merge <base> <ours> <theirs>",
	Short: "Three-way merge a state file on its entries, as a git merge driver",
	Long: `Merges two versions of the ledger, the archive or the findings store
against their common base, by slice or finding id and not by line, and
writes the result to <ours>. It is the merge metis auto does when it
rebases a slice's branch, for people who rebase or merge by hand:

  # .gitattributes
  .metis/slices.yaml      merge=metis-state
  .metis/slices-done.yaml merge=metis-state
  .metis/findings.yaml    merge=metis-state

  git config merge.metis-state.name "metis state merge"
  git config merge.metis-state.driver "metis state merge %O %A %B --path %P"

Exit 0: merged. Exit 1: a real conflict (the same slice changed on both
sides, the same id archived twice with different content, a file that
does not parse); <ours> is left as it was. A finding id both sides
created is not a conflict: theirs' finding is renumbered and keeps the
old id as its alias.`,
	Args: cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		path, _ := cmd.Flags().GetString("path")
		kind, _ := cmd.Flags().GetString("kind")
		if path == "" {
			path = args[1]
		}
		if kind == "" {
			kind = stateKindOf(&config.PathsConfig{}, path)
		}
		var sides [3][]byte
		for i, file := range args {
			data, err := os.ReadFile(file)
			if err != nil && !os.IsNotExist(err) {
				return err
			}
			sides[i] = data
		}
		if err := mergeStateFile(kind, sides[0], sides[1], sides[2], args[1]); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		return nil
	},
}

// stateKindOf names the state file a repository path is, or "": the
// project's configured path, else the file name metis gives it.
func stateKindOf(paths *config.PathsConfig, path string) string {
	path = filepath.ToSlash(path)
	defaults := config.DefaultConfig().Paths
	for _, k := range []struct{ kind, configured, standard string }{
		{stateLedger, paths.Ledger, defaults.Ledger},
		{stateArchive, paths.Archive, defaults.Archive},
		{stateFindings, paths.Findings, defaults.Findings},
	} {
		if k.configured != "" {
			if path == filepath.ToSlash(k.configured) {
				return k.kind
			}
			continue
		}
		if filepath.Base(path) == filepath.Base(k.standard) {
			return k.kind
		}
	}
	return ""
}

// mergeStateFile three-way merges one state file from its three versions
// and writes the result to dest with the file's own Save. A side that
// does not have the file is empty.
func mergeStateFile(kind string, base, ours, theirs []byte, dest string) error {
	switch kind {
	case stateLedger:
		b, o, t, err := parse3(ledger.Parse, base, ours, theirs)
		if err != nil {
			return err
		}
		merged, err := ledger.Merge(b, o, t)
		if err != nil {
			return conflictOf("slice", err)
		}
		return merged.Save(dest)
	case stateArchive:
		b, o, t, err := parse3(ledger.ParseArchive, base, ours, theirs)
		if err != nil {
			return err
		}
		merged, err := ledger.MergeArchive(b, o, t)
		if err != nil {
			return conflictOf("archived slice", err)
		}
		return merged.Save(dest)
	case stateFindings:
		b, o, t, err := parse3(findings.Parse, base, ours, theirs)
		if err != nil {
			return err
		}
		merged, err := findings.Merge(b, o, t)
		if err != nil {
			return conflictOf("finding", err)
		}
		return merged.Save(dest)
	}
	return fmt.Errorf("not a state file metis merges (kinds: %s)", strings.Join([]string{stateLedger, stateArchive, stateFindings}, ", "))
}

func parse3[T any](parse func([]byte) (*T, error), base, ours, theirs []byte) (b, o, t *T, err error) {
	if b, err = parse(base); err != nil {
		return nil, nil, nil, fmt.Errorf("base: %w", err)
	}
	if o, err = parse(ours); err != nil {
		return nil, nil, nil, fmt.Errorf("ours: %w", err)
	}
	if t, err = parse(theirs); err != nil {
		return nil, nil, nil, fmt.Errorf("theirs: %w", err)
	}
	return b, o, t, nil
}

// conflictOf says what kind of entry a merge conflict is about.
func conflictOf(what string, err error) error {
	var c *threeway.Conflict
	if errors.As(err, &c) {
		return fmt.Errorf("%s %w", what, err)
	}
	return err
}

// resolveStateConflicts settles a rebase step's conflicts when every
// conflicted path is a state file: each is merged on its entries, written
// and staged. Any other conflicted path, or a real conflict within a
// state file, is an error and nothing is staged for it.
func resolveStateConflicts(ctx *context, paths []string) error {
	kinds := make([]string, len(paths))
	for i, p := range paths {
		if kinds[i] = stateKindOf(&ctx.cfg.Paths, p); kinds[i] == "" {
			return fmt.Errorf("conflict in %s: only the state files are merged by metis", p)
		}
	}
	for i, p := range paths {
		dest := filepath.Join(ctx.repoRoot, filepath.FromSlash(p))
		err := mergeStateFile(kinds[i], git.StageBlob(ctx.repoRoot, 1, p), git.StageBlob(ctx.repoRoot, 2, p), git.StageBlob(ctx.repoRoot, 3, p), dest)
		if err != nil {
			return fmt.Errorf("conflict in %s: %w", p, err)
		}
		if err := git.Add(ctx.repoRoot, dest); err != nil {
			return err
		}
	}
	return nil
}
