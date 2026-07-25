package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/techspeque/metis/internal/config"
)

func init() {
	configCmd.AddCommand(configViewCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
	rootCmd.AddCommand(configCmd)
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "View and edit project configuration",
	Long: `Reads and writes .metis/project.yaml through dotted key paths.

'view' and 'get' show the effective configuration (defaults applied).
'set' edits the file in place, preserving comments and formatting.

Examples:
  metis config view
  metis config get project.name
  metis config get agents.claude-code/opus.model
  metis config set commands.verify "go test ./..."
  metis config set commits.require_slice_id true
  metis config set routing.high claude-code/opus,opencode/opus`,
}

var configViewCmd = &cobra.Command{
	Use:   "view",
	Short: "Show the full effective configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadContext()
		if err != nil {
			return err
		}

		if jsonOutput() {
			return printJSON(cmd, ctx.cfg)
		}

		fmt.Printf("# effective configuration (defaults applied) — source: %s\n", ctx.cfgPath)
		data, err := yaml.Marshal(ctx.cfg)
		if err != nil {
			return err
		}
		fmt.Print(string(data))
		return nil
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Show one configuration value by dotted key path",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadContext()
		if err != nil {
			return err
		}

		value, err := config.Lookup(ctx.cfg, args[0])
		if err != nil {
			return err
		}

		if jsonOutput() {
			return printJSON(cmd, value)
		}

		switch v := value.(type) {
		case string, bool, int:
			fmt.Println(v)
		default:
			data, err := yaml.Marshal(v)
			if err != nil {
				return err
			}
			fmt.Print(string(data))
		}
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set one configuration value (comments are preserved)",
	Long: `Sets a value in .metis/project.yaml by dotted key path. The edit preserves
comments and unrelated formatting. Lists take comma-separated values.
Run 'metis check --config' afterwards to validate the full configuration.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadContext()
		if err != nil {
			return err
		}

		if err := config.SetInFile(ctx.cfgPath, args[0], args[1]); err != nil {
			return err
		}

		fmt.Printf("Set %s = %s\n", args[0], args[1])
		return nil
	},
}
