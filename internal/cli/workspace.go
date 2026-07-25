package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/techspeque/metis/internal/config"
	"github.com/techspeque/metis/internal/userconfig"
)

func init() {
	workspaceCmd.AddCommand(workspaceListCmd)
	workspaceCmd.AddCommand(workspaceAddCmd)
	workspaceCmd.AddCommand(workspaceRemoveCmd)
	workspaceCmd.AddCommand(workspaceUseCmd)
	workspaceCmd.AddCommand(workspaceCurrentCmd)
	rootCmd.AddCommand(workspaceCmd)
}

var workspaceCmd = &cobra.Command{
	Use:     "workspace",
	Aliases: []string{"ws"},
	Short:   "Manage the user-level workspace registry",
	Long: `Manages registered workspaces in ~/.metis/config.yaml.

Workspaces let you operate on projects from anywhere: 'metis workspace use'
sets the fallback project for commands run outside any repo, and the
--workspace flag targets a registered project explicitly. Inside a repo,
the repo itself always wins — the active workspace is never consulted.`,
}

// workspaceRow is the JSON shape of a workspace registry entry.
type workspaceRow struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Active  bool   `json:"active"`
	Missing bool   `json:"missing,omitempty"`
}

var workspaceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List registered workspaces",
	RunE: func(cmd *cobra.Command, args []string) error {
		uc, err := userconfig.Load()
		if err != nil {
			return err
		}

		rows := []workspaceRow{}
		for _, name := range uc.Names() {
			path := uc.Workspaces[name]
			_, findErr := config.FindConfigIn(path)
			rows = append(rows, workspaceRow{
				Name:    name,
				Path:    path,
				Active:  name == uc.Active,
				Missing: findErr != nil,
			})
		}

		if jsonOutput() {
			return printJSON(cmd, rows)
		}

		if len(rows) == 0 {
			fmt.Println("No workspaces registered. Run 'metis workspace add <name> [path]' or 'metis init'.")
			return nil
		}

		for _, row := range rows {
			markers := ""
			if row.Active {
				markers += " [active]"
			}
			if row.Missing {
				markers += " [missing]"
			}
			fmt.Printf("  %-20s %s%s\n", row.Name, row.Path, markers)
		}
		return nil
	},
}

var workspaceAddCmd = &cobra.Command{
	Use:   "add <name> [path]",
	Short: "Register a workspace",
	Long: `Registers a workspace under the given name. The path defaults to the
project root discovered from the current directory. The target must
contain a Metis project (.metis/project.yaml).`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		var root string
		if len(args) == 2 {
			abs, err := filepath.Abs(args[1])
			if err != nil {
				return fmt.Errorf("resolving path: %w", err)
			}
			root = abs
		} else {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting working directory: %w", err)
			}
			cfgPath, err := config.FindConfig(cwd)
			if err != nil {
				return fmt.Errorf("no path given and %w", err)
			}
			root = config.RootFromConfigPath(cfgPath)
		}

		if _, err := config.FindConfigIn(root); err != nil {
			return fmt.Errorf("no %s found at %s — run 'metis init' there first", config.FileName, root)
		}

		uc, err := userconfig.Load()
		if err != nil {
			return err
		}
		if err := uc.Add(name, root); err != nil {
			return err
		}
		if err := uc.Save(); err != nil {
			return err
		}

		fmt.Printf("Registered workspace %q → %s\n", name, root)
		return nil
	},
}

var workspaceRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Unregister a workspace (the project itself is not touched)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		uc, err := userconfig.Load()
		if err != nil {
			return err
		}
		wasActive := uc.Active == args[0]
		if err := uc.Remove(args[0]); err != nil {
			return err
		}
		if err := uc.Save(); err != nil {
			return err
		}

		fmt.Printf("Removed workspace %q from the registry\n", args[0])
		if wasActive {
			fmt.Println("It was the active workspace — no workspace is active now.")
		}
		return nil
	},
}

var workspaceUseCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Set the active workspace",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		uc, err := userconfig.Load()
		if err != nil {
			return err
		}
		if err := uc.Use(args[0]); err != nil {
			return err
		}
		if err := uc.Save(); err != nil {
			return err
		}

		fmt.Printf("Active workspace: %s (%s)\n", args[0], uc.Workspaces[args[0]])
		return nil
	},
}

var workspaceCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Show the active workspace",
	RunE: func(cmd *cobra.Command, args []string) error {
		uc, err := userconfig.Load()
		if err != nil {
			return err
		}
		if jsonOutput() {
			if uc.Active == "" {
				return printJSON(cmd, nil)
			}
			return printJSON(cmd, workspaceRow{
				Name:   uc.Active,
				Path:   uc.Workspaces[uc.Active],
				Active: true,
			})
		}
		if uc.Active == "" {
			fmt.Println("No active workspace.")
			return nil
		}
		fmt.Printf("%s (%s)\n", uc.Active, uc.Workspaces[uc.Active])
		return nil
	},
}
