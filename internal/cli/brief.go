package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/techspeque/metis/internal/brief"
)

func init() {
	briefCmd.Flags().Bool("write", false, "Write the brief template to .metis/briefs/<id>.md")
	rootCmd.AddCommand(briefCmd)
}

var briefCmd = &cobra.Command{
	Use:   "brief <id>",
	Short: "Emit or write the brief template for a slice",
	Long: `If the brief already exists, prints it. Otherwise, generates the type-appropriate
template. Use --write to create the file.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadContext()
		if err != nil {
			return err
		}

		l, err := ctx.loadLedger()
		if err != nil {
			return err
		}

		s := l.FindByID(args[0])
		if s == nil {
			// Archived slices remain readable — their briefs are the
			// archaeology later slices are told to consult.
			if archive, aerr := ctx.loadArchive(); aerr == nil {
				for i := range archive.Slices {
					if archive.Slices[i].ID == args[0] {
						s = &archive.Slices[i]
						break
					}
				}
			}
		}
		if s == nil {
			return fmt.Errorf("slice %q not found in ledger or archive", args[0])
		}

		briefPath := filepath.Join(ctx.repoRoot, ctx.cfg.Paths.Briefs, s.ID+".md")

		// If brief already exists, print it
		if data, err := os.ReadFile(briefPath); err == nil {
			if jsonOutput() {
				return printJSON(cmd, briefOutput{ID: s.ID, Path: briefPath, Exists: true, Content: string(data)})
			}
			fmt.Print(string(data))
			return nil
		}

		// Generate template
		content := brief.Render(s)

		writeMode, _ := cmd.Flags().GetBool("write")
		if writeMode {
			dir := filepath.Dir(briefPath)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("creating briefs directory: %w", err)
			}
			if err := os.WriteFile(briefPath, []byte(content), 0o644); err != nil {
				return fmt.Errorf("writing brief: %w", err)
			}
			fmt.Printf("Brief written to %s\n", briefPath)
			return nil
		}

		if jsonOutput() {
			return printJSON(cmd, briefOutput{ID: s.ID, Path: briefPath, Exists: false, Content: content})
		}
		fmt.Print(content)
		return nil
	},
}

// briefOutput is the JSON shape of 'metis brief'.
type briefOutput struct {
	ID      string `json:"id"`
	Path    string `json:"path"`
	Exists  bool   `json:"exists"`
	Content string `json:"content"`
}
